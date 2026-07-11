package store

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flenzero/aeon-backend/internal/chain"
)

var ErrNotFound = errors.New("not found")
var ErrForbidden = errors.New("forbidden")
var ErrInsufficientBalance = errors.New("insufficient balance")

type EconomyActionRequest struct {
	OpID         string
	AccountID    int64
	CharacterID  int64
	SlotIndex    int
	Quantity     int64
	EquipmentUID string
	EquipSlot    int
}

type DungeonEnterRequest struct {
	OpID        string
	AccountID   int64
	CharacterID int64
	ChapterID   int
	FloorID     int
	IsBoss      bool
	Cost        DungeonCost
}

type DungeonFinishRequest struct {
	OpID                 string
	AccountID            int64
	CharacterID          int64
	DungeonRunID         string
	ChapterID            int
	FloorID              int
	Result               string
	Exp                  int64
	Kills                []DungeonKill
	Progress             map[string]any
	RewardPlan           DungeonRewardPlan
	EquipmentWearPoints  int
	DefaultMaxDurability int
}

type DungeonKill struct {
	EnemyID   int64  `json:"enemyId"`
	EnemyName string `json:"enemyName"`
	Quantity  int64  `json:"quantity"`
}

type DungeonCost struct {
	Gold  int64           `json:"gold"`
	Items []InventoryItem `json:"items"`
}

type DungeonRewardPlan struct {
	IsBoss      bool
	Items       []DungeonRewardGrant
	TokenReward int64
}

type DungeonRewardGrant struct {
	RewardType   string
	ItemID       string
	Quantity     int64
	EquipmentUID string
	Rarity       int
	Category     string
	AffixPoolID  string
	Affixes      []EquipmentAffix
}

type EquipmentAffix struct {
	AffixID string  `json:"affixId"`
	Stat    string  `json:"stat"`
	Value   float64 `json:"value"`
}

type DungeonRewards struct {
	Exp            int64           `json:"exp"`
	LevelsGained   int             `json:"levelsGained"`
	Level          int             `json:"level"`
	ExpToNextLevel int64           `json:"expToNextLevel"`
	TokenReward    string          `json:"tokenReward"`
	Items          []InventoryItem `json:"items"`
	EquipmentItems []EquipmentItem `json:"equipmentItems"`
}

type DungeonDiscardedRewards struct {
	Items []InventoryItem `json:"items"`
}

type DungeonResult struct {
	DungeonRunID     string                  `json:"dungeonRunId"`
	ChapterID        int                     `json:"chapterId"`
	FloorID          int                     `json:"floorId"`
	IsBoss           bool                    `json:"isBoss"`
	Status           string                  `json:"status"`
	Result           string                  `json:"result,omitempty"`
	Cost             DungeonCost             `json:"cost"`
	Rewards          DungeonRewards          `json:"rewards"`
	DiscardedRewards DungeonDiscardedRewards `json:"discardedRewards"`
	Snapshot         EconomySnapshot         `json:"snapshot"`
}

type LootActionRequest struct {
	OpID        string
	AccountID   int64
	CharacterID int64
	LootID      int64
	SlotIndex   int
	Quantity    int64
}

type ActivitySettlementRequest struct {
	OpID         string
	AccountID    int64
	CharacterID  int64
	ActivityID   string
	ActivityType string
	RewardPlan   DungeonRewardPlan
}

type ActivitySettlementResult struct {
	ActivityID   string          `json:"activityId"`
	ActivityType string          `json:"activityType"`
	Rewards      DungeonRewards  `json:"rewards"`
	Snapshot     EconomySnapshot `json:"snapshot"`
}

type BossContributeRequest struct {
	OpID         string
	AccountID    int64
	CharacterID  int64
	BossEventID  int64
	Contribution int64
}

type BossContributeResult struct {
	BossEventID  int64  `json:"bossEventId"`
	BossKey      string `json:"bossKey"`
	Contribution int64  `json:"contribution"`
}

type BossSettleRequest struct {
	OpID                 string
	AccountID            int64
	CharacterID          int64
	BossEventID          int64
	BossKey              string
	RewardPlan           DungeonRewardPlan
	EquipmentWearPoints  int
	DefaultMaxDurability int
}

type BossSettleResult struct {
	BossEventID  int64           `json:"bossEventId"`
	BossKey      string          `json:"bossKey"`
	Contribution int64           `json:"contribution"`
	Rewards      DungeonRewards  `json:"rewards"`
	Snapshot     EconomySnapshot `json:"snapshot"`
}

