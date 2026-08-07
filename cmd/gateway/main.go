package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/finops-gateway/internal/config"
	"github.com/finops-gateway/internal/ledger"
	"github.com/finops-gateway/internal/middleware"
	"github.com/finops-gateway/internal/ml"
	"github.com/finops-gateway/internal/proxy"
	"github.com/jackc/pgx/v5/pgxpool"
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

	// 2. Initialize PostgreSQL Pool
	pool, err := pgxpool.New(context.Background(), cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pool.Close()
	log.Println("Connected to PostgreSQL successfully.")

	// 3. Initialize Audit Ledger (Async workers)
	auditLedger := ledger.NewLedger(pool, cfg.WorkerCount)
	
	// 4. Initialize ML Sidecar Client
	mlClient := ml.NewClient("http://localhost:8000")

	// 5. Initialize Proxy Handler
	proxyHandler, err := proxy.NewHandler(cfg.UpstreamURL, auditLedger)
	if err != nil {
		log.Fatalf("Failed to initialize proxy handler: %v", err)
	}

	// 6. Initialize RBAC Policy Engine
	rbacEngine, err := middleware.LoadPolicies("rbac/policies.json")
	if err != nil {
		log.Fatalf("Failed to load RBAC policies: %v", err)
	}
	log.Println("Loaded MCP RBAC Policies.")

	// 7. Middlewares
	
	// Non-Streaming Branch (PII Masker -> MCP Firewall -> Semantic Cache -> Proxy)
	nonStreamingChain := middleware.SemanticCache(rdb, mlClient, proxyHandler)
	nonStreamingChain = middleware.MCPFirewall(rbacEngine, nonStreamingChain)
	nonStreamingChain = middleware.PIIMasker(nonStreamingChain)

	// Streaming Branch (Stream Interceptor -> Proxy)
	streamingChain := middleware.StreamInterceptor(proxyHandler)

	// Conditional Router
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

	// Base Chain (Budget Guard -> Loop Breaker -> State Replay -> Branch Router)
	handler := middleware.StateReplay(rdb, branchRouter)
	handler = middleware.LoopBreaker(rdb, handler)
	handler = middleware.BudgetGuard(rdb, handler)

	// 8. Start HTTP Server
	log.Printf("Starting FinOps Gateway on :%s (Upstream: %s)\n", cfg.Port, cfg.UpstreamURL)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
