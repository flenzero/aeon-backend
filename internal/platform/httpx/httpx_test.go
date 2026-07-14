package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeRejectsJSONBoundaryViolations(t *testing.T) {
	type payload struct {
		Value string `json:"value"`
	}
	tests := []struct {
		name string
		body string
		want bool
	}{
		{name: "valid", body: `{"value":"ok"}`, want: true},
		{name: "unknown field", body: `{"value":"ok","extra":true}`, want: false},
		{name: "trailing document", body: `{"value":"ok"}{"value":"again"}`, want: false},
		{name: "empty", body: ``, want: false},
		{name: "oversized", body: `{"value":"` + strings.Repeat("x", (1<<20)+1) + `"}`, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/", strings.NewReader(tt.body))
			var got payload
			if ok := Decode(req, &got); ok != tt.want {
				t.Fatalf("Decode() = %v, want %v", ok, tt.want)
			}
		})
	}
}

func TestCORSAllowsServiceIdentityAdministrationAndSignedRequests(t *testing.T) {
	handler := WithCORS(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/api/admin/service-identities/game-server-01", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d want=%d", rec.Code, http.StatusNoContent)
	}
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodDelete, http.MethodOptions} {
		if !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), method) {
			t.Errorf("Access-Control-Allow-Methods missing %s: %q", method, rec.Header().Get("Access-Control-Allow-Methods"))
		}
	}
	for _, header := range []string{"X-Service-Id", "X-Service-Timestamp", "X-Service-Nonce", "X-Service-Signature"} {
		if !strings.Contains(rec.Header().Get("Access-Control-Allow-Headers"), header) {
			t.Errorf("Access-Control-Allow-Headers missing %s: %q", header, rec.Header().Get("Access-Control-Allow-Headers"))
		}
	}
}