type BossEvent struct {
	ID        int64     `json:"id"`
	BossKey   string    `json:"bossKey"`
	Status    string    `json:"status"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	CreatedAt time.Time `json:"createdAt"`
}

type BossOpenEventRequest struct {
	OpID     string
	BossKey  string
	StartsAt time.Time
	EndsAt   time.Time
	Metadata map[string]any
}

type BossCloseEventRequest struct {
	OpID        string
	BossEventID int64
}

type BossMarkSettledRequest struct {
	OpID        string
	BossEventID int64
}

type Account struct {
	ID            int64     `json:"id"`
	Username      string    `json:"username"`
	WalletAddress string    `json:"walletAddress"`
	IsBanned      bool      `json:"isBanned"`
	CreatedAt     time.Time `json:"createdAt"`
	LastLoginAt   time.Time `json:"lastLoginAt"`
}

type WalletLoginNonce struct {
	Nonce     string    `json:"nonce"`
	Wallet    string    `json:"walletAddress"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

type Character struct {
	ID        int64     `json:"id"`
	AccountID int64     `json:"accountId"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	Deleted   bool      `json:"deleted"`
}

type GameTicket struct {
	Ticket      string    `json:"ticket"`
	AccountID   int64     `json:"accountId"`
	CharacterID int64     `json:"characterId"`
	ServerID    string    `json:"serverId"`
	SessionID   string    `json:"sessionId"`
	ExpiresAt   time.Time `json:"expiresAt"`
	Consumed    bool      `json:"consumed"`
}

type AccountToken struct {
	AccountID           int64 `json:"accountId"`
	TokenBalance        int64 `json:"tokenBalance"`
	WithdrawableBalance int64 `json:"withdrawableBalance"`
	LockedBalance       int64 `json:"lockedBalance"`
	ExternalBalance     int64 `json:"externalBalance"`
	UnlockCredit        int64 `json:"unlockCredit"`
}

type EconomySnapshot struct {
	AccountID      int64           `json:"accountId"`
	CharacterID    int64           `json:"characterId"`
	Gold           int64           `json:"gold"`
	Gems           int64           `json:"gems"`
	Stamina        int             `json:"stamina"`
	BagSlots       int             `json:"bagSlots"`
	BagExpandCount int             `json:"bagExpandCount"`
	HasLicense     bool            `json:"hasLicense"`
	Level          int             `json:"level"`
	Exp            int64           `json:"exp"`
	AccountToken   AccountToken    `json:"accountToken"`
	Inventory      []InventoryItem `json:"inventory"`
	Warehouse      []InventoryItem `json:"warehouse"`
	LootTray       []InventoryItem `json:"lootTray"`
	Equipment      []EquipmentItem `json:"equipmentItems"`
}

type InventoryItem struct {
	ID           int64  `json:"id"`
	ItemID       string `json:"itemId"`
	Quantity     int64  `json:"quantity"`
	Slot         int    `json:"slot"`
	Durability   int    `json:"durability"`
	EnhanceLevel int    `json:"enhanceLevel"`
}

type EquipmentItem struct {
	ID            int64            `json:"id"`
	EquipmentUID  string           `json:"equipmentUid"`
	ItemID        string           `json:"itemId"`
	Rarity        int              `json:"rarity"`
	EnhanceLevel  int              `json:"enhanceLevel"`
	Durability    int              `json:"durability"`
	MaxDurability int              `json:"maxDurability"`
	Status        string           `json:"status"`
	EquipSlot     int              `json:"equipSlot"`
	Slot          int              `json:"slot"`
	NFTContract   *string          `json:"nftContract"`
	NFTTokenID    *string          `json:"nftTokenId"`
	Affixes       []EquipmentAffix `json:"affixes,omitempty"`
}

type LockedGame struct {
	ID        int64     `json:"id"`
	AccountID int64     `json:"accountId"`
	Amount    int64     `json:"amount"`
	Source    string    `json:"source"`
	Status    string    `json:"status"`
	Ref       string    `json:"ref,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
	UnlockAt  time.Time `json:"unlockAt"`
}

type Withdrawal struct {
	ID          int64     `json:"id"`
	AccountID   int64     `json:"accountId"`
	Wallet      string    `json:"wallet"`
	Amount      int64     `json:"amount"`
	Status      string    `json:"status"`
	Reason      string    `json:"reason,omitempty"`
	TxHash      string    `json:"txHash,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	ProcessedAt time.Time `json:"processedAt,omitempty"`
}

type LedgerEntry struct {
	ID        int64     `json:"id"`
	AccountID int64     `json:"accountId,omitempty"`
	Kind      string    `json:"kind"`
	Amount    int64     `json:"amount,omitempty"`
	Ref       string    `json:"ref,omitempty"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type AuditEntry struct {
	ID        int64     `json:"id"`
	AdminID   string    `json:"adminId"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"createdAt"`
}

type Store struct {
	mu          sync.Mutex
	nextID      int64
	nonces      map[string]*WalletLoginNonce
	accounts    map[int64]*Account
	byWallet    map[string]int64
	characters  map[int64]*Character
	tickets     map[string]*GameTicket
	tokens      map[int64]*AccountToken
	locked      map[int64]*LockedGame
	withdrawals map[int64]*Withdrawal
	ledger      []LedgerEntry
	audits      []AuditEntry
	sessions    map[string]*AccountSession
	refresh     map[string]*RefreshTokenRecord
	servers     map[string]*GameServer
	online      map[int64]*OnlineSession
}

var Default = New()

func New() *Store {
	return &Store{
		nextID:      1000,
		nonces:      map[string]*WalletLoginNonce{},
		accounts:    map[int64]*Account{},
		byWallet:    map[string]int64{},
		characters:  map[int64]*Character{},
		tickets:     map[string]*GameTicket{},
		tokens:      map[int64]*AccountToken{},
		locked:      map[int64]*LockedGame{},
		withdrawals: map[int64]*Withdrawal{},
		sessions:    map[string]*AccountSession{},
		refresh:     map[string]*RefreshTokenRecord{},
		servers:     map[string]*GameServer{},
		online:      map[int64]*OnlineSession{},
	}
}

func (s *Store) next() int64 {
	s.nextID++
	return s.nextID
}

func (s *Store) SaveWalletNonce(row WalletLoginNonce) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if row.CreatedAt.IsZero() {
		row.CreatedAt = time.Now().UTC()
	}
	if row.Status == "" {
		row.Status = "PENDING"
	}
	s.nonces[row.Nonce] = &row
}

func (s *Store) WalletNonce(nonce, wallet string, now time.Time) (WalletLoginNonce, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.nonces[nonce]
	if !ok || row.Wallet != wallet {
		return WalletLoginNonce{}, ErrNotFound
	}
	if row.Status != "PENDING" {
		return WalletLoginNonce{}, ErrForbidden
	}
	if !row.ExpiresAt.After(now) {
		row.Status = "EXPIRED"
		return WalletLoginNonce{}, ErrForbidden
	}
	return *row, nil
}

func (s *Store) ConsumeWalletNonce(nonce, wallet string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.nonces[nonce]
	if !ok || row.Wallet != wallet {
		return ErrNotFound
	}
	if row.Status != "PENDING" || !row.ExpiresAt.After(now) {
		if row.Status == "PENDING" {
			row.Status = "EXPIRED"
		}
		return ErrForbidden
	}
	row.Status = "CONSUMED"
	return nil
}

func (s *Store) UpsertAccountByWallet(wallet string) Account {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	if id, ok := s.byWallet[wallet]; ok {
		row := s.accounts[id]
		row.LastLoginAt = now
		return *row
	}
	id := s.next()
	row := &Account{
		ID:            id,
		Username:      "wallet_" + walletSuffix(wallet),
		WalletAddress: wallet,
		CreatedAt:     now,
		LastLoginAt:   now,
	}
	s.accounts[id] = row
	s.byWallet[wallet] = id
	s.ensureTokenLocked(id)
	return *row
}

func (s *Store) Account(id int64) (Account, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.accounts[id]
	if !ok {
		return Account{}, false
	}
	return *row, true
}

func (s *Store) SetBanned(accountID int64, banned bool) error {
	return s.SetAccountBan(accountID, banned, "")
}

func (s *Store) SetAccountBan(accountID int64, banned bool, reason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.accounts[accountID]
	if !ok {
		return ErrNotFound
	}
	row.IsBanned = banned
	_ = reason
	return nil
}

func (s *Store) AdminGetAccount(accountID int64, wallet string) (AdminAccountDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var account *Account
	if accountID > 0 {
		account = s.accounts[accountID]
	} else if wallet != "" {
		if id, ok := s.byWallet[wallet]; ok {
			account = s.accounts[id]
		}
	}
	if account == nil {
		return AdminAccountDetail{}, ErrNotFound
	}
	status := "ACTIVE"
	if account.IsBanned {
		status = "BANNED"
	}
	return AdminAccountDetail{
		ID:            account.ID,
		Username:      account.Username,
		WalletAddress: account.WalletAddress,
		Status:        status,
		IsBanned:      account.IsBanned,
		CreatedAt:     account.CreatedAt,
		LastLoginAt:   account.LastLoginAt,
	}, nil
}

func (s *Store) SetAccountRiskLevel(accountID int64, riskLevel int) error {
	return adminErrRequiresPostgres("set account risk level")
}

func (s *Store) SetTradingLicense(accountID int64, granted bool) error {
	return adminErrRequiresPostgres("set trading license")
}

func (s *Store) CreateMarketRestriction(in CreateMarketRestrictionInput) (MarketRestriction, error) {
	return MarketRestriction{}, adminErrRequiresPostgres("market restriction")
}

func (s *Store) ListMarketRestrictions(accountID int64, activeOnly bool, limit, offset int) ([]MarketRestriction, error) {
	return nil, adminErrRequiresPostgres("market restriction")
}

func (s *Store) RevokeMarketRestriction(id int64, adminID, reason string) (MarketRestriction, error) {
	return MarketRestriction{}, adminErrRequiresPostgres("market restriction")
}

func (s *Store) CreateRiskEvent(in CreateRiskEventInput) (RiskEvent, error) {
	return RiskEvent{}, adminErrRequiresPostgres("risk event")
}

func (s *Store) ListRiskEvents(accountID int64, limit, offset int) ([]RiskEvent, error) {
	return nil, adminErrRequiresPostgres("risk event")
}

func (s *Store) ListAudits(limit, offset int) ([]AuditEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit = clampAdminLimit(limit)
	if offset < 0 {
		offset = 0
	}
	out := make([]AuditEntry, 0, limit)
	for i := len(s.audits) - 1 - offset; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.audits[i])
	}
	return out, nil
}

