package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/flenzero/aeon-backend/internal/chain"
)

func TestPostgresStoreIntegration(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	pg, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer pg.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	wallet := "integration_wallet_" + suffix
	itemID := "integration_item_" + suffix
	equipmentUID := "integration_equipment_" + suffix
	duplicateSlotUID := "integration_equipment_slot_" + suffix

	defer func() {
		_, _ = pg.pool.Exec(ctx, `DELETE FROM accounts WHERE solana_wallet_address = $1`, wallet)
		_, _ = pg.pool.Exec(ctx, `DELETE FROM item_catalog WHERE item_id = $1`, itemID)
	}()

	nonce := WalletLoginNonce{
		Nonce:     "nonce_" + suffix,
		Wallet:    wallet,
		Message:   "Sign in to Aeonblight",
		Status:    "PENDING",
		ExpiresAt: time.Now().UTC().Add(time.Minute),
		CreatedAt: time.Now().UTC(),
	}
	pg.SaveWalletNonce(nonce)
	if _, err := pg.WalletNonce(nonce.Nonce, wallet, time.Now().UTC()); err != nil {
		t.Fatalf("wallet nonce: %v", err)
	}
	if err := pg.ConsumeWalletNonce(nonce.Nonce, wallet, time.Now().UTC()); err != nil {
		t.Fatalf("consume wallet nonce: %v", err)
	}
	if err := pg.ConsumeWalletNonce(nonce.Nonce, wallet, time.Now().UTC()); !errors.Is(err, ErrForbidden) {
		t.Fatalf("second nonce consume err = %v, want ErrForbidden", err)
	}

	account := pg.UpsertAccountByWallet(wallet)
	character, err := pg.CreateCharacter(account.ID, "Integration")
	if err != nil {
		t.Fatalf("create character: %v", err)
	}

	if _, err := pg.GrantLocked(account.ID, 100, "integration", "grant-"+suffix, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatalf("grant locked: %v", err)
	}
	settled := pg.SettleUnlocks(time.Now().UTC(), 10)
	if len(settled) == 0 {
		t.Fatal("expected at least one settled unlock")
	}
	token := pg.Token(account.ID)
	if token.WithdrawableBalance < 100 {
		t.Fatalf("withdrawable balance = %d, want at least 100", token.WithdrawableBalance)
	}
	withdrawal, err := pg.CreateWithdrawal(account.ID, 25, wallet, false)
	if err != nil {
		t.Fatalf("create withdrawal: %v", err)
	}
	processed := pg.ProcessAutoWithdrawals(time.Now().UTC(), 5000, 20000, 30000, 150000, 10)
	if !containsWithdrawal(processed, withdrawal.ID, "CONFIRMED") {
		t.Fatalf("processed withdrawals did not confirm id %d: %+v", withdrawal.ID, processed)
	}

	_, err = pg.pool.Exec(ctx, `
		INSERT INTO item_catalog (item_id, name, category, stackable)
		VALUES ($1, $2, 'equipment', FALSE)
	`, itemID, itemID)
	if err != nil {
		t.Fatalf("insert item catalog: %v", err)
	}
	_, err = pg.pool.Exec(ctx, `
		INSERT INTO inventory_items (character_id, item_id, quantity, location, slot)
		VALUES ($1, $2, 3, 'BAG', 0)
	`, character.ID, itemID)
	if err != nil {
		t.Fatalf("insert inventory: %v", err)
	}
	_, err = pg.pool.Exec(ctx, `
		INSERT INTO equipment_items (equipment_uid, account_id, character_id, item_id, location, slot, equip_slot)
		VALUES ($1, $2, $3, $4, 'IN_BAG', 1, 1)
	`, equipmentUID, account.ID, character.ID, itemID)
	if err != nil {
		t.Fatalf("insert equipment: %v", err)
	}
	snapshot, err := pg.EconomySnapshot(account.ID, character.ID)
	if err != nil {
		t.Fatalf("economy snapshot: %v", err)
	}
	if len(snapshot.Inventory) != 1 || snapshot.Inventory[0].ItemID != itemID || snapshot.Inventory[0].Quantity != 3 {
		t.Fatalf("inventory snapshot = %+v", snapshot.Inventory)
	}
	if len(snapshot.Equipment) != 1 || snapshot.Equipment[0].EquipmentUID != equipmentUID || snapshot.Equipment[0].Status != "IN_BAG" {
		t.Fatalf("equipment snapshot = %+v", snapshot.Equipment)
	}
	enterReq := DungeonEnterRequest{
		OpID:        "dungeon_enter_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		ChapterID:   0,
		FloorID:     1,
	}
	enterResult, err := pg.DungeonEnter(enterReq)
	if err != nil {
		t.Fatalf("dungeon enter: %v", err)
	}
	if enterResult.DungeonRunID == "" || enterResult.Status != "IN_PROGRESS" {
		t.Fatalf("dungeon enter result = %+v", enterResult)
	}
	enterReplay, err := pg.DungeonEnter(enterReq)
	if err != nil {
		t.Fatalf("dungeon enter replay: %v", err)
	}
	if enterReplay.DungeonRunID != enterResult.DungeonRunID {
		t.Fatalf("dungeon enter replay id = %s, want %s", enterReplay.DungeonRunID, enterResult.DungeonRunID)
	}
	finishReq := DungeonFinishRequest{
		OpID:         "dungeon_finish_" + suffix,
		AccountID:    account.ID,
		CharacterID:  character.ID,
		DungeonRunID: enterResult.DungeonRunID,
		ChapterID:    0,
		FloorID:      1,
		Result:       "victory",
		Exp:          20,
		Kills: []DungeonKill{
			{EnemyID: 101, EnemyName: "mourning_wraith", Quantity: 2},
		},
		RewardPlan: DungeonRewardPlan{
			Items: []DungeonRewardGrant{
				{
					RewardType: "item",
					ItemID:     itemID,
					Quantity:   2,
					Rarity:     1,
					Category:   "material",
				},
				{
					RewardType:   "equipment",
					ItemID:       itemID,
					Quantity:     1,
					EquipmentUID: "reward_equipment_" + suffix,
					Rarity:       2,
					Category:     "weapon",
					Affixes: []EquipmentAffix{
						{AffixID: "attack_flat", Stat: "attack", Value: 3},
					},
				},
			},
			TokenReward: 1,
		},
	}
	finishResult, err := pg.DungeonFinish(finishReq)
	if err != nil {
		t.Fatalf("dungeon finish: %v", err)
	}
	if finishResult.Status != "REWARDED" || finishResult.Rewards.Exp != 20 || finishResult.Snapshot.Exp != 20 {
		t.Fatalf("dungeon finish result = %+v", finishResult)
	}
	if len(finishResult.Rewards.Items) != 1 || finishResult.Rewards.Items[0].Quantity != 2 {
		t.Fatalf("dungeon item rewards = %+v", finishResult.Rewards.Items)
	}
	if len(finishResult.Rewards.EquipmentItems) != 1 || finishResult.Rewards.EquipmentItems[0].Status != "IN_LOOT_TRAY" {
		t.Fatalf("dungeon equipment rewards = %+v", finishResult.Rewards.EquipmentItems)
	}
	if finishResult.Rewards.TokenReward != "1" {
		t.Fatalf("dungeon token reward = %s, want 1", finishResult.Rewards.TokenReward)
	}
	finishReplay, err := pg.DungeonFinish(finishReq)
	if err != nil {
		t.Fatalf("dungeon finish replay: %v", err)
	}
	if finishReplay.Snapshot.Exp != finishResult.Snapshot.Exp {
		t.Fatalf("dungeon finish replay exp = %d, want %d", finishReplay.Snapshot.Exp, finishResult.Snapshot.Exp)
	}
	if _, err := pg.DungeonFinish(DungeonFinishRequest{
		OpID:         "dungeon_finish_again_" + suffix,
		AccountID:    account.ID,
		CharacterID:  character.ID,
		DungeonRunID: enterResult.DungeonRunID,
		ChapterID:    0,
		FloorID:      1,
		Result:       "victory",
	}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("second dungeon finish err = %v, want ErrForbidden", err)
	}
	failedEnter, err := pg.DungeonEnter(DungeonEnterRequest{
		OpID:        "dungeon_enter_failed_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		ChapterID:   0,
		FloorID:     2,
	})
	if err != nil {
		t.Fatalf("failed dungeon enter: %v", err)
	}
	failedFinish, err := pg.DungeonFinish(DungeonFinishRequest{
		OpID:         "dungeon_finish_failed_" + suffix,
		AccountID:    account.ID,
		CharacterID:  character.ID,
		DungeonRunID: failedEnter.DungeonRunID,
		ChapterID:    0,
		FloorID:      2,
		Result:       "defeat",
		Progress:     map[string]any{"reachedWave": float64(3)},
	})
	if err != nil {
		t.Fatalf("failed dungeon finish: %v", err)
	}
	if failedFinish.Status != "FAILED" || failedFinish.Result != "defeat" {
		t.Fatalf("failed dungeon finish result = %+v", failedFinish)
	}
	var reachedWave int
	if err := pg.pool.QueryRow(ctx, `
		SELECT COALESCE((result->'progress'->>'reachedWave')::int, 0)
		FROM dungeon_runs
		WHERE id = $1
	`, failedEnter.DungeonRunID).Scan(&reachedWave); err != nil {
		t.Fatalf("query failed dungeon progress: %v", err)
	}
	if reachedWave != 3 {
		t.Fatalf("stored failed dungeon progress reachedWave = %d, want 3", reachedWave)
	}
	depositReq := EconomyActionRequest{
		OpID:        "deposit_inventory_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		SlotIndex:   0,
		Quantity:    2,
	}
	snapshot, err = pg.WarehouseDeposit(depositReq)
	if err != nil {
		t.Fatalf("warehouse deposit inventory: %v", err)
	}
	if len(snapshot.Warehouse) != 1 || snapshot.Warehouse[0].Quantity != 2 {
		t.Fatalf("warehouse after deposit = %+v", snapshot.Warehouse)
	}
	replayed, err := pg.WarehouseDeposit(depositReq)
	if err != nil {
		t.Fatalf("warehouse deposit replay: %v", err)
	}
	if len(replayed.Warehouse) != len(snapshot.Warehouse) || replayed.Warehouse[0].Quantity != snapshot.Warehouse[0].Quantity {
		t.Fatalf("idempotent replay snapshot = %+v, want %+v", replayed.Warehouse, snapshot.Warehouse)
	}
	snapshot, err = pg.WarehouseWithdraw(EconomyActionRequest{
		OpID:        "withdraw_inventory_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		SlotIndex:   0,
		Quantity:    1,
	})
	if err != nil {
		t.Fatalf("warehouse withdraw inventory: %v", err)
	}
	if len(snapshot.Inventory) != 2 || len(snapshot.Warehouse) != 1 || snapshot.Warehouse[0].Quantity != 1 {
		t.Fatalf("snapshot after inventory withdraw = inventory:%+v warehouse:%+v", snapshot.Inventory, snapshot.Warehouse)
	}
	snapshot, err = pg.WarehouseDeposit(EconomyActionRequest{
		OpID:         "deposit_equipment_" + suffix,
		AccountID:    account.ID,
		CharacterID:  character.ID,
		SlotIndex:    -1,
		EquipmentUID: equipmentUID,
	})
	if err != nil {
		t.Fatalf("warehouse deposit equipment: %v", err)
	}
	if equipmentStatus(snapshot.Equipment, equipmentUID) != "IN_WAREHOUSE" {
		t.Fatalf("equipment after deposit = %+v", snapshot.Equipment)
	}
	snapshot, err = pg.WarehouseWithdraw(EconomyActionRequest{
		OpID:         "withdraw_equipment_" + suffix,
		AccountID:    account.ID,
		CharacterID:  character.ID,
		SlotIndex:    -1,
		EquipmentUID: equipmentUID,
	})
	if err != nil {
		t.Fatalf("warehouse withdraw equipment: %v", err)
	}
	if equipmentStatus(snapshot.Equipment, equipmentUID) != "IN_BAG" {
		t.Fatalf("equipment after withdraw = %+v", snapshot.Equipment)
	}
	snapshot, err = pg.EquipItem(EconomyActionRequest{
		OpID:         "equip_equipment_" + suffix,
		AccountID:    account.ID,
		CharacterID:  character.ID,
		EquipmentUID: equipmentUID,
		EquipSlot:    -1,
	})
	if err != nil {
		t.Fatalf("equip item: %v", err)
	}
	if equipmentStatus(snapshot.Equipment, equipmentUID) != "EQUIPPED" {
		t.Fatalf("equipment after equip = %+v", snapshot.Equipment)
	}
	snapshot, err = pg.UnequipItem(EconomyActionRequest{
		OpID:        "unequip_equipment_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		SlotIndex:   -1,
		EquipSlot:   1,
	})
	if err != nil {
		t.Fatalf("unequip item: %v", err)
	}
	if equipmentStatus(snapshot.Equipment, equipmentUID) != "IN_BAG" {
		t.Fatalf("equipment after unequip = %+v", snapshot.Equipment)
	}
	_, err = pg.pool.Exec(ctx, `
		INSERT INTO equipment_items (equipment_uid, account_id, character_id, item_id, location, slot, equip_slot)
		VALUES ($1, $2, $3, $4, 'IN_BAG', 1, 2)
	`, equipmentUID, account.ID, character.ID, itemID)
	if !isUniqueViolation(err) {
		t.Fatalf("duplicate equipment uid err = %v, want unique violation", err)
	}
	_, err = pg.pool.Exec(ctx, `
		INSERT INTO equipment_items (equipment_uid, account_id, character_id, item_id, location, slot, equip_slot)
		VALUES ($1, $2, $3, $4, 'IN_BAG', 1, 3)
	`, duplicateSlotUID, account.ID, character.ID, itemID)
	if !isUniqueViolation(err) {
		t.Fatalf("duplicate equipment slot err = %v, want unique violation", err)
	}
}

