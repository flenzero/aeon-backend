package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresStore struct {
	pool *pgxpool.Pool
}

type postgresReader interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func NewPostgres(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &PostgresStore{pool: pool}, nil
}

func (s *PostgresStore) Close() {
	s.pool.Close()
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *PostgresStore) HasSchemaVersion(ctx context.Context, version string) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)
	`, strings.TrimSpace(version)).Scan(&exists)
	return exists, err
}

func (s *PostgresStore) SaveWalletNonce(row WalletLoginNonce) {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO wallet_login_nonces (nonce, wallet_address, message, status, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, row.Nonce, row.Wallet, row.Message, pending(row.Status), row.ExpiresAt, row.CreatedAt)
	must(err, "save wallet nonce")
}

func (s *PostgresStore) WalletNonce(nonce, wallet string, now time.Time) (WalletLoginNonce, error) {
	var row WalletLoginNonce
	err := s.pool.QueryRow(context.Background(), `
		SELECT nonce, wallet_address, message, status, expires_at, created_at
		FROM wallet_login_nonces
		WHERE nonce = $1 AND wallet_address = $2
	`, nonce, wallet).Scan(&row.Nonce, &row.Wallet, &row.Message, &row.Status, &row.ExpiresAt, &row.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return WalletLoginNonce{}, ErrNotFound
	}
	if err != nil {
		return WalletLoginNonce{}, err
	}
	if row.Status != "PENDING" {
		return WalletLoginNonce{}, ErrForbidden
	}
	if !row.ExpiresAt.After(now) {
		_, _ = s.pool.Exec(context.Background(), `
			UPDATE wallet_login_nonces SET status = 'EXPIRED'
			WHERE nonce = $1 AND status = 'PENDING'
		`, nonce)
		return WalletLoginNonce{}, ErrForbidden
	}
	return row, nil
}

func (s *PostgresStore) ConsumeWalletNonce(nonce, wallet string, now time.Time) error {
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
		FROM wallet_login_nonces
		WHERE nonce = $1 AND wallet_address = $2
		FOR UPDATE
	`, nonce, wallet).Scan(&status, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != "PENDING" || !expiresAt.After(now) {
		if status == "PENDING" {
			_, _ = tx.Exec(ctx, `
				UPDATE wallet_login_nonces SET status = 'EXPIRED'
				WHERE nonce = $1
			`, nonce)
		}
		return ErrForbidden
	}
	_, err = tx.Exec(ctx, `
		UPDATE wallet_login_nonces SET status = 'CONSUMED', consumed_at = $2
		WHERE nonce = $1
	`, nonce, now)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *PostgresStore) UpsertAccountByWallet(wallet string) Account {
	ctx := context.Background()
	username := "wallet_" + walletSuffix(wallet)
	var account Account
	var status string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO accounts (username, solana_wallet_address, last_login_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (solana_wallet_address)
		DO UPDATE SET last_login_at = NOW(), updated_at = NOW()
		RETURNING id, username, solana_wallet_address, status, created_at, last_login_at
	`, username, wallet).Scan(
		&account.ID,
		&account.Username,
		&account.WalletAddress,
		&status,
		&account.CreatedAt,
		&account.LastLoginAt,
	)
	must(err, "upsert account")
	_, err = s.pool.Exec(ctx, `
		INSERT INTO account_tokens (account_id)
		VALUES ($1)
		ON CONFLICT (account_id) DO NOTHING
	`, account.ID)
	must(err, "ensure account token")
	account.IsBanned = status == "BANNED"
	return account
}

func (s *PostgresStore) Account(id int64) (Account, bool) {
	ctx := context.Background()
	var account Account
	var status string
	err := s.pool.QueryRow(ctx, `
		SELECT id, username, COALESCE(solana_wallet_address, ''), status, created_at, COALESCE(last_login_at, created_at)
		FROM accounts
		WHERE id = $1
	`, id).Scan(&account.ID, &account.Username, &account.WalletAddress, &status, &account.CreatedAt, &account.LastLoginAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Account{}, false
	}
	must(err, "get account")
	account.IsBanned = status == "BANNED"
	return account, true
}

func (s *PostgresStore) CreateCharacter(accountID int64, name string) (Character, error) {
	return s.CreateCharacterWithAppearance(accountID, name, nil)
}

func (s *PostgresStore) CreateCharacterWithAppearance(accountID int64, name string, appearance map[string]any) (Character, error) {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Character{}, err
	}
	defer rollback(ctx, tx)
	appearanceBytes, err := json.Marshal(nonNilMap(appearance))
	if err != nil {
		return Character{}, err
	}
	rows, err := tx.Query(ctx, `
		SELECT slot_index
		FROM characters
		WHERE account_id = $1 AND is_deleted = FALSE
		FOR UPDATE
	`, accountID)
	if err != nil {
		return Character{}, err
	}
	used := map[int]bool{}
	for rows.Next() {
		var slot int
		if err := rows.Scan(&slot); err != nil {
			rows.Close()
			return Character{}, err
		}
		used[slot] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Character{}, err
	}
	slotIndex := -1
	for candidate := 0; candidate < 3; candidate++ {
		if !used[candidate] {
			slotIndex = candidate
			break
		}
	}
	if slotIndex < 0 {
		return Character{}, errors.New("character slots are full")
	}
	var character Character
	var appearanceRaw []byte
	err = tx.QueryRow(ctx, `
		INSERT INTO characters (account_id, name, slot_index, appearance)
		VALUES ($1, $2, $3, $4)
		RETURNING id, account_id, name, slot_index, level, appearance, created_at, is_deleted
	`, accountID, name, slotIndex, appearanceBytes).Scan(
		&character.ID,
		&character.AccountID,
		&character.Name,
		&character.SlotIndex,
		&character.Level,
		&appearanceRaw,
		&character.CreatedAt,
		&character.Deleted,
	)
	if err != nil {
		return Character{}, err
	}
	if len(appearanceRaw) > 0 {
		if err := json.Unmarshal(appearanceRaw, &character.Appearance); err != nil {
			return Character{}, err
		}
	}
	character.LastPlayed = time.Unix(0, 0).UTC()
	_, err = tx.Exec(ctx, `
		INSERT INTO character_wallets (character_id)
		VALUES ($1)
		ON CONFLICT (character_id) DO NOTHING
	`, character.ID)
	if err != nil {
		return Character{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO character_states (character_id)
		VALUES ($1)
		ON CONFLICT (character_id) DO NOTHING
	`, character.ID)
	if err != nil {
		return Character{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Character{}, err
	}
	return s.attachCharacterEquipment(context.Background(), character)
}

func (s *PostgresStore) Characters(accountID int64) []Character {
	rows, err := s.pool.Query(context.Background(), `
		SELECT c.id, c.account_id, c.name, c.slot_index, c.level, c.appearance, c.created_at, COALESCE(st.last_played_at, 'epoch'::timestamptz), st.last_played_at IS NOT NULL, c.is_deleted
		FROM characters c
		LEFT JOIN character_states st ON st.character_id = c.id
		WHERE c.account_id = $1 AND c.is_deleted = FALSE
		ORDER BY c.slot_index, c.id
	`, accountID)
	must(err, "list characters")
	defer rows.Close()
	var out []Character
	for rows.Next() {
		var row Character
		var appearanceRaw []byte
		must(rows.Scan(
			&row.ID,
			&row.AccountID,
			&row.Name,
			&row.SlotIndex,
			&row.Level,
			&appearanceRaw,
			&row.CreatedAt,
			&row.LastPlayed,
			&row.HasLastPlayed,
			&row.Deleted,
		), "scan character")
		if len(appearanceRaw) > 0 {
			must(json.Unmarshal(appearanceRaw, &row.Appearance), "decode character appearance")
		}
		withEquipment, err := s.attachCharacterEquipment(context.Background(), row)
		must(err, "attach character equipment")
		out = append(out, withEquipment)
	}
	must(rows.Err(), "iterate characters")
	return out
}

func (s *PostgresStore) Character(accountID, characterID int64) (Character, bool) {
	var row Character
	var appearanceRaw []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT c.id, c.account_id, c.name, c.slot_index, c.level, c.appearance, c.created_at, COALESCE(st.last_played_at, 'epoch'::timestamptz), st.last_played_at IS NOT NULL, c.is_deleted
		FROM characters c
		LEFT JOIN character_states st ON st.character_id = c.id
		WHERE c.account_id = $1 AND c.id = $2 AND c.is_deleted = FALSE
	`, accountID, characterID).Scan(
		&row.ID,
		&row.AccountID,
		&row.Name,
		&row.SlotIndex,
		&row.Level,
		&appearanceRaw,
		&row.CreatedAt,
		&row.LastPlayed,
		&row.HasLastPlayed,
		&row.Deleted,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Character{}, false
	}
	must(err, "get character")
	if len(appearanceRaw) > 0 {
		must(json.Unmarshal(appearanceRaw, &row.Appearance), "decode character appearance")
	}
	row, err = s.attachCharacterEquipment(context.Background(), row)
	must(err, "attach character equipment")
	return row, true
}

func nonNilMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	return in
}

func (s *PostgresStore) attachCharacterEquipment(ctx context.Context, character Character) (Character, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			e.id,
			e.equipment_uid,
			e.item_id,
			e.rarity,
			e.enhance_level,
			COALESCE(e.durability, 0),
			COALESCE(e.max_durability, 0),
			e.location,
			COALESCE(e.equip_slot, -1),
			COALESCE(e.slot, -1),
			e.affixes,
			COALESCE(n.mint_address, '')
		FROM equipment_items e
		LEFT JOIN nft_assets n
			ON n.source_asset_type = 'EQUIPMENT'
			AND n.source_asset_id = e.id
			AND n.status IN ('MINT_REQUESTED', 'MINTED')
		WHERE e.character_id = $1 AND e.location = 'EQUIPPED'
		ORDER BY e.equip_slot, e.id
	`, character.ID)
	if err != nil {
		return Character{}, err
	}
	defer rows.Close()
	character.EquipmentItems = []EquipmentItem{}
	for rows.Next() {
		var row EquipmentItem
		var nftContract string
		var affixes []byte
		if err := rows.Scan(
			&row.ID,
			&row.EquipmentUID,
			&row.ItemID,
			&row.Rarity,
			&row.EnhanceLevel,
			&row.Durability,
			&row.MaxDurability,
			&row.Status,
			&row.EquipSlot,
			&row.Slot,
			&affixes,
			&nftContract,
		); err != nil {
			return Character{}, err
		}
		if len(affixes) > 0 {
			if err := json.Unmarshal(affixes, &row.Affixes); err != nil {
				return Character{}, err
			}
		}
		if nftContract != "" {
			row.NFTContract = &nftContract
		}
		character.EquipmentItems = append(character.EquipmentItems, row)
	}
	if err := rows.Err(); err != nil {
		return Character{}, err
	}
	if character.Appearance == nil {
		character.Appearance = map[string]any{}
	}
	return character, nil
}