func (s *Store) AuditTarget(adminID, action, targetType, targetID, reason string) AuditEntry {
	target := targetType
	if targetID != "" {
		target = targetType + ":" + targetID
	}
	return s.Audit(adminID, action, target, reason)
}

func (s *Store) ListPaymentOrdersAdmin(filter AdminListFilter) ([]PaymentOrder, error) {
	return nil, adminErrRequiresPostgres("payment orders")
}

func (s *Store) ListNFTMintRequests(filter AdminListFilter) ([]NFTMintRequest, error) {
	return nil, adminErrRequiresPostgres("nft mint requests")
}

func (s *Store) GetHotWalletStatus(wallet string) (HotWalletStatus, error) {
	return HotWalletStatus{}, adminErrRequiresPostgres("hot wallet status")
}

func (s *Store) SetHotWalletPayoutsPaused(wallet, network, tokenMint string, paused bool) (HotWalletStatus, error) {
	return HotWalletStatus{}, adminErrRequiresPostgres("hot wallet pause")
}

func (s *Store) AdminRevokeAccountSessions(accountID int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	now := time.Now().UTC()
	for _, sess := range s.sessions {
		if sess.AccountID == accountID && sess.Status == "ACTIVE" {
			sess.Status = "REVOKED"
			sess.RevokedAt = &now
			n++
		}
	}
	return n, nil
}

func (s *Store) CreateCharacter(accountID int64, name string) (Character, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.accounts[accountID]; !ok {
		return Character{}, ErrNotFound
	}
	id := s.next()
	row := &Character{ID: id, AccountID: accountID, Name: name, CreatedAt: time.Now().UTC()}
	s.characters[id] = row
	return *row, nil
}

func (s *Store) Characters(accountID int64) []Character {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []Character{}
	for _, row := range s.characters {
		if row.AccountID == accountID && !row.Deleted {
			rows = append(rows, *row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].ID < rows[j].ID })
	return rows
}

