package ingestion

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testDocumentPayload() *DocumentPayload {
	return &DocumentPayload{WorkspaceID: "ws_default", ConnectorID: "connector-1", ExternalID: "s3/connector-1/manual.txt", FileName: "manual.txt", ContentType: "text/plain", Content: []byte("content"), ACLRoles: []string{"default"}}
}

func TestDocumentIngestReturnsServerErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "failed", http.StatusInternalServerError) }))
	defer server.Close()
	if _, err := NewPythonClient(server.URL, "secret").IngestDocument(context.Background(), testDocumentPayload()); err == nil {
		t.Fatal("expected ingestion error")
	}
}

func TestDocumentIngestSendsContentAndInternalSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Internal-Secret") != "secret" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["content_base64"] != "Y29udGVudA==" {
			t.Fatalf("unexpected encoded content: %v", payload["content_base64"])
		}
		_ = json.NewEncoder(w).Encode(DocumentResult{Changed: true, NewChunksEmbedded: 1})
	}))
	defer server.Close()
	result, err := NewPythonClient(server.URL, "secret").IngestDocument(context.Background(), testDocumentPayload())
	if err != nil {
		t.Fatalf("unexpected ingestion error: %v", err)
	}
	if !result.Changed || result.NewChunksEmbedded != 1 {
		t.Fatalf("unexpected ingestion result: %+v", result)
	}
}
