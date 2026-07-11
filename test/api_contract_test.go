package test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flenzero/aeon-backend/internal/account"
	"github.com/flenzero/aeon-backend/internal/admin"
	"github.com/flenzero/aeon-backend/internal/chain"
	"github.com/flenzero/aeon-backend/internal/economy"
	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type endpoint struct {
	method string
	path   string
	want   int
}

func TestEveryHTTPRouteIsRegisteredAndGuarded(t *testing.T) {
	cfg := testConfig()
	st := store.New()
	tests := []struct {
		name      string
		handler   http.Handler
		endpoints []endpoint
		wantCount int
	}{
		{"account-api", account.NewHandler(cfg, st).Routes(), accountEndpoints(), 20},
		{"economy-api", economy.NewHandler(cfg, st).Routes(), economyEndpoints(), 49},
		{"admin-api", admin.NewHandler(cfg, st).Routes(), adminEndpoints(), 20},
	}

	for _, service := range tests {
		t.Run(service.name, func(t *testing.T) {
			if len(service.endpoints) != service.wantCount {
				t.Fatalf("manifest has %d routes, want %d", len(service.endpoints), service.wantCount)
			}
			for _, ep := range service.endpoints {
				name := ep.method + " " + ep.path
				t.Run(name, func(t *testing.T) {
					req := httptest.NewRequest(ep.method, ep.path, strings.NewReader("{}"))
					req.Header.Set("Content-Type", "application/json")
					rec := httptest.NewRecorder()
					service.handler.ServeHTTP(rec, req)
					if rec.Code != ep.want {
						t.Fatalf("status=%d want=%d body=%s", rec.Code, ep.want, rec.Body.String())
					}
				})
			}
		})
	}
}

func TestAccountEconomyAdminHTTPWorkflow(t *testing.T) {
	cfg := testConfig()
	st := store.New()
	accountHandler := account.NewHandler(cfg, st).Routes()
	economyHandler := economy.NewHandler(cfg, st).Routes()
	adminHandler := admin.NewHandler(cfg, st).Routes()

	seed := []byte("aeon-http-contract-seed-000000000")
	privateKey := ed25519.NewKeyFromSeed(seed[:ed25519.SeedSize])
	wallet := chain.EncodeBase58(privateKey.Public().(ed25519.PublicKey))

	var nonce struct {
		Nonce   string `json:"nonce"`
		Message string `json:"message"`
	}
	doHandlerJSON(t, accountHandler, http.MethodGet, "/api/auth/wallet/nonce?walletAddress="+url.QueryEscape(wallet), nil, nil, http.StatusOK, &nonce)
	signature := chain.EncodeBase58(ed25519.Sign(privateKey, []byte(nonce.Message)))

	var login struct {
		AccessToken  string        `json:"accessToken"`
		RefreshToken string        `json:"refreshToken"`
		SessionID    string        `json:"sessionId"`
		Account      store.Account `json:"account"`
	}
	doHandlerJSON(t, accountHandler, http.MethodPost, "/api/auth/wallet", map[string]any{
		"walletAddress": wallet,
		"walletPlugin":  "contract-test",
		"nonce":         nonce.Nonce,
		"signature":     signature,
	}, nil, http.StatusOK, &login)
	if login.AccessToken == "" || login.RefreshToken == "" || login.SessionID == "" || login.Account.ID == 0 {
		t.Fatalf("incomplete login response: %+v", login)
	}

	internal := map[string]string{"X-Internal-Key": cfg.InternalKey, "X-Account-Id": fmt.Sprint(login.Account.ID)}
	var character store.Character
	doHandlerJSON(t, accountHandler, http.MethodPost, "/api/character/create", map[string]any{"name": "ContractHero"}, internal, http.StatusCreated, &character)
	if character.ID == 0 {
		t.Fatal("character was not created")
	}
	internal["X-Character-Id"] = fmt.Sprint(character.ID)

	var snapshot store.EconomySnapshot
	doHandlerJSON(t, economyHandler, http.MethodGet, "/api/economy/snapshot", nil, internal, http.StatusOK, &snapshot)
	if snapshot.AccountID != login.Account.ID || snapshot.CharacterID != character.ID {
		t.Fatalf("snapshot=%+v", snapshot)
	}

	doHandlerJSON(t, economyHandler, http.MethodPost, "/api/economy/rewards/grant-locked", map[string]any{
		"amount": 25,
		"source": "contract-test",
		"ref":    "workflow",
	}, internal, http.StatusCreated, nil)

	adminHeaders := map[string]string{"Authorization": "Bearer " + cfg.AdminToken}
	var detail store.AdminAccountDetail
	doHandlerJSON(t, adminHandler, http.MethodGet, "/api/admin/accounts?accountId="+fmt.Sprint(login.Account.ID), nil, adminHeaders, http.StatusOK, &detail)
	if detail.ID != login.Account.ID {
		t.Fatalf("admin account=%+v", detail)
	}

	jwtHeaders := map[string]string{"Authorization": "Bearer " + login.AccessToken}
	doHandlerJSON(t, accountHandler, http.MethodGet, "/api/auth/verify", nil, jwtHeaders, http.StatusOK, nil)
}

