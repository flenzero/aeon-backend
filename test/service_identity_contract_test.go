package test

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flenzero/aeon-backend/internal/account"
	"github.com/flenzero/aeon-backend/internal/admin"
	"github.com/flenzero/aeon-backend/internal/chain"
	"github.com/flenzero/aeon-backend/internal/economy"
	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/security"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func TestSuperAdminCanRegisterGameServerIdentity(t *testing.T) {
	cfg := testConfig()
	handler := admin.NewHandler(cfg, store.New()).Routes()
	publicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}

	var created struct {
		ServiceID    string   `json:"serviceId"`
		Name         string   `json:"name"`
		Kind         string   `json:"kind"`
		SubjectID    string   `json:"subjectId"`
		PublicKey    string   `json:"publicKey"`
		Capabilities []string `json:"capabilities"`
		Status       string   `json:"status"`
	}
	doHandlerJSON(t, handler, http.MethodPost, "/api/admin/service-identities", map[string]any{
		"serviceId":    "game-server-shanghai-01",
		"name":         "Shanghai Game Server 01",
		"kind":         "GAME_SERVER",
		"subjectId":    "shanghai-01",
		"publicKey":    chain.EncodeBase58(publicKey),
		"capabilities": []string{"account.gameplay", "economy.gameplay"},
		"reason":       "initial production registration",
	}, map[string]string{"Authorization": "Bearer " + cfg.AdminToken}, http.StatusCreated, &created)

	if created.ServiceID != "game-server-shanghai-01" || created.Status != "ACTIVE" {
		t.Fatalf("created identity = %+v", created)
	}
	if len(created.Capabilities) != 2 {
		t.Fatalf("capabilities = %v", created.Capabilities)
	}
}

