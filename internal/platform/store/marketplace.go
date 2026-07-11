package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type MarketplaceRules struct {
	Enabled                       bool
	MinListPrice                  int64
	FeeBps                        int64
	FeeBurnBps                    int64
	FeeTreasuryBps                int64
	FeeRewardsBps                 int64
	ListingDepositBps             int64
	BaseListingSlots              int
	MaterialExpandSlots           int
	MaterialExpandMaxTimes        int
	MaterialExpandItemID          string
	MaterialExpandItemQuantity    int64
	WalletExpandSlots             int
	WalletExpandMaxTimes          int
	WalletExpandPriceToken        int64
	WalletExpandPaymentTimeoutSec int
	DepositReceiverWallet         string
	MaxListingsCreatedPerDay      int
	MaxCancelsPerDay              int
	MaxPurchasesPerDay            int
	PurchaseCooldownSeconds       int
	DailyLimitTimezone            string
	DefaultCooldownHours          int
}

func (r MarketplaceRules) withDefaults() MarketplaceRules {
	if r.MinListPrice <= 0 {
		r.MinListPrice = 1
	}
	if r.FeeBps <= 0 {
		r.FeeBps = 500
	}
	if r.FeeBurnBps <= 0 {
		r.FeeBurnBps = 100
	}
	if r.FeeTreasuryBps <= 0 {
		r.FeeTreasuryBps = 300
	}
	if r.FeeRewardsBps <= 0 {
		r.FeeRewardsBps = 100
	}
	if r.ListingDepositBps <= 0 {
		r.ListingDepositBps = 100
	}
	if r.BaseListingSlots <= 0 {
		r.BaseListingSlots = 5
	}
	if r.MaterialExpandSlots <= 0 {
		r.MaterialExpandSlots = 2
	}
	if r.MaterialExpandMaxTimes <= 0 {
		r.MaterialExpandMaxTimes = 5
	}
	if strings.TrimSpace(r.MaterialExpandItemID) == "" {
		r.MaterialExpandItemID = "market_stall_permit"
	}
	if r.MaterialExpandItemQuantity <= 0 {
		r.MaterialExpandItemQuantity = 1
	}
	if r.WalletExpandSlots <= 0 {
		r.WalletExpandSlots = 2
	}
	if r.WalletExpandMaxTimes <= 0 {
		r.WalletExpandMaxTimes = 5
	}
	if r.WalletExpandPriceToken <= 0 {
		r.WalletExpandPriceToken = 100
	}
	if r.WalletExpandPaymentTimeoutSec <= 0 {
		r.WalletExpandPaymentTimeoutSec = 600
	}
	if r.MaxListingsCreatedPerDay <= 0 {
		r.MaxListingsCreatedPerDay = 50
	}
	if r.MaxCancelsPerDay <= 0 {
		r.MaxCancelsPerDay = 20
	}
	if r.MaxPurchasesPerDay <= 0 {
		r.MaxPurchasesPerDay = 100
	}
	if r.PurchaseCooldownSeconds < 0 {
		r.PurchaseCooldownSeconds = 3
	}
	if strings.TrimSpace(r.DailyLimitTimezone) == "" {
		r.DailyLimitTimezone = "Asia/Shanghai"
	}
	if r.DefaultCooldownHours <= 0 {
		r.DefaultCooldownHours = 74
	}
	return r
}

func (r MarketplaceRules) SlotCapacity(materialExpand, walletExpand int) int {
	return r.BaseListingSlots + materialExpand*r.MaterialExpandSlots + walletExpand*r.WalletExpandSlots
}

type MarketplaceListRequest struct {
	OpID           string
	AccountID      int64
	CharacterID    int64
	AssetType      string // EQUIPMENT | ITEM
	EquipmentUID   string
	SourceLocation string // BAG | WAREHOUSE (items)
	SlotIndex      int
	Quantity       int64
	PriceToken     int64
	Rules          MarketplaceRules
}

type MarketplaceBuyRequest struct {
	OpID        string
	AccountID   int64
	CharacterID int64
	ListingID   int64
	Rules       MarketplaceRules
}

type MarketplaceCancelRequest struct {
	OpID      string
	AccountID int64
	ListingID int64
	Rules     MarketplaceRules
}

type MarketplaceExpandSlotsRequest struct {
	OpID        string
	AccountID   int64
	CharacterID int64
	Rules       MarketplaceRules
}

type MarketplaceExpandWalletRequest struct {
	OpID        string
	AccountID   int64
	CharacterID int64
	Rules       MarketplaceRules
}

type MarketplaceSubmitPaymentRequest struct {
	OpID        string
	AccountID   int64
	OrderID     string
	TxSignature string
}

type MarketplaceExpandWalletResult struct {
	Order PaymentOrder     `json:"order"`
	Slots MarketplaceSlots `json:"slots"`
}

