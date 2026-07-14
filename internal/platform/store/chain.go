package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/flenzero/aeon-backend/internal/chain"
)

type ChainScanConfig struct {
	Network          string
	TokenMint        string
	TokenDecimals    int
	DepositWallet    string
	PayoutWallet     string
	PayoutPrivateKey string
	PayoutMode       string // stub | record | rpc
	ScanLimit        int
	CursorName       string
	LowBalanceRaw    uint64 // pause payouts when hot ATA below this (raw units); 0 = skip
}

func (c ChainScanConfig) withDefaults() ChainScanConfig {
	if strings.TrimSpace(c.Network) == "" {
		c.Network = "solana-devnet"
	}
	if strings.TrimSpace(c.CursorName) == "" {
		c.CursorName = "solana_deposits"
	}
	if c.ScanLimit <= 0 {
		c.ScanLimit = 50
	}
	if strings.TrimSpace(c.PayoutMode) == "" {
		c.PayoutMode = "stub"
	}
	return c
}

type DepositScanResult struct {
	Scanned           int      `json:"scanned"`
	Credited          int      `json:"credited"`
	Ignored           int      `json:"ignored"`
	PaymentsFulfilled int      `json:"paymentsFulfilled"`
	CursorSlot        uint64   `json:"cursorSlot"`
	Signatures        []string `json:"signatures,omitempty"`
	Disabled          bool     `json:"disabled,omitempty"`
	Message           string   `json:"message,omitempty"`
}

type PayoutJobResult struct {
	Processed int     `json:"processed"`
	Submitted int     `json:"submitted"`
	Confirmed int     `json:"confirmed"`
	Failed    int     `json:"failed"`
	Disabled  bool    `json:"disabled,omitempty"`
	Message   string  `json:"message,omitempty"`
	IDs       []int64 `json:"ids,omitempty"`
}

