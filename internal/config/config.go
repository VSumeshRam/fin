package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port         string
	RedisAddr    string
	RedisPass    string
	PostgresDSN  string
	UpstreamURL  string
	WorkerCount  int
}

func Load() *Config {
	_ = godotenv.Load() // Ignore error if .env doesn't exist

	port := getEnv("PORT", "8080")
	redisAddr := getEnv("REDIS_ADDR", "localhost:6379")
	redisPass := getEnv("REDIS_PASS", "")
	postgresDSN := getEnv("POSTGRES_DSN", "postgres://finops:finops_password@localhost:5432/finops_db?sslmode=disable")
	upstreamURL := getEnv("UPSTREAM_URL", "https://api.openai.com")
	
	workerCountStr := getEnv("WORKER_COUNT", "10")
	workerCount, err := strconv.Atoi(workerCountStr)
	if err != nil {
		log.Printf("Invalid WORKER_COUNT, defaulting to 10")
		workerCount = 10
	}

	return &Config{
		Port:         port,
		RedisAddr:    redisAddr,
		RedisPass:    redisPass,
		PostgresDSN:  postgresDSN,
		UpstreamURL:  upstreamURL,
		WorkerCount:  workerCount,
	}
}

func getEnv(key, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}
