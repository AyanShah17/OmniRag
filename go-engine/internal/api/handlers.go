package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/omnirag/go-engine/internal/connectors"
	"github.com/omnirag/go-engine/internal/database"
	"github.com/omnirag/go-engine/internal/middleware"
	"github.com/omnirag/go-engine/internal/security"
)

type APIHandler struct {
	store        database.ConnectorStore
	orchestrator SyncRunner
}

const maxJSONBodyBytes = 1 << 20

type SyncRunner interface {
	SyncConnector(ctx context.Context, connectorID string, triggerType string) (*database.SyncJob, error)
}

func NewAPIHandler(store database.ConnectorStore, orchestrator SyncRunner) *APIHandler {
	return &APIHandler{
		store:        store,
		orchestrator: orchestrator,
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

func (h *APIHandler) authorizeWorkspace(w http.ResponseWriter, r *http.Request, requested string) (string, bool) {
	identityWorkspace, ok := middleware.WorkspaceIDFromContext(r.Context())
	if !ok || identityWorkspace == "" {
		writeError(w, http.StatusForbidden, "No workspace is associated with this identity")
		return "", false
	}
	if requested == "" {
		requested = identityWorkspace
	}
	if requested != identityWorkspace {
		writeError(w, http.StatusForbidden, "Not authorized for this workspace")
		return "", false
	}

	workspace, err := h.store.GetWorkspace(r.Context(), requested)
	if err != nil || workspace == nil {
		writeError(w, http.StatusNotFound, "Workspace not found")
		return "", false
	}
	identityTenant, ok := middleware.TenantIDFromContext(r.Context())
	if !ok || identityTenant == "" || workspace.TenantID != identityTenant {
		writeError(w, http.StatusForbidden, "Not authorized for this workspace")
		return "", false
	}
	return requested, true
}

func requirePrivilegedRole(w http.ResponseWriter, r *http.Request) bool {
	roles, ok := middleware.RolesFromContext(r.Context())
	if ok {
		for _, role := range roles {
			normalized := strings.ToLower(role)
			if normalized == "owner" || normalized == "admin" || normalized == "org:admin" {
				return true
			}
		}
	}
	writeError(w, http.StatusForbidden, "This action requires an owner or admin role")
	return false
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
	if !requirePrivilegedRole(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
	var conn database.Connector
	if err := json.NewDecoder(r.Body).Decode(&conn); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	if conn.Type == "" || conn.Name == "" || conn.WorkspaceID == "" {
		writeError(w, http.StatusBadRequest, "type, name, and workspace_id are required")
		return
	}
	workspaceID, ok := h.authorizeWorkspace(w, r, conn.WorkspaceID)
	if !ok {
		return
	}
	conn.WorkspaceID = workspaceID

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
	if conn.ID == "" {
		conn.ID = uuid.New().String()
	}
	if conn.SyncFrequency == "" {
		conn.SyncFrequency = "hourly"
	}
	encryptedConfig, err := security.EncryptConfig(conn.Config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Connector credentials could not be encrypted")
		return
	}
	conn.Config = encryptedConfig

	if err := h.store.CreateConnector(r.Context(), &conn); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to persist connector")
		return
	}

	response := conn
	response.Config = security.MaskConfig(conn.Config)
	writeJSON(w, http.StatusCreated, response)
}

// GET /api/v1/connectors?workspace_id=...
func (h *APIHandler) ListConnectors(w http.ResponseWriter, r *http.Request) {
	workspaceID, ok := h.authorizeWorkspace(w, r, r.URL.Query().Get("workspace_id"))
	if !ok {
		return
	}
	list, err := h.store.ListConnectors(r.Context(), workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list connectors")
		return
	}
	masked := make([]*database.Connector, 0, len(list))
	for _, connector := range list {
		response := *connector
		response.Config = security.MaskConfig(connector.Config)
		masked = append(masked, &response)
	}
	writeJSON(w, http.StatusOK, masked)
}

// POST /api/v1/connectors/test
func (h *APIHandler) TestConnector(w http.ResponseWriter, r *http.Request) {
	if !requirePrivilegedRole(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
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
	if !requirePrivilegedRole(w, r) {
		return
	}
	connector, err := h.store.GetConnector(r.Context(), connectorID)
	if err != nil || connector == nil {
		writeError(w, http.StatusNotFound, "Connector not found")
		return
	}
	if _, ok := h.authorizeWorkspace(w, r, connector.WorkspaceID); !ok {
		return
	}
	job, err := h.orchestrator.SyncConnector(r.Context(), connectorID, "online_manual")
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("Sync failed: %v", err))
		return
	}
	writeJSON(w, http.StatusOK, job)
}
