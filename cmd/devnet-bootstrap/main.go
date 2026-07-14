// devnet-bootstrap creates the disposable AEB Devnet test economy.
// It creates a fixed-supply SPL mint and never prints a private key.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	solanarpc "github.com/gagliardetto/solana-go/rpc"
)

const (
	devnetRPC       = "https://api.devnet.solana.com"
	tokenDecimals   = 9
	lamportsPerSOL  = 1_000_000_000
	unitsPerAEB     = 1_000_000_000
	totalSupplyAEB  = 1_000_000_000
	minimumTreasury = 50_000_000 // 0.05 SOL covers mint, ATAs, and transactions.
)

type wallet struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Address string `json:"address"`
}

type walletManifest struct {
	Network string   `json:"network"`
	Wallets []wallet `json:"wallets"`
}

type allocation struct {
	Name  string `json:"name"`
	Units uint64 `json:"units"`
}

type publicBootstrap struct {
	Network       string            `json:"network"`
	RPCURL        string            `json:"rpcUrl"`
	TokenSymbol   string            `json:"tokenSymbol"`
	Mint          string            `json:"mint"`
	Decimals      int               `json:"decimals"`
	TotalSupply   uint64            `json:"totalSupply"`
	Treasury      string            `json:"treasury"`
	PayoutWallet  string            `json:"payoutWallet"`
	TokenAccounts map[string]string `json:"tokenAccounts"`
	Allocations   []allocation      `json:"allocations"`
	Transactions  []string          `json:"transactions"`
}