func (s *PostgresStore) EconomySnapshot(accountID, characterID int64) (EconomySnapshot, error) {
	return s.economySnapshot(context.Background(), s.pool, accountID, characterID)
}

func (s *PostgresStore) WarehouseDeposit(req EconomyActionRequest) (EconomySnapshot, error) {
	return s.runEconomyAction("warehouse_deposit", req, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return err
		}
		if strings.TrimSpace(req.EquipmentUID) != "" {
			targetSlot, err := s.resolveStorageSlot(ctx, tx, req.CharacterID, "WAREHOUSE", req.SlotIndex)
			if err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `
				UPDATE equipment_items
				SET location = 'IN_WAREHOUSE', slot = $4, updated_at = NOW()
				WHERE account_id = $1
					AND character_id = $2
					AND equipment_uid = $3
					AND location = 'IN_BAG'
			`, req.AccountID, req.CharacterID, strings.TrimSpace(req.EquipmentUID), targetSlot)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return ErrForbidden
			}
			return s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "EQUIPMENT_WAREHOUSE_DEPOSIT", "equipment", 1, req.OpID)
		}
		return s.moveInventory(ctx, tx, req, "BAG", "WAREHOUSE", "INVENTORY_WAREHOUSE_DEPOSIT")
	})
}

func (s *PostgresStore) WarehouseWithdraw(req EconomyActionRequest) (EconomySnapshot, error) {
	return s.runEconomyAction("warehouse_withdraw", req, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return err
		}
		if strings.TrimSpace(req.EquipmentUID) != "" {
			targetSlot, err := s.resolveStorageSlot(ctx, tx, req.CharacterID, "BAG", req.SlotIndex)
			if err != nil {
				return err
			}
			tag, err := tx.Exec(ctx, `
				UPDATE equipment_items
				SET location = 'IN_BAG', slot = $4, updated_at = NOW()
				WHERE account_id = $1
					AND character_id = $2
					AND equipment_uid = $3
					AND location = 'IN_WAREHOUSE'
			`, req.AccountID, req.CharacterID, strings.TrimSpace(req.EquipmentUID), targetSlot)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return ErrForbidden
			}
			return s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "EQUIPMENT_WAREHOUSE_WITHDRAW", "equipment", 1, req.OpID)
		}
		return s.moveInventory(ctx, tx, req, "WAREHOUSE", "BAG", "INVENTORY_WAREHOUSE_WITHDRAW")
	})
}

func (s *PostgresStore) EquipItem(req EconomyActionRequest) (EconomySnapshot, error) {
	return s.runEconomyAction("equipment_equip", req, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return err
		}
		equipmentUID := strings.TrimSpace(req.EquipmentUID)
		if equipmentUID == "" {
			return errors.New("equipmentUid is required")
		}
		var equipmentID int64
		var currentEquipSlot int
		var durability, maxDurability *int
		err := tx.QueryRow(ctx, `
			SELECT id, COALESCE(equip_slot, -1), durability, max_durability
			FROM equipment_items
			WHERE account_id = $1
				AND character_id = $2
				AND equipment_uid = $3
				AND location = 'IN_BAG'
			FOR UPDATE
		`, req.AccountID, req.CharacterID, equipmentUID).Scan(&equipmentID, &currentEquipSlot, &durability, &maxDurability)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrForbidden
		}
		if err != nil {
			return err
		}
		maxD := 0
		if maxDurability != nil {
			maxD = *maxDurability
		}
		cur := 0
		if durability != nil {
			cur = *durability
		}
		if maxD > 0 && cur <= 0 {
			return errors.New("equipment is broken and must be repaired")
		}
		targetEquipSlot := req.EquipSlot
		if targetEquipSlot < 0 {
			targetEquipSlot = currentEquipSlot
		}
		if targetEquipSlot < 0 {
			return errors.New("equipSlot is required")
		}

		rows, err := tx.Query(ctx, `
			SELECT id
			FROM equipment_items
			WHERE character_id = $1
				AND location = 'EQUIPPED'
				AND equip_slot = $2
			FOR UPDATE
		`, req.CharacterID, targetEquipSlot)
		if err != nil {
			return err
		}
		var replaced []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			replaced = append(replaced, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, id := range replaced {
			targetSlot, err := s.resolveStorageSlot(ctx, tx, req.CharacterID, "BAG", -1)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				UPDATE equipment_items
				SET location = 'IN_BAG', slot = $2, updated_at = NOW()
				WHERE id = $1
			`, id, targetSlot); err != nil {
				return err
			}
		}
		tag, err := tx.Exec(ctx, `
			UPDATE equipment_items
			SET location = 'EQUIPPED', slot = NULL, equip_slot = $3, updated_at = NOW()
			WHERE id = $1 AND character_id = $2
		`, equipmentID, req.CharacterID, targetEquipSlot)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrForbidden
		}
		return s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "EQUIPMENT_EQUIPPED", "equipment", 1, req.OpID)
	})
}

func (s *PostgresStore) UnequipItem(req EconomyActionRequest) (EconomySnapshot, error) {
	return s.runEconomyAction("equipment_unequip", req, func(ctx context.Context, tx pgx.Tx) error {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return err
		}
		targetSlot, err := s.resolveStorageSlot(ctx, tx, req.CharacterID, "BAG", req.SlotIndex)
		if err != nil {
			return err
		}
		var tag pgconn.CommandTag
		equipmentUID := strings.TrimSpace(req.EquipmentUID)
		if equipmentUID != "" {
			tag, err = tx.Exec(ctx, `
				UPDATE equipment_items
				SET location = 'IN_BAG', slot = $4, updated_at = NOW()
				WHERE account_id = $1
					AND character_id = $2
					AND equipment_uid = $3
					AND location = 'EQUIPPED'
			`, req.AccountID, req.CharacterID, equipmentUID, targetSlot)
		} else {
			if req.EquipSlot < 0 {
				return errors.New("equipmentUid or equipSlot is required")
			}
			tag, err = tx.Exec(ctx, `
				UPDATE equipment_items
				SET location = 'IN_BAG', slot = $4, updated_at = NOW()
				WHERE account_id = $1
					AND character_id = $2
					AND equip_slot = $3
					AND location = 'EQUIPPED'
			`, req.AccountID, req.CharacterID, req.EquipSlot, targetSlot)
		}
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrForbidden
		}
		return s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "EQUIPMENT_UNEQUIPPED", "equipment", 1, req.OpID)
	})
}

func (s *PostgresStore) DungeonEnter(req DungeonEnterRequest) (DungeonResult, error) {
	if req.ChapterID < 0 {
		return DungeonResult{}, errors.New("chapterId is invalid")
	}
	if req.FloorID < 0 {
		return DungeonResult{}, errors.New("floorId is invalid")
	}
	return runIdempotentAction(s, "dungeon_enter", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (DungeonResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return DungeonResult{}, err
		}
		cost := req.Cost
		if cost.Items == nil {
			cost.Items = []InventoryItem{}
		}
		for index := range cost.Items {
			cost.Items[index].ItemID = strings.TrimSpace(cost.Items[index].ItemID)
			if cost.Items[index].ItemID == "" {
				return DungeonResult{}, errors.New("dungeon cost itemId is required")
			}
			if cost.Items[index].Quantity <= 0 {
				return DungeonResult{}, errors.New("dungeon cost item quantity must be positive")
			}
		}
		costBytes, err := json.Marshal(cost)
		if err != nil {
			return DungeonResult{}, err
		}
		if cost.Gold > 0 {
			tag, err := tx.Exec(ctx, `
				UPDATE character_wallets
				SET gold = gold - $2, updated_at = NOW()
				WHERE character_id = $1 AND gold >= $2
			`, req.CharacterID, cost.Gold)
			if err != nil {
				return DungeonResult{}, err
			}
			if tag.RowsAffected() == 0 {
				return DungeonResult{}, ErrInsufficientBalance
			}
		}
		for _, item := range cost.Items {
			if err := s.consumeBagItem(ctx, tx, req.CharacterID, item.ItemID, item.Quantity); err != nil {
				return DungeonResult{}, fmt.Errorf("dungeon cost item %s: %w", item.ItemID, err)
			}
		}
		var dungeonRunID string
		err = tx.QueryRow(ctx, `
			INSERT INTO dungeon_runs (account_id, character_id, origin_server_id, dungeon_key, status, enter_cost)
			VALUES ($1, $2, NULLIF($3, ''), $4, 'STARTED', $5)
			RETURNING id::text
		`, req.AccountID, req.CharacterID, strings.TrimSpace(req.ServerID), dungeonKey(req.ChapterID, req.FloorID), costBytes).Scan(&dungeonRunID)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "uq_dungeon_runs_active_character" {
				return DungeonResult{}, ErrForbidden
			}
			return DungeonResult{}, err
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "DUNGEON_ENTERED", dungeonRunID, 0, strings.TrimSpace(req.OpID)); err != nil {
			return DungeonResult{}, err
		}
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		if err != nil {
			return DungeonResult{}, err
		}
		return DungeonResult{
			DungeonRunID: dungeonRunID,
			ChapterID:    req.ChapterID,
			FloorID:      req.FloorID,
			IsBoss:       req.IsBoss,
			Status:       "IN_PROGRESS",
			Cost:         cost,
			Rewards: DungeonRewards{
				TokenReward: "0",
				Items:       []InventoryItem{},
			},
			DiscardedRewards: DungeonDiscardedRewards{Items: []InventoryItem{}},
			Snapshot:         snapshot,
		}, nil
	})
}

func (s *PostgresStore) ActiveDungeonRun(accountID, characterID int64) (DungeonRunRecovery, error) {
	var row DungeonRunRecovery
	var dungeonKeyValue string
	err := s.pool.QueryRow(context.Background(), `
		SELECT id::text, account_id, character_id, COALESCE(origin_server_id, ''), dungeon_key, started_at
		FROM dungeon_runs
		WHERE account_id = $1 AND character_id = $2 AND status = 'STARTED'
		ORDER BY started_at DESC
		LIMIT 1
	`, accountID, characterID).Scan(
		&row.DungeonRunID, &row.AccountID, &row.CharacterID, &row.ServerID, &dungeonKeyValue, &row.StartedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return DungeonRunRecovery{}, ErrNotFound
	}
	if err != nil {
		return DungeonRunRecovery{}, err
	}
	if _, err := fmt.Sscanf(dungeonKeyValue, "chapter:%d:floor:%d", &row.ChapterID, &row.FloorID); err != nil {
		return DungeonRunRecovery{}, fmt.Errorf("invalid stored dungeon key %q: %w", dungeonKeyValue, err)
	}
	row.Required = true
	return row, nil
}

func (s *PostgresStore) CancelDungeonRun(accountID, characterID int64, dungeonRunID, reason string) (DungeonRunRecovery, error) {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return DungeonRunRecovery{}, err
	}
	defer rollback(ctx, tx)
	var row DungeonRunRecovery
	var dungeonKeyValue string
	err = tx.QueryRow(ctx, `
		SELECT id::text, account_id, character_id, COALESCE(origin_server_id, ''), dungeon_key, status, started_at
		FROM dungeon_runs
		WHERE id = $1 AND account_id = $2 AND character_id = $3
		FOR UPDATE
	`, strings.TrimSpace(dungeonRunID), accountID, characterID).Scan(
		&row.DungeonRunID, &row.AccountID, &row.CharacterID, &row.ServerID, &dungeonKeyValue, &row.Status, &row.StartedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return DungeonRunRecovery{}, ErrNotFound
	}
	if err != nil {
		return DungeonRunRecovery{}, err
	}
	if row.Status != "STARTED" && row.Status != "CANCELLED" {
		return DungeonRunRecovery{}, ErrForbidden
	}
	if row.Status == "STARTED" {
		result, err := json.Marshal(map[string]any{"result": "abandoned", "reason": strings.TrimSpace(reason)})
		if err != nil {
			return DungeonRunRecovery{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE dungeon_runs SET status='CANCELLED', result=$2, finished_at=NOW() WHERE id=$1`, row.DungeonRunID, result); err != nil {
			return DungeonRunRecovery{}, err
		}
		row.Status = "CANCELLED"
	}
	if _, err := tx.Exec(ctx, `
			UPDATE game_tickets
			SET status='CANCELLED'
			WHERE account_id=$1
			  AND (character_id=$2 OR (character_id IS NULL AND server_id=$3))
			  AND status='ACTIVE'
		`, accountID, characterID, row.ServerID); err != nil {
		return DungeonRunRecovery{}, err
	}
	if _, err := fmt.Sscanf(dungeonKeyValue, "chapter:%d:floor:%d", &row.ChapterID, &row.FloorID); err != nil {
		return DungeonRunRecovery{}, fmt.Errorf("invalid stored dungeon key %q: %w", dungeonKeyValue, err)
	}
	row.Required = false
	if err := tx.Commit(ctx); err != nil {
		return DungeonRunRecovery{}, err
	}
	return row, nil
}

