package account

import (
	"context"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/flenzero/aeon-backend/internal/chain"
	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/redisx"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type Handler struct {
	cfg     config.Config
	service *Service
	store   store.Repository
}

func NewHandler(cfg config.Config, st store.Repository) *Handler {
	cache := openRedis(cfg)
	return &Handler{
		cfg:     cfg,
		service: NewServiceWithCache(st, cache, cfg.JWTSecret, cfg.SessionTTLHours, cfg.OnlinePresenceTTLSec),
		store:   st,
	}
}

func openRedis(cfg config.Config) redisx.Client {
	if !cfg.RedisEnabled {
		log.Printf("%s redis disabled; sessions use postgres only", cfg.ServiceName)
		return redisx.NopClient{}
	}
	client, err := redisx.Open(context.Background(), cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Printf("%s redis unavailable (%v); falling back to memory cache", cfg.ServiceName, err)
		return redisx.NewMemoryClient()
	}
	log.Printf("%s connected to redis at %s", cfg.ServiceName, cfg.RedisAddr)
	return client
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", httpx.Health(h.cfg.ServiceName))
	mux.HandleFunc("GET /api/auth/wallet/nonce", h.walletNonce)
	mux.HandleFunc("POST /api/auth/wallet", h.walletLogin)
	mux.HandleFunc("POST /api/auth/refresh", h.refresh)
	mux.HandleFunc("POST /api/auth/logout", httpx.RequireJWT(h.cfg, h.logout))
	mux.HandleFunc("GET /api/auth/verify", httpx.RequireJWT(h.cfg, h.verify))
	mux.HandleFunc("GET /api/auth/session/redis", httpx.RequireInternal(h.cfg, h.redisStatus))
	mux.HandleFunc("GET /api/character/list", httpx.RequireInternal(h.cfg, h.characterList))
	mux.HandleFunc("POST /api/character/create", httpx.RequireInternal(h.cfg, h.characterCreate))
	mux.HandleFunc("POST /api/game/launch", httpx.RequireJWT(h.cfg, h.launch))
	mux.HandleFunc("POST /api/game/launch/consume", httpx.RequireInternal(h.cfg, h.consumeTicket))
	mux.HandleFunc("POST /api/game/servers/register", httpx.RequireInternal(h.cfg, h.registerServer))
	mux.HandleFunc("POST /api/game/servers/heartbeat", httpx.RequireInternal(h.cfg, h.serverHeartbeat))
	mux.HandleFunc("GET /api/game/servers", httpx.RequireInternal(h.cfg, h.listServers))
	mux.HandleFunc("POST /api/game/online/enter", httpx.RequireInternal(h.cfg, h.onlineEnter))
	mux.HandleFunc("POST /api/game/online/heartbeat", httpx.RequireInternal(h.cfg, h.onlineHeartbeat))
	mux.HandleFunc("POST /api/game/online/leave", httpx.RequireInternal(h.cfg, h.onlineLeave))
	mux.HandleFunc("GET /api/game/online", httpx.RequireInternal(h.cfg, h.onlineGet))
	mux.HandleFunc("GET /api/game/online/server", httpx.RequireInternal(h.cfg, h.onlineListServer))
	mux.HandleFunc("POST /api/game/online/sweep", httpx.RequireInternal(h.cfg, h.onlineSweep))
	return httpx.Recover(httpx.WithCORS(mux))
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
	_ = httpx.Decode(r, &body)
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
	_ = httpx.Decode(r, &body)
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
	server, err := h.service.RegisterGameServer(body)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4101, err.Error())
		return
	}
	httpx.OK(w, server)
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
	httpx.OK(w, row)
}

func (h *Handler) onlineListServer(w http.ResponseWriter, r *http.Request) {
	serverID := strings.TrimSpace(r.URL.Query().Get("serverId"))
	if serverID == "" {
		httpx.Error(w, http.StatusBadRequest, 4205, "serverId is required")
		return
	}
	items, err := h.service.ListOnline(serverID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 4205, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"items": items, "count": len(items)})
}

func (h *Handler) onlineSweep(w http.ResponseWriter, r *http.Request) {
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
