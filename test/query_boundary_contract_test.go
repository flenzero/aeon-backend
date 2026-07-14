package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flenzero/aeon-backend/internal/admin"
	"github.com/flenzero/aeon-backend/internal/economy"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func TestListRoutesRejectMalformedAndExtremeQueryBoundaries(t *testing.T) {
	cfg := testConfig()
	st := store.New()
	adminHandler := admin.NewHandler(cfg, st).Routes()
	economyHandler := economy.NewHandler(cfg, st).Routes()
	tests := []struct {
		name    string
		handler http.Handler
		target  string
		headers map[string]string
	}{
		{name: "admin restriction malformed account", handler: adminHandler, target: "/api/admin/market/restrictions?accountId=abc", headers: map[string]string{"Authorization": "Bearer " + cfg.AdminToken}},
		{name: "admin risk negative account", handler: adminHandler, target: "/api/admin/risk/events?accountId=-1", headers: map[string]string{"Authorization": "Bearer " + cfg.AdminToken}},
		{name: "admin payments overflowing account", handler: adminHandler, target: "/api/admin/payments?accountId=999999999999999999999999", headers: map[string]string{"Authorization": "Bearer " + cfg.AdminToken}},
		{name: "admin nft negative offset", handler: adminHandler, target: "/api/admin/nft/requests?offset=-1", headers: map[string]string{"Authorization": "Bearer " + cfg.AdminToken}},
		{name: "admin audit excessive limit", handler: adminHandler, target: "/api/admin/audits?limit=201", headers: map[string]string{"Authorization": "Bearer " + cfg.AdminToken}},
		{name: "admin boolean malformed", handler: adminHandler, target: "/api/admin/market/restrictions?activeOnly=sometimes", headers: map[string]string{"Authorization": "Bearer " + cfg.AdminToken}},
		{name: "market excessive limit", handler: economyHandler, target: "/api/economy/marketplace/listings?limit=101", headers: map[string]string{"X-Internal-Key": cfg.InternalKey}},
		{name: "my market malformed offset", handler: economyHandler, target: "/api/economy/marketplace/listings/mine?offset=abc", headers: map[string]string{"X-Internal-Key": cfg.InternalKey, "X-Account-Id": "1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			for key, value := range tc.headers {
				req.Header.Set(key, value)
			}
			rec := httptest.NewRecorder()
			tc.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}
		})
	}
}
