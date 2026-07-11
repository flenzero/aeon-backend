package store

import (
	"testing"
	"time"
)

func TestSettleUnlocksMovesLockedGameToWithdrawable(t *testing.T) {
	st := New()
	account := st.UpsertAccountByWallet("7J6xqPp3EnZrWjF7V4Q5Q9wMZB7xBqk9XbSGkH6X8qT")
	_, err := st.GrantLocked(account.ID, 100, "gold_convert", "op-1", time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("grant locked: %v", err)
	}

	settled := st.SettleUnlocks(time.Now(), 10)
	if len(settled) != 1 {
		t.Fatalf("settled len = %d, want 1", len(settled))
	}
	token := st.Token(account.ID)
	if token.LockedBalance != 0 {
		t.Fatalf("locked balance = %d, want 0", token.LockedBalance)
	}
	if token.WithdrawableBalance != 100 {
		t.Fatalf("withdrawable balance = %d, want 100", token.WithdrawableBalance)
	}
}

func TestProcessAutoWithdrawalsRoutesLargeRequestToManualReview(t *testing.T) {
	st := New()
	account := st.UpsertAccountByWallet("7J6xqPp3EnZrWjF7V4Q5Q9wMZB7xBqk9XbSGkH6X8qT")
	_, err := st.GrantLocked(account.ID, 6000, "gold_convert", "op-1", time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatalf("grant locked: %v", err)
	}
	st.SettleUnlocks(time.Now(), 10)
	_, err = st.CreateWithdrawal(account.ID, 6000, account.WalletAddress, false)
	if err != nil {
		t.Fatalf("create withdrawal: %v", err)
	}

	processed := st.ProcessAutoWithdrawals(time.Now(), 5000, 20000, 30000, 150000, 10)
	if len(processed) != 1 {
		t.Fatalf("processed len = %d, want 1", len(processed))
	}
	if processed[0].Status != "manual_review" {
		t.Fatalf("status = %s, want manual_review", processed[0].Status)
	}
}
