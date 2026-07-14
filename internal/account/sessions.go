package account

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/security"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

const (
	redisSessionPrefix = "aeon:session:"
	redisRefreshPrefix = "aeon:refresh:"
	redisOnlinePrefix  = "aeon:online:acct:"
	redisServerSetPref = "aeon:online:server:"
	redisServerMeta    = "aeon:server:"
	redisDungeonResume = "aeon:dungeon:resume:"
)

type cachedSession struct {
	SessionID string `json:"sessionId"`
	AccountID int64  `json:"accountId"`
	Status    string `json:"status"`
}

func (s *Service) DungeonRecovery(accountID, characterID int64) (store.DungeonRunRecovery, error) {
	if accountID <= 0 || characterID <= 0 {
		return store.DungeonRunRecovery{}, errors.New("accountId and characterId are required")
	}
	ctx := context.Background()
	key := redisDungeonResume + strconv.FormatInt(accountID, 10) + ":" + strconv.FormatInt(characterID, 10)
	if s.redis != nil && s.redis.Enabled() {
		// Redis is a homepage hint only. PostgreSQL must confirm the run is still
		// STARTED so a stale cache can never resurrect a finished/cancelled run.
		var cached store.DungeonRunRecovery
		_ = s.redis.GetJSON(ctx, key, &cached)
	}
	row, err := s.store.ActiveDungeonRun(accountID, characterID)
	if errors.Is(err, store.ErrNotFound) {
		if s.redis != nil && s.redis.Enabled() {
			_ = s.redis.Delete(ctx, key)
		}
		return store.DungeonRunRecovery{Required: false}, nil
	}
	if err != nil {
		return store.DungeonRunRecovery{}, err
	}
	if s.redis != nil && s.redis.Enabled() {
		_ = s.redis.SetJSON(ctx, key, row, 5*time.Minute)
	}
	return row, nil
}

func (s *Service) ResolveDungeonRecovery(accountID, characterID int64, dungeonRunID, action, sessionID string) (DungeonRecoveryDecision, error) {
	action = strings.ToLower(strings.TrimSpace(action))
	if action != "abandon" && action != "resume" {
		return DungeonRecoveryDecision{}, errors.New("action must be abandon or resume")
	}
	if err := s.RequireActiveSession(sessionID, accountID); err != nil {
		return DungeonRecoveryDecision{}, err
	}
	if action == "resume" {
		row, err := s.store.ActiveDungeonRun(accountID, characterID)
		if err != nil {
			return DungeonRecoveryDecision{}, err
		}
		if row.DungeonRunID != strings.TrimSpace(dungeonRunID) {
			return DungeonRecoveryDecision{}, store.ErrForbidden
		}
		if strings.TrimSpace(row.ServerID) == "" {
			return DungeonRecoveryDecision{}, errors.New("dungeon origin server is unavailable")
		}
		ticket := security.RandomToken("ticket")
		expiresAt := time.Now().UTC().Add(90 * time.Second)
		s.store.SaveTicket(store.GameTicket{
			Ticket: ticket, AccountID: accountID, CharacterID: characterID,
			ServerID: row.ServerID, SessionID: strings.TrimSpace(sessionID), ExpiresAt: expiresAt,
		})
		return DungeonRecoveryDecision{
			Action: "resume", Status: "RESUME_READY", DungeonRunID: row.DungeonRunID,
			ServerID: row.ServerID, Ticket: ticket, ExpiresAt: expiresAt,
		}, nil
	}
	row, err := s.store.CancelDungeonRun(accountID, characterID, strings.TrimSpace(dungeonRunID), "player declined reconnect")
	if err != nil {
		return DungeonRecoveryDecision{}, err
	}
	if s.redis != nil && s.redis.Enabled() {
		key := redisDungeonResume + strconv.FormatInt(accountID, 10) + ":" + strconv.FormatInt(characterID, 10)
		_ = s.redis.Delete(context.Background(), key)
	}
	_ = sessionID
	return DungeonRecoveryDecision{
		Action: "abandon", Status: row.Status, DungeonRunID: row.DungeonRunID, ServerID: row.ServerID,
	}, nil
}

