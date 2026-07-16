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

type shopBuyPaymentPayload struct {
	ShopID           string            `json:"shopId"`
	ItemID           string            `json:"itemId"`
	Quantity         int64             `json:"quantity"`
	MysterySlotIndex int               `json:"mysterySlotIndex,omitempty"`
	ShopSlotIndex    int               `json:"shopSlotIndex,omitempty"`
	ShopDailyLimit   int64             `json:"shopDailyLimit,omitempty"`
	ShopBusinessDate string            `json:"shopBusinessDate,omitempty"`
	GrantGold        int64             `json:"grantGold,omitempty"`
	RewardPlan       DungeonRewardPlan `json:"rewardPlan"`
	ConfigSnapshot   any               `json:"configSnapshot,omitempty"`
}

func (s *PostgresStore) ShopBuyGold(req ShopBuyRequest) (ShopBuyResult, error) {
	if req.GoldCost <= 0 {
		return ShopBuyResult{}, errors.New("gold price is not configured")
	}
	if req.Quantity <= 0 || strings.TrimSpace(req.ItemID) == "" || strings.TrimSpace(req.ShopID) == "" {
		return ShopBuyResult{}, errors.New("invalid shop buy request")
	}
	return runIdempotentAction(s, "shop_buy_gold", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (ShopBuyResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return ShopBuyResult{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO character_wallets (character_id) VALUES ($1) ON CONFLICT (character_id) DO NOTHING`, req.CharacterID); err != nil {
			return ShopBuyResult{}, err
		}
		if req.ShopSlotIndex > 0 {
			if err := s.recordShopDailyPurchaseTx(ctx, tx, req); err != nil {
				return ShopBuyResult{}, err
			}
		}
		tag, err := tx.Exec(ctx, `
			UPDATE character_wallets
			SET gold = gold - $2, updated_at = NOW()
			WHERE character_id = $1 AND gold >= $2
		`, req.CharacterID, req.GoldCost)
		if err != nil {
			return ShopBuyResult{}, err
		}
		if tag.RowsAffected() == 0 {
			return ShopBuyResult{}, ErrInsufficientBalance
		}
		if err := s.deliverShopPurchase(ctx, tx, req, "shop_buy"); err != nil {
			return ShopBuyResult{}, err
		}
		if req.MysterySlotIndex > 0 {
			if err := s.markMysteryShopOfferPurchased(ctx, tx, req.CharacterID, req.ShopID, req.MysterySlotIndex, req.ItemID, req.Quantity); err != nil {
				return ShopBuyResult{}, err
			}
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "SHOP_BUY_GOLD", req.ItemID, req.GoldCost, req.OpID); err != nil {
			return ShopBuyResult{}, err
		}
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		if err != nil {
			return ShopBuyResult{}, err
		}
		return ShopBuyResult{Snapshot: snapshot}, nil
	})
}

func (s *PostgresStore) CreateShopBuyPayment(req ShopBuyRequest) (ShopBuyResult, error) {
	if req.TokenCost <= 0 {
		return ShopBuyResult{}, errors.New("token price is not configured")
	}
	if req.Quantity <= 0 || strings.TrimSpace(req.ItemID) == "" || strings.TrimSpace(req.ShopID) == "" || strings.TrimSpace(req.ReceiverWallet) == "" {
		return ShopBuyResult{}, errors.New("invalid shop payment request")
	}
	return runIdempotentAction(s, "shop_buy_payment_create", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (ShopBuyResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return ShopBuyResult{}, err
		}
		payload, err := json.Marshal(shopBuyPaymentPayload{
			ShopID:           strings.TrimSpace(req.ShopID),
			ItemID:           strings.TrimSpace(req.ItemID),
			Quantity:         req.Quantity,
			MysterySlotIndex: req.MysterySlotIndex,
			ShopSlotIndex:    req.ShopSlotIndex,
			ShopDailyLimit:   req.ShopDailyLimit,
			ShopBusinessDate: req.ShopBusinessDate,
			GrantGold:        req.GrantGold,
			RewardPlan:       req.RewardPlan,
			ConfigSnapshot:   req.ConfigSnapshot,
		})
		if err != nil {
			return ShopBuyResult{}, err
		}
		order, err := s.insertPaymentOrder(ctx, tx, req.AccountID, req.CharacterID, PaymentPurposeShopBuy, req.TokenCost, req.ReceiverWallet, time.Now().UTC().Add(10*time.Minute), req.OpID, string(payload))
		if err != nil {
			return ShopBuyResult{}, err
		}
		return ShopBuyResult{Order: order}, nil
	})
}

func (s *PostgresStore) ShopSell(req ShopSellRequest) (ShopSellResult, error) {
	if req.GoldCredit <= 0 {
		return ShopSellResult{}, errors.New("sell price is not configured")
	}
	return runIdempotentAction(s, "shop_sell", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (ShopSellResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return ShopSellResult{}, err
		}
		if strings.TrimSpace(req.EquipmentUID) != "" {
			if err := s.sellShopEquipment(ctx, tx, req); err != nil {
				return ShopSellResult{}, err
			}
		} else {
			if err := s.sellShopInventory(ctx, tx, req); err != nil {
				return ShopSellResult{}, err
			}
		}
		if _, err := tx.Exec(ctx, `INSERT INTO character_wallets (character_id) VALUES ($1) ON CONFLICT (character_id) DO NOTHING`, req.CharacterID); err != nil {
			return ShopSellResult{}, err
		}
		if _, err := tx.Exec(ctx, `UPDATE character_wallets SET gold = gold + $2, updated_at = NOW() WHERE character_id = $1`, req.CharacterID, req.GoldCredit); err != nil {
			return ShopSellResult{}, err
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "SHOP_SELL", req.ShopID, req.GoldCredit, req.OpID); err != nil {
			return ShopSellResult{}, err
		}
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		if err != nil {
			return ShopSellResult{}, err
		}
		return ShopSellResult{Snapshot: snapshot}, nil
	})
}

func (s *PostgresStore) MysteryShopBoard(accountID, characterID int64, shopID string) (MysteryShopBoardState, error) {
	ctx := context.Background()
	var state MysteryShopBoardState
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		SELECT shop_id, character_id, next_free_refresh_at, generated_at, offers
		FROM mystery_shop_boards
		WHERE account_id = $1 AND character_id = $2 AND shop_id = $3
	`, accountID, characterID, strings.TrimSpace(shopID)).Scan(
		&state.ShopID,
		&state.CharacterID,
		&state.NextFreeRefreshAt,
		&state.GeneratedAt,
		&raw,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return MysteryShopBoardState{}, ErrNotFound
	}
	if err != nil {
		return MysteryShopBoardState{}, err
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &state.Offers); err != nil {
			return MysteryShopBoardState{}, err
		}
	}
	return state, nil
}

func (s *PostgresStore) MysteryShopPaidRefreshCount(accountID, characterID int64, shopID string, dayStart, dayEnd time.Time) (int, error) {
	var count int
	err := s.pool.QueryRow(context.Background(), `
		SELECT COUNT(*)::int
		FROM economy_ledger
		WHERE account_id = $1
			AND character_id = $2
			AND kind = 'MYSTERY_SHOP_REFRESH'
			AND ref_id = $3
			AND created_at >= $4
			AND created_at < $5
	`, accountID, characterID, strings.TrimSpace(shopID), dayStart, dayEnd).Scan(&count)
	return count, err
}

func (s *PostgresStore) RefreshMysteryShop(req MysteryShopRefreshRequest) (MysteryShopBoardState, error) {
	if strings.TrimSpace(req.ShopID) == "" || req.CharacterID == 0 {
		return MysteryShopBoardState{}, errors.New("invalid mystery shop refresh request")
	}
	if req.TokenCost < 0 {
		return MysteryShopBoardState{}, errors.New("invalid mystery shop refresh cost")
	}
	return runIdempotentAction(s, "mystery_shop_refresh", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (MysteryShopBoardState, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return MysteryShopBoardState{}, err
		}
		var spend TokenSpendBreakdown
		if req.TokenCost > 0 {
			var err error
			spend, err = s.spendTokenInTx(ctx, tx, req.AccountID, req.TokenCost)
			if err != nil {
				return MysteryShopBoardState{}, err
			}
		}
		raw, err := json.Marshal(req.Offers)
		if err != nil {
			return MysteryShopBoardState{}, err
		}
		var state MysteryShopBoardState
		if err := tx.QueryRow(ctx, `
			INSERT INTO mystery_shop_boards (
				account_id, character_id, shop_id, next_free_refresh_at, generated_at, offers, updated_at
			)
			VALUES ($1, $2, $3, $4, $5, $6::jsonb, NOW())
			ON CONFLICT (character_id, shop_id) DO UPDATE SET
				account_id = EXCLUDED.account_id,
				next_free_refresh_at = EXCLUDED.next_free_refresh_at,
				generated_at = EXCLUDED.generated_at,
				offers = EXCLUDED.offers,
				updated_at = NOW()
			RETURNING shop_id, character_id, next_free_refresh_at, generated_at
		`, req.AccountID, req.CharacterID, strings.TrimSpace(req.ShopID), req.NextFreeRefreshAt, req.GeneratedAt, raw).Scan(
			&state.ShopID,
			&state.CharacterID,
			&state.NextFreeRefreshAt,
			&state.GeneratedAt,
		); err != nil {
			return MysteryShopBoardState{}, err
		}
		state.Offers = append([]MysteryShopOffer(nil), req.Offers...)
		if req.TokenCost > 0 {
			if err := s.insertSystemConsumption(ctx, tx, req.OpID, req.AccountID, req.CharacterID, spend, "MYSTERY_SHOP_REFRESH", req.TokenCost, 0, req.TokenCost, 0, fmt.Sprintf(`{"shopId":%q}`, strings.TrimSpace(req.ShopID))); err != nil {
				return MysteryShopBoardState{}, err
			}
			if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "MYSTERY_SHOP_REFRESH", req.ShopID, req.TokenCost, req.OpID); err != nil {
				return MysteryShopBoardState{}, err
			}
		}
		return state, nil
	})
}