func (s *Store) Character(accountID, characterID int64) (Character, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.characters[characterID]
	if !ok || row.AccountID != accountID || row.Deleted {
		return Character{}, false
	}
	return *row, true
}

func (s *Store) EconomySnapshot(accountID, characterID int64) (EconomySnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	character, ok := s.characters[characterID]
	if !ok || character.AccountID != accountID || character.Deleted {
		return EconomySnapshot{}, ErrNotFound
	}
	token := *s.ensureTokenLocked(accountID)
	return EconomySnapshot{
		AccountID:    accountID,
		CharacterID:  characterID,
		BagSlots:     25,
		Level:        1,
		AccountToken: token,
		Inventory:    []InventoryItem{},
		Warehouse:    []InventoryItem{},
		LootTray:     []InventoryItem{},
		Equipment:    []EquipmentItem{},
	}, nil
}

func (s *Store) WarehouseDeposit(req EconomyActionRequest) (EconomySnapshot, error) {
	return s.EconomySnapshot(req.AccountID, req.CharacterID)
}

func (s *Store) WarehouseWithdraw(req EconomyActionRequest) (EconomySnapshot, error) {
	return s.EconomySnapshot(req.AccountID, req.CharacterID)
}

func (s *Store) EquipItem(req EconomyActionRequest) (EconomySnapshot, error) {
	return s.EconomySnapshot(req.AccountID, req.CharacterID)
}

func (s *Store) UnequipItem(req EconomyActionRequest) (EconomySnapshot, error) {
	return s.EconomySnapshot(req.AccountID, req.CharacterID)
}

func (s *Store) DungeonEnter(req DungeonEnterRequest) (DungeonResult, error) {
	snapshot, err := s.EconomySnapshot(req.AccountID, req.CharacterID)
	if err != nil {
		return DungeonResult{}, err
	}
	return DungeonResult{
		DungeonRunID: "memory-" + strings.TrimSpace(req.OpID),
		ChapterID:    req.ChapterID,
		FloorID:      req.FloorID,
		IsBoss:       req.IsBoss,
		Status:       "IN_PROGRESS",
		Cost:         req.Cost,
		Rewards: DungeonRewards{
			TokenReward:    "0",
			Items:          []InventoryItem{},
			EquipmentItems: []EquipmentItem{},
		},
		DiscardedRewards: DungeonDiscardedRewards{Items: []InventoryItem{}},
		Snapshot:         snapshot,
	}, nil
}

func (s *Store) DungeonFinish(req DungeonFinishRequest) (DungeonResult, error) {
	snapshot, err := s.EconomySnapshot(req.AccountID, req.CharacterID)
	if err != nil {
		return DungeonResult{}, err
	}
	result := strings.ToLower(strings.TrimSpace(req.Result))
	status := "FAILED"
	if result == "victory" {
		status = "REWARDED"
	}
	rewardItems := []InventoryItem{}
	rewardEquipment := []EquipmentItem{}
	for index, reward := range req.RewardPlan.Items {
		if reward.RewardType == "equipment" {
			rewardEquipment = append(rewardEquipment, EquipmentItem{
				ID:           int64(index + 1),
				EquipmentUID: reward.EquipmentUID,
				ItemID:       reward.ItemID,
				Rarity:       reward.Rarity,
				Status:       "IN_LOOT_TRAY",
				EquipSlot:    -1,
				Slot:         -1,
				Affixes:      reward.Affixes,
			})
			continue
		}
		rewardItems = append(rewardItems, InventoryItem{
			ID:       int64(index + 1),
			ItemID:   reward.ItemID,
			Quantity: reward.Quantity,
			Slot:     -1,
		})
	}
	return DungeonResult{
		DungeonRunID: strings.TrimSpace(req.DungeonRunID),
		ChapterID:    req.ChapterID,
		FloorID:      req.FloorID,
		IsBoss:       req.RewardPlan.IsBoss,
		Status:       status,
		Result:       result,
		Cost:         DungeonCost{Items: []InventoryItem{}},
		Rewards: DungeonRewards{
			Exp:            req.Exp,
			Level:          snapshot.Level,
			TokenReward:    strconv.FormatInt(req.RewardPlan.TokenReward, 10),
			Items:          rewardItems,
			EquipmentItems: rewardEquipment,
		},
		DiscardedRewards: DungeonDiscardedRewards{Items: []InventoryItem{}},
		Snapshot:         snapshot,
	}, nil
}

func (s *Store) LootClaim(req LootActionRequest) (EconomySnapshot, error) {
	return s.EconomySnapshot(req.AccountID, req.CharacterID)
}

func (s *Store) LootClaimAll(req LootActionRequest) (EconomySnapshot, error) {
	return s.EconomySnapshot(req.AccountID, req.CharacterID)
}

func (s *Store) LootDiscard(req LootActionRequest) (EconomySnapshot, error) {
	return s.EconomySnapshot(req.AccountID, req.CharacterID)
}

func (s *Store) GatheringSettle(req ActivitySettlementRequest) (ActivitySettlementResult, error) {
	return s.activitySettlement(req)
}

func (s *Store) FarmingHarvest(req ActivitySettlementRequest) (ActivitySettlementResult, error) {
	return s.activitySettlement(req)
}

func (s *Store) BossContribute(req BossContributeRequest) (BossContributeResult, error) {
	if req.Contribution <= 0 {
		return BossContributeResult{}, errors.New("contribution must be positive")
	}
	return BossContributeResult{
		BossEventID:  req.BossEventID,
		BossKey:      "shadow_leviathan",
		Contribution: req.Contribution,
	}, nil
}

func (s *Store) BossContribution(accountID, bossEventID int64) (int64, string, error) {
	_ = accountID
	_ = bossEventID
	return 0, "shadow_leviathan", nil
}

