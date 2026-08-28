package database

import (
	"time"
)

type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
}

type Workspace struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Connector struct {
	ID            string                 `json:"id"`
	WorkspaceID   string                 `json:"workspace_id"`
	Type          string                 `json:"type"` // "s3", "azure_blob", "supabase_storage", "confluence", "local"
	Name          string                 `json:"name"`
	Config        map[string]interface{} `json:"config"`
	IsActive      bool                   `json:"is_active"`
	SyncFrequency string                 `json:"sync_frequency"` // "realtime", "hourly", "daily", "manual"
	LastSyncedAt  *time.Time             `json:"last_synced_at,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type SyncJob struct {
	ID             string     `json:"id"`
	ConnectorID    string     `json:"connector_id"`
	TriggerType    string     `json:"trigger_type"` // "online_manual" or "offline_scheduled"
	Status         string     `json:"status"`       // "pending", "running", "completed", "failed"
	TotalDocs      int        `json:"total_docs"`
	ModifiedDocs   int        `json:"modified_docs"`
	EmbeddedChunks int        `json:"embedded_chunks"`
	SkippedChunks  int        `json:"skipped_chunks"`
	ErrorLog       string     `json:"error_log,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}
