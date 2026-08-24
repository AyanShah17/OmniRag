package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port              string
	DatabaseURL       string
	DBDriver          string
	RedisURL          string
	UseInMemoryQueue  bool
	PythonRAGURL      string
	Environment       string
	SecretKey         string
	DefaultSyncPeriod int // in seconds
}

func LoadConfig() *Config {
	port := getEnv("GO_ENGINE_PORT", "8080")
	dbURL := getEnv("GO_DATABASE_URL", "./omnirag_go.db")
	dbDriver := getEnv("DB_DRIVER", "sqlite")
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379/0")
	useInMemoryQueue := getEnvBool("USE_IN_MEMORY_QUEUE", true)
	pythonRAGURL := getEnv("PYTHON_RAG_URL", "http://localhost:8000")
	environment := getEnv("ENVIRONMENT", "development")
	secretKey := getEnv("SECRET_KEY", "omnirag-default-secret-key-2026")
	syncPeriod := getEnvInt("DEFAULT_SYNC_PERIOD_SECONDS", 60)

	return &Config{
		Port:              port,
		DatabaseURL:       dbURL,
		DBDriver:          dbDriver,
		RedisURL:          redisURL,
		UseInMemoryQueue:  useInMemoryQueue,
		PythonRAGURL:      pythonRAGURL,
		Environment:       environment,
		SecretKey:         secretKey,
		DefaultSyncPeriod: syncPeriod,
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return fallback
	}
	return b
}

func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return i
}
