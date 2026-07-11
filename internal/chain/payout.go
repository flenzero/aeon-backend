package chain

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/mr-tron/base58"
)

var (
	ErrRecipientATAMissing = errors.New("recipient associated token account does not exist; player must already hold this token")
	ErrPayoutKeyMissing    = errors.New("SOLANA_PAYOUT_PRIVATE_KEY is required for rpc payout mode")
	ErrPayoutConfig        = errors.New("payout mint/wallet configuration is incomplete")
)

type PayoutRequest struct {
	RecipientWallet string
	AmountGame      int64
	Decimals        int
	Mint            string
	PayerPrivateKey string
}

type PayoutSender interface {
	SendSPLTransfer(ctx context.Context, rpc RPC, req PayoutRequest) (signature string, err error)
}

type SPLPayoutSender struct{}

func (SPLPayoutSender) SendSPLTransfer(ctx context.Context, rpc RPC, req PayoutRequest) (string, error) {
	if rpc == nil {
		return "", errors.New("rpc client is required")
	}
	mint := strings.TrimSpace(req.Mint)
	recipient := strings.TrimSpace(req.RecipientWallet)
	if mint == "" || recipient == "" {
		return "", ErrPayoutConfig
	}
	payer, err := ParseSolanaPrivateKey(req.PayerPrivateKey)
	if err != nil {
		return "", err
	}
	mintPK, err := solana.PublicKeyFromBase58(mint)
	if err != nil {
		return "", fmt.Errorf("invalid mint: %w", err)
	}
	recipientPK, err := solana.PublicKeyFromBase58(recipient)
	if err != nil {
		return "", fmt.Errorf("invalid recipient wallet: %w", err)
	}

	sourceATA, _, err := solana.FindAssociatedTokenAddress(payer.PublicKey(), mintPK)
	if err != nil {
		return "", err
	}
	destATA, _, err := solana.FindAssociatedTokenAddress(recipientPK, mintPK)
	if err != nil {
		return "", err
	}

	exists, err := rpc.AccountExists(ctx, destATA.String())
	if err != nil {
		return "", err
	}
	if !exists {
		return "", ErrRecipientATAMissing
	}

	rawAmount, err := GameToRawAmount(req.AmountGame, req.Decimals)
	if err != nil {
		return "", err
	}

	blockhash, err := rpc.GetLatestBlockhash(ctx)
	if err != nil {
		return "", err
	}
	recent, err := solana.HashFromBase58(blockhash)
	if err != nil {
		return "", fmt.Errorf("invalid blockhash: %w", err)
	}

	transferIx := token.NewTransferCheckedInstruction(
		rawAmount,
		uint8(req.Decimals),
		sourceATA,
		mintPK,
		destATA,
		payer.PublicKey(),
		[]solana.PublicKey{},
	).Build()

	tx, err := solana.NewTransaction(
		[]solana.Instruction{transferIx},
		recent,
		solana.TransactionPayer(payer.PublicKey()),
	)
	if err != nil {
		return "", err
	}
	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(payer.PublicKey()) {
			return &payer
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	bin, err := tx.MarshalBinary()
	if err != nil {
		return "", err
	}
	return rpc.SendTransaction(ctx, base64.StdEncoding.EncodeToString(bin))
}

// AssociatedTokenAddress derives the ATA for wallet+mint (base58 strings).
func AssociatedTokenAddress(wallet, mint string) (string, error) {
	walletPK, err := solana.PublicKeyFromBase58(strings.TrimSpace(wallet))
	if err != nil {
		return "", fmt.Errorf("invalid wallet: %w", err)
	}
	mintPK, err := solana.PublicKeyFromBase58(strings.TrimSpace(mint))
	if err != nil {
		return "", fmt.Errorf("invalid mint: %w", err)
	}
	ata, _, err := solana.FindAssociatedTokenAddress(walletPK, mintPK)
	if err != nil {
		return "", err
	}
	return ata.String(), nil
}

// ParseSolanaPrivateKey accepts base58 64-byte secret keys or JSON byte arrays.
func ParseSolanaPrivateKey(raw string) (solana.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, ErrPayoutKeyMissing
	}
	if strings.HasPrefix(raw, "[") {
		var bytes []byte
		if err := json.Unmarshal([]byte(raw), &bytes); err != nil {
			return nil, fmt.Errorf("invalid json private key: %w", err)
		}
		if len(bytes) != ed25519.PrivateKeySize {
			return nil, fmt.Errorf("json private key must be %d bytes", ed25519.PrivateKeySize)
		}
		return solana.PrivateKey(bytes), nil
	}
	decoded, err := base58.Decode(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid base58 private key: %w", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("base58 private key must decode to %d bytes", ed25519.PrivateKeySize)
	}
	return solana.PrivateKey(decoded), nil
}

func GameToRawAmount(amount int64, decimals int) (uint64, error) {
	if amount <= 0 {
		return 0, errors.New("amount must be positive")
	}
	if decimals < 0 || decimals > 18 {
		return 0, errors.New("decimals out of range")
	}
	raw := new(big.Int).SetInt64(amount)
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	raw.Mul(raw, scale)
	if !raw.IsUint64() {
		return 0, errors.New("amount overflows uint64 raw units")
	}
	return raw.Uint64(), nil
}

// VerifyInboundPayment checks a signature paid the deposit wallet enough of mint from expectedPayer (optional).
func VerifyInboundPayment(tx *TransactionDetail, depositOwner, mint, expectedPayer string, minGameAmount int64, decimals int) error {
	if tx == nil {
		return errors.New("transaction not found")
	}
	credits := InboundTokenCredits(tx, depositOwner, mint)
	if len(credits) == 0 {
		return errors.New("transaction has no inbound token transfer to deposit wallet")
	}
	needRaw, err := GameToRawAmount(minGameAmount, decimals)
	if err != nil {
		return err
	}
	expectedPayer = strings.TrimSpace(expectedPayer)
	for _, credit := range credits {
		if credit.AmountRaw < needRaw {
			continue
		}
		if expectedPayer != "" && credit.FromOwner != "" && credit.FromOwner != expectedPayer {
			continue
		}
		return nil
	}
	return fmt.Errorf("payment amount/payer mismatch (need >= %d game units)", minGameAmount)
}
