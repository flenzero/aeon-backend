package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// EquipmentEnhanceRequest is prepared by economy rules so the store merely
// executes the configured cost atomically with the semantic affix update.
type EquipmentEnhanceRequest struct {
	OpID          string
	AccountID     int64
	CharacterID   int64
	EquipmentUID  string
	MaxLevel      int
	GoldCost      int64
	StoneItemID   string
	StoneQuantity int64
}

type EquipmentEnhanceResult struct {
	Equipment     EquipmentItem   `json:"equipment"`
	GoldCost      int64           `json:"goldCost"`
	StoneItemID   string          `json:"stoneItemId,omitempty"`
	StoneQuantity int64           `json:"stoneQuantity,omitempty"`
	EnhancedAffix EquipmentAffix  `json:"enhancedAffix"`
	Snapshot      EconomySnapshot `json:"snapshot"`
}

func (s *PostgresStore) EquipmentEnhance(req EquipmentEnhanceRequest) (EquipmentEnhanceResult, error) {
	uid := strings.TrimSpace(req.EquipmentUID)
	if uid == "" {
		return EquipmentEnhanceResult{}, errors.New("equipmentUid is required")
	}
	if req.MaxLevel <= 0 || req.GoldCost < 0 || req.StoneQuantity < 0 {
		return EquipmentEnhanceResult{}, errors.New("invalid enhancement rule")
	}
	if req.StoneQuantity > 0 && strings.TrimSpace(req.StoneItemID) == "" {
		return EquipmentEnhanceResult{}, errors.New("enhancement stone item is required")
	}
	return runIdempotentAction(s, "equipment_enhance", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (EquipmentEnhanceResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return EquipmentEnhanceResult{}, err
		}
		var id int64
		var level int
		var affixJSON []byte
		err := tx.QueryRow(ctx, `
			SELECT e.id, e.enhance_level, e.affixes
			FROM equipment_items e
			WHERE e.equipment_uid = $1 AND e.account_id = $2 AND e.character_id = $3
				AND e.location IN ('IN_BAG', 'IN_WAREHOUSE')
				AND NOT EXISTS (
					SELECT 1 FROM nft_assets n
					WHERE n.source_asset_type = 'EQUIPMENT' AND n.source_asset_id = e.id
						AND n.status IN ('MINT_REQUESTED', 'MINTED')
				)
			FOR UPDATE
		`, uid, req.AccountID, req.CharacterID).Scan(&id, &level, &affixJSON)
		if errors.Is(err, pgx.ErrNoRows) {
			return EquipmentEnhanceResult{}, ErrForbidden
		}
		if err != nil {
			return EquipmentEnhanceResult{}, err
		}
		nextLevel := level + 1
		if nextLevel > req.MaxLevel {
			return EquipmentEnhanceResult{}, fmt.Errorf("equipment is already at enhancement cap +%d", req.MaxLevel)
		}
		var affixes []EquipmentAffix
		if err := json.Unmarshal(affixJSON, &affixes); err != nil {
			return EquipmentEnhanceResult{}, fmt.Errorf("decode equipment affixes: %w", err)
		}
		if len(affixes) == 0 {
			return EquipmentEnhanceResult{}, errors.New("equipment has no affixes to enhance")
		}
		if req.GoldCost > 0 {
			tag, err := tx.Exec(ctx, `
				UPDATE character_wallets
				SET gold = gold - $2, updated_at = NOW()
				WHERE character_id = $1 AND gold >= $2
			`, req.CharacterID, req.GoldCost)
			if err != nil {
				return EquipmentEnhanceResult{}, err
			}
			if tag.RowsAffected() == 0 {
				return EquipmentEnhanceResult{}, ErrInsufficientBalance
			}
		}
		if req.StoneQuantity > 0 {
			if err := s.consumeBagItem(ctx, tx, req.CharacterID, req.StoneItemID, req.StoneQuantity); err != nil {
				return EquipmentEnhanceResult{}, fmt.Errorf("enhancement stone: %w", err)
			}
		}
		index := deterministicAffixIndex(req.OpID, uid, len(affixes))
		affixes[index].EnhanceHits++
		if strings.TrimSpace(affixes[index].InstanceID) == "" {
			affixes[index].InstanceID = fmt.Sprintf("legacy-%d", index+1)
		}
		encoded, err := json.Marshal(affixes)
		if err != nil {
			return EquipmentEnhanceResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE equipment_items
			SET enhance_level = $2, affixes = $3, updated_at = NOW()
			WHERE id = $1
		`, id, nextLevel, encoded); err != nil {
			return EquipmentEnhanceResult{}, err
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "EQUIPMENT_ENHANCED", uid, int64(nextLevel), req.OpID); err != nil {
			return EquipmentEnhanceResult{}, err
		}
		equipment, err := s.loadEquipmentByUID(ctx, tx, uid)
		if err != nil {
			return EquipmentEnhanceResult{}, err
		}
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		if err != nil {
			return EquipmentEnhanceResult{}, err
		}
		return EquipmentEnhanceResult{Equipment: equipment, GoldCost: req.GoldCost, StoneItemID: req.StoneItemID, StoneQuantity: req.StoneQuantity, EnhancedAffix: affixes[index], Snapshot: snapshot}, nil
	})
}

func deterministicAffixIndex(opID, uid string, length int) int {
	if length <= 1 {
		return 0
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(opID))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(uid))
	return int(h.Sum64() % uint64(length))
}
