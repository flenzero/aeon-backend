package store

import (
	"context"
	"crypto/ed25519"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/flenzero/aeon-backend/internal/chain"
)

const (
	AdminUserActive   = "ACTIVE"
	AdminUserDisabled = "DISABLED"
)

var adminIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)

var ordinaryAdminRoles = map[string]bool{
	"OPERATOR": true,
	"FINANCE":  true,
	"SUPPORT":  true,
	"VIEWER":   true,
}

type AdminUser struct {
	AdminID       string     `json:"adminId"`
	Username      string     `json:"username"`
	PublicKey     string     `json:"publicKey"`
	Role          string     `json:"role"`
	Status        string     `json:"status"`
	CreatedBy     string     `json:"createdBy,omitempty"`
	CreatedAt     time.Time  `json:"createdAt"`
	LastLoginAt   *time.Time `json:"lastLoginAt,omitempty"`
	DisabledBy    string     `json:"disabledBy,omitempty"`
	DisabledAt    *time.Time `json:"disabledAt,omitempty"`
	DisableReason string     `json:"disableReason,omitempty"`
}

type CreateAdminUserInput struct {
	AdminID   string
	Username  string
	PublicKey string
	Role      string
	CreatedBy string
	Reason    string
}

type AdminLoginNonce struct {
	Nonce     string    `json:"nonce"`
	AdminID   string    `json:"adminId"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

func normalizeAdminUserInput(in CreateAdminUserInput) (CreateAdminUserInput, error) {
	in.AdminID = strings.ToLower(strings.TrimSpace(in.AdminID))
	in.Username = strings.TrimSpace(in.Username)
	in.PublicKey = strings.TrimSpace(in.PublicKey)
	in.Role = strings.ToUpper(strings.TrimSpace(in.Role))
	in.CreatedBy = strings.TrimSpace(in.CreatedBy)
	in.Reason = strings.TrimSpace(in.Reason)
	if !adminIDPattern.MatchString(in.AdminID) {
		return in, errors.New("adminId must be 3-64 lowercase letters, digits, dot, underscore, or dash")
	}
	if in.Username == "" {
		in.Username = in.AdminID
	}
	if utf8.RuneCountInString(in.Username) > 100 {
		return in, errors.New("username must not exceed 100 characters")
	}
	if in.Role == "" {
		in.Role = "OPERATOR"
	}
	if !ordinaryAdminRoles[in.Role] {
		return in, errors.New("role must be OPERATOR, FINANCE, SUPPORT, or VIEWER")
	}
	key, err := chain.DecodeBase58(in.PublicKey)
	if err != nil || len(key) != ed25519.PublicKeySize {
		return in, errors.New("publicKey must be a 32-byte Ed25519 base58 public key")
	}
	if in.CreatedBy == "" {
		return in, errors.New("createdBy is required")
	}
	if in.Reason == "" || utf8.RuneCountInString(in.Reason) > 500 {
		return in, errors.New("reason must be 1-500 characters")
	}
	return in, nil
}

func cloneAdminUser(row AdminUser) AdminUser {
	return row
}

func (s *Store) CreateAdminUser(in CreateAdminUserInput) (AdminUser, error) {
	in, err := normalizeAdminUserInput(in)
	if err != nil {
		return AdminUser{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.adminUsers[in.AdminID]; exists {
		return AdminUser{}, errors.New("admin user already exists")
	}
	for _, existing := range s.adminUsers {
		if existing.Username == in.Username {
			return AdminUser{}, errors.New("username already registered")
		}
		if existing.PublicKey == in.PublicKey {
			return AdminUser{}, errors.New("public key already registered")
		}
	}
	row := AdminUser{
		AdminID: in.AdminID, Username: in.Username, PublicKey: in.PublicKey,
		Role: in.Role, Status: AdminUserActive, CreatedBy: in.CreatedBy, CreatedAt: time.Now().UTC(),
	}
	s.adminUsers[row.AdminID] = &row
	s.nextID++
	s.audits = append(s.audits, AuditEntry{ID: s.nextID, AdminID: in.CreatedBy, Action: "admin_user_create", Target: "admin_user:" + row.AdminID, Reason: in.Reason, CreatedAt: row.CreatedAt})
	return cloneAdminUser(row), nil
}

func (s *Store) AdminUser(adminID string) (AdminUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.adminUsers[strings.ToLower(strings.TrimSpace(adminID))]
	if !ok {
		return AdminUser{}, ErrNotFound
	}
	return cloneAdminUser(*row), nil
}

func (s *Store) ListAdminUsers(status string, limit, offset int) ([]AdminUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status = strings.ToUpper(strings.TrimSpace(status))
	rows := make([]AdminUser, 0, len(s.adminUsers))
	for _, row := range s.adminUsers {
		if status == "" || row.Status == status {
			rows = append(rows, cloneAdminUser(*row))
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].AdminID < rows[j].AdminID })
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset >= len(rows) {
		return []AdminUser{}, nil
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[offset:end], nil
}

func (s *Store) DisableAdminUser(adminID, disabledBy, reason string) (AdminUser, error) {
	adminID = strings.ToLower(strings.TrimSpace(adminID))
	disabledBy = strings.TrimSpace(disabledBy)
	reason = strings.TrimSpace(reason)
	if disabledBy == "" || reason == "" || utf8.RuneCountInString(reason) > 500 {
		return AdminUser{}, errors.New("disabledBy and reason are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.adminUsers[adminID]
	if !ok {
		return AdminUser{}, ErrNotFound
	}
	if row.Status == AdminUserDisabled {
		return cloneAdminUser(*row), nil
	}
	now := time.Now().UTC()
	row.Status, row.DisabledBy, row.DisabledAt, row.DisableReason = AdminUserDisabled, disabledBy, &now, reason
	s.nextID++
	s.audits = append(s.audits, AuditEntry{ID: s.nextID, AdminID: disabledBy, Action: "admin_user_disable", Target: "admin_user:" + adminID, Reason: reason, CreatedAt: now})
	return cloneAdminUser(*row), nil
}

func (s *Store) SaveAdminLoginNonce(row AdminLoginNonce) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now().UTC()
	}
	if row.Status == "" {
		row.Status = "PENDING"
	}
	s.adminLoginNonces[row.Nonce] = &row
}

func (s *Store) AdminLoginNonce(nonce, adminID string, now time.Time) (AdminLoginNonce, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.adminLoginNonces[strings.TrimSpace(nonce)]
	if !ok || row.AdminID != strings.ToLower(strings.TrimSpace(adminID)) {
		return AdminLoginNonce{}, ErrNotFound
	}
	if row.Status != "PENDING" {
		return AdminLoginNonce{}, ErrForbidden
	}
	if !row.ExpiresAt.After(now) {
		row.Status = "EXPIRED"
		return AdminLoginNonce{}, ErrForbidden
	}
	return *row, nil
}

func (s *Store) ConsumeAdminLoginNonce(nonce, adminID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.adminLoginNonces[strings.TrimSpace(nonce)]
	if !ok || row.AdminID != strings.ToLower(strings.TrimSpace(adminID)) {
		return ErrNotFound
	}
	if row.Status != "PENDING" || !row.ExpiresAt.After(now) {
		if row.Status == "PENDING" {
			row.Status = "EXPIRED"
		}
		return ErrForbidden
	}
	row.Status = "CONSUMED"
	return nil
}

func (s *Store) TouchAdminUserLogin(adminID string, now time.Time) (AdminUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.adminUsers[strings.ToLower(strings.TrimSpace(adminID))]
	if !ok {
		return AdminUser{}, ErrNotFound
	}
	row.LastLoginAt = &now
	return cloneAdminUser(*row), nil
}

func (s *PostgresStore) CreateAdminUser(in CreateAdminUserInput) (AdminUser, error) {
	in, err := normalizeAdminUserInput(in)
	if err != nil {
		return AdminUser{}, err
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AdminUser{}, err
	}
	defer rollback(ctx, tx)
	var row AdminUser
	err = tx.QueryRow(ctx, `
		INSERT INTO admin_users (id, username, public_key, role, status, created_by)
		VALUES ($1, $2, $3, $4, 'ACTIVE', $5)
		RETURNING `+adminUserColumns+`
	`, in.AdminID, in.Username, in.PublicKey, in.Role, in.CreatedBy).Scan(
		&row.AdminID, &row.Username, &row.PublicKey, &row.Role, &row.Status, &row.CreatedBy, &row.CreatedAt, &row.LastLoginAt, &row.DisabledBy, &row.DisabledAt, &row.DisableReason,
	)
	if err != nil {
		return AdminUser{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (admin_id, action, target_type, target_id, reason)
		VALUES ($1, 'admin_user_create', 'admin_user', $2, $3)
	`, in.CreatedBy, row.AdminID, in.Reason); err != nil {
		return AdminUser{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AdminUser{}, err
	}
	return row, nil
}

