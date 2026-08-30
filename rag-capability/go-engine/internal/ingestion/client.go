package ingestion

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type DocumentPayload struct {
	WorkspaceID string
	ConnectorID string
	ExternalID  string
	FileName    string
	ContentType string
	Content     []byte
	ACLRoles    []string
	Metadata    map[string]interface{}
}

type DocumentResult struct {
	Changed           bool `json:"changed"`
	ReusedChunksCount int  `json:"reused_chunks_count"`
	NewChunksEmbedded int  `json:"new_chunks_embedded"`
}

type Client interface {
	IngestDocument(ctx context.Context, document *DocumentPayload) (*DocumentResult, error)
}

type PythonClient struct {
	pythonRAGURL   string
	httpClient     *http.Client
	internalSecret string
}

func NewPythonClient(pythonRAGURL string, internalSecret string) *PythonClient {
	return &PythonClient{
		pythonRAGURL:   pythonRAGURL,
		httpClient:     &http.Client{Timeout: 60 * time.Second},
		internalSecret: internalSecret,
	}
}

func (p *PythonClient) IngestDocument(ctx context.Context, document *DocumentPayload) (*DocumentResult, error) {
	if p.pythonRAGURL == "" {
		return nil, fmt.Errorf("Python RAG URL is not configured")
	}
	wirePayload := map[string]interface{}{
		"workspace_id": document.WorkspaceID, "connector_id": document.ConnectorID,
		"external_id": document.ExternalID, "file_name": document.FileName,
		"content_type": document.ContentType, "content_base64": base64.StdEncoding.EncodeToString(document.Content),
		"acl_roles": document.ACLRoles, "metadata": document.Metadata,
	}
	data, err := json.Marshal(wirePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal document payload: %w", err)
	}

	endpoint := fmt.Sprintf("%s/api/v1/internal/ingest-document", p.pythonRAGURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create document ingestion request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.internalSecret != "" {
		req.Header.Set("X-Internal-Secret", p.internalSecret)
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("document ingestion request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Python RAG document ingestion returned status %d", resp.StatusCode)
	}

	var result DocumentResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode document ingestion response: %w", err)
	}
	return &result, nil
}
