package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/omnirag/go-engine/internal/database"
	"github.com/omnirag/go-engine/internal/middleware"
)

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	store := database.NewMemoryStore()
	ctx := context.Background()
	if err := store.CreateTenant(ctx, &database.Tenant{ID: "tenant_default", Name: "Test"}); err != nil {
		t.Fatal(err)
	}
	if err := store.CreateWorkspace(ctx, &database.Workspace{ID: "ws_default", TenantID: "tenant_default", Name: "Test"}); err != nil {
		t.Fatal(err)
	}
	return NewRouter(
		NewAPIHandler(store, nil),
		middleware.AuthConfig{Mode: "development"},
		[]string{"http://localhost:3000"},
	)
}

func TestListConnectorsRejectsWorkspaceMismatch(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/connectors?workspace_id=ws_other", nil)
	req.Header.Set("X-Workspace-ID", "ws_default")
	recorder := httptest.NewRecorder()

	testRouter(t).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestListConnectorsUsesAuthenticatedWorkspace(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/connectors?workspace_id=ws_default", nil)
	req.Header.Set("X-Workspace-ID", "ws_default")
	recorder := httptest.NewRecorder()

	testRouter(t).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
}
