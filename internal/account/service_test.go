package account

import (
	"crypto/ed25519"
	"testing"

	"github.com/flenzero/aeon-backend/internal/chain"
	"github.com/flenzero/aeon-backend/internal/platform/redisx"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func TestWalletLoginRequiresValidSolanaSignature(t *testing.T) {
	publicKey, privateKey := testKeypair()
	wallet := chain.EncodeBase58(publicKey)
	st := store.New()
	service := NewServiceWithCache(st, redisx.NewMemoryClient(), "test-secret", 24, 60)

	nonce, err := service.WalletNonce(wallet)
	if err != nil {
		t.Fatalf("wallet nonce: %v", err)
	}
	signature := chain.EncodeBase58(ed25519.Sign(privateKey, []byte(nonce.Message)))

	result, err := service.WalletLogin(wallet, nonce.Nonce, signature, LoginMeta{WalletPlugin: "phantom"})
	if err != nil {
		t.Fatalf("wallet login: %v", err)
	}
	if result.Account.WalletAddress != wallet {
		t.Fatalf("wallet = %s, want %s", result.Account.WalletAddress, wallet)
	}
	if result.AccessToken == "" || result.RefreshToken == "" || result.SessionID == "" {
		t.Fatal("expected issued tokens and session")
	}

	if _, err := service.WalletLogin(wallet, nonce.Nonce, signature, LoginMeta{WalletPlugin: "phantom"}); err == nil {
		t.Fatal("expected consumed nonce to fail")
	}
}

func TestWalletLoginRejectsBadSignatureWithoutConsumingNonce(t *testing.T) {
	publicKey, privateKey := testKeypair()
	wallet := chain.EncodeBase58(publicKey)
	st := store.New()
	service := NewService(st, "test-secret")

	nonce, err := service.WalletNonce(wallet)
	if err != nil {
		t.Fatalf("wallet nonce: %v", err)
	}
	badSignature := chain.EncodeBase58(ed25519.Sign(privateKey, []byte("wrong message")))
	if _, err := service.WalletLogin(wallet, nonce.Nonce, badSignature, LoginMeta{WalletPlugin: "phantom"}); err == nil {
		t.Fatal("expected bad signature to fail")
	}

	goodSignature := chain.EncodeBase58(ed25519.Sign(privateKey, []byte(nonce.Message)))
	if _, err := service.WalletLogin(wallet, nonce.Nonce, goodSignature, LoginMeta{WalletPlugin: "phantom"}); err != nil {
		t.Fatalf("expected nonce to remain usable after bad signature: %v", err)
	}
}

func TestSessionRefreshLogoutAndOnlineFlow(t *testing.T) {
	publicKey, privateKey := testKeypair()
	wallet := chain.EncodeBase58(publicKey)
	st := store.New()
	cache := redisx.NewMemoryClient()
	service := NewServiceWithCache(st, cache, "test-secret", 24, 60)

	nonce, err := service.WalletNonce(wallet)
	if err != nil {
		t.Fatal(err)
	}
	sig := chain.EncodeBase58(ed25519.Sign(privateKey, []byte(nonce.Message)))
	login, err := service.WalletLogin(wallet, nonce.Nonce, sig, LoginMeta{WalletPlugin: "phantom", DeviceID: "dev-1"})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	refreshed, err := service.Refresh(login.RefreshToken)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if refreshed.SessionID != login.SessionID || refreshed.RefreshToken == login.RefreshToken {
		t.Fatalf("refresh=%+v", refreshed)
	}
	if _, err := service.Refresh(login.RefreshToken); err == nil {
		t.Fatal("old refresh should be revoked")
	}

	char, err := service.CreateCharacter(login.Account.ID, "Hero")
	if err != nil {
		t.Fatal(err)
	}
	server, err := service.RegisterGameServer(store.GameServer{
		ServerID: "world-1", DisplayName: "World 1", Host: "127.0.0.1", Port: 7777, MaxPlayers: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	launch, err := service.Launch(login.Account.ID, char.ID, login.SessionID, server.ServerID)
	if err != nil {
		t.Fatalf("launch: %v", err)
	}
	consumed, err := service.ConsumeTicket(launch.Ticket, server.ServerID, "conn-1")
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if consumed.Online.ServerID != server.ServerID || consumed.Online.ConnectionID != "conn-1" {
		t.Fatalf("online=%+v", consumed.Online)
	}
	if _, err := service.OnlineHeartbeat(login.Account.ID, "conn-1"); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	listed, err := service.ListOnline(server.ServerID)
	if err != nil || len(listed) != 1 {
		t.Fatalf("list online=%v err=%v", listed, err)
	}
	if _, err := service.LeaveOnline(login.Account.ID, "conn-1"); err != nil {
		t.Fatalf("leave: %v", err)
	}
	if _, err := service.GetOnline(login.Account.ID); err == nil {
		t.Fatal("expected offline")
	}

	if _, err := service.Logout(login.SessionID, refreshed.RefreshToken); err != nil {
		t.Fatalf("logout: %v", err)
	}
	if err := service.RequireActiveSession(login.SessionID, login.Account.ID); err == nil {
		t.Fatal("expected revoked session")
	}
}

func testKeypair() (ed25519.PublicKey, ed25519.PrivateKey) {
	seed := []byte("fedcba9876543210fedcba9876543210")
	privateKey := ed25519.NewKeyFromSeed(seed)
	return privateKey.Public().(ed25519.PublicKey), privateKey
}