func TestPostgresLootClaimAndActivitySettlements(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	pg, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer pg.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	wallet := "loot_wallet_" + suffix
	materialID := "loot_material_" + suffix
	equipmentItemID := "loot_weapon_" + suffix

	defer func() {
		_, _ = pg.pool.Exec(ctx, `DELETE FROM accounts WHERE solana_wallet_address = $1`, wallet)
		_, _ = pg.pool.Exec(ctx, `DELETE FROM item_catalog WHERE item_id IN ($1, $2)`, materialID, equipmentItemID)
	}()

	account := pg.UpsertAccountByWallet(wallet)
	character, err := pg.CreateCharacter(account.ID, "Loot")
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	if _, err := pg.pool.Exec(ctx, `
		INSERT INTO item_catalog (item_id, name, category, stackable)
		VALUES ($1, $1, 'material', TRUE), ($2, $2, 'weapon', FALSE)
	`, materialID, equipmentItemID); err != nil {
		t.Fatalf("insert catalog: %v", err)
	}

	var itemLootID int64
	if err := pg.pool.QueryRow(ctx, `
		INSERT INTO loot_tray_items (account_id, character_id, item_type, item_id, quantity, source)
		VALUES ($1, $2, 'ITEM', $3, 3, 'test')
		RETURNING id
	`, account.ID, character.ID, materialID).Scan(&itemLootID); err != nil {
		t.Fatalf("insert item loot: %v", err)
	}
	snapshot, err := pg.LootClaim(LootActionRequest{
		OpID:        "claim_item_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		LootID:      itemLootID,
		Quantity:    2,
	})
	if err != nil {
		t.Fatalf("claim item loot: %v", err)
	}
	if inventoryQuantity(snapshot.Inventory, materialID) != 2 || lootQuantity(snapshot.LootTray, itemLootID) != 1 {
		t.Fatalf("snapshot after partial claim inventory=%+v loot=%+v", snapshot.Inventory, snapshot.LootTray)
	}

	var equipmentID int64
	equipmentUID := "claim_equipment_" + suffix
	if err := pg.pool.QueryRow(ctx, `
		INSERT INTO equipment_items (equipment_uid, account_id, character_id, item_id, location, rarity, affixes)
		VALUES ($1, $2, $3, $4, 'IN_LOOT_TRAY', 2, '[{"affixId":"attack_flat","stat":"attack","value":4}]'::jsonb)
		RETURNING id
	`, equipmentUID, account.ID, character.ID, equipmentItemID).Scan(&equipmentID); err != nil {
		t.Fatalf("insert equipment loot equipment: %v", err)
	}
	if _, err := pg.pool.Exec(ctx, `
		INSERT INTO loot_tray_items (account_id, character_id, item_type, item_id, equipment_id, quantity, source)
		VALUES ($1, $2, 'EQUIPMENT', $3, $4, 1, 'test')
	`, account.ID, character.ID, equipmentItemID, equipmentID); err != nil {
		t.Fatalf("insert equipment loot row: %v", err)
	}
	snapshot, err = pg.LootClaimAll(LootActionRequest{
		OpID:        "claim_all_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
	})
	if err != nil {
		t.Fatalf("claim all loot: %v", err)
	}
	if inventoryQuantity(snapshot.Inventory, materialID) != 3 {
		t.Fatalf("inventory after claim all = %+v", snapshot.Inventory)
	}
	if equipmentStatus(snapshot.Equipment, equipmentUID) != "IN_BAG" {
		t.Fatalf("equipment after claim all = %+v", snapshot.Equipment)
	}
	if len(equipmentAffixes(snapshot.Equipment, equipmentUID)) != 1 {
		t.Fatalf("equipment affixes after claim all = %+v", snapshot.Equipment)
	}

	discardUID := "discard_equipment_" + suffix
	if err := pg.pool.QueryRow(ctx, `
		INSERT INTO equipment_items (equipment_uid, account_id, character_id, item_id, location)
		VALUES ($1, $2, $3, $4, 'IN_LOOT_TRAY')
		RETURNING id
	`, discardUID, account.ID, character.ID, equipmentItemID).Scan(&equipmentID); err != nil {
		t.Fatalf("insert discard equipment: %v", err)
	}
	var discardLootID int64
	if err := pg.pool.QueryRow(ctx, `
		INSERT INTO loot_tray_items (account_id, character_id, item_type, item_id, equipment_id, quantity, source)
		VALUES ($1, $2, 'EQUIPMENT', $3, $4, 1, 'test')
		RETURNING id
	`, account.ID, character.ID, equipmentItemID, equipmentID).Scan(&discardLootID); err != nil {
		t.Fatalf("insert discard loot: %v", err)
	}
	snapshot, err = pg.LootDiscard(LootActionRequest{
		OpID:        "discard_loot_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		LootID:      discardLootID,
	})
	if err != nil {
		t.Fatalf("discard loot: %v", err)
	}
	if equipmentStatus(snapshot.Equipment, discardUID) != "" {
		t.Fatalf("discarded equipment still visible = %+v", snapshot.Equipment)
	}

	gatherResult, err := pg.GatheringSettle(ActivitySettlementRequest{
		OpID:        "gather_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		ActivityID:  "node_" + suffix,
		RewardPlan:  directRewardPlan(materialID, equipmentItemID, "gather_equipment_"+suffix),
	})
	if err != nil {
		t.Fatalf("gathering settle: %v", err)
	}
	if inventoryQuantity(gatherResult.Snapshot.Inventory, materialID) < 4 {
		t.Fatalf("gathering should add material directly to bag: %+v", gatherResult.Snapshot.Inventory)
	}
	if equipmentStatus(gatherResult.Snapshot.Equipment, "gather_equipment_"+suffix) != "IN_BAG" {
		t.Fatalf("gathering equipment should enter bag: %+v", gatherResult.Snapshot.Equipment)
	}
	if len(gatherResult.Snapshot.LootTray) != 0 {
		t.Fatalf("gathering should not create loot tray rows: %+v", gatherResult.Snapshot.LootTray)
	}

	farmResult, err := pg.FarmingHarvest(ActivitySettlementRequest{
		OpID:        "farm_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		ActivityID:  "crop_" + suffix,
		RewardPlan:  directRewardPlan(materialID, equipmentItemID, "farm_equipment_"+suffix),
	})
	if err != nil {
		t.Fatalf("farming harvest: %v", err)
	}
	if len(farmResult.Snapshot.LootTray) != 0 {
		t.Fatalf("farming should not create loot tray rows: %+v", farmResult.Snapshot.LootTray)
	}
	if equipmentStatus(farmResult.Snapshot.Equipment, "farm_equipment_"+suffix) != "IN_BAG" {
		t.Fatalf("farming equipment should enter bag: %+v", farmResult.Snapshot.Equipment)
	}
}

