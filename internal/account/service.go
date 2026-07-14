package account

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/chain"
	"github.com/flenzero/aeon-backend/internal/platform/redisx"
	"github.com/flenzero/aeon-backend/internal/platform/security"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type Service struct {
	store           store.Repository
	redis           redisx.Client
	jwtSecret       string
	sessionTTLHours int
	onlineTTLSec    int
}

func NewService(st store.Repository, jwtSecret string) *Service {
	return NewServiceWithCache(st, redisx.NopClient{}, jwtSecret, 168, 90)
}

func NewServiceWithCache(st store.Repository, cache redisx.Client, jwtSecret string, sessionTTLHours, onlineTTLSec int) *Service {
	if cache == nil {
		cache = redisx.NopClient{}
	}
	if sessionTTLHours <= 0 {
		sessionTTLHours = 168
	}
	if onlineTTLSec <= 0 {
		onlineTTLSec = 90
	}
	return &Service{
		store:           st,
		redis:           cache,
		jwtSecret:       jwtSecret,
		sessionTTLHours: sessionTTLHours,
		onlineTTLSec:    onlineTTLSec,
	}
}

type WalletLoginResult struct {
	Account      store.Account `json:"account"`
	AccessToken  string        `json:"accessToken"`
	RefreshToken string        `json:"refreshToken"`
	SessionID    string        `json:"sessionId"`
	WalletPlugin string        `json:"walletPlugin,omitempty"`
	ExpiresAt    time.Time     `json:"expiresAt"`
}

type LaunchResult struct {
	Status    string    `json:"status"`
	Ticket    string    `json:"ticket"`
	ExpiresAt time.Time `json:"expiresAt"`
	ServerID  string    `json:"serverId,omitempty"`
}

type DungeonRecoveryDecision struct {
	Action       string    `json:"action"`
	Status       string    `json:"status"`
	DungeonRunID string    `json:"dungeonRunId"`
	ServerID     string    `json:"serverId,omitempty"`
	Ticket       string    `json:"ticket,omitempty"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
}

type WalletNonceResult struct {
	Nonce     string    `json:"nonce"`
	Message   string    `json:"message"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type LoginMeta struct {
	WalletPlugin string
	DeviceID     string
	IPAddress    string
	UserAgent    string
}

func (s *Service) WalletNonce(wallet string) (WalletNonceResult, error) {
	normalized, err := chain.NormalizeSolanaAddress(wallet)
	if err != nil {
		return WalletNonceResult{}, err
	}
	nonce := security.RandomToken("nonce")
	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	message := "Sign in to Aeonblight\nWallet: " + normalized + "\nNonce: " + nonce + "\nThis signature does not authorize any transaction."
	s.store.SaveWalletNonce(store.WalletLoginNonce{
		Nonce:     nonce,
		Wallet:    normalized,
		Message:   message,
		Status:    "PENDING",
		ExpiresAt: expiresAt,
		CreatedAt: time.Now().UTC(),
	})
	return WalletNonceResult{Nonce: nonce, Message: message, ExpiresAt: expiresAt}, nil
}

func (s *Service) WalletLogin(wallet, nonce, signature string, meta LoginMeta) (WalletLoginResult, error) {
	normalized, err := chain.NormalizeSolanaAddress(wallet)
	if err != nil {
		return WalletLoginResult{}, err
	}
	now := time.Now().UTC()
	loginNonce, err := s.store.WalletNonce(strings.TrimSpace(nonce), normalized, now)
	if err != nil {
		return WalletLoginResult{}, errors.New("wallet nonce is invalid or expired")
	}
	if err := chain.VerifyWalletSignature(normalized, loginNonce.Message, signature); err != nil {
		return WalletLoginResult{}, err
	}
	if err := s.store.ConsumeWalletNonce(loginNonce.Nonce, normalized, now); err != nil {
		return WalletLoginResult{}, errors.New("wallet nonce is invalid or already consumed")
	}
	account := s.store.UpsertAccountByWallet(normalized)
	if account.IsBanned {
		return WalletLoginResult{}, errors.New("account is banned")
	}
	return s.issueSession(account, meta.WalletPlugin, meta.DeviceID, meta.IPAddress, meta.UserAgent)
}

func (s *Service) CreateCharacter(accountID int64, name string) (store.Character, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Player"
	}
	return s.store.CreateCharacter(accountID, name)
}

func (s *Service) Launch(accountID, characterID int64, sessionID, serverID string) (LaunchResult, error) {
	account, ok := s.store.Account(accountID)
	if !ok {
		return LaunchResult{}, store.ErrNotFound
	}
	if account.IsBanned {
		return LaunchResult{}, errors.New("account is banned")
	}
	if _, ok := s.store.Character(accountID, characterID); !ok {
		return LaunchResult{}, errors.New("character not found")
	}
	if recovery, err := s.store.ActiveDungeonRun(accountID, characterID); err == nil {
		originServerID := strings.TrimSpace(recovery.ServerID)
		if originServerID == "" {
			return LaunchResult{}, errors.New("active dungeon origin server is unavailable; abandon the dungeon first")
		}
		if strings.TrimSpace(serverID) != originServerID {
			return LaunchResult{}, fmt.Errorf("active dungeon must reconnect to server %s or be abandoned", originServerID)
		}
	} else if !errors.Is(err, store.ErrNotFound) {
		return LaunchResult{}, err
	}
	if err := s.RequireActiveSession(sessionID, accountID); err != nil {
		return LaunchResult{}, err
	}
	ticket := security.RandomToken("ticket")
	expiresAt := time.Now().UTC().Add(90 * time.Second)
	s.store.SaveTicket(store.GameTicket{
		Ticket:      ticket,
		AccountID:   accountID,
		CharacterID: characterID,
		ServerID:    strings.TrimSpace(serverID),
		SessionID:   sessionID,
		ExpiresAt:   expiresAt,
	})
	return LaunchResult{Status: "ready", Ticket: ticket, ExpiresAt: expiresAt, ServerID: serverID}, nil
}

type ConsumeTicketResult struct {
	Ticket store.GameTicket    `json:"ticket"`
	Online store.OnlineSession `json:"online"`
}

func (s *Service) ConsumeTicket(ticket, serverID, connectionID string) (ConsumeTicketResult, error) {
	row, err := s.store.ConsumeTicket(strings.TrimSpace(ticket), strings.TrimSpace(serverID), time.Now().UTC())
	if err != nil {
		return ConsumeTicketResult{}, err
	}
	sid := strings.TrimSpace(serverID)
	if sid == "" {
		sid = row.ServerID
	}
	if sid == "" {
		return ConsumeTicketResult{}, errors.New("serverId is required to enter online session")
	}
	online, err := s.EnterOnline(row.AccountID, row.CharacterID, row.SessionID, sid, connectionID)
	if err != nil {
		return ConsumeTicketResult{}, err
	}
	return ConsumeTicketResult{Ticket: row, Online: online}, nil
}
