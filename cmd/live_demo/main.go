package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/alicebob/miniredis/v2"
	"github.com/finops-gateway/internal/middleware"
	"github.com/finops-gateway/internal/ml"
	"github.com/finops-gateway/internal/proxy"
	"github.com/redis/go-redis/v9"
)

func main() {
	log.Println("Starting FinOps Gateway Live Demo...")

	// 1. Start In-Memory Redis (No Docker required!)
	mr, err := miniredis.Run()
	if err != nil {
		log.Fatalf("Failed to start miniredis: %v", err)
	}
	defer mr.Close()
	log.Printf("Started in-memory Redis at %s", mr.Addr())

	rdb := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// 2. We skip Postgres for this demo (nil passed to handler)
	log.Println("Bypassing Postgres Audit Ledger for local demo run.")

	// 3. Connect to the local ML Sidecar
	mlClient := ml.NewClient("http://localhost:8000")

	// 4. Point Upstream directly to OpenRouter (Free OpenAI Compatible API)
	upstreamURL := "https://openrouter.ai"
	proxyHandler, err := proxy.NewHandler(upstreamURL, nil)
	if err != nil {
		log.Fatalf("Failed to initialize proxy handler: %v", err)
	}

	// 5. Load RBAC
	rbacEngine, err := middleware.LoadPolicies("rbac/policies.json")
	if err != nil {
		log.Fatalf("Failed to load RBAC policies: %v", err)
	}

	// 6. Build the Middleware Chain
	// Non-Streaming Branch
	nonStreamingChain := middleware.SemanticCache(rdb, mlClient, proxyHandler)
	nonStreamingChain = middleware.MCPFirewall(rbacEngine, nonStreamingChain)
	nonStreamingChain = middleware.PIIMasker(nonStreamingChain)

	// Streaming Branch
	streamingChain := middleware.StreamInterceptor(proxyHandler)

	// Branch Router
	branchRouter := func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusInternalServerError)
			return
		}
		
		isStream := false
		var payload map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &payload); err == nil {
			if stream, ok := payload["stream"].(bool); ok && stream {
				isStream = true
			}
		}
		
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		if isStream {
			streamingChain.ServeHTTP(w, r)
		} else {
			nonStreamingChain.ServeHTTP(w, r)
		}
	}

	// Base Chain
	handler := middleware.StateReplay(rdb, branchRouter)
	handler = middleware.LoopBreaker(rdb, handler)
	handler = middleware.BudgetGuard(rdb, handler)

	// Inject 50,000 tokens into the marketing team's budget so the demo runs
	rdb.Set(context.Background(), "budget:team_marketing", 50000, 0)
	rdb.Set(context.Background(), "budget:team_engineering", 50000, 0)

	// 7. Start the Server
	log.Printf("Listening on :8080 and forwarding to %s...\n", upstreamURL)
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