type cachedRefresh struct {
	SessionID string `json:"sessionId"`
	AccountID int64  `json:"accountId"`
}

func (s *Service) sessionTTL() time.Duration {
	hours := s.sessionTTLHours
	if hours <= 0 {
		hours = 168
	}
	return time.Duration(hours) * time.Hour
}

func (s *Service) onlineTTL() time.Duration {
	sec := s.onlineTTLSec
	if sec <= 0 {
		sec = 90
	}
	return time.Duration(sec) * time.Second
}

func (s *Service) cacheSession(ctx context.Context, session store.AccountSession, refreshRaw string, expiresAt time.Time) {
	if s.redis == nil || !s.redis.Enabled() {
		return
	}
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		ttl = s.sessionTTL()
	}
	_ = s.redis.SetJSON(ctx, redisSessionPrefix+session.SessionID, cachedSession{
		SessionID: session.SessionID,
		AccountID: session.AccountID,
		Status:    session.Status,
	}, ttl)
	_ = s.redis.SetJSON(ctx, redisRefreshPrefix+store.HashToken(refreshRaw), cachedRefresh{
		SessionID: session.SessionID,
		AccountID: session.AccountID,
	}, ttl)
}

func (s *Service) dropSessionCache(ctx context.Context, sessionID, refreshRaw string) {
	if s.redis == nil || !s.redis.Enabled() {
		return
	}
	keys := []string{redisSessionPrefix + sessionID}
	if refreshRaw != "" {
		keys = append(keys, redisRefreshPrefix+store.HashToken(refreshRaw))
	}
	_ = s.redis.Delete(ctx, keys...)
}

func (s *Service) cacheOnline(ctx context.Context, row store.OnlineSession) {
	if s.redis == nil || !s.redis.Enabled() {
		return
	}
	ttl := s.onlineTTL()
	acctKey := redisOnlinePrefix + strconv.FormatInt(row.AccountID, 10)
	_ = s.redis.SetJSON(ctx, acctKey, row, ttl)
	_ = s.redis.SAdd(ctx, redisServerSetPref+row.ServerID, strconv.FormatInt(row.AccountID, 10))
}

func (s *Service) dropOnlineCache(ctx context.Context, row store.OnlineSession) {
	if s.redis == nil || !s.redis.Enabled() {
		return
	}
	_ = s.redis.Delete(ctx, redisOnlinePrefix+strconv.FormatInt(row.AccountID, 10))
	_ = s.redis.SRem(ctx, redisServerSetPref+row.ServerID, strconv.FormatInt(row.AccountID, 10))
}

type RefreshResult struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	SessionID    string `json:"sessionId"`
	ExpiresIn    int64  `json:"expiresIn"`
}

type LogoutResult struct {
	Status    string `json:"status"`
	SessionID string `json:"sessionId"`
}

func (s *Service) issueSession(account store.Account, walletPlugin, deviceID, ip, userAgent string) (WalletLoginResult, error) {
	access, err := security.SignAccessToken(s.jwtSecret, account.ID, account.Username, 2*time.Hour)
	if err != nil {
		return WalletLoginResult{}, err
	}
	sessionID := security.RandomToken("session")
	refresh := security.RandomToken("refresh")
	expiresAt := time.Now().UTC().Add(s.sessionTTL())
	session, err := s.store.CreateAccountSession(store.CreateSessionRequest{
		SessionID:    sessionID,
		AccountID:    account.ID,
		RefreshToken: refresh,
		WalletPlugin: walletPlugin,
		DeviceID:     deviceID,
		IPAddress:    ip,
		UserAgent:    userAgent,
		ExpiresAt:    expiresAt,
	})
	if err != nil {
		return WalletLoginResult{}, err
	}
	s.cacheSession(context.Background(), session, refresh, expiresAt)
	return WalletLoginResult{
		Account:      account,
		AccessToken:  access,
		RefreshToken: refresh,
		SessionID:    sessionID,
		WalletPlugin: strings.TrimSpace(walletPlugin),
		ExpiresAt:    expiresAt,
	}, nil
}