func (s *PostgresStore) DungeonFinish(req DungeonFinishRequest) (DungeonResult, error) {
	req.Result = strings.ToLower(strings.TrimSpace(req.Result))
	if strings.TrimSpace(req.DungeonRunID) == "" {
		return DungeonResult{}, errors.New("dungeonRunId is required")
	}
	if req.ChapterID < 0 {
		return DungeonResult{}, errors.New("chapterId is invalid")
	}
	if req.FloorID < 0 {
		return DungeonResult{}, errors.New("floorId is invalid")
	}
	if req.Exp < 0 {
		return DungeonResult{}, errors.New("exp must be non-negative")
	}
	switch req.Result {
	case "victory", "defeat", "timeout":
	default:
		return DungeonResult{}, errors.New("result must be victory, defeat or timeout")
	}
	return runIdempotentAction(s, "dungeon_finish", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (DungeonResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return DungeonResult{}, err
		}
		var storedRunID string
		var storedKey string
		var storedStatus string
		var storedServerID string
		err := tx.QueryRow(ctx, `
			SELECT id::text, dungeon_key, status, COALESCE(origin_server_id, '')
			FROM dungeon_runs
			WHERE id = $1
				AND account_id = $2
				AND character_id = $3
			FOR UPDATE
		`, strings.TrimSpace(req.DungeonRunID), req.AccountID, req.CharacterID).Scan(&storedRunID, &storedKey, &storedStatus, &storedServerID)
		if errors.Is(err, pgx.ErrNoRows) {
			return DungeonResult{}, ErrNotFound
		}
		if err != nil {
			return DungeonResult{}, err
		}
		if storedStatus != "STARTED" {
			return DungeonResult{}, ErrForbidden
		}
		if storedServerID != "" && strings.TrimSpace(req.ServerID) != storedServerID {
			return DungeonResult{}, ErrForbidden
		}
		if storedKey != dungeonKey(req.ChapterID, req.FloorID) {
			return DungeonResult{}, ErrForbidden
		}
		dbStatus := "FAILED"
		responseStatus := "FAILED"
		ledgerKind := "DUNGEON_FAILED"
		if req.Result == "victory" {
			dbStatus = "FINISHED"
			responseStatus = "REWARDED"
			ledgerKind = "DUNGEON_REWARDED"
		}
		resultPayload := map[string]any{
			"chapterId": req.ChapterID,
			"floorId":   req.FloorID,
			"result":    req.Result,
			"exp":       req.Exp,
			"kills":     req.Kills,
			"progress":  req.Progress,
		}
		resultBytes, err := json.Marshal(resultPayload)
		if err != nil {
			return DungeonResult{}, err
		}
		_, err = tx.Exec(ctx, `
			UPDATE dungeon_runs
			SET status = $2, result = $3, finished_at = NOW()
			WHERE id = $1
		`, storedRunID, dbStatus, resultBytes)
		if err != nil {
			return DungeonResult{}, err
		}
		if req.Result == "victory" {
			_, err = tx.Exec(ctx, `
				UPDATE characters
				SET highest_cleared_chapter = CASE
						WHEN highest_cleared_chapter < $2 OR (highest_cleared_chapter = $2 AND highest_cleared_floor < $3) THEN $2
						ELSE highest_cleared_chapter
					END,
					highest_cleared_floor = CASE
						WHEN highest_cleared_chapter < $2 OR (highest_cleared_chapter = $2 AND highest_cleared_floor < $3) THEN $3
						ELSE highest_cleared_floor
					END,
					highest_cleared_at = CASE
						WHEN highest_cleared_chapter < $2 OR (highest_cleared_chapter = $2 AND highest_cleared_floor < $3) THEN NOW()
						ELSE highest_cleared_at
					END,
					dungeon_clear_count = dungeon_clear_count + 1,
					last_dungeon_cleared_at = NOW(), updated_at = NOW()
				WHERE id = $1 AND account_id = $4
			`, req.CharacterID, req.ChapterID, req.FloorID, req.AccountID)
			if err != nil {
				return DungeonResult{}, err
			}
		}
		levelProgress, err := s.applyCharacterExpProgress(ctx, tx, req)
		if err != nil {
			return DungeonResult{}, err
		}
		rewards, err := s.applyDungeonRewards(ctx, tx, req)
		if err != nil {
			return DungeonResult{}, err
		}
		if _, err := s.publishRareRewardAnnouncementsTx(ctx, tx, req.RewardPlan, rareAnnouncementContext{
			AccountID: req.AccountID, CharacterID: req.CharacterID, Source: "副本掉落",
			RefType: "dungeon_run", RefID: storedRunID, AnnouncementOn: req.Result == "victory",
		}); err != nil {
			return DungeonResult{}, err
		}
		if err := s.wearEquippedGear(ctx, tx, req.AccountID, req.CharacterID, req.EquipmentWearPoints, req.DefaultMaxDurability); err != nil {
			return DungeonResult{}, err
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, ledgerKind, storedRunID, req.Exp, strings.TrimSpace(req.OpID)); err != nil {
			return DungeonResult{}, err
		}
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		if err != nil {
			return DungeonResult{}, err
		}
		return DungeonResult{
			DungeonRunID: storedRunID,
			ChapterID:    req.ChapterID,
			FloorID:      req.FloorID,
			IsBoss:       req.RewardPlan.IsBoss,
			Status:       responseStatus,
			Result:       req.Result,
			Cost:         DungeonCost{Items: []InventoryItem{}},
			Rewards: DungeonRewards{
				Exp:            req.Exp,
				LevelsGained:   levelProgress.LevelsGained,
				Level:          snapshot.Level,
				TokenReward:    strconv.FormatInt(req.RewardPlan.TokenReward, 10),
				Items:          rewards.Items,
				EquipmentItems: rewards.EquipmentItems,
			},
			DiscardedRewards: DungeonDiscardedRewards{Items: []InventoryItem{}},
			Snapshot:         snapshot,
		}, nil
	})
}

func (s *PostgresStore) applyCharacterExpProgress(ctx context.Context, tx pgx.Tx, req DungeonFinishRequest) (LevelProgress, error) {
	var currentLevel int
	var totalExp int64
	var err error
	if req.Exp > 0 {
		err = tx.QueryRow(ctx, `
			UPDATE characters
			SET exp = exp + $3, updated_at = NOW()
			WHERE account_id = $1 AND id = $2
			RETURNING level, exp
		`, req.AccountID, req.CharacterID, req.Exp).Scan(&currentLevel, &totalExp)
	} else {
		err = tx.QueryRow(ctx, `
			SELECT level, exp
			FROM characters
			WHERE account_id = $1 AND id = $2
		`, req.AccountID, req.CharacterID).Scan(&currentLevel, &totalExp)
	}
	if err != nil {
		return LevelProgress{}, err
	}
	progress := req.LevelProgression.Progress(currentLevel, totalExp)
	if progress.Level != currentLevel {
		if _, err := tx.Exec(ctx, `
			UPDATE characters
			SET level = $3, updated_at = NOW()
			WHERE account_id = $1 AND id = $2
		`, req.AccountID, req.CharacterID, progress.Level); err != nil {
			return LevelProgress{}, err
		}
	}
	return progress, nil
}

func (s *PostgresStore) LootClaim(req LootActionRequest) (EconomySnapshot, error) {
	return runIdempotentAction(s, "loot_claim", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (EconomySnapshot, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return EconomySnapshot{}, err
		}
		lootID, err := s.resolveLootID(ctx, tx, req)
		if err != nil {
			return EconomySnapshot{}, err
		}
		if err := s.claimLootRow(ctx, tx, req, lootID); err != nil {
			return EconomySnapshot{}, err
		}
		return s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
	})
}