func directRewardPlan(materialID, equipmentItemID, equipmentUID string) DungeonRewardPlan {
	return DungeonRewardPlan{
		Items: []DungeonRewardGrant{
			{RewardType: "item", ItemID: materialID, Quantity: 1, Rarity: 1, Category: "material"},
			{
				RewardType:   "equipment",
				ItemID:       equipmentItemID,
				Quantity:     1,
				EquipmentUID: equipmentUID,
				Rarity:       2,
				Category:     "weapon",
				Affixes:      []EquipmentAffix{{AffixID: "rare_find", Stat: "rareFind", Value: 0.02}},
			},
		},
		TokenReward: 1,
	}
}

func inventoryQuantity(rows []InventoryItem, itemID string) int64 {
	var total int64
	for _, row := range rows {
		if row.ItemID == itemID {
			total += row.Quantity
		}
	}
	return total
}

func lootQuantity(rows []InventoryItem, lootID int64) int64 {
	for _, row := range rows {
		if row.ID == lootID {
			return row.Quantity
		}
	}
	return 0
}

func equipmentAffixes(rows []EquipmentItem, uid string) []EquipmentAffix {
	for _, row := range rows {
		if row.EquipmentUID == uid {
			return row.Affixes
		}
	}
	return nil
}

func equipmentStatus(rows []EquipmentItem, uid string) string {
	for _, row := range rows {
		if row.EquipmentUID == uid {
			return row.Status
		}
	}
	return ""
}

