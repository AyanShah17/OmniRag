package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/omnirag/go-engine/internal/connectors"
	"github.com/omnirag/go-engine/internal/database"
	"github.com/omnirag/go-engine/internal/diff"
	"github.com/omnirag/go-engine/internal/scheduler"
)

type APIHandler struct {
	store        database.Store
	orchestrator *scheduler.SyncOrchestrator
	differ       *diff.Differ
}

func NewAPIHandler(store database.Store, orchestrator *scheduler.SyncOrchestrator, differ *diff.Differ) *APIHandler {
	return &APIHandler{
		store:        store,
		orchestrator: orchestrator,
		differ:       differ,
	}
}

// JSON Helper
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// GET /healthz
func (h *APIHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "omnirag-go-engine",
		"version": "1.0.0",
	})
}

// POST /api/v1/connectors
func (h *APIHandler) CreateConnector(w http.ResponseWriter, r *http.Request) {
	var conn database.Connector
	if err := json.NewDecoder(r.Body).Decode(&conn); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if conn.Type == "" || conn.Name == "" || conn.WorkspaceID == "" {
		writeError(w, http.StatusBadRequest, "type, name, and workspace_id are required")
		return
	}

	// Validate config with connector adapter
	adapter, err := connectors.CreateConnector(conn.Type, conn.Name, conn.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Failed to initialize connector: %v", err))
		return
	}

	if err := adapter.ValidateConfig(r.Context()); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid connector credentials/configuration: %v", err))
		return
	}

	conn.IsActive = true
	if conn.SyncFrequency == "" {
		conn.SyncFrequency = "hourly"
	}

	if err := h.store.CreateConnector(r.Context(), &conn); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to persist connector")
		return
	}

	writeJSON(w, http.StatusCreated, conn)
}

// GET /api/v1/connectors?workspace_id=...
func (h *APIHandler) ListConnectors(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.URL.Query().Get("workspace_id")
	list, err := h.store.ListConnectors(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list connectors")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

// POST /api/v1/connectors/test
func (h *APIHandler) TestConnector(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Type   string                 `json:"type"`
		Name   string                 `json:"name"`
		Config map[string]interface{} `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	adapter, err := connectors.CreateConnector(payload.Type, payload.Name, payload.Config)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := adapter.ValidateConfig(r.Context()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Try scanning 1 item
	objects, err := adapter.ListObjects(r.Context(), "")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Connection successful but listing failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":       true,
		"message":       "Connector configuration is valid and reachable",
		"objects_found": len(objects),
	})
}

// POST /api/v1/connectors/{id}/sync
func (h *APIHandler) TriggerSync(w http.ResponseWriter, r *http.Request, connectorID string) {
	job, err := h.orchestrator.SyncConnector(r.Context(), connectorID, "online_manual")
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Sync failed: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// POST /api/v1/ingest/file
func (h *APIHandler) DirectFileUpload(w http.ResponseWriter, r *http.Request) {
	// Support multipart form upload or JSON base64
	if err := r.ParseMultipartForm(32 << 20); err == nil {
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, "file parameter is required")
			return
		}
		defer file.Close()

		workspaceID := r.FormValue("workspace_id")
		connectorID := r.FormValue("connector_id")
		if workspaceID == "" {
			workspaceID = "default"
		}

		data, err := io.ReadAll(file)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to read file content")
			return
		}

		diffRes, err := h.orchestrator.IngestRawFile(r.Context(), workspaceID, connectorID, header.Filename, data, map[string]interface{}{
			"source":    "direct_upload",
			"file_size": len(data),
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("Ingest failed: %v", err))
			return
		}

		writeJSON(w, http.StatusOK, diffRes)
		return
	}

	// JSON payload fallback
	var req struct {
		WorkspaceID string `json:"workspace_id"`
		ConnectorID string `json:"connector_id"`
		FileName    string `json:"file_name"`
		Content     string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	diffRes, err := h.orchestrator.IngestRawFile(r.Context(), req.WorkspaceID, req.ConnectorID, req.FileName, []byte(req.Content), map[string]interface{}{
		"source": "api_json",
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Ingest failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, diffRes)
}

// POST /api/v1/webhooks/s3
func (h *APIHandler) S3Webhook(w http.ResponseWriter, r *http.Request) {
	var s3Event struct {
		Records []struct {
			S3 struct {
				Bucket struct {
					Name string `json:"name"`
				} `json:"bucket"`
				Object struct {
					Key  string `json:"key"`
					Size int64  `json:"size"`
					ETag string `json:"eTag"`
				} `json:"object"`
			} `json:"s3"`
		} `json:"Records"`
	}

	if err := json.NewDecoder(r.Body).Decode(&s3Event); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid S3 event notification")
		return
	}

	log.Printf("[Webhook] Received S3 event with %d records", len(s3Event.Records))
	for _, rec := range s3Event.Records {
		log.Printf("[Webhook] S3 Object created/modified: bucket=%s, key=%s", rec.S3.Bucket.Name, rec.S3.Object.Key)
		// Trigger online immediate sync for connector matching bucket
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "received"})
}

// POST /api/v1/webhooks/confluence
func (h *APIHandler) ConfluenceWebhook(w http.ResponseWriter, r *http.Request) {
	var event struct {
		Event string `json:"event"`
		Page  struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"page"`
	}
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid Confluence webhook")
		return
	}

	log.Printf("[Webhook] Confluence event: %s on page: %s (%s)", event.Event, event.Page.Title, event.Page.ID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "received"})
}
