package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/omnirag/go-engine/internal/connectors"
	"github.com/omnirag/go-engine/internal/database"
	"github.com/omnirag/go-engine/internal/diff"
	"github.com/omnirag/go-engine/internal/queue"
)

type SyncOrchestrator struct {
	store    database.Store
	differ   *diff.Differ
	producer queue.Producer
	interval time.Duration
	stopChan chan struct{}
}

func NewSyncOrchestrator(store database.Store, differ *diff.Differ, producer queue.Producer, intervalSeconds int) *SyncOrchestrator {
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}
	return &SyncOrchestrator{
		store:    store,
		differ:   differ,
		producer: producer,
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
	close(s.stopChan)
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

// SyncConnector orchestrates end-to-end cloud scanning, incremental chunk diffing, and queue dispatch
func (s *SyncOrchestrator) SyncConnector(ctx context.Context, connectorID string, triggerType string) (*database.SyncJob, error) {
	conn, err := s.store.GetConnector(ctx, connectorID)
	if err != nil {
		return nil, fmt.Errorf("connector not found: %w", err)
	}

	adapter, err := connectors.CreateConnector(conn.Type, conn.Name, conn.Config)
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
		_ = s.store.UpdateSyncJob(ctx, syncJob)
		return syncJob, fmt.Errorf("failed to list objects: %w", err)
	}

	syncJob.TotalDocs = len(objects)

	for _, obj := range objects {
		doc, err := s.store.GetDocumentByExternalID(ctx, conn.WorkspaceID, conn.ID, obj.Key)
		if err != nil {
			log.Printf("[Sync] Error retrieving document %s: %v", obj.Key, err)
			continue
		}

		if doc == nil {
			doc = &database.Document{
				ID:          uuid.New().String(),
				WorkspaceID: conn.WorkspaceID,
				ConnectorID: conn.ID,
				ExternalID:  obj.Key,
				FileName:    extractFileName(obj.Key),
				FileType:    obj.ContentType,
				FileSize:    obj.Size,
				Status:      "pending",
				Metadata:    obj.Metadata,
			}
			if err := s.store.CreateDocument(ctx, doc); err != nil {
				log.Printf("[Sync] Error creating document %s: %v", obj.Key, err)
				continue
			}
		}

		// Download file content
		data, _, err := adapter.FetchObject(ctx, obj.Key)
		if err != nil {
			log.Printf("[Sync] Error fetching object %s: %v", obj.Key, err)
			continue
		}

		// Fast deterministic chunking on text files
		rawChunks := chunkDocumentText(string(data), 500, 50)

		diffResult, err := s.differ.ProcessDocumentDiff(ctx, doc, data, rawChunks)
		if err != nil {
			log.Printf("[Sync] Diffing failed for document %s: %v", doc.ID, err)
			continue
		}

		if diffResult.IsDocumentChanged {
			syncJob.ModifiedDocs++
		}
		syncJob.EmbeddedChunks += diffResult.NewChunksCount
		syncJob.SkippedChunks += diffResult.ReusedChunksCount

		// Enqueue embedding task if new chunks exist
		if len(diffResult.NewChunksToEmbed) > 0 {
			embJob := &queue.EmbeddingJobPayload{
				JobID:       uuid.New().String(),
				WorkspaceID: conn.WorkspaceID,
				DocumentID:  doc.ID,
				VersionID:   diffResult.VersionID,
				Namespace:   fmt.Sprintf("ws_%s", conn.WorkspaceID),
				FileName:    doc.FileName,
				SourceURI:   obj.Key,
				FileType:    doc.FileType,
				ACLRoles:    []string{"default"},
				Chunks:      diffResult.NewChunksToEmbed,
				Metadata:    obj.Metadata,
				EnqueuedAt:  time.Now().UTC(),
			}

			if err := s.producer.EnqueueEmbeddingJob(ctx, embJob); err != nil {
				log.Printf("[Sync] Error enqueueing embedding job: %v", err)
			}
		}
	}

	syncJob.Status = "completed"
	now := time.Now().UTC()
	syncJob.CompletedAt = &now
	_ = s.store.UpdateSyncJob(ctx, syncJob)
	_ = s.store.UpdateConnectorSyncTime(ctx, conn.ID, now)

	return syncJob, nil
}

