package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type AdminAccountDetail struct {
	ID                 int64      `json:"id"`
	Username           string     `json:"username"`
	WalletAddress      string     `json:"walletAddress"`
	Status             string     `json:"status"`
	IsBanned           bool       `json:"isBanned"`
	RiskLevel          int        `json:"riskLevel"`
	BanReason          string     `json:"banReason,omitempty"`
	HasTradingLicense  bool       `json:"hasTradingLicense"`
	TradingLicenseAt   *time.Time `json:"tradingLicenseAt,omitempty"`
	CreatedAt          time.Time  `json:"createdAt"`
	LastLoginAt        time.Time  `json:"lastLoginAt"`
	ActiveRestrictions int        `json:"activeRestrictions"`
}

type MarketRestriction struct {
	ID              int64      `json:"id"`
	AccountID       int64      `json:"accountId"`
	RestrictionType string     `json:"restrictionType"`
	Reason          string     `json:"reason,omitempty"`
	CreatedBy       string     `json:"createdBy,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	ExpiresAt       *time.Time `json:"expiresAt,omitempty"`
	RevokedAt       *time.Time `json:"revokedAt,omitempty"`
}

type CreateMarketRestrictionInput struct {
	AccountID       int64
	RestrictionType string
	Reason          string
	CreatedBy       string
	ExpiresAt       *time.Time
}

type RiskEvent struct {
	ID        int64          `json:"id"`
	AccountID int64          `json:"accountId,omitempty"`
	EventType string         `json:"eventType"`
	Severity  int            `json:"severity"`
	DeviceID  string         `json:"deviceId,omitempty"`
	IPAddress string         `json:"ipAddress,omitempty"`
	Wallet    string         `json:"wallet,omitempty"`
	Detail    map[string]any `json:"detail,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
}

type CreateRiskEventInput struct {
	AccountID int64
	EventType string
	Severity  int
	DeviceID  string
	IPAddress string
	Wallet    string
	Detail    map[string]any
}

type HotWalletStatus struct {
	Wallet              string     `json:"wallet"`
	Network             string     `json:"network"`
	TokenMint           string     `json:"tokenMint,omitempty"`
	Balance             int64      `json:"balance"`
	LowBalanceThreshold int64      `json:"lowBalanceThreshold"`
	PayoutsPaused       bool       `json:"payoutsPaused"`
	LastCheckedAt       *time.Time `json:"lastCheckedAt,omitempty"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

type AdminListFilter struct {
	AccountID int64
	Status    string
	Limit     int
	Offset    int
}

type AdminPaymentTrace struct {
	Order  PaymentOrder  `json:"order"`
	Ledger []LedgerEntry `json:"ledger"`
}

type AdminCharacterListFilter struct {
	Keyword           string
	AccountID         int64
	Wallet            string
	MinLevel          int
	MaxLevel          int
	HasTradingLicense *bool
	Status            string
	OnlineOnly        bool
	ServerID          string
	Limit             int
	Offset            int
}

type AdminCharacterSummary struct {
	CharacterID       int64     `json:"characterId"`
	AccountID         int64     `json:"accountId"`
	Name              string    `json:"name"`
	Level             int       `json:"level"`
	Exp               int64     `json:"exp"`
	WalletAddress     string    `json:"walletAddress"`
	AccountStatus     string    `json:"accountStatus"`
	RiskLevel         int       `json:"riskLevel"`
	HasTradingLicense bool      `json:"hasTradingLicense"`
	LastLoginAt       time.Time `json:"lastLoginAt"`
	Online            bool      `json:"online"`
	ServerID          string    `json:"serverId,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
}

type AdminCharacterCore struct {
	CharacterID           int64     `json:"characterId"`
	AccountID             int64     `json:"accountId"`
	Name                  string    `json:"name"`
	Level                 int       `json:"level"`
	Exp                   int64     `json:"exp"`
	Stamina               int       `json:"stamina"`
	BagSlots              int       `json:"bagSlots"`
	BagExpandCount        int       `json:"bagExpandCount"`
	HighestClearedChapter int       `json:"highestClearedChapter"`
	HighestClearedFloor   int       `json:"highestClearedFloor"`
	DungeonClearCount     int64     `json:"dungeonClearCount"`
	CreatedAt             time.Time `json:"createdAt"`
}

