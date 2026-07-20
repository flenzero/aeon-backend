package store

import (
	"context"
	"encoding/json"
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
	ServerID    string
	ChapterID   int
	FloorID     int
	IsBoss      bool
	Cost        DungeonCost
}

type DungeonRunRecovery struct {
	Required     bool      `json:"required"`
	Status       string    `json:"status,omitempty"`
	DungeonRunID string    `json:"dungeonRunId,omitempty"`
	AccountID    int64     `json:"accountId,omitempty"`
	CharacterID  int64     `json:"characterId,omitempty"`
	ServerID     string    `json:"serverId,omitempty"`
	ChapterID    int       `json:"chapterId,omitempty"`
	FloorID      int       `json:"floorId,omitempty"`
	StartedAt    time.Time `json:"startedAt,omitempty"`
}

type DungeonFinishRequest struct {
	OpID                 string
	AccountID            int64
	CharacterID          int64
	DungeonRunID         string
	ServerID             string
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
	// AffixID, InstanceID and EnhanceHits are the persisted semantic form for
	// Aeonblight equipment. Value is retained only to decode legacy equipment;
	// newly-created equipment must not persist a rolled numeric value.
	AffixID     string  `json:"affixId"`
	InstanceID  string  `json:"instanceId,omitempty"`
	EnhanceHits int     `json:"enhanceHits,omitempty"`
	Stat        string  `json:"stat,omitempty"`
	Value       float64 `json:"value,omitempty"`
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
	ID             int64           `json:"id"`
	AccountID      int64           `json:"accountId"`
	Name           string          `json:"name"`
	SlotIndex      int             `json:"slotIndex"`
	Level          int             `json:"level"`
	Appearance     map[string]any  `json:"appearance"`
	EquipmentItems []EquipmentItem `json:"equipmentItems,omitempty"`
	CreatedAt      time.Time       `json:"createdAt"`
	LastPlayed     time.Time       `json:"lastPlayed"`
	HasLastPlayed  bool            `json:"hasLastPlayed"`
	Deleted        bool            `json:"deleted"`
}

type PlayerState struct {
	AccountID     int64          `json:"accountId"`
	CharacterID   int64          `json:"characterId"`
	PosX          float64        `json:"posX"`
	PosY          float64        `json:"posY"`
	CurrentMap    string         `json:"currentMap"`
	PlayTimeSec   int64          `json:"playTimeSec"`
	Hunger        float64        `json:"hunger"`
	Level         int            `json:"level"`
	Exp           int64          `json:"exp"`
	Appearance    map[string]any `json:"appearanceJson"`
	CharacterName string         `json:"characterName"`
	LastPlayed    time.Time      `json:"lastPlayed"`
	HasLastPlayed bool           `json:"hasLastPlayed"`
}

type PlayerSaveRequest struct {
	AccountID   int64
	CharacterID int64
	PosX        float64
	PosY        float64
	CurrentMap  string
	PlayTimeSec int64
	Hunger      float64
}

type characterStateRecord struct {
	PosX          float64
	PosY          float64
	CurrentMap    string
	PlayTimeSec   int64
	Hunger        float64
	LastPlayed    time.Time
	HasLastPlayed bool
}

