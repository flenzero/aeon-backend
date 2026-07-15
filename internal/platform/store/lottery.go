package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type LotteryPaymentRequest struct {
	OpID           string
	AccountID      int64
	CharacterID    int64
	Amount         int64
	ReceiverWallet string
	Count          int
	ConfigSnapshot any
	RewardPlan     DungeonRewardPlan
}

type LotteryPaymentResult struct {
	Order   PaymentOrder      `json:"order"`
	Rewards DungeonRewardPlan `json:"rewards"`
}

type lotteryPaymentPayload struct {
	Count          int               `json:"count"`
	ConfigSnapshot any               `json:"configSnapshot"`
	RewardPlan     DungeonRewardPlan `json:"rewardPlan"`
}

func (s *PostgresStore) CreateLotteryPayment(req LotteryPaymentRequest) (LotteryPaymentResult, error) {
	if req.Amount <= 0 || req.Count < 1 || strings.TrimSpace(req.ReceiverWallet) == "" {
		return LotteryPaymentResult{}, errors.New("invalid lottery payment request")
	}
	return runIdempotentAction(s, "lottery_payment_create", req.OpID, req.AccountID, req.CharacterID, req, func(ctx context.Context, tx pgx.Tx) (LotteryPaymentResult, error) {
		if err := s.lockCharacter(ctx, tx, req.AccountID, req.CharacterID); err != nil {
			return LotteryPaymentResult{}, err
		}
		payload, err := json.Marshal(lotteryPaymentPayload{Count: req.Count, ConfigSnapshot: req.ConfigSnapshot, RewardPlan: req.RewardPlan})
		if err != nil {
			return LotteryPaymentResult{}, err
		}
		order, err := s.insertPaymentOrder(ctx, tx, req.AccountID, req.CharacterID, PaymentPurposeLotteryDraw, req.Amount, req.ReceiverWallet, time.Now().UTC().Add(10*time.Minute), req.OpID, string(payload))
		if err != nil {
			return LotteryPaymentResult{}, err
		}
		return LotteryPaymentResult{Order: order, Rewards: req.RewardPlan}, nil
	})
}

func (s *PostgresStore) fulfillLotteryPaymentTx(ctx context.Context, tx pgx.Tx, order PaymentOrder) error {
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT payload FROM economy_payment_orders WHERE id = $1::uuid`, order.ID).Scan(&raw); err != nil {
		return err
	}
	var payload lotteryPaymentPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	if _, err := s.applyTrayRewards(ctx, tx, order.AccountID, order.CharacterID, order.ID, order.ID, "lottery", "lottery_reward", payload.RewardPlan); err != nil {
		return err
	}
	_, err := s.publishRareRewardAnnouncementsTx(ctx, tx, payload.RewardPlan, rareAnnouncementContext{
		AccountID: order.AccountID, CharacterID: order.CharacterID, Source: "抽奖",
		RefType: "lottery_order", RefID: order.ID, AnnouncementOn: true,
	})
	return err
}