type MarketplaceListing struct {
	ID                  int64      `json:"id"`
	SellerAccountID     int64      `json:"sellerAccountId"`
	SellerCharacterID   int64      `json:"sellerCharacterId,omitempty"`
	AssetType           string     `json:"assetType"`
	AssetID             int64      `json:"assetId"`
	ItemID              string     `json:"itemId"`
	Quantity            int64      `json:"quantity"`
	PriceToken          int64      `json:"priceToken"`
	ListingDepositToken int64      `json:"listingDepositToken"`
	FeeBps              int        `json:"feeBps"`
	Status              string     `json:"status"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	CancelledAt         *time.Time `json:"cancelledAt,omitempty"`
	SoldAt              *time.Time `json:"soldAt,omitempty"`
}

type MarketplaceOrder struct {
	ID                   string     `json:"id"`
	ListingID            int64      `json:"listingId"`
	BuyerAccountID       int64      `json:"buyerAccountId"`
	BuyerCharacterID     int64      `json:"buyerCharacterId,omitempty"`
	AmountToken          int64      `json:"amountToken"`
	FeeToken             int64      `json:"feeToken"`
	BurnToken            int64      `json:"burnToken"`
	TreasuryToken        int64      `json:"treasuryToken"`
	RewardsToken         int64      `json:"rewardsToken"`
	SellerProceedsToken  int64      `json:"sellerProceedsToken"`
	DepositReturnedToken int64      `json:"depositReturnedToken"`
	SpendSource          string     `json:"spendSource,omitempty"`
	Status               string     `json:"status"`
	CreatedAt            time.Time  `json:"createdAt"`
	CompletedAt          *time.Time `json:"completedAt,omitempty"`
}

type MarketplaceListResult struct {
	Listing  MarketplaceListing `json:"listing"`
	Snapshot EconomySnapshot    `json:"snapshot"`
}

type MarketplaceBuyResult struct {
	Listing  MarketplaceListing `json:"listing"`
	Order    MarketplaceOrder   `json:"order"`
	Snapshot EconomySnapshot    `json:"snapshot"`
}

type MarketplaceCancelResult struct {
	Listing  MarketplaceListing `json:"listing"`
	Snapshot EconomySnapshot    `json:"snapshot"`
}

type MarketplaceSlots struct {
	AccountID           int64 `json:"accountId"`
	BaseSlots           int   `json:"baseSlots"`
	MaterialExpandCount int   `json:"materialExpandCount"`
	WalletExpandCount   int   `json:"walletExpandCount"`
	Capacity            int   `json:"capacity"`
	Used                int   `json:"used"`
	Available           int   `json:"available"`
}

type MarketplaceExpandResult struct {
	Slots    MarketplaceSlots `json:"slots"`
	Snapshot EconomySnapshot  `json:"snapshot"`
}

type MarketplaceListFilter struct {
	Status    string
	AssetType string
	ItemID    string
	Limit     int
	Offset    int
}

func (s *PostgresStore) MarketplaceCreateListing(req MarketplaceListRequest) (MarketplaceListResult, error) {
	rules := req.Rules.withDefaults()
	req.AssetType = strings.ToUpper(strings.TrimSpace(req.AssetType))
	return runIdempotentAction(s, "marketplace_list", req.OpID, req.AccountID, req.CharacterID, func(ctx context.Context, tx pgx.Tx) (MarketplaceListResult, error) {
		if !rules.Enabled {
			return MarketplaceListResult{}, errors.New("marketplace is disabled")
		}
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return MarketplaceListResult{}, err
		}
		if err := s.assertHasTradingLicense(ctx, tx, req.AccountID); err != nil {
			return MarketplaceListResult{}, err
		}
		if err := s.assertMarketNotRestricted(ctx, tx, req.AccountID, "SELL"); err != nil {
			return MarketplaceListResult{}, err
		}
		if req.PriceToken < rules.MinListPrice {
			return MarketplaceListResult{}, fmt.Errorf("price below minimum %d", rules.MinListPrice)
		}
		if err := s.assertDailyListingCreateLimit(ctx, tx, req.AccountID, rules); err != nil {
			return MarketplaceListResult{}, err
		}
		if err := s.assertListingSlotAvailable(ctx, tx, req.AccountID, rules); err != nil {
			return MarketplaceListResult{}, err
		}

		deposit := bpsCeil(req.PriceToken, rules.ListingDepositBps)
		spend, err := s.spendTokenInTx(ctx, tx, req.AccountID, deposit)
		if err != nil {
			return MarketplaceListResult{}, fmt.Errorf("listing deposit: %w", err)
		}
		if err := s.insertSystemConsumption(ctx, tx, req.OpID+"#deposit", req.AccountID, req.CharacterID, spend, "MARKET_LISTING_DEPOSIT", deposit, 0, deposit, 0, `{"phase":"list"}`); err != nil {
			return MarketplaceListResult{}, err
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "MARKET_LISTING_DEPOSIT", req.OpID, deposit, req.OpID); err != nil {
			return MarketplaceListResult{}, err
		}

		var listing MarketplaceListing
		switch req.AssetType {
		case "EQUIPMENT":
			listing, err = s.lockEquipmentForListing(ctx, tx, req, deposit, int(rules.FeeBps))
		case "ITEM":
			listing, err = s.lockItemForListing(ctx, tx, req, deposit, int(rules.FeeBps))
		default:
			return MarketplaceListResult{}, errors.New("assetType must be EQUIPMENT or ITEM")
		}
		if err != nil {
			return MarketplaceListResult{}, err
		}
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		if err != nil {
			return MarketplaceListResult{}, err
		}
		return MarketplaceListResult{Listing: listing, Snapshot: snapshot}, nil
	})
}

func (s *PostgresStore) MarketplaceBuy(req MarketplaceBuyRequest) (MarketplaceBuyResult, error) {
	rules := req.Rules.withDefaults()
	return runIdempotentAction(s, "marketplace_buy", req.OpID, req.AccountID, req.CharacterID, func(ctx context.Context, tx pgx.Tx) (MarketplaceBuyResult, error) {
		if !rules.Enabled {
			return MarketplaceBuyResult{}, errors.New("marketplace is disabled")
		}
		if err := s.assertHasTradingLicense(ctx, tx, req.AccountID); err != nil {
			return MarketplaceBuyResult{}, err
		}
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return MarketplaceBuyResult{}, err
		}
		if err := s.assertMarketNotRestricted(ctx, tx, req.AccountID, "BUY"); err != nil {
			return MarketplaceBuyResult{}, err
		}
		if err := s.assertDailyPurchaseLimit(ctx, tx, req.AccountID, rules); err != nil {
			return MarketplaceBuyResult{}, err
		}
		if err := s.assertPurchaseCooldown(ctx, tx, req.AccountID, rules); err != nil {
			return MarketplaceBuyResult{}, err
		}

		listing, err := s.lockListing(ctx, tx, req.ListingID)
		if err != nil {
			return MarketplaceBuyResult{}, err
		}
		if listing.Status != "LISTED" {
			return MarketplaceBuyResult{}, errors.New("listing is not available")
		}
		if listing.SellerAccountID == req.AccountID {
			return MarketplaceBuyResult{}, errors.New("cannot buy own listing")
		}

		price := listing.PriceToken
		fee := bpsAmount(price, rules.FeeBps)
		burn := bpsAmount(price, rules.FeeBurnBps)
		treasury := bpsAmount(price, rules.FeeTreasuryBps)
		rewards := fee - burn - treasury
		if rewards < 0 {
			rewards = 0
		}
		sellerNet := price - fee
		depositReturn := listing.ListingDepositToken

		spend, err := s.spendTokenInTx(ctx, tx, req.AccountID, price)
		if err != nil {
			return MarketplaceBuyResult{}, err
		}
		if err := s.insertSystemConsumption(ctx, tx, req.OpID, req.AccountID, req.CharacterID, spend, "MARKET_BUY", price, burn, treasury, rewards, fmt.Sprintf(`{"listingId":%d}`, listing.ID)); err != nil {
			return MarketplaceBuyResult{}, err
		}
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "MARKET_BUY", fmt.Sprintf("%d", listing.ID), price, req.OpID); err != nil {
			return MarketplaceBuyResult{}, err
		}

		unlockAt := time.Now().UTC().Add(time.Duration(rules.DefaultCooldownHours) * time.Hour)
		if err := s.grantLockedTokenInTxAt(ctx, tx, listing.SellerAccountID, sellerNet, "marketplace", req.OpID, unlockAt); err != nil {
			return MarketplaceBuyResult{}, err
		}
		if depositReturn > 0 {
			if err := s.grantLockedTokenInTxAt(ctx, tx, listing.SellerAccountID, depositReturn, "marketplace_deposit_return", req.OpID, unlockAt); err != nil {
				return MarketplaceBuyResult{}, err
			}
		}
		if err := s.insertEconomyLedger(ctx, tx, listing.SellerAccountID, listing.SellerCharacterID, "MARKET_SALE_PROCEEDS", req.OpID, sellerNet+depositReturn, req.OpID); err != nil {
			return MarketplaceBuyResult{}, err
		}

		if err := s.transferListedAssetToBuyer(ctx, tx, listing, req.AccountID, req.CharacterID); err != nil {
			return MarketplaceBuyResult{}, err
		}

		now := time.Now().UTC()
		tag, err := tx.Exec(ctx, `
			UPDATE marketplace_listings
			SET status = 'SOLD', sold_at = $2, updated_at = $2
			WHERE id = $1 AND status = 'LISTED'
		`, listing.ID, now)
		if err != nil {
			return MarketplaceBuyResult{}, err
		}
		if tag.RowsAffected() == 0 {
			return MarketplaceBuyResult{}, errors.New("listing is not available")
		}
		listing.Status = "SOLD"
		listing.SoldAt = &now
		listing.UpdatedAt = now

		var orderID string
		var completedAt time.Time
		err = tx.QueryRow(ctx, `
			INSERT INTO marketplace_orders (
				listing_id, buyer_account_id, buyer_character_id,
				amount_token, fee_token, burn_token, treasury_token, rewards_token,
				seller_proceeds_token, deposit_returned_token, spend_source,
				status, op_id, expires_at, completed_at
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11,
				'COMPLETED', $12, $13, $13
			)
			RETURNING id::text, completed_at
		`, listing.ID, req.AccountID, req.CharacterID, price, fee, burn, treasury, rewards,
			sellerNet, depositReturn, spend.SpendSource, req.OpID, now,
		).Scan(&orderID, &completedAt)
		if err != nil {
			return MarketplaceBuyResult{}, err
		}
		order := MarketplaceOrder{
			ID:                   orderID,
			ListingID:            listing.ID,
			BuyerAccountID:       req.AccountID,
			BuyerCharacterID:     req.CharacterID,
			AmountToken:          price,
			FeeToken:             fee,
			BurnToken:            burn,
			TreasuryToken:        treasury,
			RewardsToken:         rewards,
			SellerProceedsToken:  sellerNet,
			DepositReturnedToken: depositReturn,
			SpendSource:          spend.SpendSource,
			Status:               "COMPLETED",
			CreatedAt:            completedAt,
			CompletedAt:          &completedAt,
		}
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		if err != nil {
			return MarketplaceBuyResult{}, err
		}
		return MarketplaceBuyResult{Listing: listing, Order: order, Snapshot: snapshot}, nil
	})
}

func (s *PostgresStore) MarketplaceCancel(req MarketplaceCancelRequest) (MarketplaceCancelResult, error) {
	rules := req.Rules.withDefaults()
	return runIdempotentAction(s, "marketplace_cancel", req.OpID, req.AccountID, 0, func(ctx context.Context, tx pgx.Tx) (MarketplaceCancelResult, error) {
		if !rules.Enabled {
			return MarketplaceCancelResult{}, errors.New("marketplace is disabled")
		}
		if err := s.assertDailyCancelLimit(ctx, tx, req.AccountID, rules); err != nil {
			return MarketplaceCancelResult{}, err
		}
		listing, err := s.lockListing(ctx, tx, req.ListingID)
		if err != nil {
			return MarketplaceCancelResult{}, err
		}
		if listing.SellerAccountID != req.AccountID {
			return MarketplaceCancelResult{}, ErrForbidden
		}
		if listing.Status != "LISTED" {
			return MarketplaceCancelResult{}, errors.New("listing is not cancellable")
		}
		sellerCharacterID := listing.SellerCharacterID
		if sellerCharacterID <= 0 {
			return MarketplaceCancelResult{}, errors.New("listing missing seller character")
		}
		if err := s.lockCharacter(ctx, tx, req.AccountID, sellerCharacterID); err != nil {
			return MarketplaceCancelResult{}, err
		}
		if err := s.returnListedAssetToSellerBag(ctx, tx, listing); err != nil {
			return MarketplaceCancelResult{}, err
		}
		now := time.Now().UTC()
		tag, err := tx.Exec(ctx, `
			UPDATE marketplace_listings
			SET status = 'CANCELLED', cancelled_at = $2, updated_at = $2
			WHERE id = $1 AND status = 'LISTED'
		`, listing.ID, now)
		if err != nil {
			return MarketplaceCancelResult{}, err
		}
		if tag.RowsAffected() == 0 {
			return MarketplaceCancelResult{}, errors.New("listing is not cancellable")
		}
		listing.Status = "CANCELLED"
		listing.CancelledAt = &now
		listing.UpdatedAt = now

		// Deposit already spent at list time; cancel keeps it as treasury (no refund).
		if listing.ListingDepositToken > 0 {
			if err := s.insertEconomyLedger(ctx, tx, req.AccountID, sellerCharacterID, "MARKET_LISTING_DEPOSIT_TREASURY", fmt.Sprintf("%d", listing.ID), listing.ListingDepositToken, req.OpID); err != nil {
				return MarketplaceCancelResult{}, err
			}
		}
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, sellerCharacterID)
		if err != nil {
			return MarketplaceCancelResult{}, err
		}
		return MarketplaceCancelResult{Listing: listing, Snapshot: snapshot}, nil
	})
}

func (s *PostgresStore) MarketplaceExpandMaterialSlots(req MarketplaceExpandSlotsRequest) (MarketplaceExpandResult, error) {
	rules := req.Rules.withDefaults()
	return runIdempotentAction(s, "marketplace_expand_material", req.OpID, req.AccountID, req.CharacterID, func(ctx context.Context, tx pgx.Tx) (MarketplaceExpandResult, error) {
		if !rules.Enabled {
			return MarketplaceExpandResult{}, errors.New("marketplace is disabled")
		}
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return MarketplaceExpandResult{}, err
		}
		slots, err := s.ensureMarketSlots(ctx, tx, req.AccountID)
		if err != nil {
			return MarketplaceExpandResult{}, err
		}
		if slots.MaterialExpandCount >= rules.MaterialExpandMaxTimes {
			return MarketplaceExpandResult{}, errors.New("material market slot expands exhausted")
		}
		if err := s.consumeBagItem(ctx, tx, req.CharacterID, rules.MaterialExpandItemID, rules.MaterialExpandItemQuantity); err != nil {
			return MarketplaceExpandResult{}, err
		}
		_, err = tx.Exec(ctx, `
			UPDATE account_market_slots
			SET material_expand_count = material_expand_count + 1, updated_at = NOW()
			WHERE account_id = $1
		`, req.AccountID)
		if err != nil {
			return MarketplaceExpandResult{}, err
		}
		slots.MaterialExpandCount++
		slots = s.decorateSlots(req.AccountID, slots, rules)
		if err := s.insertEconomyLedger(ctx, tx, req.AccountID, req.CharacterID, "MARKET_SLOTS_EXPANDED", rules.MaterialExpandItemID, 1, req.OpID); err != nil {
			return MarketplaceExpandResult{}, err
		}
		snapshot, err := s.economySnapshot(ctx, tx, req.AccountID, req.CharacterID)
		if err != nil {
			return MarketplaceExpandResult{}, err
		}
		return MarketplaceExpandResult{Slots: slots, Snapshot: snapshot}, nil
	})
}

func (s *PostgresStore) MarketplaceExpandWalletSlots(req MarketplaceExpandWalletRequest) (MarketplaceExpandWalletResult, error) {
	rules := req.Rules.withDefaults()
	return runIdempotentAction(s, "marketplace_expand_wallet", req.OpID, req.AccountID, req.CharacterID, func(ctx context.Context, tx pgx.Tx) (MarketplaceExpandWalletResult, error) {
		if !rules.Enabled {
			return MarketplaceExpandWalletResult{}, errors.New("marketplace is disabled")
		}
		if strings.TrimSpace(rules.DepositReceiverWallet) == "" {
			return MarketplaceExpandWalletResult{}, errors.New("deposit receiver wallet is not configured")
		}
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return MarketplaceExpandWalletResult{}, err
		}
		slots, err := s.ensureMarketSlots(ctx, tx, req.AccountID)
		if err != nil {
			return MarketplaceExpandWalletResult{}, err
		}
		if slots.WalletExpandCount >= rules.WalletExpandMaxTimes {
			return MarketplaceExpandWalletResult{}, errors.New("wallet market slot expands exhausted")
		}
		var openCount int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*)::int FROM economy_payment_orders
			WHERE account_id = $1 AND purpose = 'MARKET_SLOT_WALLET_EXPAND'
				AND status IN ('PENDING_PAYMENT', 'SUBMITTED', 'CONFIRMED')
		`, req.AccountID).Scan(&openCount); err != nil {
			return MarketplaceExpandWalletResult{}, err
		}
		if openCount > 0 {
			return MarketplaceExpandWalletResult{}, errors.New("open wallet expand payment already exists")
		}
		expires := time.Now().UTC().Add(time.Duration(rules.WalletExpandPaymentTimeoutSec) * time.Second)
		var order PaymentOrder
		err = tx.QueryRow(ctx, `
			INSERT INTO economy_payment_orders (
				account_id, character_id, purpose, pay_asset, amount, receiver_wallet, status, payload, expires_at
			) VALUES (
				$1, $2, 'MARKET_SLOT_WALLET_EXPAND', 'AEB', $3, $4, 'PENDING_PAYMENT',
				jsonb_build_object('slotsPerExpand', $5::int, 'opId', $6::text), $7
			)
			RETURNING id::text, account_id, COALESCE(character_id, 0), purpose, pay_asset, amount::bigint,
				COALESCE(receiver_wallet, ''), status, created_at, expires_at
		`, req.AccountID, req.CharacterID, rules.WalletExpandPriceToken, rules.DepositReceiverWallet,
			rules.WalletExpandSlots, req.OpID, expires,
		).Scan(
			&order.ID, &order.AccountID, &order.CharacterID, &order.Purpose, &order.PayAsset, &order.Amount,
			&order.ReceiverWallet, &order.Status, &order.CreatedAt, &order.ExpiresAt,
		)
		if err != nil {
			return MarketplaceExpandWalletResult{}, err
		}
		slots = s.decorateSlots(req.AccountID, slots, rules)
		return MarketplaceExpandWalletResult{Order: order, Slots: slots}, nil
	})
}

