package main

import (
	"context"
	"log"
	"net/http"

	"github.com/alicebob/miniredis/v2"
	"github.com/finops-gateway/internal/ledger"
	"github.com/finops-gateway/internal/middleware"
	"github.com/finops-gateway/internal/ml"
	"github.com/finops-gateway/internal/proxy"
	"github.com/finops-gateway/internal/schema"
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

	// 2. Initialize the JSONL Audit Ledger
	auditLedger := ledger.NewLedger("audit_logs.jsonl")

	// 3. Connect to the local ML Sidecar
	mlClient := ml.NewClient("http://localhost:8000")

	// 4. Point Upstream to local mock LLM
	upstreamURL := "http://localhost:8081"
	proxyHandler, err := proxy.NewHandler(upstreamURL)
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

	// Streaming Branch (with PII masking in-memory)
	streamingChain := middleware.StreamInterceptor(proxyHandler)
	streamingChain = middleware.PIIMasker(streamingChain)

	// Branch Router
	branchRouter := func(w http.ResponseWriter, r *http.Request) {
		isStream := false
		if payload, ok := r.Context().Value(middleware.StandardPayloadKey).(*schema.StandardPayload); ok {
			isStream = payload.Stream
		}

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
	handler = middleware.InboundAdapter(handler)
	
	// Add the Audit Logger at the very edge!
	handler = middleware.AuditLogger(auditLedger, handler)

	// Inject $0.05 (50,000 micro-dollars) into the teams' budgets so the demo runs
	rdb.Set(context.Background(), "budget:team:team_marketing", 50000, 0)
	rdb.Set(context.Background(), "budget:team:team_engineering", 50000, 0)

	// 7. Start the Server
	log.Printf("Listening on :8080 and forwarding to %s...\n", upstreamURL)
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
