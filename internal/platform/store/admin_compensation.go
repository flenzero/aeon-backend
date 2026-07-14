package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type AdminCompensationFilter struct {
	MinLevel          int        `json:"minLevel,omitempty"`
	MaxLevel          int        `json:"maxLevel,omitempty"`
	LastLoginFrom     *time.Time `json:"lastLoginFrom,omitempty"`
	LastLoginTo       *time.Time `json:"lastLoginTo,omitempty"`
	MinClearedChapter int        `json:"minClearedChapter,omitempty"`
	MinClearedFloor   int        `json:"minClearedFloor,omitempty"`
	MinClearCount     int64      `json:"minDungeonClearCount,omitempty"`
	HasLicense        *bool      `json:"hasTradingLicense,omitempty"`
}

type AdminCompensationRewards struct {
	Gold            int64                `json:"gold,omitempty"`
	WithdrawableAEB int64                `json:"withdrawableAeb,omitempty"`
	LockedAEB       int64                `json:"lockedAeb,omitempty"`
	Items           []DungeonRewardGrant `json:"items,omitempty"`
}

type AdminCompensationTarget struct {
	AccountID   int64     `json:"accountId"`
	CharacterID int64     `json:"characterId"`
	Name        string    `json:"name"`
	Level       int       `json:"level"`
	LastLoginAt time.Time `json:"lastLoginAt"`
}

type AdminCompensationPreview struct {
	PreviewID   string                    `json:"previewId"`
	TargetCount int                       `json:"targetCount"`
	ExpiresAt   time.Time                 `json:"expiresAt"`
	Filters     AdminCompensationFilter   `json:"filters"`
	Rewards     AdminCompensationRewards  `json:"rewards"`
	Items       []AdminCompensationTarget `json:"items"`
}

type AdminCompensationCommit struct {
	PreviewID string `json:"previewId"`
	OpID      string `json:"opId"`
	AdminID   string `json:"adminId"`
	Reason    string `json:"reason"`
}

type AdminCompensationResult struct {
	PreviewID string `json:"previewId"`
	Processed int    `json:"processed"`
}

type compensationPreviewPayload struct {
	Filters AdminCompensationFilter  `json:"filters"`
	Rewards AdminCompensationRewards `json:"rewards"`
}

func validateCompensationRewards(rewards AdminCompensationRewards) error {
	if rewards.Gold < 0 || rewards.WithdrawableAEB < 0 || rewards.LockedAEB < 0 {
		return errors.New("reward amounts must be non-negative")
	}
	if rewards.Gold == 0 && rewards.WithdrawableAEB == 0 && rewards.LockedAEB == 0 && len(rewards.Items) == 0 {
		return errors.New("at least one reward is required")
	}
	return nil
}

func newPreviewID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "preview_" + hex.EncodeToString(buf), nil
}