func (s *PostgresStore) LootClaimAll(req LootActionRequest) (EconomySnapshot, error) {
	return runIdempotentAction(s, "loot_claim_all", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (EconomySnapshot, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return EconomySnapshot{}, err
		}
		rows, err := tx.Query(ctx, `
			SELECT id
			FROM loot_tray_items
			WHERE account_id = $1
				AND character_id = $2
				AND status = 'PENDING'
			ORDER BY created_at, id
			FOR UPDATE
		`, req.AccountID, req.CharacterID)
		if err != nil {
			return EconomySnapshot{}, err
		}
		defer rows.Close()
		var lootIDs []int64
		for rows.Next() {
			var lootID int64
			if err := rows.Scan(&lootID); err != nil {
				return EconomySnapshot{}, err
			}
			lootIDs = append(lootIDs, lootID)
		}
		if err := rows.Err(); err != nil {
			return EconomySnapshot{}, err
		}
		for _, lootID := range lootIDs {
			claimReq := req
			claimReq.Quantity = 0
			claimReq.SlotIndex = -1
			if err := s.claimLootRow(ctx, tx, claimReq, lootID); err != nil {
				return EconomySnapshot{}, err
			}
		}
		return s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
	})
}

func (s *PostgresStore) LootDiscard(req LootActionRequest) (EconomySnapshot, error) {
	return runIdempotentAction(s, "loot_discard", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (EconomySnapshot, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return EconomySnapshot{}, err
		}
		lootID, err := s.resolveLootID(ctx, tx, req)
		if err != nil {
			return EconomySnapshot{}, err
		}
		var equipmentID *int64
		err = tx.QueryRow(ctx, `
			SELECT equipment_id
			FROM loot_tray_items
			WHERE id = $1
				AND account_id = $2
				AND character_id = $3
				AND status = 'PENDING'
			FOR UPDATE
		`, lootID, req.AccountID, req.CharacterID).Scan(&equipmentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return EconomySnapshot{}, ErrNotFound
		}
		if err != nil {
			return EconomySnapshot{}, err
		}
		if equipmentID != nil {
			if _, err := tx.Exec(ctx, `
				UPDATE equipment_items
				SET location = 'DELETED', updated_at = NOW()
				WHERE id = $1
					AND account_id = $2
					AND character_id = $3
					AND location = 'IN_LOOT_TRAY'
			`, *equipmentID, req.AccountID, req.CharacterID); err != nil {
				return EconomySnapshot{}, err
			}
		}
		tag, err := tx.Exec(ctx, `
			UPDATE loot_tray_items
			SET status = 'DISCARDED', claimed_at = NOW()
			WHERE id = $1
				AND account_id = $2
				AND character_id = $3
				AND status = 'PENDING'
		`, lootID, req.AccountID, req.CharacterID)
		if err != nil {
			return EconomySnapshot{}, err
		}
		if tag.RowsAffected() == 0 {
			return EconomySnapshot{}, ErrNotFound
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "LOOT_DISCARDED", strconv.FormatInt(lootID, 10), 1, req.OpID); err != nil {
			return EconomySnapshot{}, err
		}
		return s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
	})
}

func (s *PostgresStore) GatheringSettle(req ActivitySettlementRequest) (ActivitySettlementResult, error) {
	req.ActivityType = "gathering"
	return s.runActivitySettlement("gathering_settle", req)
}

func (s *PostgresStore) FarmingHarvest(req ActivitySettlementRequest) (ActivitySettlementResult, error) {
	req.ActivityType = "farming"
	return s.runActivitySettlement("farming_harvest", req)
}

func (s *PostgresStore) BossContribute(req BossContributeRequest) (BossContributeResult, error) {
	if req.Contribution <= 0 {
		return BossContributeResult{}, errors.New("contribution must be positive")
	}
	if req.BossEventID <= 0 {
		return BossContributeResult{}, errors.New("bossEventId is required")
	}
	return runIdempotentAction(s, "boss_contribute", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (BossContributeResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return BossContributeResult{}, err
		}
		bossKey, status, startsAt, endsAt, err := s.lockBossEvent(ctx, tx, req.BossEventID)
		if err != nil {
			return BossContributeResult{}, err
		}
		now := time.Now().UTC()
		if status != "OPEN" {
			return BossContributeResult{}, errors.New("boss event is not open")
		}
		if now.Before(startsAt) || now.After(endsAt) {
			return BossContributeResult{}, errors.New("boss event is outside active window")
		}
		var total int64
		err = tx.QueryRow(ctx, `
			INSERT INTO boss_contributions (boss_event_id, account_id, character_id, contribution)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (boss_event_id, account_id) DO UPDATE SET
				contribution = boss_contributions.contribution + EXCLUDED.contribution,
				character_id = EXCLUDED.character_id,
				updated_at = NOW()
			RETURNING contribution
		`, req.BossEventID, req.AccountID, req.CharacterID, req.Contribution).Scan(&total)
		if err != nil {
			return BossContributeResult{}, err
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "BOSS_CONTRIBUTION", strconv.FormatInt(req.BossEventID, 10), req.Contribution, req.OpID); err != nil {
			return BossContributeResult{}, err
		}
		return BossContributeResult{
			BossEventID:  req.BossEventID,
			BossKey:      bossKey,
			Contribution: total,
		}, nil
	})
}

func (s *PostgresStore) BossContribution(accountID, bossEventID int64) (int64, string, error) {
	var bossKey string
	var contribution int64
	err := s.pool.QueryRow(context.Background(), `
		SELECT e.boss_key, COALESCE(c.contribution, 0)
		FROM boss_events e
		LEFT JOIN boss_contributions c
			ON c.boss_event_id = e.id AND c.account_id = $2
		WHERE e.id = $1
	`, bossEventID, accountID).Scan(&bossKey, &contribution)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", ErrNotFound
	}
	if err != nil {
		return 0, "", err
	}
	return contribution, bossKey, nil
}

func (s *PostgresStore) BossSettle(req BossSettleRequest) (BossSettleResult, error) {
	if req.BossEventID <= 0 {
		return BossSettleResult{}, errors.New("bossEventId is required")
	}
	return runIdempotentAction(s, "boss_settle", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (BossSettleResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return BossSettleResult{}, err
		}
		bossKey, status, _, endsAt, err := s.lockBossEvent(ctx, tx, req.BossEventID)
		if err != nil {
			return BossSettleResult{}, err
		}
		if strings.TrimSpace(req.BossKey) == "" {
			req.BossKey = bossKey
		} else if req.BossKey != bossKey {
			return BossSettleResult{}, errors.New("bossKey does not match boss event")
		}
		now := time.Now().UTC()
		if status != "OPEN" && status != "SETTLING" {
			return BossSettleResult{}, errors.New("boss event is not ready for settlement")
		}
		if now.Before(endsAt) && status != "SETTLING" {
			return BossSettleResult{}, errors.New("boss event is still active")
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO boss_contributions (boss_event_id, account_id, character_id, contribution)
			VALUES ($1, $2, $3, 0)
			ON CONFLICT (boss_event_id, account_id) DO NOTHING
		`, req.BossEventID, req.AccountID, req.CharacterID); err != nil {
			return BossSettleResult{}, err
		}
		var contribution int64
		var rewardRaw []byte
		err = tx.QueryRow(ctx, `
			SELECT contribution, reward
			FROM boss_contributions
			WHERE boss_event_id = $1 AND account_id = $2
			FOR UPDATE
		`, req.BossEventID, req.AccountID).Scan(&contribution, &rewardRaw)
		if err != nil {
			return BossSettleResult{}, err
		}
		if bossRewardClaimed(rewardRaw) {
			return BossSettleResult{}, errors.New("boss rewards already claimed")
		}
		refID := strconv.FormatInt(req.BossEventID, 10)
		applied, err := s.applyTrayRewards(ctx, tx, req.AccountID, req.CharacterID, req.OpID, refID, "boss", "boss_reward", req.RewardPlan)
		if err != nil {
			return BossSettleResult{}, err
		}
		if _, err := s.publishRareRewardAnnouncementsTx(ctx, tx, req.RewardPlan, rareAnnouncementContext{
			AccountID: req.AccountID, CharacterID: req.CharacterID, Source: "Boss结算",
			RefType: "boss_event", RefID: refID, AnnouncementOn: true,
		}); err != nil {
			return BossSettleResult{}, err
		}
		rewardPayload := DungeonRewards{
			TokenReward:    strconv.FormatInt(req.RewardPlan.TokenReward, 10),
			Items:          applied.Items,
			EquipmentItems: applied.EquipmentItems,
		}
		rewardBytes, err := json.Marshal(rewardPayload)
		if err != nil {
			return BossSettleResult{}, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE boss_contributions
			SET reward = $3, character_id = $4, updated_at = NOW()
			WHERE boss_event_id = $1 AND account_id = $2
		`, req.BossEventID, req.AccountID, rewardBytes, req.CharacterID); err != nil {
			return BossSettleResult{}, err
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "BOSS_SETTLED", refID, contribution, req.OpID); err != nil {
			return BossSettleResult{}, err
		}
		if err := s.wearEquippedGear(ctx, tx, req.AccountID, req.CharacterID, req.EquipmentWearPoints, req.DefaultMaxDurability); err != nil {
			return BossSettleResult{}, err
		}
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		if err != nil {
			return BossSettleResult{}, err
		}
		return BossSettleResult{
			BossEventID:  req.BossEventID,
			BossKey:      req.BossKey,
			Contribution: contribution,
			Rewards:      rewardPayload,
			Snapshot:     snapshot,
		}, nil
	})
}

func (s *PostgresStore) lockBossEvent(ctx context.Context, tx pgx.Tx, bossEventID int64) (bossKey, status string, startsAt, endsAt time.Time, err error) {
	err = tx.QueryRow(ctx, `
		SELECT boss_key, status, starts_at, ends_at
		FROM boss_events
		WHERE id = $1
		FOR UPDATE
	`, bossEventID).Scan(&bossKey, &status, &startsAt, &endsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", time.Time{}, time.Time{}, ErrNotFound
	}
	return bossKey, status, startsAt, endsAt, err
}

func bossRewardClaimed(raw []byte) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed != "" && trimmed != "{}" && trimmed != "null"
}