func (s *PostgresStore) MarketplaceSubmitWalletExpandPayment(req MarketplaceSubmitPaymentRequest) (PaymentOrder, error) {
	return runIdempotentAction(s, "payment_submit", req.OpID, req.AccountID, 0, func(ctx context.Context, tx pgx.Tx) (PaymentOrder, error) {
		sig := strings.TrimSpace(req.TxSignature)
		if sig == "" {
			return PaymentOrder{}, errors.New("txSignature is required")
		}
		order, err := s.lockPaymentOrder(ctx, tx, req.OrderID)
		if err != nil {
			return PaymentOrder{}, err
		}
		if order.AccountID != req.AccountID {
			return PaymentOrder{}, ErrForbidden
		}
		if !supportedPaymentPurpose(order.Purpose) {
			return PaymentOrder{}, errors.New("order purpose mismatch")
		}
		if order.Status != "PENDING_PAYMENT" && order.Status != "SUBMITTED" {
			return PaymentOrder{}, fmt.Errorf("order status %s cannot accept signature", order.Status)
		}
		if time.Now().UTC().After(order.ExpiresAt) {
			_, _ = tx.Exec(ctx, `UPDATE economy_payment_orders SET status = 'EXPIRED' WHERE id = $1::uuid`, order.ID)
			return PaymentOrder{}, errors.New("payment order expired")
		}
		now := time.Now().UTC()
		tag, err := tx.Exec(ctx, `
			UPDATE economy_payment_orders
			SET status = 'SUBMITTED', tx_signature = $2, submitted_at = $3
			WHERE id = $1::uuid AND status IN ('PENDING_PAYMENT', 'SUBMITTED')
		`, order.ID, sig, now)
		if err != nil {
			return PaymentOrder{}, err
		}
		if tag.RowsAffected() == 0 {
			return PaymentOrder{}, errors.New("payment order update failed")
		}
		return s.lockPaymentOrder(ctx, tx, order.ID)
	})
}

