package account

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

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
		service: NewServiceWithCache(st, cache, cfg.JWTSecret, cfg.SessionTTLHours, cfg.OnlinePresenceTTLSec),
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
	gameplay := func(next http.HandlerFunc) http.HandlerFunc {
		return httpx.RequireService(h.cfg, h.store, "account.gameplay", next)
	}
	accountOps := func(next http.HandlerFunc) http.HandlerFunc {
		return httpx.RequireService(h.cfg, h.store, "account.ops", next)
	}
	mux.HandleFunc("GET /api/auth/session/redis", accountOps(h.redisStatus))
	mux.HandleFunc("GET /api/character/list", gameplay(h.characterList))
	mux.HandleFunc("POST /api/character/create", gameplay(h.characterCreate))
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
	httpx.OK(w, map[string]any{"characters": h.store.Characters(accountID)})
}

func (h *Handler) characterCreate(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	character, err := h.service.CreateCharacter(accountID, body.Name)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2005, err.Error())
		return
	}
	httpx.Created(w, character)
}

func (h *Handler) launch(w http.ResponseWriter, r *http.Request) {
	accountID, _ := httpx.ContextAccountID(r)
	var body struct {
		CharacterID int64  `json:"characterId"`
		SessionID   string `json:"sessionId"`
		ServerID    string `json:"serverId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if body.CharacterID == 0 {
		body.CharacterID, _ = strconv.ParseInt(r.URL.Query().Get("characterId"), 10, 64)
	}
	result, err := h.service.Launch(accountID, body.CharacterID, body.SessionID, body.ServerID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4013, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) consumeTicket(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Ticket       string `json:"ticket"`
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
	result, err := h.service.ConsumeTicket(body.Ticket, body.ServerID, body.ConnectionID)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, 4013, err.Error())
		return
	}
	httpx.OK(w, result)
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