type PaymentOrder struct {
	ID             string     `json:"id"`
	AccountID      int64      `json:"accountId"`
	CharacterID    int64      `json:"characterId,omitempty"`
	Purpose        string     `json:"purpose"`
	PayAsset       string     `json:"payAsset"`
	Amount         int64      `json:"amount"`
	ReceiverWallet string     `json:"receiverWallet,omitempty"`
	Status         string     `json:"status"`
	TxSignature    string     `json:"txSignature,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
	ExpiresAt      time.Time  `json:"expiresAt"`
	SubmittedAt    *time.Time `json:"submittedAt,omitempty"`
	ConfirmedAt    *time.Time `json:"confirmedAt,omitempty"`
	FulfilledAt    *time.Time `json:"fulfilledAt,omitempty"`
}

// ScanAndCreditDeposits pulls inbound SPL transfers to the deposit wallet and credits external_balance.
func (s *PostgresStore) ScanAndCreditDeposits(ctx context.Context, rpc chain.RPC, cfg ChainScanConfig) (DepositScanResult, error) {
	cfg = cfg.withDefaults()
	out := DepositScanResult{}
	if rpc == nil || strings.TrimSpace(cfg.DepositWallet) == "" || strings.TrimSpace(cfg.TokenMint) == "" {
		out.Disabled = true
		out.Message = "solana deposit scan requires rpc, deposit wallet and token mint"
		return out, nil
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return out, err
	}
	defer rollback(ctx, tx)

	cursorSlot, cursorSignature, err := s.ensureChainCursor(ctx, tx, cfg.CursorName, cfg.Network)
	if err != nil {
		return out, err
	}

	var maxSlot = cursorSlot
	headSignature := ""
	before := ""
	stop := false
	for !stop {
		sigs, err := rpc.GetSignaturesForAddress(ctx, cfg.DepositWallet, before, cfg.ScanLimit)
		if err != nil {
			return out, err
		}
		if len(sigs) == 0 {
			break
		}
		if headSignature == "" {
			headSignature = sigs[0].Signature
		}
		for _, sig := range sigs {
			if cursorSignature != "" && sig.Signature == cursorSignature {
				stop = true
				break
			}
			out.Scanned++
			if err := s.processDepositSignature(ctx, tx, rpc, cfg, sig, &out, &maxSlot); err != nil {
				return out, err
			}
		}
		if stop || len(sigs) < cfg.ScanLimit {
			break
		}
		before = sigs[len(sigs)-1].Signature
	}
	latest, err := rpc.GetSlot(ctx)
	if err == nil && latest > maxSlot {
		maxSlot = latest
	}
	if err := s.updateChainCursor(ctx, tx, cfg.CursorName, cfg.Network, maxSlot, latest, headSignature); err != nil {
		return out, err
	}
	if err := tx.Commit(ctx); err != nil {
		return out, err
	}
	out.CursorSlot = maxSlot
	return out, nil
}

func (s *PostgresStore) processDepositSignature(ctx context.Context, tx pgx.Tx, rpc chain.RPC, cfg ChainScanConfig, sig chain.SignatureInfo, out *DepositScanResult, maxSlot *uint64) error {
	if sig.Err != nil {
		out.Ignored++
		return nil
	}
	detail, err := rpc.GetTransaction(ctx, sig.Signature)
	if err != nil {
		return err
	}
	credits := chain.InboundTokenCredits(detail, cfg.DepositWallet, cfg.TokenMint)
	if len(credits) == 0 {
		out.Ignored++
		if sig.Slot > *maxSlot {
			*maxSlot = sig.Slot
		}
		return nil
	}
	for _, credit := range credits {
		gameAmount := chain.RawToGameAmount(credit.AmountRaw, cfg.TokenDecimals)
		if gameAmount <= 0 {
			out.Ignored++
			continue
		}
		wallet := strings.TrimSpace(credit.FromOwner)
		if wallet == "" {
			out.Ignored++
			continue
		}
		matched, err := s.matchPaymentOrderFromDeposit(ctx, tx, wallet, gameAmount, credit.Signature, cfg.TokenMint, int64(credit.Slot))
		if err != nil {
			return err
		}
		if matched {
			out.PaymentsFulfilled++
			out.Signatures = append(out.Signatures, credit.Signature)
			if credit.Slot > *maxSlot {
				*maxSlot = credit.Slot
			}
			continue
		}
		credited, err := s.creditSolanaDeposit(ctx, tx, wallet, cfg.TokenMint, gameAmount, credit.Signature, int64(credit.Slot))
		if err != nil {
			return err
		}
		if credited {
			out.Credited++
			out.Signatures = append(out.Signatures, credit.Signature)
		} else {
			out.Ignored++
		}
		if credit.Slot > *maxSlot {
			*maxSlot = credit.Slot
		}
	}
	if sig.Slot > *maxSlot {
		*maxSlot = sig.Slot
	}
	return nil
}

// matchPaymentOrderFromDeposit fulfills an open on-chain payment order instead of crediting balance.
// Used when inbound AEB is a marketplace wallet-expand (or similar) payment, not a player deposit.
func (s *PostgresStore) matchPaymentOrderFromDeposit(ctx context.Context, tx pgx.Tx, wallet string, amount int64, signature, mint string, slot int64) (bool, error) {
	var accountID int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM accounts WHERE solana_wallet_address = $1 AND status = 'ACTIVE'
	`, wallet).Scan(&accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	var orderID string
	err = tx.QueryRow(ctx, `
		SELECT id::text
		FROM economy_payment_orders
		WHERE account_id = $1
		  AND status IN ('PENDING_PAYMENT', 'SUBMITTED')
		  AND amount = $2
		  AND (tx_signature IS NULL OR tx_signature = $3)
		  AND expires_at > NOW()
		ORDER BY
		  CASE WHEN tx_signature = $3 THEN 0 ELSE 1 END,
		  created_at
		LIMIT 1
		FOR UPDATE SKIP LOCKED
	`, accountID, amount, signature).Scan(&orderID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	now := time.Now().UTC()
	tag, err := tx.Exec(ctx, `
		INSERT INTO solana_deposits (account_id, wallet, token_mint, amount, signature, slot, status, credited_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'PAYMENT_MATCHED', $7)
		ON CONFLICT (signature) DO NOTHING
	`, accountID, wallet, mint, amount, signature, slot, now)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `
		UPDATE economy_payment_orders
		SET status = 'SUBMITTED', tx_signature = $2, submitted_at = COALESCE(submitted_at, $3), confirmed_at = COALESCE(confirmed_at, $3)
		WHERE id = $1::uuid
	`, orderID, signature, now); err != nil {
		return false, err
	}
	if _, err := s.fulfillPaymentOrderTx(ctx, tx, orderID, "deposit_scan_match"); err != nil {
		return false, err
	}
	return true, nil
}

