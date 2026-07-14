package account

import (
	"crypto/ed25519"
	"testing"
	"time"

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

func TestPlayerHomeRequiresReconnectToDungeonOriginServer(t *testing.T) {
	st := store.New()
	cache := redisx.NewMemoryClient()
	service := NewServiceWithCache(st, cache, "test-secret", 24, 60)
	accountRow := st.UpsertAccountByWallet("resume_wallet")
	character, err := st.CreateCharacter(accountRow.ID, "Resume Hero")
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.DungeonEnter(store.DungeonEnterRequest{
		OpID: "resume-enter-1", AccountID: accountRow.ID, CharacterID: character.ID,
		ChapterID: 0, FloorID: 1, ServerID: "world-a",
	})
	if err != nil {
		t.Fatal(err)
	}

	recovery, err := service.DungeonRecovery(accountRow.ID, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !recovery.Required || recovery.DungeonRunID != run.DungeonRunID || recovery.ServerID != "world-a" {
		t.Fatalf("recovery = %+v", recovery)
	}
}

func TestDecliningDungeonReconnectClosesRunWithoutRewards(t *testing.T) {
	st := store.New()
	service := NewServiceWithCache(st, redisx.NewMemoryClient(), "test-secret", 24, 60)
	accountRow := st.UpsertAccountByWallet("decline_resume_wallet")
	character, err := st.CreateCharacter(accountRow.ID, "Decline Hero")
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateAccountSession(store.CreateSessionRequest{
		SessionID: "decline-session", AccountID: accountRow.ID, RefreshToken: "decline-refresh", ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.DungeonEnter(store.DungeonEnterRequest{
		OpID: "decline-enter-1", AccountID: accountRow.ID, CharacterID: character.ID,
		ChapterID: 0, FloorID: 1, ServerID: "world-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	beforeToken := st.Token(accountRow.ID)
	beforeSnapshot, err := st.EconomySnapshot(accountRow.ID, character.ID)
	if err != nil {
		t.Fatal(err)
	}

	decision, err := service.ResolveDungeonRecovery(accountRow.ID, character.ID, run.DungeonRunID, "abandon", "decline-session")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Status != "CANCELLED" || decision.Action != "abandon" {
		t.Fatalf("decision=%+v", decision)
	}
	recovery, err := service.DungeonRecovery(accountRow.ID, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovery.Required {
		t.Fatalf("recovery still required: %+v", recovery)
	}
	afterToken := st.Token(accountRow.ID)
	afterSnapshot, err := st.EconomySnapshot(accountRow.ID, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterToken != beforeToken || afterSnapshot.Exp != beforeSnapshot.Exp || len(afterSnapshot.LootTray) != len(beforeSnapshot.LootTray) {
		t.Fatalf("abandon granted rewards: token before=%+v after=%+v snapshot before=%+v after=%+v", beforeToken, afterToken, beforeSnapshot, afterSnapshot)
	}
}

func TestAcceptingDungeonReconnectIssuesTicketOnlyForOriginServer(t *testing.T) {
	st := store.New()
	service := NewServiceWithCache(st, redisx.NewMemoryClient(), "test-secret", 24, 60)
	accountRow := st.UpsertAccountByWallet("accept_resume_wallet")
	character, err := st.CreateCharacter(accountRow.ID, "Accept Hero")
	if err != nil {
		t.Fatal(err)
	}
	for _, serverID := range []string{"world-a", "world-b"} {
		if _, err := st.UpsertGameServer(store.GameServer{ServerID: serverID, DisplayName: serverID, Host: "127.0.0.1", Port: 7001}); err != nil {
			t.Fatal(err)
		}
	}
	_, err = st.CreateAccountSession(store.CreateSessionRequest{
		SessionID: "resume-session", AccountID: accountRow.ID, RefreshToken: "resume-refresh", ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.DungeonEnter(store.DungeonEnterRequest{
		OpID: "accept-enter-1", AccountID: accountRow.ID, CharacterID: character.ID,
		ChapterID: 0, FloorID: 1, ServerID: "world-a",
	})
	if err != nil {
		t.Fatal(err)
	}

	decision, err := service.ResolveDungeonRecovery(accountRow.ID, character.ID, run.DungeonRunID, "resume", "resume-session")
	if err != nil {
		t.Fatal(err)
	}
	if decision.Status != "RESUME_READY" || decision.ServerID != "world-a" || decision.Ticket == "" {
		t.Fatalf("decision=%+v", decision)
	}
	if _, err := service.ConsumeTicket(decision.Ticket, "world-b", "wrong-server"); err == nil {
		t.Fatal("resume ticket was accepted by a different server")
	}
	consumed, err := service.ConsumeTicket(decision.Ticket, "world-a", "resume-connection")
	if err != nil {
		t.Fatal(err)
	}
	if consumed.Online.ServerID != "world-a" || consumed.Online.CharacterID != character.ID {
		t.Fatalf("online=%+v", consumed.Online)
	}
}

func TestActiveDungeonBlocksLaunchingAnotherServer(t *testing.T) {
	st := store.New()
	service := NewServiceWithCache(st, redisx.NewMemoryClient(), "test-secret", 24, 60)
	accountRow := st.UpsertAccountByWallet("cross_server_launch_wallet")
	character, err := st.CreateCharacter(accountRow.ID, "Cross Server Hero")
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateAccountSession(store.CreateSessionRequest{
		SessionID: "cross-server-session", AccountID: accountRow.ID, RefreshToken: "cross-server-refresh", ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.DungeonEnter(store.DungeonEnterRequest{
		OpID: "cross-server-enter", AccountID: accountRow.ID, CharacterID: character.ID,
		ChapterID: 0, FloorID: 1, ServerID: "world-a",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := service.Launch(accountRow.ID, character.ID, "cross-server-session", "world-b"); err == nil {
		t.Fatal("active dungeon allowed launch into a different server")
	}
}

func TestFinishedDungeonDoesNotRemainRecoverableFromStaleRedis(t *testing.T) {
	st := store.New()
	service := NewServiceWithCache(st, redisx.NewMemoryClient(), "test-secret", 24, 60)
	accountRow := st.UpsertAccountByWallet("stale_resume_wallet")
	character, err := st.CreateCharacter(accountRow.ID, "Stale Resume Hero")
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.DungeonEnter(store.DungeonEnterRequest{
		OpID: "stale-enter", AccountID: accountRow.ID, CharacterID: character.ID,
		ChapterID: 0, FloorID: 1, ServerID: "world-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if recovery, err := service.DungeonRecovery(accountRow.ID, character.ID); err != nil || !recovery.Required {
		t.Fatalf("initial recovery=%+v err=%v", recovery, err)
	}
	if _, err := st.DungeonFinish(store.DungeonFinishRequest{
		OpID: "stale-finish", AccountID: accountRow.ID, CharacterID: character.ID,
		DungeonRunID: run.DungeonRunID, ServerID: "world-a", ChapterID: 0, FloorID: 1, Result: "defeat",
	}); err != nil {
		t.Fatal(err)
	}

	recovery, err := service.DungeonRecovery(accountRow.ID, character.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovery.Required {
		t.Fatalf("finished dungeon was returned from stale cache: %+v", recovery)
	}
}

func TestRevokedSessionCannotAbandonDungeon(t *testing.T) {
	st := store.New()
	service := NewServiceWithCache(st, redisx.NewMemoryClient(), "test-secret", 24, 60)
	accountRow := st.UpsertAccountByWallet("revoked_recovery_wallet")
	character, err := st.CreateCharacter(accountRow.ID, "Revoked Recovery Hero")
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateAccountSession(store.CreateSessionRequest{
		SessionID: "revoked-recovery-session", AccountID: accountRow.ID,
		RefreshToken: "revoked-recovery-refresh", ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.DungeonEnter(store.DungeonEnterRequest{
		OpID: "revoked-recovery-enter", AccountID: accountRow.ID, CharacterID: character.ID,
		ChapterID: 0, FloorID: 1, ServerID: "world-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.RequireActiveSession("revoked-recovery-session", accountRow.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.RevokeAccountSession("revoked-recovery-session", time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveDungeonRecovery(accountRow.ID, character.ID, run.DungeonRunID, "abandon", "revoked-recovery-session"); err == nil {
		t.Fatal("revoked session abandoned an active dungeon")
	}
	if recovery, err := st.ActiveDungeonRun(accountRow.ID, character.ID); err != nil || !recovery.Required {
		t.Fatalf("run was closed by revoked session: recovery=%+v err=%v", recovery, err)
	}
}

func TestAbandonInvalidatesPreviouslyIssuedResumeTicket(t *testing.T) {
	st := store.New()
	service := NewServiceWithCache(st, redisx.NewMemoryClient(), "test-secret", 24, 60)
	accountRow := st.UpsertAccountByWallet("resume_then_abandon_wallet")
	character, err := st.CreateCharacter(accountRow.ID, "Resume Then Abandon Hero")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertGameServer(store.GameServer{ServerID: "world-a", DisplayName: "World A", Host: "127.0.0.1", Port: 7001}); err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateAccountSession(store.CreateSessionRequest{
		SessionID: "resume-then-abandon-session", AccountID: accountRow.ID,
		RefreshToken: "resume-then-abandon-refresh", ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.DungeonEnter(store.DungeonEnterRequest{
		OpID: "resume-then-abandon-enter", AccountID: accountRow.ID, CharacterID: character.ID,
		ChapterID: 0, FloorID: 1, ServerID: "world-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	resume, err := service.ResolveDungeonRecovery(accountRow.ID, character.ID, run.DungeonRunID, "resume", "resume-then-abandon-session")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ResolveDungeonRecovery(accountRow.ID, character.ID, run.DungeonRunID, "abandon", "resume-then-abandon-session"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ConsumeTicket(resume.Ticket, "world-a", "late-resume"); err == nil {
		t.Fatal("resume ticket remained usable after dungeon abandonment")
	}
}

func testKeypair() (ed25519.PublicKey, ed25519.PrivateKey) {
	seed := []byte("fedcba9876543210fedcba9876543210")
	privateKey := ed25519.NewKeyFromSeed(seed)
	return privateKey.Public().(ed25519.PublicKey), privateKey
}
