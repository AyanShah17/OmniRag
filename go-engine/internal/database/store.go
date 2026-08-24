package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Store interface {
	// Tenants & Workspaces
	CreateTenant(ctx context.Context, tenant *Tenant) error
	GetTenant(ctx context.Context, id string) (*Tenant, error)
	CreateWorkspace(ctx context.Context, ws *Workspace) error
	GetWorkspace(ctx context.Context, id string) (*Workspace, error)

	// Connectors
	CreateConnector(ctx context.Context, conn *Connector) error
	GetConnector(ctx context.Context, id string) (*Connector, error)
	ListConnectors(ctx context.Context, workspaceID string) ([]*Connector, error)
	ListActiveConnectors(ctx context.Context) ([]*Connector, error)
	UpdateConnectorSyncTime(ctx context.Context, id string, t time.Time) error

	// Sync Jobs
	CreateSyncJob(ctx context.Context, job *SyncJob) error
	UpdateSyncJob(ctx context.Context, job *SyncJob) error
	GetSyncJob(ctx context.Context, id string) (*SyncJob, error)

	// Documents & Versions
	GetDocumentByExternalID(ctx context.Context, workspaceID, connectorID, externalID string) (*Document, error)
	CreateDocument(ctx context.Context, doc *Document) error
	UpdateDocument(ctx context.Context, doc *Document) error
	ListDocuments(ctx context.Context, workspaceID string) ([]*Document, error)

	CreateDocumentVersion(ctx context.Context, version *DocumentVersion) error
	GetLatestDocumentVersion(ctx context.Context, docID string) (*DocumentVersion, error)

	// Chunks & Diffing
	GetExistingChunksForDoc(ctx context.Context, docID string) (map[string]*Chunk, error) // Returns map[chunk_hash]*Chunk
	UpsertChunk(ctx context.Context, chunk *Chunk) error
	LinkVersionChunks(ctx context.Context, versionID string, chunkIDs []string) error
	GetChunksForVersion(ctx context.Context, versionID string) ([]*Chunk, error)
}

// MemoryStore provides a thread-safe in-memory store for local testing & high performance fallback
type MemoryStore struct {
	mu           sync.RWMutex
	tenants      map[string]*Tenant
	workspaces   map[string]*Workspace
	connectors   map[string]*Connector
	syncJobs     map[string]*SyncJob
	documents    map[string]*Document        // key: id
	docExtIndex  map[string]*Document        // key: workspaceID:connectorID:externalID
	docVersions  map[string]*DocumentVersion // key: id
	docVerLatest map[string]*DocumentVersion // key: docID
	chunks       map[string]*Chunk           // key: id
	docChunkMap  map[string]map[string]*Chunk // key: docID -> map[chunk_hash]*Chunk
	verChunks    map[string][]string         // key: versionID -> []chunkID
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tenants:      make(map[string]*Tenant),
		workspaces:   make(map[string]*Workspace),
		connectors:   make(map[string]*Connector),
		syncJobs:     make(map[string]*SyncJob),
		documents:    make(map[string]*Document),
		docExtIndex:  make(map[string]*Document),
		docVersions:  make(map[string]*DocumentVersion),
		docVerLatest: make(map[string]*DocumentVersion),
		chunks:       make(map[string]*Chunk),
		docChunkMap:  make(map[string]map[string]*Chunk),
		verChunks:    make(map[string][]string),
	}
}

func (m *MemoryStore) CreateTenant(ctx context.Context, tenant *Tenant) error {
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

func (m *MemoryStore) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.tenants[id]
	if !ok {
		return nil, fmt.Errorf("tenant not found: %s", id)
	}
	return t, nil
}

func (m *MemoryStore) CreateWorkspace(ctx context.Context, ws *Workspace) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ws.ID == "" {
		ws.ID = uuid.New().String()
	}
	if ws.CreatedAt.IsZero() {
		ws.CreatedAt = time.Now().UTC()
	}
	m.workspaces[ws.ID] = ws
	return nil
}

func (m *MemoryStore) GetWorkspace(ctx context.Context, id string) (*Workspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.workspaces[id]
	if !ok {
		return nil, fmt.Errorf("workspace not found: %s", id)
	}
	return w, nil
}

func (m *MemoryStore) CreateConnector(ctx context.Context, conn *Connector) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if conn.ID == "" {
		conn.ID = uuid.New().String()
	}
	if conn.CreatedAt.IsZero() {
		conn.CreatedAt = time.Now().UTC()
	}
	m.connectors[conn.ID] = conn
	return nil
}

func (m *MemoryStore) GetConnector(ctx context.Context, id string) (*Connector, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.connectors[id]
	if !ok {
		return nil, fmt.Errorf("connector not found: %s", id)
	}
	return c, nil
}

