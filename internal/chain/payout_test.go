package chain_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"

	"github.com/flenzero/aeon-backend/internal/chain"
)

func TestVerifyInboundPayment(t *testing.T) {
	tx := &chain.TransactionDetail{
		Signature: "sig1",
		Slot:      10,
		PreTokenBalances: []chain.TokenBalance{
			{AccountIndex: 0, Mint: "MintAEB", Owner: "Player", Amount: 100_000_000_000, Decimals: 9},
			{AccountIndex: 1, Mint: "MintAEB", Owner: "Treasury", Amount: 0, Decimals: 9},
		},
		PostTokenBalances: []chain.TokenBalance{
			{AccountIndex: 0, Mint: "MintAEB", Owner: "Player", Amount: 50_000_000_000, Decimals: 9},
			{AccountIndex: 1, Mint: "MintAEB", Owner: "Treasury", Amount: 50_000_000_000, Decimals: 9},
		},
	}
	if err := chain.VerifyInboundPayment(tx, "Treasury", "MintAEB", "Player", 50, 9); err != nil {
		t.Fatalf("expected ok: %v", err)
	}
	if err := chain.VerifyInboundPayment(tx, "Treasury", "MintAEB", "Other", 50, 9); err == nil {
		t.Fatal("expected payer mismatch")
	}
	if err := chain.VerifyInboundPayment(tx, "Treasury", "MintAEB", "Player", 51, 9); err == nil {
		t.Fatal("expected amount mismatch")
	}
}

func TestGameToRawAmount(t *testing.T) {
	raw, err := chain.GameToRawAmount(50, 9)
	if err != nil {
		t.Fatal(err)
	}
	if raw != 50_000_000_000 {
		t.Fatalf("raw=%d", raw)
	}
}

func TestParseSolanaPrivateKeyJSON(t *testing.T) {
	raw := make([]int, 64)
	for i := range raw {
		raw[i] = i
	}
	buf, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	pk, err := chain.ParseSolanaPrivateKey(string(buf))
	if err != nil {
		t.Fatal(err)
	}
	if len(pk) != 64 {
		t.Fatalf("len=%d", len(pk))
	}
}

func TestSPLPayoutSenderFailsWhenRecipientATAMissing(t *testing.T) {
	payer := solana.NewWallet()
	recipient := solana.NewWallet()
	mint := solana.NewWallet().PublicKey()

	destATA, _, err := solana.FindAssociatedTokenAddress(recipient.PublicKey(), mint)
	if err != nil {
		t.Fatal(err)
	}
	rpc := &chain.MemoryRPC{
		Accounts: map[string]bool{
			destATA.String(): false,
		},
	}
	_, err = chain.SPLPayoutSender{}.SendSPLTransfer(context.Background(), rpc, chain.PayoutRequest{
		RecipientWallet: recipient.PublicKey().String(),
		AmountGame:      1,
		Decimals:        9,
		Mint:            mint.String(),
		PayerPrivateKey: base58.Encode(payer.PrivateKey),
	})
	if !errors.Is(err, chain.ErrRecipientATAMissing) {
		t.Fatalf("got %v, want ErrRecipientATAMissing", err)
	}
}
