package account

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/security"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func TestPublicServersExposeOnlySelectionFields(t *testing.T) {
	st := store.New()
	service := NewServiceWithCache(st, nil, "test-jwt", 24, 60)
	accountRow := st.UpsertAccountByWallet("server_view_wallet")
	character, err := st.CreateCharacter(accountRow.ID, "ServerHero")
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateAccountSession(store.CreateSessionRequest{
		SessionID: "server-view-session", AccountID: accountRow.ID, RefreshToken: "server-view-refresh",
		ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RegisterGameServer(store.GameServer{
		ServerID: "asia-01", DisplayName: "Asia 01", Region: "asia", Host: "10.0.0.1", Port: 7001,
		PublicEndpoint: "wss://asia-01.example.com", MaxPlayers: 100,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.HeartbeatGameServer("asia-01", 42); err != nil {
		t.Fatal(err)
	}
	if _, err := service.EnterOnline(accountRow.ID, character.ID, "server-view-session", "asia-01", "conn-server-view"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{ServiceName: "account-api-test", JWTSecret: "test-jwt", InternalKey: "test-internal", Profile: config.ProfileTest}
	req := httptest.NewRequest(http.MethodGet, "/api/public/servers", nil)
	rec := httptest.NewRecorder()
	NewHandler(cfg, st).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	data := raw["data"].(map[string]any)
	servers := data["servers"].([]any)
	first := servers[0].(map[string]any)
	for _, forbidden := range []string{"host", "port", "publicEndpoint", "lastPing"} {
		if _, ok := first[forbidden]; ok {
			t.Fatalf("public server leaked %s: %+v", forbidden, first)
		}
	}
	if first["serverId"] != "asia-01" || first["status"] != "online" || first["curPlayers"].(float64) != 1 {
		t.Fatalf("public server=%+v", first)
	}
}

func TestPublicHomeStatsAndConfig(t *testing.T) {
	st := store.New()
	service := NewServiceWithCache(st, nil, "test-jwt", 24, 60)
	accountRow := st.UpsertAccountByWallet("home_stats_wallet")
	character, err := st.CreateCharacter(accountRow.ID, "StatsHero")
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateAccountSession(store.CreateSessionRequest{
		SessionID: "home-stats-session", AccountID: accountRow.ID, RefreshToken: "home-stats-refresh",
		WalletPlugin: "phantom", ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RegisterGameServer(store.GameServer{
		ServerID: "world-home", DisplayName: "World Home", Host: "10.0.0.10", Port: 7001, MaxPlayers: 100,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.HeartbeatGameServer("world-home", 50); err != nil {
		t.Fatal(err)
	}
	if _, err := service.EnterOnline(accountRow.ID, character.ID, "home-stats-session", "world-home", "conn-home-stats"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		ServiceName: "account-api-test", JWTSecret: "test-jwt", InternalKey: "test-internal", Profile: config.ProfileTest,
		SolanaTokenMint: "AEBMint111", TokenSymbol: "AEB", GameClientBaseURL: "https://game.example.com",
		SupportWallets: []string{"phantom", "solflare"},
	}
	handler := NewHandler(cfg, st).Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/public/home/stats", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("stats status=%d body=%s", rec.Code, rec.Body.String())
	}
	var stats struct {
		OK   bool `json:"ok"`
		Data struct {
			OnlinePlayers        int       `json:"onlinePlayers"`
			MonthlyActivePlayers int       `json:"monthlyActivePlayers"`
			UpdatedAt            time.Time `json:"updatedAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatal(err)
	}
	if !stats.OK || stats.Data.OnlinePlayers != 1 || stats.Data.MonthlyActivePlayers != 1 || stats.Data.UpdatedAt.IsZero() {
		t.Fatalf("stats=%+v", stats)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/public/home/config", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("config status=%d body=%s", rec.Code, rec.Body.String())
	}
	var homeConfig struct {
		OK   bool `json:"ok"`
		Data struct {
			ContractAddress   string    `json:"contractAddress"`
			TokenSymbol       string    `json:"tokenSymbol"`
			GameClientBaseURL string    `json:"gameClientBaseUrl"`
			SupportWallets    []string  `json:"supportWallets"`
			UpdatedAt         time.Time `json:"updatedAt"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &homeConfig); err != nil {
		t.Fatal(err)
	}
	if !homeConfig.OK || homeConfig.Data.ContractAddress != "AEBMint111" || homeConfig.Data.TokenSymbol != "AEB" ||
		homeConfig.Data.GameClientBaseURL != "https://game.example.com" || len(homeConfig.Data.SupportWallets) != 2 ||
		homeConfig.Data.UpdatedAt.IsZero() {
		t.Fatalf("config=%+v", homeConfig)
	}
}

func TestLaunchIsAccountLevelAndReturnsSelectedServer(t *testing.T) {
	st := store.New()
	accountRow := st.UpsertAccountByWallet("launch_http_wallet")
	_, err := st.CreateAccountSession(store.CreateSessionRequest{
		SessionID: "launch-http-session", AccountID: accountRow.ID, RefreshToken: "launch-http-refresh",
		WalletPlugin: "phantom", ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertGameServer(store.GameServer{
		ServerID: "world-a", DisplayName: "World A", Host: "game.example.com", Port: 7777,
		PublicEndpoint: "wss://game.example.com", MaxPlayers: 100, Status: "ONLINE",
	}); err != nil {
		t.Fatal(err)
	}
	token, err := security.SignAccessToken("test-jwt", accountRow.ID, accountRow.Username, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		ServiceName: "account-api-test", JWTSecret: "test-jwt", InternalKey: "test-internal", Profile: config.ProfileTest,
		GameClientBaseURL: "https://launch-entry.example.com/custom-api?source=home",
	}
	handler := NewHandler(cfg, st).Routes()

	req := httptest.NewRequest(http.MethodPost, "/api/game/launch", strings.NewReader(`{"sessionId":"launch-http-session","serverId":"world-a","characterId":1}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("character launch status=%d want 400 body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/game/launch", strings.NewReader(`{"sessionId":"launch-http-session","serverId":"world-a"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		OK   bool           `json:"ok"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	ticket, _ := response.Data["ticket"].(string)
	if !response.OK || response.Data["status"] != "ready" || ticket == "" || response.Data["serverId"] != "world-a" {
		t.Fatalf("response=%+v", response)
	}
	if response.Data["host"] != "game.example.com" || response.Data["port"] != float64(7777) {
		t.Fatalf("server launch data=%+v", response.Data)
	}
	for _, forbidden := range []string{"publicEndpoint"} {
		if _, ok := response.Data[forbidden]; ok {
			t.Fatalf("launch leaked %s: %+v", forbidden, response.Data)
		}
	}
	if response.Data["walletAddress"] != "launch_http_wallet" || response.Data["walletPlugin"] != "phantom" {
		t.Fatalf("wallet launch data=%+v", response.Data)
	}
	gameURL, _ := response.Data["gameUrl"].(string)
	parsedGameURL, err := url.Parse(gameURL)
	if err != nil {
		t.Fatalf("gameUrl=%q parse: %v", gameURL, err)
	}
	query := parsedGameURL.Query()
	for key, want := range map[string]string{
		"ticket":        ticket,
		"serverId":      "world-a",
		"host":          "game.example.com",
		"port":          "7777",
		"walletAddress": "launch_http_wallet",
		"walletPlugin":  "phantom",
		"source":        "home",
	} {
		if got := query.Get(key); got != want {
			t.Fatalf("gameUrl=%q query %s=%q want %q", gameURL, key, got, want)
		}
	}
	for _, forbidden := range []string{"publicEndpoint"} {
		if query.Has(forbidden) {
			t.Fatalf("gameUrl=%q leaked %s", gameURL, forbidden)
		}
	}
	baseURL, err := url.Parse(cfg.GameClientBaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsedGameURL.Scheme != baseURL.Scheme || parsedGameURL.Host != baseURL.Host || parsedGameURL.Path != baseURL.Path {
		t.Fatalf("gameUrl=%q changed base URL %q", gameURL, cfg.GameClientBaseURL)
	}
}