func (s *PostgresStore) creditSolanaDeposit(ctx context.Context, tx pgx.Tx, wallet, mint string, amount int64, signature string, slot int64) (bool, error) {
	var accountID *int64
	var id int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM accounts WHERE solana_wallet_address = $1 AND status = 'ACTIVE'
	`, wallet).Scan(&id)
	if err == nil {
		accountID = &id
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}

	status := "IGNORED"
	var creditedAt *time.Time
	if accountID != nil {
		status = "CREDITED"
		now := time.Now().UTC()
		creditedAt = &now
	}
	tag, err := tx.Exec(ctx, `
		INSERT INTO solana_deposits (account_id, wallet, token_mint, amount, signature, slot, status, credited_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (signature) DO NOTHING
	`, accountID, wallet, mint, amount, signature, slot, status, creditedAt)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 || accountID == nil {
		return false, nil
	}
	if _, err := tx.Exec(ctx, `INSERT INTO account_tokens (account_id) VALUES ($1) ON CONFLICT (account_id) DO NOTHING`, *accountID); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE account_tokens
		SET external_balance = external_balance + $2,
		    token_balance = token_balance + $2,
		    updated_at = NOW()
		WHERE account_id = $1
	`, *accountID, amount); err != nil {
		return false, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO economy_ledger (account_id, kind, currency, amount, ref_id, reason)
		VALUES ($1, 'SOLANA_DEPOSIT_CREDITED', 'AEB', $2, $3, $4)
	`, *accountID, amount, signature, mint); err != nil {
		return false, err
	}
	return true, nil
}

func (s *PostgresStore) ensureChainCursor(ctx context.Context, tx pgx.Tx, name, network string) (uint64, string, error) {
	var slot int64
	var signature string
	err := tx.QueryRow(ctx, `
		INSERT INTO chain_cursors (name, network, cursor_slot)
		VALUES ($1, $2, 0)
		ON CONFLICT (name) DO UPDATE SET network = EXCLUDED.network
		RETURNING cursor_slot, COALESCE(cursor_signature, '')
	`, name, network).Scan(&slot, &signature)
	return uint64(slot), signature, err
}

func (s *PostgresStore) updateChainCursor(ctx context.Context, tx pgx.Tx, name, network string, cursorSlot, latest uint64, signature string) error {
	lag := int64(0)
	if latest > cursorSlot {
		lag = int64(latest - cursorSlot)
	}
	_, err := tx.Exec(ctx, `
		UPDATE chain_cursors
		SET network = $2, cursor_slot = $3, lag_slots = $4, cursor_signature = NULLIF($5, ''), status = 'OK', updated_at = NOW()
		WHERE name = $1
	`, name, network, int64(cursorSlot), lag, signature)
	return err
}

// PrepareWithdrawalPayouts turns limit-approved QUEUED withdrawals into solana_payouts.
// stub mode immediately marks SUBMITTED+CONFIRMED; record/rpc leave CREATED for submit/confirm jobs.
func (s *PostgresStore) ProcessAutoWithdrawals(now time.Time, singleMax, userDailyMax, globalHourlyMax, globalDailyMax int64, limit int) []Withdrawal {
	return s.ProcessAutoWithdrawalsWithChain(now, singleMax, userDailyMax, globalHourlyMax, globalDailyMax, limit, ChainScanConfig{PayoutMode: "stub"})
}

func (s *PostgresStore) ProcessAutoWithdrawalsWithChain(now time.Time, singleMax, userDailyMax, globalHourlyMax, globalDailyMax int64, limit int, cfg ChainScanConfig) []Withdrawal {
	cfg = cfg.withDefaults()
	ctx := context.Background()
	if limit <= 0 {
		limit = 20
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	must(err, "begin process withdrawals")
	defer rollback(ctx, tx)
	userDaily := map[int64]int64{}
	var globalHourly int64
	var globalDaily int64
	rows, err := tx.Query(ctx, `
		SELECT account_id, amount::bigint, processed_at
		FROM withdrawals
		WHERE status IN ('PAYOUT_CREATED', 'SUBMITTED', 'CONFIRMED') AND processed_at IS NOT NULL
	`)
	must(err, "query withdrawal budgets")
	for rows.Next() {
		var accountID int64
		var amount int64
		var processedAt time.Time
		must(rows.Scan(&accountID, &amount, &processedAt), "scan withdrawal budget")
		if sameDay(processedAt, now) {
			userDaily[accountID] += amount
			globalDaily += amount
		}
		if sameHour(processedAt, now) {
			globalHourly += amount
		}
	}
	rows.Close()
	must(rows.Err(), "iterate withdrawal budgets")

	rows, err = tx.Query(ctx, `
		SELECT id, account_id, wallet, amount::bigint, status, COALESCE(reason, ''), COALESCE(tx_signature, ''), created_at
		FROM withdrawals
		WHERE status = 'QUEUED'
		ORDER BY created_at
		LIMIT $1
		FOR UPDATE SKIP LOCKED
	`, limit)
	must(err, "query queued withdrawals")
	var out []Withdrawal
	for rows.Next() {
		var row Withdrawal
		must(rows.Scan(&row.ID, &row.AccountID, &row.Wallet, &row.Amount, &row.Status, &row.Reason, &row.TxHash, &row.CreatedAt), "scan queued withdrawal")
		out = append(out, row)
	}
	rows.Close()
	must(rows.Err(), "iterate queued withdrawals")

	mint := strings.TrimSpace(cfg.TokenMint)
	if mint == "" {
		mint = "AEB"
	}
	payoutWallet := strings.TrimSpace(cfg.PayoutWallet)
	if payoutWallet == "" {
		payoutWallet = "payout_wallet_unconfigured"
	}

	for i := range out {
		row := &out[i]
		switch {
		case row.Amount > singleMax:
			row.Status = "MANUAL_REVIEW"
			row.Reason = "single limit exceeded"
			_, err = tx.Exec(ctx, `UPDATE withdrawals SET status = $2, reason = $3 WHERE id = $1`, row.ID, row.Status, row.Reason)
		case userDaily[row.AccountID]+row.Amount > userDailyMax:
			row.Status = "MANUAL_REVIEW"
			row.Reason = "user daily limit exceeded"
			_, err = tx.Exec(ctx, `UPDATE withdrawals SET status = $2, reason = $3 WHERE id = $1`, row.ID, row.Status, row.Reason)
		case globalHourly+row.Amount > globalHourlyMax:
			row.Reason = "global hourly limit delayed"
			_, err = tx.Exec(ctx, `UPDATE withdrawals SET reason = $2 WHERE id = $1`, row.ID, row.Reason)
		case globalDaily+row.Amount > globalDailyMax:
			row.Status = "MANUAL_REVIEW"
			row.Reason = "global daily limit exceeded"
			_, err = tx.Exec(ctx, `UPDATE withdrawals SET status = $2, reason = $3 WHERE id = $1`, row.ID, row.Status, row.Reason)
		default:
			var payoutID int64
			err = tx.QueryRow(ctx, `
				INSERT INTO solana_payouts (withdrawal_id, wallet, token_mint, amount, status)
				VALUES ($1, $2, $3, $4, 'CREATED')
				RETURNING id
			`, row.ID, row.Wallet, mint, row.Amount).Scan(&payoutID)
			if err != nil {
				must(err, "insert solana payout")
			}
			userDaily[row.AccountID] += row.Amount
			globalHourly += row.Amount
			globalDaily += row.Amount

			if cfg.PayoutMode == "stub" {
				sig := fmt.Sprintf("stub_tx_%d_%s", row.ID, now.UTC().Format("20060102150405"))
				row.Status = "CONFIRMED"
				row.TxHash = sig
				row.ProcessedAt = now
				_, err = tx.Exec(ctx, `
					UPDATE solana_payouts
					SET status = 'CONFIRMED', signature = $2, submitted_at = $3, confirmed_at = $3, attempt_count = attempt_count + 1
					WHERE id = $1
				`, payoutID, sig, now)
				if err == nil {
					_, err = tx.Exec(ctx, `
						UPDATE withdrawals
						SET status = 'CONFIRMED', tx_signature = $2, processed_at = $3, confirmed_at = $3
						WHERE id = $1
					`, row.ID, sig, now)
				}
				if err == nil {
					_, err = tx.Exec(ctx, `
						INSERT INTO economy_ledger (account_id, kind, currency, amount, ref_id, reason)
						VALUES ($1, 'WITHDRAWAL_CONFIRMED', 'AEB', $2, $3, 'stub')
					`, row.AccountID, row.Amount, sig)
				}
			} else {
				row.Status = "PAYOUT_CREATED"
				row.Reason = "awaiting_chain_payout"
				row.ProcessedAt = now
				_, err = tx.Exec(ctx, `UPDATE withdrawals SET status = $2, reason = $3, processed_at = $4 WHERE id = $1`, row.ID, row.Status, row.Reason, now)
			}
			_ = payoutWallet
		}
		must(err, "update withdrawal")
	}
	must(tx.Commit(ctx), "commit process withdrawals")
	return out
}

func (s *PostgresStore) SubmitSolanaPayouts(ctx context.Context, rpc chain.RPC, cfg ChainScanConfig, limit int) (PayoutJobResult, error) {
	cfg = cfg.withDefaults()
	out := PayoutJobResult{}
	if cfg.PayoutMode == "stub" {
		out.Disabled = true
		out.Message = "payout mode stub auto-confirms during processWithdrawals"
		return out, nil
	}
	if limit <= 0 {
		limit = 20
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return out, err
	}
	defer rollback(ctx, tx)

	paused, err := s.payoutsPaused(ctx, tx, cfg)
	if err != nil {
		return out, err
	}
	if paused {
		out.Message = "payouts paused (hot wallet)"
		_ = tx.Commit(ctx)
		return out, nil
	}

	if cfg.PayoutMode == "rpc" {
		if rpc == nil {
			out.Message = "rpc client required for payout mode rpc"
			_ = tx.Commit(ctx)
			return out, nil
		}
		if err := s.ensureHotWalletFunded(ctx, tx, rpc, cfg); err != nil {
			out.Message = err.Error()
			_ = tx.Commit(ctx)
			return out, nil
		}
	}

	rows, err := tx.Query(ctx, `
		SELECT p.id, p.withdrawal_id, p.wallet, p.amount::bigint, w.account_id
		FROM solana_payouts p
		JOIN withdrawals w ON w.id = p.withdrawal_id
		WHERE p.status = 'CREATED'
		ORDER BY p.created_at
		LIMIT $1
		FOR UPDATE OF p SKIP LOCKED
	`, limit)
	if err != nil {
		return out, err
	}
	type payoutRow struct {
		ID, WithdrawalID, AccountID int64
		Wallet                      string
		Amount                      int64
	}
	var list []payoutRow
	for rows.Next() {
		var row payoutRow
		if err := rows.Scan(&row.ID, &row.WithdrawalID, &row.Wallet, &row.Amount, &row.AccountID); err != nil {
			rows.Close()
			return out, err
		}
		list = append(list, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	sender := chain.PayoutSender(chain.SPLPayoutSender{})
	now := time.Now().UTC()
	for _, row := range list {
		out.Processed++
		sig := ""
		if cfg.PayoutMode == "rpc" {
			sent, sendErr := sender.SendSPLTransfer(ctx, rpc, chain.PayoutRequest{
				RecipientWallet: row.Wallet,
				AmountGame:      row.Amount,
				Decimals:        cfg.TokenDecimals,
				Mint:            cfg.TokenMint,
				PayerPrivateKey: cfg.PayoutPrivateKey,
			})
			if sendErr != nil {
				out.Failed++
				errMsg := sendErr.Error()
				_, _ = tx.Exec(ctx, `
					UPDATE solana_payouts
					SET attempt_count = attempt_count + 1, last_error = $2
					WHERE id = $1
				`, row.ID, errMsg)
				if errors.Is(sendErr, chain.ErrRecipientATAMissing) {
					_, _ = tx.Exec(ctx, `
						UPDATE solana_payouts SET status = 'FAILED', last_error = $2 WHERE id = $1
					`, row.ID, errMsg)
					_, _ = tx.Exec(ctx, `
						UPDATE withdrawals SET status = 'FAILED', reason = $2 WHERE id = $1
					`, row.WithdrawalID, "recipient ATA missing; player must hold AEB (or trading license path)")
					continue
				}
				if errors.Is(sendErr, chain.ErrPayoutKeyMissing) || errors.Is(sendErr, chain.ErrPayoutConfig) {
					_ = s.setPayoutsPaused(ctx, tx, cfg, true, errMsg)
					out.Message = errMsg
					break
				}
				continue
			}
			sig = sent
		} else {
			sig = fmt.Sprintf("record_tx_%d_%d", row.ID, now.UnixNano())
		}
		if _, err := tx.Exec(ctx, `
			UPDATE solana_payouts
			SET status = 'SUBMITTED', signature = $2, submitted_at = $3, attempt_count = attempt_count + 1, last_error = NULL
			WHERE id = $1
		`, row.ID, sig, now); err != nil {
			return out, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE withdrawals
			SET status = 'SUBMITTED', tx_signature = $2, processed_at = $3, reason = NULL
			WHERE id = $1
		`, row.WithdrawalID, sig, now); err != nil {
			return out, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO economy_ledger (account_id, kind, currency, amount, ref_id, reason)
			VALUES ($1, 'WITHDRAWAL_SUBMITTED', 'AEB', $2, $3, $4)
		`, row.AccountID, row.Amount, sig, cfg.PayoutMode); err != nil {
			return out, err
		}
		out.Submitted++
		out.IDs = append(out.IDs, row.ID)
	}
	if err := tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}

func (s *PostgresStore) ConfirmSolanaPayouts(ctx context.Context, rpc chain.RPC, cfg ChainScanConfig, limit int) (PayoutJobResult, error) {
	cfg = cfg.withDefaults()
	out := PayoutJobResult{}
	if limit <= 0 {
		limit = 50
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return out, err
	}
	defer rollback(ctx, tx)

	rows, err := tx.Query(ctx, `
		SELECT p.id, p.withdrawal_id, p.signature, p.amount::bigint, w.account_id
		FROM solana_payouts p
		JOIN withdrawals w ON w.id = p.withdrawal_id
		WHERE p.status = 'SUBMITTED' AND p.signature IS NOT NULL
		ORDER BY p.submitted_at NULLS FIRST, p.id
		LIMIT $1
		FOR UPDATE OF p SKIP LOCKED
	`, limit)
	if err != nil {
		return out, err
	}
	type payoutRow struct {
		ID, WithdrawalID, AccountID int64
		Signature                   string
		Amount                      int64
	}
	var list []payoutRow
	for rows.Next() {
		var row payoutRow
		if err := rows.Scan(&row.ID, &row.WithdrawalID, &row.Signature, &row.Amount, &row.AccountID); err != nil {
			rows.Close()
			return out, err
		}
		list = append(list, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	if len(list) == 0 {
		_ = tx.Commit(ctx)
		return out, nil
	}

	sigs := make([]string, len(list))
	for i, row := range list {
		sigs[i] = row.Signature
	}
	statuses := make([]chain.SignatureStatus, len(list))
	if rpc != nil {
		statuses, err = rpc.GetSignatureStatuses(ctx, sigs)
		if err != nil {
			return out, err
		}
	} else {
		for i, sig := range sigs {
			statuses[i] = chain.SignatureStatus{Signature: sig, ConfirmationStatus: "finalized"}
		}
	}

	now := time.Now().UTC()
	for i, row := range list {
		out.Processed++
		st := statuses[i]
		if st.Err != nil {
			out.Failed++
			_, _ = tx.Exec(ctx, `
				UPDATE solana_payouts SET status = 'FAILED', last_error = $2 WHERE id = $1
			`, row.ID, fmt.Sprintf("%v", st.Err))
			_, _ = tx.Exec(ctx, `
				UPDATE withdrawals SET status = 'FAILED', reason = $2 WHERE id = $1
			`, row.WithdrawalID, "payout signature failed")
			continue
		}
		confirmed := st.ConfirmationStatus == "confirmed" || st.ConfirmationStatus == "finalized" || rpc == nil
		if !confirmed {
			continue
		}
		if _, err := tx.Exec(ctx, `
			UPDATE solana_payouts SET status = 'CONFIRMED', confirmed_at = $2 WHERE id = $1
		`, row.ID, now); err != nil {
			return out, err
		}
		if _, err := tx.Exec(ctx, `
			UPDATE withdrawals SET status = 'CONFIRMED', confirmed_at = $2 WHERE id = $1
		`, row.WithdrawalID, now); err != nil {
			return out, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO economy_ledger (account_id, kind, currency, amount, ref_id, reason)
			VALUES ($1, 'WITHDRAWAL_CONFIRMED', 'AEB', $2, $3, 'chain')
		`, row.AccountID, row.Amount, row.Signature); err != nil {
			return out, err
		}
		out.Confirmed++
		out.IDs = append(out.IDs, row.ID)
	}
	if err := tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}