func (m *MemoryStore) ListConnectors(ctx context.Context, workspaceID string) ([]*Connector, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*Connector
	for _, c := range m.connectors {
		if workspaceID == "" || c.WorkspaceID == workspaceID {
			list = append(list, c)
		}
	}
	return list, nil
}

func (m *MemoryStore) ListActiveConnectors(ctx context.Context) ([]*Connector, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*Connector
	for _, c := range m.connectors {
		if c.IsActive {
			list = append(list, c)
		}
	}
	return list, nil
}

func (m *MemoryStore) UpdateConnectorSyncTime(ctx context.Context, id string, t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.connectors[id]; ok {
		c.LastSyncedAt = &t
		return nil
	}
	return fmt.Errorf("connector not found: %s", id)
}

func (m *MemoryStore) CreateSyncJob(ctx context.Context, job *SyncJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if job.ID == "" {
		job.ID = uuid.New().String()
	}
	if job.StartedAt.IsZero() {
		job.StartedAt = time.Now().UTC()
	}
	m.syncJobs[job.ID] = job
	return nil
}

func (m *MemoryStore) UpdateSyncJob(ctx context.Context, job *SyncJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.syncJobs[job.ID] = job
	return nil
}

func (m *MemoryStore) GetSyncJob(ctx context.Context, id string) (*SyncJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	j, ok := m.syncJobs[id]
	if !ok {
		return nil, fmt.Errorf("sync job not found: %s", id)
	}
	return j, nil
}

func (m *MemoryStore) GetDocumentByExternalID(ctx context.Context, workspaceID, connectorID, externalID string) (*Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := fmt.Sprintf("%s:%s:%s", workspaceID, connectorID, externalID)
	doc, ok := m.docExtIndex[key]
	if !ok {
		return nil, nil // Return nil if not found
	}
	return doc, nil
}

func (m *MemoryStore) CreateDocument(ctx context.Context, doc *Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}
	now := time.Now().UTC()
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = now
	}
	doc.UpdatedAt = now

	m.documents[doc.ID] = doc
	key := fmt.Sprintf("%s:%s:%s", doc.WorkspaceID, doc.ConnectorID, doc.ExternalID)
	m.docExtIndex[key] = doc
	return nil
}

func (m *MemoryStore) UpdateDocument(ctx context.Context, doc *Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc.UpdatedAt = time.Now().UTC()
	m.documents[doc.ID] = doc
	key := fmt.Sprintf("%s:%s:%s", doc.WorkspaceID, doc.ConnectorID, doc.ExternalID)
	m.docExtIndex[key] = doc
	return nil
}

func (m *MemoryStore) ListDocuments(ctx context.Context, workspaceID string) ([]*Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var list []*Document
	for _, d := range m.documents {
		if workspaceID == "" || d.WorkspaceID == workspaceID {
			list = append(list, d)
		}
	}
	return list, nil
}

func (m *MemoryStore) CreateDocumentVersion(ctx context.Context, version *DocumentVersion) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if version.ID == "" {
		version.ID = uuid.New().String()
	}
	if version.CreatedAt.IsZero() {
		version.CreatedAt = time.Now().UTC()
	}
	m.docVersions[version.ID] = version
	m.docVerLatest[version.DocumentID] = version
	return nil
}

func (m *MemoryStore) GetLatestDocumentVersion(ctx context.Context, docID string) (*DocumentVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ver, ok := m.docVerLatest[docID]
	if !ok {
		return nil, nil
	}
	return ver, nil
}

func (m *MemoryStore) GetExistingChunksForDoc(ctx context.Context, docID string) (map[string]*Chunk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*Chunk)
	if hashMap, ok := m.docChunkMap[docID]; ok {
		for k, v := range hashMap {
			result[k] = v
		}
	}
	return result, nil
}

func (m *MemoryStore) UpsertChunk(ctx context.Context, chunk *Chunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if chunk.ID == "" {
		chunk.ID = uuid.New().String()
	}
	if chunk.CreatedAt.IsZero() {
		chunk.CreatedAt = time.Now().UTC()
	}
	m.chunks[chunk.ID] = chunk

	if _, ok := m.docChunkMap[chunk.DocumentID]; !ok {
		m.docChunkMap[chunk.DocumentID] = make(map[string]*Chunk)
	}
	m.docChunkMap[chunk.DocumentID][chunk.ChunkHash] = chunk
	return nil
}

func (m *MemoryStore) LinkVersionChunks(ctx context.Context, versionID string, chunkIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.verChunks[versionID] = chunkIDs
	return nil
}

func (m *MemoryStore) GetChunksForVersion(ctx context.Context, versionID string) ([]*Chunk, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	chunkIDs, ok := m.verChunks[versionID]
	if !ok {
		return nil, nil
	}
	var chunks []*Chunk
	for _, cid := range chunkIDs {
		if c, found := m.chunks[cid]; found {
			chunks = append(chunks, c)
		}
	}
	return chunks, nil
}
