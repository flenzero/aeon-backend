package account

import (
	"errors"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/flenzero/aeon-backend/internal/chain"
	"github.com/flenzero/aeon-backend/internal/economy/rules"
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
	economyRules    *rules.Config
	economyRulesErr error
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

func (s *Service) LoadEconomyRules(configDir string) *Service {
	configDir = strings.TrimSpace(configDir)
	if configDir == "" {
		return s
	}
	economyRules, err := rules.LoadDir(configDir)
	s.economyRules = economyRules
	s.economyRulesErr = err
	return s
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
	Status        string    `json:"status"`
	Ticket        string    `json:"ticket"`
	ExpiresIn     int64     `json:"expiresIn"`
	ExpiresAt     time.Time `json:"expiresAt"`
	ServerID      string    `json:"serverId"`
	Host          string    `json:"host"`
	Port          int       `json:"port"`
	WalletAddress string    `json:"walletAddress"`
	WalletPlugin  string    `json:"walletPlugin,omitempty"`
	GameURL       string    `json:"gameUrl,omitempty"`
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
	return s.CreateCharacterWithAppearance(accountID, name, nil)
}

func (s *Service) CreateCharacterWithAppearance(accountID int64, name string, appearance map[string]any) (store.Character, error) {
	name = strings.TrimSpace(name)
	if !validCharacterName(name) {
		return store.Character{}, errors.New("character name must be 2-12 Chinese characters, letters, digits, or underscores")
	}
	return s.store.CreateCharacterWithAppearance(accountID, name, appearance)
}

func (s *Service) Characters(accountID int64) []store.Character {
	rows := s.store.Characters(accountID)
	for index := range rows {
		rows[index] = s.resolveCharacterEquipment(rows[index])
	}
	return rows
}

func (s *Service) DeleteCharacter(accountID, characterID int64) error {
	return s.store.DeleteCharacter(accountID, characterID)
}

func (s *Service) PlayerProfile(accountID, characterID int64) (store.PlayerState, store.EconomySnapshot, error) {
	player, err := s.store.PlayerProfile(accountID, characterID)
	if err != nil {
		return store.PlayerState{}, store.EconomySnapshot{}, err
	}
	economy, err := s.store.EconomySnapshot(accountID, characterID)
	if err != nil {
		return store.PlayerState{}, store.EconomySnapshot{}, err
	}
	return player, s.resolveEconomySnapshot(economy), nil
}

func (s *Service) resolveCharacterEquipment(character store.Character) store.Character {
	if s.economyRules == nil || s.economyRulesErr != nil {
		return character
	}
	for index := range character.EquipmentItems {
		resolved, err := s.economyRules.ResolveEquipmentItem(character.EquipmentItems[index])
		if err == nil {
			character.EquipmentItems[index] = resolved
		}
	}
	return character
}

func (s *Service) resolveEconomySnapshot(snapshot store.EconomySnapshot) store.EconomySnapshot {
	if s.economyRules == nil || s.economyRulesErr != nil {
		return snapshot
	}
	for index := range snapshot.Equipment {
		resolved, err := s.economyRules.ResolveEquipmentItem(snapshot.Equipment[index])
		if err == nil {
			snapshot.Equipment[index] = resolved
		}
	}
	return snapshot
}

func (s *Service) SavePlayerState(req store.PlayerSaveRequest) error {
	return s.store.SavePlayerState(req)
}

func (s *Service) ClearProgressLeaderboard(limit int) (store.ClearProgressLeaderboard, error) {
	return s.store.ClearProgressLeaderboard(limit)
}

func (s *Service) WeeklyScoreLeaderboard(now time.Time, limit int) (store.WeeklyScoreLeaderboard, error) {
	return s.store.WeeklyScoreLeaderboard(now, limit)
}

func validCharacterName(name string) bool {
	runes := []rune(name)
	if len(runes) < 2 || len(runes) > 12 {
		return false
	}
	for _, r := range runes {
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Han, r) {
			continue
		}
		return false
	}
	return true
}

func (s *Service) Launch(accountID int64, sessionID, serverID string) (LaunchResult, error) {
	account, ok := s.store.Account(accountID)
	if !ok {
		return LaunchResult{}, store.ErrNotFound
	}
	if account.IsBanned {
		return LaunchResult{}, errors.New("account is banned")
	}
	if err := s.RequireActiveSession(sessionID, accountID); err != nil {
		return LaunchResult{}, err
	}
	session, err := s.store.GetAccountSession(strings.TrimSpace(sessionID))
	if err != nil {
		return LaunchResult{}, err
	}
	server, err := s.selectLaunchServer(serverID)
	if err != nil {
		return LaunchResult{}, err
	}
	ticket := security.RandomToken("ticket")
	expiresIn := int64(90)
	expiresAt := time.Now().UTC().Add(time.Duration(expiresIn) * time.Second)
	s.store.SaveTicket(store.GameTicket{
		Ticket:    ticket,
		AccountID: accountID,
		ServerID:  server.ServerID,
		SessionID: strings.TrimSpace(sessionID),
		ExpiresAt: expiresAt,
	})
	return LaunchResult{
		Status: "ready", Ticket: ticket, ExpiresIn: expiresIn, ExpiresAt: expiresAt,
		ServerID:      server.ServerID,
		Host:          server.Host,
		Port:          server.Port,
		WalletAddress: account.WalletAddress, WalletPlugin: strings.TrimSpace(session.WalletPlugin),
	}, nil
}

type ConsumeTicketResult struct {
	AccountID     int64  `json:"accountId"`
	WalletAddress string `json:"walletAddress"`
	WalletPlugin  string `json:"walletPlugin,omitempty"`
	SessionID     string `json:"sessionId"`
	ServerID      string `json:"serverId"`
}

