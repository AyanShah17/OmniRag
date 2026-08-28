package scheduler

import (
	"context"
	"fmt"
	"log"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/omnirag/go-engine/internal/connectors"
	"github.com/omnirag/go-engine/internal/database"
	"github.com/omnirag/go-engine/internal/ingestion"
	"github.com/omnirag/go-engine/internal/security"
)

type SyncOrchestrator struct {
	store    database.SyncStore
	ingester ingestion.Client
	interval time.Duration
	stopChan chan struct{}
	stopOnce sync.Once
}

func NewSyncOrchestrator(store database.SyncStore, ingester ingestion.Client, intervalSeconds int) *SyncOrchestrator {
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}
	return &SyncOrchestrator{
		store:    store,
		ingester: ingester,
		interval: time.Duration(intervalSeconds) * time.Second,
		stopChan: make(chan struct{}),
	}
}

// StartScheduler begins background offline periodic scanning
func (s *SyncOrchestrator) StartScheduler() {
	ticker := time.NewTicker(s.interval)
	go func() {
		log.Printf("[Scheduler] Background offline sync poller started (interval: %v)", s.interval)
		for {
			select {
			case <-ticker.C:
				s.runScheduledSyncs()
			case <-s.stopChan:
				ticker.Stop()
				log.Println("[Scheduler] Background sync poller stopped")
				return
			}
		}
	}()
}

func (s *SyncOrchestrator) Stop() {
	s.stopOnce.Do(func() { close(s.stopChan) })
}

func (s *SyncOrchestrator) runScheduledSyncs() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	activeConnectors, err := s.store.ListActiveConnectors(ctx)
	if err != nil {
		log.Printf("[Scheduler] Error listing active connectors: %v", err)
		return
	}

	for _, conn := range activeConnectors {
		if conn.SyncFrequency == "manual" {
			continue
		}
		log.Printf("[Scheduler] Running offline scheduled sync for connector %s (%s)", conn.Name, conn.Type)
		job, err := s.SyncConnector(ctx, conn.ID, "offline_scheduled")
		if err != nil {
			log.Printf("[Scheduler] Sync failed for connector %s: %v", conn.ID, err)
		} else {
			log.Printf("[Scheduler] Sync completed for connector %s: docs=%d, modified=%d, new_chunks=%d, skipped_chunks=%d",
				conn.Name, job.TotalDocs, job.ModifiedDocs, job.EmbeddedChunks, job.SkippedChunks)
		}
	}
}

// SyncConnector discovers connector objects and sends each one through Python's canonical ingestion pipeline.
func (s *SyncOrchestrator) SyncConnector(ctx context.Context, connectorID string, triggerType string) (*database.SyncJob, error) {
	conn, err := s.store.GetConnector(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("connector not found: %w", err)
	}
	if conn == nil {
		return nil, fmt.Errorf("connector not found")
	}

	connectorConfig, err := security.DecryptConfig(conn.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt connector credentials: %w", err)
	}
	adapter, err := connectors.CreateConnector(conn.Type, conn.Name, connectorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate connector adapter: %w", err)
	}

	jobID := uuid.New().String()
	syncJob := &database.SyncJob{
		ID:          jobID,
		ConnectorID: connectorID,
		TriggerType: triggerType,
		Status:      "running",
		StartedAt:   time.Now().UTC(),
	}

	if err := s.store.CreateSyncJob(ctx, syncJob); err != nil {
		return nil, fmt.Errorf("failed to create sync job: %w", err)
	}

	objects, err := adapter.ListObjects(ctx, "")
	if err != nil {
		syncJob.Status = "failed"
		syncJob.ErrorLog = err.Error()
		now := time.Now().UTC()
		syncJob.CompletedAt = &now
		if updateErr := s.store.UpdateSyncJob(ctx, syncJob); updateErr != nil {
			return syncJob, fmt.Errorf("failed to persist sync failure: %w", updateErr)
		}
		return syncJob, fmt.Errorf("failed to list objects: %w", err)
	}

	syncJob.TotalDocs = len(objects)
	var syncErrors []string

	for _, obj := range objects {
		data, _, err := adapter.FetchObject(ctx, obj.Key)
		if err != nil {
			log.Printf("[Sync] Error fetching object %s: %v", obj.Key, err)
			syncErrors = append(syncErrors, fmt.Sprintf("%s: fetch failed: %v", obj.Key, err))
			continue
		}

		result, err := s.ingester.IngestDocument(ctx, &ingestion.DocumentPayload{
			WorkspaceID: conn.WorkspaceID,
			ConnectorID: conn.ID,
			ExternalID:  fmt.Sprintf("%s/%s/%s", conn.Type, conn.ID, obj.Key),
			FileName:    path.Base(obj.Key),
			ContentType: obj.ContentType,
			Content:     data,
			ACLRoles:    []string{"default"},
			Metadata:    obj.Metadata,
		})
		if err != nil {
			log.Printf("[Sync] Python ingestion failed for object %s: %v", obj.Key, err)
			syncErrors = append(syncErrors, fmt.Sprintf("%s: ingestion failed: %v", obj.Key, err))
			continue
		}

		if result.Changed {
			syncJob.ModifiedDocs++
		}
		syncJob.SkippedChunks += result.ReusedChunksCount
		syncJob.EmbeddedChunks += result.NewChunksEmbedded
	}

	syncJob.Status = "completed"
	if len(syncErrors) > 0 {
		syncJob.Status = "failed"
		syncJob.ErrorLog = strings.Join(syncErrors, "\n")
	}
	now := time.Now().UTC()
	syncJob.CompletedAt = &now
	if err := s.store.CompleteSync(ctx, conn.ID, now, syncJob); err != nil {
		return syncJob, fmt.Errorf("failed to atomically complete sync: %w", err)
	}

	if len(syncErrors) > 0 {
		return syncJob, fmt.Errorf("sync completed with %d error(s)", len(syncErrors))
	}
	return syncJob, nil
}
