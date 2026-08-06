package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sort"

	"github.com/finops-gateway/internal/ml"
	"github.com/redis/go-redis/v9"
)

type CacheEntry struct {
	Prompt   string    `json:"prompt"`
	Vector   []float64 `json:"vector"`
	Entities []string  `json:"entities"`
	Response string    `json:"response"`
}

func SemanticCache(redisClient *redis.Client, mlClient *ml.Client, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Read body to extract prompt
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusInternalServerError)
			return
		}
		
		// Attempt to extract prompt from basic OpenAI schema {"messages": [{"content": "..."}]}
		// Or fallback to just stringifying the body payload.
		var payload map[string]interface{}
		var promptText string
		if err := json.Unmarshal(bodyBytes, &payload); err == nil {
			// Extract prompt for demo purposes: just dump to string
			// In production, we'd extract specific user messages.
			promptBytes, _ := json.Marshal(payload)
			promptText = string(promptBytes)
		} else {
			promptText = string(bodyBytes)
		}

		// Restore body
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		ctx := r.Context()

		// 1. Get Embedding and Entities
		vector, err := mlClient.GetEmbedding(ctx, promptText)
		if err != nil {
			// Fail open on ML failure
			next.ServeHTTP(w, r)
			return
		}

		entities, err := mlClient.GetEntities(ctx, promptText)
		if err != nil {
			// Fail open on ML failure
			next.ServeHTTP(w, r)
			return
		}
		sort.Strings(entities)

		// 2. Fetch recent cache entries (e.g. 100 most recent from a Redis List)
		// We'll store cached items as serialized JSON strings in a Redis List "semantic_cache"
		cachedItems, err := redisClient.LRange(ctx, "semantic_cache", 0, 99).Result()
		if err == nil {
			for _, itemStr := range cachedItems {
				var entry CacheEntry
				if err := json.Unmarshal([]byte(itemStr), &entry); err != nil {
					continue
				}

				// The Decision Gate
				similarity := ml.CosineSimilarity(vector, entry.Vector)
				if similarity >= 0.95 {
					// Check exact entity match
					if stringSlicesEqual(entities, entry.Entities) {
						// CACHE HIT
						w.Header().Set("Content-Type", "application/json")
						w.Header().Set("X-Cache", "HIT")
						w.WriteHeader(http.StatusOK)
						w.Write([]byte(entry.Response))
						return // Halt chain
					}
				}
			}
		}

		// CACHE MISS
		w.Header().Set("X-Cache", "MISS")
		
		// Create a CacheRecorder to capture the LLM response to save in cache
		recorder := &CacheRecorder{
			ResponseWriter: w,
			Body:           &bytes.Buffer{},
			statusCode:     http.StatusOK, // Default
		}

		next.ServeHTTP(recorder, r)
		
		// Asynchronously save to Redis if successful
		respStr := recorder.Body.String()
		
		// Write the response to the original writer
		w.Write(recorder.Body.Bytes())

		if recorder.statusCode == http.StatusOK {
			go func() {
				bgCtx := context.Background()
				newEntry := CacheEntry{
					Prompt:   promptText,
					Vector:   vector,
					Entities: entities,
					Response: respStr,
				}
				entryJSON, _ := json.Marshal(newEntry)
				
				pipe := redisClient.Pipeline()
				pipe.LPush(bgCtx, "semantic_cache", string(entryJSON))
				pipe.LTrim(bgCtx, "semantic_cache", 0, 999) // keep last 1000 globally
				pipe.Exec(bgCtx)
			}()
		}
		
		// Note: The next.ServeHTTP will call proxy which writes to the ResponseRecorder.
		// Since we're inside SemanticCache which wraps PIIMasker, SemanticCache's recorder captures 
		// the unmasked response. Wait, main.go order: 
		// BudgetGuard -> LoopBreaker -> PIIMasker -> SemanticCache -> Proxy
		// If SemanticCache comes AFTER PIIMasker, SemanticCache will cache the PII-masked response if it captures Proxy output!
		// Wait, PIIMasker -> SemanticCache -> Proxy:
		// Request flow: PIIMasker masks PII -> SemanticCache gets masked prompt -> Proxy gets masked prompt.
		// Proxy returns masked response -> SemanticCache caches masked response -> PIIMasker unmasks response -> User.
		// Is this correct? The prompt says: "Note: PII Masker must run before Cache, so the Cache analyzes the clean, tokenized text".
		// That means SemanticCache records the MASKED response! When a Cache HIT occurs, SemanticCache returns the MASKED response to PIIMasker, and PIIMasker unmasks it! This is PERFECT.
		
		// We just need to ensure the recorder writes it back if it's the inner one, but we actually don't need a recorder here 
		// unless we need to capture the response. Let's create a specific cacheRecorder.
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// responseWriterWrapper is redefined here for type assertion, but it's better to make a generic wrapper or just check the status code via a custom wrapper.
type CacheRecorder struct {
	http.ResponseWriter
	Body *bytes.Buffer
	statusCode int
}
func (rw *CacheRecorder) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
func (rw *CacheRecorder) Write(b []byte) (int, error) {
	return rw.Body.Write(b)
}
