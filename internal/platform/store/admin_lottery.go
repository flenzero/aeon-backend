package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type AdminLotteryPreview struct {
	PreviewID   string            `json:"previewId"`
	CharacterID int64             `json:"characterId"`
	Rewards     DungeonRewardPlan `json:"rewards"`
	ExpiresAt   time.Time         `json:"expiresAt"`
}

type AdminLotteryCommit struct {
	PreviewID   string `json:"previewId"`
	OpID        string `json:"opId"`
	AdminID     string `json:"adminId"`
	CharacterID int64  `json:"characterId"`
	Reason      string `json:"reason"`
}

func (s *PostgresStore) CreateAdminLotteryPreview(adminID string, characterID int64, rewards DungeonRewardPlan) (AdminLotteryPreview, error) {
	if strings.TrimSpace(adminID) == "" || characterID <= 0 || len(rewards.Items) == 0 {
		return AdminLotteryPreview{}, errors.New("adminId, characterId, and lottery rewards are required")
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AdminLotteryPreview{}, err
	}
	defer rollback(ctx, tx)
	var exists int
	err = tx.QueryRow(ctx, `SELECT 1 FROM characters WHERE id=$1 AND is_deleted=FALSE`, characterID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminLotteryPreview{}, ErrNotFound
	}
	if err != nil {
		return AdminLotteryPreview{}, err
	}
	previewID, err := newPreviewID()
	if err != nil {
		return AdminLotteryPreview{}, err
	}
	expires := time.Now().UTC().Add(30 * time.Minute)
	payload, err := json.Marshal(rewards)
	if err != nil {
		return AdminLotteryPreview{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO admin_operation_previews (preview_id,admin_id,kind,character_id,payload,expires_at) VALUES ($1,$2,'LOTTERY',$3,$4,$5)`, previewID, adminID, characterID, payload, expires); err != nil {
		return AdminLotteryPreview{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AdminLotteryPreview{}, err
	}
	return AdminLotteryPreview{PreviewID: previewID, CharacterID: characterID, Rewards: rewards, ExpiresAt: expires}, nil
}

func (s *PostgresStore) CommitAdminLotteryPreview(req AdminLotteryCommit) (AdminRewardGrantResult, error) {
	if strings.TrimSpace(req.PreviewID) == "" || strings.TrimSpace(req.AdminID) == "" || req.CharacterID <= 0 || strings.TrimSpace(req.Reason) == "" {
		return AdminRewardGrantResult{}, errors.New("previewId, adminId, characterId, and reason are required")
	}
	return runIdempotentAction(s, "admin_lottery_commit", req.OpID, 0, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (AdminRewardGrantResult, error) {
		var adminID, status string
		var characterID int64
		var expires time.Time
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT admin_id,character_id,status,expires_at,payload FROM admin_operation_previews WHERE preview_id=$1 AND kind='LOTTERY' FOR UPDATE`, req.PreviewID).Scan(&adminID, &characterID, &status, &expires, &raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return AdminRewardGrantResult{}, ErrNotFound
		}
		if err != nil {
			return AdminRewardGrantResult{}, err
		}
		if adminID != req.AdminID || characterID != req.CharacterID {
			return AdminRewardGrantResult{}, ErrForbidden
		}
		if status != "PENDING" {
			return AdminRewardGrantResult{}, errors.New("lottery preview is no longer pending")
		}
		if !expires.After(time.Now().UTC()) {
			return AdminRewardGrantResult{}, errors.New("lottery preview has expired")
		}
		var plan DungeonRewardPlan
		if err := json.Unmarshal(raw, &plan); err != nil {
			return AdminRewardGrantResult{}, err
		}
		var accountID int64
		if err := tx.QueryRow(ctx, `SELECT account_id FROM characters WHERE id=$1 AND is_deleted=FALSE`, characterID).Scan(&accountID); errors.Is(err, pgx.ErrNoRows) {
			return AdminRewardGrantResult{}, ErrNotFound
		} else if err != nil {
			return AdminRewardGrantResult{}, err
		}
		grant := AdminRewardGrant{OpID: req.OpID, AdminID: req.AdminID, CharacterID: characterID, Reason: req.Reason, Items: plan.Items}
		if err := s.applyAdminRewardsTx(ctx, tx, accountID, grant, "admin_lottery_commit"); err != nil {
			return AdminRewardGrantResult{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE admin_operation_previews SET status='COMMITTED',committed_at=NOW() WHERE preview_id=$1`, req.PreviewID); err != nil {
			return AdminRewardGrantResult{}, err
		}
		snapshot, err := s.economySnapshot(ctx, tx, accountID, characterID)
		if err != nil {
			return AdminRewardGrantResult{}, err
		}
		return AdminRewardGrantResult{Snapshot: snapshot}, nil
	})
}