func (s *PostgresStore) runActivitySettlement(scope string, req ActivitySettlementRequest) (ActivitySettlementResult, error) {
	req.ActivityID = strings.TrimSpace(req.ActivityID)
	if req.ActivityID == "" {
		return ActivitySettlementResult{}, errors.New("activityId is required")
	}
	return runIdempotentAction(s, scope, req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (ActivitySettlementResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return ActivitySettlementResult{}, err
		}
		if req.ActivityType == "gathering" {
			if err := s.ensureGatheringNodeReady(ctx, tx, req.CharacterID, req.ActivityID, req.RespawnSeconds); err != nil {
				return ActivitySettlementResult{}, err
			}
		}
		rewards, err := s.applyRewardsToBag(ctx, tx, req.AccountID, req.CharacterID, req.OpID, req.ActivityID, req.ActivityType, req.RewardPlan)
		if err != nil {
			return ActivitySettlementResult{}, err
		}
		source := "活动奖励"
		switch req.ActivityType {
		case "gathering":
			source = "采集"
		case "farming":
			source = "农场"
		}
		if _, err := s.publishRareRewardAnnouncementsTx(ctx, tx, req.RewardPlan, rareAnnouncementContext{
			AccountID: req.AccountID, CharacterID: req.CharacterID, Source: source,
			RefType: req.ActivityType, RefID: req.ActivityID, AnnouncementOn: true,
		}); err != nil {
			return ActivitySettlementResult{}, err
		}
		if req.ActivityType == "gathering" {
			if _, err := s.advanceGatheringBounties(ctx, tx, req.CharacterID, req.RewardPlan); err != nil {
				return ActivitySettlementResult{}, err
			}
			rewardsBytes, err := json.Marshal(rewards)
			if err != nil {
				return ActivitySettlementResult{}, err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO gathering_settlements (op_id, account_id, character_id, node_key, rewards)
				VALUES ($1, $2, $3, $4, $5)
			`, req.OpID, req.AccountID, req.CharacterID, req.ActivityID, rewardsBytes); err != nil {
				return ActivitySettlementResult{}, err
			}
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, strings.ToUpper(req.ActivityType)+"_SETTLED", req.ActivityID, int64(len(rewards.Items)+len(rewards.EquipmentItems)), req.OpID); err != nil {
			return ActivitySettlementResult{}, err
		}
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		if err != nil {
			return ActivitySettlementResult{}, err
		}
		return ActivitySettlementResult{
			ActivityID:   req.ActivityID,
			ActivityType: req.ActivityType,
			Rewards:      rewards,
			Snapshot:     snapshot,
		}, nil
	})
}

func (s *PostgresStore) ensureGatheringNodeReady(ctx context.Context, tx pgx.Tx, characterID int64, nodeID string, respawnSeconds int) error {
	if respawnSeconds <= 0 {
		return nil
	}
	nodeID = strings.TrimSpace(nodeID)
	var lastSettledAt time.Time
	err := tx.QueryRow(ctx, `
		SELECT created_at
		FROM gathering_settlements
		WHERE character_id = $1 AND node_key = $2
		ORDER BY created_at DESC
		LIMIT 1
	`, characterID, nodeID).Scan(&lastSettledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	readyAt := lastSettledAt.Add(time.Duration(respawnSeconds) * time.Second)
	now := time.Now().UTC()
	if !now.Before(readyAt) {
		return nil
	}
	remaining := int((readyAt.Sub(now) + time.Second - 1) / time.Second)
	return fmt.Errorf("gathering node %q is on cooldown for %ds", nodeID, remaining)
}

type appliedDungeonRewards struct {
	Items          []InventoryItem
	EquipmentItems []EquipmentItem
}

func (s *PostgresStore) applyDungeonRewards(ctx context.Context, tx pgx.Tx, req DungeonFinishRequest) (appliedDungeonRewards, error) {
	if req.Result != "victory" {
		return appliedDungeonRewards{
			Items:          []InventoryItem{},
			EquipmentItems: []EquipmentItem{},
		}, nil
	}
	return s.applyTrayRewards(ctx, tx, req.AccountID, req.CharacterID, req.OpID, req.DungeonRunID, "dungeon", "dungeon_reward", req.RewardPlan)
}

func (s *PostgresStore) applyTrayRewards(ctx context.Context, tx pgx.Tx, accountID, characterID int64, opID, refID, lootSource, tokenSource string, plan DungeonRewardPlan) (appliedDungeonRewards, error) {
	out := appliedDungeonRewards{
		Items:          []InventoryItem{},
		EquipmentItems: []EquipmentItem{},
	}
	for _, reward := range plan.Items {
		reward.RewardType = strings.ToLower(strings.TrimSpace(reward.RewardType))
		if reward.Quantity <= 0 {
			continue
		}
		category := strings.TrimSpace(reward.Category)
		if category == "" {
			category = reward.RewardType
		}
		if reward.RewardType == "equipment" {
			if err := s.ensureItemCatalog(ctx, tx, reward.ItemID, category, reward.Rarity, false); err != nil {
				return out, err
			}
			affixes, err := json.Marshal(reward.Affixes)
			if err != nil {
				return out, err
			}
			equipmentUID := strings.TrimSpace(reward.EquipmentUID)
			if equipmentUID == "" {
				equipmentUID = fmt.Sprintf("equipment-%s-%s", strings.TrimSpace(opID), reward.ItemID)
			}
			var row EquipmentItem
			err = tx.QueryRow(ctx, `
				INSERT INTO equipment_items (
					equipment_uid,
					account_id,
					character_id,
					item_id,
					location,
					rarity,
					affixes,
					durability,
					max_durability
				)
				VALUES ($1, $2, $3, $4, 'IN_LOOT_TRAY', $5, $6, 100, 100)
				RETURNING id, equipment_uid, item_id, rarity
			`, equipmentUID, accountID, characterID, reward.ItemID, reward.Rarity, affixes).Scan(
				&row.ID,
				&row.EquipmentUID,
				&row.ItemID,
				&row.Rarity,
			)
			if err != nil {
				return out, err
			}
			row.Status = "IN_LOOT_TRAY"
			row.EquipSlot = -1
			row.Slot = -1
			row.Affixes = reward.Affixes
			if _, err := tx.Exec(ctx, `
				INSERT INTO loot_tray_items (account_id, character_id, item_type, item_id, equipment_id, quantity, source)
				VALUES ($1, $2, 'EQUIPMENT', $3, $4, 1, $5)
			`, accountID, characterID, reward.ItemID, row.ID, lootSource); err != nil {
				return out, err
			}
			out.EquipmentItems = append(out.EquipmentItems, row)
			continue
		}
		if err := s.ensureItemCatalog(ctx, tx, reward.ItemID, category, reward.Rarity, true); err != nil {
			return out, err
		}
		var lootID int64
		err := tx.QueryRow(ctx, `
			INSERT INTO loot_tray_items (account_id, character_id, item_type, item_id, quantity, source)
			VALUES ($1, $2, 'ITEM', $3, $4, $5)
			RETURNING id
		`, accountID, characterID, reward.ItemID, reward.Quantity, lootSource).Scan(&lootID)
		if err != nil {
			return out, err
		}
		out.Items = append(out.Items, InventoryItem{
			ID:       lootID,
			ItemID:   reward.ItemID,
			Quantity: reward.Quantity,
			Slot:     -1,
		})
	}
	if plan.TokenReward > 0 {
		unlockAt := time.Now().UTC().Add(74 * time.Hour)
		if _, err := tx.Exec(ctx, `INSERT INTO account_tokens (account_id) VALUES ($1) ON CONFLICT (account_id) DO NOTHING`, accountID); err != nil {
			return out, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO locked_token_records (account_id, amount, remaining_amount, source, ref_id, unlock_at)
			VALUES ($1, $2, $2, $3, NULLIF($4, ''), $5)
		`, accountID, plan.TokenReward, tokenSource, refID, unlockAt); err != nil {
			return out, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE account_tokens
			SET token_balance = token_balance + $2,
			    locked_balance = locked_balance + $2,
			    updated_at = NOW()
			WHERE account_id = $1
		`, accountID, plan.TokenReward); err != nil {
			return out, err
		}
	}
	return out, nil
}

func (s *PostgresStore) applyRewardsToBag(ctx context.Context, tx pgx.Tx, accountID, characterID int64, opID, refID, source string, plan DungeonRewardPlan) (DungeonRewards, error) {
	out := DungeonRewards{
		TokenReward:    strconv.FormatInt(plan.TokenReward, 10),
		Items:          []InventoryItem{},
		EquipmentItems: []EquipmentItem{},
	}
	for _, reward := range plan.Items {
		reward.RewardType = strings.ToLower(strings.TrimSpace(reward.RewardType))
		if reward.Quantity <= 0 {
			continue
		}
		category := strings.TrimSpace(reward.Category)
		if category == "" {
			category = reward.RewardType
		}
		if reward.RewardType == "equipment" {
			if err := s.ensureItemCatalog(ctx, tx, reward.ItemID, category, reward.Rarity, false); err != nil {
				return DungeonRewards{}, err
			}
			targetSlot, err := s.resolveStorageSlot(ctx, tx, characterID, "BAG", -1)
			if err != nil {
				return DungeonRewards{}, err
			}
			affixes, err := json.Marshal(reward.Affixes)
			if err != nil {
				return DungeonRewards{}, err
			}
			equipmentUID := strings.TrimSpace(reward.EquipmentUID)
			if equipmentUID == "" {
				equipmentUID = fmt.Sprintf("%s-%s-%s", source, strings.TrimSpace(opID), reward.ItemID)
			}
			var row EquipmentItem
			err = tx.QueryRow(ctx, `
				INSERT INTO equipment_items (
					equipment_uid,
					account_id,
					character_id,
					item_id,
					location,
					slot,
					rarity,
					affixes,
					durability,
					max_durability
				)
				VALUES ($1, $2, $3, $4, 'IN_BAG', $5, $6, $7, 100, 100)
				RETURNING id, equipment_uid, item_id, rarity, slot
			`, equipmentUID, accountID, characterID, reward.ItemID, targetSlot, reward.Rarity, affixes).Scan(
				&row.ID,
				&row.EquipmentUID,
				&row.ItemID,
				&row.Rarity,
				&row.Slot,
			)
			if err != nil {
				return DungeonRewards{}, err
			}
			row.Status = "IN_BAG"
			row.EquipSlot = -1
			row.Affixes = reward.Affixes
			out.EquipmentItems = append(out.EquipmentItems, row)
			continue
		}
		if err := s.ensureItemCatalog(ctx, tx, reward.ItemID, category, reward.Rarity, true); err != nil {
			return DungeonRewards{}, err
		}
		row, err := s.addInventoryItemToStorage(ctx, tx, characterID, "BAG", reward.ItemID, reward.Quantity)
		if err != nil {
			return DungeonRewards{}, err
		}
		out.Items = append(out.Items, row)
	}
	if plan.TokenReward > 0 {
		if err := s.grantLockedTokenInTx(ctx, tx, accountID, plan.TokenReward, source+"_reward", refID); err != nil {
			return DungeonRewards{}, err
		}
	}
	return out, nil
}

func (s *PostgresStore) addInventoryItemToStorage(ctx context.Context, tx pgx.Tx, characterID int64, location, itemID string, quantity int64) (InventoryItem, error) {
	location = strings.TrimSpace(location)
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return InventoryItem{}, errors.New("itemId is required")
	}
	if quantity <= 0 {
		return InventoryItem{}, errors.New("quantity must be positive")
	}

	var stackable bool
	err := tx.QueryRow(ctx, `SELECT COALESCE(stackable, TRUE) FROM item_catalog WHERE item_id = $1`, itemID).Scan(&stackable)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return InventoryItem{}, err
	}
	if stackable {
		var row InventoryItem
		err = tx.QueryRow(ctx, `
			SELECT id, item_id, COALESCE(slot, -1)
			FROM inventory_items
			WHERE character_id = $1
				AND location = $2
				AND item_id = $3
				AND bind_type = 'BOUND'
			ORDER BY slot NULLS LAST, id
			LIMIT 1
			FOR UPDATE
		`, characterID, location, itemID).Scan(&row.ID, &row.ItemID, &row.Slot)
		if err == nil {
			if _, err := tx.Exec(ctx, `
				UPDATE inventory_items
				SET quantity = quantity + $2, updated_at = NOW()
				WHERE id = $1
			`, row.ID, quantity); err != nil {
				return InventoryItem{}, err
			}
			row.Quantity = quantity
			return row, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return InventoryItem{}, err
		}
	}

	targetSlot, err := s.resolveStorageSlot(ctx, tx, characterID, location, -1)
	if err != nil {
		return InventoryItem{}, err
	}
	var row InventoryItem
	err = tx.QueryRow(ctx, `
		INSERT INTO inventory_items (character_id, item_id, quantity, location, slot)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, item_id, quantity, slot
	`, characterID, itemID, quantity, location, targetSlot).Scan(
		&row.ID,
		&row.ItemID,
		&row.Quantity,
		&row.Slot,
	)
	if err != nil {
		return InventoryItem{}, err
	}
	return row, nil
}

func (s *PostgresStore) resolveLootID(ctx context.Context, tx pgx.Tx, req LootActionRequest) (int64, error) {
	if req.LootID > 0 {
		return req.LootID, nil
	}
	if req.SlotIndex < 0 {
		return 0, errors.New("lootId or slotIndex is required")
	}
	var lootID int64
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM loot_tray_items
		WHERE account_id = $1
			AND character_id = $2
			AND status = 'PENDING'
		ORDER BY created_at, id
		OFFSET $3
		LIMIT 1
	`, req.AccountID, req.CharacterID, req.SlotIndex).Scan(&lootID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return lootID, err
}

func (s *PostgresStore) claimLootRow(ctx context.Context, tx pgx.Tx, req LootActionRequest, lootID int64) error {
	var itemType string
	var itemID *string
	var equipmentID *int64
	var available int64
	err := tx.QueryRow(ctx, `
		SELECT item_type, item_id, equipment_id, quantity
		FROM loot_tray_items
		WHERE id = $1
			AND account_id = $2
			AND character_id = $3
			AND status = 'PENDING'
		FOR UPDATE
	`, lootID, req.AccountID, req.CharacterID).Scan(&itemType, &itemID, &equipmentID, &available)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	itemType = strings.ToUpper(strings.TrimSpace(itemType))
	if itemType == "EQUIPMENT" {
		if equipmentID == nil {
			return errors.New("loot equipment_id is required")
		}
		targetSlot, err := s.resolveStorageSlot(ctx, tx, req.CharacterID, "BAG", -1)
		if err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE equipment_items
			SET location = 'IN_BAG', slot = $4, updated_at = NOW()
			WHERE id = $1
				AND account_id = $2
				AND character_id = $3
				AND location = 'IN_LOOT_TRAY'
		`, *equipmentID, req.AccountID, req.CharacterID, targetSlot)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrForbidden
		}
		if _, err := tx.Exec(ctx, `
			UPDATE loot_tray_items
			SET status = 'CLAIMED', claimed_at = NOW()
			WHERE id = $1
		`, lootID); err != nil {
			return err
		}
		return s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "LOOT_CLAIMED", strconv.FormatInt(lootID, 10), 1, req.OpID)
	}
	if itemID == nil || strings.TrimSpace(*itemID) == "" {
		return errors.New("loot item_id is required")
	}
	quantity := req.Quantity
	if quantity <= 0 {
		quantity = available
	}
	if quantity <= 0 || quantity > available {
		return ErrInsufficientBalance
	}
	if _, err := s.addInventoryItemToStorage(ctx, tx, req.CharacterID, "BAG", *itemID, quantity); err != nil {
		return err
	}
	if quantity == available {
		_, err = tx.Exec(ctx, `
			UPDATE loot_tray_items
			SET status = 'CLAIMED', claimed_at = NOW()
			WHERE id = $1
		`, lootID)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE loot_tray_items
			SET quantity = quantity - $2
			WHERE id = $1
		`, lootID, quantity)
	}
	if err != nil {
		return err
	}
	return s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "LOOT_CLAIMED", strconv.FormatInt(lootID, 10), quantity, req.OpID)
}

func (s *PostgresStore) grantLockedTokenInTx(ctx context.Context, tx pgx.Tx, accountID, amount int64, source, ref string) error {
	return s.grantLockedTokenInTxAt(ctx, tx, accountID, amount, source, ref, time.Now().UTC().Add(74*time.Hour))
}

func (s *PostgresStore) grantLockedTokenInTxAt(ctx context.Context, tx pgx.Tx, accountID, amount int64, source, ref string, unlockAt time.Time) error {
	if amount <= 0 {
		return nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO account_tokens (account_id) VALUES ($1) ON CONFLICT (account_id) DO NOTHING`, accountID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO locked_token_records (account_id, amount, remaining_amount, source, ref_id, unlock_at)
		VALUES ($1, $2, $2, $3, NULLIF($4, ''), $5)
	`, accountID, amount, source, ref, unlockAt); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		UPDATE account_tokens
		SET token_balance = token_balance + $2,
		    locked_balance = locked_balance + $2,
		    updated_at = NOW()
		WHERE account_id = $1
	`, accountID, amount)
	return err
}

func (s *PostgresStore) ensureItemCatalog(ctx context.Context, tx pgx.Tx, itemID, category string, rarity int, stackable bool) error {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return errors.New("reward itemId is required")
	}
	if category == "" {
		category = "misc"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO item_catalog (item_id, name, category, rarity, stackable)
		VALUES ($1, $1, $2, $3, $4)
		ON CONFLICT (item_id) DO NOTHING
	`, itemID, category, rarity, stackable)
	return err
}

