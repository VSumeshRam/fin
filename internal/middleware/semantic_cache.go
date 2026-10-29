package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/finops-gateway/internal/ledger"
	"github.com/finops-gateway/internal/ml"
	"github.com/finops-gateway/internal/schema"
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
		// 1. Consume in-memory StandardPayload pointer from Context (Zero JSON unmarshal overhead!)
		payload, ok := r.Context().Value(StandardPayloadKey).(*schema.StandardPayload)
		if !ok || payload == nil {
			next.ServeHTTP(w, r)
			return
		}

		// 2. Pure Prompt Extraction: Extract only user conversational intent, ignoring model/metadata noise
		var userMessages []string
		for _, msg := range payload.Messages {
			if strings.ToLower(msg.Role) == "user" && strings.TrimSpace(msg.Content) != "" {
				userMessages = append(userMessages, strings.TrimSpace(msg.Content))
			}
		}
		promptText := strings.Join(userMessages, " ")
		if promptText == "" {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()

		// 3. Query ML Sidecar for Vector Embedding and Named Entities (Fail-open if sidecar is offline)
		vector, err := mlClient.GetEmbedding(ctx, promptText)
		if err != nil {
			log.Printf("[SemanticCache] ML Sidecar embed error (failing open): %v", err)
			next.ServeHTTP(w, r)
			return
		}

		entities, err := mlClient.GetEntities(ctx, promptText)
		if err != nil {
			log.Printf("[SemanticCache] ML Sidecar NER error (failing open): %v", err)
			next.ServeHTTP(w, r)
			return
		}
		sort.Strings(entities)

		// 4. Scan Redis Semantic Cache List for a >= 0.90 Cosine + Exact Entity Gate match
		cachedItems, err := redisClient.LRange(ctx, "semantic_cache", 0, 99).Result()
		if err == nil {
			for _, itemStr := range cachedItems {
				var entry CacheEntry
				if err := json.Unmarshal([]byte(itemStr), &entry); err != nil {
					continue
				}

				similarity := ml.CosineSimilarity(vector, entry.Vector)
				if similarity >= 0.90 {
					// Entity Verification Gate: Prevent false positives (e.g. France vs Germany)
					if stringSlicesEqual(entities, entry.Entities) {
						log.Printf("[SemanticCache] CACHE HIT (Similarity: %.4f, Entities: %v)", similarity, entities)

						// Populate FinOps AuditRecord with $0 cost and CacheHit = true
						if rec, ok := r.Context().Value(LogRecordKey).(*ledger.AuditRecord); ok {
							hash := sha256.Sum256([]byte(promptText))
							rec.Model = payload.Model
							rec.PromptHash = hex.EncodeToString(hash[:])
							rec.PromptTokens = 0
							rec.CompletionTokens = 0
							rec.TotalTokens = 0
							rec.MicroDollarCost = 0
							rec.CacheHit = true
						}

						respBytes := []byte(entry.Response)
						w.Header().Set("Content-Type", "application/json")
						w.Header().Set("X-Cache", "HIT")
						w.Header().Set("X-Cache-Similarity", fmt.Sprintf("%.4f", similarity))
						w.Header().Set("Content-Length", strconv.Itoa(len(respBytes)))
						w.WriteHeader(http.StatusOK)
						w.Write(respBytes)
						return // Short-circuit upstream LLM call!
					}
				}
			}
		}

		// 5. CACHE MISS: Forward to Upstream Proxy
		w.Header().Set("X-Cache", "MISS")

		recorder := &CacheRecorder{
			ResponseWriter: w,
			Body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
		}

		next.ServeHTTP(recorder, r)

		respBytes := recorder.Body.Bytes()
		w.Header().Set("Content-Length", strconv.Itoa(len(respBytes)))
		w.WriteHeader(recorder.statusCode)
		w.Write(respBytes)

		// 6. Asynchronously store successful non-empty responses in Redis
		if recorder.statusCode == http.StatusOK && len(respBytes) > 0 {
			respStr := string(respBytes)
			go func() {
				bgCtx := context.Background()
				newEntry := CacheEntry{
					Prompt:   promptText,
					Vector:   vector,
					Entities: entities,
					Response: respStr,
				}
				entryJSON, err := json.Marshal(newEntry)
				if err != nil {
					return
				}

				pipe := redisClient.Pipeline()
				pipe.LPush(bgCtx, "semantic_cache", string(entryJSON))
				pipe.LTrim(bgCtx, "semantic_cache", 0, 999)
				_, _ = pipe.Exec(bgCtx)
			}()
		}
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

// CacheRecorder buffers the body AND defers WriteHeader so inner middlewares
// (like MCPFirewall) can inspect and override status codes cleanly.
type CacheRecorder struct {
	http.ResponseWriter
	Body       *bytes.Buffer
	statusCode int
}

func (rw *CacheRecorder) WriteHeader(code int) {
	rw.statusCode = code
}

func (rw *CacheRecorder) Write(b []byte) (int, error) {
	return rw.Body.Write(b)
}