func (s *Service) ConsumeTicket(ticket, serverID string) (ConsumeTicketResult, error) {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return ConsumeTicketResult{}, errors.New("serverId is required")
	}
	row, err := s.store.ConsumeTicket(strings.TrimSpace(ticket), serverID, time.Now().UTC())
	if err != nil {
		return ConsumeTicketResult{}, err
	}
	if row.ServerID != serverID {
		return ConsumeTicketResult{}, store.ErrForbidden
	}
	account, ok := s.store.Account(row.AccountID)
	if !ok || account.IsBanned {
		return ConsumeTicketResult{}, errors.New("account unavailable")
	}
	session, err := s.store.GetAccountSession(row.SessionID)
	if err != nil {
		return ConsumeTicketResult{}, err
	}
	return ConsumeTicketResult{
		AccountID: row.AccountID, WalletAddress: account.WalletAddress, WalletPlugin: strings.TrimSpace(session.WalletPlugin),
		SessionID: row.SessionID, ServerID: row.ServerID,
	}, nil
}

type PublicServerView struct {
	ServerID    string `json:"serverId"`
	Name        string `json:"name"`
	CurPlayers  int    `json:"curPlayers"`
	MaxPlayers  int    `json:"maxPlayers"`
	Status      string `json:"status"`
	QueueLength int    `json:"queueLength"`
	Region      string `json:"region,omitempty"`
}

type HomeStatsResult struct {
	OnlinePlayers        int       `json:"onlinePlayers"`
	MonthlyActivePlayers int       `json:"monthlyActivePlayers"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

const launchServerHeartbeatWindow = 30 * time.Second

func (s *Service) PublicGameServers(onlineOnly bool) ([]PublicServerView, error) {
	rows, err := s.store.ListGameServers("")
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	counts, err := s.onlinePresenceCounts(rows, now)
	if err != nil {
		return nil, err
	}
	out := make([]PublicServerView, 0, len(rows))
	for _, row := range rows {
		view := publicServerView(row, now, counts[row.ServerID])
		if onlineOnly && view.Status != "online" {
			continue
		}
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return publicStatusRank(out[i].Status) < publicStatusRank(out[j].Status)
		}
		if out[i].CurPlayers != out[j].CurPlayers {
			return out[i].CurPlayers < out[j].CurPlayers
		}
		return out[i].ServerID < out[j].ServerID
	})
	return out, nil
}

func (s *Service) HomeStats() (HomeStatsResult, error) {
	now := time.Now().UTC()
	rows, err := s.store.ListGameServers("")
	if err != nil {
		return HomeStatsResult{}, err
	}
	counts, err := s.onlinePresenceCounts(rows, now)
	if err != nil {
		return HomeStatsResult{}, err
	}
	totalOnline := 0
	for _, count := range counts {
		totalOnline += count
	}
	mau, err := s.store.CountMonthlyActiveAccounts(now.AddDate(0, 0, -30))
	if err != nil {
		return HomeStatsResult{}, err
	}
	return HomeStatsResult{
		OnlinePlayers:        totalOnline,
		MonthlyActivePlayers: mau,
		UpdatedAt:            now,
	}, nil
}

func (s *Service) selectLaunchServer(serverID string) (store.GameServer, error) {
	serverID = strings.TrimSpace(serverID)
	rows, err := s.store.ListGameServers("")
	if err != nil {
		return store.GameServer{}, err
	}
	now := time.Now().UTC()
	counts, err := s.onlinePresenceCounts(rows, now)
	if err != nil {
		return store.GameServer{}, err
	}
	var candidates []store.GameServer
	for _, row := range rows {
		status := publicServerStatus(row, now, counts[row.ServerID])
		if serverID != "" {
			if row.ServerID == serverID {
				if status != "online" {
					return store.GameServer{}, errors.New("serverId is unavailable")
				}
				return row, nil
			}
			continue
		}
		if status == "online" {
			candidates = append(candidates, row)
		}
	}
	if serverID != "" {
		return store.GameServer{}, errors.New("serverId is unavailable")
	}
	if len(candidates) == 0 {
		return store.GameServer{}, errors.New("no available game server")
	}
	sort.Slice(candidates, func(i, j int) bool {
		left := counts[candidates[i].ServerID]
		right := counts[candidates[j].ServerID]
		if left != right {
			return left < right
		}
		return candidates[i].ServerID < candidates[j].ServerID
	})
	return candidates[0], nil
}

func publicServerView(server store.GameServer, now time.Time, curPlayers int) PublicServerView {
	return PublicServerView{
		ServerID: server.ServerID, Name: server.DisplayName, CurPlayers: curPlayers,
		MaxPlayers: server.MaxPlayers, Status: publicServerStatus(server, now, curPlayers), QueueLength: 0, Region: server.Region,
	}
}

func publicServerStatus(server store.GameServer, now time.Time, curPlayers int) string {
	live := server.LastHeartbeatAt != nil && !server.LastHeartbeatAt.Before(now.Add(-launchServerHeartbeatWindow))
	if strings.ToUpper(strings.TrimSpace(server.Status)) != "ONLINE" || !live {
		return "offline"
	}
	if server.MaxPlayers > 0 && curPlayers >= publicFullThreshold(server.MaxPlayers) {
		return "full"
	}
	return "online"
}

func publicFullThreshold(maxPlayers int) int {
	if maxPlayers <= 0 {
		return 0
	}
	return (maxPlayers*95 + 99) / 100
}

func publicStatusRank(status string) int {
	switch status {
	case "online":
		return 0
	case "full":
		return 1
	default:
		return 2
	}
}
