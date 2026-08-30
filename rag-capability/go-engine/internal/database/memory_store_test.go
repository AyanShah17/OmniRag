package database

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStoreReturnsDefensiveConnectorCopies(t *testing.T) {
	store := NewMemoryStore()
	connector := &Connector{ID: "connector-1", WorkspaceID: "ws-1", Config: map[string]interface{}{"token": "secret"}}
	if err := store.CreateConnector(context.Background(), connector); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.GetConnector(context.Background(), connector.ID)
	if err != nil {
		t.Fatal(err)
	}
	loaded.Config["token"] = "mutated"

	reloaded, err := store.GetConnector(context.Background(), connector.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Config["token"] != "secret" {
		t.Fatal("store state was mutated through a returned pointer")
	}
}

func TestMemoryStoreCompletesSyncAtomically(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	connector := &Connector{ID: "connector-1", WorkspaceID: "ws-1", Config: map[string]interface{}{}}
	job := &SyncJob{ID: "job-1", ConnectorID: connector.ID, Status: "running"}
	if err := store.CreateConnector(ctx, connector); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateSyncJob(ctx, job); err != nil {
		t.Fatal(err)
	}

	completedAt := time.Now().UTC()
	job.Status = "completed"
	job.CompletedAt = &completedAt
	if err := store.CompleteSync(ctx, connector.ID, completedAt, job); err != nil {
		t.Fatal(err)
	}

	storedJob, _ := store.GetSyncJob(ctx, job.ID)
	storedConnector, _ := store.GetConnector(ctx, connector.ID)
	if storedJob.Status != "completed" || storedConnector.LastSyncedAt == nil {
		t.Fatal("sync job and connector were not completed together")
	}
}
