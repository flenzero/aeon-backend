package account

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/chain"
	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/readiness"
	"github.com/flenzero/aeon-backend/internal/platform/redisx"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type Handler struct {
	cfg     config.Config
	service *Service
	store   store.Repository
	cache   redisx.Client
	ready   readiness.Probe
}

func NewHandler(cfg config.Config, st store.Repository) *Handler {
	handler, _ := newHandler(cfg, st, false)
	return handler
}

func OpenHandler(cfg config.Config, st store.Repository) (*Handler, error) {
	return newHandler(cfg, st, true)
}

func newHandler(cfg config.Config, st store.Repository, strict bool) (*Handler, error) {
	cache, fallbackErr := openRedis(cfg)
	if strict && fallbackErr != nil && !cfg.AllowRedisFallback {
		return nil, fallbackErr
	}
	checks := readiness.PersistenceChecks(cfg, st)
	redisCheck := func(ctx context.Context) error {
		if fallbackErr != nil {
			return fallbackErr
		}
		return cache.Ping(ctx)
	}
	if cfg.AllowRedisFallback {
		checks = append(checks, readiness.Optional("redis", redisCheck))
	} else {
		checks = append(checks, readiness.Required("redis", redisCheck))
	}
	handler := &Handler{
		cfg:     cfg,
		service: NewServiceWithCache(st, cache, cfg.JWTSecret, cfg.SessionTTLHours, cfg.OnlinePresenceTTLSec).LoadEconomyRules(cfg.EconomyConfigDir),
		store:   st,
		cache:   cache,
		ready:   readiness.New(cfg.ServiceName, checks...),
	}
	return handler, nil
}

func openRedis(cfg config.Config) (redisx.Client, error) {
	if !cfg.RedisEnabled {
		log.Printf("%s redis disabled; sessions use postgres only", cfg.ServiceName)
		return redisx.NopClient{}, errors.New("Redis is disabled; PostgreSQL-only fallback is active")
	}
	client, err := redisx.Open(context.Background(), cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Printf("%s redis unavailable (%v); falling back to memory cache", cfg.ServiceName, err)
		return redisx.NewMemoryClient(), errors.New("Redis is unavailable; in-memory fallback is active: " + err.Error())
	}
	log.Printf("%s connected to redis at %s", cfg.ServiceName, cfg.RedisAddr)
	return client, nil
}

func (h *Handler) Close() error {
	if h.cache == nil {
		return nil
	}
	return h.cache.Close()
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", httpx.Health(h.cfg.ServiceName))
	mux.Handle("GET /ready", h.ready.Handler())
	mux.HandleFunc("GET /api/auth/wallet/nonce", h.walletNonce)
	mux.HandleFunc("POST /api/auth/wallet", h.walletLogin)
	mux.HandleFunc("POST /api/auth/refresh", h.refresh)
	mux.HandleFunc("POST /api/auth/logout", httpx.RequireJWT(h.cfg, h.logout))
	mux.HandleFunc("GET /api/auth/verify", httpx.RequireJWT(h.cfg, h.verify))
	mux.HandleFunc("GET /api/public/servers", h.publicServers)
	mux.HandleFunc("GET /api/public/servers/online", h.publicOnlineServers)
	mux.HandleFunc("GET /api/public/home/stats", h.publicHomeStats)
	mux.HandleFunc("GET /api/public/home/config", h.publicHomeConfig)
	mux.HandleFunc("GET /api/public/leaderboards/clear-progress", h.publicClearProgressLeaderboard)
	mux.HandleFunc("GET /api/public/leaderboards/weekly-score", h.publicWeeklyScoreLeaderboard)
	gameplay := func(next http.HandlerFunc) http.HandlerFunc {
		return httpx.RequireService(h.cfg, h.store, "account.gameplay", next)
	}
	accountOps := func(next http.HandlerFunc) http.HandlerFunc {
		return httpx.RequireService(h.cfg, h.store, "account.ops", next)
	}
	mux.HandleFunc("GET /api/auth/session/redis", accountOps(h.redisStatus))
	mux.HandleFunc("GET /api/character/list", gameplay(h.characterList))
	mux.HandleFunc("POST /api/character/create", gameplay(h.characterCreate))
	mux.HandleFunc("POST /api/character/delete", gameplay(h.characterDelete))
	mux.HandleFunc("GET /api/player/profile", gameplay(h.playerProfile))
	mux.HandleFunc("POST /api/player/save", gameplay(h.playerSave))
	mux.HandleFunc("POST /api/game/launch", httpx.RequireJWT(h.cfg, h.launch))
	mux.HandleFunc("GET /api/game/dungeon/recovery", httpx.RequireJWT(h.cfg, h.dungeonRecovery))
	mux.HandleFunc("POST /api/game/dungeon/recovery", httpx.RequireJWT(h.cfg, h.resolveDungeonRecovery))
	mux.HandleFunc("POST /api/game/launch/consume", gameplay(h.consumeTicket))
	mux.HandleFunc("POST /api/game/servers/register", gameplay(h.registerServer))
	mux.HandleFunc("POST /api/game/servers/heartbeat", gameplay(h.serverHeartbeat))
	mux.HandleFunc("GET /api/game/servers", gameplay(h.listServers))
	mux.HandleFunc("POST /api/game/online/enter", gameplay(h.onlineEnter))
	mux.HandleFunc("POST /api/game/online/heartbeat", gameplay(h.onlineHeartbeat))
	mux.HandleFunc("POST /api/game/online/leave", gameplay(h.onlineLeave))
	mux.HandleFunc("GET /api/game/online", gameplay(h.onlineGet))
	mux.HandleFunc("GET /api/game/online/server", gameplay(h.onlineListServer))
	mux.HandleFunc("POST /api/game/online/sweep", accountOps(h.onlineSweep))
	return httpx.Recover(httpx.WithCORS(mux))
}