func (s *Store) BossSettle(req BossSettleRequest) (BossSettleResult, error) {
	snapshot, err := s.EconomySnapshot(req.AccountID, req.CharacterID)
	if err != nil {
		return BossSettleResult{}, err
	}
	return BossSettleResult{
		BossEventID:  req.BossEventID,
		BossKey:      strings.TrimSpace(req.BossKey),
		Contribution: 0,
		Rewards: DungeonRewards{
			TokenReward:    strconv.FormatInt(req.RewardPlan.TokenReward, 10),
			Items:          []InventoryItem{},
			EquipmentItems: []EquipmentItem{},
		},
		Snapshot: snapshot,
	}, nil
}

func (s *Store) BossOpenEvent(req BossOpenEventRequest) (BossEvent, error) {
	now := time.Now().UTC()
	startsAt := req.StartsAt
	if startsAt.IsZero() {
		startsAt = now
	}
	endsAt := req.EndsAt
	if endsAt.IsZero() {
		endsAt = startsAt.Add(2 * time.Hour)
	}
	return BossEvent{
		ID:        s.next(),
		BossKey:   strings.TrimSpace(req.BossKey),
		Status:    "OPEN",
		StartsAt:  startsAt.UTC(),
		EndsAt:    endsAt.UTC(),
		CreatedAt: now,
	}, nil
}

func (s *Store) BossCloseEvent(req BossCloseEventRequest) (BossEvent, error) {
	return BossEvent{ID: req.BossEventID, Status: "SETTLING", EndsAt: time.Now().UTC()}, nil
}

func (s *Store) BossMarkSettled(req BossMarkSettledRequest) (BossEvent, error) {
	return BossEvent{ID: req.BossEventID, Status: "SETTLED"}, nil
}

func (s *Store) BossListActiveEvents() ([]BossEvent, error) {
	return []BossEvent{}, nil
}

func (s *Store) InventoryOrganize(req EconomyActionRequest, bagSlots int) (EconomySnapshot, error) {
	return s.EconomySnapshot(req.AccountID, req.CharacterID)
}

func (s *Store) WarehouseOrganize(req EconomyActionRequest, warehouseSlots int) (EconomySnapshot, error) {
	return s.EconomySnapshot(req.AccountID, req.CharacterID)
}

func (s *Store) InventoryDiscard(req InventoryDiscardRequest) (EconomySnapshot, error) {
	return s.EconomySnapshot(req.AccountID, req.CharacterID)
}

func (s *Store) Synthesize(req SynthesizeRequest) (EconomySnapshot, error) {
	return s.EconomySnapshot(req.AccountID, req.CharacterID)
}

func (s *Store) EquipmentRepair(req EquipmentRepairRequest) (EquipmentRepairResult, error) {
	return EquipmentRepairResult{}, errors.New("equipment repair requires postgres store")
}

func (s *Store) RequestNFTMint(req NFTMintRequestInput) (NFTMintRequestResult, error) {
	return NFTMintRequestResult{}, errors.New("nft mint requires postgres store")
}

func (s *Store) ConfirmNFTMint(req NFTMintConfirmInput) (NFTMintRequestResult, error) {
	return NFTMintRequestResult{}, errors.New("nft mint requires postgres store")
}

func (s *Store) CancelNFTMint(opID string, accountID, requestID int64) (NFTMintRequestResult, error) {
	return NFTMintRequestResult{}, errors.New("nft mint requires postgres store")
}

func (s *Store) ListNFTAssets(accountID int64) ([]NFTAsset, error) {
	return nil, errors.New("nft mint requires postgres store")
}

func (s *Store) MarketplaceCreateListing(req MarketplaceListRequest) (MarketplaceListResult, error) {
	return MarketplaceListResult{}, errors.New("marketplace requires postgres store")
}

func (s *Store) MarketplaceBuy(req MarketplaceBuyRequest) (MarketplaceBuyResult, error) {
	return MarketplaceBuyResult{}, errors.New("marketplace requires postgres store")
}

func (s *Store) MarketplaceCancel(req MarketplaceCancelRequest) (MarketplaceCancelResult, error) {
	return MarketplaceCancelResult{}, errors.New("marketplace requires postgres store")
}

func (s *Store) MarketplaceExpandMaterialSlots(req MarketplaceExpandSlotsRequest) (MarketplaceExpandResult, error) {
	return MarketplaceExpandResult{}, errors.New("marketplace requires postgres store")
}

func (s *Store) MarketplaceExpandWalletSlots(req MarketplaceExpandWalletRequest) (MarketplaceExpandWalletResult, error) {
	return MarketplaceExpandWalletResult{}, errors.New("marketplace requires postgres store")
}

func (s *Store) MarketplaceSubmitWalletExpandPayment(req MarketplaceSubmitPaymentRequest) (PaymentOrder, error) {
	return PaymentOrder{}, errors.New("marketplace requires postgres store")
}

func (s *Store) SubmitPaymentOrderVerified(ctx context.Context, rpc chain.RPC, cfg ChainScanConfig, req MarketplaceSubmitPaymentRequest) (PaymentOrder, error) {
	return PaymentOrder{}, errors.New("payment verification requires postgres store")
}

func (s *Store) CreateBagExpandPayment(req GrowthPaymentRequest) (GrowthPaymentResult, error) {
	return GrowthPaymentResult{}, errors.New("bag expand requires postgres store")
}

func (s *Store) CreateTradingLicensePayment(req GrowthPaymentRequest) (GrowthPaymentResult, error) {
	return GrowthPaymentResult{}, errors.New("trading license requires postgres store")
}

func (s *Store) MarketplaceSlots(accountID int64, rules MarketplaceRules) (MarketplaceSlots, error) {
	return MarketplaceSlots{}, errors.New("marketplace requires postgres store")
}