const adminUserColumns = `id, username, COALESCE(public_key, ''), role, status, COALESCE(created_by, ''),
	created_at, last_login_at, COALESCE(disabled_by, ''), disabled_at, COALESCE(disable_reason, '')`

func scanAdminUser(row pgx.Row) (AdminUser, error) {
	var out AdminUser
	err := row.Scan(&out.AdminID, &out.Username, &out.PublicKey, &out.Role, &out.Status, &out.CreatedBy, &out.CreatedAt, &out.LastLoginAt, &out.DisabledBy, &out.DisabledAt, &out.DisableReason)
	if err != nil {
		return AdminUser{}, err
	}
	return out, nil
}

func (s *PostgresStore) AdminUser(adminID string) (AdminUser, error) {
	row, err := scanAdminUser(s.pool.QueryRow(context.Background(), `SELECT `+adminUserColumns+` FROM admin_users WHERE id = $1`, strings.ToLower(strings.TrimSpace(adminID))))
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminUser{}, ErrNotFound
	}
	return row, err
}

func (s *PostgresStore) ListAdminUsers(status string, limit, offset int) ([]AdminUser, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.pool.Query(context.Background(), `SELECT `+adminUserColumns+` FROM admin_users WHERE ($1 = '' OR status = $1) ORDER BY id LIMIT $2 OFFSET $3`, status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AdminUser
	for rows.Next() {
		var row AdminUser
		if err := rows.Scan(&row.AdminID, &row.Username, &row.PublicKey, &row.Role, &row.Status, &row.CreatedBy, &row.CreatedAt, &row.LastLoginAt, &row.DisabledBy, &row.DisabledAt, &row.DisableReason); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) DisableAdminUser(adminID, disabledBy, reason string) (AdminUser, error) {
	adminID, disabledBy, reason = strings.ToLower(strings.TrimSpace(adminID)), strings.TrimSpace(disabledBy), strings.TrimSpace(reason)
	if disabledBy == "" || reason == "" || utf8.RuneCountInString(reason) > 500 {
		return AdminUser{}, errors.New("disabledBy and reason are required")
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AdminUser{}, err
	}
	defer rollback(ctx, tx)
	row, err := scanAdminUser(tx.QueryRow(ctx, `SELECT `+adminUserColumns+` FROM admin_users WHERE id=$1 FOR UPDATE`, adminID))
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminUser{}, ErrNotFound
	}
	if err != nil {
		return AdminUser{}, err
	}
	if row.Status == AdminUserDisabled {
		if err := tx.Commit(ctx); err != nil {
			return AdminUser{}, err
		}
		return row, nil
	}
	row, err = scanAdminUser(tx.QueryRow(ctx, `UPDATE admin_users SET status='DISABLED', disabled_by=$2, disabled_at=NOW(), disable_reason=$3 WHERE id=$1 RETURNING `+adminUserColumns, adminID, disabledBy, reason))
	if err != nil {
		return AdminUser{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO admin_audit_logs (admin_id, action, target_type, target_id, reason) VALUES ($1, 'admin_user_disable', 'admin_user', $2, $3)`, disabledBy, adminID, reason); err != nil {
		return AdminUser{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AdminUser{}, err
	}
	return row, nil
}

func (s *PostgresStore) SaveAdminLoginNonce(row AdminLoginNonce) {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO admin_login_nonces (nonce, admin_id, message, status, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, row.Nonce, strings.ToLower(strings.TrimSpace(row.AdminID)), row.Message, pending(row.Status), row.ExpiresAt, row.CreatedAt)
	must(err, "save admin login nonce")
}

func (s *PostgresStore) AdminLoginNonce(nonce, adminID string, now time.Time) (AdminLoginNonce, error) {
	var row AdminLoginNonce
	err := s.pool.QueryRow(context.Background(), `
		SELECT nonce, admin_id, message, status, expires_at, created_at
		FROM admin_login_nonces
		WHERE nonce = $1 AND admin_id = $2
	`, strings.TrimSpace(nonce), strings.ToLower(strings.TrimSpace(adminID))).Scan(&row.Nonce, &row.AdminID, &row.Message, &row.Status, &row.ExpiresAt, &row.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminLoginNonce{}, ErrNotFound
	}
	if err != nil {
		return AdminLoginNonce{}, err
	}
	if row.Status != "PENDING" {
		return AdminLoginNonce{}, ErrForbidden
	}
	if !row.ExpiresAt.After(now) {
		_, _ = s.pool.Exec(context.Background(), `UPDATE admin_login_nonces SET status='EXPIRED' WHERE nonce=$1 AND status='PENDING'`, row.Nonce)
		return AdminLoginNonce{}, ErrForbidden
	}
	return row, nil
}

func (s *PostgresStore) ConsumeAdminLoginNonce(nonce, adminID string, now time.Time) error {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	var status string
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT status, expires_at
		FROM admin_login_nonces
		WHERE nonce = $1 AND admin_id = $2
		FOR UPDATE
	`, strings.TrimSpace(nonce), strings.ToLower(strings.TrimSpace(adminID))).Scan(&status, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "PENDING" || !expiresAt.After(now) {
		if status == "PENDING" {
			_, _ = tx.Exec(ctx, `UPDATE admin_login_nonces SET status='EXPIRED' WHERE nonce=$1`, strings.TrimSpace(nonce))
		}
		return ErrForbidden
	}
	if _, err := tx.Exec(ctx, `UPDATE admin_login_nonces SET status='CONSUMED', consumed_at=$2 WHERE nonce=$1`, strings.TrimSpace(nonce), now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) TouchAdminUserLogin(adminID string, now time.Time) (AdminUser, error) {
	row, err := scanAdminUser(s.pool.QueryRow(context.Background(), `UPDATE admin_users SET last_login_at=$2 WHERE id=$1 RETURNING `+adminUserColumns, strings.ToLower(strings.TrimSpace(adminID)), now))
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminUser{}, ErrNotFound
	}
	return row, err
}