func compensationWhere(filter AdminCompensationFilter) (string, []any, error) {
	if filter.MinLevel < 0 || filter.MaxLevel < 0 || filter.MinClearCount < 0 || filter.MinClearedChapter < 0 || filter.MinClearedFloor < 0 {
		return "", nil, errors.New("compensation filters must be non-negative")
	}
	if filter.MaxLevel > 0 && filter.MaxLevel < filter.MinLevel {
		return "", nil, errors.New("maxLevel must be >= minLevel")
	}
	clauses := []string{"c.is_deleted = FALSE", "a.status = 'ACTIVE'"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.MinLevel > 0 {
		add("c.level >= $%d", filter.MinLevel)
	}
	if filter.MaxLevel > 0 {
		add("c.level <= $%d", filter.MaxLevel)
	}
	if filter.LastLoginFrom != nil {
		add("a.last_login_at >= $%d", filter.LastLoginFrom.UTC())
	}
	if filter.LastLoginTo != nil {
		add("a.last_login_at <= $%d", filter.LastLoginTo.UTC())
	}
	if filter.MinClearedChapter > 0 || filter.MinClearedFloor > 0 {
		args = append(args, filter.MinClearedChapter, filter.MinClearedFloor)
		n := len(args)
		clauses = append(clauses, fmt.Sprintf("(c.highest_cleared_chapter > $%d OR (c.highest_cleared_chapter = $%d AND c.highest_cleared_floor >= $%d))", n-1, n-1, n))
	}
	if filter.MinClearCount > 0 {
		add("c.dungeon_clear_count >= $%d", filter.MinClearCount)
	}
	if filter.HasLicense != nil {
		add("a.has_trading_license = $%d", *filter.HasLicense)
	}
	return strings.Join(clauses, " AND "), args, nil
}

func (s *PostgresStore) CreateAdminCompensationPreview(adminID string, filter AdminCompensationFilter, rewards AdminCompensationRewards) (AdminCompensationPreview, error) {
	if strings.TrimSpace(adminID) == "" {
		return AdminCompensationPreview{}, errors.New("adminId is required")
	}
	if err := validateCompensationRewards(rewards); err != nil {
		return AdminCompensationPreview{}, err
	}
	where, args, err := compensationWhere(filter)
	if err != nil {
		return AdminCompensationPreview{}, err
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return AdminCompensationPreview{}, err
	}
	defer rollback(ctx, tx)
	rows, err := tx.Query(ctx, `SELECT a.id, c.id, c.name, c.level, COALESCE(a.last_login_at, a.created_at) FROM characters c JOIN accounts a ON a.id=c.account_id WHERE `+where+` ORDER BY c.id`, args...)
	if err != nil {
		return AdminCompensationPreview{}, err
	}
	defer rows.Close()
	targets := []AdminCompensationTarget{}
	for rows.Next() {
		var target AdminCompensationTarget
		if err := rows.Scan(&target.AccountID, &target.CharacterID, &target.Name, &target.Level, &target.LastLoginAt); err != nil {
			return AdminCompensationPreview{}, err
		}
		targets = append(targets, target)
	}
	if err := rows.Err(); err != nil {
		return AdminCompensationPreview{}, err
	}
	if len(targets) == 0 {
		return AdminCompensationPreview{TargetCount: 0, Filters: filter, Rewards: rewards, Items: []AdminCompensationTarget{}}, nil
	}
	previewID, err := newPreviewID()
	if err != nil {
		return AdminCompensationPreview{}, err
	}
	expiresAt := time.Now().UTC().Add(30 * time.Minute)
	payload, err := json.Marshal(compensationPreviewPayload{Filters: filter, Rewards: rewards})
	if err != nil {
		return AdminCompensationPreview{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO admin_operation_previews (preview_id,admin_id,kind,payload,expires_at) VALUES ($1,$2,'COMPENSATION',$3,$4)`, previewID, adminID, payload, expiresAt); err != nil {
		return AdminCompensationPreview{}, err
	}
	for _, target := range targets {
		if _, err := tx.Exec(ctx, `INSERT INTO admin_operation_preview_targets (preview_id,account_id,character_id) VALUES ($1,$2,$3)`, previewID, target.AccountID, target.CharacterID); err != nil {
			return AdminCompensationPreview{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return AdminCompensationPreview{}, err
	}
	items := targets
	if len(items) > 50 {
		items = items[:50]
	}
	return AdminCompensationPreview{PreviewID: previewID, TargetCount: len(targets), ExpiresAt: expiresAt, Filters: filter, Rewards: rewards, Items: items}, nil
}

func (s *PostgresStore) applyAdminRewardsTx(ctx context.Context, tx pgx.Tx, accountID int64, req AdminRewardGrant, action string) error {
	if err := s.lockCharacter(ctx, tx, accountID, req.CharacterID); err != nil {
		return err
	}
	if req.Gold > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO character_wallets (character_id) VALUES ($1) ON CONFLICT (character_id) DO NOTHING`, req.CharacterID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE character_wallets SET gold=gold+$2,updated_at=NOW() WHERE character_id=$1`, req.CharacterID, req.Gold); err != nil {
			return err
		}
		if err := s.insertEconomyLedger(ctx, tx, accountID, req.CharacterID, "ADMIN_GRANT_GOLD", "admin:"+req.OpID, req.Gold, req.OpID); err != nil {
			return err
		}
	}
	if req.WithdrawableAEB > 0 || req.LockedAEB > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO account_tokens (account_id) VALUES ($1) ON CONFLICT (account_id) DO NOTHING`, accountID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE account_tokens SET token_balance=token_balance+$2+$3,withdrawable_balance=withdrawable_balance+$2,locked_balance=locked_balance+$3,updated_at=NOW() WHERE account_id=$1`, accountID, req.WithdrawableAEB, req.LockedAEB); err != nil {
			return err
		}
		if req.WithdrawableAEB > 0 {
			if err := s.insertEconomyLedger(ctx, tx, accountID, req.CharacterID, "ADMIN_GRANT_AEB_WITHDRAWABLE", "admin:"+req.OpID, req.WithdrawableAEB, req.OpID); err != nil {
				return err
			}
		}
		if req.LockedAEB > 0 {
			if err := s.insertEconomyLedger(ctx, tx, accountID, req.CharacterID, "ADMIN_GRANT_AEB_LOCKED", "admin:"+req.OpID, req.LockedAEB, req.OpID); err != nil {
				return err
			}
		}
	}
	items := append([]DungeonRewardGrant(nil), req.Items...)
	for index := range items {
		if items[index].EquipmentUID != "" {
			items[index].EquipmentUID = fmt.Sprintf("%s-c%d-%d", items[index].EquipmentUID, req.CharacterID, index)
		}
	}
	if _, err := s.applyTrayRewards(ctx, tx, accountID, req.CharacterID, req.OpID, req.OpID, "admin_grant", "admin_grant", DungeonRewardPlan{Items: items}); err != nil {
		return err
	}
	if len(items) > 0 {
		if err := s.insertEconomyLedger(ctx, tx, accountID, req.CharacterID, "ADMIN_GRANT_ITEMS", "admin:"+req.OpID, int64(len(items)), req.OpID); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `INSERT INTO admin_audit_logs (admin_id,action,target_type,target_id,reason) VALUES ($1,$2,'character',$3,$4)`, req.AdminID, action, fmt.Sprint(req.CharacterID), req.Reason)
	return err
}

func (s *PostgresStore) CommitAdminCompensation(req AdminCompensationCommit) (AdminCompensationResult, error) {
	if strings.TrimSpace(req.PreviewID) == "" || strings.TrimSpace(req.AdminID) == "" || strings.TrimSpace(req.Reason) == "" {
		return AdminCompensationResult{}, errors.New("previewId, adminId, and reason are required")
	}
	return runIdempotentAction(s, "admin_compensation_commit", req.OpID, 0, 0, req, func(ctx context.Context, tx pgx.Tx) (AdminCompensationResult, error) {
		var adminID, status string
		var expires time.Time
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT admin_id,status,expires_at,payload FROM admin_operation_previews WHERE preview_id=$1 AND kind='COMPENSATION' FOR UPDATE`, req.PreviewID).Scan(&adminID, &status, &expires, &raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return AdminCompensationResult{}, ErrNotFound
		}
		if err != nil {
			return AdminCompensationResult{}, err
		}
		if adminID != req.AdminID {
			return AdminCompensationResult{}, ErrForbidden
		}
		if status != "PENDING" {
			return AdminCompensationResult{}, errors.New("compensation preview is no longer pending")
		}
		if !expires.After(time.Now().UTC()) {
			return AdminCompensationResult{}, errors.New("compensation preview has expired")
		}
		var payload compensationPreviewPayload
		if err := json.Unmarshal(raw, &payload); err != nil {
			return AdminCompensationResult{}, err
		}
		processed := 0
		for offset := 0; ; offset += 250 {
			rows, err := tx.Query(ctx, `SELECT account_id,character_id FROM admin_operation_preview_targets WHERE preview_id=$1 ORDER BY character_id LIMIT 250 OFFSET $2`, req.PreviewID, offset)
			if err != nil {
				return AdminCompensationResult{}, err
			}
			batch := 0
			for rows.Next() {
				var accountID, characterID int64
				if err := rows.Scan(&accountID, &characterID); err != nil {
					rows.Close()
					return AdminCompensationResult{}, err
				}
				batch++
				grant := AdminRewardGrant{OpID: fmt.Sprintf("%s:%d", req.OpID, characterID), AdminID: req.AdminID, CharacterID: characterID, Reason: req.Reason, Gold: payload.Rewards.Gold, WithdrawableAEB: payload.Rewards.WithdrawableAEB, LockedAEB: payload.Rewards.LockedAEB, Items: payload.Rewards.Items}
				if err := s.applyAdminRewardsTx(ctx, tx, accountID, grant, "admin_compensation_grant"); err != nil {
					rows.Close()
					return AdminCompensationResult{}, err
				}
				processed++
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return AdminCompensationResult{}, err
			}
			rows.Close()
			if batch < 250 {
				break
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE admin_operation_previews SET status='COMMITTED',committed_at=NOW() WHERE preview_id=$1`, req.PreviewID); err != nil {
			return AdminCompensationResult{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO admin_audit_logs (admin_id,action,target_type,target_id,reason) VALUES ($1,'admin_compensation_commit','compensation_preview',$2,$3)`, req.AdminID, req.PreviewID, req.Reason); err != nil {
			return AdminCompensationResult{}, err
		}
		return AdminCompensationResult{PreviewID: req.PreviewID, Processed: processed}, nil
	})
}
