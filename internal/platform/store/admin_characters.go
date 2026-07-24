package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *PostgresStore) ListAdminCharacters(filter AdminCharacterListFilter) ([]AdminCharacterSummary, error) {
	limit := clampAdminLimit(filter.Limit)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if filter.MinLevel < 0 || filter.MaxLevel < 0 {
		return nil, errors.New("level filters must be non-negative")
	}
	if filter.MaxLevel > 0 && filter.MaxLevel < filter.MinLevel {
		return nil, errors.New("maxLevel must be >= minLevel")
	}

	clauses := []string{"c.is_deleted = FALSE"}
	args := []any{}
	add := func(value any) string {
		args = append(args, value)
		return fmt.Sprintf("$%d", len(args))
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		placeholder := add("%" + keyword + "%")
		clauses = append(clauses, fmt.Sprintf(`(
			c.name ILIKE %[1]s
			OR c.id::text ILIKE %[1]s
			OR c.account_id::text ILIKE %[1]s
			OR COALESCE(a.username, '') ILIKE %[1]s
			OR COALESCE(a.solana_wallet_address, '') ILIKE %[1]s
		)`, placeholder))
	}
	if filter.AccountID > 0 {
		clauses = append(clauses, "c.account_id = "+add(filter.AccountID))
	}
	if wallet := strings.TrimSpace(filter.Wallet); wallet != "" {
		clauses = append(clauses, "a.solana_wallet_address = "+add(wallet))
	}
	if filter.MinLevel > 0 {
		clauses = append(clauses, "c.level >= "+add(filter.MinLevel))
	}
	if filter.MaxLevel > 0 {
		clauses = append(clauses, "c.level <= "+add(filter.MaxLevel))
	}
	if filter.HasTradingLicense != nil {
		clauses = append(clauses, "a.has_trading_license = "+add(*filter.HasTradingLicense))
	}
	if status := strings.ToUpper(strings.TrimSpace(filter.Status)); status != "" {
		clauses = append(clauses, "a.status = "+add(status))
	}
	if filter.OnlineOnly {
		clauses = append(clauses, "os.account_id IS NOT NULL")
	}
	if serverID := strings.TrimSpace(filter.ServerID); serverID != "" {
		clauses = append(clauses, "os.server_id = "+add(serverID))
	}
	where := strings.Join(clauses, " AND ")
	limitPlaceholder := add(limit)
	offsetPlaceholder := add(offset)

	rows, err := s.pool.Query(context.Background(), `
		SELECT
			c.id,
			c.account_id,
			c.name,
			c.level,
			c.exp,
			COALESCE(a.solana_wallet_address, ''),
			a.status,
			a.risk_level,
			a.has_trading_license,
			COALESCE(a.last_login_at, a.created_at),
			(os.account_id IS NOT NULL),
			COALESCE(os.server_id, ''),
			c.created_at
		FROM characters c
		JOIN accounts a ON a.id = c.account_id
		LEFT JOIN online_sessions os
			ON os.account_id = a.id
			AND os.character_id = c.id
		WHERE `+where+`
		ORDER BY c.created_at DESC, c.id DESC
		LIMIT `+limitPlaceholder+` OFFSET `+offsetPlaceholder, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []AdminCharacterSummary{}
	for rows.Next() {
		var row AdminCharacterSummary
		if err := rows.Scan(
			&row.CharacterID,
			&row.AccountID,
			&row.Name,
			&row.Level,
			&row.Exp,
			&row.WalletAddress,
			&row.AccountStatus,
			&row.RiskLevel,
			&row.HasTradingLicense,
			&row.LastLoginAt,
			&row.Online,
			&row.ServerID,
			&row.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func (s *PostgresStore) AdminCharacterDetail(characterID int64) (AdminCharacterDetail, error) {
	if characterID <= 0 {
		return AdminCharacterDetail{}, errors.New("characterId is required")
	}
	ctx := context.Background()
	var out AdminCharacterDetail
	err := s.pool.QueryRow(ctx, `
		SELECT
			c.id,
			c.account_id,
			c.name,
			c.level,
			c.exp,
			COALESCE(w.stamina, 0),
			c.bag_expand_count,
			c.highest_cleared_chapter,
			c.highest_cleared_floor,
			c.dungeon_clear_count,
			c.created_at
		FROM characters c
		LEFT JOIN character_wallets w ON w.character_id = c.id
		WHERE c.id = $1 AND c.is_deleted = FALSE
	`, characterID).Scan(
		&out.Character.CharacterID,
		&out.Character.AccountID,
		&out.Character.Name,
		&out.Character.Level,
		&out.Character.Exp,
		&out.Character.Stamina,
		&out.Character.BagExpandCount,
		&out.Character.HighestClearedChapter,
		&out.Character.HighestClearedFloor,
		&out.Character.DungeonClearCount,
		&out.Character.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminCharacterDetail{}, ErrNotFound
	}
	if err != nil {
		return AdminCharacterDetail{}, err
	}
	out.Character.BagSlots = GrowthPaymentRules{}.withDefaults().EffectiveBagSlots(out.Character.BagExpandCount)

	account, err := s.AdminGetAccount(out.Character.AccountID, "")
	if err != nil {
		return AdminCharacterDetail{}, err
	}
	out.Account = account

	snapshot, err := s.adminEconomySnapshot(ctx, s.pool, out.Character.AccountID, out.Character.CharacterID)
	if err != nil {
		return AdminCharacterDetail{}, err
	}
	out.Economy = snapshot

	var online OnlineSession
	err = s.pool.QueryRow(ctx, `
		SELECT account_id, character_id, session_id, server_id, connection_id, entered_at, last_seen_at
		FROM online_sessions
		WHERE character_id = $1
	`, out.Character.CharacterID).Scan(
		&online.AccountID,
		&online.CharacterID,
		&online.SessionID,
		&online.ServerID,
		&online.ConnectionID,
		&online.EnteredAt,
		&online.LastSeenAt,
	)
	if err == nil {
		lastSeen := online.LastSeenAt
		out.Online = AdminOnlineStatus{
			Online:       true,
			ServerID:     online.ServerID,
			ConnectionID: online.ConnectionID,
			SessionID:    online.SessionID,
			LastSeenAt:   &lastSeen,
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return AdminCharacterDetail{}, err
	}

	return out, nil
}

func (s *PostgresStore) adminEconomySnapshot(ctx context.Context, q postgresReader, accountID, characterID int64) (EconomySnapshot, error) {
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
			a.has_trading_license,
			COALESCE(t.token_balance::bigint, 0),
			COALESCE(t.withdrawable_balance::bigint, 0),
			COALESCE(t.locked_balance::bigint, 0),
			COALESCE(t.external_balance::bigint, 0),
			COALESCE(t.unlock_credit::bigint, 0)
		FROM characters c
		JOIN accounts a ON a.id = c.account_id
		LEFT JOIN character_wallets w ON w.character_id = c.id
		LEFT JOIN account_tokens t ON t.account_id = a.id
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
		&snapshot.AccountToken.TokenBalance,
		&snapshot.AccountToken.WithdrawableBalance,
		&snapshot.AccountToken.LockedBalance,
		&snapshot.AccountToken.ExternalBalance,
		&snapshot.AccountToken.UnlockCredit,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return EconomySnapshot{}, ErrNotFound
	}
	if err != nil {
		return EconomySnapshot{}, err
	}
	snapshot.BagSlots = GrowthPaymentRules{}.withDefaults().EffectiveBagSlots(snapshot.BagExpandCount)
	snapshot.AccountToken.AccountID = accountID
	snapshot.Inventory = []InventoryItem{}
	snapshot.Warehouse = []InventoryItem{}
	snapshot.LootTray = []InventoryItem{}
	snapshot.Equipment = []EquipmentItem{}

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
			COALESCE(n.mint_address, ''),
			COALESCE(n.status, '')
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
		var nftStatus string
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
			&nftStatus,
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
		if nftStatus != "" {
			row.NFTStatus = &nftStatus
		}
		snapshot.Equipment = append(snapshot.Equipment, row)
	}
	if err := rows.Err(); err != nil {
		return EconomySnapshot{}, err
	}

	return snapshot, nil
}

func (s *PostgresStore) ListCharacterLedger(characterID int64, kind string, limit, offset int) ([]LedgerEntry, error) {
	if characterID <= 0 {
		return nil, errors.New("characterId is required")
	}
	limit = clampAdminLimit(limit)
	if offset < 0 {
		offset = 0
	}
	kind = strings.TrimSpace(kind)
	var exists int
	err := s.pool.QueryRow(context.Background(), `
		SELECT 1 FROM characters WHERE id = $1 AND is_deleted = FALSE
	`, characterID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, COALESCE(account_id, 0), kind, COALESCE(amount::bigint, 0), COALESCE(ref_id, ''), COALESCE(reason, ''), created_at
		FROM economy_ledger
		WHERE character_id = $1
			AND ($2 = '' OR kind = $2)
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4
	`, characterID, kind, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LedgerEntry{}
	for rows.Next() {
		var row LedgerEntry
		if err := rows.Scan(&row.ID, &row.AccountID, &row.Kind, &row.Amount, &row.Ref, &row.Detail, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) ListCharacterAudits(characterID int64, limit, offset int) ([]AuditEntry, error) {
	if characterID <= 0 {
		return nil, errors.New("characterId is required")
	}
	limit = clampAdminLimit(limit)
	if offset < 0 {
		offset = 0
	}
	var accountID int64
	err := s.pool.QueryRow(context.Background(), `
		SELECT account_id FROM characters WHERE id = $1 AND is_deleted = FALSE
	`, characterID).Scan(&accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, COALESCE(admin_id, ''), action, target_type,
			COALESCE(target_id, ''), COALESCE(reason, ''), created_at
		FROM admin_audit_logs
		WHERE (target_type = 'character' AND target_id = $1)
			OR (target_type = 'account' AND target_id = $2)
		ORDER BY created_at DESC, id DESC
		LIMIT $3 OFFSET $4
	`, fmt.Sprint(characterID), fmt.Sprint(accountID), limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEntry{}
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

func (s *PostgresStore) AdminCharacterTimeline(characterID int64, filter AdminCharacterTimelineFilter) (AdminCharacterTimelinePage, error) {
	accountID, err := s.adminCharacterAccountID(characterID)
	if err != nil {
		return AdminCharacterTimelinePage{}, err
	}
	limit := clampAdminLimit(filter.Limit)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	types, err := adminTimelineTypes(filter.Types)
	if err != nil {
		return AdminCharacterTimelinePage{}, err
	}
	fetchLimit := limit + offset
	if fetchLimit < limit {
		fetchLimit = limit
	}

	items := []AdminCharacterTimelineItem{}
	total := 0
	if types["ledger"] {
		count, rows, err := s.adminTimelineLedger(characterID, fetchLimit)
		if err != nil {
			return AdminCharacterTimelinePage{}, err
		}
		total += count
		items = append(items, rows...)
	}
	if types["audit"] {
		count, rows, err := s.adminTimelineAudits(characterID, accountID, fetchLimit)
		if err != nil {
			return AdminCharacterTimelinePage{}, err
		}
		total += count
		items = append(items, rows...)
	}
	if types["risk"] {
		count, rows, err := s.adminTimelineRisk(accountID, fetchLimit)
		if err != nil {
			return AdminCharacterTimelinePage{}, err
		}
		total += count
		items = append(items, rows...)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	if offset >= len(items) {
		return AdminCharacterTimelinePage{Items: []AdminCharacterTimelineItem{}, Count: 0, Total: total, Limit: limit, Offset: offset}, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := append([]AdminCharacterTimelineItem(nil), items[offset:end]...)
	return AdminCharacterTimelinePage{Items: page, Count: len(page), Total: total, Limit: limit, Offset: offset}, nil
}

func adminTimelineTypes(raw []string) (map[string]bool, error) {
	out := map[string]bool{}
	for _, value := range raw {
		for _, part := range strings.Split(value, ",") {
			key := strings.ToLower(strings.TrimSpace(part))
			if key == "" {
				continue
			}
			switch key {
			case "ledger", "audit", "risk":
				out[key] = true
			default:
				return nil, fmt.Errorf("timeline type %q is not supported", key)
			}
		}
	}
	if len(out) == 0 {
		out["ledger"] = true
		out["audit"] = true
		out["risk"] = true
	}
	return out, nil
}

func (s *PostgresStore) adminCharacterAccountID(characterID int64) (int64, error) {
	if characterID <= 0 {
		return 0, errors.New("characterId is required")
	}
	var accountID int64
	err := s.pool.QueryRow(context.Background(), `
		SELECT account_id FROM characters WHERE id = $1 AND is_deleted = FALSE
	`, characterID).Scan(&accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return accountID, err
}

func (s *PostgresStore) adminTimelineLedger(characterID int64, limit int) (int, []AdminCharacterTimelineItem, error) {
	var total int
	if err := s.pool.QueryRow(context.Background(), `
		SELECT COUNT(*)::int FROM economy_ledger WHERE character_id = $1
	`, characterID).Scan(&total); err != nil {
		return 0, nil, err
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, COALESCE(account_id, 0), kind, COALESCE(amount::bigint, 0), COALESCE(ref_id, ''), COALESCE(reason, ''), created_at
		FROM economy_ledger
		WHERE character_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, characterID, limit)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	out := []AdminCharacterTimelineItem{}
	for rows.Next() {
		var row LedgerEntry
		if err := rows.Scan(&row.ID, &row.AccountID, &row.Kind, &row.Amount, &row.Ref, &row.Detail, &row.CreatedAt); err != nil {
			return 0, nil, err
		}
		out = append(out, AdminCharacterTimelineItem{
			Type:      "ledger",
			ID:        fmt.Sprintf("ledger:%d", row.ID),
			Title:     row.Kind,
			Detail:    row.Detail,
			Amount:    row.Amount,
			Ref:       row.Ref,
			CreatedAt: row.CreatedAt,
			Raw: map[string]any{
				"id":        row.ID,
				"accountId": row.AccountID,
				"kind":      row.Kind,
				"amount":    row.Amount,
				"ref":       row.Ref,
				"detail":    row.Detail,
			},
		})
	}
	return total, out, rows.Err()
}

func (s *PostgresStore) adminTimelineAudits(characterID, accountID int64, limit int) (int, []AdminCharacterTimelineItem, error) {
	characterTarget := fmt.Sprint(characterID)
	accountTarget := fmt.Sprint(accountID)
	var total int
	if err := s.pool.QueryRow(context.Background(), `
		SELECT COUNT(*)::int
		FROM admin_audit_logs
		WHERE (target_type = 'character' AND target_id = $1)
			OR (target_type = 'account' AND target_id = $2)
	`, characterTarget, accountTarget).Scan(&total); err != nil {
		return 0, nil, err
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, COALESCE(admin_id, ''), action, target_type, COALESCE(target_id, ''), COALESCE(reason, ''), created_at
		FROM admin_audit_logs
		WHERE (target_type = 'character' AND target_id = $1)
			OR (target_type = 'account' AND target_id = $2)
		ORDER BY created_at DESC, id DESC
		LIMIT $3
	`, characterTarget, accountTarget, limit)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	out := []AdminCharacterTimelineItem{}
	for rows.Next() {
		var row AuditEntry
		var targetType, targetID string
		if err := rows.Scan(&row.ID, &row.AdminID, &row.Action, &targetType, &targetID, &row.Reason, &row.CreatedAt); err != nil {
			return 0, nil, err
		}
		row.Target = targetType
		if targetID != "" {
			row.Target = targetType + ":" + targetID
		}
		out = append(out, AdminCharacterTimelineItem{
			Type:      "audit",
			ID:        fmt.Sprintf("audit:%d", row.ID),
			Title:     row.Action,
			Detail:    row.Reason,
			CreatedAt: row.CreatedAt,
			Raw: map[string]any{
				"id":      row.ID,
				"adminId": row.AdminID,
				"action":  row.Action,
				"target":  row.Target,
				"reason":  row.Reason,
			},
		})
	}
	return total, out, rows.Err()
}

func (s *PostgresStore) adminTimelineRisk(accountID int64, limit int) (int, []AdminCharacterTimelineItem, error) {
	var total int
	if err := s.pool.QueryRow(context.Background(), `
		SELECT COUNT(*)::int FROM account_risk_events WHERE account_id = $1
	`, accountID).Scan(&total); err != nil {
		return 0, nil, err
	}
	rows, err := s.pool.Query(context.Background(), `
		SELECT id, event_type, severity, COALESCE(device_id, ''), COALESCE(host(ip_address)::text, ''), COALESCE(wallet, ''), detail, created_at
		FROM account_risk_events
		WHERE account_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2
	`, accountID, limit)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()
	out := []AdminCharacterTimelineItem{}
	for rows.Next() {
		var id int64
		var eventType, deviceID, ipAddress, wallet string
		var severity int
		var raw []byte
		var createdAt time.Time
		if err := rows.Scan(&id, &eventType, &severity, &deviceID, &ipAddress, &wallet, &raw, &createdAt); err != nil {
			return 0, nil, err
		}
		detail := map[string]any{}
		_ = json.Unmarshal(raw, &detail)
		out = append(out, AdminCharacterTimelineItem{
			Type:      "risk",
			ID:        fmt.Sprintf("risk:%d", id),
			Title:     eventType,
			Severity:  severity,
			CreatedAt: createdAt,
			Raw: map[string]any{
				"id":        id,
				"accountId": accountID,
				"eventType": eventType,
				"severity":  severity,
				"deviceId":  deviceID,
				"ipAddress": ipAddress,
				"wallet":    wallet,
				"detail":    detail,
			},
		})
	}
	return total, out, rows.Err()
}

func (s *PostgresStore) AdminEquipmentDetail(equipmentUID string) (AdminEquipmentDetail, error) {
	equipmentUID = strings.TrimSpace(equipmentUID)
	if equipmentUID == "" {
		return AdminEquipmentDetail{}, errors.New("equipmentUid is required")
	}
	ctx := context.Background()
	var out AdminEquipmentDetail
	var affixes []byte
	err := s.pool.QueryRow(ctx, `
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
			e.account_id,
			COALESCE(e.character_id, 0),
			COALESCE(c.name, ''),
			COALESCE(a.solana_wallet_address, ''),
			a.status
		FROM equipment_items e
		JOIN accounts a ON a.id = e.account_id
		LEFT JOIN characters c ON c.id = e.character_id
		WHERE e.equipment_uid = $1
	`, equipmentUID).Scan(
		&out.Equipment.ID,
		&out.Equipment.EquipmentUID,
		&out.Equipment.ItemID,
		&out.Equipment.Rarity,
		&out.Equipment.EnhanceLevel,
		&out.Equipment.Durability,
		&out.Equipment.MaxDurability,
		&out.Equipment.Status,
		&out.Equipment.EquipSlot,
		&out.Equipment.Slot,
		&affixes,
		&out.Owner.AccountID,
		&out.Owner.CharacterID,
		&out.Owner.CharacterName,
		&out.Owner.WalletAddress,
		&out.Owner.AccountStatus,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminEquipmentDetail{}, ErrNotFound
	}
	if err != nil {
		return AdminEquipmentDetail{}, err
	}
	if len(affixes) > 0 {
		if err := json.Unmarshal(affixes, &out.Equipment.Affixes); err != nil {
			return AdminEquipmentDetail{}, err
		}
	}
	if err := s.attachAdminEquipmentNFT(ctx, out.Equipment.ID, &out); err != nil {
		return AdminEquipmentDetail{}, err
	}
	if err := s.attachAdminEquipmentMarketplace(ctx, out.Equipment.ID, &out); err != nil {
		return AdminEquipmentDetail{}, err
	}
	return out, nil
}

func (s *PostgresStore) attachAdminEquipmentNFT(ctx context.Context, equipmentID int64, out *AdminEquipmentDetail) error {
	var assetID *int64
	var assetStatus, mintAddress, metadataURI, requestStatus, txSignature *string
	var requestID *int64
	var assetCreatedAt, mintedAt, requestCreatedAt, submittedAt, confirmedAt *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT
			n.id,
			n.status,
			n.mint_address,
			n.metadata_uri,
			n.created_at,
			n.minted_at,
			r.id,
			r.status,
			r.tx_signature,
			r.created_at,
			r.submitted_at,
			r.confirmed_at
		FROM equipment_items e
		LEFT JOIN LATERAL (
			SELECT id, status, mint_address, metadata_uri, created_at, minted_at
			FROM nft_assets
			WHERE source_asset_type = 'EQUIPMENT' AND source_asset_id = e.id
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) n ON TRUE
		LEFT JOIN LATERAL (
			SELECT id, status, tx_signature, created_at, submitted_at, confirmed_at
			FROM nft_mint_requests
			WHERE source_asset_type = 'EQUIPMENT' AND source_asset_id = e.id
			ORDER BY created_at DESC, id DESC
			LIMIT 1
		) r ON TRUE
		WHERE e.id = $1
	`, equipmentID).Scan(
		&assetID,
		&assetStatus,
		&mintAddress,
		&metadataURI,
		&assetCreatedAt,
		&mintedAt,
		&requestID,
		&requestStatus,
		&txSignature,
		&requestCreatedAt,
		&submittedAt,
		&confirmedAt,
	)
	if err != nil {
		return err
	}
	if assetID == nil && requestID == nil {
		return nil
	}
	nft := &AdminEquipmentNFTDetail{MintedAt: mintedAt, SubmittedAt: submittedAt, ConfirmedAt: confirmedAt}
	if assetID != nil {
		nft.AssetID = *assetID
		nft.CreatedAt = assetCreatedAt
	}
	if assetStatus != nil {
		nft.Status = *assetStatus
		out.Equipment.NFTStatus = assetStatus
	}
	if mintAddress != nil {
		nft.MintAddress = *mintAddress
		if *mintAddress != "" {
			out.Equipment.NFTContract = mintAddress
		}
	}
	if metadataURI != nil {
		nft.MetadataURI = *metadataURI
	}
	if requestID != nil {
		nft.RequestID = *requestID
		if nft.CreatedAt == nil {
			nft.CreatedAt = requestCreatedAt
		}
	}
	if requestStatus != nil {
		nft.RequestStatus = *requestStatus
		if nft.Status == "" {
			nft.Status = *requestStatus
			out.Equipment.NFTStatus = requestStatus
		}
	}
	if txSignature != nil {
		nft.TxSignature = *txSignature
	}
	out.NFT = nft
	return nil
}

func (s *PostgresStore) attachAdminEquipmentMarketplace(ctx context.Context, equipmentID int64, out *AdminEquipmentDetail) error {
	rows, err := s.pool.Query(ctx, `
		SELECT id, seller_account_id, COALESCE(seller_character_id, 0), asset_type, asset_id,
			COALESCE(item_id, ''), quantity::bigint, price_token::bigint, listing_deposit_token::bigint,
			fee_bps, status, created_at, updated_at, cancelled_at, sold_at
		FROM marketplace_listings
		WHERE asset_type = 'EQUIPMENT' AND asset_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`, equipmentID)
	if err != nil {
		return err
	}
	defer rows.Close()
	listings, err := scanListings(rows)
	if err != nil {
		return err
	}
	if len(listings) > 0 {
		out.Marketplace = &listings[0]
	}
	return nil
}

func (s *PostgresStore) AdminCompensationPreview(previewID, adminID string) (AdminCompensationPreviewDetail, error) {
	previewID = strings.TrimSpace(previewID)
	if previewID == "" {
		return AdminCompensationPreviewDetail{}, errors.New("previewId is required")
	}
	var row AdminCompensationPreviewDetail
	var ownerID string
	var raw []byte
	err := s.pool.QueryRow(context.Background(), `
		SELECT
			p.preview_id,
			p.admin_id,
			p.status,
			p.expires_at,
			p.payload,
			p.created_at,
			p.committed_at,
			COUNT(t.character_id)::int
		FROM admin_operation_previews p
		LEFT JOIN admin_operation_preview_targets t ON t.preview_id = p.preview_id
		WHERE p.preview_id = $1 AND p.kind = 'COMPENSATION'
		GROUP BY p.preview_id, p.admin_id, p.status, p.expires_at, p.payload, p.created_at, p.committed_at
	`, previewID).Scan(&row.PreviewID, &ownerID, &row.Status, &row.ExpiresAt, &raw, &row.CreatedAt, &row.CommittedAt, &row.TargetCount)
	if errors.Is(err, pgx.ErrNoRows) {
		return AdminCompensationPreviewDetail{}, ErrNotFound
	}
	if err != nil {
		return AdminCompensationPreviewDetail{}, err
	}
	if strings.TrimSpace(adminID) != "" && ownerID != strings.TrimSpace(adminID) {
		return AdminCompensationPreviewDetail{}, ErrForbidden
	}
	var payload compensationPreviewPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return AdminCompensationPreviewDetail{}, err
	}
	row.Filters = payload.Filters
	row.Rewards = payload.Rewards
	return row, nil
}

func (s *PostgresStore) ListAdminCompensationPreviewTargets(previewID, adminID, keyword string, limit, offset int) (AdminCompensationTargetPage, error) {
	if _, err := s.AdminCompensationPreview(previewID, adminID); err != nil {
		return AdminCompensationTargetPage{}, err
	}
	limit = clampAdminLimit(limit)
	if offset < 0 {
		offset = 0
	}
	clauses := []string{"t.preview_id = $1"}
	args := []any{strings.TrimSpace(previewID)}
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		args = append(args, "%"+keyword+"%")
		placeholder := fmt.Sprintf("$%d", len(args))
		clauses = append(clauses, fmt.Sprintf(`(
			c.name ILIKE %[1]s
			OR c.id::text ILIKE %[1]s
			OR a.id::text ILIKE %[1]s
			OR COALESCE(a.solana_wallet_address, '') ILIKE %[1]s
		)`, placeholder))
	}
	where := strings.Join(clauses, " AND ")
	var total int
	if err := s.pool.QueryRow(context.Background(), `
		SELECT COUNT(*)::int
		FROM admin_operation_preview_targets t
		JOIN characters c ON c.id = t.character_id
		JOIN accounts a ON a.id = t.account_id
		WHERE `+where, args...).Scan(&total); err != nil {
		return AdminCompensationTargetPage{}, err
	}
	args = append(args, limit, offset)
	limitPlaceholder := fmt.Sprintf("$%d", len(args)-1)
	offsetPlaceholder := fmt.Sprintf("$%d", len(args))
	rows, err := s.pool.Query(context.Background(), `
		SELECT t.account_id, t.character_id, c.name, c.level, COALESCE(a.last_login_at, a.created_at)
		FROM admin_operation_preview_targets t
		JOIN characters c ON c.id = t.character_id
		JOIN accounts a ON a.id = t.account_id
		WHERE `+where+`
		ORDER BY t.character_id
		LIMIT `+limitPlaceholder+` OFFSET `+offsetPlaceholder, args...)
	if err != nil {
		return AdminCompensationTargetPage{}, err
	}
	defer rows.Close()
	items := []AdminCompensationTarget{}
	for rows.Next() {
		var target AdminCompensationTarget
		if err := rows.Scan(&target.AccountID, &target.CharacterID, &target.Name, &target.Level, &target.LastLoginAt); err != nil {
			return AdminCompensationTargetPage{}, err
		}
		items = append(items, target)
	}
	if err := rows.Err(); err != nil {
		return AdminCompensationTargetPage{}, err
	}
	return AdminCompensationTargetPage{Items: items, Count: len(items), Total: total, Limit: limit, Offset: offset}, nil
}
