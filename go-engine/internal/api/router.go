package api

import (
	"log"
	"net/http"
	"strings"
	"time"
)

func NewRouter(handler *APIHandler) http.Handler {
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

	// Ingestion
	mux.HandleFunc("/api/v1/ingest/file", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler.DirectFileUpload(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Webhooks
	mux.HandleFunc("/api/v1/webhooks/s3", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler.S3Webhook(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/webhooks/confluence", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handler.ConfluenceWebhook(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Wrap with Middleware: Logging, Recovery & CORS
	return corsMiddleware(loggingMiddleware(mux))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("[HTTP] %s %s took %v", r.Method, r.URL.Path, time.Since(start))
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Workspace-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
