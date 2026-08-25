package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(databaseURL string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	store := &PostgresStore{db: db}
	if err := store.autoMigrate(ctx); err != nil {
		return nil, fmt.Errorf("postgres auto-migration failed: %w", err)
	}

	return store, nil
}

func (s *PostgresStore) autoMigrate(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS tenants (
		id VARCHAR(36) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		plan VARCHAR(64) NOT NULL DEFAULT 'enterprise',
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS workspaces (
		id VARCHAR(64) PRIMARY KEY,
		tenant_id VARCHAR(36) NOT NULL,
		name VARCHAR(255) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS connectors (
		id VARCHAR(36) PRIMARY KEY,
		workspace_id VARCHAR(64) NOT NULL,
		name VARCHAR(255) NOT NULL,
		type VARCHAR(64) NOT NULL,
		config JSONB NOT NULL DEFAULT '{}'::jsonb,
		is_active BOOLEAN NOT NULL DEFAULT true,
		sync_frequency VARCHAR(32) NOT NULL DEFAULT 'hourly',
		last_synced_at TIMESTAMP WITH TIME ZONE,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS documents (
		id VARCHAR(36) PRIMARY KEY,
		workspace_id VARCHAR(64) NOT NULL,
		connector_id VARCHAR(36),
		external_id VARCHAR(512) NOT NULL,
		file_name VARCHAR(255) NOT NULL,
		file_type VARCHAR(64) NOT NULL,
		file_size BIGINT NOT NULL DEFAULT 0,
		current_version_id VARCHAR(36),
		status VARCHAR(32) NOT NULL DEFAULT 'pending',
		metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS document_versions (
		id VARCHAR(36) PRIMARY KEY,
		document_id VARCHAR(36) NOT NULL,
		version_number INT NOT NULL,
		file_hash VARCHAR(64) NOT NULL,
		total_chunks INT NOT NULL DEFAULT 0,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS chunks (
		id VARCHAR(36) PRIMARY KEY,
		document_id VARCHAR(36) NOT NULL,
		chunk_hash VARCHAR(64) NOT NULL,
		chunk_index INT NOT NULL,
		text_content TEXT NOT NULL,
		token_count INT NOT NULL DEFAULT 0,
		metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
		is_embedded BOOLEAN NOT NULL DEFAULT false,
		created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS version_chunks (
		version_id VARCHAR(36) NOT NULL,
		chunk_id VARCHAR(36) NOT NULL,
		chunk_order INT NOT NULL DEFAULT 0,
		PRIMARY KEY (version_id, chunk_id)
	);

	CREATE TABLE IF NOT EXISTS sync_jobs (
		id VARCHAR(36) PRIMARY KEY,
		connector_id VARCHAR(36) NOT NULL,
		trigger_type VARCHAR(64) NOT NULL,
		status VARCHAR(32) NOT NULL DEFAULT 'running',
		total_docs INT NOT NULL DEFAULT 0,
		modified_docs INT NOT NULL DEFAULT 0,
		embedded_chunks INT NOT NULL DEFAULT 0,
		skipped_chunks INT NOT NULL DEFAULT 0,
		error_log TEXT,
		started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
		completed_at TIMESTAMP WITH TIME ZONE
	);
	`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *PostgresStore) CreateConnector(ctx context.Context, conn *Connector) error {
	configJSON, _ := json.Marshal(conn.Config)
	query := `
		INSERT INTO connectors (id, workspace_id, name, type, config, is_active, sync_frequency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			config = EXCLUDED.config,
			is_active = EXCLUDED.is_active,
			sync_frequency = EXCLUDED.sync_frequency,
			updated_at = EXCLUDED.updated_at
	`
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, query, conn.ID, conn.WorkspaceID, conn.Name, conn.Type, configJSON, conn.IsActive, conn.SyncFrequency, now, now)
	return err
}

func (s *PostgresStore) GetConnector(ctx context.Context, id string) (*Connector, error) {
	query := `SELECT id, workspace_id, name, type, config, is_active, sync_frequency, last_synced_at, created_at, updated_at FROM connectors WHERE id = $1`
	row := s.db.QueryRowContext(ctx, query, id)

	var c Connector
	var configBytes []byte
	var lastSynced sql.NullTime

	if err := row.Scan(&c.ID, &c.WorkspaceID, &c.Name, &c.Type, &configBytes, &c.IsActive, &c.SyncFrequency, &lastSynced, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	_ = json.Unmarshal(configBytes, &c.Config)
	if lastSynced.Valid {
		c.LastSyncedAt = &lastSynced.Time
	}
	return &c, nil
}

func (s *PostgresStore) ListConnectors(ctx context.Context, workspaceID string) ([]*Connector, error) {
	query := `SELECT id, workspace_id, name, type, config, is_active, sync_frequency, last_synced_at, created_at, updated_at FROM connectors`
	var rows *sql.Rows
	var err error

	if workspaceID != "" {
		query += ` WHERE workspace_id = $1 ORDER BY created_at DESC`
		rows, err = s.db.QueryContext(ctx, query, workspaceID)
	} else {
		query += ` ORDER BY created_at DESC`
		rows, err = s.db.QueryContext(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Connector
	for rows.Next() {
		var c Connector
		var configBytes []byte
		var lastSynced sql.NullTime

		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.Name, &c.Type, &configBytes, &c.IsActive, &c.SyncFrequency, &lastSynced, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(configBytes, &c.Config)
		if lastSynced.Valid {
			c.LastSyncedAt = &lastSynced.Time
		}
		result = append(result, &c)
	}
	return result, nil
}

func (s *PostgresStore) ListActiveConnectors(ctx context.Context) ([]*Connector, error) {
	query := `SELECT id, workspace_id, name, type, config, is_active, sync_frequency, last_synced_at, created_at, updated_at FROM connectors WHERE is_active = true`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Connector
	for rows.Next() {
		var c Connector
		var configBytes []byte
		var lastSynced sql.NullTime

		if err := rows.Scan(&c.ID, &c.WorkspaceID, &c.Name, &c.Type, &configBytes, &c.IsActive, &c.SyncFrequency, &lastSynced, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(configBytes, &c.Config)
		if lastSynced.Valid {
			c.LastSyncedAt = &lastSynced.Time
		}
		result = append(result, &c)
	}
	return result, nil
}

func (s *PostgresStore) UpdateConnectorSyncTime(ctx context.Context, id string, syncTime time.Time) error {
	query := `UPDATE connectors SET last_synced_at = $1, updated_at = $1 WHERE id = $2`
	_, err := s.db.ExecContext(ctx, query, syncTime.UTC(), id)
	return err
}

func (s *PostgresStore) CreateDocument(ctx context.Context, doc *Document) error {
	metaJSON, _ := json.Marshal(doc.Metadata)
	query := `
		INSERT INTO documents (id, workspace_id, connector_id, external_id, file_name, file_type, file_size, current_version_id, status, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	now := time.Now().UTC()
	var connID *string
	if doc.ConnectorID != "" {
		connID = &doc.ConnectorID
	}
	var verID *string
	if doc.CurrentVersionID != "" {
		verID = &doc.CurrentVersionID
	}

	_, err := s.db.ExecContext(ctx, query, doc.ID, doc.WorkspaceID, connID, doc.ExternalID, doc.FileName, doc.FileType, doc.FileSize, verID, doc.Status, metaJSON, now, now)
	return err
}

func (s *PostgresStore) GetDocument(ctx context.Context, id string) (*Document, error) {
	query := `SELECT id, workspace_id, connector_id, external_id, file_name, file_type, file_size, current_version_id, status, metadata, created_at, updated_at FROM documents WHERE id = $1`
	row := s.db.QueryRowContext(ctx, query, id)

	var d Document
	var connID, curVer sql.NullString
	var metaBytes []byte

	if err := row.Scan(&d.ID, &d.WorkspaceID, &connID, &d.ExternalID, &d.FileName, &d.FileType, &d.FileSize, &curVer, &d.Status, &metaBytes, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	d.ConnectorID = connID.String
	d.CurrentVersionID = curVer.String
	_ = json.Unmarshal(metaBytes, &d.Metadata)
	return &d, nil
}

func (s *PostgresStore) GetDocumentByExternalID(ctx context.Context, workspaceID, connectorID, externalID string) (*Document, error) {
	query := `SELECT id, workspace_id, connector_id, external_id, file_name, file_type, file_size, current_version_id, status, metadata, created_at, updated_at FROM documents WHERE workspace_id = $1 AND external_id = $2 LIMIT 1`
	row := s.db.QueryRowContext(ctx, query, workspaceID, externalID)

	var d Document
	var connID, curVer sql.NullString
	var metaBytes []byte

	if err := row.Scan(&d.ID, &d.WorkspaceID, &connID, &d.ExternalID, &d.FileName, &d.FileType, &d.FileSize, &curVer, &d.Status, &metaBytes, &d.CreatedAt, &d.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	d.ConnectorID = connID.String
	d.CurrentVersionID = curVer.String
	_ = json.Unmarshal(metaBytes, &d.Metadata)
	return &d, nil
}

func (s *PostgresStore) UpdateDocument(ctx context.Context, doc *Document) error {
	metaJSON, _ := json.Marshal(doc.Metadata)
	query := `
		UPDATE documents SET
			file_size = $1,
			current_version_id = $2,
			status = $3,
			metadata = $4,
			updated_at = $5
		WHERE id = $6
	`
	now := time.Now().UTC()
	var curVer *string
	if doc.CurrentVersionID != "" {
		curVer = &doc.CurrentVersionID
	}
	_, err := s.db.ExecContext(ctx, query, doc.FileSize, curVer, doc.Status, metaJSON, now, doc.ID)
	return err
}

func (s *PostgresStore) CreateDocumentVersion(ctx context.Context, ver *DocumentVersion) error {
	query := `
		INSERT INTO document_versions (id, document_id, version_number, file_hash, total_chunks, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, query, ver.ID, ver.DocumentID, ver.VersionNumber, ver.FileHash, ver.TotalChunks, now)
	return err
}

func (s *PostgresStore) GetLatestDocumentVersion(ctx context.Context, documentID string) (*DocumentVersion, error) {
	query := `SELECT id, document_id, version_number, file_hash, total_chunks, created_at FROM document_versions WHERE document_id = $1 ORDER BY version_number DESC LIMIT 1`
	row := s.db.QueryRowContext(ctx, query, documentID)

	var v DocumentVersion
	if err := row.Scan(&v.ID, &v.DocumentID, &v.VersionNumber, &v.FileHash, &v.TotalChunks, &v.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

func (s *PostgresStore) GetExistingChunksForDoc(ctx context.Context, documentID string) (map[string]*Chunk, error) {
	query := `SELECT id, document_id, chunk_hash, chunk_index, text_content, token_count, metadata, is_embedded, created_at FROM chunks WHERE document_id = $1`
	rows, err := s.db.QueryContext(ctx, query, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]*Chunk)
	for rows.Next() {
		var c Chunk
		var metaBytes []byte
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.ChunkHash, &c.ChunkIndex, &c.TextContent, &c.TokenCount, &metaBytes, &c.IsEmbedded, &c.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metaBytes, &c.Metadata)
		result[c.ChunkHash] = &c
	}
	return result, nil
}

func (s *PostgresStore) UpsertChunk(ctx context.Context, chunk *Chunk) error {
	metaJSON, _ := json.Marshal(chunk.Metadata)
	query := `
		INSERT INTO chunks (id, document_id, chunk_hash, chunk_index, text_content, token_count, metadata, is_embedded, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO NOTHING
	`
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, query, chunk.ID, chunk.DocumentID, chunk.ChunkHash, chunk.ChunkIndex, chunk.TextContent, chunk.TokenCount, metaJSON, chunk.IsEmbedded, now)
	return err
}

func (s *PostgresStore) LinkVersionChunks(ctx context.Context, versionID string, chunkIDs []string) error {
	if len(chunkIDs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `INSERT INTO version_chunks (version_id, chunk_id, chunk_order) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for idx, cid := range chunkIDs {
		if _, err := stmt.ExecContext(ctx, versionID, cid, idx); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresStore) CreateSyncJob(ctx context.Context, job *SyncJob) error {
	query := `
		INSERT INTO sync_jobs (id, connector_id, trigger_type, status, total_docs, modified_docs, embedded_chunks, skipped_chunks, error_log, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, query, job.ID, job.ConnectorID, job.TriggerType, job.Status, job.TotalDocs, job.ModifiedDocs, job.EmbeddedChunks, job.SkippedChunks, job.ErrorLog, now)
	return err
}

func (s *PostgresStore) UpdateSyncJob(ctx context.Context, job *SyncJob) error {
	query := `
		UPDATE sync_jobs SET
			status = $1,
			total_docs = $2,
			modified_docs = $3,
			embedded_chunks = $4,
			skipped_chunks = $5,
			error_log = $6,
			completed_at = $7
		WHERE id = $8
	`
	_, err := s.db.ExecContext(ctx, query, job.Status, job.TotalDocs, job.ModifiedDocs, job.EmbeddedChunks, job.SkippedChunks, job.ErrorLog, job.CompletedAt, job.ID)
	return err
}

func (s *PostgresStore) CreateTenant(ctx context.Context, tenant *Tenant) error {
	query := `INSERT INTO tenants (id, name, plan, created_at) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, plan = EXCLUDED.plan`
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, query, tenant.ID, tenant.Name, tenant.Plan, now)
	return err
}

func (s *PostgresStore) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	query := `SELECT id, name, plan, created_at FROM tenants WHERE id = $1`
	row := s.db.QueryRowContext(ctx, query, id)
	var t Tenant
	if err := row.Scan(&t.ID, &t.Name, &t.Plan, &t.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &t, nil
}

func (s *PostgresStore) CreateWorkspace(ctx context.Context, ws *Workspace) error {
	query := `INSERT INTO workspaces (id, tenant_id, name, created_at) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, query, ws.ID, ws.TenantID, ws.Name, now)
	return err
}

func (s *PostgresStore) GetWorkspace(ctx context.Context, id string) (*Workspace, error) {
	query := `SELECT id, tenant_id, name, created_at FROM workspaces WHERE id = $1`
	row := s.db.QueryRowContext(ctx, query, id)
	var w Workspace
	if err := row.Scan(&w.ID, &w.TenantID, &w.Name, &w.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &w, nil
}

func (s *PostgresStore) GetSyncJob(ctx context.Context, id string) (*SyncJob, error) {
	query := `SELECT id, connector_id, trigger_type, status, total_docs, modified_docs, embedded_chunks, skipped_chunks, error_log, started_at, completed_at FROM sync_jobs WHERE id = $1`
	row := s.db.QueryRowContext(ctx, query, id)
	var j SyncJob
	var completed sql.NullTime
	var errLog sql.NullString
	if err := row.Scan(&j.ID, &j.ConnectorID, &j.TriggerType, &j.Status, &j.TotalDocs, &j.ModifiedDocs, &j.EmbeddedChunks, &j.SkippedChunks, &errLog, &j.StartedAt, &completed); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	j.ErrorLog = errLog.String
	if completed.Valid {
		j.CompletedAt = &completed.Time
	}
	return &j, nil
}

func (s *PostgresStore) ListDocuments(ctx context.Context, workspaceID string) ([]*Document, error) {
	query := `SELECT id, workspace_id, connector_id, external_id, file_name, file_type, file_size, current_version_id, status, metadata, created_at, updated_at FROM documents WHERE workspace_id = $1 ORDER BY created_at DESC`
	rows, err := s.db.QueryContext(ctx, query, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Document
	for rows.Next() {
		var d Document
		var connID, curVer sql.NullString
		var metaBytes []byte
		if err := rows.Scan(&d.ID, &d.WorkspaceID, &connID, &d.ExternalID, &d.FileName, &d.FileType, &d.FileSize, &curVer, &d.Status, &metaBytes, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		d.ConnectorID = connID.String
		d.CurrentVersionID = curVer.String
		_ = json.Unmarshal(metaBytes, &d.Metadata)
		result = append(result, &d)
	}
	return result, nil
}

func (s *PostgresStore) GetChunksForVersion(ctx context.Context, versionID string) ([]*Chunk, error) {
	query := `
		SELECT c.id, c.document_id, c.chunk_hash, c.chunk_index, c.text_content, c.token_count, c.metadata, c.is_embedded, c.created_at
		FROM chunks c
		INNER JOIN version_chunks vc ON c.id = vc.chunk_id
		WHERE vc.version_id = $1
		ORDER BY vc.chunk_order ASC
	`
	rows, err := s.db.QueryContext(ctx, query, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*Chunk
	for rows.Next() {
		var c Chunk
		var metaBytes []byte
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.ChunkHash, &c.ChunkIndex, &c.TextContent, &c.TokenCount, &metaBytes, &c.IsEmbedded, &c.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metaBytes, &c.Metadata)
		result = append(result, &c)
	}
	return result, nil
}