func (s *Service) Refresh(refreshToken string) (RefreshResult, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return RefreshResult{}, errors.New("refreshToken is required")
	}
	now := time.Now().UTC()
	rec, err := s.store.LookupRefreshToken(refreshToken, now)
	if err != nil {
		return RefreshResult{}, errors.New("refresh token is invalid or expired")
	}
	session, err := s.store.GetAccountSession(rec.SessionID)
	if err != nil || session.Status != "ACTIVE" {
		return RefreshResult{}, errors.New("session is not active")
	}
	account, ok := s.store.Account(rec.AccountID)
	if !ok || account.IsBanned {
		return RefreshResult{}, errors.New("account unavailable")
	}
	newRefresh := security.RandomToken("refresh")
	expiresAt := now.Add(s.sessionTTL())
	if err := s.store.RotateRefreshToken(refreshToken, newRefresh, rec.SessionID, rec.AccountID, expiresAt, now); err != nil {
		return RefreshResult{}, err
	}
	access, err := security.SignAccessToken(s.jwtSecret, account.ID, account.Username, 2*time.Hour)
	if err != nil {
		return RefreshResult{}, err
	}
	s.dropSessionCache(context.Background(), rec.SessionID, refreshToken)
	s.cacheSession(context.Background(), session, newRefresh, expiresAt)
	return RefreshResult{
		AccessToken:  access,
		RefreshToken: newRefresh,
		SessionID:    rec.SessionID,
		ExpiresIn:    int64((2 * time.Hour).Seconds()),
	}, nil
}

func (s *Service) Logout(sessionID, refreshToken string) (LogoutResult, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" && refreshToken != "" {
		if rec, err := s.store.LookupRefreshToken(refreshToken, time.Now().UTC()); err == nil {
			sessionID = rec.SessionID
		}
	}
	if sessionID == "" {
		return LogoutResult{}, errors.New("sessionId or refreshToken is required")
	}
	now := time.Now().UTC()
	_ = s.store.RevokeAccountSession(sessionID, now)
	s.dropSessionCache(context.Background(), sessionID, refreshToken)
	return LogoutResult{Status: "revoked", SessionID: sessionID}, nil
}

func (s *Service) RequireActiveSession(sessionID string, accountID int64) error {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return errors.New("sessionId is required")
	}
	ctx := context.Background()
	if s.redis != nil && s.redis.Enabled() {
		var cached cachedSession
		// Redis is only a hot hint. PostgreSQL confirmation is required so admin
		// revocation/logout cannot be bypassed by an ACTIVE cache entry.
		_ = s.redis.GetJSON(ctx, redisSessionPrefix+sessionID, &cached)
	}
	session, err := s.store.GetAccountSession(sessionID)
	if err != nil {
		return errors.New("session not found")
	}
	if session.Status != "ACTIVE" || session.AccountID != accountID {
		return errors.New("session is not active")
	}
	_ = s.store.TouchAccountSession(sessionID, time.Now().UTC())
	s.cacheSession(ctx, session, "", time.Now().UTC().Add(s.sessionTTL()))
	return nil
}

func (s *Service) RegisterGameServer(server store.GameServer) (store.GameServer, error) {
	server.ServerID = strings.TrimSpace(server.ServerID)
	if server.ServerID == "" {
		return store.GameServer{}, errors.New("serverId is required")
	}
	if strings.TrimSpace(server.Host) == "" {
		return store.GameServer{}, errors.New("host is required")
	}
	if server.Port <= 0 {
		return store.GameServer{}, errors.New("port is required")
	}
	out, err := s.store.UpsertGameServer(server)
	if err != nil {
		return store.GameServer{}, err
	}
	if s.redis != nil && s.redis.Enabled() {
		_ = s.redis.SetJSON(context.Background(), redisServerMeta+out.ServerID, out, 5*time.Minute)
	}
	return out, nil
}