type GameTicket struct {
	Ticket      string    `json:"ticket"`
	AccountID   int64     `json:"accountId"`
	CharacterID int64     `json:"characterId,omitempty"`
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

type ShopBuyRequest struct {
	OpID             string
	AccountID        int64
	CharacterID      int64
	ShopID           string
	ItemID           string
	Quantity         int64
	MysterySlotIndex int
	ShopSlotIndex    int
	ShopDailyLimit   int64
	ShopBusinessDate string
	GoldCost         int64
	TokenCost        int64
	ReceiverWallet   string
	GrantGold        int64
	RewardPlan       DungeonRewardPlan
	ConfigSnapshot   any
}

type ShopBuyResult struct {
	Order    PaymentOrder    `json:"order,omitempty"`
	Snapshot EconomySnapshot `json:"snapshot,omitempty"`
}

type ShopCatalog struct {
	ShopID         string            `json:"shopId"`
	DisplayName    string            `json:"displayName"`
	CharacterID    int64             `json:"characterId"`
	CharacterLevel int               `json:"characterLevel"`
	BusinessDate   string            `json:"businessDate"`
	NextResetAt    time.Time         `json:"nextResetAt"`
	Items          []ShopCatalogItem `json:"items"`
}

type ShopCatalogItem struct {
	SlotIndex      int    `json:"slotIndex"`
	ItemID         string `json:"itemId"`
	DisplayName    string `json:"displayName"`
	Quantity       int64  `json:"quantity"`
	Rarity         int    `json:"rarity,omitempty"`
	BuyCurrency    int    `json:"buyCurrency"`
	BuyPrice       int64  `json:"buyPrice"`
	MinLevel       int    `json:"minLevel,omitempty"`
	MaxLevel       int    `json:"maxLevel,omitempty"`
	DailyLimit     int64  `json:"dailyLimit"`
	PurchasedToday int64  `json:"purchasedToday"`
	RemainingToday int64  `json:"remainingToday"`
	Available      bool   `json:"available"`
}

type ShopSellRequest struct {
	OpID         string
	AccountID    int64
	CharacterID  int64
	ShopID       string
	SlotIndex    int
	Quantity     int64
	EquipmentUID string
	GoldCredit   int64
}

type ShopSellResult struct {
	Snapshot EconomySnapshot `json:"snapshot"`
}

type MysteryShopBoard struct {
	ShopID                     string             `json:"shopId"`
	CharacterID                int64              `json:"characterId"`
	CharacterLevel             int                `json:"characterLevel"`
	UnlockedSlots              int                `json:"unlockedSlots"`
	MaxSlots                   int                `json:"maxSlots"`
	NextManualRefreshTokenCost int64              `json:"nextManualRefreshTokenCost"`
	SoldOut                    bool               `json:"soldOut"`
	NextFreeRefreshAt          time.Time          `json:"nextFreeRefreshAt"`
	FreeRefreshAvailable       bool               `json:"freeRefreshAvailable"`
	GeneratedAt                time.Time          `json:"generatedAt"`
	Offers                     []MysteryShopOffer `json:"offers"`
}

type MysteryShopOffer struct {
	SlotIndex   int    `json:"slotIndex"`
	ItemID      string `json:"itemId"`
	Quantity    int64  `json:"quantity"`
	Rarity      int    `json:"rarity,omitempty"`
	GoldPrice   int64  `json:"goldPrice,omitempty"`
	TokenPrice  int64  `json:"tokenPrice,omitempty"`
	DiscountBps int    `json:"discountBps"`
	Purchased   bool   `json:"purchased"`
}

type MysteryShopBoardState struct {
	ShopID            string             `json:"shopId"`
	CharacterID       int64              `json:"characterId"`
	NextFreeRefreshAt time.Time          `json:"nextFreeRefreshAt"`
	GeneratedAt       time.Time          `json:"generatedAt"`
	Offers            []MysteryShopOffer `json:"offers"`
}

type MysteryShopRefreshRequest struct {
	OpID              string
	AccountID         int64
	CharacterID       int64
	ShopID            string
	TokenCost         int64
	NextFreeRefreshAt time.Time
	GeneratedAt       time.Time
	Offers            []MysteryShopOffer
}

type ClearProgressLeaderboard struct {
	Intro     string                         `json:"intro"`
	Items     []ClearProgressLeaderboardItem `json:"items"`
	UpdatedAt time.Time                      `json:"updatedAt"`
}

type ClearProgressLeaderboardItem struct {
	Rank             int        `json:"rank"`
	CharacterID      int64      `json:"characterId"`
	CharacterName    string     `json:"characterName"`
	HighestChapterID int        `json:"highestChapterId"`
	HighestFloorID   int        `json:"highestFloorId"`
	FirstReachedAt   *time.Time `json:"firstReachedAt"`
	ClearCount       int64      `json:"clearCount"`
}

type WeeklyScoreLeaderboard struct {
	Intro     string                       `json:"intro"`
	Period    LeaderboardPeriod            `json:"period"`
	Items     []WeeklyScoreLeaderboardItem `json:"items"`
	UpdatedAt time.Time                    `json:"updatedAt"`
}

type LeaderboardPeriod struct {
	PeriodID string    `json:"periodId"`
	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt"`
}

type WeeklyScoreLeaderboardItem struct {
	Rank          int        `json:"rank"`
	CharacterID   int64      `json:"characterId"`
	CharacterName string     `json:"characterName"`
	Score         int64      `json:"score"`
	ClearCount    int64      `json:"clearCount"`
	LastScoredAt  *time.Time `json:"lastScoredAt"`
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
	WeaponType    int              `json:"weaponType"`
	WeaponTypeKey string           `json:"weaponTypeKey"`
	NFTContract   *string          `json:"nftContract"`
	NFTTokenID    *string          `json:"nftTokenId"`
	NFTStatus     *string          `json:"nftStatus,omitempty"`
	Affixes       []EquipmentAffix `json:"affixes,omitempty"`
	// The following fields are response-only, derived from the active economy
	// configuration. They are deliberately not columns on equipment_items.
	ResolvedBaseFlat     map[string]float64 `json:"resolvedBaseFlat,omitempty"`
	ResolvedBasePercent  map[string]float64 `json:"resolvedBasePercent,omitempty"`
	ResolvedFlatStats    map[string]float64 `json:"resolvedFlatStats,omitempty"`
	ResolvedPercentStats map[string]float64 `json:"resolvedPercentStats,omitempty"`
	FinalBonuses         map[string]float64 `json:"finalBonuses,omitempty"`
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
	mu                    sync.Mutex
	nextID                int64
	nonces                map[string]*WalletLoginNonce
	accounts              map[int64]*Account
	byWallet              map[string]int64
	characters            map[int64]*Character
	characterStates       map[int64]*characterStateRecord
	tickets               map[string]*GameTicket
	tokens                map[int64]*AccountToken
	locked                map[int64]*LockedGame
	withdrawals           map[int64]*Withdrawal
	ledger                []LedgerEntry
	audits                []AuditEntry
	adminUsers            map[string]*AdminUser
	adminLoginNonces      map[string]*AdminLoginNonce
	adminOperations       map[string]*AdminOperation
	announcementTemplates map[string]*AnnouncementTemplate
	announcements         map[int64]*Announcement
	sessions              map[string]*AccountSession
	refresh               map[string]*RefreshTokenRecord
	servers               map[string]*GameServer
	online                map[int64]*OnlineSession
	serviceIdentities     map[string]*ServiceIdentity
	serviceNonces         map[string]time.Time
	dungeonRecoveries     map[int64]*DungeonRunRecovery
	mysteryShops          map[string]*MysteryShopBoardState
}

var Default = New()

func New() *Store {
	return &Store{
		nextID:            1000,
		nonces:            map[string]*WalletLoginNonce{},
		accounts:          map[int64]*Account{},
		byWallet:          map[string]int64{},
		characters:        map[int64]*Character{},
		characterStates:   map[int64]*characterStateRecord{},
		tickets:           map[string]*GameTicket{},
		tokens:            map[int64]*AccountToken{},
		locked:            map[int64]*LockedGame{},
		withdrawals:       map[int64]*Withdrawal{},
		sessions:          map[string]*AccountSession{},
		refresh:           map[string]*RefreshTokenRecord{},
		servers:           map[string]*GameServer{},
		online:            map[int64]*OnlineSession{},
		adminUsers:        map[string]*AdminUser{},
		adminLoginNonces:  map[string]*AdminLoginNonce{},
		adminOperations:   map[string]*AdminOperation{},
		serviceIdentities: map[string]*ServiceIdentity{},
		serviceNonces:     map[string]time.Time{},
		dungeonRecoveries: map[int64]*DungeonRunRecovery{},
		mysteryShops:      map[string]*MysteryShopBoardState{},
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

func (s *Store) ListAdminAccountSelector(filter AdminAccountSelectorFilter) ([]AdminAccountSelectorItem, error) {
	filter, err := normalizeAdminAccountSelectorFilter(filter)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	keyword := strings.ToLower(filter.Keyword)
	matches := []AdminAccountSelectorItem{}
	for _, account := range s.accounts {
		status := "ACTIVE"
		if account.IsBanned {
			status = "BANNED"
		}
		if filter.Status != "" && status != filter.Status {
			continue
		}
		roles := []Character{}
		for _, character := range s.characters {
			if character.AccountID == account.ID && !character.Deleted {
				roles = append(roles, *character)
			}
		}
		sort.Slice(roles, func(i, j int) bool {
			if roles[i].SlotIndex == roles[j].SlotIndex {
				return roles[i].ID < roles[j].ID
			}
			return roles[i].SlotIndex < roles[j].SlotIndex
		})
		roleNames := make([]string, 0, len(roles))
		for _, role := range roles {
			roleNames = append(roleNames, role.Name)
		}
		roleText := strings.Join(roleNames, ",")
		if keyword != "" {
			haystack := strings.ToLower(strings.Join([]string{
				strconv.FormatInt(account.ID, 10),
				account.Username,
				account.WalletAddress,
				roleText,
			}, " "))
			if !strings.Contains(haystack, keyword) {
				continue
			}
		}
		matches = append(matches, AdminAccountSelectorItem{
			AccountID:     account.ID,
			Username:      account.Username,
			WalletAddress: account.WalletAddress,
			Status:        status,
			Roles:         roleText,
			CreatedAt:     account.CreatedAt,
			LastLoginAt:   account.LastLoginAt,
		})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].LastLoginAt.Equal(matches[j].LastLoginAt) {
			return matches[i].AccountID > matches[j].AccountID
		}
		return matches[i].LastLoginAt.After(matches[j].LastLoginAt)
	})
	if filter.Offset >= len(matches) {
		return []AdminAccountSelectorItem{}, nil
	}
	end := filter.Offset + filter.Limit
	if end > len(matches) {
		end = len(matches)
	}
	return append([]AdminAccountSelectorItem(nil), matches[filter.Offset:end]...), nil
}

func (s *Store) ListAdminCharacters(filter AdminCharacterListFilter) ([]AdminCharacterSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit := clampAdminLimit(filter.Limit)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	keyword := strings.ToLower(strings.TrimSpace(filter.Keyword))
	wallet := strings.TrimSpace(filter.Wallet)
	status := strings.ToUpper(strings.TrimSpace(filter.Status))
	serverID := strings.TrimSpace(filter.ServerID)
	matches := []AdminCharacterSummary{}
	for _, character := range s.characters {
		if character.Deleted {
			continue
		}
		account := s.accounts[character.AccountID]
		if account == nil {
			continue
		}
		accountStatus := "ACTIVE"
		if account.IsBanned {
			accountStatus = "BANNED"
		}
		online := s.online[character.AccountID]
		isOnline := online != nil && online.CharacterID == character.ID
		if keyword != "" {
			haystack := strings.ToLower(strings.Join([]string{
				character.Name,
				strconv.FormatInt(character.ID, 10),
				strconv.FormatInt(character.AccountID, 10),
				account.Username,
				account.WalletAddress,
			}, " "))
			if !strings.Contains(haystack, keyword) {
				continue
			}
		}
		if filter.AccountID > 0 && character.AccountID != filter.AccountID {
			continue
		}
		if wallet != "" && account.WalletAddress != wallet {
			continue
		}
		if filter.MinLevel > 1 {
			continue
		}
		if filter.MaxLevel > 0 && filter.MaxLevel < 1 {
			continue
		}
		if filter.HasTradingLicense != nil && *filter.HasTradingLicense {
			continue
		}
		if status != "" && accountStatus != status {
			continue
		}
		if filter.OnlineOnly && !isOnline {
			continue
		}
		if serverID != "" && (!isOnline || online.ServerID != serverID) {
			continue
		}
		row := AdminCharacterSummary{
			CharacterID:       character.ID,
			AccountID:         character.AccountID,
			Name:              character.Name,
			Level:             1,
			WalletAddress:     account.WalletAddress,
			AccountStatus:     accountStatus,
			HasTradingLicense: false,
			LastLoginAt:       account.LastLoginAt,
			Online:            isOnline,
			CreatedAt:         character.CreatedAt,
		}
		if isOnline {
			row.ServerID = online.ServerID
		}
		matches = append(matches, row)
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].CreatedAt.After(matches[j].CreatedAt) })
	if offset >= len(matches) {
		return []AdminCharacterSummary{}, nil
	}
	end := offset + limit
	if end > len(matches) {
		end = len(matches)
	}
	return append([]AdminCharacterSummary(nil), matches[offset:end]...), nil
}