func (h *Handler) dungeonRecovery(w http.ResponseWriter, r *http.Request) {
	accountID, _ := httpx.ContextAccountID(r)
	characterID, _, err := httpx.OptionalPositiveInt64(r, "characterId")
	if err != nil || characterID == 0 {
		httpx.Error(w, http.StatusBadRequest, 400, "characterId must be a positive integer")
		return
	}
	result, err := h.service.DungeonRecovery(accountID, characterID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4014, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) resolveDungeonRecovery(w http.ResponseWriter, r *http.Request) {
	accountID, _ := httpx.ContextAccountID(r)
	var body struct {
		CharacterID  int64  `json:"characterId"`
		DungeonRunID string `json:"dungeonRunId"`
		Action       string `json:"action"`
		SessionID    string `json:"sessionId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if body.CharacterID <= 0 || strings.TrimSpace(body.DungeonRunID) == "" {
		httpx.Error(w, http.StatusBadRequest, 400, "characterId and dungeonRunId are required")
		return
	}
	result, err := h.service.ResolveDungeonRecovery(accountID, body.CharacterID, body.DungeonRunID, body.Action, body.SessionID)
	if err != nil {
		httpx.Error(w, http.StatusConflict, 4015, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) walletNonce(w http.ResponseWriter, r *http.Request) {
	wallet := r.URL.Query().Get("walletAddress")
	if wallet == "" {
		httpx.Error(w, http.StatusBadRequest, 1005, "walletAddress is required")
		return
	}
	if _, err := chain.NormalizeSolanaAddress(wallet); err != nil {
		httpx.Error(w, http.StatusBadRequest, 1005, err.Error())
		return
	}
	result, err := h.service.WalletNonce(wallet)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 1005, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) walletLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		WalletAddress string `json:"walletAddress"`
		WalletPlugin  string `json:"walletPlugin"`
		Nonce         string `json:"nonce"`
		Signature     string `json:"signature"`
		DeviceID      string `json:"deviceId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	result, err := h.service.WalletLogin(body.WalletAddress, body.Nonce, body.Signature, LoginMeta{
		WalletPlugin: body.WalletPlugin,
		DeviceID:     body.DeviceID,
		IPAddress:    clientIP(r),
		UserAgent:    r.UserAgent(),
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 1007, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refreshToken"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	result, err := h.service.Refresh(body.RefreshToken)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, 1008, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SessionID    string `json:"sessionId"`
		RefreshToken string `json:"refreshToken"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	result, err := h.service.Logout(body.SessionID, body.RefreshToken)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 1009, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	accountID, _ := httpx.ContextAccountID(r)
	account, ok := h.store.Account(accountID)
	if !ok {
		httpx.Error(w, http.StatusForbidden, 9001, "account does not exist")
		return
	}
	httpx.OK(w, account)
}

func (h *Handler) redisStatus(w http.ResponseWriter, r *http.Request) {
	httpx.OK(w, h.service.RedisStatus())
}

func (h *Handler) characterList(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"characters": h.service.Characters(accountID)})
}

func (h *Handler) characterCreate(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body struct {
		Name       string         `json:"name"`
		Appearance map[string]any `json:"appearance"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	character, err := h.service.CreateCharacterWithAppearance(accountID, body.Name, body.Appearance)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2005, err.Error())
		return
	}
	httpx.Created(w, character)
}

