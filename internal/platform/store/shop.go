package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type shopBuyPaymentPayload struct {
	ShopID         string            `json:"shopId"`
	ItemID         string            `json:"itemId"`
	Quantity       int64             `json:"quantity"`
	GrantGold      int64             `json:"grantGold,omitempty"`
	RewardPlan     DungeonRewardPlan `json:"rewardPlan"`
	ConfigSnapshot any               `json:"configSnapshot,omitempty"`
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
			ShopID:         strings.TrimSpace(req.ShopID),
			ItemID:         strings.TrimSpace(req.ItemID),
			Quantity:       req.Quantity,
			GrantGold:      req.GrantGold,
			RewardPlan:     req.RewardPlan,
			ConfigSnapshot: req.ConfigSnapshot,
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
		OpID:        order.ID,
		AccountID:   order.AccountID,
		CharacterID: order.CharacterID,
		ShopID:      payload.ShopID,
		ItemID:      payload.ItemID,
		Quantity:    payload.Quantity,
		GrantGold:   payload.GrantGold,
		RewardPlan:  payload.RewardPlan,
	}
	if err := s.lockCharacter(ctx, tx, order.AccountID, order.CharacterID); err != nil {
		return err
	}
	if err := s.deliverShopPurchase(ctx, tx, req, "shop_payment"); err != nil {
		return err
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
	if len(req.RewardPlan.Items) == 0 {
		return errors.New("shop purchase reward is not configured")
	}
	_, err := s.applyRewardsToBag(ctx, tx, req.AccountID, req.CharacterID, req.OpID, strings.TrimSpace(req.ShopID), source, req.RewardPlan)
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
