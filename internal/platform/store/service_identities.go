package store

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/flenzero/aeon-backend/internal/chain"
)

const (
	ServiceIdentityActive   = "ACTIVE"
	ServiceIdentityDisabled = "DISABLED"
)

var serviceIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,63}$`)

var capabilitiesByKind = map[string]map[string]bool{
	"GAME_SERVER": {
		"account.gameplay": true,
		"economy.gameplay": true,
	},
	"WORKER": {
		"economy.worker": true,
	},
	"CHAIN_OPERATOR": {
		"economy.payments": true,
	},
	"MINT_OPERATOR": {
		"economy.mint": true,
	},
	"OPS": {
		"account.ops":      true,
		"economy.boss_ops": true,
		"economy.rewards":  true,
	},
}

type ServiceIdentity struct {
	ServiceID     string     `json:"serviceId"`
	Name          string     `json:"name"`
	Kind          string     `json:"kind"`
	SubjectID     string     `json:"subjectId,omitempty"`
	PublicKey     string     `json:"publicKey"`
	Capabilities  []string   `json:"capabilities"`
	Status        string     `json:"status"`
	CreatedBy     string     `json:"createdBy"`
	CreatedAt     time.Time  `json:"createdAt"`
	DisabledBy    string     `json:"disabledBy,omitempty"`
	DisabledAt    *time.Time `json:"disabledAt,omitempty"`
	DisableReason string     `json:"disableReason,omitempty"`
}

type CreateServiceIdentityInput struct {
	ServiceID    string
	Name         string
	Kind         string
	SubjectID    string
	PublicKey    string
	Capabilities []string
	CreatedBy    string
	Reason       string
}

func normalizeServiceIdentityInput(in CreateServiceIdentityInput) (CreateServiceIdentityInput, error) {
	in.ServiceID = strings.ToLower(strings.TrimSpace(in.ServiceID))
	in.Name = strings.TrimSpace(in.Name)
	in.Kind = strings.ToUpper(strings.TrimSpace(in.Kind))
	in.SubjectID = strings.TrimSpace(in.SubjectID)
	in.PublicKey = strings.TrimSpace(in.PublicKey)
	in.CreatedBy = strings.TrimSpace(in.CreatedBy)
	in.Reason = strings.TrimSpace(in.Reason)
	if !serviceIDPattern.MatchString(in.ServiceID) {
		return in, errors.New("serviceId must be 3-64 lowercase letters, digits, dot, underscore, or dash")
	}
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 100 {
		return in, errors.New("name must be 1-100 characters")
	}
	allowed, ok := capabilitiesByKind[in.Kind]
	if !ok {
		return in, errors.New("kind must be GAME_SERVER, WORKER, CHAIN_OPERATOR, MINT_OPERATOR, or OPS")
	}
	if in.Kind == "GAME_SERVER" && in.SubjectID == "" {
		return in, errors.New("subjectId is required for GAME_SERVER")
	}
	if utf8.RuneCountInString(in.SubjectID) > 100 {
		return in, errors.New("subjectId must not exceed 100 characters")
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
	if len(in.Capabilities) == 0 || len(in.Capabilities) > len(allowed) {
		return in, errors.New("capabilities are required and must match identity kind")
	}
	seen := map[string]bool{}
	capabilities := make([]string, 0, len(in.Capabilities))
	for _, capability := range in.Capabilities {
		capability = strings.ToLower(strings.TrimSpace(capability))
		if !allowed[capability] {
			return in, fmt.Errorf("capability %q is not allowed for %s", capability, in.Kind)
		}
		if !seen[capability] {
			seen[capability] = true
			capabilities = append(capabilities, capability)
		}
	}
	sort.Strings(capabilities)
	in.Capabilities = capabilities
	return in, nil
}

func (row ServiceIdentity) HasCapability(capability string) bool {
	capability = strings.ToLower(strings.TrimSpace(capability))
	for _, current := range row.Capabilities {
		if current == capability {
			return true
		}
	}
	return false
}

func cloneServiceIdentity(row ServiceIdentity) ServiceIdentity {
	row.Capabilities = append([]string(nil), row.Capabilities...)
	return row
}

func (s *Store) CreateServiceIdentity(in CreateServiceIdentityInput) (ServiceIdentity, error) {
	in, err := normalizeServiceIdentityInput(in)
	if err != nil {
		return ServiceIdentity{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.serviceIdentities[in.ServiceID]; exists {
		return ServiceIdentity{}, errors.New("service identity already exists")
	}
	for _, existing := range s.serviceIdentities {
		if existing.PublicKey == in.PublicKey {
			return ServiceIdentity{}, errors.New("public key already registered")
		}
		if in.Kind == "GAME_SERVER" && existing.Kind == in.Kind && existing.SubjectID == in.SubjectID && existing.Status == ServiceIdentityActive {
			return ServiceIdentity{}, errors.New("active game server subject already registered")
		}
	}
	row := ServiceIdentity{
		ServiceID: in.ServiceID, Name: in.Name, Kind: in.Kind, SubjectID: in.SubjectID,
		PublicKey: in.PublicKey, Capabilities: append([]string(nil), in.Capabilities...),
		Status: ServiceIdentityActive, CreatedBy: in.CreatedBy, CreatedAt: time.Now().UTC(),
	}
	s.serviceIdentities[row.ServiceID] = &row
	s.nextID++
	s.audits = append(s.audits, AuditEntry{ID: s.nextID, AdminID: in.CreatedBy, Action: "service_identity_create", Target: "service_identity:" + row.ServiceID, Reason: in.Reason, CreatedAt: row.CreatedAt})
	return cloneServiceIdentity(row), nil
}

func (s *Store) ServiceIdentity(serviceID string) (ServiceIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.serviceIdentities[strings.ToLower(strings.TrimSpace(serviceID))]
	if !ok {
		return ServiceIdentity{}, ErrNotFound
	}
	return cloneServiceIdentity(*row), nil
}

func (s *Store) ListServiceIdentities(status string, limit, offset int) ([]ServiceIdentity, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status = strings.ToUpper(strings.TrimSpace(status))
	rows := make([]ServiceIdentity, 0, len(s.serviceIdentities))
	for _, row := range s.serviceIdentities {
		if status == "" || row.Status == status {
			rows = append(rows, cloneServiceIdentity(*row))
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ServiceID < rows[j].ServiceID })
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset >= len(rows) {
		return []ServiceIdentity{}, nil
	}
	end := offset + limit
	if end > len(rows) {
		end = len(rows)
	}
	return rows[offset:end], nil
}

func (s *Store) DisableServiceIdentity(serviceID, disabledBy, reason string) (ServiceIdentity, error) {
	serviceID = strings.ToLower(strings.TrimSpace(serviceID))
	disabledBy = strings.TrimSpace(disabledBy)
	reason = strings.TrimSpace(reason)
	if disabledBy == "" || reason == "" || utf8.RuneCountInString(reason) > 500 {
		return ServiceIdentity{}, errors.New("disabledBy and reason are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.serviceIdentities[serviceID]
	if !ok {
		return ServiceIdentity{}, ErrNotFound
	}
	if row.Status == ServiceIdentityDisabled {
		return cloneServiceIdentity(*row), nil
	}
	now := time.Now().UTC()
	row.Status, row.DisabledBy, row.DisabledAt, row.DisableReason = ServiceIdentityDisabled, disabledBy, &now, reason
	s.nextID++
	s.audits = append(s.audits, AuditEntry{ID: s.nextID, AdminID: disabledBy, Action: "service_identity_disable", Target: "service_identity:" + serviceID, Reason: reason, CreatedAt: now})
	return cloneServiceIdentity(*row), nil
}

func (s *Store) ConsumeServiceNonce(serviceID, nonce string, expiresAt time.Time) error {
	serviceID, nonce = strings.TrimSpace(serviceID), strings.TrimSpace(nonce)
	if serviceID == "" || nonce == "" {
		return errors.New("serviceId and nonce are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for key, expiry := range s.serviceNonces {
		if !expiry.After(now) {
			delete(s.serviceNonces, key)
		}
	}
	key := serviceID + "\x00" + nonce
	if _, exists := s.serviceNonces[key]; exists {
		return ErrForbidden
	}
	s.serviceNonces[key] = expiresAt
	return nil
}

func (s *PostgresStore) CreateServiceIdentity(in CreateServiceIdentityInput) (ServiceIdentity, error) {
	in, err := normalizeServiceIdentityInput(in)
	if err != nil {
		return ServiceIdentity{}, err
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ServiceIdentity{}, err
	}
	defer rollback(ctx, tx)
	caps, _ := json.Marshal(in.Capabilities)
	var row ServiceIdentity
	var raw []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO service_identities (service_id, name, kind, subject_id, public_key, capabilities, status, created_by)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5, $6::jsonb, 'ACTIVE', $7)
		RETURNING service_id, name, kind, COALESCE(subject_id, ''), public_key, capabilities, status, created_by, created_at
	`, in.ServiceID, in.Name, in.Kind, in.SubjectID, in.PublicKey, caps, in.CreatedBy).Scan(
		&row.ServiceID, &row.Name, &row.Kind, &row.SubjectID, &row.PublicKey, &raw, &row.Status, &row.CreatedBy, &row.CreatedAt,
	)
	if err != nil {
		return ServiceIdentity{}, err
	}
	if err := json.Unmarshal(raw, &row.Capabilities); err != nil {
		return ServiceIdentity{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (admin_id, action, target_type, target_id, reason)
		VALUES ($1, 'service_identity_create', 'service_identity', $2, $3)
	`, in.CreatedBy, row.ServiceID, in.Reason); err != nil {
		return ServiceIdentity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ServiceIdentity{}, err
	}
	return row, nil
}

func scanServiceIdentity(row pgx.Row) (ServiceIdentity, error) {
	var out ServiceIdentity
	var raw []byte
	err := row.Scan(&out.ServiceID, &out.Name, &out.Kind, &out.SubjectID, &out.PublicKey, &raw, &out.Status, &out.CreatedBy, &out.CreatedAt, &out.DisabledBy, &out.DisabledAt, &out.DisableReason)
	if err != nil {
		return ServiceIdentity{}, err
	}
	if err := json.Unmarshal(raw, &out.Capabilities); err != nil {
		return ServiceIdentity{}, err
	}
	return out, nil
}

const serviceIdentityColumns = `service_id, name, kind, COALESCE(subject_id, ''), public_key, capabilities, status,
	created_by, created_at, COALESCE(disabled_by, ''), disabled_at, COALESCE(disable_reason, '')`

func (s *PostgresStore) ServiceIdentity(serviceID string) (ServiceIdentity, error) {
	row, err := scanServiceIdentity(s.pool.QueryRow(context.Background(), `SELECT `+serviceIdentityColumns+` FROM service_identities WHERE service_id = $1`, strings.ToLower(strings.TrimSpace(serviceID))))
	if errors.Is(err, pgx.ErrNoRows) {
		return ServiceIdentity{}, ErrNotFound
	}
	return row, err
}

func (s *PostgresStore) ListServiceIdentities(status string, limit, offset int) ([]ServiceIdentity, error) {
	status = strings.ToUpper(strings.TrimSpace(status))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.pool.Query(context.Background(), `SELECT `+serviceIdentityColumns+` FROM service_identities WHERE ($1 = '' OR status = $1) ORDER BY service_id LIMIT $2 OFFSET $3`, status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ServiceIdentity
	for rows.Next() {
		var row ServiceIdentity
		var raw []byte
		if err := rows.Scan(&row.ServiceID, &row.Name, &row.Kind, &row.SubjectID, &row.PublicKey, &raw, &row.Status, &row.CreatedBy, &row.CreatedAt, &row.DisabledBy, &row.DisabledAt, &row.DisableReason); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &row.Capabilities); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) DisableServiceIdentity(serviceID, disabledBy, reason string) (ServiceIdentity, error) {
	serviceID, disabledBy, reason = strings.ToLower(strings.TrimSpace(serviceID)), strings.TrimSpace(disabledBy), strings.TrimSpace(reason)
	if disabledBy == "" || reason == "" || utf8.RuneCountInString(reason) > 500 {
		return ServiceIdentity{}, errors.New("disabledBy and reason are required")
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ServiceIdentity{}, err
	}
	defer rollback(ctx, tx)
	row, err := scanServiceIdentity(tx.QueryRow(ctx, `SELECT `+serviceIdentityColumns+` FROM service_identities WHERE service_id=$1 FOR UPDATE`, serviceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return ServiceIdentity{}, ErrNotFound
	}
	if err != nil {
		return ServiceIdentity{}, err
	}
	if row.Status == ServiceIdentityDisabled {
		if err := tx.Commit(ctx); err != nil {
			return ServiceIdentity{}, err
		}
		return row, nil
	}
	row, err = scanServiceIdentity(tx.QueryRow(ctx, `UPDATE service_identities SET status='DISABLED', disabled_by=$2, disabled_at=NOW(), disable_reason=$3 WHERE service_id=$1 RETURNING `+serviceIdentityColumns, serviceID, disabledBy, reason))
	if err != nil {
		return ServiceIdentity{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO admin_audit_logs (admin_id, action, target_type, target_id, reason) VALUES ($1, 'service_identity_disable', 'service_identity', $2, $3)`, disabledBy, serviceID, reason); err != nil {
		return ServiceIdentity{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ServiceIdentity{}, err
	}
	return row, nil
}

func (s *PostgresStore) ConsumeServiceNonce(serviceID, nonce string, expiresAt time.Time) error {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer rollback(ctx, tx)
	_, _ = tx.Exec(ctx, `DELETE FROM service_request_nonces WHERE expires_at <= NOW()`)
	if _, err = tx.Exec(ctx, `INSERT INTO service_request_nonces (service_id, nonce, expires_at) VALUES ($1, $2, $3)`, serviceID, nonce, expiresAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return ErrForbidden
		}
		return err
	}
	return tx.Commit(ctx)
}
