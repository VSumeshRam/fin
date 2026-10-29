package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	DashboardPort  string
	RedisAddr      string
	RedisPass      string
	PostgresDSN    string
	UpstreamURL    string
	UpstreamAPIKey string
	MLSidecarURL   string
	WorkerCount    int
}

func Load() *Config {
	_ = godotenv.Load() // Ignore error if .env doesn't exist

	port := getEnv("PORT", "8080")
	dashboardPort := getEnv("DASHBOARD_PORT", "3000")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPass := getEnv("REDIS_PASS", "")
	postgresDSN := getEnv("POSTGRES_DSN", "postgres://finops:finops_password@localhost:5432/finops_db?sslmode=disable")
	upstreamURL := getEnv("UPSTREAM_URL", "https://api.openai.com")
	upstreamAPIKey := getEnv("UPSTREAM_API_KEY", "")
	mlSidecarURL := getEnv("ML_SIDECAR_URL", "http://localhost:8000")
	
	workerCountStr := getEnv("WORKER_COUNT", "10")
	workerCount, err := strconv.Atoi(workerCountStr)
	if err != nil {
		log.Printf("Invalid WORKER_COUNT, defaulting to 10")
		workerCount = 10
	}

	return &Config{
		Port:           port,
		DashboardPort:  dashboardPort,
		RedisAddr:      redisAddr,
		RedisPass:      redisPass,
		PostgresDSN:    postgresDSN,
		UpstreamURL:    upstreamURL,
		UpstreamAPIKey: upstreamAPIKey,
		MLSidecarURL:   mlSidecarURL,
		WorkerCount:    workerCount,
	}
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