func (s *PostgresStore) MarketplaceSlots(accountID int64, rules MarketplaceRules) (MarketplaceSlots, error) {
	rules = rules.withDefaults()
	ctx := context.Background()
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return MarketplaceSlots{}, err
	}
	defer rollback(ctx, tx)
	slots, err := s.ensureMarketSlots(ctx, tx, accountID)
	if err != nil {
		return MarketplaceSlots{}, err
	}
	used, err := s.countActiveListings(ctx, tx, accountID)
	if err != nil {
		return MarketplaceSlots{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return MarketplaceSlots{}, err
	}
	slots.Used = used
	return s.decorateSlots(accountID, slots, rules), nil
}

func (s *PostgresStore) MarketplaceListListings(filter MarketplaceListFilter) ([]MarketplaceListing, error) {
	ctx := context.Background()
	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	status := strings.TrimSpace(filter.Status)
	if status == "" {
		status = "LISTED"
	}
	args := []any{status, limit, offset}
	query := `
		SELECT id, seller_account_id, COALESCE(seller_character_id, 0), asset_type, asset_id,
			COALESCE(item_id, ''), quantity::bigint, price_token::bigint, listing_deposit_token::bigint,
			fee_bps, status, created_at, updated_at, cancelled_at, sold_at
		FROM marketplace_listings
		WHERE status = $1
	`
	argN := 4
	if assetType := strings.ToUpper(strings.TrimSpace(filter.AssetType)); assetType != "" {
		query += fmt.Sprintf(" AND asset_type = $%d", argN)
		args = append(args, assetType)
		argN++
	}
	if itemID := strings.TrimSpace(filter.ItemID); itemID != "" {
		query += fmt.Sprintf(" AND item_id = $%d", argN)
		args = append(args, itemID)
		argN++
	}
	query += " ORDER BY created_at DESC LIMIT $2 OFFSET $3"
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanListings(rows)
}

