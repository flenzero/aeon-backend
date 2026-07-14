package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const npcRecycleRetention = 7 * 24 * time.Hour

type EquipmentNPCRecycleRequest struct {
	OpID         string
	AccountID    int64
	CharacterID  int64
	EquipmentUID string
	GoldCredit   int64
}

type EquipmentNPCRecycleResult struct {
	GoldCredit int64           `json:"goldCredit"`
	ExpiresAt  time.Time       `json:"expiresAt"`
	Snapshot   EconomySnapshot `json:"snapshot"`
}

// EquipmentNPCRecycle preserves the row only for the fixed seven-day admin
// recovery window. NFT-linked equipment is intentionally never eligible.
func (s *PostgresStore) EquipmentNPCRecycle(req EquipmentNPCRecycleRequest) (EquipmentNPCRecycleResult, error) {
	uid := strings.TrimSpace(req.EquipmentUID)
	if uid == "" || req.GoldCredit < 0 {
		return EquipmentNPCRecycleResult{}, errors.New("invalid npc recycle request")
	}
	return runIdempotentAction(s, "equipment_npc_recycle", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (EquipmentNPCRecycleResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return EquipmentNPCRecycleResult{}, err
		}
		expiresAt := time.Now().UTC().Add(npcRecycleRetention)
		tag, err := tx.Exec(ctx, `
			UPDATE equipment_items e
			SET location = 'NPC_RECYCLED', slot = NULL, equip_slot = NULL,
				npc_recycled_at = NOW(), npc_recycle_expires_at = $4, updated_at = NOW()
			WHERE e.account_id = $1 AND e.character_id = $2 AND e.equipment_uid = $3
				AND e.location = 'IN_BAG'
				AND NOT EXISTS (
					SELECT 1 FROM nft_assets n
					WHERE n.source_asset_type = 'EQUIPMENT' AND n.source_asset_id = e.id
						AND n.status IN ('MINT_REQUESTED', 'MINTED')
				)
		`, req.AccountID, req.CharacterID, uid, expiresAt)
		if err != nil {
			return EquipmentNPCRecycleResult{}, err
		}
		if tag.RowsAffected() == 0 {
			return EquipmentNPCRecycleResult{}, ErrForbidden
		}
		if req.GoldCredit > 0 {
			tag, err = tx.Exec(ctx, `
				UPDATE character_wallets SET gold = gold + $2, updated_at = NOW()
				WHERE character_id = $1
			`, req.CharacterID, req.GoldCredit)
			if err != nil {
				return EquipmentNPCRecycleResult{}, err
			}
			if tag.RowsAffected() == 0 {
				return EquipmentNPCRecycleResult{}, errors.New("character wallet is missing")
			}
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "EQUIPMENT_NPC_RECYCLED", uid, req.GoldCredit, req.OpID); err != nil {
			return EquipmentNPCRecycleResult{}, err
		}
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		if err != nil {
			return EquipmentNPCRecycleResult{}, err
		}
		return EquipmentNPCRecycleResult{GoldCredit: req.GoldCredit, ExpiresAt: expiresAt, Snapshot: snapshot}, nil
	})
}

// PurgeExpiredNPCRecycledEquipment is a worker operation. Its ledger record
// remains, but the equipment row is physically removed after the recovery
// window and therefore cannot be restored.
func (s *PostgresStore) PurgeExpiredNPCRecycledEquipment(now time.Time, limit int) (int64, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer rollback(ctx, tx)
	var purged int64
	{
		rows, err := tx.Query(ctx, `
			SELECT id FROM equipment_items
			WHERE location = 'NPC_RECYCLED' AND npc_recycle_expires_at <= $1
			ORDER BY npc_recycle_expires_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT $2
		`, now, limit)
		if err != nil {
			return 0, err
		}
		defer rows.Close()
		ids := make([]int64, 0, limit)
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return 0, err
			}
			ids = append(ids, id)
		}
		if err := rows.Err(); err != nil {
			return 0, err
		}
		for _, id := range ids {
			tag, err := tx.Exec(ctx, `DELETE FROM equipment_items WHERE id = $1 AND location = 'NPC_RECYCLED'`, id)
			if err != nil {
				return 0, err
			}
			purged += tag.RowsAffected()
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("purge npc recycled equipment: %w", err)
	}
	return purged, nil
}