func (h *Handler) characterDelete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID   int64 `json:"accountId"`
		CharacterID int64 `json:"characterId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	accountID := body.AccountID
	if accountID == 0 {
		var err error
		accountID, err = httpx.AccountID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
			return
		}
	}
	if body.CharacterID <= 0 {
		httpx.Error(w, http.StatusBadRequest, 2007, "characterId is required")
		return
	}
	if err := h.service.DeleteCharacter(accountID, body.CharacterID); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		httpx.Error(w, status, 2008, err.Error())
		return
	}
	httpx.OK(w, map[string]any{})
}

func (h *Handler) playerProfile(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	characterID, err := httpx.CharacterID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
		return
	}
	player, economy, err := h.service.PlayerProfile(accountID, characterID)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		httpx.Error(w, status, 2008, err.Error())
		return
	}
	httpx.OK(w, map[string]any{
		"player":         player,
		"economy":        economy,
		"appearanceJson": player.Appearance,
		"characterName":  player.CharacterName,
	})
}

func (h *Handler) playerSave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID   int64    `json:"accountId"`
		CharacterID int64    `json:"characterId"`
		PosX        *float64 `json:"posX"`
		PosY        *float64 `json:"posY"`
		CurrentMap  string   `json:"currentMap"`
		PlayTimeSec *int64   `json:"playTimeSec"`
		Hunger      *float64 `json:"hunger"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	accountID := body.AccountID
	if accountID == 0 {
		var err error
		accountID, err = httpx.AccountID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
			return
		}
	}
	if body.CharacterID == 0 {
		var err error
		body.CharacterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	if body.PosX == nil || body.PosY == nil || body.PlayTimeSec == nil || body.Hunger == nil {
		httpx.Error(w, http.StatusBadRequest, 2009, "posX, posY, playTimeSec, and hunger are required")
		return
	}
	err := h.service.SavePlayerState(store.PlayerSaveRequest{
		AccountID: accountID, CharacterID: body.CharacterID,
		PosX: *body.PosX, PosY: *body.PosY, CurrentMap: body.CurrentMap,
		PlayTimeSec: *body.PlayTimeSec, Hunger: *body.Hunger,
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
		}
		httpx.Error(w, status, 2009, err.Error())
		return
	}
	httpx.OK(w, map[string]any{})
}

func (h *Handler) launch(w http.ResponseWriter, r *http.Request) {
	accountID, _ := httpx.ContextAccountID(r)
	var body struct {
		SessionID string `json:"sessionId"`
		ServerID  string `json:"serverId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	result, err := h.service.Launch(accountID, body.SessionID, body.ServerID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4013, err.Error())
		return
	}
	result.GameURL = launchGameURL(h.cfg.GameClientBaseURL, result)
	httpx.OK(w, result)
}

func (h *Handler) consumeTicket(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ticket   string `json:"ticket"`
		ServerID string `json:"serverId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if !requireServerSubject(w, r, body.ServerID) {
		return
	}
	result, err := h.service.ConsumeTicket(body.Ticket, body.ServerID)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, 4013, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) publicServers(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.PublicGameServers(false)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4103, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"servers": items})
}

func (h *Handler) publicOnlineServers(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.PublicGameServers(true)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4103, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"servers": items})
}

func (h *Handler) publicHomeStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.HomeStats()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, 4300, err.Error())
		return
	}
	httpx.OK(w, stats)
}

func (h *Handler) publicHomeConfig(w http.ResponseWriter, r *http.Request) {
	wallets := h.cfg.SupportWallets
	if len(wallets) == 0 {
		wallets = []string{"phantom", "solflare", "backpack", "okx"}
	}
	tokenSymbol := strings.TrimSpace(h.cfg.TokenSymbol)
	if tokenSymbol == "" {
		tokenSymbol = "AEB"
	}
	httpx.OK(w, map[string]any{
		"contractAddress":   strings.TrimSpace(h.cfg.SolanaTokenMint),
		"tokenSymbol":       tokenSymbol,
		"gameClientBaseUrl": strings.TrimSpace(h.cfg.GameClientBaseURL),
		"supportWallets":    wallets,
		"updatedAt":         time.Now().UTC(),
	})
}

func (h *Handler) publicClearProgressLeaderboard(w http.ResponseWriter, r *http.Request) {
	limit, _, err := httpx.Pagination(r, 10, 100)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	result, err := h.service.ClearProgressLeaderboard(limit)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4200, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) publicWeeklyScoreLeaderboard(w http.ResponseWriter, r *http.Request) {
	limit, _, err := httpx.Pagination(r, 10, 100)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	result, err := h.service.WeeklyScoreLeaderboard(time.Now().UTC(), limit)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4201, err.Error())
		return
	}
	httpx.OK(w, result)
}

