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
		return nil, fmt.Errorf("open postgres database: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping postgres database: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) CreateConnector(ctx context.Context, connector *Connector) error {
	configJSON, err := json.Marshal(connector.Config)
	if err != nil {
		return fmt.Errorf("encode connector config: %w", err)
	}
	now := time.Now().UTC()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO connectors (id, workspace_id, name, type, config, is_active, sync_frequency, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, config = EXCLUDED.config,
			is_active = EXCLUDED.is_active, sync_frequency = EXCLUDED.sync_frequency,
			updated_at = EXCLUDED.updated_at`,
		connector.ID, connector.WorkspaceID, connector.Name, connector.Type, string(configJSON),
		connector.IsActive, connector.SyncFrequency, now, now)
	return err
}

const connectorColumns = `id, workspace_id, name, type, config, is_active, sync_frequency, last_synced_at, created_at, updated_at`

func scanConnector(scanner interface{ Scan(...interface{}) error }) (*Connector, error) {
	var connector Connector
	var configJSON []byte
	var lastSynced sql.NullTime
	err := scanner.Scan(&connector.ID, &connector.WorkspaceID, &connector.Name, &connector.Type,
		&configJSON, &connector.IsActive, &connector.SyncFrequency, &lastSynced,
		&connector.CreatedAt, &connector.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(configJSON, &connector.Config); err != nil {
		return nil, fmt.Errorf("decode connector config: %w", err)
	}
	if lastSynced.Valid {
		connector.LastSyncedAt = &lastSynced.Time
	}
	return &connector, nil
}

func (s *PostgresStore) GetConnector(ctx context.Context, id string) (*Connector, error) {
	connector, err := scanConnector(s.db.QueryRowContext(ctx, `SELECT `+connectorColumns+` FROM connectors WHERE id = $1`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return connector, err
}

func (s *PostgresStore) listConnectors(ctx context.Context, query string, args ...interface{}) ([]*Connector, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var connectors []*Connector
	for rows.Next() {
		connector, err := scanConnector(rows)
		if err != nil {
			return nil, err
		}
		connectors = append(connectors, connector)
	}
	return connectors, rows.Err()
}

func (s *PostgresStore) ListConnectors(ctx context.Context, workspaceID string) ([]*Connector, error) {
	if workspaceID == "" {
		return s.listConnectors(ctx, `SELECT `+connectorColumns+` FROM connectors ORDER BY created_at DESC`)
	}
	return s.listConnectors(ctx, `SELECT `+connectorColumns+` FROM connectors WHERE workspace_id = $1 ORDER BY created_at DESC`, workspaceID)
}

func (s *PostgresStore) ListActiveConnectors(ctx context.Context) ([]*Connector, error) {
	return s.listConnectors(ctx, `SELECT `+connectorColumns+` FROM connectors WHERE is_active = true ORDER BY created_at ASC`)
}

func (s *PostgresStore) UpdateConnectorSyncTime(ctx context.Context, id string, syncTime time.Time) error {
	_, err := s.db.ExecContext(ctx, `UPDATE connectors SET last_synced_at = $1, updated_at = $1 WHERE id = $2`, syncTime.UTC(), id)
	return err
}

func (s *PostgresStore) CreateSyncJob(ctx context.Context, job *SyncJob) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO sync_jobs (id, connector_id, trigger_type, status, total_docs, modified_docs,
			embedded_chunks, skipped_chunks, error_log, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		job.ID, job.ConnectorID, job.TriggerType, job.Status, job.TotalDocs, job.ModifiedDocs,
		job.EmbeddedChunks, job.SkippedChunks, job.ErrorLog, job.StartedAt)
	return err
}

func (s *PostgresStore) UpdateSyncJob(ctx context.Context, job *SyncJob) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sync_jobs SET status = $1, total_docs = $2, modified_docs = $3,
			embedded_chunks = $4, skipped_chunks = $5, error_log = $6, completed_at = $7
		WHERE id = $8`, job.Status, job.TotalDocs, job.ModifiedDocs, job.EmbeddedChunks,
		job.SkippedChunks, job.ErrorLog, job.CompletedAt, job.ID)
	return err
}

func (s *PostgresStore) CompleteSync(ctx context.Context, connectorID string, syncTime time.Time, job *SyncJob) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return fmt.Errorf("begin sync completion transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
		UPDATE sync_jobs SET status = $1, total_docs = $2, modified_docs = $3,
			embedded_chunks = $4, skipped_chunks = $5, error_log = $6, completed_at = $7
		WHERE id = $8`, job.Status, job.TotalDocs, job.ModifiedDocs, job.EmbeddedChunks,
		job.SkippedChunks, job.ErrorLog, job.CompletedAt, job.ID)
	if err != nil {
		return fmt.Errorf("update sync job: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return fmt.Errorf("sync job %s was not updated", job.ID)
	}

	result, err = tx.ExecContext(ctx,
		`UPDATE connectors SET last_synced_at = $1, updated_at = $1 WHERE id = $2`,
		syncTime.UTC(), connectorID,
	)
	if err != nil {
		return fmt.Errorf("update connector sync time: %w", err)
	}
	if count, err := result.RowsAffected(); err != nil || count != 1 {
		return fmt.Errorf("connector %s was not updated", connectorID)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sync completion: %w", err)
	}
	return nil
}

func (s *PostgresStore) GetSyncJob(ctx context.Context, id string) (*SyncJob, error) {
	var job SyncJob
	var completed sql.NullTime
	var errorLog sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT id, connector_id, trigger_type, status, total_docs,
		modified_docs, embedded_chunks, skipped_chunks, error_log, started_at, completed_at
		FROM sync_jobs WHERE id = $1`, id).Scan(&job.ID, &job.ConnectorID, &job.TriggerType,
		&job.Status, &job.TotalDocs, &job.ModifiedDocs, &job.EmbeddedChunks, &job.SkippedChunks,
		&errorLog, &job.StartedAt, &completed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	job.ErrorLog = errorLog.String
	if completed.Valid {
		job.CompletedAt = &completed.Time
	}
	return &job, nil
}

func (s *PostgresStore) CreateTenant(ctx context.Context, tenant *Tenant) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO tenants (id, name, plan, created_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, plan = EXCLUDED.plan`,
		tenant.ID, tenant.Name, tenant.Plan, time.Now().UTC())
	return err
}

func (s *PostgresStore) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	var tenant Tenant
	err := s.db.QueryRowContext(ctx, `SELECT id, name, plan, created_at FROM tenants WHERE id = $1`, id).
		Scan(&tenant.ID, &tenant.Name, &tenant.Plan, &tenant.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &tenant, err
}

func (s *PostgresStore) CreateWorkspace(ctx context.Context, workspace *Workspace) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO workspaces (id, tenant_id, name, created_at) VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name`,
		workspace.ID, workspace.TenantID, workspace.Name, time.Now().UTC())
	return err
}

func (s *PostgresStore) GetWorkspace(ctx context.Context, id string) (*Workspace, error) {
	var workspace Workspace
	err := s.db.QueryRowContext(ctx, `SELECT id, tenant_id, name, created_at FROM workspaces WHERE id = $1`, id).
		Scan(&workspace.ID, &workspace.TenantID, &workspace.Name, &workspace.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &workspace, err
}
