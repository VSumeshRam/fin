package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/redis/go-redis/v9"
)

func LoopBreaker(redisClient *redis.Client, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		teamID := r.Header.Get("X-Team-ID")
		if teamID == "" {
			// Skip if no team ID, budget guard will catch it if required
			next.ServeHTTP(w, r)
			return
		}

		// Read and hash the request body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		
		// Restore body for proxy
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		hash := sha256.Sum256(body)
		hashStr := hex.EncodeToString(hash[:])

		ctx := context.Background()
		key := "history:team:" + teamID

		// Use a pipeline to push, trim, and fetch the list
		pipe := redisClient.Pipeline()
		pipe.LPush(ctx, key, hashStr)
		pipe.LTrim(ctx, key, 0, 4) // Keep last 5
		historyCmd := pipe.LRange(ctx, key, 0, 4)
		
		_, err = pipe.Exec(ctx)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		history := historyCmd.Val()
		
		// Count occurrences of the current hash in the sliding window
		count := 0
		for _, h := range history {
			if h == hashStr {
				count++
			}
		}

		if count >= 3 {
			http.Error(w, "Loop Detected: Request Dropped", http.StatusTooManyRequests) // HTTP 429
			return
		}

		next.ServeHTTP(w, r)
	}
}