func TestNFTBackendLifecycleHTTPContract(t *testing.T) {
	cfg := testConfig()
	repo := &nftContractRepository{Repository: store.New()}
	handler := economy.NewHandler(cfg, repo).Routes()
	headers := map[string]string{
		"X-Internal-Key": cfg.InternalKey,
		"X-Account-Id":   "7",
		"X-Character-Id": "9",
	}

	var requested store.NFTMintRequestResult
	doHandlerJSON(t, handler, http.MethodPost, "/api/economy/nft/mint/request", map[string]any{
		"opId": "nft-request-1", "characterId": 9, "equipmentUid": "equipment-1",
	}, headers, http.StatusOK, &requested)
	if requested.Request.Status != "PAID" || requested.Asset.Status != "MINT_REQUESTED" {
		t.Fatalf("requested=%+v", requested)
	}

	var confirmed store.NFTMintRequestResult
	doHandlerJSON(t, handler, http.MethodPost, "/api/economy/internal/nft/mint/confirm", map[string]any{
		"opId": "nft-confirm-1", "requestId": requested.Request.ID,
		"mintAddress": "LocalMint111", "txSignature": "local-signature", "metadataUri": "http://127.0.0.1/metadata.json",
	}, headers, http.StatusOK, &confirmed)
	if confirmed.Request.Status != "CONFIRMED" || confirmed.Asset.Status != "MINTED" || confirmed.Asset.MintAddress != "LocalMint111" {
		t.Fatalf("confirmed=%+v", confirmed)
	}

	var listed struct {
		Items []store.NFTAsset `json:"items"`
	}
	doHandlerJSON(t, handler, http.MethodGet, "/api/economy/nft/assets", nil, headers, http.StatusOK, &listed)
	if len(listed.Items) != 1 || listed.Items[0].Status != "MINTED" {
		t.Fatalf("assets=%+v", listed.Items)
	}
}

type nftContractRepository struct {
	store.Repository
	mu      sync.Mutex
	request store.NFTMintRequest
	asset   store.NFTAsset
}

func (r *nftContractRepository) RequestNFTMint(req store.NFTMintRequestInput) (store.NFTMintRequestResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.request = store.NFTMintRequest{ID: 1, AccountID: req.AccountID, NFTAssetID: 1, SourceAssetType: "EQUIPMENT", SourceAssetID: 1, Status: "PAID", EquipmentUID: req.EquipmentUID, CreatedAt: time.Now().UTC()}
	r.asset = store.NFTAsset{ID: 1, AccountID: req.AccountID, SourceAssetType: "EQUIPMENT", SourceAssetID: 1, Status: "MINT_REQUESTED", EquipmentUID: req.EquipmentUID, CreatedAt: time.Now().UTC()}
	return store.NFTMintRequestResult{Request: r.request, Asset: r.asset}, nil
}