func (s *PostgresStore) economySnapshot(ctx context.Context, q postgresReader, accountID, characterID int64) (EconomySnapshot, error) {
	var snapshot EconomySnapshot
	err := q.QueryRow(ctx, `
		SELECT
			c.account_id,
			c.id,
			COALESCE(w.gold, 0),
			COALESCE(w.gems, 0),
			COALESCE(w.stamina, 0),
			c.level,
			c.exp,
			c.highest_cleared_chapter,
			c.highest_cleared_floor,
			c.bag_expand_count,
			a.has_trading_license
		FROM characters c
		JOIN accounts a ON a.id = c.account_id
		LEFT JOIN character_wallets w ON w.character_id = c.id
		WHERE c.account_id = $1 AND c.id = $2 AND c.is_deleted = FALSE
	`, accountID, characterID).Scan(
		&snapshot.AccountID,
		&snapshot.CharacterID,
		&snapshot.Gold,
		&snapshot.Gems,
		&snapshot.Stamina,
		&snapshot.Level,
		&snapshot.Exp,
		&snapshot.HighestClearedChapterID,
		&snapshot.HighestClearedFloorID,
		&snapshot.BagExpandCount,
		&snapshot.HasLicense,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return EconomySnapshot{}, ErrNotFound
	}
	if err != nil {
		return EconomySnapshot{}, err
	}
	// Default capacity; service overlays config-driven EffectiveBagSlots when available.
	snapshot.BagSlots = GrowthPaymentRules{}.withDefaults().EffectiveBagSlots(snapshot.BagExpandCount)
	snapshot.Inventory = []InventoryItem{}
	snapshot.Warehouse = []InventoryItem{}
	snapshot.LootTray = []InventoryItem{}
	snapshot.Equipment = []EquipmentItem{}

	if err := q.QueryRow(ctx, `
		INSERT INTO account_tokens (account_id)
		VALUES ($1)
		ON CONFLICT (account_id) DO UPDATE SET account_id = EXCLUDED.account_id
		RETURNING account_id
	`, accountID).Scan(&snapshot.AccountToken.AccountID); err != nil {
		return EconomySnapshot{}, err
	}
	err = q.QueryRow(ctx, `
		SELECT
			account_id,
			token_balance::bigint,
			withdrawable_balance::bigint,
			locked_balance::bigint,
			external_balance::bigint,
			unlock_credit::bigint
		FROM account_tokens
		WHERE account_id = $1
	`, accountID).Scan(
		&snapshot.AccountToken.AccountID,
		&snapshot.AccountToken.TokenBalance,
		&snapshot.AccountToken.WithdrawableBalance,
		&snapshot.AccountToken.LockedBalance,
		&snapshot.AccountToken.ExternalBalance,
		&snapshot.AccountToken.UnlockCredit,
	)
	if err != nil {
		return EconomySnapshot{}, err
	}

	rows, err := q.Query(ctx, `
		SELECT id, item_id, quantity, location, COALESCE(slot, -1)
		FROM inventory_items
		WHERE character_id = $1 AND location IN ('BAG', 'WAREHOUSE')
		ORDER BY location, slot, id
	`, characterID)
	if err != nil {
		return EconomySnapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var row InventoryItem
		var location string
		if err := rows.Scan(&row.ID, &row.ItemID, &row.Quantity, &location, &row.Slot); err != nil {
			return EconomySnapshot{}, err
		}
		switch location {
		case "BAG":
			snapshot.Inventory = append(snapshot.Inventory, row)
		case "WAREHOUSE":
			snapshot.Warehouse = append(snapshot.Warehouse, row)
		}
	}
	if err := rows.Err(); err != nil {
		return EconomySnapshot{}, err
	}

	rows, err = q.Query(ctx, `
		SELECT id, COALESCE(item_id, ''), quantity
		FROM loot_tray_items
		WHERE character_id = $1 AND status = 'PENDING'
		ORDER BY created_at, id
	`, characterID)
	if err != nil {
		return EconomySnapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var row InventoryItem
		if err := rows.Scan(&row.ID, &row.ItemID, &row.Quantity); err != nil {
			return EconomySnapshot{}, err
		}
		row.Slot = -1
		snapshot.LootTray = append(snapshot.LootTray, row)
	}
	if err := rows.Err(); err != nil {
		return EconomySnapshot{}, err
	}

	rows, err = q.Query(ctx, `
		SELECT
			e.id,
			e.equipment_uid,
			e.item_id,
			e.rarity,
			e.enhance_level,
			COALESCE(e.durability, 0),
			COALESCE(e.max_durability, 0),
			e.location,
			COALESCE(e.equip_slot, -1),
			COALESCE(e.slot, -1),
			e.affixes,
			COALESCE(n.mint_address, '')
		FROM equipment_items e
		LEFT JOIN nft_assets n
			ON n.source_asset_type = 'EQUIPMENT'
			AND n.source_asset_id = e.id
			AND n.status IN ('MINT_REQUESTED', 'MINTED')
		WHERE e.character_id = $1
			AND e.location NOT IN ('CONSUMED', 'DELETED', 'BURNED', 'LISTED', 'MARKET_CLAIM_PENDING', 'NPC_RECYCLED')
		ORDER BY e.location, e.slot, e.equip_slot, e.id
	`, characterID)
	if err != nil {
		return EconomySnapshot{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var row EquipmentItem
		var nftContract string
		var affixes []byte
		if err := rows.Scan(
			&row.ID,
			&row.EquipmentUID,
			&row.ItemID,
			&row.Rarity,
			&row.EnhanceLevel,
			&row.Durability,
			&row.MaxDurability,
			&row.Status,
			&row.EquipSlot,
			&row.Slot,
			&affixes,
			&nftContract,
		); err != nil {
			return EconomySnapshot{}, err
		}
		if len(affixes) > 0 {
			if err := json.Unmarshal(affixes, &row.Affixes); err != nil {
				return EconomySnapshot{}, err
			}
		}
		if nftContract != "" {
			row.NFTContract = &nftContract
		}
		snapshot.Equipment = append(snapshot.Equipment, row)
	}
	if err := rows.Err(); err != nil {
		return EconomySnapshot{}, err
	}

	return snapshot, nil
}

func (s *PostgresStore) runEconomyAction(scope string, req EconomyActionRequest, mutate func(context.Context, pgx.Tx) error) (EconomySnapshot, error) {
	return runIdempotentAction(s, scope, req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (EconomySnapshot, error) {
		if err := mutate(ctx, tx); err != nil {
			return EconomySnapshot{}, err
		}
		return s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
	})
}

func runIdempotentAction[T any](s *PostgresStore, scope, opID string, accountID, characterID int64, request any, run func(context.Context, pgx.Tx) (T, error)) (T, error) {
	var zero T
	opID = strings.TrimSpace(opID)
	if opID == "" {
		return zero, errors.New("opId is required")
	}
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return zero, fmt.Errorf("encode idempotent request: %w", err)
	}
	requestHash := fmt.Sprintf("%x", sha256.Sum256(requestJSON))
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return zero, err
	}
	defer rollback(ctx, tx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, opID); err != nil {
		return zero, err
	}

	var existingScope string
	var existingAccountID, existingCharacterID int64
	var existingRequestHash string
	var response []byte
	err = tx.QueryRow(ctx, `
		SELECT scope, COALESCE(account_id, 0), COALESCE(character_id, 0), COALESCE(request_hash, ''), response
		FROM idempotency_keys
		WHERE op_id = $1
	`, opID).Scan(&existingScope, &existingAccountID, &existingCharacterID, &existingRequestHash, &response)
	if err == nil {
		if existingScope != scope {
			return zero, errors.New("opId already used for another operation")
		}
		if existingAccountID != accountID || existingCharacterID != characterID {
			return zero, errors.New("opId already belongs to another account or character")
		}
		if existingRequestHash != "" && existingRequestHash != requestHash {
			return zero, errors.New("opId already used with different request parameters")
		}
		var result T
		if err := json.Unmarshal(response, &result); err != nil {
			return zero, err
		}
		if err := tx.Commit(ctx); err != nil {
			return zero, err
		}
		return result, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return zero, err
	}

	result, err := run(ctx, tx)
	if err != nil {
		return zero, err
	}
	response, err = json.Marshal(result)
	if err != nil {
		return zero, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO idempotency_keys (op_id, scope, account_id, character_id, request_hash, response)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, opID, scope, accountID, characterID, requestHash, response)
	if err != nil {
		return zero, err
	}
	if err := tx.Commit(ctx); err != nil {
		return zero, err
	}
	return result, nil
}