func TestSignedGameServerCannotFinishAnotherServersDungeon(t *testing.T) {
	cfg := testConfig()
	cfg.Profile = config.ProfileProduction
	st := store.New()
	accountRow := st.UpsertAccountByWallet("origin_binding_wallet")
	character, err := st.CreateCharacter(accountRow.ID, "Origin Binding Hero")
	if err != nil {
		t.Fatal(err)
	}
	type keyPair struct {
		serviceID, serverID string
		privateKey          ed25519.PrivateKey
	}
	servers := make([]keyPair, 0, 2)
	for _, row := range []struct{ serviceID, serverID string }{
		{"game-server-origin-a", "origin-a"}, {"game-server-origin-b", "origin-b"},
	} {
		publicKey, privateKey, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.CreateServiceIdentity(store.CreateServiceIdentityInput{
			ServiceID: row.serviceID, Name: row.serviceID, Kind: "GAME_SERVER", SubjectID: row.serverID,
			PublicKey: chain.EncodeBase58(publicKey), Capabilities: []string{"economy.gameplay"},
			CreatedBy: "bootstrap-super-admin", Reason: "test",
		}); err != nil {
			t.Fatal(err)
		}
		servers = append(servers, keyPair{serviceID: row.serviceID, serverID: row.serverID, privateKey: privateKey})
	}
	if _, err := st.UpsertGameServer(store.GameServer{ServerID: "origin-a", DisplayName: "Origin A", Host: "127.0.0.1", Port: 7001}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnterOnlineSession(store.OnlineSession{
		AccountID: accountRow.ID, CharacterID: character.ID, SessionID: "origin-binding-session",
		ServerID: "origin-a", ConnectionID: "origin-binding-connection",
	}); err != nil {
		t.Fatal(err)
	}
	handler := economy.NewHandler(cfg, st).Routes()
	enterBody := fmt.Sprintf(`{"opId":"origin-http-enter","characterId":%d,"chapterId":0,"floorId":1}`, character.ID)
	enterReq := httptest.NewRequest(http.MethodPost, "/api/economy/dungeon/enter", strings.NewReader(enterBody))
	enterReq.Header.Set("Content-Type", "application/json")
	enterReq.Header.Set("X-Account-Id", fmt.Sprint(accountRow.ID))
	if err := security.SignServiceRequest(enterReq, servers[0].serviceID, servers[0].privateKey, time.Now().UTC(), "nonce-origin-enter-001"); err != nil {
		t.Fatal(err)
	}
	enterRec := httptest.NewRecorder()
	handler.ServeHTTP(enterRec, enterReq)
	if enterRec.Code != http.StatusOK {
		t.Fatalf("enter status=%d body=%s", enterRec.Code, enterRec.Body.String())
	}
	var envelope struct {
		Data store.DungeonResult `json:"data"`
	}
	if err := json.Unmarshal(enterRec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	finishBody := fmt.Sprintf(`{"opId":"origin-http-finish","characterId":%d,"dungeonRunId":%q,"chapterId":0,"floorId":1,"result":"victory","exp":0}`, character.ID, envelope.Data.DungeonRunID)
	finishReq := httptest.NewRequest(http.MethodPost, "/api/economy/dungeon/finish", strings.NewReader(finishBody))
	finishReq.Header.Set("Content-Type", "application/json")
	finishReq.Header.Set("X-Account-Id", fmt.Sprint(accountRow.ID))
	if err := security.SignServiceRequest(finishReq, servers[1].serviceID, servers[1].privateKey, time.Now().UTC(), "nonce-origin-finish-01"); err != nil {
		t.Fatal(err)
	}
	finishRec := httptest.NewRecorder()
	handler.ServeHTTP(finishRec, finishReq)
	if finishRec.Code != http.StatusBadRequest {
		t.Fatalf("finish status=%d want=%d body=%s", finishRec.Code, http.StatusBadRequest, finishRec.Body.String())
	}
}

func TestSignedGameServerCannotStartDungeonForPlayerOnlineElsewhere(t *testing.T) {
	cfg := testConfig()
	cfg.Profile = config.ProfileProduction
	st := store.New()
	accountRow := st.UpsertAccountByWallet("online_origin_wallet")
	character, err := st.CreateCharacter(accountRow.ID, "Online Origin Hero")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertGameServer(store.GameServer{ServerID: "origin-b", DisplayName: "Origin B", Host: "127.0.0.1", Port: 7002}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnterOnlineSession(store.OnlineSession{
		AccountID: accountRow.ID, CharacterID: character.ID, SessionID: "online-origin-session",
		ServerID: "origin-b", ConnectionID: "origin-b-connection",
	}); err != nil {
		t.Fatal(err)
	}
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateServiceIdentity(store.CreateServiceIdentityInput{
		ServiceID: "game-server-online-origin-a", Name: "Online Origin A", Kind: "GAME_SERVER", SubjectID: "origin-a",
		PublicKey: chain.EncodeBase58(publicKey), Capabilities: []string{"economy.gameplay"},
		CreatedBy: "bootstrap-super-admin", Reason: "test",
	}); err != nil {
		t.Fatal(err)
	}
	body := fmt.Sprintf(`{"opId":"online-origin-enter","characterId":%d,"chapterId":0,"floorId":1}`, character.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/economy/dungeon/enter", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Account-Id", fmt.Sprint(accountRow.ID))
	if err := security.SignServiceRequest(req, "game-server-online-origin-a", privateKey, time.Now().UTC(), "nonce-online-origin-01"); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	economy.NewHandler(cfg, st).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestAdminCanListServiceIdentities(t *testing.T) {
	cfg := testConfig()
	st := store.New()
	publicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateServiceIdentity(store.CreateServiceIdentityInput{
		ServiceID: "game-server-shanghai-01", Name: "Shanghai Game Server 01", Kind: "GAME_SERVER",
		SubjectID: "shanghai-01", PublicKey: chain.EncodeBase58(publicKey),
		Capabilities: []string{"account.gameplay", "economy.gameplay"}, CreatedBy: "bootstrap-super-admin", Reason: "test",
	}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Items []store.ServiceIdentity `json:"items"`
	}
	doHandlerJSON(t, admin.NewHandler(cfg, st).Routes(), http.MethodGet, "/api/admin/service-identities?status=ACTIVE&limit=50&offset=0", nil,
		map[string]string{"Authorization": "Bearer " + cfg.AdminToken}, http.StatusOK, &result)
	if len(result.Items) != 1 || result.Items[0].ServiceID != "game-server-shanghai-01" {
		t.Fatalf("items = %+v", result.Items)
	}
}

func TestAdminRequestBodyCannotSpoofAuditActor(t *testing.T) {
	cfg := testConfig()
	st := store.New()
	accountRow := st.UpsertAccountByWallet("audit_actor_wallet")
	handler := admin.NewHandler(cfg, st).Routes()

	var response struct {
		Audit store.AuditEntry `json:"audit"`
	}
	doHandlerJSON(t, handler, http.MethodPost, "/api/admin/accounts/ban", map[string]any{
		"accountId": accountRow.ID,
		"banned":    true,
		"reason":    "security test",
		"adminId":   "forged-admin",
	}, map[string]string{"Authorization": "Bearer " + cfg.AdminToken}, http.StatusOK, &response)
	if response.Audit.AdminID != "bootstrap-super-admin" {
		t.Fatalf("audit actor=%q want authenticated actor", response.Audit.AdminID)
	}
}

func TestGameServerCannotCallWorkerCapability(t *testing.T) {
	cfg := testConfig()
	cfg.Profile = config.ProfileProduction
	st := store.New()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateServiceIdentity(store.CreateServiceIdentityInput{
		ServiceID: "game-server-shanghai-01", Name: "Shanghai Game Server 01", Kind: "GAME_SERVER",
		SubjectID: "shanghai-01", PublicKey: chain.EncodeBase58(publicKey),
		Capabilities: []string{"economy.gameplay"}, CreatedBy: "bootstrap-super-admin", Reason: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/economy/internal/unlocks/settle", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	if err := security.SignServiceRequest(req, "game-server-shanghai-01", privateKey, time.Now().UTC(), "nonce-capability-0001"); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	economy.NewHandler(cfg, st).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestSignedGameServerCanCallGameplayRoute(t *testing.T) {
	cfg := testConfig()
	cfg.Profile = config.ProfileProduction
	st := store.New()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateServiceIdentity(store.CreateServiceIdentityInput{
		ServiceID: "game-server-shanghai-01", Name: "Shanghai Game Server 01", Kind: "GAME_SERVER",
		SubjectID: "shanghai-01", PublicKey: chain.EncodeBase58(publicKey),
		Capabilities: []string{"economy.gameplay"}, CreatedBy: "bootstrap-super-admin", Reason: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/economy/snapshot", nil)
	req.Header.Set("X-Account-Id", "1")
	req.Header.Set("X-Character-Id", "1")
	if err := security.SignServiceRequest(req, "game-server-shanghai-01", privateKey, time.Now().UTC(), "nonce-gameplay-0001"); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	economy.NewHandler(cfg, st).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

func TestSuperAdminDisableImmediatelyRevokesIdentity(t *testing.T) {
	cfg := testConfig()
	cfg.Profile = config.ProfileProduction
	cfg.SuperAdminOpsKey = "test-super-admin-ops-key"
	st := store.New()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateServiceIdentity(store.CreateServiceIdentityInput{
		ServiceID: "game-server-shanghai-01", Name: "Shanghai Game Server 01", Kind: "GAME_SERVER",
		SubjectID: "shanghai-01", PublicKey: chain.EncodeBase58(publicKey),
		Capabilities: []string{"economy.gameplay"}, CreatedBy: "bootstrap-super-admin", Reason: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	doHandlerJSON(t, admin.NewHandler(cfg, st).Routes(), http.MethodDelete, "/api/admin/service-identities/game-server-shanghai-01", map[string]any{
		"reason": "server retired",
	}, map[string]string{"X-Super-Admin-Key": cfg.SuperAdminOpsKey}, http.StatusOK, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/economy/snapshot", nil)
	req.Header.Set("X-Account-Id", "1")
	req.Header.Set("X-Character-Id", "1")
	if err := security.SignServiceRequest(req, "game-server-shanghai-01", privateKey, time.Now().UTC(), "nonce-disabled-0001"); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	economy.NewHandler(cfg, st).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestGameServerIdentityCannotActAsAnotherServer(t *testing.T) {
	cfg := testConfig()
	cfg.Profile = config.ProfileProduction
	st := store.New()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateServiceIdentity(store.CreateServiceIdentityInput{
		ServiceID: "game-server-shanghai-01", Name: "Shanghai Game Server 01", Kind: "GAME_SERVER",
		SubjectID: "shanghai-01", PublicKey: chain.EncodeBase58(publicKey),
		Capabilities: []string{"account.gameplay"}, CreatedBy: "bootstrap-super-admin", Reason: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/game/servers/register", strings.NewReader(`{
		"serverId":"beijing-01","displayName":"Beijing 01","host":"127.0.0.1","port":9001
	}`))
	req.Header.Set("Content-Type", "application/json")
	if err := security.SignServiceRequest(req, "game-server-shanghai-01", privateKey, time.Now().UTC(), "nonce-subject-0001"); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	account.NewHandler(cfg, st).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
	}
}

func TestGameServerIdentityIsBoundToItsServerAcrossGameplayRoutes(t *testing.T) {
	cfg := testConfig()
	cfg.Profile = config.ProfileProduction
	st := store.New()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateServiceIdentity(store.CreateServiceIdentityInput{
		ServiceID: "game-server-shanghai-01", Name: "Shanghai Game Server 01", Kind: "GAME_SERVER",
		SubjectID: "shanghai-01", PublicKey: chain.EncodeBase58(publicKey), Capabilities: []string{"account.gameplay"},
		CreatedBy: "bootstrap-super-admin", Reason: "test",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertGameServer(store.GameServer{ServerID: "beijing-01", DisplayName: "Beijing 01", Host: "127.0.0.1", Port: 9001}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnterOnlineSession(store.OnlineSession{AccountID: 42, CharacterID: 7, SessionID: "session-42", ServerID: "beijing-01", ConnectionID: "conn-42"}); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "consume ticket", method: http.MethodPost, target: "/api/game/launch/consume", body: `{"ticket":"invalid","serverId":"beijing-01","connectionId":"conn"}`},
		{name: "server heartbeat", method: http.MethodPost, target: "/api/game/servers/heartbeat", body: `{"serverId":"beijing-01","onlinePlayers":1}`},
		{name: "online enter", method: http.MethodPost, target: "/api/game/online/enter", body: `{"accountId":42,"characterId":7,"sessionId":"session-42","serverId":"beijing-01","connectionId":"conn-42"}`},
		{name: "online list", method: http.MethodGet, target: "/api/game/online/server?serverId=beijing-01"},
		{name: "online get", method: http.MethodGet, target: "/api/game/online?accountId=42"},
		{name: "online heartbeat", method: http.MethodPost, target: "/api/game/online/heartbeat", body: `{"accountId":42,"connectionId":"conn-42"}`},
		{name: "online leave", method: http.MethodPost, target: "/api/game/online/leave", body: `{"accountId":42,"connectionId":"conn-42"}`},
	}
	handler := account.NewHandler(cfg, st).Routes()
	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.target, strings.NewReader(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			nonce := fmt.Sprintf("nonce-server-binding-%02d", i)
			if err := security.SignServiceRequest(req, "game-server-shanghai-01", privateKey, time.Now().UTC(), nonce); err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
			}
		})
	}
}

func TestServiceRequestNonceCannotBeReplayed(t *testing.T) {
	cfg := testConfig()
	cfg.Profile = config.ProfileProduction
	st := store.New()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateServiceIdentity(store.CreateServiceIdentityInput{
		ServiceID: "game-server-shanghai-01", Name: "Shanghai Game Server 01", Kind: "GAME_SERVER",
		SubjectID: "shanghai-01", PublicKey: chain.EncodeBase58(publicKey), Capabilities: []string{"economy.gameplay"},
		CreatedBy: "bootstrap-super-admin", Reason: "test",
	}); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/economy/snapshot", nil)
	req.Header.Set("X-Account-Id", "1")
	req.Header.Set("X-Character-Id", "1")
	if err := security.SignServiceRequest(req, "game-server-shanghai-01", privateKey, time.Now().UTC(), "nonce-replay-check-001"); err != nil {
		t.Fatal(err)
	}
	handler := economy.NewHandler(cfg, st).Routes()
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, req)
	if first.Code != http.StatusNotFound {
		t.Fatalf("first status=%d body=%s", first.Code, first.Body.String())
	}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, req)
	if second.Code != http.StatusUnauthorized {
		t.Fatalf("replay status=%d want=%d body=%s", second.Code, http.StatusUnauthorized, second.Body.String())
	}
}

func TestProductionRejectsLegacyInternalKey(t *testing.T) {
	cfg := testConfig()
	cfg.Profile = config.ProfileProduction
	req := httptest.NewRequest(http.MethodGet, "/api/economy/snapshot", nil)
	req.Header.Set("X-Internal-Key", cfg.InternalKey)
	req.Header.Set("X-Account-Id", "1")
	req.Header.Set("X-Character-Id", "1")
	rec := httptest.NewRecorder()
	economy.NewHandler(cfg, store.New()).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
}

func TestProductionNFTConfirmationFailsClosedWithoutCoreAdapter(t *testing.T) {
	cfg := testConfig()
	cfg.Profile = config.ProfileProduction
	cfg.StubMode = config.StubDisabled
	cfg.SuperAdminOpsKey = "test-super-admin-ops-key"
	st := store.New()
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateServiceIdentity(store.CreateServiceIdentityInput{
		ServiceID: "mint-operator-primary", Name: "Mint Operator", Kind: "MINT_OPERATOR",
		PublicKey: chain.EncodeBase58(publicKey), Capabilities: []string{"economy.mint"},
		CreatedBy: "bootstrap-super-admin", Reason: "test",
	}); err != nil {
		t.Fatal(err)
	}
	body := `{"opId":"mint-confirm-boundary","requestId":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/economy/internal/nft/mint/confirm", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if err := security.SignServiceRequest(req, "mint-operator-primary", privateKey, time.Now().UTC(), "nonce-mint-disabled-01"); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	economy.NewHandler(cfg, st).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("economy status=%d want=%d body=%s", rec.Code, http.StatusServiceUnavailable, rec.Body.String())
	}

	adminReq := httptest.NewRequest(http.MethodPost, "/api/admin/nft/mint/confirm", strings.NewReader(body))
	adminReq.Header.Set("Content-Type", "application/json")
	adminReq.Header.Set("X-Super-Admin-Key", cfg.SuperAdminOpsKey)
	adminRec := httptest.NewRecorder()
	admin.NewHandler(cfg, st).Routes().ServeHTTP(adminRec, adminReq)
	if adminRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("admin status=%d want=%d body=%s", adminRec.Code, http.StatusServiceUnavailable, adminRec.Body.String())
	}
}