func main() {
	walletDir := flag.String("wallet-dir", "work/devnet-wallets", "directory created by scripts/create_devnet_wallets.go")
	recoverMint := flag.String("recover-mint", "", "finalized Devnet AEB mint address to recover after an interrupted bootstrap")
	revocationSignature := flag.String("revocation-signature", "", "finalized mint-authority revocation signature required with --recover-mint")
	flag.Parse()

	publicPath := filepath.Join(*walletDir, "aeb-devnet.json")
	if _, err := os.Stat(publicPath); err == nil {
		exitf("refusing to create another Devnet mint; existing bootstrap metadata is %s", publicPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		exitf("inspect bootstrap metadata: %v", err)
	}

	manifest := readManifest(filepath.Join(*walletDir, "manifest.json"))
	wallets := mapWallets(manifest.Wallets)
	ensureLPProvisioning(*walletDir, &manifest, wallets)
	if *recoverMint != "" {
		recoverBootstrap(*walletDir, publicPath, wallets, *recoverMint, *revocationSignature)
		return
	}

	treasury := readPrivateKey(*walletDir, "treasury")
	payout := readPrivateKey(*walletDir, "reward-distribution")
	client := solanarpc.New(devnetRPC)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	balance, err := client.GetBalance(ctx, treasury.PublicKey(), solanarpc.CommitmentConfirmed)
	if err != nil {
		exitf("read treasury SOL balance: %v", err)
	}
	if balance.Value < minimumTreasury {
		exitf("treasury needs at least 0.05 DEVNET SOL; has %d lamports", balance.Value)
	}

	mint, err := solana.NewRandomPrivateKey()
	if err != nil {
		exitf("generate AEB mint keypair: %v", err)
	}
	mintRent, err := client.GetMinimumBalanceForRentExemption(ctx, token.MINT_SIZE, solanarpc.CommitmentConfirmed)
	if err != nil {
		exitf("fetch mint rent exemption: %v", err)
	}

	transactions := make([]string, 0, 6)
	transactions = append(transactions, submit(ctx, client, treasury, []solana.PrivateKey{mint}, []solana.Instruction{
		system.NewCreateAccountInstruction(mintRent, token.MINT_SIZE, solana.TokenProgramID, treasury.PublicKey(), mint.PublicKey()).Build(),
		token.NewInitializeMint2Instruction(tokenDecimals, treasury.PublicKey(), solana.PublicKey{}, mint.PublicKey()).Build(),
	}, "create AEB mint"))

	ataOwners, tokenAccounts := deriveTokenAccounts(wallets, mint.PublicKey())
	for start := 0; start < len(ataOwners); start += 3 {
		end := start + 3
		if end > len(ataOwners) {
			end = len(ataOwners)
		}
		instructions := make([]solana.Instruction, 0, end-start)
		for _, name := range ataOwners[start:end] {
			instructions = append(instructions, associatedtokenaccount.NewCreateInstruction(treasury.PublicKey(), mustPublicKey(wallets, name), mint.PublicKey()).Build())
		}
		transactions = append(transactions, submit(ctx, client, treasury, nil, instructions, "create associated token accounts"))
	}

	allocations := aebAllocations()
	if sumAllocations(allocations) != totalSupplyAEB {
		exitf("internal allocation total is invalid")
	}
	mintInstructions := make([]solana.Instruction, 0, len(allocations))
	for _, item := range allocations {
		mintInstructions = append(mintInstructions, token.NewMintToInstruction(item.Units*unitsPerAEB, mint.PublicKey(), mustPublicKeyFromString(tokenAccounts[item.Name]), treasury.PublicKey(), nil).Build())
	}
	transactions = append(transactions, submit(ctx, client, treasury, nil, mintInstructions, "mint AEB allocations"))
	transactions = append(transactions, submit(ctx, client, treasury, nil, []solana.Instruction{
		token.NewSetAuthorityInstruction(token.AuthorityMintTokens, solana.PublicKey{}, mint.PublicKey(), treasury.PublicKey(), nil).Build(),
	}, "revoke AEB mint authority"))

	for name, ata := range tokenAccounts {
		balance, err := client.GetTokenAccountBalance(ctx, mustPublicKeyFromString(ata), solanarpc.CommitmentConfirmed)
		if err != nil {
			exitf("verify %s token account: %v", name, err)
		}
		if balance == nil || balance.Value == nil {
			exitf("verify %s token account: empty RPC response", name)
		}
	}

	writeBootstrapArtifacts(*walletDir, publicPath, mint.PublicKey(), treasury, payout, tokenAccounts, allocations, transactions)

	fmt.Printf("AEB Devnet mint: %s\n", mint.PublicKey())
	fmt.Printf("Treasury: %s\nPayout hot wallet: %s\n", treasury.PublicKey(), payout.PublicKey())
	fmt.Printf("Created %d ATAs, minted %d AEB, and revoked mint authority.\n", len(tokenAccounts), totalSupplyAEB)
	fmt.Printf("Public metadata: %s\n", publicPath)
	fmt.Println("The runtime environment snippet is private and was not printed.")
}

func recoverBootstrap(walletDir, publicPath string, wallets map[string]wallet, mintAddress, revocationSignature string) {
	if revocationSignature == "" {
		exitf("--revocation-signature is required with --recover-mint")
	}
	mint := mustPublicKeyFromString(mintAddress)
	treasury := readPrivateKey(walletDir, "treasury")
	payout := readPrivateKey(walletDir, "reward-distribution")
	client := solanarpc.New(devnetRPC)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	signature, err := solana.SignatureFromBase58(revocationSignature)
	if err != nil {
		exitf("parse revocation signature: %v", err)
	}
	statuses, err := client.GetSignatureStatuses(ctx, true, signature)
	if err != nil || len(statuses.Value) != 1 || statuses.Value[0] == nil {
		exitf("look up revocation signature: %v", err)
	}
	status := statuses.Value[0]
	if status.Err != nil || status.ConfirmationStatus != solanarpc.ConfirmationStatusFinalized {
		exitf("revocation signature is not finalized successfully")
	}

	_, tokenAccounts := deriveTokenAccounts(wallets, mint)
	allocations := aebAllocations()
	for _, allocation := range allocations {
		balance, err := client.GetTokenAccountBalance(ctx, mustPublicKeyFromString(tokenAccounts[allocation.Name]), solanarpc.CommitmentConfirmed)
		if err != nil || balance == nil || balance.Value == nil {
			exitf("verify recovered %s token account: %v", allocation.Name, err)
		}
		if balance.Value.Amount != fmt.Sprintf("%d", allocation.Units*unitsPerAEB) {
			exitf("recovered %s balance is %s, want %d", allocation.Name, balance.Value.Amount, allocation.Units*unitsPerAEB)
		}
	}
	for _, player := range []string{"player-01", "player-02", "player-03"} {
		balance, err := client.GetTokenAccountBalance(ctx, mustPublicKeyFromString(tokenAccounts[player]), solanarpc.CommitmentConfirmed)
		if err != nil || balance == nil || balance.Value == nil || balance.Value.Amount != "0" {
			exitf("verify recovered %s token account: unexpected balance", player)
		}
	}
	writeBootstrapArtifacts(walletDir, publicPath, mint, treasury, payout, tokenAccounts, allocations, []string{revocationSignature})
	fmt.Printf("Recovered finalized AEB Devnet mint: %s\n", mint)
}

func deriveTokenAccounts(wallets map[string]wallet, mint solana.PublicKey) ([]string, map[string]string) {
	ataOwners := []string{
		"team-locked", "treasury", "game-rewards", "growth", "lp-provisioning", "reward-distribution",
		"player-01", "player-02", "player-03",
	}
	tokenAccounts := make(map[string]string, len(ataOwners))
	for _, name := range ataOwners {
		ata, _, err := solana.FindAssociatedTokenAddress(mustPublicKey(wallets, name), mint)
		if err != nil {
			exitf("derive %s ATA: %v", name, err)
		}
		tokenAccounts[name] = ata.String()
	}
	return ataOwners, tokenAccounts
}

func aebAllocations() []allocation {
	return []allocation{
		{Name: "team-locked", Units: 250_000_000},
		{Name: "treasury", Units: 320_000_000},
		// 300 AEB is a disposable two-to-three-day Devnet payout budget.
		{Name: "game-rewards", Units: 249_999_700},
		{Name: "growth", Units: 160_000_000},
		{Name: "lp-provisioning", Units: 20_000_000},
		{Name: "reward-distribution", Units: 300},
	}
}

func writeBootstrapArtifacts(walletDir, publicPath string, mint solana.PublicKey, treasury, payout solana.PrivateKey, tokenAccounts map[string]string, allocations []allocation, transactions []string) {
	metadata := publicBootstrap{
		Network:       "solana-devnet",
		RPCURL:        devnetRPC,
		TokenSymbol:   "AEB",
		Mint:          mint.String(),
		Decimals:      tokenDecimals,
		TotalSupply:   totalSupplyAEB,
		Treasury:      treasury.PublicKey().String(),
		PayoutWallet:  payout.PublicKey().String(),
		TokenAccounts: tokenAccounts,
		Allocations:   allocations,
		Transactions:  transactions,
	}
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		exitf("encode bootstrap metadata: %v", err)
	}
	writeNew(publicPath, append(encoded, '\n'), 0o600)
	env := fmt.Sprintf("# Generated Devnet test-chain configuration. Keep local.\nSTUB_MODE=disabled\nSOLANA_RPC_ENABLED=true\nSOLANA_RPC_URL=%s\nSOLANA_NETWORK=solana-devnet\nSOLANA_TOKEN_MINT=%s\nSOLANA_TOKEN_DECIMALS=%d\nSOLANA_DEPOSIT_WALLET=%s\nSOLANA_PAYOUT_WALLET=%s\nSOLANA_PAYOUT_PRIVATE_KEY=%s\nSOLANA_PAYOUT_MODE=rpc\nSOLANA_PAYOUT_LOW_BALANCE_RAW=0\n", devnetRPC, mint, tokenDecimals, treasury.PublicKey(), payout.PublicKey(), payout.String())
	writeNew(filepath.Join(walletDir, "aeb-devnet.env"), []byte(env), 0o600)
}