func (s *PostgresStore) MarketplaceMyListings(accountID int64, status string, limit, offset int) ([]MarketplaceListing, error) {
	ctx := context.Background()
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	query := `
		SELECT id, seller_account_id, COALESCE(seller_character_id, 0), asset_type, asset_id,
			COALESCE(item_id, ''), quantity::bigint, price_token::bigint, listing_deposit_token::bigint,
			fee_bps, status, created_at, updated_at, cancelled_at, sold_at
		FROM marketplace_listings
		WHERE seller_account_id = $1
	`
	args := []any{accountID}
	if strings.TrimSpace(status) != "" {
		query += " AND status = $2"
		args = append(args, strings.TrimSpace(status))
		query += " ORDER BY created_at DESC LIMIT $3 OFFSET $4"
		args = append(args, limit, offset)
	} else {
		query += " ORDER BY created_at DESC LIMIT $2 OFFSET $3"
		args = append(args, limit, offset)
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanListings(rows)
}

func scanListings(rows pgx.Rows) ([]MarketplaceListing, error) {
	out := []MarketplaceListing{}
	for rows.Next() {
		var row MarketplaceListing
		var cancelled, sold pgtype.Timestamptz
		if err := rows.Scan(
			&row.ID, &row.SellerAccountID, &row.SellerCharacterID, &row.AssetType, &row.AssetID,
			&row.ItemID, &row.Quantity, &row.PriceToken, &row.ListingDepositToken,
			&row.FeeBps, &row.Status, &row.CreatedAt, &row.UpdatedAt, &cancelled, &sold,
		); err != nil {
			return nil, err
		}
		if cancelled.Valid {
			t := cancelled.Time
			row.CancelledAt = &t
		}
		if sold.Valid {
			t := sold.Time
			row.SoldAt = &t
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *PostgresStore) lockEquipmentForListing(ctx context.Context, tx pgx.Tx, req MarketplaceListRequest, deposit int64, feeBps int) (MarketplaceListing, error) {
	uid := strings.TrimSpace(req.EquipmentUID)
	if uid == "" {
		return MarketplaceListing{}, errors.New("equipmentUid is required")
	}
	var assetID int64
	var itemID, bindType, location string
	var tradable bool
	err := tx.QueryRow(ctx, `
		SELECT e.id, e.item_id, e.bind_type, e.location, COALESCE(c.tradable, FALSE)
		FROM equipment_items e
		LEFT JOIN item_catalog c ON c.item_id = e.item_id
		WHERE e.account_id = $1 AND e.character_id = $2 AND e.equipment_uid = $3
		FOR UPDATE OF e
	`, req.AccountID, req.CharacterID, uid).Scan(&assetID, &itemID, &bindType, &location, &tradable)
	if errors.Is(err, pgx.ErrNoRows) {
		return MarketplaceListing{}, ErrNotFound
	}
	if err != nil {
		return MarketplaceListing{}, err
	}
	if location != "IN_BAG" && location != "IN_WAREHOUSE" {
		return MarketplaceListing{}, errors.New("equipment must be in bag or warehouse to list")
	}
	if bindType != "UNBOUND" {
		return MarketplaceListing{}, errors.New("only UNBOUND equipment can be listed")
	}
	if !tradable {
		return MarketplaceListing{}, errors.New("equipment item is not tradable")
	}
	tag, err := tx.Exec(ctx, `
		UPDATE equipment_items
		SET location = 'LISTED', slot = NULL, updated_at = NOW()
		WHERE id = $1 AND location IN ('IN_BAG', 'IN_WAREHOUSE')
	`, assetID)
	if err != nil {
		return MarketplaceListing{}, err
	}
	if tag.RowsAffected() == 0 {
		return MarketplaceListing{}, errors.New("equipment could not be listed")
	}
	return s.insertListing(ctx, tx, req, "EQUIPMENT", assetID, itemID, 1, deposit, feeBps)
}

func (s *PostgresStore) lockItemForListing(ctx context.Context, tx pgx.Tx, req MarketplaceListRequest, deposit int64, feeBps int) (MarketplaceListing, error) {
	if req.SlotIndex < 0 {
		return MarketplaceListing{}, errors.New("slotIndex is required")
	}
	if req.Quantity <= 0 {
		return MarketplaceListing{}, errors.New("quantity must be positive")
	}
	location := strings.ToUpper(strings.TrimSpace(req.SourceLocation))
	if location == "" {
		location = "BAG"
	}
	if location != "BAG" && location != "WAREHOUSE" {
		return MarketplaceListing{}, errors.New("sourceLocation must be BAG or WAREHOUSE")
	}
	var rowID int64
	var itemID, bindType string
	var available int64
	var tradable bool
	err := tx.QueryRow(ctx, `
		SELECT i.id, i.item_id, i.quantity, i.bind_type, COALESCE(c.tradable, FALSE)
		FROM inventory_items i
		LEFT JOIN item_catalog c ON c.item_id = i.item_id
		WHERE i.character_id = $1 AND i.location = $2 AND i.slot = $3
		FOR UPDATE OF i
	`, req.CharacterID, location, req.SlotIndex).Scan(&rowID, &itemID, &available, &bindType, &tradable)
	if errors.Is(err, pgx.ErrNoRows) {
		return MarketplaceListing{}, ErrNotFound
	}
	if err != nil {
		return MarketplaceListing{}, err
	}
	if bindType != "UNBOUND" {
		return MarketplaceListing{}, errors.New("only UNBOUND items can be listed")
	}
	if !tradable {
		return MarketplaceListing{}, errors.New("item is not tradable")
	}
	if req.Quantity > available {
		return MarketplaceListing{}, ErrInsufficientBalance
	}
	var listedID int64
	if req.Quantity == available {
		err = tx.QueryRow(ctx, `
			UPDATE inventory_items
			SET location = 'LISTED', slot = NULL, updated_at = NOW()
			WHERE id = $1
			RETURNING id
		`, rowID).Scan(&listedID)
	} else {
		_, err = tx.Exec(ctx, `
			UPDATE inventory_items
			SET quantity = quantity - $2, updated_at = NOW()
			WHERE id = $1
		`, rowID, req.Quantity)
		if err == nil {
			err = tx.QueryRow(ctx, `
				INSERT INTO inventory_items (character_id, item_id, quantity, location, slot, bind_type)
				VALUES ($1, $2, $3, 'LISTED', NULL, 'UNBOUND')
				RETURNING id
			`, req.CharacterID, itemID, req.Quantity).Scan(&listedID)
		}
	}
	if err != nil {
		return MarketplaceListing{}, err
	}
	return s.insertListing(ctx, tx, req, "ITEM", listedID, itemID, req.Quantity, deposit, feeBps)
}

func (s *PostgresStore) insertListing(ctx context.Context, tx pgx.Tx, req MarketplaceListRequest, assetType string, assetID int64, itemID string, quantity, deposit int64, feeBps int) (MarketplaceListing, error) {
	var row MarketplaceListing
	err := tx.QueryRow(ctx, `
		INSERT INTO marketplace_listings (
			seller_account_id, seller_character_id, asset_type, asset_id, item_id, quantity,
			price_token, listing_deposit_token, fee_bps, status, op_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'LISTED', $10)
		RETURNING id, seller_account_id, COALESCE(seller_character_id, 0), asset_type, asset_id,
			COALESCE(item_id, ''), quantity::bigint, price_token::bigint, listing_deposit_token::bigint,
			fee_bps, status, created_at, updated_at
	`, req.AccountID, req.CharacterID, assetType, assetID, itemID, quantity, req.PriceToken, deposit, feeBps, req.OpID).Scan(
		&row.ID, &row.SellerAccountID, &row.SellerCharacterID, &row.AssetType, &row.AssetID,
		&row.ItemID, &row.Quantity, &row.PriceToken, &row.ListingDepositToken,
		&row.FeeBps, &row.Status, &row.CreatedAt, &row.UpdatedAt,
	)
	return row, err
}

func (s *PostgresStore) lockListing(ctx context.Context, tx pgx.Tx, listingID int64) (MarketplaceListing, error) {
	var row MarketplaceListing
	var cancelled, sold pgtype.Timestamptz
	err := tx.QueryRow(ctx, `
		SELECT id, seller_account_id, COALESCE(seller_character_id, 0), asset_type, asset_id,
			COALESCE(item_id, ''), quantity::bigint, price_token::bigint, listing_deposit_token::bigint,
			fee_bps, status, created_at, updated_at, cancelled_at, sold_at
		FROM marketplace_listings
		WHERE id = $1
		FOR UPDATE
	`, listingID).Scan(
		&row.ID, &row.SellerAccountID, &row.SellerCharacterID, &row.AssetType, &row.AssetID,
		&row.ItemID, &row.Quantity, &row.PriceToken, &row.ListingDepositToken,
		&row.FeeBps, &row.Status, &row.CreatedAt, &row.UpdatedAt, &cancelled, &sold,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return MarketplaceListing{}, ErrNotFound
	}
	if err != nil {
		return MarketplaceListing{}, err
	}
	if cancelled.Valid {
		t := cancelled.Time
		row.CancelledAt = &t
	}
	if sold.Valid {
		t := sold.Time
		row.SoldAt = &t
	}
	return row, nil
}

func (s *PostgresStore) transferListedAssetToBuyer(ctx context.Context, tx pgx.Tx, listing MarketplaceListing, buyerAccountID, buyerCharacterID int64) error {
	switch listing.AssetType {
	case "EQUIPMENT":
		targetSlot, err := s.resolveStorageSlot(ctx, tx, buyerCharacterID, "BAG", -1)
		if err != nil {
			return fmt.Errorf("buyer bag full: %w", err)
		}
		tag, err := tx.Exec(ctx, `
			UPDATE equipment_items
			SET account_id = $2, character_id = $3, location = 'IN_BAG', slot = $4, updated_at = NOW()
			WHERE id = $1 AND location = 'LISTED'
		`, listing.AssetID, buyerAccountID, buyerCharacterID, targetSlot)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("listed equipment missing")
		}
		return nil
	case "ITEM":
		targetSlot, err := s.resolveStorageSlot(ctx, tx, buyerCharacterID, "BAG", -1)
		if err != nil {
			return fmt.Errorf("buyer bag full: %w", err)
		}
		tag, err := tx.Exec(ctx, `
			UPDATE inventory_items
			SET character_id = $2, location = 'BAG', slot = $3, updated_at = NOW()
			WHERE id = $1 AND location = 'LISTED'
		`, listing.AssetID, buyerCharacterID, targetSlot)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("listed item missing")
		}
		return nil
	default:
		return errors.New("unsupported asset type")
	}
}

func (s *PostgresStore) returnListedAssetToSellerBag(ctx context.Context, tx pgx.Tx, listing MarketplaceListing) error {
	switch listing.AssetType {
	case "EQUIPMENT":
		targetSlot, err := s.resolveStorageSlot(ctx, tx, listing.SellerCharacterID, "BAG", -1)
		if err != nil {
			return fmt.Errorf("seller bag full: %w", err)
		}
		tag, err := tx.Exec(ctx, `
			UPDATE equipment_items
			SET location = 'IN_BAG', slot = $2, character_id = $3, updated_at = NOW()
			WHERE id = $1 AND location = 'LISTED' AND account_id = $4
		`, listing.AssetID, targetSlot, listing.SellerCharacterID, listing.SellerAccountID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("listed equipment missing")
		}
		return nil
	case "ITEM":
		targetSlot, err := s.resolveStorageSlot(ctx, tx, listing.SellerCharacterID, "BAG", -1)
		if err != nil {
			return fmt.Errorf("seller bag full: %w", err)
		}
		tag, err := tx.Exec(ctx, `
			UPDATE inventory_items
			SET character_id = $2, location = 'BAG', slot = $3, updated_at = NOW()
			WHERE id = $1 AND location = 'LISTED'
		`, listing.AssetID, listing.SellerCharacterID, targetSlot)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return errors.New("listed item missing")
		}
		return nil
	default:
		return errors.New("unsupported asset type")
	}
}

