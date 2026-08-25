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
	"github.com/omnirag/go-engine/internal/diff"
	"github.com/omnirag/go-engine/internal/queue"
	"github.com/omnirag/go-engine/internal/scheduler"
)

func main() {
	// Load .env if present
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	cfg := config.LoadConfig()

	log.Println("=========================================================")
	log.Println("  OmniRAG Go Connector & Crawling Engine v1.0.0          ")
	log.Println("=========================================================")

	// 1. Initialize DB Store (Postgres / Supabase / NeonDB with fallback to MemoryStore)
	var store database.Store
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL != "" {
		pgStore, err := database.NewPostgresStore(databaseURL)
		if err != nil {
			log.Printf("[DB] Warning: Failed to connect to PostgreSQL (%v). Falling back to MemoryStore.", err)
			store = database.NewMemoryStore()
		} else {
			log.Println("[DB] Connected to PostgreSQL Store (Supabase / NeonDB)")
			store = pgStore
		}
	} else {
		log.Println("[DB] Using In-Memory Store (Set DATABASE_URL for persistent Supabase/NeonDB)")
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

	// 2. Initialize Chunk Differ & Queue Producer
	differ := diff.NewDiffer(store)
	producer := queue.NewProducer(cfg.RedisURL, cfg.PythonRAGURL, cfg.UseInMemoryQueue)

	// 3. Initialize Ingestion Orchestrator & Offline Background Scheduler
	orchestrator := scheduler.NewSyncOrchestrator(store, differ, producer, cfg.DefaultSyncPeriod)
	orchestrator.StartScheduler()
	defer orchestrator.Stop()

	// 4. Initialize HTTP API & Router
	handler := api.NewAPIHandler(store, orchestrator, differ)
	router := api.NewRouter(handler)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
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