func (s *Store) MarketplaceListListings(filter MarketplaceListFilter) ([]MarketplaceListing, error) {
	return nil, errors.New("marketplace requires postgres store")
}

func (s *Store) MarketplaceMyListings(accountID int64, status string, limit, offset int) ([]MarketplaceListing, error) {
	return nil, errors.New("marketplace requires postgres store")
}

func (s *Store) ScanAndCreditDeposits(ctx context.Context, rpc chain.RPC, cfg ChainScanConfig) (DepositScanResult, error) {
	return DepositScanResult{Disabled: true, Message: "requires postgres store"}, nil
}

func (s *Store) ProcessAutoWithdrawalsWithChain(now time.Time, singleMax, userDailyMax, globalHourlyMax, globalDailyMax int64, limit int, cfg ChainScanConfig) []Withdrawal {
	return s.ProcessAutoWithdrawals(now, singleMax, userDailyMax, globalHourlyMax, globalDailyMax, limit)
}

func (s *Store) SubmitSolanaPayouts(ctx context.Context, rpc chain.RPC, cfg ChainScanConfig, limit int) (PayoutJobResult, error) {
	return PayoutJobResult{Disabled: true}, nil
}

func (s *Store) ConfirmSolanaPayouts(ctx context.Context, rpc chain.RPC, cfg ChainScanConfig, limit int) (PayoutJobResult, error) {
	return PayoutJobResult{Disabled: true}, nil
}

func (s *Store) ConfirmPaymentOrder(ctx context.Context, orderID, reason string) (PaymentOrder, error) {
	return PaymentOrder{}, errors.New("payments require postgres store")
}

func (s *Store) activitySettlement(req ActivitySettlementRequest) (ActivitySettlementResult, error) {
	snapshot, err := s.EconomySnapshot(req.AccountID, req.CharacterID)
	if err != nil {
		return ActivitySettlementResult{}, err
	}
	return ActivitySettlementResult{
		ActivityID:   strings.TrimSpace(req.ActivityID),
		ActivityType: strings.TrimSpace(req.ActivityType),
		Rewards: DungeonRewards{
			TokenReward:    strconv.FormatInt(req.RewardPlan.TokenReward, 10),
			Items:          []InventoryItem{},
			EquipmentItems: []EquipmentItem{},
		},
		Snapshot: snapshot,
	}, nil
}

func (s *Store) SaveTicket(ticket GameTicket) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tickets[ticket.Ticket] = &ticket
}

func (s *Store) ConsumeTicket(ticket, serverID string, now time.Time) (GameTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.tickets[ticket]
	if !ok {
		return GameTicket{}, ErrNotFound
	}
	if row.Consumed || row.ExpiresAt.Before(now) {
		return GameTicket{}, ErrForbidden
	}
	if row.ServerID != "" && serverID != "" && row.ServerID != serverID {
		return GameTicket{}, ErrForbidden
	}
	row.Consumed = true
	return *row, nil
}

func (s *Store) CreateAccountSession(req CreateSessionRequest) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sessions == nil {
		s.sessions = map[string]*AccountSession{}
		s.refresh = map[string]*RefreshTokenRecord{}
	}
	now := time.Now().UTC()
	row := &AccountSession{
		SessionID: req.SessionID, AccountID: req.AccountID, WalletPlugin: req.WalletPlugin,
		DeviceID: req.DeviceID, IPAddress: req.IPAddress, UserAgent: req.UserAgent,
		Status: "ACTIVE", CreatedAt: now, LastSeenAt: &now,
	}
	s.sessions[req.SessionID] = row
	s.refresh[HashToken(req.RefreshToken)] = &RefreshTokenRecord{
		TokenHash: HashToken(req.RefreshToken), AccountID: req.AccountID, SessionID: req.SessionID,
		ExpiresAt: req.ExpiresAt, CreatedAt: now,
	}
	return *row, nil
}

func (s *Store) GetAccountSession(sessionID string) (AccountSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.sessions[sessionID]
	if !ok {
		return AccountSession{}, ErrNotFound
	}
	return *row, nil
}

func (s *Store) TouchAccountSession(sessionID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.sessions[sessionID]
	if !ok || row.Status != "ACTIVE" {
		return ErrNotFound
	}
	row.LastSeenAt = &now
	return nil
}

func (s *Store) RevokeAccountSession(sessionID string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if row, ok := s.sessions[sessionID]; ok {
		row.Status = "REVOKED"
		row.RevokedAt = &now
	}
	for _, rt := range s.refresh {
		if rt.SessionID == sessionID && rt.RevokedAt == nil {
			rt.RevokedAt = &now
		}
	}
	return nil
}

func (s *Store) LookupRefreshToken(rawToken string, now time.Time) (RefreshTokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.refresh[HashToken(rawToken)]
	if !ok {
		return RefreshTokenRecord{}, ErrNotFound
	}
	if row.RevokedAt != nil || row.ExpiresAt.Before(now) {
		return RefreshTokenRecord{}, ErrForbidden
	}
	return *row, nil
}

func (s *Store) RotateRefreshToken(oldRaw, newRaw, sessionID string, accountID int64, expiresAt, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.refresh[HashToken(oldRaw)]
	if old == nil || old.SessionID != sessionID || old.RevokedAt != nil {
		return ErrForbidden
	}
	old.RevokedAt = &now
	s.refresh[HashToken(newRaw)] = &RefreshTokenRecord{
		TokenHash: HashToken(newRaw), AccountID: accountID, SessionID: sessionID,
		ExpiresAt: expiresAt, CreatedAt: now,
	}
	if sess, ok := s.sessions[sessionID]; ok {
		sess.LastSeenAt = &now
	}
	return nil
}