func (s *PostgresStore) assertMarketNotRestricted(ctx context.Context, tx pgx.Tx, accountID int64, action string) error {
	var blocked bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM account_market_restrictions
			WHERE account_id = $1
				AND revoked_at IS NULL
				AND (expires_at IS NULL OR expires_at > NOW())
				AND restriction_type IN ($2, 'ALL')
		)
	`, accountID, action).Scan(&blocked)
	if err != nil {
		return err
	}
	if blocked {
		return errors.New("account marketplace access is restricted")
	}
	return nil
}

func (s *PostgresStore) assertListingSlotAvailable(ctx context.Context, tx pgx.Tx, accountID int64, rules MarketplaceRules) error {
	slots, err := s.ensureMarketSlots(ctx, tx, accountID)
	if err != nil {
		return err
	}
	used, err := s.countActiveListings(ctx, tx, accountID)
	if err != nil {
		return err
	}
	capacity := rules.SlotCapacity(slots.MaterialExpandCount, slots.WalletExpandCount)
	if used >= capacity {
		return fmt.Errorf("active listings at capacity (%d)", capacity)
	}
	return nil
}

func (s *PostgresStore) countActiveListings(ctx context.Context, tx pgx.Tx, accountID int64) (int, error) {
	var used int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(*)::int
		FROM marketplace_listings
		WHERE seller_account_id = $1 AND status IN ('LISTED', 'LOCKED')
	`, accountID).Scan(&used)
	return used, err
}