func dungeonKey(chapterID, floorID int) string {
	return fmt.Sprintf("chapter:%d:floor:%d", chapterID, floorID)
}

func (s *PostgresStore) lockCharacter(ctx context.Context, tx pgx.Tx, accountID, characterID int64) error {
	var id int64
	err := tx.QueryRow(ctx, `
		SELECT id
		FROM characters
		WHERE account_id = $1 AND id = $2 AND is_deleted = FALSE
		FOR UPDATE
	`, accountID, characterID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func (s *PostgresStore) moveInventory(ctx context.Context, tx pgx.Tx, req EconomyActionRequest, fromLocation, toLocation, ledgerKind string) error {
	if req.SlotIndex < 0 {
		return errors.New("slotIndex is required")
	}
	if req.Quantity <= 0 {
		return errors.New("quantity must be positive")
	}
	var row InventoryItem
	err := tx.QueryRow(ctx, `
		SELECT id, item_id, quantity
		FROM inventory_items
		WHERE character_id = $1 AND location = $2 AND slot = $3
		FOR UPDATE
	`, req.CharacterID, fromLocation, req.SlotIndex).Scan(&row.ID, &row.ItemID, &row.Quantity)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if req.Quantity > row.Quantity {
		return ErrInsufficientBalance
	}
	targetSlot, err := s.resolveStorageSlot(ctx, tx, req.CharacterID, toLocation, -1)
	if err != nil {
		return err
	}
	if req.Quantity == row.Quantity {
		_, err = tx.Exec(ctx, `
			UPDATE inventory_items
			SET location = $2, slot = $3, updated_at = NOW()
			WHERE id = $1
		`, row.ID, toLocation, targetSlot)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE inventory_items
			SET quantity = quantity - $2, updated_at = NOW()
			WHERE id = $1
		`, row.ID, req.Quantity)
		if err == nil {
			_, err = tx.Exec(ctx, `
				INSERT INTO inventory_items (character_id, item_id, quantity, location, slot)
				VALUES ($1, $2, $3, $4, $5)
			`, req.CharacterID, row.ItemID, req.Quantity, toLocation, targetSlot)
		}
	}
	if err != nil {
		return err
	}
	return s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, ledgerKind, row.ItemID, req.Quantity, req.OpID)
}

func (s *PostgresStore) resolveStorageSlot(ctx context.Context, tx pgx.Tx, characterID int64, location string, requested int) (int, error) {
	if requested >= 0 {
		free, err := s.storageSlotFree(ctx, tx, characterID, location, requested)
		if err != nil {
			return 0, err
		}
		if !free {
			return 0, errors.New("target slot is occupied")
		}
		return requested, nil
	}
	for slot := 0; slot < 1000; slot++ {
		free, err := s.storageSlotFree(ctx, tx, characterID, location, slot)
		if err != nil {
			return 0, err
		}
		if free {
			return slot, nil
		}
	}
	return 0, errors.New("no available slot")
}

func (s *PostgresStore) storageSlotFree(ctx context.Context, tx pgx.Tx, characterID int64, location string, slot int) (bool, error) {
	equipmentLocation, err := equipmentStorageLocation(location)
	if err != nil {
		return false, err
	}
	var count int
	err = tx.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM inventory_items WHERE character_id = $1 AND location = $2 AND slot = $3)
			+
			(SELECT COUNT(*) FROM equipment_items WHERE character_id = $1 AND location = $4 AND slot = $3)
	`, characterID, location, slot, equipmentLocation).Scan(&count)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func equipmentStorageLocation(location string) (string, error) {
	switch location {
	case "BAG":
		return "IN_BAG", nil
	case "WAREHOUSE":
		return "IN_WAREHOUSE", nil
	default:
		return "", errors.New("unsupported storage location")
	}
}

func (s *PostgresStore) insertEconomyLedger(ctx context.Context, tx pgx.Tx, accountID, characterID int64, kind, ref string, amount int64, opID string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO economy_ledger (account_id, character_id, kind, amount, ref_id, reason)
		VALUES ($1, NULLIF($2, 0), $3, $4, NULLIF($5, ''), NULLIF($6, ''))
	`, accountID, characterID, kind, amount, ref, opID)
	return err
}

func (s *PostgresStore) SaveTicket(ticket GameTicket) {
	_, err := s.pool.Exec(context.Background(), `
		INSERT INTO game_tickets (ticket, account_id, character_id, session_id, server_id, expires_at)
		VALUES ($1, $2, NULLIF($3, 0), $4, NULLIF($5, ''), $6)
	`, ticket.Ticket, ticket.AccountID, ticket.CharacterID, ticket.SessionID, ticket.ServerID, ticket.ExpiresAt)
	must(err, "save ticket")
}

func (s *PostgresStore) ConsumeTicket(ticket, serverID string, now time.Time) (GameTicket, error) {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return GameTicket{}, ErrForbidden
	}
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return GameTicket{}, err
	}
	defer rollback(ctx, tx)
	var row GameTicket
	var status string
	var storedServerID *string
	var characterID sql.NullInt64
	err = tx.QueryRow(ctx, `
		SELECT ticket, account_id, character_id, session_id, server_id, status, expires_at, consumed_at IS NOT NULL
		FROM game_tickets
		WHERE ticket = $1
		FOR UPDATE
	`, ticket).Scan(&row.Ticket, &row.AccountID, &characterID, &row.SessionID, &storedServerID, &status, &row.ExpiresAt, &row.Consumed)
	if errors.Is(err, pgx.ErrNoRows) {
		return GameTicket{}, ErrNotFound
	}
	if err != nil {
		return GameTicket{}, err
	}
	if characterID.Valid {
		row.CharacterID = characterID.Int64
	}
	if storedServerID != nil {
		row.ServerID = *storedServerID
	}
	if status != "ACTIVE" || row.Consumed || row.ExpiresAt.Before(now) {
		return GameTicket{}, ErrForbidden
	}
	if row.ServerID == "" || row.ServerID != serverID {
		return GameTicket{}, ErrForbidden
	}
	var sessionStatus string
	err = tx.QueryRow(ctx, `
		SELECT status
		FROM account_sessions
		WHERE session_id = $1 AND account_id = $2
	`, row.SessionID, row.AccountID).Scan(&sessionStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return GameTicket{}, ErrForbidden
	}
	if err != nil {
		return GameTicket{}, err
	}
	if sessionStatus != "ACTIVE" {
		return GameTicket{}, ErrForbidden
	}
	_, err = tx.Exec(ctx, `
		UPDATE game_tickets SET status = 'CONSUMED', consumed_at = $2
		WHERE ticket = $1
	`, ticket, now)
	if err != nil {
		return GameTicket{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return GameTicket{}, err
	}
	row.Consumed = true
	return row, nil
}

func (s *PostgresStore) Token(accountID int64) AccountToken {
	ctx := context.Background()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO account_tokens (account_id)
		VALUES ($1)
		ON CONFLICT (account_id) DO NOTHING
	`, accountID)
	must(err, "ensure token")
	var token AccountToken
	err = s.pool.QueryRow(ctx, `
		SELECT
			account_id,
			token_balance::bigint,
			withdrawable_balance::bigint,
			locked_balance::bigint,
			external_balance::bigint,
			unlock_credit::bigint
		FROM account_tokens
		WHERE account_id = $1
	`, accountID).Scan(
		&token.AccountID,
		&token.TokenBalance,
		&token.WithdrawableBalance,
		&token.LockedBalance,
		&token.ExternalBalance,
		&token.UnlockCredit,
	)
	must(err, "get token")
	return token
}

func (s *PostgresStore) GrantLocked(accountID, amount int64, source, ref string, unlockAt time.Time) (LockedGame, error) {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return LockedGame{}, err
	}
	defer rollback(ctx, tx)
	if _, err := tx.Exec(ctx, `INSERT INTO account_tokens (account_id) VALUES ($1) ON CONFLICT (account_id) DO NOTHING`, accountID); err != nil {
		return LockedGame{}, err
	}
	var row LockedGame
	err = tx.QueryRow(ctx, `
		INSERT INTO locked_token_records (account_id, amount, remaining_amount, source, ref_id, unlock_at)
		VALUES ($1, $2, $2, $3, NULLIF($4, ''), $5)
		RETURNING id, account_id, amount::bigint, source, status, COALESCE(ref_id, ''), created_at, unlock_at
	`, accountID, amount, source, ref, unlockAt).Scan(
		&row.ID,
		&row.AccountID,
		&row.Amount,
		&row.Source,
		&row.Status,
		&row.Ref,
		&row.CreatedAt,
		&row.UnlockAt,
	)
	if err != nil {
		return LockedGame{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE account_tokens
		SET token_balance = token_balance + $2,
		    locked_balance = locked_balance + $2,
		    updated_at = NOW()
		WHERE account_id = $1
	`, accountID, amount); err != nil {
		return LockedGame{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO economy_ledger (account_id, kind, currency, amount, ref_id, reason)
		VALUES ($1, 'LOCKED_TOKEN_GRANTED', 'AEB', $2, NULLIF($3, ''), $4)
	`, accountID, amount, ref, source); err != nil {
		return LockedGame{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return LockedGame{}, err
	}
	return row, nil
}

func (s *PostgresStore) SettleUnlocks(now time.Time, limit int) []LockedGame {
	ctx := context.Background()
	if limit <= 0 {
		limit = 100
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	must(err, "begin settle unlocks")
	defer rollback(ctx, tx)
	rows, err := tx.Query(ctx, `
		SELECT id, account_id, remaining_amount::bigint, source, status, COALESCE(ref_id, ''), created_at, unlock_at
		FROM locked_token_records
		WHERE status = 'LOCKED' AND unlock_at <= $1
		ORDER BY unlock_at
		LIMIT $2
		FOR UPDATE SKIP LOCKED
	`, now, limit)
	must(err, "query unlocks")
	defer rows.Close()
	var out []LockedGame
	for rows.Next() {
		var row LockedGame
		must(rows.Scan(&row.ID, &row.AccountID, &row.Amount, &row.Source, &row.Status, &row.Ref, &row.CreatedAt, &row.UnlockAt), "scan unlock")
		out = append(out, row)
	}
	must(rows.Err(), "iterate unlocks")
	for _, row := range out {
		_, err := tx.Exec(ctx, `
			UPDATE locked_token_records
			SET status = 'UNLOCKED', settled_at = $2
			WHERE id = $1
		`, row.ID, now)
		must(err, "update locked record")
		_, err = tx.Exec(ctx, `
			UPDATE account_tokens
			SET locked_balance = locked_balance - $2,
			    withdrawable_balance = withdrawable_balance + $2,
			    updated_at = NOW()
			WHERE account_id = $1
		`, row.AccountID, row.Amount)
		must(err, "update token unlock")
		_, err = tx.Exec(ctx, `
			INSERT INTO economy_ledger (account_id, kind, currency, amount, ref_id, reason)
			VALUES ($1, 'LOCKED_TOKEN_UNLOCKED', 'AEB', $2, NULLIF($3, ''), $4)
		`, row.AccountID, row.Amount, row.Ref, row.Source)
		must(err, "insert unlock ledger")
	}
	must(tx.Commit(ctx), "commit settle unlocks")
	return out
}

func (s *PostgresStore) CreateWithdrawal(accountID int64, amount int64, wallet string, manual bool) (Withdrawal, error) {
	if amount <= 0 {
		return Withdrawal{}, errors.New("amount must be positive")
	}
	ctx := context.Background()
	if wallet == "" {
		wallet = fmt.Sprintf("pending_wallet_for_account_%d", accountID)
	}
	status := "QUEUED"
	reason := ""
	if manual {
		status = "MANUAL_REVIEW"
		reason = "manual requested"
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Withdrawal{}, err
	}
	defer rollback(ctx, tx)
	tag, err := tx.Exec(ctx, `
		UPDATE account_tokens
		SET withdrawable_balance = withdrawable_balance - $2,
		    updated_at = NOW()
		WHERE account_id = $1 AND withdrawable_balance >= $2
	`, accountID, amount)
	if err != nil {
		return Withdrawal{}, err
	}
	if tag.RowsAffected() == 0 {
		return Withdrawal{}, ErrInsufficientBalance
	}
	var row Withdrawal
	err = tx.QueryRow(ctx, `
		INSERT INTO withdrawals (account_id, wallet, amount, status, reason)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		RETURNING id, account_id, wallet, amount::bigint, status, COALESCE(reason, ''), COALESCE(tx_signature, ''), created_at
	`, accountID, wallet, amount, status, reason).Scan(&row.ID, &row.AccountID, &row.Wallet, &row.Amount, &row.Status, &row.Reason, &row.TxHash, &row.CreatedAt)
	if err != nil {
		return Withdrawal{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO economy_ledger (account_id, kind, currency, amount, reason)
		VALUES ($1, 'WITHDRAWAL_REQUESTED', 'AEB', $2, $3)
	`, accountID, amount, status)
	if err != nil {
		return Withdrawal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Withdrawal{}, err
	}
	return row, nil
}

func (s *PostgresStore) ListWithdrawals(status string) []Withdrawal {
	status = strings.ToUpper(strings.TrimSpace(status))
	var rows pgx.Rows
	var err error
	if status == "" {
		rows, err = s.pool.Query(context.Background(), `
			SELECT id, account_id, wallet, amount::bigint, status, COALESCE(reason, ''), COALESCE(tx_signature, ''), created_at, COALESCE(processed_at, '0001-01-01'::timestamptz)
			FROM withdrawals
			ORDER BY created_at DESC
			LIMIT 200
		`)
	} else {
		rows, err = s.pool.Query(context.Background(), `
			SELECT id, account_id, wallet, amount::bigint, status, COALESCE(reason, ''), COALESCE(tx_signature, ''), created_at, COALESCE(processed_at, '0001-01-01'::timestamptz)
			FROM withdrawals
			WHERE status = $1
			ORDER BY created_at DESC
			LIMIT 200
		`, status)
	}
	must(err, "list withdrawals")
	defer rows.Close()
	var out []Withdrawal
	for rows.Next() {
		var row Withdrawal
		must(rows.Scan(&row.ID, &row.AccountID, &row.Wallet, &row.Amount, &row.Status, &row.Reason, &row.TxHash, &row.CreatedAt, &row.ProcessedAt), "scan withdrawal")
		out = append(out, row)
	}
	must(rows.Err(), "iterate withdrawals")
	return out
}

func (s *PostgresStore) ReviewWithdrawal(id int64, approve bool, adminID, reason string) (Withdrawal, error) {
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Withdrawal{}, err
	}
	defer rollback(ctx, tx)
	var row Withdrawal
	err = tx.QueryRow(ctx, `
		SELECT id, account_id, wallet, amount::bigint, status, COALESCE(reason, ''), COALESCE(tx_signature, ''), created_at
		FROM withdrawals
		WHERE id = $1
		FOR UPDATE
	`, id).Scan(&row.ID, &row.AccountID, &row.Wallet, &row.Amount, &row.Status, &row.Reason, &row.TxHash, &row.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Withdrawal{}, ErrNotFound
	}
	if err != nil {
		return Withdrawal{}, err
	}
	if row.Status != "MANUAL_REVIEW" {
		return Withdrawal{}, ErrForbidden
	}
	if approve {
		row.Status = "QUEUED"
		row.Reason = "approved: " + reason
		_, err = tx.Exec(ctx, `UPDATE withdrawals SET status = $2, reason = $3, reviewed_at = NOW() WHERE id = $1`, row.ID, row.Status, row.Reason)
	} else {
		row.Status = "REJECTED"
		row.Reason = reason
		_, err = tx.Exec(ctx, `UPDATE withdrawals SET status = $2, reason = $3, reviewed_at = NOW() WHERE id = $1`, row.ID, row.Status, row.Reason)
		if err == nil {
			_, err = tx.Exec(ctx, `
				UPDATE account_tokens
				SET withdrawable_balance = withdrawable_balance + $2, updated_at = NOW()
				WHERE account_id = $1
			`, row.AccountID, row.Amount)
		}
	}
	if err != nil {
		return Withdrawal{}, err
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO admin_audit_logs (admin_id, action, target_type, target_id, reason)
		VALUES ($1, 'withdrawal_review', 'withdrawal', $2, $3)
	`, adminID, fmt.Sprint(row.ID), reason)
	if err != nil {
		return Withdrawal{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return Withdrawal{}, err
	}
	return row, nil
}

func (s *PostgresStore) Ledger(accountID int64) []LedgerEntry {
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, COALESCE(account_id, 0), kind, COALESCE(amount::bigint, 0), COALESCE(ref_id, ''), COALESCE(reason, ''), created_at
		FROM economy_ledger
		WHERE ($1::bigint = 0 OR account_id = $1)
		ORDER BY created_at DESC
		LIMIT 200
	`, accountID)
	must(err, "list ledger")
	defer rows.Close()
	var out []LedgerEntry
	for rows.Next() {
		var row LedgerEntry
		must(rows.Scan(&row.ID, &row.AccountID, &row.Kind, &row.Amount, &row.Ref, &row.Detail, &row.CreatedAt), "scan ledger")
		out = append(out, row)
	}
	must(rows.Err(), "iterate ledger")
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func rollback(ctx context.Context, tx pgx.Tx) {
	_ = tx.Rollback(ctx)
}

func must(err error, op string) {
	if err != nil {
		panic(fmt.Errorf("%s: %w", op, err))
	}
}

func pending(status string) string {
	if strings.TrimSpace(status) == "" {
		return "PENDING"
	}
	return status
}
