package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	PaymentPurposeWalletExpand   = "MARKET_SLOT_WALLET_EXPAND"
	PaymentPurposeBagExpand      = "BAG_EXPAND"
	PaymentPurposeTradingLicense = "TRADING_LICENSE"
)

type GrowthPaymentRules struct {
	DepositReceiverWallet           string
	BagSlots                        int
	BagExpandSlots                  int
	BagExpandMaxTimes               int
	BagExpandPriceToken             int64
	BagExpandPaymentTimeoutSec      int
	TradingLicensePriceToken        int64
	TradingLicensePaymentTimeoutSec int
}

func (r GrowthPaymentRules) withDefaults() GrowthPaymentRules {
	if r.BagSlots <= 0 {
		r.BagSlots = 25
	}
	if r.BagExpandSlots <= 0 {
		r.BagExpandSlots = 5
	}
	if r.BagExpandMaxTimes <= 0 {
		r.BagExpandMaxTimes = 10
	}
	if r.BagExpandPriceToken <= 0 {
		r.BagExpandPriceToken = 50
	}
	if r.BagExpandPaymentTimeoutSec <= 0 {
		r.BagExpandPaymentTimeoutSec = 600
	}
	if r.TradingLicensePriceToken <= 0 {
		r.TradingLicensePriceToken = 100
	}
	if r.TradingLicensePaymentTimeoutSec <= 0 {
		r.TradingLicensePaymentTimeoutSec = 600
	}
	return r
}

func (r GrowthPaymentRules) EffectiveBagSlots(expandCount int) int {
	r = r.withDefaults()
	if expandCount < 0 {
		expandCount = 0
	}
	return r.BagSlots + expandCount*r.BagExpandSlots
}

type GrowthPaymentRequest struct {
	OpID        string
	AccountID   int64
	CharacterID int64
	Rules       GrowthPaymentRules
}

type GrowthPaymentResult struct {
	Order          PaymentOrder `json:"order"`
	BagExpandCount int          `json:"bagExpandCount,omitempty"`
	BagSlots       int          `json:"bagSlots,omitempty"`
	HasLicense     bool         `json:"hasLicense,omitempty"`
}

func supportedPaymentPurpose(purpose string) bool {
	switch purpose {
	case PaymentPurposeWalletExpand, PaymentPurposeBagExpand, PaymentPurposeTradingLicense:
		return true
	default:
		return false
	}
}

