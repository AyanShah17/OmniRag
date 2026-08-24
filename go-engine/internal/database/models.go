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
}

type SyncJob struct {
	ID             string     `json:"id"`
	ConnectorID    string     `json:"connector_id"`
	TriggerType    string     `json:"trigger_type"` // "online", "offline_scheduled", "webhook"
	Status         string     `json:"status"`       // "pending", "running", "completed", "failed"
	TotalDocs      int        `json:"total_docs"`
	ModifiedDocs   int        `json:"modified_docs"`
	EmbeddedChunks int        `json:"embedded_chunks"`
	SkippedChunks  int        `json:"skipped_chunks"`
	ErrorLog       string     `json:"error_log,omitempty"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

type Document struct {
	ID               string                 `json:"id"`
	WorkspaceID      string                 `json:"workspace_id"`
	ConnectorID      string                 `json:"connector_id"`
	ExternalID       string                 `json:"external_id"` // E.g., s3://bucket/path/to/file.pdf or confluence-page-123
	FileName         string                 `json:"file_name"`
	FileType         string                 `json:"file_type"`
	FileSize         int64                  `json:"file_size"`
	CurrentVersionID string                 `json:"current_version_id"`
	Status           string                 `json:"status"` // "pending", "syncing", "synced", "error"
	Metadata         map[string]interface{} `json:"metadata"`
	CreatedAt        time.Time              `json:"created_at"`
	UpdatedAt        time.Time              `json:"updated_at"`
}

type DocumentVersion struct {
	ID            string    `json:"id"`
	DocumentID    string    `json:"document_id"`
	VersionNumber int       `json:"version_number"`
	FileHash      string    `json:"file_hash"` // SHA-256
	TotalChunks   int       `json:"total_chunks"`
	CreatedAt     time.Time `json:"created_at"`
}

type Chunk struct {
	ID          string                 `json:"id"`
	DocumentID  string                 `json:"document_id"`
	ChunkHash   string                 `json:"chunk_hash"` // SHA-256 of text
	ChunkIndex  int                    `json:"chunk_index"`
	TextContent string                 `json:"text_content"`
	TokenCount  int                    `json:"token_count"`
	Metadata    map[string]interface{} `json:"metadata"`
	IsEmbedded  bool                   `json:"is_embedded"`
	CreatedAt   time.Time              `json:"created_at"`
}

type VersionChunk struct {
	VersionID  string `json:"version_id"`
	ChunkID    string `json:"chunk_id"`
	ChunkOrder int    `json:"chunk_order"`
}
