package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type TokenSpendBreakdown struct {
	Locked         int64                `json:"locked"`
	Withdrawable   int64                `json:"withdrawable"`
	External       int64                `json:"external"`
	SpendSource    string               `json:"spendSource"`
	LockedSegments []LockedSpendSegment `json:"lockedSegments,omitempty"`
}

type LockedSpendSegment struct {
	Amount   int64     `json:"amount"`
	UnlockAt time.Time `json:"unlockAt"`
}

func bpsAmount(amount, bps int64) int64 {
	if amount <= 0 || bps <= 0 {
		return 0
	}
	return amount * bps / 10000
}

func bpsCeil(amount, bps int64) int64 {
	if amount <= 0 || bps <= 0 {
		return 0
	}
	out := (amount*bps + 9999) / 10000
	if out < 1 {
		return 1
	}
	return out
}

func spendSourceLabel(locked, withdrawable, external int64) string {
	n := 0
	if locked > 0 {
		n++
	}
	if withdrawable > 0 {
		n++
	}
	if external > 0 {
		n++
	}
	if n > 1 {
		return "MIXED"
	}
	if locked > 0 {
		return "LOCKED_TOKEN"
	}
	if withdrawable > 0 {
		return "WITHDRAWABLE_TOKEN"
	}
	if external > 0 {
		return "EXTERNAL_TOKEN"
	}
	return "MIXED"
}

// spendTokenInTx debits locked (FIFO) → withdrawable → external.
func (s *PostgresStore) spendTokenInTx(ctx context.Context, tx pgx.Tx, accountID, amount int64) (TokenSpendBreakdown, error) {
	var zero TokenSpendBreakdown
	if amount <= 0 {
		return zero, errors.New("spend amount must be positive")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO account_tokens (account_id) VALUES ($1) ON CONFLICT (account_id) DO NOTHING`, accountID); err != nil {
		return zero, err
	}
	var lockedBal, withdrawableBal, externalBal int64
	err := tx.QueryRow(ctx, `
		SELECT locked_balance::bigint, withdrawable_balance::bigint, external_balance::bigint
		FROM account_tokens
		WHERE account_id = $1
		FOR UPDATE
	`, accountID).Scan(&lockedBal, &withdrawableBal, &externalBal)
	if err != nil {
		return zero, err
	}
	if lockedBal+withdrawableBal+externalBal < amount {
		return zero, ErrInsufficientBalance
	}

	remaining := amount
	out := TokenSpendBreakdown{}

	if remaining > 0 && lockedBal > 0 {
		take := remaining
		if take > lockedBal {
			take = lockedBal
		}
		segments, err := s.consumeLockedTokenFIFO(ctx, tx, accountID, take)
		if err != nil {
			return zero, err
		}
		out.Locked = take
		out.LockedSegments = segments
		remaining -= take
	}
	if remaining > 0 && withdrawableBal > 0 {
		take := remaining
		if take > withdrawableBal {
			take = withdrawableBal
		}
		out.Withdrawable = take
		remaining -= take
	}
	if remaining > 0 {
		if remaining > externalBal {
			return zero, ErrInsufficientBalance
		}
		out.External = remaining
		remaining = 0
	}
	_ = remaining
	out.SpendSource = spendSourceLabel(out.Locked, out.Withdrawable, out.External)

	tag, err := tx.Exec(ctx, `
		UPDATE account_tokens
		SET locked_balance = locked_balance - $2,
		    withdrawable_balance = withdrawable_balance - $3,
		    external_balance = external_balance - $4,
		    token_balance = token_balance - $5,
		    updated_at = NOW()
		WHERE account_id = $1
			AND locked_balance >= $2
			AND withdrawable_balance >= $3
			AND external_balance >= $4
			AND token_balance >= $5
	`, accountID, out.Locked, out.Withdrawable, out.External, amount)
	if err != nil {
		return zero, err
	}
	if tag.RowsAffected() == 0 {
		return zero, ErrInsufficientBalance
	}
	return out, nil
}

func (s *PostgresStore) consumeLockedTokenFIFO(ctx context.Context, tx pgx.Tx, accountID, amount int64) ([]LockedSpendSegment, error) {
	rows, err := tx.Query(ctx, `
		SELECT id, remaining_amount::bigint, unlock_at
		FROM locked_token_records
		WHERE account_id = $1 AND status = 'LOCKED' AND remaining_amount > 0
		ORDER BY unlock_at, id
		FOR UPDATE
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type lockedRow struct {
		ID       int64
		Rem      int64
		UnlockAt time.Time
	}
	var list []lockedRow
	for rows.Next() {
		var row lockedRow
		if err := rows.Scan(&row.ID, &row.Rem, &row.UnlockAt); err != nil {
			return nil, err
		}
		list = append(list, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	remaining := amount
	segments := make([]LockedSpendSegment, 0, len(list))
	for _, row := range list {
		if remaining <= 0 {
			break
		}
		take := remaining
		if take > row.Rem {
			take = row.Rem
		}
		newRem := row.Rem - take
		if newRem == 0 {
			if _, err := tx.Exec(ctx, `
				UPDATE locked_token_records
				SET remaining_amount = 0, status = 'CONSUMED', settled_at = NOW()
				WHERE id = $1
			`, row.ID); err != nil {
				return nil, err
			}
		} else {
			if _, err := tx.Exec(ctx, `
				UPDATE locked_token_records
				SET remaining_amount = $2
				WHERE id = $1
			`, row.ID, newRem); err != nil {
				return nil, err
			}
		}
		segments = append(segments, LockedSpendSegment{Amount: take, UnlockAt: row.UnlockAt})
		remaining -= take
	}
	if remaining > 0 {
		return nil, fmt.Errorf("%w: locked token records", ErrInsufficientBalance)
	}
	return segments, nil
}

func (s *PostgresStore) insertSystemConsumption(ctx context.Context, tx pgx.Tx, opID string, accountID, characterID int64, spend TokenSpendBreakdown, purpose string, amount, burn, recycle, rewards int64, metadata string) error {
	if metadata == "" {
		metadata = "{}"
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO system_consumptions (
			op_id, account_id, character_id, spend_source, purpose,
			amount_token, burn_amount, recycle_amount, reward_pool_amount, metadata
		) VALUES ($1, $2, NULLIF($3, 0), $4, $5, $6, $7, $8, $9, $10::jsonb)
	`, opID, accountID, characterID, spend.SpendSource, purpose, amount, burn, recycle, rewards, metadata)
	return err
}

func dayRangeInLocation(now time.Time, tzName string) (time.Time, time.Time, error) {
	if tzName == "" {
		tzName = "Asia/Shanghai"
	}
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	local := now.In(loc)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	return start.UTC(), start.Add(24 * time.Hour).UTC(), nil
}
