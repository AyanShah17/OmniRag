package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/omnirag/go-engine/internal/database"
	"github.com/redis/go-redis/v9"
)

const (
	EmbeddingQueueKey = "rag:embedding:jobs"
	SyncJobQueueKey   = "rag:sync:jobs"
)

type EmbeddingJobPayload struct {
	JobID        string                 `json:"job_id"`
	TenantID     string                 `json:"tenant_id"`
	WorkspaceID  string                 `json:"workspace_id"`
	DocumentID   string                 `json:"document_id"`
	VersionID    string                 `json:"version_id"`
	Namespace    string                 `json:"namespace"`
	FileName     string                 `json:"file_name"`
	SourceURI    string                 `json:"source_uri"`
	FileType     string                 `json:"file_type"`
	ACLRoles     []string               `json:"acl_roles"`
	Chunks       []*database.Chunk      `json:"chunks"` // Only new/modified chunks!
	Metadata     map[string]interface{} `json:"metadata"`
	EnqueuedAt   time.Time              `json:"enqueued_at"`
}

type Producer interface {
	EnqueueEmbeddingJob(ctx context.Context, job *EmbeddingJobPayload) error
}

type HybridProducer struct {
	redisClient      *redis.Client
	pythonRAGURL     string
	useInMemoryQueue bool
	httpClient       *http.Client
}

func NewProducer(redisURL string, pythonRAGURL string, useInMemory bool) *HybridProducer {
	var rdb *redis.Client
	if !useInMemory && redisURL != "" {
		opt, err := redis.ParseURL(redisURL)
		if err == nil {
			rdb = redis.NewClient(opt)
		}
	}

	return &HybridProducer{
		redisClient:      rdb,
		pythonRAGURL:     pythonRAGURL,
		useInMemoryQueue: useInMemory,
		httpClient:       &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *HybridProducer) EnqueueEmbeddingJob(ctx context.Context, job *EmbeddingJobPayload) error {
	if len(job.Chunks) == 0 {
		// Nothing to embed! (All chunks were reused from previous versions)
		log.Printf("[Queue] Job %s has 0 new chunks to embed (all reused)", job.JobID)
		return nil
	}

	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to marshal job payload: %w", err)
	}

	// 1. Try pushing to Redis queue if client is active
	if p.redisClient != nil {
		err := p.redisClient.LPush(ctx, EmbeddingQueueKey, data).Err()
		if err == nil {
			log.Printf("[Queue] Pushed embedding job %s with %d chunks to Redis", job.JobID, len(job.Chunks))
			return nil
		}
		log.Printf("[Queue] Redis push failed, falling back to direct HTTP dispatch: %v", err)
	}

	// 2. Direct HTTP dispatch to Python RAG Core
	if p.pythonRAGURL != "" {
		endpoint := fmt.Sprintf("%s/api/v1/internal/embed-chunks", p.pythonRAGURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(data))
		if err != nil {
			return fmt.Errorf("failed to create http request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := p.httpClient.Do(req)
		if err != nil {
			log.Printf("[Queue] Direct dispatch to %s failed (Python service might be starting up): %v", endpoint, err)
			return nil // Don't crash ingestion if Python worker isn't up yet
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("[Queue] Successfully dispatched %d chunks to Python RAG Core via HTTP", len(job.Chunks))
			return nil
		}
		log.Printf("[Queue] Python RAG Core responded with status %d", resp.StatusCode)
	}

	return nil
}