func (s *Store) AdminCharacterDetail(characterID int64) (AdminCharacterDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	character := s.characters[characterID]
	if character == nil || character.Deleted {
		return AdminCharacterDetail{}, ErrNotFound
	}
	account := s.accounts[character.AccountID]
	if account == nil {
		return AdminCharacterDetail{}, ErrNotFound
	}
	status := "ACTIVE"
	if account.IsBanned {
		status = "BANNED"
	}
	token := *s.ensureTokenLocked(account.ID)
	out := AdminCharacterDetail{
		Character: AdminCharacterCore{
			CharacterID: character.ID,
			AccountID:   character.AccountID,
			Name:        character.Name,
			Level:       1,
			BagSlots:    25,
			CreatedAt:   character.CreatedAt,
		},
		Account: AdminAccountDetail{
			ID:            account.ID,
			Username:      account.Username,
			WalletAddress: account.WalletAddress,
			Status:        status,
			IsBanned:      account.IsBanned,
			CreatedAt:     account.CreatedAt,
			LastLoginAt:   account.LastLoginAt,
		},
		Economy: EconomySnapshot{
			AccountID:    account.ID,
			CharacterID:  character.ID,
			BagSlots:     25,
			Level:        1,
			AccountToken: token,
			Inventory:    []InventoryItem{},
			Warehouse:    []InventoryItem{},
			LootTray:     []InventoryItem{},
			Equipment:    []EquipmentItem{},
		},
	}
	if online := s.online[account.ID]; online != nil && online.CharacterID == character.ID {
		lastSeen := online.LastSeenAt
		out.Online = AdminOnlineStatus{
			Online:       true,
			ServerID:     online.ServerID,
			ConnectionID: online.ConnectionID,
			SessionID:    online.SessionID,
			LastSeenAt:   &lastSeen,
		}
	}
	return out, nil
}