func (r *nftContractRepository) ConfirmNFTMint(req store.NFTMintConfirmInput) (store.NFTMintRequestResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if req.RequestID != r.request.ID {
		return store.NFTMintRequestResult{}, store.ErrNotFound
	}
	now := time.Now().UTC()
	r.request.Status = "CONFIRMED"
	r.request.TxSignature = req.TxSignature
	r.request.ConfirmedAt = &now
	r.asset.Status = "MINTED"
	r.asset.MintAddress = req.MintAddress
	r.asset.MetadataURI = req.MetadataURI
	r.asset.MintedAt = &now
	return store.NFTMintRequestResult{Request: r.request, Asset: r.asset}, nil
}

func (r *nftContractRepository) CancelNFTMint(_ string, accountID, requestID int64) (store.NFTMintRequestResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if accountID != r.request.AccountID || requestID != r.request.ID {
		return store.NFTMintRequestResult{}, store.ErrNotFound
	}
	r.request.Status = "CANCELLED"
	r.asset.Status = "OFFCHAIN"
	return store.NFTMintRequestResult{Request: r.request, Asset: r.asset}, nil
}

func (r *nftContractRepository) ListNFTAssets(accountID int64) ([]store.NFTAsset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.asset.AccountID != accountID {
		return []store.NFTAsset{}, nil
	}
	return []store.NFTAsset{r.asset}, nil
}

func testConfig() config.Config {
	return config.Config{
		ServiceName:           "contract-test",
		InternalKey:           "test-internal-key",
		JWTSecret:             "test-jwt-secret",
		AdminToken:            "test-admin-token",
		EconomyConfigDir:      "../configs/economy",
		SessionTTLHours:       24,
		OnlinePresenceTTLSec:  60,
		AutoWithdrawSingleMax: 5000,
		UserDailyWithdrawMax:  20000,
		GlobalHourlyMax:       30000,
		GlobalDailyMax:        150000,
	}
}