// IngestRawFile handles direct online uploads
func (s *SyncOrchestrator) IngestRawFile(
	ctx context.Context,
	workspaceID string,
	connectorID string,
	fileName string,
	fileBytes []byte,
	metadata map[string]interface{},
) (*diff.DiffResult, error) {
	externalID := fmt.Sprintf("direct/%s", fileName)
	doc, err := s.store.GetDocumentByExternalID(ctx, workspaceID, connectorID, externalID)
	if err != nil {
		return nil, err
	}

	if doc == nil {
		doc = &database.Document{
			ID:          uuid.New().String(),
			WorkspaceID: workspaceID,
			ConnectorID: connectorID,
			ExternalID:  externalID,
			FileName:    fileName,
			FileType:    diff.ComputeFileHash(fileBytes),
			FileSize:    int64(len(fileBytes)),
			Status:      "syncing",
			Metadata:    metadata,
		}
		if err := s.store.CreateDocument(ctx, doc); err != nil {
			return nil, err
		}
	}

	rawChunks := chunkDocumentText(string(fileBytes), 500, 50)
	diffResult, err := s.differ.ProcessDocumentDiff(ctx, doc, fileBytes, rawChunks)
	if err != nil {
		return nil, err
	}

	if len(diffResult.NewChunksToEmbed) > 0 {
		embJob := &queue.EmbeddingJobPayload{
			JobID:       uuid.New().String(),
			WorkspaceID: workspaceID,
			DocumentID:  doc.ID,
			VersionID:   diffResult.VersionID,
			Namespace:   fmt.Sprintf("ws_%s", workspaceID),
			FileName:    fileName,
			SourceURI:   externalID,
			FileType:    "text/plain",
			ACLRoles:    []string{"default"},
			Chunks:      diffResult.NewChunksToEmbed,
			Metadata:    metadata,
			EnqueuedAt:  time.Now().UTC(),
		}
		_ = s.producer.EnqueueEmbeddingJob(ctx, embJob)
	}

	return diffResult, nil
}

func chunkDocumentText(text string, chunkSize int, chunkOverlap int) []diff.RawChunkInput {
	if chunkSize <= 0 {
		chunkSize = 500
	}
	if chunkOverlap < 0 {
		chunkOverlap = 50
	}

	// Normalize newlines
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var rawParts []string
	paragraphs := strings.Split(text, "\n\n")

	for _, p := range paragraphs {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		if len(trimmed) <= chunkSize {
			rawParts = append(rawParts, trimmed)
		} else {
			// Split by lines
			lines := strings.Split(trimmed, "\n")
			for _, line := range lines {
				lTrimmed := strings.TrimSpace(line)
				if lTrimmed == "" {
					continue
				}
				if len(lTrimmed) <= chunkSize {
					rawParts = append(rawParts, lTrimmed)
				} else {
					// Split by sentence
					sentences := strings.Split(lTrimmed, ". ")
					for _, s := range sentences {
						sTrimmed := strings.TrimSpace(s)
						if sTrimmed == "" {
							continue
						}
						// If still exceeding chunk size, hard-split
						for len(sTrimmed) > chunkSize {
							rawParts = append(rawParts, sTrimmed[:chunkSize])
							sTrimmed = strings.TrimSpace(sTrimmed[chunkSize:])
						}
						if sTrimmed != "" {
							rawParts = append(rawParts, sTrimmed)
						}
					}
				}
			}
		}
	}

	var chunks []diff.RawChunkInput
	var currentChunk strings.Builder
	chunkIndex := 0

	for _, part := range rawParts {
		if currentChunk.Len()+len(part)+2 > chunkSize && currentChunk.Len() > 0 {
			chunkStr := currentChunk.String()
			chunks = append(chunks, diff.RawChunkInput{
				TextContent: chunkStr,
				ChunkIndex:  chunkIndex,
				TokenCount:  len(strings.Fields(chunkStr)),
				Metadata: map[string]interface{}{
					"chunk_index": chunkIndex,
				},
			})
			chunkIndex++
			currentChunk.Reset()
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(part)
	}

	if currentChunk.Len() > 0 {
		chunkStr := currentChunk.String()
		chunks = append(chunks, diff.RawChunkInput{
			TextContent: chunkStr,
			ChunkIndex:  chunkIndex,
			TokenCount:  len(strings.Fields(chunkStr)),
			Metadata: map[string]interface{}{
				"chunk_index": chunkIndex,
			},
		})
	}

	return chunks
}

func extractFileName(key string) string {
	parts := strings.Split(key, "/")
	return parts[len(parts)-1]
}