func (s *Store) UpsertGameServer(server GameServer) (GameServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.servers == nil {
		s.servers = map[string]*GameServer{}
	}
	now := time.Now().UTC()
	if existing, ok := s.servers[server.ServerID]; ok {
		existing.DisplayName = server.DisplayName
		existing.Region = server.Region
		existing.Host = server.Host
		existing.Port = server.Port
		existing.PublicEndpoint = server.PublicEndpoint
		existing.MaxPlayers = server.MaxPlayers
		existing.Status = server.Status
		existing.LastHeartbeatAt = &now
		return *existing, nil
	}
	if server.MaxPlayers <= 0 {
		server.MaxPlayers = 50
	}
	if server.Status == "" {
		server.Status = "ONLINE"
	}
	server.RegisteredAt = now
	server.LastHeartbeatAt = &now
	cp := server
	s.servers[server.ServerID] = &cp
	return server, nil
}

func (s *Store) HeartbeatGameServer(serverID string, onlinePlayers int, now time.Time) (GameServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.servers[serverID]
	if !ok {
		return GameServer{}, ErrNotFound
	}
	row.OnlinePlayers = onlinePlayers
	row.LastHeartbeatAt = &now
	if row.Status == "OFFLINE" {
		row.Status = "ONLINE"
	}
	return *row, nil
}

func (s *Store) ListGameServers(status string) ([]GameServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]GameServer, 0, len(s.servers))
	for _, row := range s.servers {
		if status != "" && row.Status != strings.ToUpper(status) {
			continue
		}
		out = append(out, *row)
	}
	return out, nil
}

func (s *Store) EnterOnlineSession(row OnlineSession) (OnlineSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.servers == nil || s.servers[row.ServerID] == nil {
		return OnlineSession{}, errors.New("game server is not registered")
	}
	if s.online == nil {
		s.online = map[int64]*OnlineSession{}
	}
	now := time.Now().UTC()
	row.EnteredAt = now
	row.LastSeenAt = now
	cp := row
	s.online[row.AccountID] = &cp
	s.recountOnlineLocked(row.ServerID)
	return row, nil
}

func (s *Store) TouchOnlineSession(accountID int64, connectionID string, now time.Time) (OnlineSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.online[accountID]
	if !ok {
		return OnlineSession{}, ErrNotFound
	}
	if connectionID != "" && row.ConnectionID != connectionID {
		return OnlineSession{}, ErrNotFound
	}
	row.LastSeenAt = now
	return *row, nil
}

func (s *Store) LeaveOnlineSession(accountID int64, connectionID string) (OnlineSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.online[accountID]
	if !ok {
		return OnlineSession{}, ErrNotFound
	}
	if connectionID != "" && row.ConnectionID != connectionID {
		return OnlineSession{}, ErrNotFound
	}
	cp := *row
	delete(s.online, accountID)
	s.recountOnlineLocked(cp.ServerID)
	return cp, nil
}

func (s *Store) GetOnlineSession(accountID int64) (OnlineSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.online[accountID]
	if !ok {
		return OnlineSession{}, ErrNotFound
	}
	return *row, nil
}

func (s *Store) ListOnlineByServer(serverID string) ([]OnlineSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []OnlineSession
	for _, row := range s.online {
		if row.ServerID == serverID {
			out = append(out, *row)
		}
	}
	return out, nil
}

func (s *Store) SweepStaleOnlineSessions(olderThan time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var n int64
	touched := map[string]struct{}{}
	for id, row := range s.online {
		if row.LastSeenAt.Before(olderThan) {
			touched[row.ServerID] = struct{}{}
			delete(s.online, id)
			n++
		}
	}
	for serverID := range touched {
		s.recountOnlineLocked(serverID)
	}
	return n, nil
}

func (s *Store) recountOnlineLocked(serverID string) {
	if s.servers == nil || s.servers[serverID] == nil {
		return
	}
	count := 0
	for _, row := range s.online {
		if row.ServerID == serverID {
			count++
		}
	}
	s.servers[serverID].OnlinePlayers = count
}

func (s *Store) Token(accountID int64) AccountToken {
	s.mu.Lock()
	defer s.mu.Unlock()
	return *s.ensureTokenLocked(accountID)
}

func (s *Store) GrantLocked(accountID, amount int64, source, ref string, unlockAt time.Time) (LockedGame, error) {
	if amount <= 0 {
		return LockedGame{}, errors.New("amount must be positive")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	token := s.ensureTokenLocked(accountID)
	id := s.next()
	row := &LockedGame{
		ID:        id,
		AccountID: accountID,
		Amount:    amount,
		Source:    source,
		Status:    "locked",
		Ref:       ref,
		CreatedAt: time.Now().UTC(),
		UnlockAt:  unlockAt.UTC(),
	}
	s.locked[id] = row
	token.TokenBalance += amount
	token.LockedBalance += amount
	s.addLedgerLocked(accountID, "LOCKED_TOKEN_GRANTED", amount, ref, source)
	return *row, nil
}

func (s *Store) SettleUnlocks(now time.Time, limit int) []LockedGame {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	rows := []*LockedGame{}
	for _, row := range s.locked {
		if row.Status == "locked" && !row.UnlockAt.After(now) {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].UnlockAt.Before(rows[j].UnlockAt) })
	if len(rows) > limit {
		rows = rows[:limit]
	}
	out := []LockedGame{}
	for _, row := range rows {
		token := s.ensureTokenLocked(row.AccountID)
		row.Status = "unlocked"
		token.LockedBalance -= row.Amount
		token.WithdrawableBalance += row.Amount
		out = append(out, *row)
		s.addLedgerLocked(row.AccountID, "LOCKED_TOKEN_UNLOCKED", row.Amount, row.Ref, row.Source)
	}
	return out
}

