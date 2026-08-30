package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/omnirag/go-engine/internal/api"
	"github.com/omnirag/go-engine/internal/config"
	"github.com/omnirag/go-engine/internal/database"
	"github.com/omnirag/go-engine/internal/ingestion"
	"github.com/omnirag/go-engine/internal/middleware"
	"github.com/omnirag/go-engine/internal/scheduler"
)

func main() {
	// Load .env if present
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	cfg := config.LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("[Config] Invalid configuration: %v", err)
	}

	log.Println("=========================================================")
	log.Println("  OmniRAG Go Connector & Crawling Engine v1.0.0          ")
	log.Println("=========================================================")

	// 1. Initialize DB Store (Postgres / Supabase / NeonDB with fallback to MemoryStore)
	var store database.Store
	databaseURL := cfg.DatabaseURL

	if databaseURL != "" {
		pgStore, err := database.NewPostgresStore(databaseURL)
		if err != nil {
			if cfg.AuthMode == "production" {
				log.Fatalf("[DB] PostgreSQL is required in production: %v", err)
			}
			log.Printf("[DB] PostgreSQL unavailable (%v); using development MemoryStore", err)
			store = database.NewMemoryStore()
		} else {
			log.Println("[DB] Connected to PostgreSQL Store (Supabase / NeonDB)")
			store = pgStore
		}
	} else {
		if cfg.AuthMode == "production" {
			log.Fatal("[DB] GO_DATABASE_URL is required in production")
		}
		log.Println("[DB] Using development MemoryStore (set GO_DATABASE_URL for persistence)")
		store = database.NewMemoryStore()
	}

	// Seed default tenant & workspace if in-memory
	ctx := context.Background()
	_ = store.CreateTenant(ctx, &database.Tenant{
		ID:   "tenant_default",
		Name: "Enterprise Demo Tenant",
		Plan: "enterprise",
	})
	_ = store.CreateWorkspace(ctx, &database.Workspace{
		ID:       "ws_default",
		TenantID: "tenant_default",
		Name:     "Main Knowledge Base",
	})

	// 2. Initialize Python ingestion client
	ingestionClient := ingestion.NewPythonClient(cfg.PythonRAGURL, cfg.InternalServiceSecret)

	// 3. Initialize Ingestion Orchestrator & Offline Background Scheduler
	orchestrator := scheduler.NewSyncOrchestrator(store, ingestionClient, cfg.DefaultSyncPeriod)
	orchestrator.StartScheduler()
	defer orchestrator.Stop()

	// 4. Initialize HTTP API & Router
	handler := api.NewAPIHandler(store, orchestrator)
	authCfg := middleware.AuthConfig{
		Mode:    cfg.AuthMode,
		JWKSURL: cfg.ClerkJWKSURL,
		Issuer:  cfg.ClerkIssuer,
		ExemptPaths: []string{
			"/healthz",
			"/api/v1/healthz",
		},
	}
	router := api.NewRouter(handler, authCfg, cfg.CORSOrigins)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// 5. Graceful shutdown handler
	go func() {
		log.Printf("[Server] Go Connector Engine listening on http://localhost:%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[Server] Failed to listen: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("[Server] Shutting down Go Connector Engine gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Server] Forced shutdown: %v", err)
	}
	log.Println("[Server] Go Engine exited cleanly")
}