type AdminOnlineStatus struct {
	Online       bool       `json:"online"`
	ServerID     string     `json:"serverId,omitempty"`
	ConnectionID string     `json:"connectionId,omitempty"`
	SessionID    string     `json:"sessionId,omitempty"`
	LastSeenAt   *time.Time `json:"lastSeenAt,omitempty"`
}

type AdminCharacterDetail struct {
	Character AdminCharacterCore `json:"character"`
	Account   AdminAccountDetail `json:"account"`
	Economy   EconomySnapshot    `json:"economy"`
	Online    AdminOnlineStatus  `json:"online"`
}

type AdminCharacterTimelineFilter struct {
	Types  []string
	Limit  int
	Offset int
}

type AdminCharacterTimelineItem struct {
	Type      string         `json:"type"`
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Detail    string         `json:"detail,omitempty"`
	Amount    int64          `json:"amount,omitempty"`
	Ref       string         `json:"ref,omitempty"`
	Severity  int            `json:"severity,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	Raw       map[string]any `json:"raw,omitempty"`
}

type AdminCharacterTimelinePage struct {
	Items  []AdminCharacterTimelineItem `json:"items"`
	Count  int                          `json:"count"`
	Total  int                          `json:"total"`
	Limit  int                          `json:"limit"`
	Offset int                          `json:"offset"`
}

type AdminEquipmentOwner struct {
	AccountID     int64  `json:"accountId"`
	CharacterID   int64  `json:"characterId,omitempty"`
	CharacterName string `json:"characterName,omitempty"`
	WalletAddress string `json:"walletAddress"`
	AccountStatus string `json:"accountStatus"`
}

type AdminEquipmentNFTDetail struct {
	AssetID       int64      `json:"assetId,omitempty"`
	Status        string     `json:"status,omitempty"`
	MintAddress   string     `json:"mintAddress,omitempty"`
	MetadataURI   string     `json:"metadataUri,omitempty"`
	RequestID     int64      `json:"requestId,omitempty"`
	RequestStatus string     `json:"requestStatus,omitempty"`
	TxSignature   string     `json:"txSignature,omitempty"`
	CreatedAt     *time.Time `json:"createdAt,omitempty"`
	MintedAt      *time.Time `json:"mintedAt,omitempty"`
	SubmittedAt   *time.Time `json:"submittedAt,omitempty"`
	ConfirmedAt   *time.Time `json:"confirmedAt,omitempty"`
}

type AdminEquipmentDetail struct {
	Equipment   EquipmentItem            `json:"equipment"`
	Owner       AdminEquipmentOwner      `json:"owner"`
	NFT         *AdminEquipmentNFTDetail `json:"nft,omitempty"`
	Marketplace *MarketplaceListing      `json:"marketplace,omitempty"`
}

type AdminCompensationPreviewDetail struct {
	PreviewID   string                   `json:"previewId"`
	Status      string                   `json:"status"`
	TargetCount int                      `json:"targetCount"`
	ExpiresAt   time.Time                `json:"expiresAt"`
	Filters     AdminCompensationFilter  `json:"filters"`
	Rewards     AdminCompensationRewards `json:"rewards"`
	CreatedAt   time.Time                `json:"createdAt"`
	CommittedAt *time.Time               `json:"committedAt,omitempty"`
}

type AdminCompensationTargetPage struct {
	Items  []AdminCompensationTarget `json:"items"`
	Count  int                       `json:"count"`
	Total  int                       `json:"total"`
	Limit  int                       `json:"limit"`
	Offset int                       `json:"offset"`
}

// AdminOperation stores the successful response of a super-admin write.  A
// repeated opId returns this exact response instead of applying the mutation
// and writing another audit record.
type AdminOperation struct {
	OpID      string          `json:"opId"`
	AdminID   string          `json:"adminId"`
	Action    string          `json:"action"`
	Target    string          `json:"target"`
	Response  json.RawMessage `json:"-"`
	CreatedAt time.Time       `json:"createdAt"`
}

// AdminRewardGrant is the Aeonblight-native replacement for a generic ops
// reward: Gold belongs to a Character while AEB belongs to its Account.
// Items and Equipment are placed into the Loot Tray so no existing inventory
// capacity is displaced by an administrator action.
type AdminRewardGrant struct {
	OpID            string
	AdminID         string
	CharacterID     int64
	Reason          string
	Gold            int64
	WithdrawableAEB int64
	LockedAEB       int64
	Items           []DungeonRewardGrant
}

type AdminRewardGrantResult struct {
	Snapshot EconomySnapshot       `json:"snapshot"`
	Items    appliedDungeonRewards `json:"items"`
}

func clampAdminLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 200 {
		return 200
	}
	return limit
}

func (s *PostgresStore) AdminOperation(opID string) (AdminOperation, error) {
	var row AdminOperation
	err := s.pool.QueryRow(context.Background(), `
		SELECT op_id, admin_id, action, target, response, created_at
		FROM admin_operation_logs
		WHERE op_id = $1
	`, strings.TrimSpace(opID)).Scan(&row.OpID, &row.AdminID, &row.Action, &row.Target, &row.Response, &row.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminOperation{}, ErrNotFound
	}
	return row, err
}

func (s *PostgresStore) SaveAdminOperation(in AdminOperation) (AdminOperation, error) {
	if strings.TrimSpace(in.OpID) == "" {
		return AdminOperation{}, errors.New("opId is required")
	}
	if len(in.Response) == 0 || !json.Valid(in.Response) {
		return AdminOperation{}, errors.New("admin operation response is invalid")
	}
	var row AdminOperation
	err := s.pool.QueryRow(context.Background(), `
		INSERT INTO admin_operation_logs (op_id, admin_id, action, target, response)
		VALUES ($1, $2, $3, $4, $5::jsonb)
		ON CONFLICT (op_id) DO NOTHING
		RETURNING op_id, admin_id, action, target, response, created_at
	`, strings.TrimSpace(in.OpID), strings.TrimSpace(in.AdminID), strings.TrimSpace(in.Action), strings.TrimSpace(in.Target), in.Response).Scan(
		&row.OpID, &row.AdminID, &row.Action, &row.Target, &row.Response, &row.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return s.AdminOperation(in.OpID)
	}
	return row, err
}

func (s *PostgresStore) AdminGrantRewards(req AdminRewardGrant) (AdminRewardGrantResult, error) {
	if req.CharacterID <= 0 || strings.TrimSpace(req.AdminID) == "" || strings.TrimSpace(req.Reason) == "" {
		return AdminRewardGrantResult{}, errors.New("characterId, adminId, and reason are required")
	}
	if req.Gold < 0 || req.WithdrawableAEB < 0 || req.LockedAEB < 0 {
		return AdminRewardGrantResult{}, errors.New("reward amounts must be non-negative")
	}
	if req.Gold == 0 && req.WithdrawableAEB == 0 && req.LockedAEB == 0 && len(req.Items) == 0 {
		return AdminRewardGrantResult{}, errors.New("at least one reward is required")
	}
	var accountID int64
	err := s.pool.QueryRow(context.Background(), `
		SELECT account_id FROM characters WHERE id = $1 AND is_deleted = FALSE
	`, req.CharacterID).Scan(&accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminRewardGrantResult{}, ErrNotFound
	}
	if err != nil {
		return AdminRewardGrantResult{}, err
	}
	return runIdempotentAction(s, "admin_reward_grant", req.OpID, accountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (AdminRewardGrantResult, error) {
		if err := s.lockCharacter(ctx, tx, accountID, req.CharacterID); err != nil {
			return AdminRewardGrantResult{}, err
		}
		if req.Gold > 0 {
			if _, err := tx.Exec(ctx, `INSERT INTO character_wallets (character_id) VALUES ($1) ON CONFLICT (character_id) DO NOTHING`, req.CharacterID); err != nil {
				return AdminRewardGrantResult{}, err
			}
			if _, err := tx.Exec(ctx, `UPDATE character_wallets SET gold = gold + $2, updated_at = NOW() WHERE character_id = $1`, req.CharacterID, req.Gold); err != nil {
				return AdminRewardGrantResult{}, err
			}
			if err := s.insertEconomyLedger(ctx, tx, accountID, req.CharacterID, "ADMIN_GRANT_GOLD", "admin:"+req.OpID, req.Gold, req.OpID); err != nil {
				return AdminRewardGrantResult{}, err
			}
		}
		if req.WithdrawableAEB > 0 || req.LockedAEB > 0 {
			if _, err := tx.Exec(ctx, `INSERT INTO account_tokens (account_id) VALUES ($1) ON CONFLICT (account_id) DO NOTHING`, accountID); err != nil {
				return AdminRewardGrantResult{}, err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE account_tokens
				SET token_balance = token_balance + $2 + $3,
					withdrawable_balance = withdrawable_balance + $2,
					locked_balance = locked_balance + $3,
					updated_at = NOW()
				WHERE account_id = $1
			`, accountID, req.WithdrawableAEB, req.LockedAEB); err != nil {
				return AdminRewardGrantResult{}, err
			}
			if req.WithdrawableAEB > 0 {
				if err := s.insertEconomyLedger(ctx, tx, accountID, req.CharacterID, "ADMIN_GRANT_AEB_WITHDRAWABLE", "admin:"+req.OpID, req.WithdrawableAEB, req.OpID); err != nil {
					return AdminRewardGrantResult{}, err
				}
			}
			if req.LockedAEB > 0 {
				if err := s.insertEconomyLedger(ctx, tx, accountID, req.CharacterID, "ADMIN_GRANT_AEB_LOCKED", "admin:"+req.OpID, req.LockedAEB, req.OpID); err != nil {
					return AdminRewardGrantResult{}, err
				}
			}
		}
		items, err := s.applyTrayRewards(ctx, tx, accountID, req.CharacterID, req.OpID, req.OpID, "admin_grant", "admin_grant", DungeonRewardPlan{Items: req.Items})
		if err != nil {
			return AdminRewardGrantResult{}, err
		}
		if len(req.Items) > 0 {
			if err := s.insertEconomyLedger(ctx, tx, accountID, req.CharacterID, "ADMIN_GRANT_ITEMS", "admin:"+req.OpID, int64(len(req.Items)), req.OpID); err != nil {
				return AdminRewardGrantResult{}, err
			}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO admin_audit_logs (admin_id, action, target_type, target_id, reason)
			VALUES ($1, 'admin_reward_grant', 'character', $2, $3)
		`, req.AdminID, fmt.Sprint(req.CharacterID), req.Reason); err != nil {
			return AdminRewardGrantResult{}, err
		}
		snapshot, err := s.economySnapshot(ctx, tx, accountID, req.CharacterID)
		if err != nil {
			return AdminRewardGrantResult{}, err
		}
		return AdminRewardGrantResult{Snapshot: snapshot, Items: items}, nil
	})
}

func (s *PostgresStore) AdminCharacterLevel(characterID int64) (int, error) {
	var level int
	err := s.pool.QueryRow(context.Background(), `SELECT level FROM characters WHERE id = $1 AND is_deleted = FALSE`, characterID).Scan(&level)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return level, err
}

func (s *PostgresStore) AdminPaymentTrace(orderID string) (AdminPaymentTrace, error) {
	var out AdminPaymentTrace
	var characterID *int64
	var receiver, signature *string
	err := s.pool.QueryRow(context.Background(), `
		SELECT id::text,account_id,character_id,purpose,pay_asset,amount::bigint,receiver_wallet,status,tx_signature,created_at,expires_at,submitted_at,confirmed_at,fulfilled_at
		FROM economy_payment_orders WHERE id=$1::uuid
	`, strings.TrimSpace(orderID)).Scan(&out.Order.ID, &out.Order.AccountID, &characterID, &out.Order.Purpose, &out.Order.PayAsset, &out.Order.Amount, &receiver, &out.Order.Status, &signature, &out.Order.CreatedAt, &out.Order.ExpiresAt, &out.Order.SubmittedAt, &out.Order.ConfirmedAt, &out.Order.FulfilledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminPaymentTrace{}, ErrNotFound
	}
	if err != nil {
		return AdminPaymentTrace{}, err
	}
	if characterID != nil {
		out.Order.CharacterID = *characterID
	}
	if receiver != nil {
		out.Order.ReceiverWallet = *receiver
	}
	if signature != nil {
		out.Order.TxSignature = *signature
	}
	rows, err := s.pool.Query(context.Background(), `SELECT id,COALESCE(account_id,0),kind,COALESCE(amount::bigint,0),COALESCE(ref_id,''),COALESCE(reason,''),created_at FROM economy_ledger WHERE ref_id=$1 ORDER BY id`, out.Order.ID)
	if err != nil {
		return AdminPaymentTrace{}, err
	}
	defer rows.Close()
	out.Ledger = []LedgerEntry{}
	for rows.Next() {
		var row LedgerEntry
		if err := rows.Scan(&row.ID, &row.AccountID, &row.Kind, &row.Amount, &row.Ref, &row.Detail, &row.CreatedAt); err != nil {
			return AdminPaymentTrace{}, err
		}
		out.Ledger = append(out.Ledger, row)
	}
	return out, rows.Err()
}

func normalizeRestrictionType(v string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(v)) {
	case "BUY", "SELL", "ALL":
		return strings.ToUpper(strings.TrimSpace(v)), nil
	default:
		return "", errors.New("restrictionType must be BUY, SELL, or ALL")
	}
}

func (s *PostgresStore) AdminGetAccount(accountID int64, wallet string) (AdminAccountDetail, error) {
	ctx := context.Background()
	wallet = strings.TrimSpace(wallet)
	var row AdminAccountDetail
	var status string
	var banReason *string
	var licenseAt *time.Time
	var err error
	if accountID > 0 {
		err = s.pool.QueryRow(ctx, `
			SELECT a.id, a.username, COALESCE(a.solana_wallet_address, ''), a.status, a.risk_level,
				a.ban_reason, a.has_trading_license, a.trading_license_at, a.created_at,
				COALESCE(a.last_login_at, a.created_at),
				(
					SELECT COUNT(*)::int FROM account_market_restrictions r
					WHERE r.account_id = a.id
						AND r.revoked_at IS NULL
						AND (r.expires_at IS NULL OR r.expires_at > NOW())
				)
			FROM accounts a
			WHERE a.id = $1
		`, accountID).Scan(
			&row.ID, &row.Username, &row.WalletAddress, &status, &row.RiskLevel,
			&banReason, &row.HasTradingLicense, &licenseAt, &row.CreatedAt,
			&row.LastLoginAt, &row.ActiveRestrictions,
		)
	} else if wallet != "" {
		err = s.pool.QueryRow(ctx, `
			SELECT a.id, a.username, COALESCE(a.solana_wallet_address, ''), a.status, a.risk_level,
				a.ban_reason, a.has_trading_license, a.trading_license_at, a.created_at,
				COALESCE(a.last_login_at, a.created_at),
				(
					SELECT COUNT(*)::int FROM account_market_restrictions r
					WHERE r.account_id = a.id
						AND r.revoked_at IS NULL
						AND (r.expires_at IS NULL OR r.expires_at > NOW())
				)
			FROM accounts a
			WHERE a.solana_wallet_address = $1
		`, wallet).Scan(
			&row.ID, &row.Username, &row.WalletAddress, &status, &row.RiskLevel,
			&banReason, &row.HasTradingLicense, &licenseAt, &row.CreatedAt,
			&row.LastLoginAt, &row.ActiveRestrictions,
		)
	} else {
		return AdminAccountDetail{}, errors.New("accountId or wallet is required")
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminAccountDetail{}, ErrNotFound
	}
	if err != nil {
		return AdminAccountDetail{}, err
	}
	row.Status = status
	row.IsBanned = status == "BANNED"
	row.TradingLicenseAt = licenseAt
	if banReason != nil {
		row.BanReason = *banReason
	}
	return row, nil
}

func (s *PostgresStore) SetBanned(accountID int64, banned bool) error {
	return s.SetAccountBan(accountID, banned, "")
}

func (s *PostgresStore) SetAccountBan(accountID int64, banned bool, reason string) error {
	status := "ACTIVE"
	var banReason any
	if banned {
		status = "BANNED"
		if strings.TrimSpace(reason) != "" {
			banReason = strings.TrimSpace(reason)
		}
	} else {
		banReason = nil
	}
	tag, err := s.pool.Exec(context.Background(), `
		UPDATE accounts
		SET status = $1,
			ban_reason = $2,
			updated_at = NOW()
		WHERE id = $3
	`, status, banReason, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) SetAccountRiskLevel(accountID int64, riskLevel int) error {
	if riskLevel < 0 {
		return errors.New("riskLevel must be >= 0")
	}
	tag, err := s.pool.Exec(context.Background(), `
		UPDATE accounts SET risk_level = $1, updated_at = NOW() WHERE id = $2
	`, riskLevel, accountID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) SetTradingLicense(accountID int64, granted bool) error {
	ctx := context.Background()
	var err error
	var rows int64
	if granted {
		tag, e := s.pool.Exec(ctx, `
			UPDATE accounts
			SET has_trading_license = TRUE,
				trading_license_at = COALESCE(trading_license_at, NOW()),
				updated_at = NOW()
			WHERE id = $1
		`, accountID)
		err = e
		if e == nil {
			rows = tag.RowsAffected()
		}
	} else {
		tag, e := s.pool.Exec(ctx, `
			UPDATE accounts
			SET has_trading_license = FALSE,
				trading_license_at = NULL,
				updated_at = NOW()
			WHERE id = $1
		`, accountID)
		err = e
		if e == nil {
			rows = tag.RowsAffected()
		}
	}
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) CreateMarketRestriction(in CreateMarketRestrictionInput) (MarketRestriction, error) {
	rtype, err := normalizeRestrictionType(in.RestrictionType)
	if err != nil {
		return MarketRestriction{}, err
	}
	if in.AccountID <= 0 {
		return MarketRestriction{}, errors.New("accountId is required")
	}
	var row MarketRestriction
	var expiresAt *time.Time
	err = s.pool.QueryRow(context.Background(), `
		INSERT INTO account_market_restrictions (
			account_id, restriction_type, reason, created_by, expires_at
		) VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, ''), $5)
		RETURNING id, account_id, restriction_type, COALESCE(reason, ''), COALESCE(created_by, ''),
			created_at, expires_at, revoked_at
	`, in.AccountID, rtype, strings.TrimSpace(in.Reason), strings.TrimSpace(in.CreatedBy), in.ExpiresAt).Scan(
		&row.ID, &row.AccountID, &row.RestrictionType, &row.Reason, &row.CreatedBy,
		&row.CreatedAt, &expiresAt, &row.RevokedAt,
	)
	if err != nil {
		return MarketRestriction{}, err
	}
	row.ExpiresAt = expiresAt
	return row, nil
}

func (s *PostgresStore) ListMarketRestrictions(accountID int64, activeOnly bool, limit, offset int) ([]MarketRestriction, error) {
	limit = clampAdminLimit(limit)
	if offset < 0 {
		offset = 0
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, account_id, restriction_type, COALESCE(reason, ''), COALESCE(created_by, ''),
			created_at, expires_at, revoked_at
		FROM account_market_restrictions
		WHERE ($1::bigint = 0 OR account_id = $1)
			AND (
				NOT $2::bool
				OR (revoked_at IS NULL AND (expires_at IS NULL OR expires_at > NOW()))
			)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, accountID, activeOnly, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MarketRestriction
	for rows.Next() {
		var row MarketRestriction
		if err := rows.Scan(
			&row.ID, &row.AccountID, &row.RestrictionType, &row.Reason, &row.CreatedBy,
			&row.CreatedAt, &row.ExpiresAt, &row.RevokedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) RevokeMarketRestriction(id int64, adminID, reason string) (MarketRestriction, error) {
	if id <= 0 {
		return MarketRestriction{}, errors.New("id is required")
	}
	var row MarketRestriction
	err := s.pool.QueryRow(context.Background(), `
		UPDATE account_market_restrictions
		SET revoked_at = NOW(),
			reason = CASE
				WHEN NULLIF($2, '') IS NULL THEN reason
				ELSE COALESCE(reason || ' | ', '') || 'revoked: ' || $2
			END
		WHERE id = $1 AND revoked_at IS NULL
		RETURNING id, account_id, restriction_type, COALESCE(reason, ''), COALESCE(created_by, ''),
			created_at, expires_at, revoked_at
	`, id, strings.TrimSpace(reason)).Scan(
		&row.ID, &row.AccountID, &row.RestrictionType, &row.Reason, &row.CreatedBy,
		&row.CreatedAt, &row.ExpiresAt, &row.RevokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return MarketRestriction{}, ErrNotFound
	}
	if err != nil {
		return MarketRestriction{}, err
	}
	_ = adminID
	return row, nil
}

func (s *PostgresStore) CreateRiskEvent(in CreateRiskEventInput) (RiskEvent, error) {
	eventType := strings.TrimSpace(in.EventType)
	if eventType == "" {
		return RiskEvent{}, errors.New("eventType is required")
	}
	if in.Severity < 0 {
		return RiskEvent{}, errors.New("severity must be >= 0")
	}
	detail := in.Detail
	if detail == nil {
		detail = map[string]any{}
	}
	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return RiskEvent{}, err
	}
	var accountID any
	if in.AccountID > 0 {
		accountID = in.AccountID
	}
	var row RiskEvent
	var detailRaw []byte
	err = s.pool.QueryRow(context.Background(), `
		INSERT INTO account_risk_events (
			account_id, event_type, severity, device_id, ip_address, wallet, detail
		) VALUES ($1, $2, $3, NULLIF($4, ''), CAST(NULLIF($5, '') AS inet), NULLIF($6, ''), $7::jsonb)
		RETURNING id, COALESCE(account_id, 0), event_type, severity, COALESCE(device_id, ''),
			COALESCE(host(ip_address)::text, ''), COALESCE(wallet, ''), detail, created_at
	`, accountID, eventType, in.Severity, strings.TrimSpace(in.DeviceID), strings.TrimSpace(in.IPAddress), strings.TrimSpace(in.Wallet), detailJSON).Scan(
		&row.ID, &row.AccountID, &row.EventType, &row.Severity, &row.DeviceID,
		&row.IPAddress, &row.Wallet, &detailRaw, &row.CreatedAt,
	)
	if err != nil {
		return RiskEvent{}, err
	}
	_ = json.Unmarshal(detailRaw, &row.Detail)
	return row, nil
}

func (s *PostgresStore) ListRiskEvents(accountID int64, limit, offset int) ([]RiskEvent, error) {
	limit = clampAdminLimit(limit)
	if offset < 0 {
		offset = 0
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, COALESCE(account_id, 0), event_type, severity, COALESCE(device_id, ''),
			COALESCE(host(ip_address)::text, ''), COALESCE(wallet, ''), detail, created_at
		FROM account_risk_events
		WHERE ($1::bigint = 0 OR account_id = $1)
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RiskEvent
	for rows.Next() {
		var row RiskEvent
		var detailRaw []byte
		if err := rows.Scan(
			&row.ID, &row.AccountID, &row.EventType, &row.Severity, &row.DeviceID,
			&row.IPAddress, &row.Wallet, &detailRaw, &row.CreatedAt,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(detailRaw, &row.Detail)
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ListAudits(limit, offset int) ([]AuditEntry, error) {
	limit = clampAdminLimit(limit)
	if offset < 0 {
		offset = 0
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, COALESCE(admin_id, ''), action, target_type,
			COALESCE(target_id, ''), COALESCE(reason, ''), created_at
		FROM admin_audit_logs
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []AuditEntry
	for rows.Next() {
		var row AuditEntry
		var targetID string
		if err := rows.Scan(&row.ID, &row.AdminID, &row.Action, &row.Target, &targetID, &row.Reason, &row.CreatedAt); err != nil {
			return nil, err
		}
		if targetID != "" {
			row.Target = row.Target + ":" + targetID
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) AuditTarget(adminID, action, targetType, targetID, reason string) AuditEntry {
	var row AuditEntry
	err := s.pool.QueryRow(context.Background(), `
		INSERT INTO admin_audit_logs (admin_id, action, target_type, target_id, reason)
		VALUES ($1, $2, $3, NULLIF($4, ''), $5)
		RETURNING id, COALESCE(admin_id, ''), action, target_type, COALESCE(reason, ''), created_at
	`, adminID, action, targetType, targetID, reason).Scan(
		&row.ID, &row.AdminID, &row.Action, &row.Target, &row.Reason, &row.CreatedAt,
	)
	must(err, "insert audit target")
	if targetID != "" {
		row.Target = targetType + ":" + targetID
	}
	return row
}

func (s *PostgresStore) ListPaymentOrdersAdmin(filter AdminListFilter) ([]PaymentOrder, error) {
	limit := clampAdminLimit(filter.Limit)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	status := strings.TrimSpace(filter.Status)
	rows, err := s.pool.Query(context.Background(), `
		SELECT id::text, account_id, COALESCE(character_id, 0), purpose, pay_asset, amount::bigint,
			COALESCE(receiver_wallet, ''), status, COALESCE(tx_signature, ''),
			created_at, expires_at, submitted_at, confirmed_at, fulfilled_at
		FROM economy_payment_orders
		WHERE ($1::bigint = 0 OR account_id = $1)
			AND ($2 = '' OR status = $2)
		ORDER BY created_at DESC
		LIMIT $3 OFFSET $4
	`, filter.AccountID, status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PaymentOrder
	for rows.Next() {
		var row PaymentOrder
		if err := rows.Scan(
			&row.ID, &row.AccountID, &row.CharacterID, &row.Purpose, &row.PayAsset, &row.Amount,
			&row.ReceiverWallet, &row.Status, &row.TxSignature,
			&row.CreatedAt, &row.ExpiresAt, &row.SubmittedAt, &row.ConfirmedAt, &row.FulfilledAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ListNFTMintRequests(filter AdminListFilter) ([]NFTMintRequest, error) {
	limit := clampAdminLimit(filter.Limit)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	status := strings.TrimSpace(filter.Status)
	rows, err := s.pool.Query(context.Background(), `
		SELECT r.id, r.account_id, COALESCE(r.nft_asset_id, 0), r.source_asset_type, r.source_asset_id,
			r.mint_fee_token::bigint, r.status, COALESCE(r.tx_signature, ''), r.created_at,
			r.submitted_at, r.confirmed_at, COALESCE(e.equipment_uid, '')
		FROM nft_mint_requests r
		LEFT JOIN equipment_items e ON e.id = r.source_asset_id AND r.source_asset_type = 'EQUIPMENT'
		WHERE ($1::bigint = 0 OR r.account_id = $1)
			AND ($2 = '' OR r.status = $2)
		ORDER BY r.created_at DESC
		LIMIT $3 OFFSET $4
	`, filter.AccountID, status, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NFTMintRequest
	for rows.Next() {
		var row NFTMintRequest
		if err := rows.Scan(
			&row.ID, &row.AccountID, &row.NFTAssetID, &row.SourceAssetType, &row.SourceAssetID,
			&row.MintFeeToken, &row.Status, &row.TxSignature, &row.CreatedAt,
			&row.SubmittedAt, &row.ConfirmedAt, &row.EquipmentUID,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) GetHotWalletStatus(wallet string) (HotWalletStatus, error) {
	wallet = strings.TrimSpace(wallet)
	if wallet == "" {
		return HotWalletStatus{}, errors.New("wallet is required")
	}
	var row HotWalletStatus
	err := s.pool.QueryRow(context.Background(), `
		SELECT wallet, network, COALESCE(token_mint, ''), balance::bigint, low_balance_threshold::bigint,
			payouts_paused, last_checked_at, updated_at
		FROM hot_wallet_status
		WHERE wallet = $1
	`, wallet).Scan(
		&row.Wallet, &row.Network, &row.TokenMint, &row.Balance, &row.LowBalanceThreshold,
		&row.PayoutsPaused, &row.LastCheckedAt, &row.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return HotWalletStatus{}, ErrNotFound
	}
	if err != nil {
		return HotWalletStatus{}, err
	}
	return row, nil
}

func (s *PostgresStore) SetHotWalletPayoutsPaused(wallet, network, tokenMint string, paused bool) (HotWalletStatus, error) {
	wallet = strings.TrimSpace(wallet)
	if wallet == "" {
		return HotWalletStatus{}, errors.New("wallet is required")
	}
	if strings.TrimSpace(network) == "" {
		network = "solana-devnet"
	}
	var row HotWalletStatus
	err := s.pool.QueryRow(context.Background(), `
		INSERT INTO hot_wallet_status (wallet, network, token_mint, payouts_paused)
		VALUES ($1, $2, NULLIF($3, ''), $4)
		ON CONFLICT (wallet) DO UPDATE
		SET network = EXCLUDED.network,
			token_mint = COALESCE(EXCLUDED.token_mint, hot_wallet_status.token_mint),
			payouts_paused = EXCLUDED.payouts_paused,
			updated_at = NOW()
		RETURNING wallet, network, COALESCE(token_mint, ''), balance::bigint, low_balance_threshold::bigint,
			payouts_paused, last_checked_at, updated_at
	`, wallet, network, strings.TrimSpace(tokenMint), paused).Scan(
		&row.Wallet, &row.Network, &row.TokenMint, &row.Balance, &row.LowBalanceThreshold,
		&row.PayoutsPaused, &row.LastCheckedAt, &row.UpdatedAt,
	)
	if err != nil {
		return HotWalletStatus{}, err
	}
	return row, nil
}

func (s *PostgresStore) AdminRevokeAccountSessions(accountID int64) (int64, error) {
	if accountID <= 0 {
		return 0, errors.New("accountId is required")
	}
	tag, err := s.pool.Exec(context.Background(), `
		UPDATE account_sessions
		SET status = 'REVOKED', revoked_at = NOW()
		WHERE account_id = $1 AND status = 'ACTIVE'
	`, accountID)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// Ensure Audit still writes usable rows when only target type is provided.
func (s *PostgresStore) Audit(adminID, action, target, reason string) AuditEntry {
	parts := strings.SplitN(target, ":", 2)
	targetType := parts[0]
	targetID := ""
	if len(parts) == 2 {
		targetID = parts[1]
	}
	return s.AuditTarget(adminID, action, targetType, targetID, reason)
}

func adminErrRequiresPostgres(op string) error {
	return fmt.Errorf("%s requires postgres store", op)
}
