package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/netip"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type AccountSession struct {
	SessionID    string     `json:"sessionId"`
	AccountID    int64      `json:"accountId"`
	WalletPlugin string     `json:"walletPlugin,omitempty"`
	DeviceID     string     `json:"deviceId,omitempty"`
	IPAddress    string     `json:"ipAddress,omitempty"`
	UserAgent    string     `json:"userAgent,omitempty"`
	Status       string     `json:"status"`
	CreatedAt    time.Time  `json:"createdAt"`
	LastSeenAt   *time.Time `json:"lastSeenAt,omitempty"`
	RevokedAt    *time.Time `json:"revokedAt,omitempty"`
}

type RefreshTokenRecord struct {
	TokenHash string     `json:"-"`
	AccountID int64      `json:"accountId"`
	SessionID string     `json:"sessionId"`
	ExpiresAt time.Time  `json:"expiresAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
	CreatedAt time.Time  `json:"createdAt"`
}

type OnlineSession struct {
	AccountID    int64     `json:"accountId"`
	CharacterID  int64     `json:"characterId"`
	SessionID    string    `json:"sessionId"`
	ServerID     string    `json:"serverId"`
	ConnectionID string    `json:"connectionId"`
	EnteredAt    time.Time `json:"enteredAt"`
	LastSeenAt   time.Time `json:"lastSeenAt"`
}

type GameServer struct {
	ServerID        string     `json:"serverId"`
	DisplayName     string     `json:"displayName"`
	Region          string     `json:"region,omitempty"`
	Host            string     `json:"host"`
	Port            int        `json:"port"`
	PublicEndpoint  string     `json:"publicEndpoint,omitempty"`
	MaxPlayers      int        `json:"maxPlayers"`
	OnlinePlayers   int        `json:"onlinePlayers"`
	Status          string     `json:"status"`
	RegisteredAt    time.Time  `json:"registeredAt"`
	LastHeartbeatAt *time.Time `json:"lastHeartbeatAt,omitempty"`
}

type CreateSessionRequest struct {
	SessionID    string
	AccountID    int64
	RefreshToken string
	WalletPlugin string
	DeviceID     string
	IPAddress    string
	UserAgent    string
	ExpiresAt    time.Time
}

func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

func (s *PostgresStore) CreateAccountSession(req CreateSessionRequest) (AccountSession, error) {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AccountSession{}, err
	}
	defer rollback(ctx, tx)

	now := time.Now().UTC()
	var ip any
	if strings.TrimSpace(req.IPAddress) != "" {
		if addr, err := netip.ParseAddr(strings.TrimSpace(req.IPAddress)); err == nil {
			ip = addr.String()
		}
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO account_sessions (session_id, account_id, wallet_plugin, device_id, ip_address, user_agent, status, created_at, last_seen_at)
		VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5::inet, NULLIF($6, ''), 'ACTIVE', $7, $7)
	`, req.SessionID, req.AccountID, req.WalletPlugin, req.DeviceID, ip, req.UserAgent, now)
	if err != nil {
		return AccountSession{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_tokens (token, account_id, session_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, HashToken(req.RefreshToken), req.AccountID, req.SessionID, req.ExpiresAt, now)
	if err != nil {
		return AccountSession{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AccountSession{}, err
	}
	return AccountSession{
		SessionID:    req.SessionID,
		AccountID:    req.AccountID,
		WalletPlugin: req.WalletPlugin,
		DeviceID:     req.DeviceID,
		IPAddress:    req.IPAddress,
		UserAgent:    req.UserAgent,
		Status:       "ACTIVE",
		CreatedAt:    now,
		LastSeenAt:   &now,
	}, nil
}

func (s *PostgresStore) CountMonthlyActiveAccounts(since time.Time) (int, error) {
	var count int
	err := s.pool.QueryRow(context.Background(), `
		SELECT COUNT(DISTINCT account_id)::int
		FROM account_sessions
		WHERE last_seen_at >= $1
	`, since.UTC()).Scan(&count)
	return count, err
}

func (s *PostgresStore) GetAccountSession(sessionID string) (AccountSession, error) {
	ctx := context.Background()
	var row AccountSession
	var plugin, device, ua, ip *string
	err := s.pool.QueryRow(ctx, `
		SELECT session_id, account_id, wallet_plugin, device_id, host(ip_address)::text, user_agent, status, created_at, last_seen_at, revoked_at
		FROM account_sessions WHERE session_id = $1
	`, sessionID).Scan(
		&row.SessionID, &row.AccountID, &plugin, &device, &ip, &ua, &row.Status, &row.CreatedAt, &row.LastSeenAt, &row.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AccountSession{}, ErrNotFound
	}
	if err != nil {
		return AccountSession{}, err
	}
	if plugin != nil {
		row.WalletPlugin = *plugin
	}
	if device != nil {
		row.DeviceID = *device
	}
	if ua != nil {
		row.UserAgent = *ua
	}
	if ip != nil {
		row.IPAddress = *ip
	}
	return row, nil
}

func (s *PostgresStore) TouchAccountSession(sessionID string, now time.Time) error {
	_, err := s.pool.Exec(context.Background(), `
		UPDATE account_sessions SET last_seen_at = $2 WHERE session_id = $1 AND status = 'ACTIVE'
	`, sessionID, now)
	return err
}

func (s *PostgresStore) RevokeAccountSession(sessionID string, now time.Time) error {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	tag, err := tx.Exec(ctx, `
		UPDATE account_sessions
		SET status = 'REVOKED', revoked_at = $2
		WHERE session_id = $1 AND status = 'ACTIVE'
	`, sessionID, now)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// still revoke refresh tokens for the session
	}
	_, err = tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = $2
		WHERE session_id = $1 AND revoked_at IS NULL
	`, sessionID, now)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) LookupRefreshToken(rawToken string, now time.Time) (RefreshTokenRecord, error) {
	ctx := context.Background()
	var row RefreshTokenRecord
	err := s.pool.QueryRow(ctx, `
		SELECT token, account_id, session_id, expires_at, revoked_at, created_at
		FROM refresh_tokens WHERE token = $1
	`, HashToken(rawToken)).Scan(&row.TokenHash, &row.AccountID, &row.SessionID, &row.ExpiresAt, &row.RevokedAt, &row.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshTokenRecord{}, ErrNotFound
	}
	if err != nil {
		return RefreshTokenRecord{}, err
	}
	if row.RevokedAt != nil || row.ExpiresAt.Before(now) {
		return RefreshTokenRecord{}, ErrForbidden
	}
	return row, nil
}

func (s *PostgresStore) RotateRefreshToken(oldRaw, newRaw, sessionID string, accountID int64, expiresAt, now time.Time) error {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	tag, err := tx.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = $2
		WHERE token = $1 AND session_id = $3 AND revoked_at IS NULL
	`, HashToken(oldRaw), now, sessionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrForbidden
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_tokens (token, account_id, session_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, HashToken(newRaw), accountID, sessionID, expiresAt, now)
	if err != nil {
		return err
	}
	_, _ = tx.Exec(ctx, `UPDATE account_sessions SET last_seen_at = $2 WHERE session_id = $1`, sessionID, now)
	return tx.Commit(ctx)
}

func (s *PostgresStore) UpsertGameServer(server GameServer) (GameServer, error) {
	ctx := context.Background()
	now := time.Now().UTC()
	if server.MaxPlayers <= 0 {
		server.MaxPlayers = 50
	}
	if strings.TrimSpace(server.Status) == "" {
		server.Status = "ONLINE"
	}
	if strings.TrimSpace(server.DisplayName) == "" {
		server.DisplayName = server.ServerID
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO game_servers (
			server_id, display_name, region, host, port, public_endpoint, max_players, online_players, status, registered_at, last_heartbeat_at
		) VALUES ($1, $2, NULLIF($3, ''), $4, $5, NULLIF($6, ''), $7, 0, $8, $9, $9)
		ON CONFLICT (server_id) DO UPDATE SET
			display_name = EXCLUDED.display_name,
			region = EXCLUDED.region,
			host = EXCLUDED.host,
			port = EXCLUDED.port,
			public_endpoint = EXCLUDED.public_endpoint,
			max_players = EXCLUDED.max_players,
			status = EXCLUDED.status,
			last_heartbeat_at = EXCLUDED.last_heartbeat_at
		RETURNING server_id, display_name, COALESCE(region, ''), host, port, COALESCE(public_endpoint, ''),
			max_players, online_players, status, registered_at, last_heartbeat_at
	`, server.ServerID, server.DisplayName, server.Region, server.Host, server.Port, server.PublicEndpoint,
		server.MaxPlayers, server.Status, now,
	).Scan(
		&server.ServerID, &server.DisplayName, &server.Region, &server.Host, &server.Port, &server.PublicEndpoint,
		&server.MaxPlayers, &server.OnlinePlayers, &server.Status, &server.RegisteredAt, &server.LastHeartbeatAt,
	)
	return server, err
}

func (s *PostgresStore) HeartbeatGameServer(serverID string, onlinePlayers int, now time.Time) (GameServer, error) {
	ctx := context.Background()
	var server GameServer
	err := s.pool.QueryRow(ctx, `
		UPDATE game_servers
		SET last_heartbeat_at = $2,
		    online_players = GREATEST($3, 0),
		    status = CASE WHEN status = 'OFFLINE' THEN 'ONLINE' ELSE status END
		WHERE server_id = $1
		RETURNING server_id, display_name, COALESCE(region, ''), host, port, COALESCE(public_endpoint, ''),
			max_players, online_players, status, registered_at, last_heartbeat_at
	`, serverID, now, onlinePlayers).Scan(
		&server.ServerID, &server.DisplayName, &server.Region, &server.Host, &server.Port, &server.PublicEndpoint,
		&server.MaxPlayers, &server.OnlinePlayers, &server.Status, &server.RegisteredAt, &server.LastHeartbeatAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return GameServer{}, ErrNotFound
	}
	return server, err
}

func (s *PostgresStore) ListGameServers(status string) ([]GameServer, error) {
	ctx := context.Background()
	args := []any{}
	query := `
		SELECT server_id, display_name, COALESCE(region, ''), host, port, COALESCE(public_endpoint, ''),
			max_players, online_players, status, registered_at, last_heartbeat_at
		FROM game_servers
	`
	if strings.TrimSpace(status) != "" {
		query += ` WHERE status = $1`
		args = append(args, strings.ToUpper(strings.TrimSpace(status)))
	}
	query += ` ORDER BY last_heartbeat_at DESC NULLS LAST, server_id`
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GameServer
	for rows.Next() {
		var row GameServer
		if err := rows.Scan(
			&row.ServerID, &row.DisplayName, &row.Region, &row.Host, &row.Port, &row.PublicEndpoint,
			&row.MaxPlayers, &row.OnlinePlayers, &row.Status, &row.RegisteredAt, &row.LastHeartbeatAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) EnterOnlineSession(row OnlineSession) (OnlineSession, error) {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return OnlineSession{}, err
	}
	defer rollback(ctx, tx)
	now := time.Now().UTC()
	if row.EnteredAt.IsZero() {
		row.EnteredAt = now
	}
	row.LastSeenAt = now

	// Ensure game server exists (consume may race register).
	var exists int
	if err := tx.QueryRow(ctx, `SELECT 1 FROM game_servers WHERE server_id = $1`, row.ServerID).Scan(&exists); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return OnlineSession{}, errors.New("game server is not registered")
		}
		return OnlineSession{}, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO online_sessions (account_id, character_id, session_id, server_id, connection_id, entered_at, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (account_id) DO UPDATE SET
			character_id = EXCLUDED.character_id,
			session_id = EXCLUDED.session_id,
			server_id = EXCLUDED.server_id,
			connection_id = EXCLUDED.connection_id,
			entered_at = EXCLUDED.entered_at,
			last_seen_at = EXCLUDED.last_seen_at
	`, row.AccountID, row.CharacterID, row.SessionID, row.ServerID, row.ConnectionID, row.EnteredAt, row.LastSeenAt)
	if err != nil {
		return OnlineSession{}, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE game_servers SET online_players = (
			SELECT COUNT(*)::int FROM online_sessions WHERE server_id = $1
		), last_heartbeat_at = $2
		WHERE server_id = $1
	`, row.ServerID, now)
	if err != nil {
		return OnlineSession{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OnlineSession{}, err
	}
	return row, nil
}

func (s *PostgresStore) TouchOnlineSession(accountID int64, connectionID string, now time.Time) (OnlineSession, error) {
	ctx := context.Background()
	var row OnlineSession
	err := s.pool.QueryRow(ctx, `
		UPDATE online_sessions
		SET last_seen_at = $3
		WHERE account_id = $1 AND ($2 = '' OR connection_id = $2)
		RETURNING account_id, character_id, session_id, server_id, connection_id, entered_at, last_seen_at
	`, accountID, connectionID, now).Scan(
		&row.AccountID, &row.CharacterID, &row.SessionID, &row.ServerID, &row.ConnectionID, &row.EnteredAt, &row.LastSeenAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OnlineSession{}, ErrNotFound
	}
	return row, err
}

func (s *PostgresStore) LeaveOnlineSession(accountID int64, connectionID string) (OnlineSession, error) {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return OnlineSession{}, err
	}
	defer rollback(ctx, tx)
	var row OnlineSession
	err = tx.QueryRow(ctx, `
		DELETE FROM online_sessions
		WHERE account_id = $1 AND ($2 = '' OR connection_id = $2)
		RETURNING account_id, character_id, session_id, server_id, connection_id, entered_at, last_seen_at
	`, accountID, connectionID).Scan(
		&row.AccountID, &row.CharacterID, &row.SessionID, &row.ServerID, &row.ConnectionID, &row.EnteredAt, &row.LastSeenAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OnlineSession{}, ErrNotFound
	}
	if err != nil {
		return OnlineSession{}, err
	}
	_, err = tx.Exec(ctx, `
		UPDATE game_servers SET online_players = (
			SELECT COUNT(*)::int FROM online_sessions WHERE server_id = $1
		)
		WHERE server_id = $1
	`, row.ServerID)
	if err != nil {
		return OnlineSession{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return OnlineSession{}, err
	}
	return row, nil
}

func (s *PostgresStore) GetOnlineSession(accountID int64) (OnlineSession, error) {
	ctx := context.Background()
	var row OnlineSession
	err := s.pool.QueryRow(ctx, `
		SELECT account_id, character_id, session_id, server_id, connection_id, entered_at, last_seen_at
		FROM online_sessions WHERE account_id = $1
	`, accountID).Scan(
		&row.AccountID, &row.CharacterID, &row.SessionID, &row.ServerID, &row.ConnectionID, &row.EnteredAt, &row.LastSeenAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return OnlineSession{}, ErrNotFound
	}
	return row, err
}

func (s *PostgresStore) ListOnlineByServer(serverID string) ([]OnlineSession, error) {
	ctx := context.Background()
	rows, err := s.pool.Query(ctx, `
		SELECT account_id, character_id, session_id, server_id, connection_id, entered_at, last_seen_at
		FROM online_sessions WHERE server_id = $1
		ORDER BY last_seen_at DESC
	`, serverID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OnlineSession
	for rows.Next() {
		var row OnlineSession
		if err := rows.Scan(&row.AccountID, &row.CharacterID, &row.SessionID, &row.ServerID, &row.ConnectionID, &row.EnteredAt, &row.LastSeenAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) SweepStaleOnlineSessions(olderThan time.Time) (int64, error) {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer rollback(ctx, tx)
	rows, err := tx.Query(ctx, `
		DELETE FROM online_sessions WHERE last_seen_at < $1
		RETURNING server_id
	`, olderThan)
	if err != nil {
		return 0, err
	}
	servers := map[string]struct{}{}
	var n int64
	for rows.Next() {
		var serverID string
		if err := rows.Scan(&serverID); err != nil {
			rows.Close()
			return 0, err
		}
		servers[serverID] = struct{}{}
		n++
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for serverID := range servers {
		_, err = tx.Exec(ctx, `
			UPDATE game_servers SET online_players = (
				SELECT COUNT(*)::int FROM online_sessions WHERE server_id = $1
			)
			WHERE server_id = $1
		`, serverID)
		if err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return n, nil
}
