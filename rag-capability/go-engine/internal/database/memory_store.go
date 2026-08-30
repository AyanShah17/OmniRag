package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type BootstrapStore interface {
	CreateTenant(context.Context, *Tenant) error
	CreateWorkspace(context.Context, *Workspace) error
}

type ConnectorStore interface {
	GetWorkspace(context.Context, string) (*Workspace, error)
	CreateConnector(context.Context, *Connector) error
	GetConnector(context.Context, string) (*Connector, error)
	ListConnectors(context.Context, string) ([]*Connector, error)
}

type SyncStore interface {
	GetConnector(context.Context, string) (*Connector, error)
	ListActiveConnectors(context.Context) ([]*Connector, error)
	CreateSyncJob(context.Context, *SyncJob) error
	UpdateSyncJob(context.Context, *SyncJob) error
	CompleteSync(context.Context, string, time.Time, *SyncJob) error
}

type Store interface {
	BootstrapStore
	ConnectorStore
	SyncStore
	GetTenant(context.Context, string) (*Tenant, error)
	GetSyncJob(context.Context, string) (*SyncJob, error)
}

func cloneConnector(connector *Connector) *Connector {
	if connector == nil {
		return nil
	}
	clone := *connector
	clone.Config = make(map[string]interface{}, len(connector.Config))
	for key, value := range connector.Config {
		clone.Config[key] = value
	}
	return &clone
}

func cloneSyncJob(job *SyncJob) *SyncJob {
	if job == nil {
		return nil
	}
	clone := *job
	return &clone
}

type MemoryStore struct {
	mu         sync.RWMutex
	tenants    map[string]*Tenant
	workspaces map[string]*Workspace
	connectors map[string]*Connector
	syncJobs   map[string]*SyncJob
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tenants:    make(map[string]*Tenant),
		workspaces: make(map[string]*Workspace),
		connectors: make(map[string]*Connector),
		syncJobs:   make(map[string]*SyncJob),
	}
}

func (m *MemoryStore) CreateTenant(_ context.Context, tenant *Tenant) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if tenant.ID == "" {
		tenant.ID = uuid.New().String()
	}
	if tenant.CreatedAt.IsZero() {
		tenant.CreatedAt = time.Now().UTC()
	}
	m.tenants[tenant.ID] = tenant
	return nil
}

func (m *MemoryStore) GetTenant(_ context.Context, id string) (*Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tenant, ok := m.tenants[id]
	if !ok {
		return nil, fmt.Errorf("tenant not found: %s", id)
	}
	return tenant, nil
}

func (m *MemoryStore) CreateWorkspace(_ context.Context, workspace *Workspace) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if workspace.ID == "" {
		workspace.ID = uuid.New().String()
	}
	if workspace.CreatedAt.IsZero() {
		workspace.CreatedAt = time.Now().UTC()
	}
	m.workspaces[workspace.ID] = workspace
	return nil
}

func (m *MemoryStore) GetWorkspace(_ context.Context, id string) (*Workspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	workspace, ok := m.workspaces[id]
	if !ok {
		return nil, fmt.Errorf("workspace not found: %s", id)
	}
	return workspace, nil
}

func (m *MemoryStore) CreateConnector(_ context.Context, connector *Connector) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if connector.ID == "" {
		connector.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	if connector.CreatedAt.IsZero() {
		connector.CreatedAt = now
	}
	connector.UpdatedAt = now
	m.connectors[connector.ID] = cloneConnector(connector)
	return nil
}

func (m *MemoryStore) GetConnector(_ context.Context, id string) (*Connector, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	connector, ok := m.connectors[id]
	if !ok {
		return nil, fmt.Errorf("connector not found: %s", id)
	}
	return cloneConnector(connector), nil
}

func (m *MemoryStore) ListConnectors(_ context.Context, workspaceID string) ([]*Connector, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var connectors []*Connector
	for _, connector := range m.connectors {
		if workspaceID == "" || connector.WorkspaceID == workspaceID {
			connectors = append(connectors, cloneConnector(connector))
		}
	}
	return connectors, nil
}

func (m *MemoryStore) ListActiveConnectors(_ context.Context) ([]*Connector, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var connectors []*Connector
	for _, connector := range m.connectors {
		if connector.IsActive {
			connectors = append(connectors, cloneConnector(connector))
		}
	}
	return connectors, nil
}

func (m *MemoryStore) UpdateConnectorSyncTime(_ context.Context, id string, syncTime time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	connector, ok := m.connectors[id]
	if !ok {
		return fmt.Errorf("connector not found: %s", id)
	}
	connector.LastSyncedAt = &syncTime
	connector.UpdatedAt = syncTime
	return nil
}

func (m *MemoryStore) CreateSyncJob(_ context.Context, job *SyncJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	if job.StartedAt.IsZero() {
		job.StartedAt = time.Now().UTC()
	}
	m.syncJobs[job.ID] = cloneSyncJob(job)
	return nil
}

func (m *MemoryStore) UpdateSyncJob(_ context.Context, job *SyncJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncJobs[job.ID] = cloneSyncJob(job)
	return nil
}

func (m *MemoryStore) GetSyncJob(_ context.Context, id string) (*SyncJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	job, ok := m.syncJobs[id]
	if !ok {
		return nil, fmt.Errorf("sync job not found: %s", id)
	}
	return cloneSyncJob(job), nil
}

func (m *MemoryStore) CompleteSync(_ context.Context, connectorID string, syncTime time.Time, job *SyncJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	connector, ok := m.connectors[connectorID]
	if !ok {
		return fmt.Errorf("connector not found: %s", connectorID)
	}
	if _, ok := m.syncJobs[job.ID]; !ok {
		return fmt.Errorf("sync job not found: %s", job.ID)
	}
	connector.LastSyncedAt = &syncTime
	connector.UpdatedAt = syncTime
	m.syncJobs[job.ID] = cloneSyncJob(job)
	return nil
}
