// create_devnet_wallets creates disposable Solana Devnet identities for local
// integration testing. It deliberately never prints private keys.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/gagliardetto/solana-go"
)

type walletDefinition struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type publicWallet struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Address string `json:"address"`
}

var wallets = []walletDefinition{
	{Name: "player-01", Role: "independent Devnet test player"},
	{Name: "player-02", Role: "independent Devnet test player"},
	{Name: "player-03", Role: "independent Devnet test player"},
	{Name: "team-locked", Role: "Team Locked Wallet; never configured in the backend"},
	{Name: "treasury", Role: "Treasury Wallet and inbound AEB payment receiver"},
	{Name: "game-rewards", Role: "Game Rewards Wallet; manually funds the hot wallet"},
	{Name: "growth", Role: "Growth Wallet for partners and test incentives"},
	{Name: "revenue", Role: "Revenue Wallet; receives non-AEB project revenue"},
	{Name: "reward-distribution", Role: "Reward Distribution hot wallet; signs automated AEB payouts"},
}

func main() {
	outDir := flag.String("out", "work/devnet-wallets", "directory for generated Devnet keypairs")
	flag.Parse()

	manifestPath := filepath.Join(*outDir, "manifest.json")
	if _, err := os.Stat(manifestPath); err == nil {
		exitf("refusing to overwrite existing wallet set at %s", *outDir)
	} else if !errors.Is(err, os.ErrNotExist) {
		exitf("inspect wallet output directory: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(*outDir, "keypairs"), 0o700); err != nil {
		exitf("create wallet output directory: %v", err)
	}

	public := make([]publicWallet, 0, len(wallets))
	privateKeys := make(map[string]solana.PrivateKey, len(wallets))
	for _, definition := range wallets {
		privateKey, err := solana.NewRandomPrivateKey()
		if err != nil {
			exitf("generate %s: %v", definition.Name, err)
		}
		privateKeys[definition.Name] = privateKey
		public = append(public, publicWallet{
			Name:    definition.Name,
			Role:    definition.Role,
			Address: privateKey.PublicKey().String(),
		})
	}

	for _, definition := range wallets {
		encoded, err := json.Marshal(privateKeys[definition.Name])
		if err != nil {
			exitf("encode %s keypair: %v", definition.Name, err)
		}
		writeNew(filepath.Join(*outDir, "keypairs", definition.Name+".json"), encoded, 0o600)
	}

	manifest, err := json.MarshalIndent(struct {
		Network string         `json:"network"`
		Wallets []publicWallet `json:"wallets"`
	}{Network: "solana-devnet", Wallets: public}, "", "  ")
	if err != nil {
		exitf("encode public manifest: %v", err)
	}
	writeNew(manifestPath, append(manifest, '\n'), 0o600)

	// This is intentionally separate from local.env: a token mint is still
	// required before real-chain mode can start, and existing local settings must
	// not be changed by a key-generation command.
	byName := map[string]publicWallet{}
	for _, wallet := range public {
		byName[wallet.Name] = wallet
	}
	runtime := fmt.Sprintf("# Devnet wallet identities. Keep this file local.\nSOLANA_DEPOSIT_WALLET=%s\nSOLANA_PAYOUT_WALLET=%s\nSOLANA_PAYOUT_PRIVATE_KEY=%s\n", byName["treasury"].Address, byName["reward-distribution"].Address, privateKeys["reward-distribution"].String())
	writeNew(filepath.Join(*outDir, "runtime.env"), []byte(runtime), 0o600)

	fmt.Printf("Created %d Devnet wallets in %s\n", len(wallets), *outDir)
	fmt.Println("Public addresses:")
	for _, wallet := range public {
		fmt.Printf("- %-20s %s\n", wallet.Name, wallet.Address)
	}
	fmt.Println("Private keys were written only to 0600 files and were not printed.")
}

func writeNew(path string, contents []byte, mode os.FileMode) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		exitf("create %s: %v", path, err)
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		exitf("write %s: %v", path, err)
	}
	if err := file.Close(); err != nil {
		exitf("close %s: %v", path, err)
	}
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "create_devnet_wallets: "+format+"\n", args...)
	os.Exit(1)
}