func launchGameURL(base string, result LaunchResult) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return ""
	}
	query := parsed.Query()
	query.Set("ticket", result.Ticket)
	query.Set("serverId", result.ServerID)
	query.Set("host", result.Host)
	query.Set("port", strconv.Itoa(result.Port))
	query.Set("walletAddress", result.WalletAddress)
	if strings.TrimSpace(result.WalletPlugin) != "" {
		query.Set("walletPlugin", result.WalletPlugin)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (h *Handler) registerServer(w http.ResponseWriter, r *http.Request) {
	var body store.GameServer
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if !requireServerSubject(w, r, body.ServerID) {
		return
	}
	server, err := h.service.RegisterGameServer(body)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4101, err.Error())
		return
	}
	httpx.OK(w, server)
}

func requireServerSubject(w http.ResponseWriter, r *http.Request, serverID string) bool {
	identity, ok := httpx.ContextServiceIdentity(r)
	if !ok {
		httpx.Error(w, http.StatusUnauthorized, 4010, "service identity is required")
		return false
	}
	if identity.Kind == "LEGACY" {
		return true
	}
	if identity.Kind != "GAME_SERVER" || strings.TrimSpace(identity.SubjectID) == "" || identity.SubjectID != strings.TrimSpace(serverID) {
		httpx.Error(w, http.StatusForbidden, 4030, "game server identity does not match serverId")
		return false
	}
	return true
}

func (h *Handler) serverHeartbeat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ServerID      string `json:"serverId"`
		OnlinePlayers int    `json:"onlinePlayers"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if !requireServerSubject(w, r, body.ServerID) {
		return
	}
	server, err := h.service.HeartbeatGameServer(body.ServerID, body.OnlinePlayers)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4102, err.Error())
		return
	}
	httpx.OK(w, server)
}

func (h *Handler) listServers(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListGameServers(r.URL.Query().Get("status"))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4103, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"items": items})
}

func (h *Handler) onlineEnter(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID    int64  `json:"accountId"`
		CharacterID  int64  `json:"characterId"`
		SessionID    string `json:"sessionId"`
		ServerID     string `json:"serverId"`
		ConnectionID string `json:"connectionId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if !requireServerSubject(w, r, body.ServerID) {
		return
	}
	if body.AccountID == 0 {
		id, err := httpx.AccountID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
			return
		}
		body.AccountID = id
	}
	row, err := h.service.EnterOnline(body.AccountID, body.CharacterID, body.SessionID, body.ServerID, body.ConnectionID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4201, err.Error())
		return
	}
	httpx.OK(w, row)
}

func (h *Handler) onlineHeartbeat(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID    int64  `json:"accountId"`
		ConnectionID string `json:"connectionId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if body.AccountID == 0 {
		id, err := httpx.AccountID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
			return
		}
		body.AccountID = id
	}
	if !h.requireOnlineServerSubject(w, r, body.AccountID) {
		return
	}
	row, err := h.service.OnlineHeartbeat(body.AccountID, body.ConnectionID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4202, err.Error())
		return
	}
	httpx.OK(w, row)
}

func (h *Handler) onlineLeave(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID    int64  `json:"accountId"`
		ConnectionID string `json:"connectionId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if body.AccountID == 0 {
		id, err := httpx.AccountID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
			return
		}
		body.AccountID = id
	}
	if !h.requireOnlineServerSubject(w, r, body.AccountID) {
		return
	}
	row, err := h.service.LeaveOnline(body.AccountID, body.ConnectionID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4203, err.Error())
		return
	}
	httpx.OK(w, row)
}

func (h *Handler) onlineGet(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		if q := r.URL.Query().Get("accountId"); q != "" {
			accountID, err = strconv.ParseInt(q, 10, 64)
		}
	}
	if err != nil || accountID <= 0 {
		httpx.Error(w, http.StatusBadRequest, 2006, "accountId is required")
		return
	}
	row, err := h.service.GetOnline(accountID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, 4204, err.Error())
		return
	}
	if !requireServerSubject(w, r, row.ServerID) {
		return
	}
	httpx.OK(w, row)
}

func (h *Handler) onlineListServer(w http.ResponseWriter, r *http.Request) {
	serverID := strings.TrimSpace(r.URL.Query().Get("serverId"))
	if serverID == "" {
		httpx.Error(w, http.StatusBadRequest, 4205, "serverId is required")
		return
	}
	if !requireServerSubject(w, r, serverID) {
		return
	}
	items, err := h.service.ListOnline(serverID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4205, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"items": items, "count": len(items)})
}

func (h *Handler) requireOnlineServerSubject(w http.ResponseWriter, r *http.Request, accountID int64) bool {
	row, err := h.service.GetOnline(accountID)
	if err != nil {
		// Preserve the endpoint's existing not-found response when no session exists.
		return true
	}
	return requireServerSubject(w, r, row.ServerID)
}

func (h *Handler) onlineSweep(w http.ResponseWriter, r *http.Request) {
	var body struct{}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	n, err := h.service.SweepStaleOnline()
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4206, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"swept": n})
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
