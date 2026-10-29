package main

import (
	"context"
	"log"
	"net/http"

	"github.com/finops-gateway/internal/config"
	"github.com/finops-gateway/internal/dashboard"
	"github.com/finops-gateway/internal/ledger"
	"github.com/finops-gateway/internal/middleware"
	"github.com/finops-gateway/internal/ml"
	"github.com/finops-gateway/internal/proxy"
	"github.com/finops-gateway/internal/schema"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg := config.Load()

	// 1. Initialize Redis Client
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPass,
		DB:       0,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis successfully.")

	// 2. Initialize Audit Ledger (Async workers to JSONL)
	auditLedger := ledger.NewLedger("audit_logs.jsonl")
	
	// 3. Initialize ML Sidecar Client
	mlClient := ml.NewClient(cfg.MLSidecarURL)

	// 4. Initialize Proxy Handler
	proxyHandler, err := proxy.NewHandler(cfg.UpstreamURL)
	if err != nil {
		log.Fatalf("Failed to initialize proxy handler: %v", err)
	}

	// 5. Initialize RBAC Policy Engine
	rbacEngine, err := middleware.LoadPolicies("rbac/policies.json")
	if err != nil {
		log.Fatalf("Failed to load RBAC policies: %v", err)
	}
	log.Println("Loaded MCP RBAC Policies.")

	// 6. Middlewares
	
	// Non-Streaming Branch (PII Masker -> MCP Firewall -> Semantic Cache -> Proxy)
	nonStreamingChain := middleware.SemanticCache(rdb, mlClient, proxyHandler)
	nonStreamingChain = middleware.MCPFirewall(rbacEngine, nonStreamingChain)
	nonStreamingChain = middleware.PIIMasker(nonStreamingChain)

	// Streaming Branch (PII Masker in-memory -> Stream Interceptor -> Proxy)
	streamingChain := middleware.StreamInterceptor(proxyHandler)
	streamingChain = middleware.PIIMasker(streamingChain)

	// Conditional Router
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

	// Base Chain (AuditLogger -> InboundAdapter -> Budget Guard -> Loop Breaker -> State Replay -> Branch Router)
	handler := middleware.StateReplay(rdb, branchRouter)
	handler = middleware.LoopBreaker(rdb, handler)
	handler = middleware.BudgetGuard(rdb, handler)
	handler = middleware.InboundAdapter(handler)
	handler = middleware.AuditLogger(auditLedger, handler)

	// 7. Start Real-Time Executive Dashboard concurrently
	dashboard.StartServer(cfg.DashboardPort, rdb, "audit_logs.jsonl")

	// 8. Start HTTP Server
	log.Printf("Starting FinOps Gateway on :%s (Upstream: %s)\n", cfg.Port, cfg.UpstreamURL)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