func containsWithdrawal(rows []Withdrawal, id int64, status string) bool {
	for _, row := range rows {
		if row.ID == id && strings.EqualFold(row.Status, status) {
			return true
		}
	}
	return false
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func TestPostgresBossContributeAndSettle(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	pg, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer pg.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	wallet := "boss_wallet_" + suffix

	defer func() {
		_, _ = pg.pool.Exec(ctx, `DELETE FROM accounts WHERE solana_wallet_address = $1`, wallet)
	}()

	account := pg.UpsertAccountByWallet(wallet)
	character, err := pg.CreateCharacter(account.ID, "Boss")
	if err != nil {
		t.Fatalf("create character: %v", err)
	}

	startsAt := time.Now().UTC().Add(-time.Hour)
	endsAt := time.Now().UTC().Add(time.Hour)
	var bossEventID int64
	if err := pg.pool.QueryRow(ctx, `
		INSERT INTO boss_events (boss_key, status, starts_at, ends_at, reward_pool)
		VALUES ('shadow_leviathan', 'OPEN', $1, $2, '{}'::jsonb)
		RETURNING id
	`, startsAt, endsAt).Scan(&bossEventID); err != nil {
		t.Fatalf("insert boss event: %v", err)
	}

	contribute, err := pg.BossContribute(BossContributeRequest{
		OpID:         "boss_contrib_" + suffix,
		AccountID:    account.ID,
		CharacterID:  character.ID,
		BossEventID:  bossEventID,
		Contribution: 6000,
	})
	if err != nil {
		t.Fatalf("boss contribute: %v", err)
	}
	if contribute.Contribution != 6000 || contribute.BossKey != "shadow_leviathan" {
		t.Fatalf("contribute result = %+v", contribute)
	}

	if _, err := pg.pool.Exec(ctx, `
		UPDATE boss_events
		SET status = 'SETTLING', ends_at = $2
		WHERE id = $1
	`, bossEventID, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatalf("close boss event: %v", err)
	}

	contribution, bossKey, err := pg.BossContribution(account.ID, bossEventID)
	if err != nil {
		t.Fatalf("boss contribution lookup: %v", err)
	}
	if contribution != 6000 || bossKey != "shadow_leviathan" {
		t.Fatalf("contribution lookup = %d %q", contribution, bossKey)
	}

	plan := DungeonRewardPlan{
		Items: []DungeonRewardGrant{
			{RewardType: "item", ItemID: "gloomcap_spore", Quantity: 2, Rarity: 1, Category: "material"},
		},
		TokenReward: 3,
		IsBoss:      true,
	}
	settle, err := pg.BossSettle(BossSettleRequest{
		OpID:        "boss_settle_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		BossEventID: bossEventID,
		BossKey:     bossKey,
		RewardPlan:  plan,
	})
	if err != nil {
		t.Fatalf("boss settle: %v", err)
	}
	if len(settle.Snapshot.LootTray) == 0 {
		t.Fatalf("expected loot tray rewards after boss settle: %+v", settle)
	}
	if settle.Contribution != 6000 {
		t.Fatalf("settle contribution = %d, want 6000", settle.Contribution)
	}

	_, err = pg.BossSettle(BossSettleRequest{
		OpID:        "boss_settle_repeat_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		BossEventID: bossEventID,
		BossKey:     bossKey,
		RewardPlan:  plan,
	})
	if err == nil || !strings.Contains(err.Error(), "already claimed") {
		t.Fatalf("second settle err = %v, want already claimed", err)
	}
}

func TestPostgresInventoryOrganizeDiscardAndSynthesize(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	pg, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer pg.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	wallet := "inventory_wallet_" + suffix
	materialID := "inv_material_" + suffix
	sporeID := "inv_spore_" + suffix
	weaponID := "inv_weapon_" + suffix

	defer func() {
		_, _ = pg.pool.Exec(ctx, `DELETE FROM accounts WHERE solana_wallet_address = $1`, wallet)
		_, _ = pg.pool.Exec(ctx, `DELETE FROM item_catalog WHERE item_id IN ($1, $2, $3)`, materialID, sporeID, weaponID)
	}()

	account := pg.UpsertAccountByWallet(wallet)
	character, err := pg.CreateCharacter(account.ID, "Inventory")
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	for _, row := range []struct {
		itemID    string
		stackable bool
	}{
		{materialID, true},
		{sporeID, true},
		{weaponID, false},
	} {
		if _, err := pg.pool.Exec(ctx, `
			INSERT INTO item_catalog (item_id, name, category, stackable)
			VALUES ($1, $1, 'material', $2)
		`, row.itemID, row.stackable); err != nil {
			t.Fatalf("insert catalog %s: %v", row.itemID, err)
		}
	}

	if _, err := pg.pool.Exec(ctx, `
		INSERT INTO inventory_items (character_id, item_id, quantity, location, slot)
		VALUES
			($1, $2, 2, 'BAG', 5),
			($1, $2, 3, 'BAG', 9)
	`, character.ID, materialID); err != nil {
		t.Fatalf("insert scattered stacks: %v", err)
	}

	organized, err := pg.InventoryOrganize(EconomyActionRequest{
		OpID:        "organize_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
	}, 25)
	if err != nil {
		t.Fatalf("organize: %v", err)
	}
	if inventoryQuantity(organized.Inventory, materialID) != 5 {
		t.Fatalf("organized inventory = %+v", organized.Inventory)
	}
	if organized.Inventory[0].Slot != 0 {
		t.Fatalf("expected merged stack at slot 0, got %+v", organized.Inventory)
	}

	if _, err := pg.pool.Exec(ctx, `
		INSERT INTO inventory_items (character_id, item_id, quantity, location, slot)
		VALUES ($1, $2, 4, 'BAG', 1)
	`, character.ID, sporeID); err != nil {
		t.Fatalf("insert spore stack: %v", err)
	}
	discarded, err := pg.InventoryDiscard(InventoryDiscardRequest{
		OpID:        "discard_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		SlotIndex:   1,
		Quantity:    2,
	})
	if err != nil {
		t.Fatalf("discard: %v", err)
	}
	if inventoryQuantity(discarded.Inventory, sporeID) != 2 {
		t.Fatalf("inventory after discard = %+v", discarded.Inventory)
	}

	if _, err := pg.pool.Exec(ctx, `
		INSERT INTO inventory_items (character_id, item_id, quantity, location, slot)
		VALUES ($1, $2, 5, 'BAG', 2), ($1, $3, 10, 'BAG', 3)
	`, character.ID, materialID, sporeID); err != nil {
		t.Fatalf("insert synth materials: %v", err)
	}
	crafted, err := pg.Synthesize(SynthesizeRequest{
		OpID:        "synth_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		RecipeID:    "compress_aeon_shard",
		BatchCount:  1,
		Inputs: []MaterialCost{
			{ItemID: sporeID, Quantity: 3},
		},
		RewardPlan: DungeonRewardPlan{
			Items: []DungeonRewardGrant{
				{RewardType: "item", ItemID: materialID, Quantity: 1, Category: "material", Rarity: 4},
			},
		},
	})
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if inventoryQuantity(crafted.Inventory, sporeID) != 9 {
		t.Fatalf("spore after synth = %d, want 9", inventoryQuantity(crafted.Inventory, sporeID))
	}
	if inventoryQuantity(crafted.Inventory, materialID) < 6 {
		t.Fatalf("material after synth = %+v", crafted.Inventory)
	}

	if _, err := pg.pool.Exec(ctx, `
		INSERT INTO inventory_items (character_id, item_id, quantity, location, slot)
		VALUES
			($1, $2, 2, 'WAREHOUSE', 7),
			($1, $2, 4, 'WAREHOUSE', 12)
	`, character.ID, materialID); err != nil {
		t.Fatalf("insert warehouse stacks: %v", err)
	}
	warehouse, err := pg.WarehouseOrganize(EconomyActionRequest{
		OpID:        "warehouse_org_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
	}, 50)
	if err != nil {
		t.Fatalf("warehouse organize: %v", err)
	}
	if inventoryQuantity(warehouse.Warehouse, materialID) != 6 {
		t.Fatalf("warehouse after organize = %+v", warehouse.Warehouse)
	}
	if warehouse.Warehouse[0].Slot != 0 {
		t.Fatalf("expected warehouse stack at slot 0, got %+v", warehouse.Warehouse)
	}
}

func TestPostgresBossEventLifecycle(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	pg, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer pg.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	opened, err := pg.BossOpenEvent(BossOpenEventRequest{
		OpID:     "boss_open_" + suffix,
		BossKey:  "shadow_leviathan",
		StartsAt: time.Now().UTC().Add(-time.Minute),
		EndsAt:   time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("open boss event: %v", err)
	}
	if opened.Status != "OPEN" || opened.BossKey != "shadow_leviathan" {
		t.Fatalf("opened = %+v", opened)
	}
	defer func() {
		_, _ = pg.pool.Exec(ctx, `DELETE FROM boss_events WHERE id = $1`, opened.ID)
	}()

	active, err := pg.BossListActiveEvents()
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	found := false
	for _, event := range active {
		if event.ID == opened.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("opened event missing from active list: %+v", active)
	}

	_, err = pg.BossOpenEvent(BossOpenEventRequest{
		OpID:     "boss_open_dup_" + suffix,
		BossKey:  "shadow_leviathan",
		StartsAt: time.Now().UTC(),
		EndsAt:   time.Now().UTC().Add(time.Hour),
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("duplicate open err = %v", err)
	}

	closed, err := pg.BossCloseEvent(BossCloseEventRequest{
		OpID:        "boss_close_" + suffix,
		BossEventID: opened.ID,
	})
	if err != nil {
		t.Fatalf("close boss event: %v", err)
	}
	if closed.Status != "SETTLING" {
		t.Fatalf("closed = %+v", closed)
	}

	settled, err := pg.BossMarkSettled(BossMarkSettledRequest{
		OpID:        "boss_mark_settled_" + suffix,
		BossEventID: opened.ID,
	})
	if err != nil {
		t.Fatalf("mark settled: %v", err)
	}
	if settled.Status != "SETTLED" {
		t.Fatalf("settled = %+v", settled)
	}
}

func TestPostgresMarketplaceListBuyCancel(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	pg, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer pg.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	sellerWallet := "mkt_seller_" + suffix
	buyerWallet := "mkt_buyer_" + suffix
	itemID := "mkt_equip_" + suffix
	equipUID := "mkt_uid_" + suffix
	materialID := "mkt_mat_" + suffix

	defer func() {
		_, _ = pg.pool.Exec(ctx, `DELETE FROM accounts WHERE solana_wallet_address = ANY($1)`, []string{sellerWallet, buyerWallet})
		_, _ = pg.pool.Exec(ctx, `DELETE FROM item_catalog WHERE item_id = ANY($1)`, []string{itemID, materialID})
	}()

	seller := pg.UpsertAccountByWallet(sellerWallet)
	buyer := pg.UpsertAccountByWallet(buyerWallet)
	sellerChar, err := pg.CreateCharacter(seller.ID, "Seller")
	if err != nil {
		t.Fatalf("seller character: %v", err)
	}
	buyerChar, err := pg.CreateCharacter(buyer.ID, "Buyer")
	if err != nil {
		t.Fatalf("buyer character: %v", err)
	}

	if _, err := pg.pool.Exec(ctx, `
		INSERT INTO item_catalog (item_id, name, category, rarity, stackable, tradable, default_bind_type)
		VALUES
			($1, $1, 'weapon', 3, FALSE, TRUE, 'UNBOUND'),
			($2, $2, 'rare_material', 4, TRUE, TRUE, 'UNBOUND')
	`, itemID, materialID); err != nil {
		t.Fatalf("catalog: %v", err)
	}
	if _, err := pg.pool.Exec(ctx, `
		INSERT INTO equipment_items (equipment_uid, account_id, character_id, item_id, location, slot, equip_slot, bind_type, rarity)
		VALUES ($1, $2, $3, $4, 'IN_BAG', 0, 1, 'UNBOUND', 3)
	`, equipUID, seller.ID, sellerChar.ID, itemID); err != nil {
		t.Fatalf("equipment: %v", err)
	}
	if _, err := pg.pool.Exec(ctx, `
		INSERT INTO inventory_items (character_id, item_id, quantity, location, slot, bind_type)
		VALUES ($1, $2, 5, 'BAG', 1, 'UNBOUND')
	`, sellerChar.ID, materialID); err != nil {
		t.Fatalf("material: %v", err)
	}

	// Seller needs deposit; buyer needs purchase funds (use withdrawable via grant+settle).
	if _, err := pg.GrantLocked(seller.ID, 50, "test", "seller-fund-"+suffix, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatalf("seller fund: %v", err)
	}
	if _, err := pg.GrantLocked(buyer.ID, 1000, "test", "buyer-fund-"+suffix, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatalf("buyer fund: %v", err)
	}
	_ = pg.SettleUnlocks(time.Now().UTC(), 20)

	if _, err := pg.pool.Exec(ctx, `
		UPDATE accounts SET has_trading_license = TRUE, trading_license_at = NOW()
		WHERE id IN ($1, $2)
	`, seller.ID, buyer.ID); err != nil {
		t.Fatalf("grant licenses: %v", err)
	}

	rules := MarketplaceRules{Enabled: true}.withDefaults()
	rules.PurchaseCooldownSeconds = 0

	listed, err := pg.MarketplaceCreateListing(MarketplaceListRequest{
		OpID:         "mkt_list_" + suffix,
		AccountID:    seller.ID,
		CharacterID:  sellerChar.ID,
		AssetType:    "EQUIPMENT",
		EquipmentUID: equipUID,
		PriceToken:   100,
		Rules:        rules,
	})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if listed.Listing.Status != "LISTED" || listed.Listing.ListingDepositToken != 1 {
		t.Fatalf("listed = %+v", listed.Listing)
	}

	_, err = pg.MarketplaceBuy(MarketplaceBuyRequest{
		OpID:        "mkt_selfbuy_" + suffix,
		AccountID:   seller.ID,
		CharacterID: sellerChar.ID,
		ListingID:   listed.Listing.ID,
		Rules:       rules,
	})
	if err == nil || !strings.Contains(err.Error(), "own listing") {
		t.Fatalf("self-buy err = %v", err)
	}

	bought, err := pg.MarketplaceBuy(MarketplaceBuyRequest{
		OpID:        "mkt_buy_" + suffix,
		AccountID:   buyer.ID,
		CharacterID: buyerChar.ID,
		ListingID:   listed.Listing.ID,
		Rules:       rules,
	})
	if err != nil {
		t.Fatalf("buy: %v", err)
	}
	if bought.Order.Status != "COMPLETED" || bought.Order.FeeToken != 5 || bought.Order.SellerProceedsToken != 95 {
		t.Fatalf("order = %+v", bought.Order)
	}
	buyerSnap, err := pg.EconomySnapshot(buyer.ID, buyerChar.ID)
	if err != nil {
		t.Fatalf("buyer snapshot: %v", err)
	}
	foundEquip := false
	for _, row := range buyerSnap.Equipment {
		if row.EquipmentUID == equipUID && row.Status == "IN_BAG" {
			foundEquip = true
			break
		}
	}
	if !foundEquip {
		t.Fatalf("buyer missing equipment: %+v", buyerSnap.Equipment)
	}
	sellerToken := pg.Token(seller.ID)
	if sellerToken.LockedBalance < 96 { // 95 proceeds + 1 deposit return
		t.Fatalf("seller locked = %d, want >= 96", sellerToken.LockedBalance)
	}

	// Cancel path with a fresh material listing.
	matListed, err := pg.MarketplaceCreateListing(MarketplaceListRequest{
		OpID:           "mkt_list_mat_" + suffix,
		AccountID:      seller.ID,
		CharacterID:    sellerChar.ID,
		AssetType:      "ITEM",
		SourceLocation: "BAG",
		SlotIndex:      1,
		Quantity:       2,
		PriceToken:     200,
		Rules:          rules,
	})
	if err != nil {
		t.Fatalf("list material: %v", err)
	}
	cancelled, err := pg.MarketplaceCancel(MarketplaceCancelRequest{
		OpID:      "mkt_cancel_" + suffix,
		AccountID: seller.ID,
		ListingID: matListed.Listing.ID,
		Rules:     rules,
	})
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.Listing.Status != "CANCELLED" {
		t.Fatalf("cancelled = %+v", cancelled.Listing)
	}
	sellerSnap, err := pg.EconomySnapshot(seller.ID, sellerChar.ID)
	if err != nil {
		t.Fatalf("seller snapshot: %v", err)
	}
	if inventoryQuantity(sellerSnap.Inventory, materialID) != 5 {
		t.Fatalf("material after cancel = %+v", sellerSnap.Inventory)
	}
}

func TestPostgresSolanaDepositAndWalletExpand(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	ctx := context.Background()
	pg, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer pg.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	wallet := "DepWallet" + suffix // keep mixed case like Solana
	depositOwner := "Treasury" + suffix
	mint := "Mint" + suffix
	sig := "sig_deposit_" + suffix

	account := pg.UpsertAccountByWallet(wallet)
	character, err := pg.CreateCharacter(account.ID, "Depositor")
	if err != nil {
		t.Fatalf("character: %v", err)
	}
	defer func() {
		_, _ = pg.pool.Exec(ctx, `DELETE FROM accounts WHERE id = $1`, account.ID)
	}()

	rpc := &chain.MemoryRPC{
		Slot: 100,
		Signatures: map[string][]chain.SignatureInfo{
			depositOwner: {{Signature: sig, Slot: 90}},
		},
		Txs: map[string]*chain.TransactionDetail{
			sig: {
				Signature: sig,
				Slot:      90,
				PreTokenBalances: []chain.TokenBalance{
					{Owner: wallet, Mint: mint, Amount: 500},
					{Owner: depositOwner, Mint: mint, Amount: 1000},
				},
				PostTokenBalances: []chain.TokenBalance{
					{Owner: wallet, Mint: mint, Amount: 400},
					{Owner: depositOwner, Mint: mint, Amount: 1100},
				},
			},
		},
	}
	scan, err := pg.ScanAndCreditDeposits(ctx, rpc, ChainScanConfig{
		Network:       "solana-devnet",
		TokenMint:     mint,
		TokenDecimals: 0,
		DepositWallet: depositOwner,
		ScanLimit:     20,
		CursorName:    "solana_deposits_test_" + suffix,
	})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if scan.Credited != 1 {
		t.Fatalf("scan=%+v", scan)
	}
	token := pg.Token(account.ID)
	if token.ExternalBalance != 100 || token.TokenBalance < 100 {
		t.Fatalf("token after deposit = %+v", token)
	}

	// Wallet expand payment order + internal confirm.
	rules := MarketplaceRules{
		Enabled:                true,
		DepositReceiverWallet:  depositOwner,
		WalletExpandPriceToken: 50,
		WalletExpandMaxTimes:   5,
		WalletExpandSlots:      2,
	}.withDefaults()
	created, err := pg.MarketplaceExpandWalletSlots(MarketplaceExpandWalletRequest{
		OpID:        "expand_wallet_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		Rules:       rules,
	})
	if err != nil {
		t.Fatalf("expand wallet: %v", err)
	}
	if created.Order.Status != "PENDING_PAYMENT" {
		t.Fatalf("order=%+v", created.Order)
	}
	submitted, err := pg.MarketplaceSubmitWalletExpandPayment(MarketplaceSubmitPaymentRequest{
		OpID:        "expand_wallet_submit_" + suffix,
		AccountID:   account.ID,
		OrderID:     created.Order.ID,
		TxSignature: "pay_sig_" + suffix,
	})
	if err != nil {
		t.Fatalf("submit payment: %v", err)
	}
	if submitted.Status != "SUBMITTED" {
		t.Fatalf("submitted=%+v", submitted)
	}
	fulfilled, err := pg.ConfirmPaymentOrder(ctx, created.Order.ID, "test confirm")
	if err != nil {
		t.Fatalf("confirm payment: %v", err)
	}
	if fulfilled.Status != "FULFILLED" {
		t.Fatalf("fulfilled=%+v", fulfilled)
	}
	slots, err := pg.MarketplaceSlots(account.ID, rules)
	if err != nil {
		t.Fatalf("slots: %v", err)
	}
	if slots.WalletExpandCount != 1 || slots.Capacity != rules.BaseListingSlots+rules.WalletExpandSlots {
		t.Fatalf("slots=%+v", slots)
	}
}

func TestPostgresBagExpandAndTradingLicense(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx := context.Background()
	pg, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer pg.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	wallet := "bag_license_wallet_" + suffix
	defer func() {
		_, _ = pg.pool.Exec(ctx, `DELETE FROM accounts WHERE solana_wallet_address = $1`, wallet)
	}()

	account := pg.UpsertAccountByWallet(wallet)
	character, err := pg.CreateCharacter(account.ID, "BagLicenseHero")
	if err != nil {
		t.Fatalf("character: %v", err)
	}

	depositOwner := "TreasuryBagLicense" + suffix
	growth := GrowthPaymentRules{
		DepositReceiverWallet:    depositOwner,
		BagSlots:                 25,
		BagExpandSlots:           5,
		BagExpandMaxTimes:        10,
		BagExpandPriceToken:      50,
		TradingLicensePriceToken: 100,
	}.withDefaults()

	snap, err := pg.EconomySnapshot(account.ID, character.ID)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.BagSlots != 25 || snap.HasLicense || snap.BagExpandCount != 0 {
		t.Fatalf("initial snapshot=%+v", snap)
	}

	_, err = pg.MarketplaceCreateListing(MarketplaceListRequest{
		OpID:         "no_license_list_" + suffix,
		AccountID:    account.ID,
		CharacterID:  character.ID,
		AssetType:    "EQUIPMENT",
		EquipmentUID: "missing",
		PriceToken:   10,
		Rules:        MarketplaceRules{Enabled: true}.withDefaults(),
	})
	if err == nil || !strings.Contains(err.Error(), "trading license required") {
		t.Fatalf("expected license required, got %v", err)
	}

	bagOrder, err := pg.CreateBagExpandPayment(GrowthPaymentRequest{
		OpID:        "bag_expand_" + suffix,
		AccountID:   account.ID,
		CharacterID: character.ID,
		Rules:       growth,
	})
	if err != nil {
		t.Fatalf("bag expand create: %v", err)
	}
	if bagOrder.Order.Purpose != PaymentPurposeBagExpand || bagOrder.Order.Amount != 50 {
		t.Fatalf("bag order=%+v", bagOrder.Order)
	}
	submitted, err := pg.MarketplaceSubmitWalletExpandPayment(MarketplaceSubmitPaymentRequest{
		OpID:        "bag_expand_submit_" + suffix,
		AccountID:   account.ID,
		OrderID:     bagOrder.Order.ID,
		TxSignature: "bag_pay_" + suffix,
	})
	if err != nil {
		t.Fatalf("bag submit: %v", err)
	}
	if submitted.Status != "SUBMITTED" {
		t.Fatalf("submitted=%+v", submitted)
	}
	fulfilled, err := pg.ConfirmPaymentOrder(ctx, bagOrder.Order.ID, "test bag")
	if err != nil {
		t.Fatalf("bag confirm: %v", err)
	}
	if fulfilled.Status != "FULFILLED" {
		t.Fatalf("fulfilled=%+v", fulfilled)
	}
	snap, err = pg.EconomySnapshot(account.ID, character.ID)
	if err != nil {
		t.Fatalf("snapshot after bag: %v", err)
	}
	if snap.BagExpandCount != 1 || snap.BagSlots != 30 {
		t.Fatalf("after bag expand snapshot=%+v", snap)
	}

	licOrder, err := pg.CreateTradingLicensePayment(GrowthPaymentRequest{
		OpID:      "license_" + suffix,
		AccountID: account.ID,
		Rules:     growth,
	})
	if err != nil {
		t.Fatalf("license create: %v", err)
	}
	if _, err := pg.MarketplaceSubmitWalletExpandPayment(MarketplaceSubmitPaymentRequest{
		OpID:        "license_submit_" + suffix,
		AccountID:   account.ID,
		OrderID:     licOrder.Order.ID,
		TxSignature: "lic_pay_" + suffix,
	}); err != nil {
		t.Fatalf("license submit: %v", err)
	}
	if _, err := pg.ConfirmPaymentOrder(ctx, licOrder.Order.ID, "test license"); err != nil {
		t.Fatalf("license confirm: %v", err)
	}
	snap, err = pg.EconomySnapshot(account.ID, character.ID)
	if err != nil {
		t.Fatalf("snapshot after license: %v", err)
	}
	if !snap.HasLicense {
		t.Fatalf("expected hasLicense, got %+v", snap)
	}

	_, err = pg.CreateTradingLicensePayment(GrowthPaymentRequest{
		OpID:      "license_again_" + suffix,
		AccountID: account.ID,
		Rules:     growth,
	})
	if err == nil || !strings.Contains(err.Error(), "already owned") {
		t.Fatalf("expected already owned, got %v", err)
	}
}

func TestPostgresAccountSessionAndOnline(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx := context.Background()
	pg, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer pg.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	wallet := "session_wallet_" + suffix
	serverID := "srv_" + suffix
	defer func() {
		_, _ = pg.pool.Exec(ctx, `DELETE FROM accounts WHERE solana_wallet_address = $1`, wallet)
		_, _ = pg.pool.Exec(ctx, `DELETE FROM game_servers WHERE server_id = $1`, serverID)
	}()

	account := pg.UpsertAccountByWallet(wallet)
	character, err := pg.CreateCharacter(account.ID, "SessionHero")
	if err != nil {
		t.Fatal(err)
	}
	sessionID := "session_" + suffix
	refresh := "refresh_" + suffix
	expires := time.Now().UTC().Add(24 * time.Hour)
	sess, err := pg.CreateAccountSession(CreateSessionRequest{
		SessionID: sessionID, AccountID: account.ID, RefreshToken: refresh,
		WalletPlugin: "phantom", DeviceID: "d1", IPAddress: "127.0.0.1", ExpiresAt: expires,
	})
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if sess.Status != "ACTIVE" {
		t.Fatalf("sess=%+v", sess)
	}
	rec, err := pg.LookupRefreshToken(refresh, time.Now().UTC())
	if err != nil || rec.SessionID != sessionID {
		t.Fatalf("refresh lookup=%+v err=%v", rec, err)
	}
	newRefresh := "refresh2_" + suffix
	if err := pg.RotateRefreshToken(refresh, newRefresh, sessionID, account.ID, expires, time.Now().UTC()); err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if _, err := pg.LookupRefreshToken(refresh, time.Now().UTC()); err == nil {
		t.Fatal("old refresh should fail")
	}
	server, err := pg.UpsertGameServer(GameServer{
		ServerID: serverID, DisplayName: "Test", Host: "127.0.0.1", Port: 7777, Status: "ONLINE",
	})
	if err != nil {
		t.Fatal(err)
	}
	online, err := pg.EnterOnlineSession(OnlineSession{
		AccountID: account.ID, CharacterID: character.ID, SessionID: sessionID,
		ServerID: server.ServerID, ConnectionID: "c1",
	})
	if err != nil {
		t.Fatalf("enter online: %v", err)
	}
	if online.ServerID != serverID {
		t.Fatalf("online=%+v", online)
	}
	listed, err := pg.ListOnlineByServer(serverID)
	if err != nil || len(listed) != 1 {
		t.Fatalf("listed=%v err=%v", listed, err)
	}
	if _, err := pg.LeaveOnlineSession(account.ID, "c1"); err != nil {
		t.Fatal(err)
	}
	if err := pg.RevokeAccountSession(sessionID, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	got, err := pg.GetAccountSession(sessionID)
	if err != nil || got.Status != "REVOKED" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestPostgresEquipmentRepairAndWear(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx := context.Background()
	pg, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	defer pg.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	wallet := "repair_wallet_" + suffix
	itemID := "repair_sword_" + suffix
	equipUID := "repair_eq_" + suffix
	defer func() {
		_, _ = pg.pool.Exec(ctx, `DELETE FROM accounts WHERE solana_wallet_address = $1`, wallet)
		_, _ = pg.pool.Exec(ctx, `DELETE FROM item_catalog WHERE item_id = $1`, itemID)
	}()

	account := pg.UpsertAccountByWallet(wallet)
	character, err := pg.CreateCharacter(account.ID, "RepairHero")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pg.pool.Exec(ctx, `
		INSERT INTO item_catalog (item_id, name, category, rarity, stackable, tradable, default_bind_type)
		VALUES ($1, $1, 'weapon', 2, FALSE, FALSE, 'BOUND')
	`, itemID); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.pool.Exec(ctx, `
		INSERT INTO equipment_items (
			equipment_uid, account_id, character_id, item_id, location, slot, equip_slot,
			durability, max_durability, rarity
		) VALUES ($1, $2, $3, $4, 'EQUIPPED', NULL, 1, 40, 100, 2)
	`, equipUID, account.ID, character.ID, itemID); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.GrantLocked(account.ID, 200, "test", "repair-fund-"+suffix, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	_ = pg.SettleUnlocks(time.Now().UTC(), 20)

	// Simulate combat wear: 40 -> 35 missing would be wrong; start at 40/100 so missing=60 after -5?
	// Equipment starts at 40; wear to 35 then repair restores to 100 (65 points).
	if _, err := pg.pool.Exec(ctx, `
		UPDATE equipment_items SET durability = 35 WHERE equipment_uid = $1
	`, equipUID); err != nil {
		t.Fatal(err)
	}

	repaired, err := pg.EquipmentRepair(EquipmentRepairRequest{
		OpID:         "repair_" + suffix,
		AccountID:    account.ID,
		CharacterID:  character.ID,
		EquipmentUID: equipUID,
		Rules:        EquipmentRules{}.WithDefaults(),
	})
	if err != nil {
		t.Fatalf("repair: %v", err)
	}
	if repaired.Repaired != 65 || repaired.Equipment.Durability != 100 || repaired.CostToken != 65 {
		t.Fatalf("repaired=%+v equipment=%+v", repaired, repaired.Equipment)
	}
	token := pg.Token(account.ID)
	if token.TokenBalance != 200-65 {
		t.Fatalf("token after repair = %+v, want balance 135", token)
	}

	_, err = pg.EquipmentRepair(EquipmentRepairRequest{
		OpID:         "repair_again_" + suffix,
		AccountID:    account.ID,
		CharacterID:  character.ID,
		EquipmentUID: equipUID,
		Rules:        EquipmentRules{}.WithDefaults(),
	})
	if err == nil || !strings.Contains(err.Error(), "does not need repair") {
		t.Fatalf("expected no repair needed, got %v", err)
	}
}

func TestPostgresNFTMintRequestConfirm(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx := context.Background()
	pg, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pg.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	wallet := "nft_wallet_" + suffix
	itemID := "nft_item_" + suffix
	equipUID := "nft_eq_" + suffix
	defer func() {
		_, _ = pg.pool.Exec(ctx, `DELETE FROM accounts WHERE solana_wallet_address = $1`, wallet)
		_, _ = pg.pool.Exec(ctx, `DELETE FROM item_catalog WHERE item_id = $1`, itemID)
	}()

	account := pg.UpsertAccountByWallet(wallet)
	character, err := pg.CreateCharacter(account.ID, "NFTHero")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pg.pool.Exec(ctx, `
		INSERT INTO item_catalog (item_id, name, category, rarity, stackable, tradable, default_bind_type)
		VALUES ($1, $1, 'weapon', 4, FALSE, TRUE, 'UNBOUND')
	`, itemID); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.pool.Exec(ctx, `
		INSERT INTO equipment_items (
			equipment_uid, account_id, character_id, item_id, location, slot, rarity, bind_type, durability, max_durability
		) VALUES ($1, $2, $3, $4, 'IN_BAG', 0, 4, 'UNBOUND', 100, 100)
	`, equipUID, account.ID, character.ID, itemID); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.GrantLocked(account.ID, 500, "test", "nft-fund-"+suffix, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	_ = pg.SettleUnlocks(time.Now().UTC(), 20)

	requested, err := pg.RequestNFTMint(NFTMintRequestInput{
		OpID: "nft_req_" + suffix, AccountID: account.ID, CharacterID: character.ID,
		EquipmentUID: equipUID, Rules: NFTRules{}.WithDefaults(),
	})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if requested.Request.Status != "PAID" || requested.Asset.Status != "MINT_REQUESTED" {
		t.Fatalf("requested=%+v", requested)
	}
	confirmed, err := pg.ConfirmNFTMint(NFTMintConfirmInput{
		OpID: "nft_confirm_" + suffix, RequestID: requested.Request.ID,
		MintAddress: "MintAddr" + suffix,
	})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if confirmed.Asset.Status != "MINTED" || confirmed.Asset.MintAddress == "" {
		t.Fatalf("confirmed=%+v", confirmed)
	}
	assets, err := pg.ListNFTAssets(account.ID)
	if err != nil || len(assets) != 1 || assets[0].Status != "MINTED" {
		t.Fatalf("assets=%v err=%v", assets, err)
	}
}

func TestPostgresAdminOps(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	ctx := context.Background()
	pg, err := NewPostgres(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pg.Close()

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	wallet := "admin_ops_wallet_" + suffix
	defer func() {
		_, _ = pg.pool.Exec(ctx, `DELETE FROM accounts WHERE solana_wallet_address = $1`, wallet)
		_, _ = pg.pool.Exec(ctx, `DELETE FROM hot_wallet_status WHERE wallet = $1`, "hot_"+wallet)
	}()

	account := pg.UpsertAccountByWallet(wallet)

	if err := pg.SetAccountBan(account.ID, true, "abuse"); err != nil {
		t.Fatal(err)
	}
	detail, err := pg.AdminGetAccount(account.ID, "")
	if err != nil || !detail.IsBanned || detail.BanReason != "abuse" {
		t.Fatalf("detail=%+v err=%v", detail, err)
	}
	if err := pg.SetAccountBan(account.ID, false, ""); err != nil {
		t.Fatal(err)
	}

	if err := pg.SetAccountRiskLevel(account.ID, 42); err != nil {
		t.Fatal(err)
	}
	if err := pg.SetTradingLicense(account.ID, true); err != nil {
		t.Fatal(err)
	}
	detail, err = pg.AdminGetAccount(0, wallet)
	if err != nil || detail.RiskLevel != 42 || !detail.HasTradingLicense {
		t.Fatalf("detail after risk/license=%+v err=%v", detail, err)
	}

	restriction, err := pg.CreateMarketRestriction(CreateMarketRestrictionInput{
		AccountID:       account.ID,
		RestrictionType: "SELL",
		Reason:          "wash trade",
		CreatedBy:       "ops",
	})
	if err != nil {
		t.Fatal(err)
	}
	active, err := pg.ListMarketRestrictions(account.ID, true, 10, 0)
	if err != nil || len(active) != 1 || active[0].ID != restriction.ID {
		t.Fatalf("active=%v err=%v", active, err)
	}
	revoked, err := pg.RevokeMarketRestriction(restriction.ID, "ops", "cleared")
	if err != nil || revoked.RevokedAt == nil {
		t.Fatalf("revoked=%+v err=%v", revoked, err)
	}

	event, err := pg.CreateRiskEvent(CreateRiskEventInput{
		AccountID: account.ID,
		EventType: "CLUSTER_IP",
		Severity:  30,
		IPAddress: "127.0.0.1",
		Detail:    map[string]any{"note": "test"},
	})
	if err != nil || event.ID == 0 {
		t.Fatalf("event=%+v err=%v", event, err)
	}
	events, err := pg.ListRiskEvents(account.ID, 10, 0)
	if err != nil || len(events) == 0 {
		t.Fatalf("events=%v err=%v", events, err)
	}

	audit := pg.AuditTarget("ops", "test_action", "account", fmt.Sprint(account.ID), "unit")
	if audit.ID == 0 {
		t.Fatal("audit missing")
	}
	audits, err := pg.ListAudits(10, 0)
	if err != nil || len(audits) == 0 {
		t.Fatalf("audits=%v err=%v", audits, err)
	}

	hot, err := pg.SetHotWalletPayoutsPaused("hot_"+wallet, "solana-devnet", "", true)
	if err != nil || !hot.PayoutsPaused {
		t.Fatalf("hot=%+v err=%v", hot, err)
	}
	got, err := pg.GetHotWalletStatus("hot_" + wallet)
	if err != nil || !got.PayoutsPaused {
		t.Fatalf("got=%+v err=%v", got, err)
	}

	payments, err := pg.ListPaymentOrdersAdmin(AdminListFilter{AccountID: account.ID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	_ = payments
	requests, err := pg.ListNFTMintRequests(AdminListFilter{AccountID: account.ID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	_ = requests
}