func (s *PostgresStore) ensureMarketSlots(ctx context.Context, tx pgx.Tx, accountID int64) (MarketplaceSlots, error) {
	var slots MarketplaceSlots
	err := tx.QueryRow(ctx, `
		INSERT INTO account_market_slots (account_id)
		VALUES ($1)
		ON CONFLICT (account_id) DO UPDATE SET account_id = EXCLUDED.account_id
		RETURNING account_id, material_expand_count, wallet_expand_count
	`, accountID).Scan(&slots.AccountID, &slots.MaterialExpandCount, &slots.WalletExpandCount)
	return slots, err
}

func (s *PostgresStore) decorateSlots(accountID int64, slots MarketplaceSlots, rules MarketplaceRules) MarketplaceSlots {
	slots.AccountID = accountID
	slots.BaseSlots = rules.BaseListingSlots
	slots.Capacity = rules.SlotCapacity(slots.MaterialExpandCount, slots.WalletExpandCount)
	slots.Available = slots.Capacity - slots.Used
	if slots.Available < 0 {
		slots.Available = 0
	}
	return slots
}

func (s *PostgresStore) assertDailyListingCreateLimit(ctx context.Context, tx pgx.Tx, accountID int64, rules MarketplaceRules) error {
	start, end, err := dayRangeInLocation(time.Now().UTC(), rules.DailyLimitTimezone)
	if err != nil {
		return err
	}
	var count int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM marketplace_listings
		WHERE seller_account_id = $1 AND created_at >= $2 AND created_at < $3
	`, accountID, start, end).Scan(&count)
	if err != nil {
		return err
	}
	if count >= rules.MaxListingsCreatedPerDay {
		return errors.New("daily listing create limit reached")
	}
	return nil
}

func (s *PostgresStore) assertDailyCancelLimit(ctx context.Context, tx pgx.Tx, accountID int64, rules MarketplaceRules) error {
	start, end, err := dayRangeInLocation(time.Now().UTC(), rules.DailyLimitTimezone)
	if err != nil {
		return err
	}
	var count int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM marketplace_listings
		WHERE seller_account_id = $1 AND cancelled_at IS NOT NULL AND cancelled_at >= $2 AND cancelled_at < $3
	`, accountID, start, end).Scan(&count)
	if err != nil {
		return err
	}
	if count >= rules.MaxCancelsPerDay {
		return errors.New("daily cancel limit reached")
	}
	return nil
}

