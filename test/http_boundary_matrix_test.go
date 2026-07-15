package test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flenzero/aeon-backend/internal/account"
	"github.com/flenzero/aeon-backend/internal/admin"
	"github.com/flenzero/aeon-backend/internal/economy"
	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/security"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func TestEveryJSONWriteRouteRejectsOversizedBody(t *testing.T) {
	cfg := testConfig()
	st := store.New()
	accessToken, err := security.SignAccessToken(cfg.JWTSecret, 1, "boundary", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	oversized := `{"padding":"` + strings.Repeat("x", int(httpx.MaxJSONBodyBytes)+1) + `"}`
	tests := []struct {
		name      string
		handler   http.Handler
		endpoints []endpoint
		headers   map[string]string
	}{
		{
			name: "account", handler: account.NewHandler(cfg, st).Routes(), endpoints: accountEndpoints(),
			headers: map[string]string{
				"X-Internal-Key": cfg.InternalKey,
				"X-Account-Id":   "1", "X-Character-Id": "1",
				"Authorization": "Bearer " + accessToken,
			},
		},
		{
			name: "economy", handler: economy.NewHandler(cfg, st).Routes(), endpoints: economyEndpoints(),
			headers: map[string]string{"X-Internal-Key": cfg.InternalKey, "X-Account-Id": "1", "X-Character-Id": "1"},
		},
		{
			name: "admin", handler: admin.NewHandler(cfg, st).Routes(), endpoints: adminEndpoints(),
			headers: map[string]string{"Authorization": "Bearer " + cfg.AdminToken},
		},
	}
	count := 0
	for _, service := range tests {
		for _, ep := range service.endpoints {
			if ep.method != http.MethodPost && ep.method != http.MethodPut && ep.method != http.MethodPatch && ep.method != http.MethodDelete {
				continue
			}
			count++
			t.Run(service.name+"_"+ep.method+"_"+ep.path, func(t *testing.T) {
				req := httptest.NewRequest(ep.method, ep.path, strings.NewReader(oversized))
				req.Header.Set("Content-Type", "application/json")
				for key, value := range service.headers {
					req.Header.Set(key, value)
				}
				rec := httptest.NewRecorder()
				service.handler.ServeHTTP(rec, req)
				if rec.Code != http.StatusBadRequest {
					t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
				}
			})
		}
	}
	if count != 87 {
		t.Fatalf("oversized body manifest covered only %d write routes", count)
	}
}
