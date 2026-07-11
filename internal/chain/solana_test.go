package chain

import (
	"crypto/ed25519"
	"encoding/base64"
	"strings"
	"testing"
)

func TestNormalizeSolanaAddressPreservesCase(t *testing.T) {
	address := testWalletAddress()

	got, err := NormalizeSolanaAddress(address)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if got != address {
		t.Fatalf("address = %s, want %s", got, address)
	}
}

func TestNormalizeSolanaAddressRejectsEVMStyleAddress(t *testing.T) {
	_, err := NormalizeSolanaAddress("0xabc")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestVerifyWalletSignatureAcceptsBase58AndBase64(t *testing.T) {
	publicKey, privateKey := testKeypair()
	wallet := EncodeBase58(publicKey)
	message := "Sign in to Aeonblight"
	signature := ed25519.Sign(privateKey, []byte(message))

	if err := VerifyWalletSignature(wallet, message, EncodeBase58(signature)); err != nil {
		t.Fatalf("verify base58: %v", err)
	}
	if err := VerifyWalletSignature(wallet, message, base64.StdEncoding.EncodeToString(signature)); err != nil {
		t.Fatalf("verify base64: %v", err)
	}
}

func TestVerifyWalletSignatureRejectsDifferentMessage(t *testing.T) {
	publicKey, privateKey := testKeypair()
	wallet := EncodeBase58(publicKey)
	signature := ed25519.Sign(privateKey, []byte("message one"))

	err := VerifyWalletSignature(wallet, "message two", EncodeBase58(signature))
	if err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("expected invalid signature, got %v", err)
	}
}

func testKeypair() (ed25519.PublicKey, ed25519.PrivateKey) {
	seed := []byte("0123456789abcdef0123456789abcdef")
	privateKey := ed25519.NewKeyFromSeed(seed)
	return privateKey.Public().(ed25519.PublicKey), privateKey
}

func testWalletAddress() string {
	publicKey, _ := testKeypair()
	return EncodeBase58(publicKey)
}