func (s *PostgresStore) assertDailyPurchaseLimit(ctx context.Context, tx pgx.Tx, accountID int64, rules MarketplaceRules) error {
	start, end, err := dayRangeInLocation(time.Now().UTC(), rules.DailyLimitTimezone)
	if err != nil {
		return err
	}
	var count int
	err = tx.QueryRow(ctx, `
		SELECT COUNT(*)::int FROM marketplace_orders
		WHERE buyer_account_id = $1 AND created_at >= $2 AND created_at < $3
	`, accountID, start, end).Scan(&count)
	if err != nil {
		return err
	}
	if count >= rules.MaxPurchasesPerDay {
		return errors.New("daily purchase limit reached")
	}
	return nil
}

func (s *PostgresStore) assertPurchaseCooldown(ctx context.Context, tx pgx.Tx, accountID int64, rules MarketplaceRules) error {
	if rules.PurchaseCooldownSeconds <= 0 {
		return nil
	}
	var latest pgtype.Timestamptz
	err := tx.QueryRow(ctx, `
		SELECT MAX(created_at) FROM marketplace_orders WHERE buyer_account_id = $1
	`, accountID).Scan(&latest)
	if err != nil {
		return err
	}
	if latest.Valid {
		elapsed := time.Now().UTC().Sub(latest.Time.UTC()).Seconds()
		if elapsed < float64(rules.PurchaseCooldownSeconds) {
			return errors.New("purchase cooldown active")
		}
	}
	return nil
}