func (s *PostgresStore) payoutsPaused(ctx context.Context, tx pgx.Tx, cfg ChainScanConfig) (bool, error) {
	wallet := strings.TrimSpace(cfg.PayoutWallet)
	if wallet == "" {
		return false, nil
	}
	var paused bool
	err := tx.QueryRow(ctx, `
		INSERT INTO hot_wallet_status (wallet, network, token_mint, payouts_paused)
		VALUES ($1, $2, NULLIF($3, ''), FALSE)
		ON CONFLICT (wallet) DO UPDATE SET network = EXCLUDED.network, updated_at = NOW()
		RETURNING payouts_paused
	`, wallet, cfg.Network, cfg.TokenMint).Scan(&paused)
	return paused, err
}

func (s *PostgresStore) setPayoutsPaused(ctx context.Context, tx pgx.Tx, cfg ChainScanConfig, paused bool, reason string) error {
	wallet := strings.TrimSpace(cfg.PayoutWallet)
	if wallet == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO hot_wallet_status (wallet, network, token_mint, payouts_paused)
		VALUES ($1, $2, NULLIF($3, ''), $4)
		ON CONFLICT (wallet) DO UPDATE
		SET network = EXCLUDED.network,
		    token_mint = EXCLUDED.token_mint,
		    payouts_paused = EXCLUDED.payouts_paused,
		    updated_at = NOW()
	`, wallet, cfg.Network, cfg.TokenMint, paused)
	_ = reason
	return err
}

func (s *PostgresStore) ensureHotWalletFunded(ctx context.Context, tx pgx.Tx, rpc chain.RPC, cfg ChainScanConfig) error {
	payer, err := chain.ParseSolanaPrivateKey(cfg.PayoutPrivateKey)
	if err != nil {
		_ = s.setPayoutsPaused(ctx, tx, cfg, true, err.Error())
		return err
	}
	sourceATA, err := chain.AssociatedTokenAddress(payer.PublicKey().String(), cfg.TokenMint)
	if err != nil {
		return err
	}
	bal, err := rpc.GetTokenAccountBalanceRaw(ctx, sourceATA)
	if err != nil {
		return err
	}
	if cfg.LowBalanceRaw > 0 && bal < cfg.LowBalanceRaw {
		msg := fmt.Sprintf("hot wallet ATA balance %d below threshold %d", bal, cfg.LowBalanceRaw)
		_ = s.setPayoutsPaused(ctx, tx, cfg, true, msg)
		return errors.New(msg)
	}
	if bal == 0 {
		msg := "hot wallet ATA balance is zero"
		_ = s.setPayoutsPaused(ctx, tx, cfg, true, msg)
		return errors.New(msg)
	}
	_ = s.setPayoutsPaused(ctx, tx, cfg, false, "")
	return nil
}

// SubmitPaymentOrderVerified records a tx hash after on-chain verification, then fulfills the order.
func (s *PostgresStore) SubmitPaymentOrderVerified(ctx context.Context, rpc chain.RPC, cfg ChainScanConfig, req MarketplaceSubmitPaymentRequest) (PaymentOrder, error) {
	cfg = cfg.withDefaults()
	sig := strings.TrimSpace(req.TxSignature)
	if sig == "" {
		return PaymentOrder{}, errors.New("txSignature is required")
	}
	if rpc == nil {
		return PaymentOrder{}, errors.New("rpc client is required for payment verification")
	}
	depositWallet := strings.TrimSpace(cfg.DepositWallet)
	if depositWallet == "" {
		return PaymentOrder{}, errors.New("SOLANA_DEPOSIT_WALLET is required for payment verification")
	}

	return runIdempotentAction(s, "payment_submit_verified", req.OpID, req.AccountID, 0, req, func(ctx context.Context, tx pgx.Tx) (PaymentOrder, error) {
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
		if order.Status == "FULFILLED" {
			return order, nil
		}
		if order.Status != "PENDING_PAYMENT" && order.Status != "SUBMITTED" {
			return PaymentOrder{}, fmt.Errorf("order status %s cannot accept signature", order.Status)
		}
		if time.Now().UTC().After(order.ExpiresAt) {
			_, _ = tx.Exec(ctx, `UPDATE economy_payment_orders SET status = 'EXPIRED' WHERE id = $1::uuid`, order.ID)
			return PaymentOrder{}, errors.New("payment order expired")
		}

		var payerWallet string
		if err := tx.QueryRow(ctx, `
			SELECT COALESCE(solana_wallet_address, '') FROM accounts WHERE id = $1
		`, req.AccountID).Scan(&payerWallet); err != nil {
			return PaymentOrder{}, err
		}
		payerWallet = strings.TrimSpace(payerWallet)
		if payerWallet == "" {
			return PaymentOrder{}, errors.New("account has no bound solana wallet")
		}

		detail, err := rpc.GetTransaction(ctx, sig)
		if err != nil {
			return PaymentOrder{}, err
		}
		if err := chain.VerifyInboundPayment(detail, depositWallet, cfg.TokenMint, payerWallet, order.Amount, cfg.TokenDecimals); err != nil {
			return PaymentOrder{}, err
		}

		now := time.Now().UTC()
		tag, err := tx.Exec(ctx, `
			INSERT INTO solana_deposits (account_id, wallet, token_mint, amount, signature, slot, status, credited_at)
			VALUES ($1, $2, $3, $4, $5, $6, 'PAYMENT_MATCHED', $7)
			ON CONFLICT (signature) DO NOTHING
		`, req.AccountID, payerWallet, cfg.TokenMint, order.Amount, sig, int64(detail.Slot), now)
		if err != nil {
			return PaymentOrder{}, err
		}
		if tag.RowsAffected() == 0 {
			return PaymentOrder{}, errors.New("chain receipt was already consumed")
		}
		if _, err := tx.Exec(ctx, `
			UPDATE economy_payment_orders
			SET status = 'SUBMITTED', tx_signature = $2, submitted_at = COALESCE(submitted_at, $3), confirmed_at = COALESCE(confirmed_at, $3)
			WHERE id = $1::uuid
		`, order.ID, sig, now); err != nil {
			return PaymentOrder{}, err
		}
		return s.fulfillPaymentOrderTx(ctx, tx, order.ID, "chain_verified")
	})
}

func (s *PostgresStore) ConfirmPaymentOrder(ctx context.Context, orderID, reason string) (PaymentOrder, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return PaymentOrder{}, errors.New("orderId is required")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return PaymentOrder{}, err
	}
	defer rollback(ctx, tx)
	order, err := s.lockPaymentOrder(ctx, tx, orderID)
	if err != nil {
		return PaymentOrder{}, err
	}
	if order.Status == "FULFILLED" {
		_ = tx.Commit(ctx)
		return order, nil
	}
	if order.Status != "SUBMITTED" && order.Status != "CONFIRMED" {
		return PaymentOrder{}, fmt.Errorf("order status %s cannot be confirmed without a submitted chain receipt", order.Status)
	}
	if strings.TrimSpace(order.TxSignature) == "" || order.SubmittedAt == nil {
		return PaymentOrder{}, errors.New("submitted payment order is missing its chain receipt")
	}
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `
		UPDATE economy_payment_orders
		SET status = 'CONFIRMED', confirmed_at = COALESCE(confirmed_at, $2)
		WHERE id = $1::uuid
	`, orderID, now); err != nil {
		return PaymentOrder{}, err
	}
	order, err = s.fulfillPaymentOrderTx(ctx, tx, orderID, reason)
	if err != nil {
		return PaymentOrder{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PaymentOrder{}, err
	}
	return order, nil
}

func (s *PostgresStore) lockPaymentOrder(ctx context.Context, tx pgx.Tx, orderID string) (PaymentOrder, error) {
	var row PaymentOrder
	var characterID *int64
	var receiver, txSig *string
	var submitted, confirmed, fulfilled *time.Time
	err := tx.QueryRow(ctx, `
		SELECT id::text, account_id, character_id, purpose, pay_asset, amount::bigint,
			receiver_wallet, status, tx_signature, created_at, expires_at,
			submitted_at, confirmed_at, fulfilled_at
		FROM economy_payment_orders
		WHERE id = $1::uuid
		FOR UPDATE
	`, orderID).Scan(
		&row.ID, &row.AccountID, &characterID, &row.Purpose, &row.PayAsset, &row.Amount,
		&receiver, &row.Status, &txSig, &row.CreatedAt, &row.ExpiresAt,
		&submitted, &confirmed, &fulfilled,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return PaymentOrder{}, ErrNotFound
	}
	if err != nil {
		return PaymentOrder{}, err
	}
	if characterID != nil {
		row.CharacterID = *characterID
	}
	if receiver != nil {
		row.ReceiverWallet = *receiver
	}
	if txSig != nil {
		row.TxSignature = *txSig
	}
	row.SubmittedAt = submitted
	row.ConfirmedAt = confirmed
	row.FulfilledAt = fulfilled
	return row, nil
}

func (s *PostgresStore) fulfillPaymentOrderTx(ctx context.Context, tx pgx.Tx, orderID, reason string) (PaymentOrder, error) {
	order, err := s.lockPaymentOrder(ctx, tx, orderID)
	if err != nil {
		return PaymentOrder{}, err
	}
	if order.Status == "FULFILLED" {
		return order, nil
	}
	now := time.Now().UTC()
	switch order.Purpose {
	case PaymentPurposeWalletExpand:
		if _, err := tx.Exec(ctx, `
			INSERT INTO account_market_slots (account_id)
			VALUES ($1)
			ON CONFLICT (account_id) DO NOTHING
		`, order.AccountID); err != nil {
			return PaymentOrder{}, err
		}
		tag, err := tx.Exec(ctx, `
			UPDATE account_market_slots
			SET wallet_expand_count = wallet_expand_count + 1, updated_at = NOW()
			WHERE account_id = $1
		`, order.AccountID)
		if err != nil {
			return PaymentOrder{}, err
		}
		if tag.RowsAffected() == 0 {
			return PaymentOrder{}, errors.New("market slots row missing")
		}
	case PaymentPurposeBagExpand:
		if order.CharacterID <= 0 {
			return PaymentOrder{}, errors.New("bag expand order missing characterId")
		}
		tag, err := tx.Exec(ctx, `
			UPDATE characters
			SET bag_expand_count = bag_expand_count + 1, updated_at = NOW()
			WHERE id = $1 AND account_id = $2 AND is_deleted = FALSE
		`, order.CharacterID, order.AccountID)
		if err != nil {
			return PaymentOrder{}, err
		}
		if tag.RowsAffected() == 0 {
			return PaymentOrder{}, errors.New("character missing for bag expand")
		}
	case PaymentPurposeTradingLicense:
		if _, err := tx.Exec(ctx, `
			UPDATE accounts
			SET has_trading_license = TRUE,
			    trading_license_at = COALESCE(trading_license_at, $2),
			    updated_at = NOW()
			WHERE id = $1
		`, order.AccountID, now); err != nil {
			return PaymentOrder{}, err
		}
	case PaymentPurposeLotteryDraw:
		if order.CharacterID <= 0 {
			return PaymentOrder{}, errors.New("lottery order missing characterId")
		}
		if err := s.fulfillLotteryPaymentTx(ctx, tx, order); err != nil {
			return PaymentOrder{}, err
		}
	case PaymentPurposeBountySlotUnlock, PaymentPurposeBountyPremiumRefresh:
		if err := s.fulfillBountyPaymentTx(ctx, tx, order); err != nil {
			return PaymentOrder{}, err
		}
	default:
		return PaymentOrder{}, fmt.Errorf("unsupported payment purpose %q", order.Purpose)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE economy_payment_orders
		SET status = 'FULFILLED', fulfilled_at = $2, confirmed_at = COALESCE(confirmed_at, $2)
		WHERE id = $1::uuid
	`, orderID, now); err != nil {
		return PaymentOrder{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO economy_ledger (account_id, character_id, kind, currency, amount, ref_id, reason)
		VALUES ($1, NULLIF($2, 0), 'PAYMENT_FULFILLED', $3, $4, $5, $6)
	`, order.AccountID, order.CharacterID, order.PayAsset, order.Amount, orderID, reason); err != nil {
		return PaymentOrder{}, err
	}
	return s.lockPaymentOrder(ctx, tx, orderID)
}
