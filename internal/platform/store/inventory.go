package store

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

type InventoryDiscardRequest struct {
	OpID         string
	AccountID    int64
	CharacterID  int64
	SlotIndex    int
	Quantity     int64
	EquipmentUID string
}

type MaterialCost struct {
	ItemID   string
	Quantity int64
}

type SynthesizeRequest struct {
	OpID        string
	AccountID   int64
	CharacterID int64
	RecipeID    string
	BatchCount  int64
	Inputs      []MaterialCost
	RewardPlan  DungeonRewardPlan
}

type bagStackRow struct {
	ID        int64
	ItemID    string
	Quantity  int64
	BindType  string
	Stackable bool
	MaxStack  int64
}

func (s *PostgresStore) InventoryOrganize(req EconomyActionRequest, bagSlots int) (EconomySnapshot, error) {
	if bagSlots <= 0 {
		bagSlots = 25
	}
	return s.runEconomyAction("inventory_organize", req, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return err
		}
		return s.organizeStorage(ctx, tx, req.AccountID, req.CharacterID, "BAG", bagSlots, req.OpID)
	})
}

func (s *PostgresStore) WarehouseOrganize(req EconomyActionRequest, warehouseSlots int) (EconomySnapshot, error) {
	if warehouseSlots <= 0 {
		warehouseSlots = 50
	}
	return s.runEconomyAction("warehouse_organize", req, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return err
		}
		return s.organizeStorage(ctx, tx, req.AccountID, req.CharacterID, "WAREHOUSE", warehouseSlots, req.OpID)
	})
}

func (s *PostgresStore) InventoryDiscard(req InventoryDiscardRequest) (EconomySnapshot, error) {
	return runIdempotentAction(s, "inventory_discard", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (EconomySnapshot, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return EconomySnapshot{}, err
		}
		equipmentUID := trim(req.EquipmentUID)
		if equipmentUID != "" {
			tag, err := tx.Exec(ctx, `
				UPDATE equipment_items
				SET location = 'DELETED', slot = NULL, updated_at = NOW()
				WHERE account_id = $1
					AND character_id = $2
					AND equipment_uid = $3
					AND location = 'IN_BAG'
			`, req.AccountID, req.CharacterID, equipmentUID)
			if err != nil {
				return EconomySnapshot{}, err
			}
			if tag.RowsAffected() == 0 {
				return EconomySnapshot{}, ErrForbidden
			}
			if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "INVENTORY_DISCARDED", equipmentUID, 1, req.OpID); err != nil {
				return EconomySnapshot{}, err
			}
			return s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		}
		if req.SlotIndex < 0 {
			return EconomySnapshot{}, errors.New("slotIndex or equipmentUid is required")
		}
		var rowID int64
		var itemID string
		var available int64
		err := tx.QueryRow(ctx, `
			SELECT id, item_id, quantity
			FROM inventory_items
			WHERE character_id = $1 AND location = 'BAG' AND slot = $2
			FOR UPDATE
		`, req.CharacterID, req.SlotIndex).Scan(&rowID, &itemID, &available)
		if errors.Is(err, pgx.ErrNoRows) {
			return EconomySnapshot{}, ErrNotFound
		}
		if err != nil {
			return EconomySnapshot{}, err
		}
		quantity := req.Quantity
		if quantity <= 0 {
			quantity = available
		}
		if quantity <= 0 || quantity > available {
			return EconomySnapshot{}, ErrInsufficientBalance
		}
		if quantity == available {
			_, err = tx.Exec(ctx, `
				UPDATE inventory_items
				SET location = 'DELETED', slot = NULL, updated_at = NOW()
				WHERE id = $1
			`, rowID)
		} else {
			_, err = tx.Exec(ctx, `
				UPDATE inventory_items
				SET quantity = quantity - $2, updated_at = NOW()
				WHERE id = $1
			`, rowID, quantity)
		}
		if err != nil {
			return EconomySnapshot{}, err
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "INVENTORY_DISCARDED", itemID, quantity, req.OpID); err != nil {
			return EconomySnapshot{}, err
		}
		return s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
	})
}

func (s *PostgresStore) Synthesize(req SynthesizeRequest) (EconomySnapshot, error) {
	if trim(req.RecipeID) == "" {
		return EconomySnapshot{}, errors.New("recipeId is required")
	}
	return runIdempotentAction(s, "inventory_synthesize", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (EconomySnapshot, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return EconomySnapshot{}, err
		}
		for _, input := range req.Inputs {
			if err := s.consumeBagItem(ctx, tx, req.CharacterID, input.ItemID, input.Quantity); err != nil {
				return EconomySnapshot{}, err
			}
		}
		rewards, err := s.applyRewardsToBag(ctx, tx, req.AccountID, req.CharacterID, req.OpID, req.RecipeID, "synthesize", req.RewardPlan)
		if err != nil {
			return EconomySnapshot{}, err
		}
		if _, err := s.publishRareRewardAnnouncementsTx(ctx, tx, req.RewardPlan, rareAnnouncementContext{
			AccountID: req.AccountID, CharacterID: req.CharacterID, Source: "合成",
			RefType: "recipe", RefID: req.RecipeID, AnnouncementOn: true,
		}); err != nil {
			return EconomySnapshot{}, err
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "INVENTORY_SYNTHESIZED", req.RecipeID, int64(len(rewards.Items)+len(rewards.EquipmentItems)), req.OpID); err != nil {
			return EconomySnapshot{}, err
		}
		return s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
	})
}