func readManifest(path string) walletManifest {
	raw, err := os.ReadFile(path)
	if err != nil {
		exitf("read wallet manifest: %v", err)
	}
	var manifest walletManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		exitf("parse wallet manifest: %v", err)
	}
	if manifest.Network != "solana-devnet" {
		exitf("wallet manifest network must be solana-devnet")
	}
	return manifest
}

func mapWallets(rows []wallet) map[string]wallet {
	result := make(map[string]wallet, len(rows))
	for _, row := range rows {
		if _, err := solana.PublicKeyFromBase58(row.Address); err != nil {
			exitf("invalid %s address in manifest: %v", row.Name, err)
		}
		result[row.Name] = row
	}
	for _, required := range []string{"player-01", "player-02", "player-03", "team-locked", "treasury", "game-rewards", "growth", "reward-distribution"} {
		if _, exists := result[required]; !exists {
			exitf("wallet manifest is missing %s", required)
		}
	}
	return result
}

func ensureLPProvisioning(walletDir string, manifest *walletManifest, wallets map[string]wallet) {
	if _, exists := wallets["lp-provisioning"]; exists {
		return
	}
	privateKey, err := solana.NewRandomPrivateKey()
	if err != nil {
		exitf("generate LP provisioning keypair: %v", err)
	}
	keyPath := filepath.Join(walletDir, "keypairs", "lp-provisioning.json")
	encoded, err := json.Marshal(privateKey)
	if err != nil {
		exitf("encode LP provisioning keypair: %v", err)
	}
	writeNew(keyPath, encoded, 0o600)
	entry := wallet{Name: "lp-provisioning", Role: "Initial LP allocation; reserved for Devnet DEX provisioning", Address: privateKey.PublicKey().String()}
	manifest.Wallets = append(manifest.Wallets, entry)
	updated, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		exitf("encode updated wallet manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(walletDir, "manifest.json"), append(updated, '\n'), 0o600); err != nil {
		exitf("update wallet manifest: %v", err)
	}
	wallets[entry.Name] = entry
	fmt.Printf("Created LP provisioning wallet: %s\n", entry.Address)
}