func (s *PostgresStore) fulfillShopBuyPaymentTx(ctx context.Context, tx pgx.Tx, order PaymentOrder) error {
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT payload FROM economy_payment_orders WHERE id = $1::uuid`, order.ID).Scan(&raw); err != nil {
		return err
	}
	var payload shopBuyPaymentPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	req := ShopBuyRequest{
		OpID:             order.ID,
		AccountID:        order.AccountID,
		CharacterID:      order.CharacterID,
		ShopID:           payload.ShopID,
		ItemID:           payload.ItemID,
		Quantity:         payload.Quantity,
		MysterySlotIndex: payload.MysterySlotIndex,
		ShopSlotIndex:    payload.ShopSlotIndex,
		ShopDailyLimit:   payload.ShopDailyLimit,
		ShopBusinessDate: payload.ShopBusinessDate,
		GrantGold:        payload.GrantGold,
		RewardPlan:       payload.RewardPlan,
	}
	if err := s.lockCharacter(ctx, tx, order.AccountID, order.CharacterID); err != nil {
		return err
	}
	if err := s.deliverShopPurchase(ctx, tx, req, "shop_payment"); err != nil {
		return err
	}
	if req.ShopSlotIndex > 0 {
		if err := s.recordShopDailyPurchaseTx(ctx, tx, req); err != nil {
			return err
		}
	}
	if req.MysterySlotIndex > 0 {
		if err := s.markMysteryShopOfferPurchased(ctx, tx, req.CharacterID, req.ShopID, req.MysterySlotIndex, req.ItemID, req.Quantity); err != nil {
			return err
		}
	}
	if err := s.insertEconomyLedger(ctx, tx, order.AccountID, order.CharacterID, "SHOP_BUY_TOKEN", payload.ItemID, order.Amount, order.ID); err != nil {
		return err
	}
	_, err := s.publishRareRewardAnnouncementsTx(ctx, tx, payload.RewardPlan, rareAnnouncementContext{
		AccountID: order.AccountID, CharacterID: order.CharacterID, Source: "商店购买",
		RefType: "shop_order", RefID: order.ID, AnnouncementOn: true,
	})
	return err
}

func (s *PostgresStore) deliverShopPurchase(ctx context.Context, tx pgx.Tx, req ShopBuyRequest, source string) error {
	if req.GrantGold > 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO character_wallets (character_id) VALUES ($1) ON CONFLICT (character_id) DO NOTHING`, req.CharacterID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE character_wallets SET gold = gold + $2, updated_at = NOW() WHERE character_id = $1`, req.CharacterID, req.GrantGold); err != nil {
			return err
		}
		return nil
	}
	if len(req.RewardPlan.Items) == 0 && req.RewardPlan.TokenReward <= 0 {
		return errors.New("shop purchase reward is not configured")
	}
	_, err := s.applyRewardsToBag(ctx, tx, req.AccountID, req.CharacterID, req.OpID, strings.TrimSpace(req.ShopID), source, req.RewardPlan)
	return err
}

func (s *PostgresStore) markMysteryShopOfferPurchased(ctx context.Context, tx pgx.Tx, characterID int64, shopID string, slotIndex int, itemID string, quantity int64) error {
	var raw []byte
	err := tx.QueryRow(ctx, `
		SELECT offers
		FROM mystery_shop_boards
		WHERE character_id = $1 AND shop_id = $2
		FOR UPDATE
	`, characterID, strings.TrimSpace(shopID)).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	var offers []MysteryShopOffer
	if err := json.Unmarshal(raw, &offers); err != nil {
		return err
	}
	found := false
	for index := range offers {
		offer := &offers[index]
		if offer.SlotIndex != slotIndex {
			continue
		}
		if strings.TrimSpace(offer.ItemID) != strings.TrimSpace(itemID) || offer.Quantity != quantity {
			return errors.New("mystery shop offer changed")
		}
		if offer.Purchased {
			return errors.New("mystery shop offer already purchased")
		}
		offer.Purchased = true
		found = true
		break
	}
	if !found {
		return ErrNotFound
	}
	updated, err := json.Marshal(offers)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		UPDATE mystery_shop_boards
		SET offers = $3::jsonb, updated_at = NOW()
		WHERE character_id = $1 AND shop_id = $2
	`, characterID, strings.TrimSpace(shopID), updated)
	return err
}

