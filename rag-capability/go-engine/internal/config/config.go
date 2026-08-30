package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port              string
	DatabaseURL       string
	PythonRAGURL      string
	DefaultSyncPeriod int // in seconds

	// Auth: "development" or "production". See internal/middleware.ClerkAuthMiddleware.
	AuthMode     string
	ClerkJWKSURL string
	ClerkIssuer  string

	// Shared secret for service-to-service calls into Python's internal-only
	// endpoints. Must match INTERNAL_SERVICE_SECRET
	// on the Python side.
	InternalServiceSecret string
	CORSOrigins          []string
}

func LoadConfig() *Config {
	port := getEnv("GO_ENGINE_PORT", "8080")
	dbURL := getEnv("GO_DATABASE_URL", "")
	pythonRAGURL := getEnv("PYTHON_RAG_URL", "http://localhost:8000")
	syncPeriod := getEnvInt("DEFAULT_SYNC_PERIOD_SECONDS", 60)
	authMode := getEnv("AUTH_MODE", "development")
	clerkJWKSURL := getEnv("CLERK_JWKS_URL", "")
	clerkIssuer := getEnv("CLERK_ISSUER", "")
	internalServiceSecret := getEnv("INTERNAL_SERVICE_SECRET", "")
	corsOrigins := strings.Split(getEnv("CORS_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000"), ",")

	return &Config{
		Port:                  port,
		DatabaseURL:           dbURL,
		PythonRAGURL:          pythonRAGURL,
		DefaultSyncPeriod:     syncPeriod,
		AuthMode:              authMode,
		ClerkJWKSURL:          clerkJWKSURL,
		ClerkIssuer:           clerkIssuer,
		InternalServiceSecret: internalServiceSecret,
		CORSOrigins:          corsOrigins,
	}
}

func (c *Config) Validate() error {
	if !strings.EqualFold(c.AuthMode, "production") {
		return nil
	}
	if c.ClerkJWKSURL == "" || c.InternalServiceSecret == "" || os.Getenv("ENCRYPTION_KEY") == "" {
		return fmt.Errorf("production requires CLERK_JWKS_URL, INTERNAL_SERVICE_SECRET, and ENCRYPTION_KEY")
	}
	for _, origin := range c.CORSOrigins {
		if strings.TrimSpace(origin) == "*" {
			return fmt.Errorf("wildcard CORS is not allowed in production")
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
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