func readPrivateKey(walletDir, name string) solana.PrivateKey {
	raw, err := os.ReadFile(filepath.Join(walletDir, "keypairs", name+".json"))
	if err != nil {
		exitf("read %s keypair: %v", name, err)
	}
	var bytes []byte
	if err := json.Unmarshal(raw, &bytes); err != nil {
		exitf("parse %s keypair: %v", name, err)
	}
	privateKey := solana.PrivateKey(bytes)
	if err := privateKey.Validate(); err != nil {
		exitf("validate %s keypair: %v", name, err)
	}
	return privateKey
}

func mustPublicKey(wallets map[string]wallet, name string) solana.PublicKey {
	key, err := solana.PublicKeyFromBase58(wallets[name].Address)
	if err != nil {
		exitf("parse %s address: %v", name, err)
	}
	return key
}

func mustPublicKeyFromString(raw string) solana.PublicKey {
	key, err := solana.PublicKeyFromBase58(raw)
	if err != nil {
		exitf("parse public key: %v", err)
	}
	return key
}

func sumAllocations(rows []allocation) uint64 {
	var total uint64
	for _, row := range rows {
		total += row.Units
	}
	return total
}

func submit(ctx context.Context, client *solanarpc.Client, payer solana.PrivateKey, extraSigners []solana.PrivateKey, instructions []solana.Instruction, operation string) string {
	// Use the same commitment for the blockhash and preflight simulation. Public
	// Devnet RPC is load-balanced; mixing commitments can select a node that has
	// not seen a just-fetched hash yet.
	latest, err := client.GetLatestBlockhash(ctx, solanarpc.CommitmentProcessed)
	if err != nil || latest == nil || latest.Value == nil {
		exitf("%s: get latest blockhash: %v", operation, err)
	}
	tx, err := solana.NewTransaction(instructions, latest.Value.Blockhash, solana.TransactionPayer(payer.PublicKey()))
	if err != nil {
		exitf("%s: build transaction: %v", operation, err)
	}
	signers := append([]solana.PrivateKey{payer}, extraSigners...)
	if _, err := tx.Sign(func(publicKey solana.PublicKey) *solana.PrivateKey {
		for i := range signers {
			if signers[i].PublicKey().Equals(publicKey) {
				return &signers[i]
			}
		}
		return nil
	}); err != nil {
		exitf("%s: sign transaction: %v", operation, err)
	}
	signature, err := client.SendTransactionWithOpts(ctx, tx, solanarpc.TransactionOpts{
		SkipPreflight:       false,
		PreflightCommitment: solanarpc.CommitmentProcessed,
	})
	if err != nil {
		exitf("%s: submit transaction: %v", operation, err)
	}
	for attempt := 0; attempt < 30; attempt++ {
		statuses, statusErr := client.GetSignatureStatuses(ctx, true, signature)
		if statusErr == nil && len(statuses.Value) == 1 && statuses.Value[0] != nil {
			status := statuses.Value[0]
			if status.Err != nil {
				exitf("%s: transaction %s failed: %v", operation, signature, status.Err)
			}
			if status.ConfirmationStatus == solanarpc.ConfirmationStatusConfirmed || status.ConfirmationStatus == solanarpc.ConfirmationStatusFinalized {
				return signature.String()
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	exitf("%s: transaction %s was not confirmed within 15 seconds", operation, signature)
	return ""
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
	fmt.Fprintf(os.Stderr, "devnet-bootstrap: "+format+"\n", args...)
	os.Exit(1)
}
