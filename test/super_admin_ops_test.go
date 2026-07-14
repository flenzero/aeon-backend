package test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/flenzero/aeon-backend/internal/admin"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func TestSuperAdminOpsAreSeparatedAndIdempotent(t *testing.T) {
	cfg := testConfig()
	cfg.SuperAdminOpsKey = "test-super-admin-ops-key"
	handler := admin.NewHandler(cfg, store.New()).Routes()

	ordinary := map[string]string{"Authorization": "Bearer " + cfg.AdminToken}
	req := httptest.NewRequest(http.MethodPut, "/api/admin/ops/servers/asia-01", nil)
	for key, value := range ordinary {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("ordinary admin super-op status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/admin/accounts/license", nil)
	for key, value := range ordinary {
		req.Header.Set(key, value)
	}
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("ordinary admin license status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}

	super := map[string]string{"X-Super-Admin-Key": cfg.SuperAdminOpsKey}
	body := map[string]any{
		"opId": "ops-server-create-01", "reason": "register Asia server", "host": "game.example.com",
		"port": 7777, "maxPlayers": 100, "status": "maintenance", "region": "asia", "name": "Asia 1",
	}
	var first struct {
		Server  map[string]any   `json:"server"`
		Created bool             `json:"created"`
		Audit   store.AuditEntry `json:"audit"`
	}
	doHandlerJSON(t, handler, http.MethodPut, "/api/admin/ops/servers/asia-01", body, super, http.StatusOK, &first)
	if !first.Created || first.Server["status"] != "maintenance" || first.Audit.ID == 0 {
		t.Fatalf("unexpected create response: %+v", first)
	}

	var replay struct {
		Server  map[string]any   `json:"server"`
		Created bool             `json:"created"`
		Audit   store.AuditEntry `json:"audit"`
	}
	doHandlerJSON(t, handler, http.MethodPut, "/api/admin/ops/servers/asia-01", body, super, http.StatusOK, &replay)
	if replay.Audit.ID != first.Audit.ID || replay.Server["host"] != "game.example.com" {
		t.Fatalf("operation replay did not return original result: first=%+v replay=%+v", first, replay)
	}

	var status struct {
		Server map[string]any `json:"server"`
	}
	doHandlerJSON(t, handler, http.MethodPost, "/api/admin/ops/servers/asia-01/status", map[string]any{
		"opId": "ops-server-status-01", "reason": "open server", "status": "online",
	}, super, http.StatusOK, &status)
	if status.Server["status"] != "online" {
		t.Fatalf("server status=%v want online", status.Server["status"])
	}
}

func TestSuperAdminPreviewAndCommitRequireOperationMetadata(t *testing.T) {
	cfg := testConfig()
	cfg.SuperAdminOpsKey = "test-super-admin-ops-key"
	handler := admin.NewHandler(cfg, store.New()).Routes()

	tests := []struct {
		name, target, body, want string
	}{
		{"lottery preview missing opId", "/api/admin/ops/characters/1/lottery/draw", `{"count":1,"dryRun":true,"reason":"review"}`, "opId is required"},
		{"lottery commit missing reason", "/api/admin/ops/characters/1/lottery/commit-preview", `{"previewId":"preview_test","opId":"lottery-commit-1"}`, "reason is required"},
		{"compensation preview missing opId", "/api/admin/ops/compensation/preview", `{"gold":1,"reason":"review"}`, "opId is required"},
		{"compensation commit missing reason", "/api/admin/ops/compensation/commit", `{"previewId":"preview_test","opId":"compensation-commit-1"}`, "reason is required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.target, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Super-Admin-Key", cfg.SuperAdminOpsKey)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("status=%d body=%s; want 400 with %q", rec.Code, rec.Body.String(), tc.want)
			}
		})
	}
}