func doHandlerJSON(t *testing.T, handler http.Handler, method, target string, body any, headers map[string]string, wantStatus int, dst any) {
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
	raw := rec.Body.Bytes()
	if rec.Code != wantStatus {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, target, rec.Code, wantStatus, raw)
	}
	var envelope struct {
		OK    bool            `json:"ok"`
		Data  json.RawMessage `json:"data"`
		Error any             `json:"error"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		t.Fatalf("decode envelope: %v body=%s", err, raw)
	}
	if !envelope.OK {
		t.Fatalf("request failed: %s", raw)
	}
	if dst != nil {
		if err := json.Unmarshal(envelope.Data, dst); err != nil {
			t.Fatalf("decode data: %v data=%s", err, envelope.Data)
		}
	}
}

func accountEndpoints() []endpoint {
	return []endpoint{
		{"GET", "/health", 200},
		{"GET", "/api/auth/wallet/nonce?walletAddress=invalid", 400},
		{"POST", "/api/auth/wallet", 400}, {"POST", "/api/auth/refresh", 401},
		{"POST", "/api/auth/logout", 401}, {"GET", "/api/auth/verify", 401},
		{"GET", "/api/auth/session/redis", 401}, {"GET", "/api/character/list", 401},
		{"POST", "/api/character/create", 401}, {"POST", "/api/game/launch", 401},
		{"POST", "/api/game/launch/consume", 401}, {"POST", "/api/game/servers/register", 401},
		{"POST", "/api/game/servers/heartbeat", 401}, {"GET", "/api/game/servers", 401},
		{"POST", "/api/game/online/enter", 401}, {"POST", "/api/game/online/heartbeat", 401},
		{"POST", "/api/game/online/leave", 401}, {"GET", "/api/game/online", 401},
		{"GET", "/api/game/online/server", 401}, {"POST", "/api/game/online/sweep", 401},
	}
}

func economyEndpoints() []endpoint {
	paths := []struct{ method, path string }{
		{"GET", "/api/economy/snapshot"},
		{"POST", "/api/economy/warehouse/deposit"}, {"POST", "/api/economy/warehouse/withdraw"},
		{"POST", "/api/economy/equipment/equip"}, {"POST", "/api/economy/equipment/unequip"}, {"POST", "/api/economy/equipment/repair"},
		{"POST", "/api/economy/nft/mint/request"}, {"POST", "/api/economy/nft/mint/cancel"}, {"POST", "/api/economy/internal/nft/mint/confirm"}, {"GET", "/api/economy/nft/assets"},
		{"POST", "/api/economy/dungeon/enter"}, {"POST", "/api/economy/dungeon/finish"},
		{"POST", "/api/economy/loot/claim-player"}, {"POST", "/api/economy/loot/claim-all"}, {"POST", "/api/economy/loot/discard"},
		{"POST", "/api/economy/gathering/settle"}, {"POST", "/api/economy/farming/harvest"},
		{"POST", "/api/economy/boss/contribute"}, {"POST", "/api/economy/boss/settle"},
		{"POST", "/api/economy/internal/boss/events/open"}, {"POST", "/api/economy/internal/boss/events/close"}, {"POST", "/api/economy/internal/boss/events/settle"}, {"GET", "/api/economy/internal/boss/events/active"},
		{"POST", "/api/economy/inventory/organize"}, {"POST", "/api/economy/warehouse/organize"}, {"POST", "/api/economy/inventory/discard"}, {"POST", "/api/economy/inventory/synthesize"}, {"POST", "/api/economy/inventory/bag/expand"},
		{"POST", "/api/economy/license/purchase"},
		{"GET", "/api/economy/marketplace/listings"}, {"GET", "/api/economy/marketplace/listings/mine"}, {"GET", "/api/economy/marketplace/slots"},
		{"POST", "/api/economy/marketplace/list"}, {"POST", "/api/economy/marketplace/listings/1/buy"}, {"POST", "/api/economy/marketplace/listings/1/cancel"},
		{"POST", "/api/economy/marketplace/slots/expand-material"}, {"POST", "/api/economy/marketplace/slots/expand-wallet"}, {"POST", "/api/economy/marketplace/slots/expand-wallet/submit"},
		{"POST", "/api/economy/internal/payments/submit"}, {"POST", "/api/economy/rewards/grant-locked"},
		{"POST", "/api/chain/token/claim"}, {"GET", "/api/chain/token/ledger"},
		{"POST", "/api/economy/internal/unlocks/settle"}, {"POST", "/api/economy/internal/withdrawals/process"},
		{"POST", "/api/economy/internal/chain/deposits/scan"}, {"POST", "/api/economy/internal/chain/payouts/submit"}, {"POST", "/api/economy/internal/chain/payouts/confirm"}, {"POST", "/api/economy/internal/payments/confirm"},
	}
	out := []endpoint{{"GET", "/health", 200}}
	for _, item := range paths {
		out = append(out, endpoint{item.method, item.path, 401})
	}
	return out
}

func adminEndpoints() []endpoint {
	paths := []struct{ method, path string }{
		{"GET", "/api/admin/accounts"}, {"POST", "/api/admin/accounts/ban"}, {"POST", "/api/admin/accounts/risk-level"}, {"POST", "/api/admin/accounts/license"}, {"POST", "/api/admin/accounts/sessions/revoke"},
		{"GET", "/api/admin/market/restrictions"}, {"POST", "/api/admin/market/restrictions"}, {"POST", "/api/admin/market/restrictions/revoke"},
		{"GET", "/api/admin/risk/events"}, {"POST", "/api/admin/risk/events"}, {"GET", "/api/admin/audits"}, {"GET", "/api/admin/ledger"},
		{"GET", "/api/admin/withdrawals"}, {"POST", "/api/admin/withdrawals/review"}, {"GET", "/api/admin/payments"}, {"GET", "/api/admin/nft/requests"}, {"POST", "/api/admin/nft/mint/confirm"},
		{"GET", "/api/admin/hot-wallet"}, {"POST", "/api/admin/hot-wallet/pause"},
	}
	out := []endpoint{{"GET", "/health", 200}}
	for _, item := range paths {
		out = append(out, endpoint{item.method, item.path, 401})
	}
	return out
}
