package main

import (
	"context"
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

	// 6. Wrap with Middlewares
	// Order: BudgetGuard -> LoopBreaker -> PIIMasker -> SemanticCache -> Proxy
	// Note: We build from inside out
	handler := middleware.SemanticCache(rdb, mlClient, proxyHandler)
	handler = middleware.PIIMasker(handler)
	handler = middleware.LoopBreaker(rdb, handler)
	handler = middleware.BudgetGuard(rdb, handler)

	// 7. Start HTTP Server
	log.Printf("Starting FinOps Gateway on :%s (Upstream: %s)\n", cfg.Port, cfg.UpstreamURL)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