func (s *PostgresStore) ShopDailyPurchaseQuantities(accountID, characterID int64, shopID, businessDate string) (map[int]int64, error) {
	rows, err := s.pool.Query(context.Background(), `
		SELECT slot_index, quantity
		FROM shop_daily_purchases
		WHERE account_id = $1
			AND character_id = $2
			AND shop_id = $3
			AND business_date = $4::date
	`, accountID, characterID, strings.TrimSpace(shopID), strings.TrimSpace(businessDate))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]int64{}
	for rows.Next() {
		var slotIndex int
		var quantity int64
		if err := rows.Scan(&slotIndex, &quantity); err != nil {
			return nil, err
		}
		out[slotIndex] = quantity
	}
	return out, rows.Err()
}

func (s *PostgresStore) recordShopDailyPurchaseTx(ctx context.Context, tx pgx.Tx, req ShopBuyRequest) error {
	if req.ShopSlotIndex <= 0 {
		return nil
	}
	if req.ShopDailyLimit <= 0 || strings.TrimSpace(req.ShopBusinessDate) == "" {
		return errors.New("shop daily limit is not configured")
	}
	var current int64
	err := tx.QueryRow(ctx, `
		SELECT quantity
		FROM shop_daily_purchases
		WHERE character_id = $1
			AND shop_id = $2
			AND slot_index = $3
			AND business_date = $4::date
		FOR UPDATE
	`, req.CharacterID, strings.TrimSpace(req.ShopID), req.ShopSlotIndex, strings.TrimSpace(req.ShopBusinessDate)).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		if req.Quantity > req.ShopDailyLimit {
			return errors.New("shop daily purchase limit reached")
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO shop_daily_purchases (
				account_id, character_id, shop_id, slot_index, business_date, quantity, updated_at
			)
			VALUES ($1, $2, $3, $4, $5::date, $6, NOW())
		`, req.AccountID, req.CharacterID, strings.TrimSpace(req.ShopID), req.ShopSlotIndex, strings.TrimSpace(req.ShopBusinessDate), req.Quantity)
		return err
	}
	if err != nil {
		return err
	}
	if current+req.Quantity > req.ShopDailyLimit {
		return errors.New("shop daily purchase limit reached")
	}
	_, err = tx.Exec(ctx, `
		UPDATE shop_daily_purchases
		SET quantity = quantity + $5, updated_at = NOW()
		WHERE character_id = $1
			AND shop_id = $2
			AND slot_index = $3
			AND business_date = $4::date
	`, req.CharacterID, strings.TrimSpace(req.ShopID), req.ShopSlotIndex, strings.TrimSpace(req.ShopBusinessDate), req.Quantity)
	return err
}

func (s *PostgresStore) sellShopEquipment(ctx context.Context, tx pgx.Tx, req ShopSellRequest) error {
	var equipmentID int64
	err := tx.QueryRow(ctx, `
		SELECT e.id
		FROM equipment_items e
		WHERE e.account_id = $1
			AND e.character_id = $2
			AND e.equipment_uid = $3
			AND e.location = 'IN_BAG'
			AND NOT EXISTS (
				SELECT 1 FROM nft_assets n
				WHERE n.source_asset_type = 'EQUIPMENT'
					AND n.source_asset_id = e.id
					AND n.status IN ('MINT_REQUESTED', 'MINTED')
			)
		FOR UPDATE
	`, req.AccountID, req.CharacterID, strings.TrimSpace(req.EquipmentUID)).Scan(&equipmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `
		UPDATE equipment_items
		SET location = 'CONSUMED', slot = NULL, equip_slot = NULL, updated_at = NOW()
		WHERE id = $1
	`, equipmentID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrForbidden
	}
	return nil
}

func (s *PostgresStore) sellShopInventory(ctx context.Context, tx pgx.Tx, req ShopSellRequest) error {
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
		WHERE character_id = $1 AND location = 'BAG' AND slot = $2
		FOR UPDATE
	`, req.CharacterID, req.SlotIndex).Scan(&row.ID, &row.ItemID, &row.Quantity)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if req.Quantity > row.Quantity {
		return ErrInsufficientBalance
	}
	if req.Quantity == row.Quantity {
		_, err = tx.Exec(ctx, `UPDATE inventory_items SET location = 'CONSUMED', slot = NULL, updated_at = NOW() WHERE id = $1`, row.ID)
	} else {
		_, err = tx.Exec(ctx, `UPDATE inventory_items SET quantity = quantity - $2, updated_at = NOW() WHERE id = $1`, row.ID, req.Quantity)
	}
	return err
}
