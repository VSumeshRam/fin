package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

func StateReplay(redisClient *redis.Client, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.Header.Get("X-Session-ID")
		if sessionID == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Read and hash the ENTIRE request body for idempotency
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusInternalServerError)
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		hash := sha256.Sum256(bodyBytes)
		promptHash := hex.EncodeToString(hash[:])
		cacheKey := "session:" + sessionID + ":hash:" + promptHash

		ctx := r.Context()

		// Check Redis for existing checkpoint
		cachedResp, err := redisClient.Get(ctx, cacheKey).Result()
		if err == nil && cachedResp != "" {
			// CACHE HIT: Fast-forward agent
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-State-Replay", "HIT")
			w.Header().Set("Content-Length", strconv.Itoa(len(cachedResp)))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(cachedResp))
			return
		}

		// CACHE MISS: Use CacheRecorder to buffer upstream response
		recorder := &CacheRecorder{
			ResponseWriter: w,
			Body:           &bytes.Buffer{},
			statusCode:     http.StatusOK, // Default
		}

		next.ServeHTTP(recorder, r)

		respBytes := recorder.Body.Bytes()
		
		// If response wasn't already written by a nested middleware (e.g. StreamInterceptor writes directly),
		// we need to handle writing it. Actually, Proxy copies to recorder.
		// Wait, StreamInterceptor uses its own Recorder, so if stream: true, this doesn't run? 
		// StateReplay is for tool calls, which are non-streaming JSON blocks. So we only cache non-streaming responses.
		w.Write(respBytes)
		
		if recorder.statusCode == http.StatusOK {
			// Only cache if the response contains a successful tool_call
			if strings.Contains(string(respBytes), `"tool_calls"`) {
				var payload map[string]interface{}
				if err := json.Unmarshal(respBytes, &payload); err == nil {
					// Verify it actually has choices and tool_calls
					if choices, ok := payload["choices"].([]interface{}); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]interface{}); ok {
							if message, ok := choice["message"].(map[string]interface{}); ok {
								if _, ok := message["tool_calls"]; ok {
									// Cache the response with a 1-hour TTL
									redisClient.Set(context.Background(), cacheKey, respBytes, time.Hour)
								}
							}
						}
					}
				}
			}
		}
	}
}
