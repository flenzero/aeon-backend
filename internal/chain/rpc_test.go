package chain

import "testing"

func TestInboundTokenCredits(t *testing.T) {
	tx := &TransactionDetail{
		Signature: "sig1",
		Slot:      42,
		PreTokenBalances: []TokenBalance{
			{Owner: "player1", Mint: "mintAEB", Amount: 1000},
			{Owner: "treasury", Mint: "mintAEB", Amount: 5000},
		},
		PostTokenBalances: []TokenBalance{
			{Owner: "player1", Mint: "mintAEB", Amount: 900},
			{Owner: "treasury", Mint: "mintAEB", Amount: 5100},
		},
	}
	credits := InboundTokenCredits(tx, "treasury", "mintAEB")
	if len(credits) != 1 {
		t.Fatalf("credits=%d want 1", len(credits))
	}
	if credits[0].AmountRaw != 100 || credits[0].FromOwner != "player1" {
		t.Fatalf("credit=%+v", credits[0])
	}
}

func TestRawToGameAmount(t *testing.T) {
	if got := RawToGameAmount(1_500_000_000, 9); got != 1 {
		t.Fatalf("got %d", got)
	}
	if got := RawToGameAmount(42, 0); got != 42 {
		t.Fatalf("got %d", got)
	}
}
