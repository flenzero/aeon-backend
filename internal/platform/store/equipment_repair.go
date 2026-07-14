package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

type EquipmentRules struct {
	DefaultMaxDurability    int
	RepairCostPerPointToken int64
	RepairMinCostToken      int64
	RepairBurnBps           int64
	RepairRecycleBps        int64
	RepairRewardBps         int64
	DungeonWearPoints       int
	BossWearPoints          int
}

func (r EquipmentRules) WithDefaults() EquipmentRules {
	if r.DefaultMaxDurability <= 0 {
		r.DefaultMaxDurability = 100
	}
	if r.RepairCostPerPointToken <= 0 {
		r.RepairCostPerPointToken = 1
	}
	if r.RepairMinCostToken <= 0 {
		r.RepairMinCostToken = 1
	}
	if r.RepairBurnBps <= 0 {
		r.RepairBurnBps = 1000
	}
	if r.RepairRecycleBps <= 0 {
		r.RepairRecycleBps = 8000
	}
	if r.RepairRewardBps <= 0 {
		r.RepairRewardBps = 1000
	}
	if r.DungeonWearPoints < 0 {
		r.DungeonWearPoints = 0
	}
	if r.DungeonWearPoints == 0 {
		r.DungeonWearPoints = 5
	}
	if r.BossWearPoints < 0 {
		r.BossWearPoints = 0
	}
	if r.BossWearPoints == 0 {
		r.BossWearPoints = 8
	}
	return r
}

func (r EquipmentRules) RepairCost(missingPoints int) int64 {
	r = r.WithDefaults()
	if missingPoints <= 0 {
		return 0
	}
	cost := int64(missingPoints) * r.RepairCostPerPointToken
	if cost < r.RepairMinCostToken {
		return r.RepairMinCostToken
	}
	return cost
}

type EquipmentRepairRequest struct {
	OpID         string
	AccountID    int64
	CharacterID  int64
	EquipmentUID string
	Rules        EquipmentRules
}

type EquipmentRepairResult struct {
	Equipment EquipmentItem   `json:"equipment"`
	CostToken int64           `json:"costToken"`
	Repaired  int             `json:"repairedPoints"`
	Snapshot  EconomySnapshot `json:"snapshot"`
}

func (s *PostgresStore) EquipmentRepair(req EquipmentRepairRequest) (EquipmentRepairResult, error) {
	rules := req.Rules.WithDefaults()
	uid := strings.TrimSpace(req.EquipmentUID)
	if uid == "" {
		return EquipmentRepairResult{}, errors.New("equipmentUid is required")
	}
	return runIdempotentAction(s, "equipment_repair", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (EquipmentRepairResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return EquipmentRepairResult{}, err
		}
		var id int64
		var durability, maxDurability *int
		var location string
		err := tx.QueryRow(ctx, `
			SELECT id, durability, max_durability, location
			FROM equipment_items
			WHERE equipment_uid = $1 AND account_id = $2 AND character_id = $3
				AND location NOT IN ('DELETED', 'BURNED', 'CONSUMED', 'LISTED', 'ON_CHAIN', 'MINT_PENDING', 'LOCKED_FOR_MINT')
			FOR UPDATE
		`, uid, req.AccountID, req.CharacterID).Scan(&id, &durability, &maxDurability, &location)
		if errors.Is(err, pgx.ErrNoRows) {
			return EquipmentRepairResult{}, ErrNotFound
		}
		if err != nil {
			return EquipmentRepairResult{}, err
		}
		maxD := rules.DefaultMaxDurability
		if maxDurability != nil && *maxDurability > 0 {
			maxD = *maxDurability
		}
		cur := 0
		if durability != nil {
			cur = *durability
		}
		if maxD <= 0 {
			return EquipmentRepairResult{}, errors.New("equipment has no durability")
		}
		if cur >= maxD {
			return EquipmentRepairResult{}, errors.New("equipment does not need repair")
		}
		missing := maxD - cur
		cost := rules.RepairCost(missing)
		spend, err := s.spendTokenInTx(ctx, tx, req.AccountID, cost)
		if err != nil {
			return EquipmentRepairResult{}, fmt.Errorf("repair cost: %w", err)
		}
		burn := bpsCeil(cost, rules.RepairBurnBps)
		recycle := bpsCeil(cost, rules.RepairRecycleBps)
		rewards := cost - burn - recycle
		if rewards < 0 {
			rewards = 0
		}
		if err := s.insertSystemConsumption(ctx, tx, req.OpID, req.AccountID, req.CharacterID, spend, "EQUIPMENT_REPAIR", cost, burn, recycle, rewards,
			fmt.Sprintf(`{"equipmentUid":%q,"repaired":%d}`, uid, missing)); err != nil {
			return EquipmentRepairResult{}, err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE equipment_items
			SET durability = $2,
			    max_durability = COALESCE(NULLIF(max_durability, 0), $2),
			    updated_at = NOW()
			WHERE id = $1
		`, id, maxD)
		if err != nil {
			return EquipmentRepairResult{}, err
		}
		if tag.RowsAffected() == 0 {
			return EquipmentRepairResult{}, errors.New("equipment update failed")
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "EQUIPMENT_REPAIRED", "AEB", cost, req.OpID); err != nil {
			return EquipmentRepairResult{}, err
		}
		equip, err := s.loadEquipmentByUID(ctx, tx, uid)
		if err != nil {
			return EquipmentRepairResult{}, err
		}
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		if err != nil {
			return EquipmentRepairResult{}, err
		}
		return EquipmentRepairResult{
			Equipment: equip,
			CostToken: cost,
			Repaired:  missing,
			Snapshot:  snapshot,
		}, nil
	})
}

func (s *PostgresStore) loadEquipmentByUID(ctx context.Context, q postgresReader, uid string) (EquipmentItem, error) {
	var row EquipmentItem
	var affixes []byte
	var maxDur *int
	err := q.QueryRow(ctx, `
		SELECT id, equipment_uid, item_id, rarity, enhance_level,
			COALESCE(durability, 0), max_durability, location,
			COALESCE(equip_slot, -1), COALESCE(slot, -1), affixes
		FROM equipment_items WHERE equipment_uid = $1
	`, uid).Scan(
		&row.ID, &row.EquipmentUID, &row.ItemID, &row.Rarity, &row.EnhanceLevel,
		&row.Durability, &maxDur, &row.Status, &row.EquipSlot, &row.Slot, &affixes,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return EquipmentItem{}, ErrNotFound
	}
	if err != nil {
		return EquipmentItem{}, err
	}
	if maxDur != nil {
		row.MaxDurability = *maxDur
	}
	_ = decodeAffixes(affixes, &row.Affixes)
	return row, nil
}

func (s *PostgresStore) wearEquippedGear(ctx context.Context, tx pgx.Tx, accountID, characterID int64, points int, defaultMax int) error {
	if points <= 0 {
		return nil
	}
	if defaultMax <= 0 {
		defaultMax = 100
	}
	_, err := tx.Exec(ctx, `
		UPDATE equipment_items
		SET
			max_durability = COALESCE(NULLIF(max_durability, 0), $4),
			durability = GREATEST(
				0,
				COALESCE(durability, COALESCE(NULLIF(max_durability, 0), $4)) - $3
			),
			updated_at = NOW()
		WHERE account_id = $1
			AND character_id = $2
			AND location = 'EQUIPPED'
	`, accountID, characterID, points, defaultMax)
	return err
}

func decodeAffixes(raw []byte, dest *[]EquipmentAffix) error {
	if len(raw) == 0 || dest == nil {
		return nil
	}
	return json.Unmarshal(raw, dest)
}