func (s *Store) CreateWithdrawal(accountID int64, amount int64, wallet string, manual bool) (Withdrawal, error) {
	if amount <= 0 {
		return Withdrawal{}, errors.New("amount must be positive")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	wallet = strings.TrimSpace(wallet)
	if wallet == "" {
		wallet = "pending_wallet_for_account_" + strconv.FormatInt(accountID, 10)
	}
	token := s.ensureTokenLocked(accountID)
	if token.WithdrawableBalance < amount {
		return Withdrawal{}, ErrInsufficientBalance
	}
	id := s.next()
	status := "queued"
	reason := ""
	if manual {
		status = "manual_review"
		reason = "manual requested"
	}
	token.WithdrawableBalance -= amount
	row := &Withdrawal{
		ID:        id,
		AccountID: accountID,
		Wallet:    wallet,
		Amount:    amount,
		Status:    status,
		Reason:    reason,
		CreatedAt: time.Now().UTC(),
	}
	s.withdrawals[id] = row
	s.addLedgerLocked(accountID, "WITHDRAWAL_REQUESTED", amount, "", status)
	return *row, nil
}

func (s *Store) ProcessAutoWithdrawals(now time.Time, singleMax, userDailyMax, globalHourlyMax, globalDailyMax int64, limit int) []Withdrawal {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 {
		limit = 20
	}
	userDaily := map[int64]int64{}
	var globalHourly int64
	var globalDaily int64
	for _, row := range s.withdrawals {
		if row.Status != "submitted" && row.Status != "confirmed" {
			continue
		}
		if sameDay(row.ProcessedAt, now) {
			userDaily[row.AccountID] += row.Amount
			globalDaily += row.Amount
		}
		if sameHour(row.ProcessedAt, now) {
			globalHourly += row.Amount
		}
	}
	rows := []*Withdrawal{}
	for _, row := range s.withdrawals {
		if row.Status == "queued" {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.Before(rows[j].CreatedAt) })
	out := []Withdrawal{}
	for _, row := range rows {
		if len(out) >= limit {
			break
		}
		switch {
		case row.Amount > singleMax:
			row.Status = "manual_review"
			row.Reason = "single limit exceeded"
		case userDaily[row.AccountID]+row.Amount > userDailyMax:
			row.Status = "manual_review"
			row.Reason = "user daily limit exceeded"
		case globalHourly+row.Amount > globalHourlyMax:
			row.Status = "queued"
			row.Reason = "global hourly limit delayed"
			continue
		case globalDaily+row.Amount > globalDailyMax:
			row.Status = "manual_review"
			row.Reason = "global daily limit exceeded"
		default:
			row.Status = "submitted"
			row.TxHash = "mock_tx_" + time.Now().UTC().Format("20060102150405")
			row.ProcessedAt = now.UTC()
			userDaily[row.AccountID] += row.Amount
			globalHourly += row.Amount
			globalDaily += row.Amount
			s.addLedgerLocked(row.AccountID, "WITHDRAWAL_SUBMITTED", row.Amount, row.TxHash, "auto")
		}
		out = append(out, *row)
	}
	return out
}

func (s *Store) ListWithdrawals(status string) []Withdrawal {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []Withdrawal{}
	for _, row := range s.withdrawals {
		if status == "" || row.Status == status {
			rows = append(rows, *row)
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.Before(rows[j].CreatedAt) })
	return rows
}

func (s *Store) ReviewWithdrawal(id int64, approve bool, adminID, reason string) (Withdrawal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.withdrawals[id]
	if !ok {
		return Withdrawal{}, ErrNotFound
	}
	if row.Status != "manual_review" {
		return Withdrawal{}, ErrForbidden
	}
	if approve {
		row.Status = "queued"
		row.Reason = "approved: " + reason
	} else {
		row.Status = "rejected"
		row.Reason = reason
		token := s.ensureTokenLocked(row.AccountID)
		token.WithdrawableBalance += row.Amount
	}
	s.audits = append(s.audits, AuditEntry{
		ID:        s.next(),
		AdminID:   adminID,
		Action:    "withdrawal_review",
		Target:    "withdrawal",
		Reason:    reason,
		CreatedAt: time.Now().UTC(),
	})
	return *row, nil
}

func (s *Store) Audit(adminID, action, target, reason string) AuditEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := AuditEntry{ID: s.next(), AdminID: adminID, Action: action, Target: target, Reason: reason, CreatedAt: time.Now().UTC()}
	s.audits = append(s.audits, row)
	return row
}

func (s *Store) Ledger(accountID int64) []LedgerEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []LedgerEntry{}
	for _, row := range s.ledger {
		if accountID == 0 || row.AccountID == accountID {
			rows = append(rows, row)
		}
	}
	return rows
}

func (s *Store) ensureTokenLocked(accountID int64) *AccountToken {
	row, ok := s.tokens[accountID]
	if ok {
		return row
	}
	row = &AccountToken{AccountID: accountID}
	s.tokens[accountID] = row
	return row
}

func (s *Store) addLedgerLocked(accountID int64, kind string, amount int64, ref, detail string) {
	s.ledger = append(s.ledger, LedgerEntry{
		ID:        s.next(),
		AccountID: accountID,
		Kind:      kind,
		Amount:    amount,
		Ref:       ref,
		Detail:    detail,
		CreatedAt: time.Now().UTC(),
	})
}

func walletSuffix(wallet string) string {
	if len(wallet) <= 8 {
		return wallet
	}
	return wallet[len(wallet)-8:]
}

func sameHour(a, b time.Time) bool {
	if a.IsZero() {
		return false
	}
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd && a.Hour() == b.Hour()
}

func sameDay(a, b time.Time) bool {
	if a.IsZero() {
		return false
	}
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