func (s *PostgresStore) CreateBagExpandPayment(req GrowthPaymentRequest) (GrowthPaymentResult, error) {
	rules := req.Rules.withDefaults()
	return runIdempotentAction(s, "bag_expand_create", req.OpID, req.AccountID, req.CharacterID, func(ctx context.Context, tx pgx.Tx) (GrowthPaymentResult, error) {
		if strings.TrimSpace(rules.DepositReceiverWallet) == "" {
			return GrowthPaymentResult{}, errors.New("deposit receiver wallet is not configured")
		}
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return GrowthPaymentResult{}, err
		}
		var expandCount int
		if err := tx.QueryRow(ctx, `
			SELECT bag_expand_count FROM characters
			WHERE id = $1 AND account_id = $2 AND is_deleted = FALSE
			FOR UPDATE
		`, req.CharacterID, req.AccountID).Scan(&expandCount); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return GrowthPaymentResult{}, ErrNotFound
			}
			return GrowthPaymentResult{}, err
		}
		if expandCount >= rules.BagExpandMaxTimes {
			return GrowthPaymentResult{}, errors.New("bag expands exhausted")
		}
		var openCount int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*)::int FROM economy_payment_orders
			WHERE account_id = $1 AND character_id = $2 AND purpose = $3
				AND status IN ('PENDING_PAYMENT', 'SUBMITTED', 'CONFIRMED')
		`, req.AccountID, req.CharacterID, PaymentPurposeBagExpand).Scan(&openCount); err != nil {
			return GrowthPaymentResult{}, err
		}
		if openCount > 0 {
			return GrowthPaymentResult{}, errors.New("open bag expand payment already exists")
		}
		expires := time.Now().UTC().Add(time.Duration(rules.BagExpandPaymentTimeoutSec) * time.Second)
		order, err := s.insertPaymentOrder(ctx, tx, req.AccountID, req.CharacterID, PaymentPurposeBagExpand,
			rules.BagExpandPriceToken, rules.DepositReceiverWallet, expires, req.OpID,
			fmt.Sprintf(`{"slotsPerExpand":%d,"opId":%q}`, rules.BagExpandSlots, req.OpID))
		if err != nil {
			return GrowthPaymentResult{}, err
		}
		return GrowthPaymentResult{
			Order:          order,
			BagExpandCount: expandCount,
			BagSlots:       rules.EffectiveBagSlots(expandCount),
		}, nil
	})
}

func (s *PostgresStore) CreateTradingLicensePayment(req GrowthPaymentRequest) (GrowthPaymentResult, error) {
	rules := req.Rules.withDefaults()
	return runIdempotentAction(s, "trading_license_create", req.OpID, req.AccountID, 0, func(ctx context.Context, tx pgx.Tx) (GrowthPaymentResult, error) {
		if strings.TrimSpace(rules.DepositReceiverWallet) == "" {
			return GrowthPaymentResult{}, errors.New("deposit receiver wallet is not configured")
		}
		var hasLicense bool
		if err := tx.QueryRow(ctx, `
			SELECT has_trading_license FROM accounts WHERE id = $1 FOR UPDATE
		`, req.AccountID).Scan(&hasLicense); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return GrowthPaymentResult{}, ErrNotFound
			}
			return GrowthPaymentResult{}, err
		}
		if hasLicense {
			return GrowthPaymentResult{}, errors.New("trading license already owned")
		}
		var openCount int
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*)::int FROM economy_payment_orders
			WHERE account_id = $1 AND purpose = $2
				AND status IN ('PENDING_PAYMENT', 'SUBMITTED', 'CONFIRMED')
		`, req.AccountID, PaymentPurposeTradingLicense).Scan(&openCount); err != nil {
			return GrowthPaymentResult{}, err
		}
		if openCount > 0 {
			return GrowthPaymentResult{}, errors.New("open trading license payment already exists")
		}
		expires := time.Now().UTC().Add(time.Duration(rules.TradingLicensePaymentTimeoutSec) * time.Second)
		charID := req.CharacterID
		order, err := s.insertPaymentOrder(ctx, tx, req.AccountID, charID, PaymentPurposeTradingLicense,
			rules.TradingLicensePriceToken, rules.DepositReceiverWallet, expires, req.OpID,
			fmt.Sprintf(`{"opId":%q}`, req.OpID))
		if err != nil {
			return GrowthPaymentResult{}, err
		}
		return GrowthPaymentResult{Order: order, HasLicense: false}, nil
	})
}

func (s *PostgresStore) insertPaymentOrder(
	ctx context.Context,
	tx pgx.Tx,
	accountID, characterID int64,
	purpose string,
	amount int64,
	receiver string,
	expires time.Time,
	opID, payloadJSON string,
) (PaymentOrder, error) {
	var order PaymentOrder
	var charArg any
	if characterID > 0 {
		charArg = characterID
	}
	err := tx.QueryRow(ctx, `
		INSERT INTO economy_payment_orders (
			account_id, character_id, purpose, pay_asset, amount, receiver_wallet, status, payload, expires_at
		) VALUES (
			$1, $2, $3, 'AEB', $4, $5, 'PENDING_PAYMENT', $6::jsonb, $7
		)
		RETURNING id::text, account_id, COALESCE(character_id, 0), purpose, pay_asset, amount::bigint,
			COALESCE(receiver_wallet, ''), status, created_at, expires_at
	`, accountID, charArg, purpose, amount, receiver, payloadJSON, expires).Scan(
		&order.ID, &order.AccountID, &order.CharacterID, &order.Purpose, &order.PayAsset, &order.Amount,
		&order.ReceiverWallet, &order.Status, &order.CreatedAt, &order.ExpiresAt,
	)
	_ = opID
	return order, err
}

func (s *PostgresStore) assertHasTradingLicense(ctx context.Context, tx pgx.Tx, accountID int64) error {
	var has bool
	if err := tx.QueryRow(ctx, `
		SELECT has_trading_license FROM accounts WHERE id = $1
	`, accountID).Scan(&has); err != nil {
		return err
	}
	if !has {
		return errors.New("trading license required")
	}
	return nil
}
