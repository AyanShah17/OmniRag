package api

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/omnirag/go-engine/internal/middleware"
)

func NewRouter(handler *APIHandler, authCfg middleware.AuthConfig, allowedOrigins []string) http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("/healthz", handler.HealthCheck)
	mux.HandleFunc("/api/v1/healthz", handler.HealthCheck)

	// Connectors
	mux.HandleFunc("/api/v1/connectors", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handler.ListConnectors(w, r)
		case http.MethodPost:
			handler.CreateConnector(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/connectors/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler.TestConnector(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Connector specific operations (e.g. /api/v1/connectors/{id}/sync)
	mux.HandleFunc("/api/v1/connectors/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/connectors/")
		parts := strings.Split(path, "/")
		if len(parts) == 2 && parts[1] == "sync" && r.Method == http.MethodPost {
			handler.TriggerSync(w, r, parts[0])
			return
		}
		http.Error(w, "Not found", http.StatusNotFound)
	})

	// Wrap with Middleware: Logging, CORS & (strict-in-production) Clerk Auth.
	// Auth runs innermost-but-one so every handler sees a resolved identity in
	// the request context before CORS/logging finish wrapping the response.
	authed := middleware.ClerkAuthMiddleware(authCfg)(mux)
	return securityHeadersMiddleware(corsMiddleware(loggingMiddleware(authed), allowedOrigins))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[HTTP] %s %s took %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func corsMiddleware(next http.Handler, allowedOrigins []string) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		allowed[strings.TrimSpace(origin)] = struct{}{}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if _, ok := allowed[origin]; ok && origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Workspace-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
