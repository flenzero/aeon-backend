package chain

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"strings"
)

const solanaBase58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

type Network string

const (
	NetworkSolanaDevnet  Network = "solana-devnet"
	NetworkSolanaMainnet Network = "solana-mainnet"
)

func NormalizeSolanaAddress(address string) (string, error) {
	address = strings.TrimSpace(address)
	publicKey, err := DecodeBase58(address)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return "", errors.New("walletAddress must be a Solana base58 address")
	}
	return address, nil
}

func IsLikelySolanaAddress(address string) bool {
	_, err := NormalizeSolanaAddress(address)
	return err == nil
}

func VerifyWalletSignature(walletAddress, message, signature string) error {
	walletAddress, err := NormalizeSolanaAddress(walletAddress)
	if err != nil {
		return err
	}
	publicKey, err := DecodeBase58(walletAddress)
	if err != nil {
		return err
	}
	signatureBytes, err := DecodeSignature(signature)
	if err != nil {
		return err
	}
	if len(signatureBytes) != ed25519.SignatureSize {
		return errors.New("signature must decode to 64 bytes")
	}
	if !ed25519.Verify(ed25519.PublicKey(publicKey), []byte(message), signatureBytes) {
		return errors.New("signature is invalid")
	}
	return nil
}

func DecodeSignature(signature string) ([]byte, error) {
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return nil, errors.New("signature is required")
	}
	if strings.HasPrefix(signature, "0x") || strings.HasPrefix(signature, "0X") {
		signature = signature[2:]
	}
	if decoded, err := DecodeBase58(signature); err == nil && len(decoded) == ed25519.SignatureSize {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(signature); err == nil && len(decoded) == ed25519.SignatureSize {
		return decoded, nil
	}
	if decoded, err := hex.DecodeString(signature); err == nil && len(decoded) == ed25519.SignatureSize {
		return decoded, nil
	}
	return nil, errors.New("signature must be base58, base64 or hex encoded")
}

func DecodeBase58(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("base58 value is required")
	}
	result := big.NewInt(0)
	base := big.NewInt(58)
	for _, char := range value {
		index := strings.IndexRune(solanaBase58Alphabet, char)
		if index < 0 {
			return nil, errors.New("base58 value contains invalid character")
		}
		result.Mul(result, base)
		result.Add(result, big.NewInt(int64(index)))
	}
	decoded := result.Bytes()
	for _, char := range value {
		if char != '1' {
			break
		}
		decoded = append([]byte{0}, decoded...)
	}
	return decoded, nil
}

func EncodeBase58(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	value := new(big.Int).SetBytes(data)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)
	var encoded []byte
	for value.Cmp(zero) > 0 {
		value.DivMod(value, base, mod)
		encoded = append(encoded, solanaBase58Alphabet[mod.Int64()])
	}
	for _, b := range data {
		if b != 0 {
			break
		}
		encoded = append(encoded, '1')
	}
	for i, j := 0, len(encoded)-1; i < j; i, j = i+1, j-1 {
		encoded[i], encoded[j] = encoded[j], encoded[i]
	}
	return string(encoded)
}