func (s *Store) ListCharacterLedger(characterID int64, kind string, limit, offset int) ([]LedgerEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	character := s.characters[characterID]
	if character == nil || character.Deleted {
		return nil, ErrNotFound
	}
	limit = clampAdminLimit(limit)
	if offset < 0 {
		offset = 0
	}
	kind = strings.TrimSpace(kind)
	out := []LedgerEntry{}
	for _, row := range s.ledger {
		if row.AccountID != character.AccountID {
			continue
		}
		if kind != "" && row.Kind != kind {
			continue
		}
		out = append(out, row)
	}
	if offset >= len(out) {
		return []LedgerEntry{}, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return append([]LedgerEntry(nil), out[offset:end]...), nil
}

func (s *Store) ListCharacterAudits(characterID int64, limit, offset int) ([]AuditEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	character := s.characters[characterID]
	if character == nil || character.Deleted {
		return nil, ErrNotFound
	}
	limit = clampAdminLimit(limit)
	if offset < 0 {
		offset = 0
	}
	characterTarget := "character:" + strconv.FormatInt(characterID, 10)
	accountTarget := "account:" + strconv.FormatInt(character.AccountID, 10)
	out := []AuditEntry{}
	for i := len(s.audits) - 1; i >= 0; i-- {
		row := s.audits[i]
		if row.Target == characterTarget || row.Target == accountTarget {
			out = append(out, row)
		}
	}
	if offset >= len(out) {
		return []AuditEntry{}, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return append([]AuditEntry(nil), out[offset:end]...), nil
}

func (s *Store) AdminCharacterTimeline(characterID int64, filter AdminCharacterTimelineFilter) (AdminCharacterTimelinePage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	character := s.characters[characterID]
	if character == nil || character.Deleted {
		return AdminCharacterTimelinePage{}, ErrNotFound
	}
	limit := clampAdminLimit(filter.Limit)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	types, err := adminTimelineTypes(filter.Types)
	if err != nil {
		return AdminCharacterTimelinePage{}, err
	}
	items := []AdminCharacterTimelineItem{}
	if types["ledger"] {
		for _, row := range s.ledger {
			if row.AccountID != character.AccountID {
				continue
			}
			items = append(items, AdminCharacterTimelineItem{
				Type: "ledger", ID: "ledger:" + strconv.FormatInt(row.ID, 10), Title: row.Kind,
				Detail: row.Detail, Amount: row.Amount, Ref: row.Ref, CreatedAt: row.CreatedAt,
			})
		}
	}
	if types["audit"] {
		characterTarget := "character:" + strconv.FormatInt(characterID, 10)
		accountTarget := "account:" + strconv.FormatInt(character.AccountID, 10)
		for _, row := range s.audits {
			if row.Target != characterTarget && row.Target != accountTarget {
				continue
			}
			items = append(items, AdminCharacterTimelineItem{
				Type: "audit", ID: "audit:" + strconv.FormatInt(row.ID, 10), Title: row.Action,
				Detail: row.Reason, CreatedAt: row.CreatedAt,
			})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	total := len(items)
	if offset >= len(items) {
		return AdminCharacterTimelinePage{Items: []AdminCharacterTimelineItem{}, Count: 0, Total: total, Limit: limit, Offset: offset}, nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	page := append([]AdminCharacterTimelineItem(nil), items[offset:end]...)
	return AdminCharacterTimelinePage{Items: page, Count: len(page), Total: total, Limit: limit, Offset: offset}, nil
}

func (s *Store) AdminEquipmentDetail(equipmentUID string) (AdminEquipmentDetail, error) {
	return AdminEquipmentDetail{}, adminErrRequiresPostgres("admin equipment detail")
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

func (s *Store) AdminOperation(opID string) (AdminOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.adminOperations[strings.TrimSpace(opID)]
	if !ok {
		return AdminOperation{}, ErrNotFound
	}
	cp := *row
	cp.Response = append(json.RawMessage(nil), row.Response...)
	return cp, nil
}

func (s *Store) SaveAdminOperation(in AdminOperation) (AdminOperation, error) {
	if strings.TrimSpace(in.OpID) == "" {
		return AdminOperation{}, errors.New("opId is required")
	}
	if len(in.Response) == 0 || !json.Valid(in.Response) {
		return AdminOperation{}, errors.New("admin operation response is invalid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.adminOperations[strings.TrimSpace(in.OpID)]; ok {
		cp := *existing
		cp.Response = append(json.RawMessage(nil), existing.Response...)
		return cp, nil
	}
	in.OpID = strings.TrimSpace(in.OpID)
	in.AdminID = strings.TrimSpace(in.AdminID)
	in.Action = strings.TrimSpace(in.Action)
	in.Target = strings.TrimSpace(in.Target)
	in.Response = append(json.RawMessage(nil), in.Response...)
	in.CreatedAt = time.Now().UTC()
	cp := in
	s.adminOperations[in.OpID] = &cp
	return in, nil
}

func (s *Store) AdminGrantRewards(req AdminRewardGrant) (AdminRewardGrantResult, error) {
	if req.AnnounceRare && strings.TrimSpace(req.AnnouncementSource) == "" {
		return AdminRewardGrantResult{}, errors.New("announcementSource is required when announceRare is true")
	}
	return AdminRewardGrantResult{}, adminErrRequiresPostgres("admin reward grant")
}

func (s *Store) AdminCharacterLevel(characterID int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.characters[characterID]
	if !ok {
		return 0, ErrNotFound
	}
	// The in-memory repository does not model progression; it exists only for
	// route contracts, so use the baseline character level.
	return 1, nil
}

func (s *Store) CreateAdminCompensationPreview(adminID string, filter AdminCompensationFilter, rewards AdminCompensationRewards) (AdminCompensationPreview, error) {
	return AdminCompensationPreview{}, adminErrRequiresPostgres("admin compensation preview")
}

func (s *Store) AdminCompensationPreview(previewID, adminID string) (AdminCompensationPreviewDetail, error) {
	return AdminCompensationPreviewDetail{}, adminErrRequiresPostgres("admin compensation preview")
}

func (s *Store) ListAdminCompensationPreviewTargets(previewID, adminID, keyword string, limit, offset int) (AdminCompensationTargetPage, error) {
	return AdminCompensationTargetPage{}, adminErrRequiresPostgres("admin compensation preview targets")
}

func (s *Store) CommitAdminCompensation(req AdminCompensationCommit) (AdminCompensationResult, error) {
	return AdminCompensationResult{}, adminErrRequiresPostgres("admin compensation commit")
}

func (s *Store) CreateAdminLotteryPreview(adminID string, characterID int64, rewards DungeonRewardPlan) (AdminLotteryPreview, error) {
	return AdminLotteryPreview{}, adminErrRequiresPostgres("admin lottery preview")
}

func (s *Store) CommitAdminLotteryPreview(req AdminLotteryCommit) (AdminRewardGrantResult, error) {
	return AdminRewardGrantResult{}, adminErrRequiresPostgres("admin lottery preview commit")
}

func (s *Store) ListPaymentOrdersAdmin(filter AdminListFilter) ([]PaymentOrder, error) {
	return nil, adminErrRequiresPostgres("payment orders")
}

func (s *Store) AdminPaymentTrace(orderID string) (AdminPaymentTrace, error) {
	return AdminPaymentTrace{}, adminErrRequiresPostgres("payment trace")
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
	return s.CreateCharacterWithAppearance(accountID, name, nil)
}

func (s *Store) CreateCharacterWithAppearance(accountID int64, name string, appearance map[string]any) (Character, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.accounts[accountID]; !ok {
		return Character{}, ErrNotFound
	}
	used := map[int]bool{}
	for _, row := range s.characters {
		if row.AccountID == accountID && !row.Deleted {
			used[row.SlotIndex] = true
		}
	}
	slotIndex := -1
	for candidate := 0; candidate < 3; candidate++ {
		if !used[candidate] {
			slotIndex = candidate
			break
		}
	}
	if slotIndex < 0 {
		return Character{}, errors.New("character slots are full")
	}
	id := s.next()
	if appearance == nil {
		appearance = map[string]any{}
	}
	row := &Character{ID: id, AccountID: accountID, Name: name, SlotIndex: slotIndex, Level: 1, Appearance: appearance, EquipmentItems: []EquipmentItem{}, CreatedAt: time.Now().UTC(), LastPlayed: time.Unix(0, 0).UTC()}
	s.characters[id] = row
	s.characterStates[id] = &characterStateRecord{Hunger: 100, LastPlayed: time.Unix(0, 0).UTC()}
	return *row, nil
}

func (s *Store) Characters(accountID int64) []Character {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := []Character{}
	for _, row := range s.characters {
		if row.AccountID == accountID && !row.Deleted {
			copy := *row
			if state, ok := s.characterStates[row.ID]; ok {
				copy.LastPlayed = state.LastPlayed
				copy.HasLastPlayed = state.HasLastPlayed
			}
			rows = append(rows, copy)
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
	copy := *row
	if state, ok := s.characterStates[characterID]; ok {
		copy.LastPlayed = state.LastPlayed
		copy.HasLastPlayed = state.HasLastPlayed
	}
	return copy, true
}

func (s *Store) DeleteCharacter(accountID, characterID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.characters[characterID]
	if !ok || row.AccountID != accountID || row.Deleted {
		return ErrNotFound
	}
	row.Deleted = true
	delete(s.characterStates, characterID)
	return nil
}

func (s *Store) PlayerProfile(accountID, characterID int64) (PlayerState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.characters[characterID]
	if !ok || row.AccountID != accountID || row.Deleted {
		return PlayerState{}, ErrNotFound
	}
	state := s.characterStates[characterID]
	if state == nil {
		state = &characterStateRecord{Hunger: 100, LastPlayed: time.Unix(0, 0).UTC()}
		s.characterStates[characterID] = state
	}
	return PlayerState{
		AccountID: accountID, CharacterID: characterID, PosX: state.PosX, PosY: state.PosY,
		CurrentMap: state.CurrentMap, PlayTimeSec: state.PlayTimeSec, Hunger: state.Hunger,
		Level: row.Level, Appearance: row.Appearance, CharacterName: row.Name,
		LastPlayed: state.LastPlayed, HasLastPlayed: state.HasLastPlayed,
	}, nil
}

func (s *Store) SavePlayerState(req PlayerSaveRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.characters[req.CharacterID]
	if !ok || row.AccountID != req.AccountID || row.Deleted {
		return ErrNotFound
	}
	if req.PlayTimeSec < 0 {
		return errors.New("playTimeSec must be non-negative")
	}
	if req.Hunger < 0 {
		return errors.New("hunger must be non-negative")
	}
	now := time.Now().UTC()
	s.characterStates[req.CharacterID] = &characterStateRecord{
		PosX:          req.PosX,
		PosY:          req.PosY,
		CurrentMap:    strings.TrimSpace(req.CurrentMap),
		PlayTimeSec:   req.PlayTimeSec,
		Hunger:        req.Hunger,
		LastPlayed:    now,
		HasLastPlayed: true,
	}
	return nil
}

func (s *Store) ClearProgressLeaderboard(limit int) (ClearProgressLeaderboard, error) {
	return ClearProgressLeaderboard{Intro: "永久通关进度榜按角色历史最高 floorId 排名，同层时更早达成者优先。", Items: []ClearProgressLeaderboardItem{}, UpdatedAt: time.Now().UTC()}, nil
}

func (s *Store) WeeklyScoreLeaderboard(now time.Time, limit int) (WeeklyScoreLeaderboard, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	period := LeaderboardPeriod{PeriodID: "weekly-score-20260701-0", StartsAt: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), EndsAt: time.Date(2026, 7, 8, 0, 0, 0, 0, time.UTC)}
	return WeeklyScoreLeaderboard{Intro: "7 天游积分榜每次成功通关按 floorId 加分，同一楼层可重复累计。", Period: period, Items: []WeeklyScoreLeaderboardItem{}, UpdatedAt: now.UTC()}, nil
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

func (s *Store) ShopBuyGold(req ShopBuyRequest) (ShopBuyResult, error) {
	if req.ShopSlotIndex > 0 && req.ShopDailyLimit > 0 && req.Quantity > req.ShopDailyLimit {
		return ShopBuyResult{}, errors.New("shop daily purchase limit reached")
	}
	if req.MysterySlotIndex > 0 {
		s.mu.Lock()
		if state, ok := s.mysteryShops[mysteryShopKey(req.CharacterID, req.ShopID)]; ok {
			for index := range state.Offers {
				if state.Offers[index].SlotIndex == req.MysterySlotIndex {
					state.Offers[index].Purchased = true
					break
				}
			}
		}
		s.mu.Unlock()
	}
	snapshot, err := s.EconomySnapshot(req.AccountID, req.CharacterID)
	if err != nil {
		return ShopBuyResult{}, err
	}
	return ShopBuyResult{Snapshot: snapshot}, nil
}

func (s *Store) ShopDailyPurchaseQuantities(accountID, characterID int64, shopID, businessDate string) (map[int]int64, error) {
	return map[int]int64{}, nil
}

func (s *Store) CreateShopBuyPayment(req ShopBuyRequest) (ShopBuyResult, error) {
	return ShopBuyResult{}, errors.New("shop chain payment requires postgres store")
}

func (s *Store) ShopSell(req ShopSellRequest) (ShopSellResult, error) {
	snapshot, err := s.EconomySnapshot(req.AccountID, req.CharacterID)
	if err != nil {
		return ShopSellResult{}, err
	}
	return ShopSellResult{Snapshot: snapshot}, nil
}

func (s *Store) MysteryShopBoard(accountID, characterID int64, shopID string) (MysteryShopBoardState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	character, ok := s.characters[characterID]
	if !ok || character.AccountID != accountID || character.Deleted {
		return MysteryShopBoardState{}, ErrNotFound
	}
	key := mysteryShopKey(characterID, shopID)
	if row, ok := s.mysteryShops[key]; ok {
		return *row, nil
	}
	return MysteryShopBoardState{}, ErrNotFound
}

func (s *Store) MysteryShopPaidRefreshCount(accountID, characterID int64, shopID string, dayStart, dayEnd time.Time) (int, error) {
	return 0, nil
}

func (s *Store) RefreshMysteryShop(req MysteryShopRefreshRequest) (MysteryShopBoardState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	character, ok := s.characters[req.CharacterID]
	if !ok || character.AccountID != req.AccountID || character.Deleted {
		return MysteryShopBoardState{}, ErrNotFound
	}
	state := MysteryShopBoardState{
		ShopID:            strings.TrimSpace(req.ShopID),
		CharacterID:       req.CharacterID,
		NextFreeRefreshAt: req.NextFreeRefreshAt,
		GeneratedAt:       req.GeneratedAt,
		Offers:            append([]MysteryShopOffer(nil), req.Offers...),
	}
	s.mysteryShops[mysteryShopKey(req.CharacterID, req.ShopID)] = &state
	return state, nil
}

func mysteryShopKey(characterID int64, shopID string) string {
	return strconv.FormatInt(characterID, 10) + ":" + strings.TrimSpace(shopID)
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
	runID := "memory-" + strings.TrimSpace(req.OpID)
	s.mu.Lock()
	if existing := s.dungeonRecoveries[req.CharacterID]; existing != nil && existing.Required {
		s.mu.Unlock()
		return DungeonResult{}, ErrForbidden
	}
	s.dungeonRecoveries[req.CharacterID] = &DungeonRunRecovery{
		Required: true, DungeonRunID: runID, AccountID: req.AccountID, CharacterID: req.CharacterID,
		ServerID: strings.TrimSpace(req.ServerID), ChapterID: req.ChapterID, FloorID: req.FloorID, StartedAt: time.Now().UTC(),
	}
	s.mu.Unlock()
	return DungeonResult{
		DungeonRunID: runID,
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

func (s *Store) ActiveDungeonRun(accountID, characterID int64) (DungeonRunRecovery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.dungeonRecoveries[characterID]
	if row == nil || !row.Required || row.AccountID != accountID {
		return DungeonRunRecovery{}, ErrNotFound
	}
	return *row, nil
}

func (s *Store) CancelDungeonRun(accountID, characterID int64, dungeonRunID, reason string) (DungeonRunRecovery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.dungeonRecoveries[characterID]
	if row == nil || row.AccountID != accountID || row.DungeonRunID != strings.TrimSpace(dungeonRunID) {
		return DungeonRunRecovery{}, ErrNotFound
	}
	if !row.Required {
		if row.Status == "CANCELLED" {
			return *row, nil
		}
		return DungeonRunRecovery{}, ErrForbidden
	}
	row.Required = false
	row.Status = "CANCELLED"
	for _, ticket := range s.tickets {
		if ticket.AccountID == accountID && (ticket.CharacterID == characterID || (ticket.CharacterID == 0 && ticket.ServerID == row.ServerID)) && !ticket.Consumed {
			ticket.Consumed = true
		}
	}
	_ = reason
	return *row, nil
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
	s.mu.Lock()
	recovery := s.dungeonRecoveries[req.CharacterID]
	if recovery == nil || !recovery.Required || recovery.AccountID != req.AccountID || recovery.DungeonRunID != strings.TrimSpace(req.DungeonRunID) {
		s.mu.Unlock()
		return DungeonResult{}, ErrForbidden
	}
	if recovery.ServerID != "" && strings.TrimSpace(req.ServerID) != recovery.ServerID {
		s.mu.Unlock()
		return DungeonResult{}, ErrForbidden
	}
	recovery.Required = false
	if result == "victory" {
		recovery.Status = "FINISHED"
	} else {
		recovery.Status = "FAILED"
	}
	s.mu.Unlock()
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

func (s *Store) EquipmentEnhance(req EquipmentEnhanceRequest) (EquipmentEnhanceResult, error) {
	return EquipmentEnhanceResult{}, errors.New("equipment enhancement requires postgres store")
}

func (s *Store) EquipmentNPCRecycle(req EquipmentNPCRecycleRequest) (EquipmentNPCRecycleResult, error) {
	return EquipmentNPCRecycleResult{}, errors.New("equipment npc recycle requires postgres store")
}

func (s *Store) PurgeExpiredNPCRecycledEquipment(now time.Time, limit int) (int64, error) {
	return 0, errors.New("equipment npc recycle purge requires postgres store")
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

func (s *Store) CreateLotteryPayment(req LotteryPaymentRequest) (LotteryPaymentResult, error) {
	return LotteryPaymentResult{}, errors.New("lottery payment requires postgres store")
}

func (s *Store) BountyBoard(req BountyBoardRequest) (BountyBoard, error) {
	return BountyBoard{}, errors.New("bounty board requires postgres store")
}
func (s *Store) UnlockBountyGoldSlot(req BountyGoldUnlockRequest) (BountyBoard, error) {
	return BountyBoard{}, errors.New("bounty board requires postgres store")
}
func (s *Store) CreateBountyPayment(req BountyPaymentRequest) (BountyPaymentResult, error) {
	return BountyPaymentResult{}, errors.New("bounty board requires postgres store")
}
func (s *Store) RefreshBounty(req BountyRefreshRequest) (BountyBoard, error) {
	return BountyBoard{}, errors.New("bounty board requires postgres store")
}
func (s *Store) SubmitBountyEquipment(req BountyEquipmentSubmitRequest) (BountyTask, error) {
	return BountyTask{}, errors.New("bounty board requires postgres store")
}
func (s *Store) ClaimBounty(req BountyClaimRequest) (BountyTask, error) {
	return BountyTask{}, errors.New("bounty board requires postgres store")
}
func (s *Store) DrawBountyBadge(req BountyBadgeDrawRequest) (BountyBadgeDrawResult, error) {
	return BountyBadgeDrawResult{}, errors.New("bounty board requires postgres store")
}
func (s *Store) ProgressBountyCombat(req BountyCombatProgressRequest) ([]BountyTask, error) {
	return nil, errors.New("bounty board requires postgres store")
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
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return GameTicket{}, ErrForbidden
	}
	row, ok := s.tickets[ticket]
	if !ok {
		return GameTicket{}, ErrNotFound
	}
	if row.Consumed || row.ExpiresAt.Before(now) {
		return GameTicket{}, ErrForbidden
	}
	if row.ServerID == "" || row.ServerID != serverID {
		return GameTicket{}, ErrForbidden
	}
	session, ok := s.sessions[row.SessionID]
	if !ok || session.AccountID != row.AccountID || session.Status != "ACTIVE" {
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

func (s *Store) CountMonthlyActiveAccounts(since time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := map[int64]struct{}{}
	for _, row := range s.sessions {
		if row.LastSeenAt == nil || row.LastSeenAt.Before(since) {
			continue
		}
		seen[row.AccountID] = struct{}{}
	}
	return len(seen), nil
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
	if server.MaxPlayers <= 0 {
		server.MaxPlayers = 50
	}
	if strings.TrimSpace(server.Status) == "" {
		server.Status = "ONLINE"
	}
	if strings.TrimSpace(server.DisplayName) == "" {
		server.DisplayName = server.ServerID
	}
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