func (s *PostgresStore) organizeStorage(ctx context.Context, tx pgx.Tx, accountID, characterID int64, location string, slotLimit int, opID string) error {
	equipmentLocation, err := equipmentStorageLocation(location)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `
		SELECT i.id, i.item_id, i.quantity, i.bind_type, COALESCE(c.stackable, TRUE)
		FROM inventory_items i
		LEFT JOIN item_catalog c ON c.item_id = i.item_id
		WHERE i.character_id = $1 AND i.location = $2
		ORDER BY i.id
		FOR UPDATE OF i
	`, characterID, location)
	if err != nil {
		return err
	}
	defer rows.Close()

	groups := map[string]int64{}
	rowIDs := []int64{}
	for rows.Next() {
		var row bagStackRow
		if err := rows.Scan(&row.ID, &row.ItemID, &row.Quantity, &row.BindType, &row.Stackable); err != nil {
			return err
		}
		rowIDs = append(rowIDs, row.ID)
		key := row.ItemID + "|" + row.BindType
		groups[key] += row.Quantity
	}
	if err := rows.Err(); err != nil {
		return err
	}

	var equipmentCount int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM equipment_items
		WHERE character_id = $1 AND location = $2
	`, characterID, equipmentLocation).Scan(&equipmentCount); err != nil {
		return err
	}

	type mergedStack struct {
		ItemID   string
		BindType string
		Quantity int64
		MaxStack int64
	}
	merged := []mergedStack{}
	for key, total := range groups {
		parts := splitPair(key)
		maxStack := int64(999)
		var stackable bool
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(stackable, TRUE) FROM item_catalog WHERE item_id = $1
		`, parts[0]).Scan(&stackable); err == nil && !stackable {
			maxStack = 1
		}
		remaining := total
		for remaining > 0 {
			qty := remaining
			if qty > maxStack {
				qty = maxStack
			}
			merged = append(merged, mergedStack{
				ItemID:   parts[0],
				BindType: parts[1],
				Quantity: qty,
				MaxStack: maxStack,
			})
			remaining -= qty
		}
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].ItemID == merged[j].ItemID {
			return merged[i].BindType < merged[j].BindType
		}
		return merged[i].ItemID < merged[j].ItemID
	})

	if len(merged)+equipmentCount > slotLimit {
		return fmt.Errorf("%s does not have enough slots after organize: need %d, have %d", strings.ToLower(location), len(merged)+equipmentCount, slotLimit)
	}

	for _, id := range rowIDs {
		if _, err := tx.Exec(ctx, `DELETE FROM inventory_items WHERE id = $1`, id); err != nil {
			return err
		}
	}

	slot := 0
	for _, stack := range merged {
		if _, err := tx.Exec(ctx, `
			INSERT INTO inventory_items (character_id, item_id, quantity, location, slot, bind_type)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, characterID, stack.ItemID, stack.Quantity, location, slot, stack.BindType); err != nil {
			return err
		}
		slot++
	}

	equipmentRows, err := tx.Query(ctx, `
		SELECT id
		FROM equipment_items
		WHERE character_id = $1 AND location = $2
		ORDER BY id
		FOR UPDATE
	`, characterID, equipmentLocation)
	if err != nil {
		return err
	}
	defer equipmentRows.Close()
	for equipmentRows.Next() {
		var equipmentID int64
		if err := equipmentRows.Scan(&equipmentID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE equipment_items
			SET slot = $2, updated_at = NOW()
			WHERE id = $1
		`, equipmentID, slot); err != nil {
			return err
		}
		slot++
	}
	if err := equipmentRows.Err(); err != nil {
		return err
	}

	ledgerKind := "INVENTORY_ORGANIZED"
	if location == "WAREHOUSE" {
		ledgerKind = "WAREHOUSE_ORGANIZED"
	}
	return s.insertEconomyLedger(ctx, tx, accountID, characterID, ledgerKind, location, int64(len(merged)+equipmentCount), opID)
}

func (s *PostgresStore) consumeBagItem(ctx context.Context, tx pgx.Tx, characterID int64, itemID string, quantity int64) error {
	if quantity <= 0 {
		return errors.New("quantity must be positive")
	}
	rows, err := tx.Query(ctx, `
		SELECT id, quantity
		FROM inventory_items
		WHERE character_id = $1 AND location = 'BAG' AND item_id = $2
		ORDER BY slot NULLS LAST, id
		FOR UPDATE
	`, characterID, itemID)
	if err != nil {
		return err
	}
	type stack struct {
		ID, Available int64
	}
	var stacks []stack
	for rows.Next() {
		var row stack
		if err := rows.Scan(&row.ID, &row.Available); err != nil {
			rows.Close()
			return err
		}
		stacks = append(stacks, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	remaining := quantity
	for _, row := range stacks {
		if remaining <= 0 {
			break
		}
		use := row.Available
		if use > remaining {
			use = remaining
		}
		if use == row.Available {
			if _, err := tx.Exec(ctx, `
				UPDATE inventory_items
				SET location = 'CONSUMED', slot = NULL, updated_at = NOW()
				WHERE id = $1
			`, row.ID); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(ctx, `
				UPDATE inventory_items
				SET quantity = quantity - $2, updated_at = NOW()
				WHERE id = $1
			`, row.ID, use); err != nil {
				return err
			}
		}
		remaining -= use
	}
	if remaining > 0 {
		return ErrInsufficientBalance
	}
	return nil
}

func splitPair(key string) [2]string {
	for i := 0; i < len(key); i++ {
		if key[i] == '|' {
			return [2]string{key[:i], key[i+1:]}
		}
	}
	return [2]string{key, "BOUND"}
}

func trim(value string) string {
	return strings.TrimSpace(value)
}
