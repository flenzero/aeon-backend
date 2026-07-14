package admin

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/flenzero/aeon-backend/internal/chain"
	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/security"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func TestSignedAdminLoginIssuesShortLivedJWT(t *testing.T) {
	cfg := adminTestConfig()
	st := store.New()
	handler := NewHandler(cfg, st).Routes()
	_, privateKey, publicKeyText := adminTestKeypair(t)
	super := map[string]string{"X-Super-Admin-Key": cfg.SuperAdminOpsKey}

	var created store.AdminUser
	doAdminJSON(t, handler, http.MethodPost, "/api/admin/admin-users", map[string]any{
		"adminId": "ops-01", "username": "Ops One", "role": "OPERATOR",
		"publicKey": publicKeyText, "reason": "initial operator",
	}, super, http.StatusCreated, &created)
	if created.AdminID != "ops-01" || created.Status != store.AdminUserActive {
		t.Fatalf("created admin = %+v", created)
	}

	var nonce struct {
		Nonce   string `json:"nonce"`
		Message string `json:"message"`
	}
	doAdminJSON(t, handler, http.MethodGet, "/api/admin/auth/nonce?adminId="+url.QueryEscape("ops-01"), nil, nil, http.StatusOK, &nonce)
	signature := chain.EncodeBase58(ed25519.Sign(privateKey, []byte(nonce.Message)))

	var login struct {
		Admin       store.AdminUser `json:"admin"`
		AccessToken string          `json:"accessToken"`
		ExpiresAt   time.Time       `json:"expiresAt"`
	}
	doAdminJSON(t, handler, http.MethodPost, "/api/admin/auth/login", map[string]any{
		"adminId": "ops-01", "nonce": nonce.Nonce, "signature": signature,
	}, nil, http.StatusOK, &login)
	if login.Admin.AdminID != "ops-01" || login.AccessToken == "" || time.Until(login.ExpiresAt) > 31*time.Minute {
		t.Fatalf("login response = %+v", login)
	}
	claims, err := security.VerifyAdminAccessToken(cfg.JWTSecret, login.AccessToken)
	if err != nil {
		t.Fatalf("verify admin token: %v", err)
	}
	if claims.Subject != "ops-01" || claims.Role != "OPERATOR" {
		t.Fatalf("claims = %+v", claims)
	}

	adminHeaders := map[string]string{"Authorization": "Bearer " + login.AccessToken}
	doAdminJSON(t, handler, http.MethodGet, "/api/admin/audits", nil, adminHeaders, http.StatusOK, nil)

	doAdminJSON(t, handler, http.MethodPost, "/api/admin/auth/login", map[string]any{
		"adminId": "ops-01", "nonce": nonce.Nonce, "signature": signature,
	}, nil, http.StatusUnauthorized, nil)

	doAdminJSON(t, handler, http.MethodDelete, "/api/admin/admin-users/ops-01", map[string]any{
		"reason": "operator retired",
	}, super, http.StatusOK, nil)
	doAdminJSON(t, handler, http.MethodGet, "/api/admin/audits", nil, adminHeaders, http.StatusUnauthorized, nil)
}

func TestExpiredAdminJWTIsRejected(t *testing.T) {
	cfg := adminTestConfig()
	st := store.New()
	_, _, publicKeyText := adminTestKeypair(t)
	if _, err := st.CreateAdminUser(store.CreateAdminUserInput{
		AdminID: "ops-expired", Username: "Expired", Role: "OPERATOR",
		PublicKey: publicKeyText, CreatedBy: "super-admin-ops", Reason: "test",
	}); err != nil {
		t.Fatal(err)
	}
	token, _, err := security.SignAdminAccessToken(cfg.JWTSecret, "ops-expired", "Expired", "OPERATOR", -time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(cfg, st).Routes()
	doAdminJSON(t, handler, http.MethodGet, "/api/admin/audits", nil,
		map[string]string{"Authorization": "Bearer " + token}, http.StatusUnauthorized, nil)
}

func adminTestConfig() config.Config {
	return config.Config{
		ServiceName: "admin-api-test", Profile: config.ProfileTest, TestScope: config.TestScopeContract,
		StubMode: config.StubEnabled, JWTSecret: "admin-test-jwt-secret",
		AdminToken: "test-admin-token", SuperAdminOpsKey: "test-super-admin-key",
		AdminSessionTTLMin: 30, EconomyConfigDir: "../../configs/economy",
	}
}

func adminTestKeypair(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey, string) {
	t.Helper()
	seed := []byte("admin-signed-login-test-seed-000")
	privateKey := ed25519.NewKeyFromSeed(seed[:ed25519.SeedSize])
	publicKey := privateKey.Public().(ed25519.PublicKey)
	return publicKey, privateKey, chain.EncodeBase58(publicKey)
}

func doAdminJSON(t *testing.T, handler http.Handler, method, target string, body any, headers map[string]string, wantStatus int, dst any) {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != wantStatus {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, target, rec.Code, wantStatus, rec.Body.String())
	}
	if dst == nil || rec.Code >= 400 {
		return
	}
	var envelope struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK {
		t.Fatalf("response not ok: %s", rec.Body.String())
	}
	if err := json.Unmarshal(envelope.Data, dst); err != nil {
		t.Fatal(err)
	}
}
