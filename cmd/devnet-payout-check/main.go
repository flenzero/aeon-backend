// devnet-payout-check verifies the same SPL TransferChecked path used by the
// economy worker against the disposable Devnet AEB environment.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gagliardetto/solana-go"

	"github.com/flenzero/aeon-backend/internal/chain"
)

const rawAEB = uint64(1_000_000_000)

type bootstrap struct {
	RPCURL        string            `json:"rpcUrl"`
	Mint          string            `json:"mint"`
	Decimals      int               `json:"decimals"`
	TokenAccounts map[string]string `json:"tokenAccounts"`
}

type manifest struct {
	Wallets []struct {
		Name    string `json:"name"`
		Address string `json:"address"`
	} `json:"wallets"`
}

func main() {
	walletDir := "work/devnet-wallets"
	config := readBootstrap(filepath.Join(walletDir, "aeb-devnet.json"))
	player := readWallet(filepath.Join(walletDir, "manifest.json"), "player-01")
	privateKey := readPrivateKey(filepath.Join(walletDir, "keypairs", "reward-distribution.json"))

	recipientATA := config.TokenAccounts["player-01"]
	if recipientATA == "" {
		exitf("player-01 ATA is absent from bootstrap metadata")
	}
	rpc := chain.NewHTTPClient(config.RPCURL)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	before, err := rpc.GetTokenAccountBalanceRaw(ctx, recipientATA)
	if err != nil {
		exitf("read player-01 balance before payout: %v", err)
	}
	signature, err := (chain.SPLPayoutSender{}).SendSPLTransfer(ctx, rpc, chain.PayoutRequest{
		RecipientWallet: player,
		AmountGame:      1,
		Decimals:        config.Decimals,
		Mint:            config.Mint,
		PayerPrivateKey: privateKey.String(),
	})
	if err != nil {
		exitf("submit production payout path: %v", err)
	}
	for attempt := 0; attempt < 30; attempt++ {
		statuses, err := rpc.GetSignatureStatuses(ctx, []string{signature})
		if err == nil && len(statuses) == 1 && statuses[0].Err == nil && (statuses[0].ConfirmationStatus == "confirmed" || statuses[0].ConfirmationStatus == "finalized") {
			after, err := rpc.GetTokenAccountBalanceRaw(ctx, recipientATA)
			if err != nil {
				exitf("read player-01 balance after payout: %v", err)
			}
			if after != before+rawAEB {
				exitf("player-01 raw balance is %d after payout, want %d", after, before+rawAEB)
			}
			fmt.Printf("Verified 1 AEB TransferChecked payout to player-01: %s\n", signature)
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	exitf("payout %s was not confirmed within 15 seconds", signature)
}

func readBootstrap(path string) bootstrap {
	raw, err := os.ReadFile(path)
	if err != nil {
		exitf("read bootstrap metadata: %v", err)
	}
	var config bootstrap
	if err := json.Unmarshal(raw, &config); err != nil {
		exitf("parse bootstrap metadata: %v", err)
	}
	if config.RPCURL == "" || config.Mint == "" || config.Decimals != 9 {
		exitf("bootstrap metadata is incomplete")
	}
	return config
}

func readWallet(path, name string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		exitf("read wallet manifest: %v", err)
	}
	var manifest manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		exitf("parse wallet manifest: %v", err)
	}
	for _, wallet := range manifest.Wallets {
		if wallet.Name == name {
			if _, err := solana.PublicKeyFromBase58(wallet.Address); err != nil {
				exitf("validate %s address: %v", name, err)
			}
			return wallet.Address
		}
	}
	exitf("wallet manifest is missing %s", name)
	return ""
}

func readPrivateKey(path string) solana.PrivateKey {
	raw, err := os.ReadFile(path)
	if err != nil {
		exitf("read payout keypair: %v", err)
	}
	var bytes []byte
	if err := json.Unmarshal(raw, &bytes); err != nil {
		exitf("parse payout keypair: %v", err)
	}
	privateKey := solana.PrivateKey(bytes)
	if err := privateKey.Validate(); err != nil {
		exitf("validate payout keypair: %v", err)
	}
	return privateKey
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "devnet-payout-check: "+format+"\n", args...)
	os.Exit(1)
}