func (s *Service) HeartbeatGameServer(serverID string, onlinePlayers int) (store.GameServer, error) {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return store.GameServer{}, errors.New("serverId is required")
	}
	out, err := s.store.HeartbeatGameServer(serverID, onlinePlayers, time.Now().UTC())
	if err != nil {
		return store.GameServer{}, err
	}
	if s.redis != nil && s.redis.Enabled() {
		_ = s.redis.SetJSON(context.Background(), redisServerMeta+out.ServerID, out, 5*time.Minute)
	}
	return out, nil
}

func (s *Service) ListGameServers(status string) ([]store.GameServer, error) {
	return s.store.ListGameServers(status)
}

func (s *Service) EnterOnline(accountID, characterID int64, sessionID, serverID, connectionID string) (store.OnlineSession, error) {
	if connectionID == "" {
		connectionID = security.RandomToken("conn")
	}
	if strings.TrimSpace(serverID) == "" {
		return store.OnlineSession{}, errors.New("serverId is required")
	}
	if err := s.RequireActiveSession(sessionID, accountID); err != nil {
		return store.OnlineSession{}, err
	}
	if _, ok := s.store.Character(accountID, characterID); !ok {
		return store.OnlineSession{}, errors.New("character not found")
	}
	// Kick previous presence from redis set if relocating.
	if prev, err := s.store.GetOnlineSession(accountID); err == nil {
		s.dropOnlineCache(context.Background(), prev)
	}
	row, err := s.store.EnterOnlineSession(store.OnlineSession{
		AccountID:    accountID,
		CharacterID:  characterID,
		SessionID:    sessionID,
		ServerID:     serverID,
		ConnectionID: connectionID,
	})
	if err != nil {
		return store.OnlineSession{}, err
	}
	s.cacheOnline(context.Background(), row)
	return row, nil
}

func (s *Service) OnlineHeartbeat(accountID int64, connectionID string) (store.OnlineSession, error) {
	row, err := s.store.TouchOnlineSession(accountID, connectionID, time.Now().UTC())
	if err != nil {
		return store.OnlineSession{}, err
	}
	s.cacheOnline(context.Background(), row)
	return row, nil
}

func (s *Service) LeaveOnline(accountID int64, connectionID string) (store.OnlineSession, error) {
	row, err := s.store.LeaveOnlineSession(accountID, connectionID)
	if err != nil {
		return store.OnlineSession{}, err
	}
	s.dropOnlineCache(context.Background(), row)
	return row, nil
}

func (s *Service) GetOnline(accountID int64) (store.OnlineSession, error) {
	ctx := context.Background()
	if s.redis != nil && s.redis.Enabled() {
		var cached store.OnlineSession
		if err := s.redis.GetJSON(ctx, redisOnlinePrefix+strconv.FormatInt(accountID, 10), &cached); err == nil {
			return cached, nil
		}
	}
	return s.store.GetOnlineSession(accountID)
}

func (s *Service) ListOnline(serverID string) ([]store.OnlineSession, error) {
	return s.store.ListOnlineByServer(strings.TrimSpace(serverID))
}

func (s *Service) SweepStaleOnline() (int64, error) {
	cutoff := time.Now().UTC().Add(-s.onlineTTL() * 2)
	n, err := s.store.SweepStaleOnlineSessions(cutoff)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Service) RedisStatus() map[string]any {
	enabled := s.redis != nil && s.redis.Enabled()
	out := map[string]any{"enabled": enabled}
	if !enabled {
		out["mode"] = "postgres-only"
		return out
	}
	if err := s.redis.Ping(context.Background()); err != nil {
		out["ok"] = false
		out["error"] = err.Error()
		return out
	}
	out["ok"] = true
	out["mode"] = "redis+postgres"
	return out
}
