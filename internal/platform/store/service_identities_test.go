package store

import (
	"crypto/ed25519"
	"strings"
	"testing"

	"github.com/flenzero/aeon-backend/internal/chain"
)

func TestServiceIdentityHumanTextLimitsCountCharactersNotUTF8Bytes(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	base := CreateServiceIdentityInput{
		ServiceID: "game-server-cn-01", Name: strings.Repeat("服", 100), Kind: "GAME_SERVER",
		SubjectID: strings.Repeat("区", 100), PublicKey: chain.EncodeBase58(publicKey),
		Capabilities: []string{"account.gameplay"}, CreatedBy: "bootstrap-super-admin", Reason: strings.Repeat("原", 500),
	}
	if _, err := normalizeServiceIdentityInput(base); err != nil {
		t.Fatalf("valid Unicode character boundaries rejected: %v", err)
	}

	base.Reason = strings.Repeat("原", 501)
	if _, err := normalizeServiceIdentityInput(base); err == nil {
		t.Fatal("501-character reason was accepted")
	}
}
