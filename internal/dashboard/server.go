package dashboard

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/redis/go-redis/v9"
)

// DashboardStats represents the aggregated metrics sent to the UI
type DashboardStats struct {
	TeamBudgets       map[string]float64 `json:"team_budgets"`
	TotalSavedUSD     float64            `json:"total_saved_usd"`
	SecurityIncidents int                `json:"security_incidents"`
	ModelUsage        map[string]int     `json:"model_usage"`
	TotalRequests     int                `json:"total_requests"`
}

// StartServer boots the analytics dashboard on the specified port
func StartServer(port string, redisClient *redis.Client, ledgerPath string) {
	mux := http.NewServeMux()

	// Serve the static HTML dashboard
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "internal/dashboard/static/index.html")
	})

	// Serve the live data API
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		stats := calculateStats(redisClient, ledgerPath)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	})

	go func() {
		fmt.Printf("Executive FinOps Dashboard running on http://localhost:%s\n", port)
		if err := http.ListenAndServe(":"+port, mux); err != nil {
			fmt.Printf("Dashboard server failed: %v\n", err)
		}
	}()
}

func calculateStats(rdb *redis.Client, ledgerPath string) DashboardStats {
	stats := DashboardStats{
		TeamBudgets: make(map[string]float64),
		ModelUsage:  make(map[string]int),
	}

	// 1. Fetch live budgets from Redis
	ctx := context.Background()
	keys, err := rdb.Keys(ctx, "budget:team:*").Result()
	if err == nil {
		for _, key := range keys {
			val, err := rdb.Get(ctx, key).Int64()
			if err == nil {
				teamName := strings.TrimPrefix(key, "budget:team:")
				stats.TeamBudgets[teamName] = float64(val) / 1000000.0 // Convert micro-dollars to USD
			}
		}
	}

	// 2. Parse Audit Ledger for historical metrics
	file, err := os.Open(ledgerPath)
	if err != nil {
		return stats // Return what we have if ledger is empty/missing
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var record map[string]interface{}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}

		stats.TotalRequests++

		// Track Model Usage
		if model, ok := record["model"].(string); ok && model != "" {
			stats.ModelUsage[model]++
		}

		// Track Security Incidents
		if blockedReason, ok := record["blocked_reason"].(string); ok && blockedReason != "" {
			stats.SecurityIncidents++
		}

		// Calculate Cache Savings (If CacheHit is true, we assume it saved the cost of a typical request. 
		// For a more robust metric, we should log the *prevented* cost, but this works for the demo).
		if cacheHit, ok := record["cache_hit"].(bool); ok && cacheHit {
			// Estimate savings: $0.0085 per typical request (as per FinOps math proof)
			stats.TotalSavedUSD += 0.0085 
		}
	}

	return stats
}
