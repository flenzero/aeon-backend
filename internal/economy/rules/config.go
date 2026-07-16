package rules

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type Config struct {
	Dir         string
	Items       map[string]Item
	Shops       map[string]Shop
	Floors      map[string]DungeonFloor
	LootPools   map[string]LootPool
	AffixPools  map[string]AffixPool
	GatherNodes map[string]GatheringNode
	Crops       map[string]Crop
	Bosses      map[string]BossDefinition
	Recipes     map[string]Recipe
	Marketplace MarketplaceConfig
	Rules       EconomyRules
	Equipment   EquipmentConfig
	Lottery     LotteryConfig
	Bounties    BountyConfig
}

const (
	WeaponTypeNone  = 0
	WeaponTypeSword = 1
	WeaponTypeAxe   = 2
	WeaponTypeBow   = 3
	WeaponTypeStaff = 4
)

var weaponTypeKeys = map[string]int{
	"none":  WeaponTypeNone,
	"sword": WeaponTypeSword,
	"axe":   WeaponTypeAxe,
	"bow":   WeaponTypeBow,
	"staff": WeaponTypeStaff,
}

type Item struct {
	ItemID          string `json:"itemId"`
	DisplayName     string `json:"displayName"`
	Category        string `json:"category"`
	Rarity          int    `json:"rarity"`
	MaxStack        int64  `json:"maxStack"`
	IsEquipment     bool   `json:"isEquipment"`
	EquipSlot       int    `json:"equipSlot"`
	WeaponType      int    `json:"weaponType"`
	WeaponTypeKey   string `json:"weaponTypeKey"`
	Tradable        bool   `json:"tradable"`
	DefaultBindType string `json:"defaultBindType"`
	BuyCurrency     int    `json:"buyCurrency"`
	BuyPrice        int64  `json:"buyPrice"`
	SellPrice       int64  `json:"sellPrice"`
	GrantGold       int64  `json:"grantGold"`
	GrantLockedAEB  int64  `json:"grantLockedAeb"`
}

type Shop struct {
	ShopID             string             `json:"shopId"`
	DisplayName        string             `json:"displayName"`
	DailyLimitTimezone string             `json:"dailyLimitTimezone,omitempty"`
	SellAllItems       bool               `json:"sellAllItems"`
	SellItems          []ShopSellItem     `json:"sellItems"`
	Mystery            *MysteryShopConfig `json:"mystery,omitempty"`
}

type ShopSellItem struct {
	SlotIndex  int    `json:"slotIndex,omitempty"`
	ItemID     string `json:"itemId"`
	MinLevel   int    `json:"minLevel,omitempty"`
	MaxLevel   int    `json:"maxLevel,omitempty"`
	Rarity     int    `json:"rarity,omitempty"`
	BuyPrice   int64  `json:"buyPrice,omitempty"`
	DailyLimit int64  `json:"dailyLimit,omitempty"`
}

func (item *ShopSellItem) UnmarshalJSON(data []byte) error {
	var id string
	if err := json.Unmarshal(data, &id); err == nil {
		item.ItemID = id
		return nil
	}
	var row struct {
		SlotIndex  int    `json:"slotIndex"`
		ItemID     string `json:"itemId"`
		MinLevel   int    `json:"minLevel"`
		MaxLevel   int    `json:"maxLevel"`
		Rarity     int    `json:"rarity"`
		BuyPrice   int64  `json:"buyPrice"`
		DailyLimit int64  `json:"dailyLimit"`
	}
	if err := json.Unmarshal(data, &row); err != nil {
		return err
	}
	item.SlotIndex = row.SlotIndex
	item.ItemID = row.ItemID
	item.MinLevel = row.MinLevel
	item.MaxLevel = row.MaxLevel
	item.Rarity = row.Rarity
	item.BuyPrice = row.BuyPrice
	item.DailyLimit = row.DailyLimit
	return nil
}

type MysteryShopConfig struct {
	ManualRefreshTokenBaseCost int64             `json:"manualRefreshTokenBaseCost"`
	ManualRefreshTokenStepCost int64             `json:"manualRefreshTokenStepCost"`
	ManualRefreshTokenMaxCost  int64             `json:"manualRefreshTokenMaxCost"`
	DailyLimitTimezone         string            `json:"dailyLimitTimezone"`
	MaxSlots                   int               `json:"maxSlots"`
	Slots                      []MysteryShopSlot `json:"slots"`
	Discounts                  []MysteryDiscount `json:"discounts"`
}

type MysteryShopSlot struct {
	SlotIndex   int                    `json:"slotIndex"`
	UnlockLevel int                    `json:"unlockLevel"`
	Pools       []MysteryShopPoolEntry `json:"pools"`
}

type MysteryShopPoolEntry struct {
	ItemID      string `json:"itemId"`
	Weight      int    `json:"weight"`
	Quantity    int64  `json:"quantity"`
	Rarity      int    `json:"rarity"`
	MinLevel    int    `json:"minLevel,omitempty"`
	MaxLevel    int    `json:"maxLevel,omitempty"`
	BaseGold    int64  `json:"baseGold"`
	BaseToken   int64  `json:"baseToken"`
	MinDiscount int    `json:"minDiscount"`
}

type MysteryDiscount struct {
	Bps    int `json:"bps"`
	Weight int `json:"weight"`
}

// EquipmentConfig defines the live, config-derived equipment model. Equipment
// instances only retain their template id, rarity and affix keys; the values
// are resolved from this configuration whenever they are read.
type EquipmentConfig struct {
	StageMultipliers      map[int]float64              `json:"stageMultipliers"`
	Rarities              map[int]EquipmentRarity      `json:"rarities"`
	NPCRecycleGoldByStage map[int]map[int]int64        `json:"npcRecycleGoldByStage"`
	Enhancement           EnhancementConfig            `json:"enhancement"`
	Series                []EquipmentSeries            `json:"series"`
	Mounts                []MountTemplate              `json:"mounts"`
	ByItemID              map[string]EquipmentTemplate `json:"-"`
}

type EnhancementConfig struct {
	MaxLevel                   int             `json:"maxLevel"`
	GoldCostByStage            map[int][]int64 `json:"goldCostByStage"`
	StoneItemByStage           map[int]string  `json:"stoneItemByStage"`
	StoneBaseCostByStageRarity map[int][]int64 `json:"stoneBaseCostByStageRarity"`
	StoneProgressionByLevel    []int64         `json:"stoneProgressionByLevel"`
}

type EquipmentRarity struct {
	Name       string  `json:"name"`
	Multiplier float64 `json:"multiplier"`
	AffixCount int     `json:"affixCount"`
}

type EquipmentSeries struct {
	SeriesID      string                   `json:"seriesId"`
	EquipSlot     int                      `json:"equipSlot"`
	Category      string                   `json:"category"`
	WeaponType    int                      `json:"weaponType"`
	WeaponTypeKey string                   `json:"weaponTypeKey"`
	DisplayType   string                   `json:"displayType"`
	AffixPoolID   string                   `json:"affixPoolId"`
	BaseFlat      map[string]float64       `json:"baseFlat"`
	Stages        []EquipmentStageTemplate `json:"stages"`
}

type EquipmentStageTemplate struct {
	Stage          int                `json:"stage"`
	ItemID         string             `json:"itemId"`
	Prefix         string             `json:"prefix"`
	BasePercent    map[string]float64 `json:"basePercent"`
	NPCRecycleGold map[int]int64      `json:"npcRecycleGold"`
}

type EquipmentTemplate struct {
	ItemID         string             `json:"itemId"`
	SeriesID       string             `json:"seriesId"`
	Stage          int                `json:"stage"`
	EquipSlot      int                `json:"equipSlot"`
	Category       string             `json:"category"`
	WeaponType     int                `json:"weaponType"`
	WeaponTypeKey  string             `json:"weaponTypeKey"`
	DisplayName    string             `json:"displayName"`
	AffixPoolID    string             `json:"affixPoolId"`
	BaseFlat       map[string]float64 `json:"baseFlat"`
	BasePercent    map[string]float64 `json:"basePercent"`
	NPCRecycleGold map[int]int64      `json:"npcRecycleGold"`
}

type MountTemplate struct {
	ItemID       string `json:"itemId"`
	DisplayName  string `json:"displayName"`
	EquipSlot    int    `json:"equipSlot"`
	FinalHPBps   int    `json:"finalHpBps"`
	FinalAtkBps  int    `json:"finalAtkBps"`
	MoveSpeedBps int    `json:"moveSpeedBps"`
}

type LotteryConfig struct {
	PriceAEB              int64          `json:"priceAeb"`
	MaxCount              int            `json:"maxCount"`
	CategoryWeight        map[string]int `json:"categoryWeights"`
	EquipmentRarityWeight map[int]int    `json:"equipmentRarityWeights"`
	RareMaterialItemIDs   []string       `json:"rareMaterialItemIds"`
	BossTicketItemIDs     []string       `json:"bossTicketItemIds"`
	MountItemIDs          []string       `json:"mountItemIds"`
}

type BountyConfig struct {
	Slots         []BountySlotConfig            `json:"slots"`
	Refresh       BountyRefreshConfig           `json:"refresh"`
	TaskTemplates []BountyTaskTemplate          `json:"taskTemplates"`
	BadgeLottery  map[string]BountyBadgeLottery `json:"badgeLottery"`
}

type BountySlotConfig struct {
	SlotIndex  int    `json:"slotIndex"`
	UnlockType string `json:"unlockType"`
	GoldCost   int64  `json:"goldCost"`
	AEBPrice   int64  `json:"aebPrice"`
}

type BountyRefreshConfig struct {
	FreeCooldownSeconds int   `json:"freeCooldownSeconds"`
	GoldCost            int64 `json:"goldCost"`
	PremiumAEBPrice     int64 `json:"premiumAebPrice"`
}

type BountyTaskTemplate struct {
	TemplateID     string `json:"templateId"`
	Type           string `json:"type"`
	Difficulty     string `json:"difficulty"`
	ItemID         string `json:"itemId"`
	QuantityMin    int64  `json:"quantityMin"`
	QuantityMax    int64  `json:"quantityMax"`
	MinRarity      int    `json:"minRarity"`
	RewardItemID   string `json:"rewardItemId"`
	RewardQuantity int64  `json:"rewardQuantity"`
	Weight         int    `json:"weight"`
}

type BountyBadgeLottery struct {
	CostItemID string              `json:"costItemId"`
	Rewards    []BountyBadgeReward `json:"rewards"`
}

type BountyBadgeReward struct {
	Type   string `json:"type"`
	ItemID string `json:"itemId"`
	Weight int    `json:"weight"`
	Min    int64  `json:"min"`
	Max    int64  `json:"max"`
}

// BountyTaskPlan is the immutable task payload passed to storage. Runtime task
// rows never need to read a mutable template again to know their requirement
// or reward.
type BountyTaskPlan struct {
	TemplateID     string
	Type           string
	Difficulty     string
	ItemID         string
	MinRarity      int
	Required       int64
	RewardItemID   string
	RewardQuantity int64
}

type MarketplaceConfig struct {
	Enabled                            bool   `json:"enabled"`
	TokenSymbol                        string `json:"tokenSymbol"`
	MinListPrice                       int64  `json:"minListPrice"`
	FeeBps                             int64  `json:"feeBps"`
	FeeBurnBps                         int64  `json:"feeBurnBps"`
	FeeTreasuryBps                     int64  `json:"feeTreasuryBps"`
	FeeRewardsBps                      int64  `json:"feeRewardsBps"`
	ListingDepositBps                  int64  `json:"listingDepositBps"`
	BaseListingSlots                   int    `json:"baseListingSlots"`
	MaterialExpandSlots                int    `json:"materialExpandSlots"`
	MaterialExpandMaxTimes             int    `json:"materialExpandMaxTimes"`
	MaterialExpandItemID               string `json:"materialExpandItemId"`
	MaterialExpandItemQuantity         int64  `json:"materialExpandItemQuantity"`
	WalletExpandSlots                  int    `json:"walletExpandSlots"`
	WalletExpandMaxTimes               int    `json:"walletExpandMaxTimes"`
	WalletExpandPriceToken             int64  `json:"walletExpandPriceToken"`
	WalletExpandPaymentTimeoutSeconds  int    `json:"walletExpandPaymentTimeoutSeconds"`
	MaxListingsCreatedPerAccountPerDay int    `json:"maxListingsCreatedPerAccountPerDay"`
	MaxCancelsPerAccountPerDay         int    `json:"maxCancelsPerAccountPerDay"`
	MaxPurchasesPerAccountPerDay       int    `json:"maxPurchasesPerAccountPerDay"`
	PurchaseCooldownSeconds            int    `json:"purchaseCooldownSeconds"`
	DailyLimitTimezone                 string `json:"dailyLimitTimezone"`
	DefaultCooldownHours               int    `json:"defaultCooldownHours"`
}

type EconomyRules struct {
	Version  string `json:"version"`
	MaxLevel int    `json:"maxLevel"`
	LevelExp struct {
		Base      int64 `json:"base"`
		Linear    int64 `json:"linear"`
		Quadratic int64 `json:"quadratic"`
	} `json:"levelExp"`
	DefaultTokenCooldownHours           int           `json:"defaultTokenCooldownHours"`
	EquipmentUIDPrefix                  string        `json:"equipmentUidPrefix"`
	BagSlots                            int           `json:"bagSlots"`
	WarehouseSlots                      int           `json:"warehouseSlots"`
	BagExpandSlots                      int           `json:"bagExpandSlots"`
	BagExpandMaxTimes                   int           `json:"bagExpandMaxTimes"`
	BagExpandPriceToken                 int64         `json:"bagExpandPriceToken"`
	BagExpandPaymentTimeoutSeconds      int           `json:"bagExpandPaymentTimeoutSeconds"`
	TradingLicensePriceToken            int64         `json:"tradingLicensePriceToken"`
	TradingLicensePaymentTimeoutSeconds int           `json:"tradingLicensePaymentTimeoutSeconds"`
	EquipmentDefaultMaxDurability       int           `json:"equipmentDefaultMaxDurability"`
	EquipmentRepairCostPerPointToken    int64         `json:"equipmentRepairCostPerPointToken"`
	EquipmentRepairMinCostToken         int64         `json:"equipmentRepairMinCostToken"`
	EquipmentRepairBurnBps              int64         `json:"equipmentRepairBurnBps"`
	EquipmentRepairRecycleBps           int64         `json:"equipmentRepairRecycleBps"`
	EquipmentRepairRewardBps            int64         `json:"equipmentRepairRewardBps"`
	EquipmentDungeonWearPoints          int           `json:"equipmentDungeonWearPoints"`
	EquipmentBossWearPoints             int           `json:"equipmentBossWearPoints"`
	NFTMintFeesTokenByRarity            map[int]int64 `json:"nftMintFeesTokenByRarity"`
	NFTMintMinRarity                    int           `json:"nftMintMinRarity"`
}

type DungeonFloor struct {
	ChapterID     int
	FloorID       int               `json:"floorId"`
	IsBoss        bool              `json:"isBoss"`
	EnterCost     store.DungeonCost `json:"enterCost"`
	MaxExp        int64             `json:"maxExp"`
	LootPoolID    string            `json:"lootPoolId"`
	EnemyHpScale  float64           `json:"enemyHpScale,omitempty"`
	EnemyAtkScale float64           `json:"enemyAtkScale,omitempty"`
}

type LootPool struct {
	LootPoolID string      `json:"lootPoolId"`
	Entries    []LootEntry `json:"entries"`
}

type LootEntry struct {
	RewardType  string  `json:"rewardType"`
	ItemID      string  `json:"itemId"`
	QuantityMin int64   `json:"quantityMin"`
	QuantityMax int64   `json:"quantityMax"`
	DropChance  float64 `json:"dropChance"`
	Rarity      int     `json:"rarity"`
	AffixPoolID string  `json:"affixPoolId"`
}

type AffixPool struct {
	AffixPoolID string        `json:"affixPoolId"`
	Rolls       AffixRolls    `json:"rolls"`
	Affixes     []AffixConfig `json:"affixes"`
}

type AffixRolls struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type AffixConfig struct {
	AffixID      string            `json:"affixId"`
	Stat         string            `json:"stat"`
	Min          float64           `json:"min,omitempty"`
	Max          float64           `json:"max,omitempty"`
	Weight       int               `json:"weight"`
	ValueByStage map[int][]float64 `json:"valueByStage,omitempty"`
}

// ResolvedEquipment is the public, current-configuration view of an equipment
// instance. None of its numeric fields are stored in equipment_items.
type ResolvedEquipment struct {
	ItemID        string                   `json:"itemId"`
	Rarity        int                      `json:"rarity"`
	IsMount       bool                     `json:"isMount"`
	WeaponType    int                      `json:"weaponType"`
	WeaponTypeKey string                   `json:"weaponTypeKey"`
	BaseFlat      map[string]float64       `json:"baseFlat,omitempty"`
	BasePercent   map[string]float64       `json:"basePercent,omitempty"`
	Affixes       []ResolvedEquipmentAffix `json:"affixes,omitempty"`
	TotalFlat     map[string]float64       `json:"totalFlat,omitempty"`
	TotalPercent  map[string]float64       `json:"totalPercent,omitempty"`
	FinalBonuses  map[string]float64       `json:"finalBonuses,omitempty"`
}

type ResolvedEquipmentAffix struct {
	AffixID     string  `json:"affixId"`
	InstanceID  string  `json:"instanceId,omitempty"`
	EnhanceHits int     `json:"enhanceHits"`
	Stat        string  `json:"stat"`
	Value       float64 `json:"value"`
}

type GatheringNode struct {
	NodeID         string `json:"nodeId"`
	NodeType       string `json:"nodeType"`
	LootPoolID     string `json:"lootPoolId"`
	StaminaCost    int    `json:"staminaCost"`
	RespawnSeconds int    `json:"respawnSeconds"`
}

type Crop struct {
	CropID            string `json:"cropId"`
	SeedItemID        string `json:"seedItemId"`
	GrowthSeconds     int    `json:"growthSeconds"`
	HarvestLootPoolID string `json:"harvestLootPoolId"`
}

type BossDefinition struct {
	BossID                  string                 `json:"bossId"`
	DisplayName             string                 `json:"displayName"`
	ParticipationLootPoolID string                 `json:"participationLootPoolId"`
	LootPoolID              string                 `json:"lootPoolId"`
	ContributionTiers       []BossContributionTier `json:"contributionTiers"`
}

type BossContributionTier struct {
	MinContribution int64  `json:"minContribution"`
	BonusLootPoolID string `json:"bonusLootPoolId"`
}

type Recipe struct {
	RecipeID    string                     `json:"recipeId"`
	DisplayName string                     `json:"displayName"`
	Inputs      []RecipeMaterial           `json:"inputs"`
	Outputs     []store.DungeonRewardGrant `json:"outputs"`
}

type RecipeMaterial struct {
	ItemID   string `json:"itemId"`
	Quantity int64  `json:"quantity"`
}

func LoadDir(dir string) (*Config, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		dir = "configs/economy"
	}
	cfg := &Config{
		Dir:         dir,
		Items:       map[string]Item{},
		Shops:       map[string]Shop{},
		Floors:      map[string]DungeonFloor{},
		LootPools:   map[string]LootPool{},
		AffixPools:  map[string]AffixPool{},
		GatherNodes: map[string]GatheringNode{},
		Crops:       map[string]Crop{},
		Bosses:      map[string]BossDefinition{},
		Recipes:     map[string]Recipe{},
	}
	if err := cfg.loadItems(); err != nil {
		return nil, err
	}
	if err := cfg.loadRules(); err != nil {
		return nil, err
	}
	if err := cfg.loadEquipment(); err != nil {
		return nil, err
	}
	if err := cfg.loadShops(); err != nil {
		return nil, err
	}
	if err := cfg.loadLottery(); err != nil {
		return nil, err
	}
	if err := cfg.loadBounties(); err != nil {
		return nil, err
	}
	if err := cfg.loadAffixes(); err != nil {
		return nil, err
	}
	if err := cfg.loadLootPools(); err != nil {
		return nil, err
	}
	if err := cfg.loadDungeons(); err != nil {
		return nil, err
	}
	if err := cfg.loadGathering(); err != nil {
		return nil, err
	}
	if err := cfg.loadFarming(); err != nil {
		return nil, err
	}
	if err := cfg.loadBosses(); err != nil {
		return nil, err
	}
	if err := cfg.loadRecipes(); err != nil {
		return nil, err
	}
	if err := cfg.loadMarketplace(); err != nil {
		return nil, err
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) DungeonFloor(chapterID, floorID int) (DungeonFloor, bool) {
	row, ok := c.Floors[floorKey(chapterID, floorID)]
	return row, ok
}

func (c *Config) DungeonRewards(req store.DungeonFinishRequest) (store.DungeonRewardPlan, error) {
	floor, ok := c.DungeonFloor(req.ChapterID, req.FloorID)
	if !ok {
		return store.DungeonRewardPlan{}, fmt.Errorf("dungeon floor %d/%d is not configured", req.ChapterID, req.FloorID)
	}
	if req.Exp > floor.MaxExp {
		return store.DungeonRewardPlan{}, fmt.Errorf("exp exceeds configured cap %d", floor.MaxExp)
	}
	plan := store.DungeonRewardPlan{IsBoss: floor.IsBoss}
	if req.Result != "victory" || floor.LootPoolID == "" {
		return plan, nil
	}
	rewards, err := c.lootPoolRewards(floor.LootPoolID, req.OpID, req.DungeonRunID, req.CharacterID, req.ChapterID, req.FloorID)
	if err != nil {
		return store.DungeonRewardPlan{}, err
	}
	plan.Items = rewards.Items
	plan.TokenReward = rewards.TokenReward
	return plan, nil
}

func (c *Config) GatheringRewards(req store.ActivitySettlementRequest) (store.DungeonRewardPlan, error) {
	node, ok := c.GatherNodes[strings.TrimSpace(req.ActivityID)]
	if !ok {
		return store.DungeonRewardPlan{}, fmt.Errorf("gathering node %q is not configured", req.ActivityID)
	}
	return c.lootPoolRewards(node.LootPoolID, req.OpID, req.ActivityID, req.CharacterID, 0, 0)
}

func (c *Config) FarmingRewards(req store.ActivitySettlementRequest) (store.DungeonRewardPlan, error) {
	crop, ok := c.Crops[strings.TrimSpace(req.ActivityID)]
	if !ok {
		return store.DungeonRewardPlan{}, fmt.Errorf("crop %q is not configured", req.ActivityID)
	}
	return c.lootPoolRewards(crop.HarvestLootPoolID, req.OpID, req.ActivityID, req.CharacterID, 0, 0)
}

func (c *Config) BossRewards(req store.BossSettleRequest, contribution int64) (store.DungeonRewardPlan, error) {
	bossKey := strings.TrimSpace(req.BossKey)
	if bossKey == "" {
		return store.DungeonRewardPlan{}, errors.New("bossKey is required")
	}
	boss, ok := c.Bosses[bossKey]
	if !ok {
		return store.DungeonRewardPlan{}, fmt.Errorf("boss %q is not configured", bossKey)
	}
	refID := fmt.Sprintf("boss-%d", req.BossEventID)
	plan := store.DungeonRewardPlan{IsBoss: true}
	if boss.ParticipationLootPoolID != "" {
		participation, err := c.lootPoolRewards(boss.ParticipationLootPoolID, req.OpID+":participation", refID, req.CharacterID, 0, 0)
		if err != nil {
			return store.DungeonRewardPlan{}, err
		}
		mergeRewardPlan(&plan, participation)
	}
	if contribution > 0 && boss.LootPoolID != "" {
		main, err := c.lootPoolRewards(boss.LootPoolID, req.OpID+":main", refID, req.CharacterID, int(contribution), 0)
		if err != nil {
			return store.DungeonRewardPlan{}, err
		}
		mergeRewardPlan(&plan, main)
	}
	if tierPool := bossTierPool(boss, contribution); tierPool != "" {
		tier, err := c.lootPoolRewards(tierPool, req.OpID+":tier", refID, req.CharacterID, int(contribution), 1)
		if err != nil {
			return store.DungeonRewardPlan{}, err
		}
		mergeRewardPlan(&plan, tier)
	}
	return plan, nil
}

func (c *Config) RecipePlan(recipeID, opID string, characterID, batchCount int64) (store.DungeonRewardPlan, []RecipeMaterial, error) {
	recipeID = strings.TrimSpace(recipeID)
	if recipeID == "" {
		return store.DungeonRewardPlan{}, nil, errors.New("recipeId is required")
	}
	if batchCount <= 0 {
		batchCount = 1
	}
	recipe, ok := c.Recipes[recipeID]
	if !ok {
		return store.DungeonRewardPlan{}, nil, fmt.Errorf("recipe %q is not configured", recipeID)
	}
	inputs := make([]RecipeMaterial, len(recipe.Inputs))
	for i, input := range recipe.Inputs {
		inputs[i] = RecipeMaterial{
			ItemID:   input.ItemID,
			Quantity: input.Quantity * batchCount,
		}
	}
	plan := store.DungeonRewardPlan{}
	refID := fmt.Sprintf("recipe:%s", recipeID)
	rng := rand.New(rand.NewPCG(seed(opID, recipeID, characterID, int(batchCount), 0), 0x9e3779b97f4a7c15))
	for index, output := range recipe.Outputs {
		totalQty := output.Quantity * batchCount
		if totalQty <= 0 {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(output.RewardType)) {
		case "item":
			plan.Items = append(plan.Items, store.DungeonRewardGrant{
				RewardType: "item",
				ItemID:     output.ItemID,
				Quantity:   totalQty,
				Rarity:     itemRarity(c.Items, output.ItemID, output.Rarity),
				Category:   itemCategory(c.Items, output.ItemID),
			})
		case "equipment":
			for i := int64(0); i < totalQty; i++ {
				grant := store.DungeonRewardGrant{
					RewardType:   "equipment",
					ItemID:       output.ItemID,
					Quantity:     1,
					Rarity:       itemRarity(c.Items, output.ItemID, output.Rarity),
					Category:     itemCategory(c.Items, output.ItemID),
					EquipmentUID: equipmentUID(c.Rules.EquipmentUIDPrefix, opID, refID, index, int(i)),
					Affixes:      c.rollRewardAffixes(rng, output.ItemID, output.Rarity, output.AffixPoolID),
				}
				plan.Items = append(plan.Items, grant)
			}
		default:
			return store.DungeonRewardPlan{}, nil, fmt.Errorf("recipe %q has unsupported output type %q", recipeID, output.RewardType)
		}
	}
	return plan, inputs, nil
}

func (c *Config) BagSlots() int {
	if c.Rules.BagSlots > 0 {
		return c.Rules.BagSlots
	}
	return 25
}

func (c *Config) EffectiveBagSlots(expandCount int) int {
	base := c.BagSlots()
	slotsPer := c.BagExpandSlots()
	if expandCount < 0 {
		expandCount = 0
	}
	return base + expandCount*slotsPer
}

func (c *Config) BagExpandSlots() int {
	if c.Rules.BagExpandSlots > 0 {
		return c.Rules.BagExpandSlots
	}
	return 5
}

func (c *Config) BagExpandMaxTimes() int {
	if c.Rules.BagExpandMaxTimes > 0 {
		return c.Rules.BagExpandMaxTimes
	}
	return 10
}

func (c *Config) BagExpandPriceToken() int64 {
	if c.Rules.BagExpandPriceToken > 0 {
		return c.Rules.BagExpandPriceToken
	}
	return 50
}

func (c *Config) BagExpandPaymentTimeoutSec() int {
	if c.Rules.BagExpandPaymentTimeoutSeconds > 0 {
		return c.Rules.BagExpandPaymentTimeoutSeconds
	}
	return 600
}

func (c *Config) TradingLicensePriceToken() int64 {
	if c.Rules.TradingLicensePriceToken > 0 {
		return c.Rules.TradingLicensePriceToken
	}
	return 100
}

func (c *Config) TradingLicensePaymentTimeoutSec() int {
	if c.Rules.TradingLicensePaymentTimeoutSeconds > 0 {
		return c.Rules.TradingLicensePaymentTimeoutSeconds
	}
	return 600
}

func (c *Config) GrowthPaymentRules(depositWallet string) store.GrowthPaymentRules {
	return store.GrowthPaymentRules{
		DepositReceiverWallet:           strings.TrimSpace(depositWallet),
		BagSlots:                        c.BagSlots(),
		BagExpandSlots:                  c.BagExpandSlots(),
		BagExpandMaxTimes:               c.BagExpandMaxTimes(),
		BagExpandPriceToken:             c.BagExpandPriceToken(),
		BagExpandPaymentTimeoutSec:      c.BagExpandPaymentTimeoutSec(),
		TradingLicensePriceToken:        c.TradingLicensePriceToken(),
		TradingLicensePaymentTimeoutSec: c.TradingLicensePaymentTimeoutSec(),
	}
}

func (c *Config) EquipmentRules() store.EquipmentRules {
	r := store.EquipmentRules{
		DefaultMaxDurability:    c.Rules.EquipmentDefaultMaxDurability,
		RepairCostPerPointToken: c.Rules.EquipmentRepairCostPerPointToken,
		RepairMinCostToken:      c.Rules.EquipmentRepairMinCostToken,
		RepairBurnBps:           c.Rules.EquipmentRepairBurnBps,
		RepairRecycleBps:        c.Rules.EquipmentRepairRecycleBps,
		RepairRewardBps:         c.Rules.EquipmentRepairRewardBps,
		DungeonWearPoints:       c.Rules.EquipmentDungeonWearPoints,
		BossWearPoints:          c.Rules.EquipmentBossWearPoints,
	}
	return r.WithDefaults()
}

func (c *Config) NFTRules() store.NFTRules {
	r := store.NFTRules{
		MintFeesByRarity: c.Rules.NFTMintFeesTokenByRarity,
		MinRarity:        c.Rules.NFTMintMinRarity,
	}
	return r.WithDefaults()
}

func (c *Config) WarehouseSlots() int {
	if c.Rules.WarehouseSlots > 0 {
		return c.Rules.WarehouseSlots
	}
	return 50
}

func mergeRewardPlan(dst *store.DungeonRewardPlan, src store.DungeonRewardPlan) {
	dst.Items = append(dst.Items, src.Items...)
	dst.TokenReward += src.TokenReward
	if src.IsBoss {
		dst.IsBoss = true
	}
}

func bossTierPool(boss BossDefinition, contribution int64) string {
	var poolID string
	for _, tier := range boss.ContributionTiers {
		if contribution >= tier.MinContribution && strings.TrimSpace(tier.BonusLootPoolID) != "" {
			poolID = tier.BonusLootPoolID
		}
	}
	return poolID
}

func (c *Config) lootPoolRewards(lootPoolID, opID, refID string, characterID int64, chapterID, floorID int) (store.DungeonRewardPlan, error) {
	pool, ok := c.LootPools[lootPoolID]
	if !ok {
		return store.DungeonRewardPlan{}, fmt.Errorf("loot pool %q is not configured", lootPoolID)
	}
	plan := store.DungeonRewardPlan{}
	rng := rand.New(rand.NewPCG(seed(opID, refID, characterID, chapterID, floorID), 0x9e3779b97f4a7c15))
	for index, entry := range pool.Entries {
		if entry.DropChance < 1 && rng.Float64() > entry.DropChance {
			continue
		}
		quantity := rollQuantity(rng, entry.QuantityMin, entry.QuantityMax)
		if quantity <= 0 {
			continue
		}
		switch strings.ToLower(entry.RewardType) {
		case "item":
			plan.Items = append(plan.Items, store.DungeonRewardGrant{
				RewardType: "item",
				ItemID:     entry.ItemID,
				Quantity:   quantity,
				Rarity:     itemRarity(c.Items, entry.ItemID, entry.Rarity),
				Category:   itemCategory(c.Items, entry.ItemID),
			})
		case "equipment":
			for i := int64(0); i < quantity; i++ {
				plan.Items = append(plan.Items, store.DungeonRewardGrant{
					RewardType:   "equipment",
					ItemID:       entry.ItemID,
					Quantity:     1,
					Rarity:       itemRarity(c.Items, entry.ItemID, entry.Rarity),
					Category:     itemCategory(c.Items, entry.ItemID),
					EquipmentUID: equipmentUID(c.Rules.EquipmentUIDPrefix, opID, refID, index, int(i)),
					Affixes:      c.rollRewardAffixes(rng, entry.ItemID, entry.Rarity, entry.AffixPoolID),
				})
			}
		case "token":
			plan.TokenReward += quantity
		default:
			return store.DungeonRewardPlan{}, fmt.Errorf("unsupported reward type %q", entry.RewardType)
		}
	}
	return plan, nil
}

func (c *Config) loadItems() error {
	var file struct {
		Items []Item `json:"items"`
	}
	if err := readJSON(filepath.Join(c.Dir, "items.json"), &file); err != nil {
		return err
	}
	for _, row := range file.Items {
		row.ItemID = strings.TrimSpace(row.ItemID)
		if row.ItemID == "" {
			return errors.New("items.json contains empty itemId")
		}
		c.Items[row.ItemID] = row
	}
	return nil
}

func (c *Config) loadShops() error {
	var file struct {
		Shops []Shop `json:"shops"`
	}
	if err := readJSON(filepath.Join(c.Dir, "shops.json"), &file); err != nil {
		return err
	}
	for _, row := range file.Shops {
		row.ShopID = strings.TrimSpace(row.ShopID)
		if row.ShopID == "" {
			return errors.New("shops.json contains empty shopId")
		}
		if _, exists := c.Shops[row.ShopID]; exists {
			return fmt.Errorf("shops.json contains duplicate shopId %q", row.ShopID)
		}
		if row.Mystery == nil && strings.TrimSpace(row.DailyLimitTimezone) == "" {
			row.DailyLimitTimezone = "Asia/Shanghai"
		}
		if row.Mystery != nil && strings.TrimSpace(row.Mystery.DailyLimitTimezone) == "" {
			row.Mystery.DailyLimitTimezone = "Asia/Shanghai"
		}
		c.Shops[row.ShopID] = row
	}
	return nil
}

func (c *Config) loadRules() error {
	return readJSON(filepath.Join(c.Dir, "economy_rules.json"), &c.Rules)
}

func (c *Config) loadEquipment() error {
	if err := readJSON(filepath.Join(c.Dir, "equipment_templates.json"), &c.Equipment); err != nil {
		return err
	}
	c.Equipment.ByItemID = map[string]EquipmentTemplate{}
	for _, series := range c.Equipment.Series {
		series.SeriesID = strings.TrimSpace(series.SeriesID)
		series.Category = strings.TrimSpace(series.Category)
		series.WeaponTypeKey = strings.TrimSpace(series.WeaponTypeKey)
		weaponType := WeaponTypeNone
		weaponTypeKey := "none"
		if series.Category == "weapon" {
			var ok bool
			weaponType, weaponTypeKey, ok = normalizeWeaponType(series.WeaponType, series.WeaponTypeKey)
			if !ok || weaponType == WeaponTypeNone {
				return fmt.Errorf("weapon equipment series %q has invalid weaponType/weaponTypeKey", series.SeriesID)
			}
		}
		if series.SeriesID == "" || series.EquipSlot < 0 || len(series.BaseFlat) == 0 {
			return errors.New("equipment_templates.json contains an incomplete equipment series")
		}
		for _, stage := range series.Stages {
			itemID := strings.TrimSpace(stage.ItemID)
			if itemID == "" || stage.Stage <= 0 {
				return fmt.Errorf("equipment series %q contains an incomplete stage", series.SeriesID)
			}
			if _, exists := c.Equipment.ByItemID[itemID]; exists {
				return fmt.Errorf("equipment template itemId %q is duplicated", itemID)
			}
			npcRecycleGold := stage.NPCRecycleGold
			if len(npcRecycleGold) == 0 {
				npcRecycleGold = c.Equipment.NPCRecycleGoldByStage[stage.Stage]
			}
			template := EquipmentTemplate{
				ItemID: itemID, SeriesID: series.SeriesID, Stage: stage.Stage,
				EquipSlot: series.EquipSlot, Category: series.Category,
				WeaponType: weaponType, WeaponTypeKey: weaponTypeKey,
				DisplayName: strings.TrimSpace(stage.Prefix) + strings.TrimSpace(series.DisplayType),
				AffixPoolID: series.AffixPoolID, BaseFlat: series.BaseFlat,
				BasePercent: stage.BasePercent, NPCRecycleGold: npcRecycleGold,
			}
			c.Equipment.ByItemID[itemID] = template
			if _, exists := c.Items[itemID]; !exists {
				c.Items[itemID] = Item{ItemID: itemID, DisplayName: template.DisplayName, Category: series.Category, Rarity: 1, MaxStack: 1, IsEquipment: true, EquipSlot: series.EquipSlot, WeaponType: template.WeaponType, WeaponTypeKey: template.WeaponTypeKey, Tradable: true, DefaultBindType: "UNBOUND"}
			}
		}
	}
	for _, mount := range c.Equipment.Mounts {
		mount.ItemID = strings.TrimSpace(mount.ItemID)
		if mount.ItemID == "" || mount.EquipSlot < 0 {
			return errors.New("equipment_templates.json contains an incomplete mount")
		}
		if _, exists := c.Items[mount.ItemID]; !exists {
			c.Items[mount.ItemID] = Item{ItemID: mount.ItemID, DisplayName: mount.DisplayName, Category: "mount", Rarity: 5, MaxStack: 1, IsEquipment: true, EquipSlot: mount.EquipSlot, WeaponType: WeaponTypeNone, WeaponTypeKey: "none", Tradable: true, DefaultBindType: "UNBOUND"}
		}
	}
	return nil
}

func (c *Config) loadLottery() error {
	return readJSON(filepath.Join(c.Dir, "lottery.json"), &c.Lottery)
}

func (c *Config) loadBounties() error {
	return readJSON(filepath.Join(c.Dir, "bounties.json"), &c.Bounties)
}

func (c *Config) EquipmentTemplate(itemID string) (EquipmentTemplate, bool) {
	row, ok := c.Equipment.ByItemID[strings.TrimSpace(itemID)]
	return row, ok
}

func (c *Config) EquipmentRarity(rarity int) (EquipmentRarity, bool) {
	row, ok := c.Equipment.Rarities[rarity]
	return row, ok
}

func (c *Config) Shop(shopID string) (Shop, bool) {
	row, ok := c.Shops[strings.TrimSpace(shopID)]
	return row, ok
}

func (shop Shop) SellsItem(itemID string) bool {
	_, ok := shop.SellItem(itemID)
	return ok
}

func (shop Shop) SellItem(itemID string) (ShopSellItem, bool) {
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return ShopSellItem{}, false
	}
	if shop.SellAllItems {
		return ShopSellItem{ItemID: itemID}, true
	}
	for _, candidate := range shop.SellItems {
		if strings.TrimSpace(candidate.ItemID) == itemID {
			return candidate, true
		}
	}
	return ShopSellItem{}, false
}

func (c *Config) ShopPurchasePlan(opID string, characterID int64, itemID string, quantity int64, rarityOverride int) (store.DungeonRewardPlan, error) {
	item, ok := c.Items[strings.TrimSpace(itemID)]
	if !ok {
		return store.DungeonRewardPlan{}, fmt.Errorf("unknown item %q", itemID)
	}
	if quantity <= 0 {
		return store.DungeonRewardPlan{}, errors.New("quantity must be positive")
	}
	if item.GrantGold > 0 {
		return store.DungeonRewardPlan{}, nil
	}
	if item.GrantLockedAEB > 0 {
		return store.DungeonRewardPlan{TokenReward: item.GrantLockedAEB * quantity}, nil
	}
	category := strings.TrimSpace(item.Category)
	if category == "" {
		category = "item"
	}
	if !item.IsEquipment {
		return store.DungeonRewardPlan{Items: []store.DungeonRewardGrant{{
			RewardType: "item",
			ItemID:     item.ItemID,
			Quantity:   quantity,
			Rarity:     item.Rarity,
			Category:   category,
		}}}, nil
	}
	rarity := rarityOverride
	if rarity <= 0 {
		rarity = item.Rarity
		if rarity <= 0 {
			rarity = 1
		}
	}
	plan := store.DungeonRewardPlan{Items: make([]store.DungeonRewardGrant, 0, quantity)}
	for i := int64(0); i < quantity; i++ {
		affixes := []store.EquipmentAffix{}
		if _, ok := c.EquipmentTemplate(item.ItemID); ok {
			rng := rand.New(rand.NewPCG(seed(opID, "shop", characterID, int(i), 0), 0x4cf5ad432745937f))
			affixes = c.rollEquipmentAffixes(rng, item.ItemID, rarity)
		}
		plan.Items = append(plan.Items, store.DungeonRewardGrant{
			RewardType:   "equipment",
			ItemID:       item.ItemID,
			Quantity:     1,
			Rarity:       rarity,
			Category:     category,
			EquipmentUID: equipmentUID(c.Rules.EquipmentUIDPrefix, opID, "shop", int(i), 0),
			Affixes:      affixes,
		})
	}
	return plan, nil
}

func (c *Config) MysteryShopPurchasePlan(opID string, characterID int64, offer store.MysteryShopOffer) (store.DungeonRewardPlan, error) {
	itemID := strings.TrimSpace(offer.ItemID)
	quantity := offer.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	plan, err := c.ShopPurchasePlan(opID, characterID, itemID, quantity, 0)
	if err != nil {
		return store.DungeonRewardPlan{}, err
	}
	if offer.Rarity > 0 {
		for index := range plan.Items {
			if strings.EqualFold(plan.Items[index].RewardType, "equipment") {
				plan.Items[index].Rarity = offer.Rarity
				plan.Items[index].Affixes = c.rollEquipmentAffixes(rand.New(rand.NewPCG(seed(opID, "mystery", characterID, index, 0), 0x4cf5ad432745937f)), plan.Items[index].ItemID, offer.Rarity)
			}
		}
	}
	return plan, nil
}

func (c *Config) EnhancementRule(itemID string, rarity, nextLevel int) (gold int64, stoneItemID string, stoneQuantity int64, err error) {
	template, ok := c.EquipmentTemplate(itemID)
	if !ok {
		return 0, "", 0, fmt.Errorf("equipment template %q is not configured", itemID)
	}
	if _, ok := c.EquipmentRarity(rarity); !ok {
		return 0, "", 0, fmt.Errorf("equipment rarity %d is not configured", rarity)
	}
	enhancement := c.Equipment.Enhancement
	if nextLevel < 1 || nextLevel > enhancement.MaxLevel {
		return 0, "", 0, fmt.Errorf("enhancement level %d is not available", nextLevel)
	}
	goldCosts := enhancement.GoldCostByStage[template.Stage]
	if len(goldCosts) < nextLevel || goldCosts[nextLevel-1] < 0 {
		return 0, "", 0, fmt.Errorf("missing gold enhancement cost for stage %d level %d", template.Stage, nextLevel)
	}
	gold = goldCosts[nextLevel-1]
	if nextLevel <= 5 {
		return gold, "", 0, nil
	}
	stoneItemID = enhancement.StoneItemByStage[template.Stage]
	baseCosts := enhancement.StoneBaseCostByStageRarity[template.Stage]
	if stoneItemID == "" || len(baseCosts) < rarity || len(enhancement.StoneProgressionByLevel) < nextLevel {
		return 0, "", 0, fmt.Errorf("missing enhancement stone cost for stage %d rarity %d", template.Stage, rarity)
	}
	stoneQuantity = baseCosts[rarity-1] + enhancement.StoneProgressionByLevel[nextLevel-1]
	if stoneQuantity <= 0 {
		return 0, "", 0, fmt.Errorf("invalid enhancement stone cost for stage %d rarity %d", template.Stage, rarity)
	}
	return gold, stoneItemID, stoneQuantity, nil
}

// LotteryPlan snapshots all generated rewards at order creation. Payment
// fulfillment never rerolls, so later configuration releases cannot alter a
// paid order's result.
func (c *Config) LotteryPlan(opID string, characterID int64, characterLevel, count int) (store.DungeonRewardPlan, error) {
	if count < 1 || count > c.Lottery.MaxCount {
		return store.DungeonRewardPlan{}, fmt.Errorf("lottery count must be 1..%d", c.Lottery.MaxCount)
	}
	rng := rand.New(rand.NewPCG(seed(opID, "lottery", characterID, characterLevel, count), 0x9e3779b97f4a7c15))
	plan := store.DungeonRewardPlan{}
	for index := 0; index < count; index++ {
		category := weightedString(rng, c.Lottery.CategoryWeight)
		switch category {
		case "equipment":
			rarity := weightedInt(rng, c.Lottery.EquipmentRarityWeight)
			candidates := make([]EquipmentTemplate, 0)
			for _, template := range c.Equipment.ByItemID {
				if template.Stage <= characterLevel {
					candidates = append(candidates, template)
				}
			}
			if len(candidates) == 0 {
				return store.DungeonRewardPlan{}, errors.New("no lottery equipment is available at character level")
			}
			template := candidates[rng.IntN(len(candidates))]
			plan.Items = append(plan.Items, store.DungeonRewardGrant{RewardType: "equipment", ItemID: template.ItemID, Quantity: 1, Rarity: rarity, Category: template.Category, EquipmentUID: equipmentUID(c.Rules.EquipmentUIDPrefix, opID, "lottery", index, 0), Affixes: c.rollEquipmentAffixes(rng, template.ItemID, rarity)})
		case "rare_material":
			itemID := c.Lottery.RareMaterialItemIDs[rng.IntN(len(c.Lottery.RareMaterialItemIDs))]
			plan.Items = append(plan.Items, store.DungeonRewardGrant{RewardType: "item", ItemID: itemID, Quantity: 1, Rarity: itemRarity(c.Items, itemID, 0), Category: itemCategory(c.Items, itemID)})
		case "boss_ticket":
			itemID := c.Lottery.BossTicketItemIDs[rng.IntN(len(c.Lottery.BossTicketItemIDs))]
			plan.Items = append(plan.Items, store.DungeonRewardGrant{RewardType: "item", ItemID: itemID, Quantity: 1, Rarity: itemRarity(c.Items, itemID, 0), Category: itemCategory(c.Items, itemID)})
		case "mount":
			itemID := c.Lottery.MountItemIDs[rng.IntN(len(c.Lottery.MountItemIDs))]
			plan.Items = append(plan.Items, store.DungeonRewardGrant{RewardType: "equipment", ItemID: itemID, Quantity: 1, Rarity: 5, Category: "mount", EquipmentUID: equipmentUID(c.Rules.EquipmentUIDPrefix, opID, "lottery-mount", index, 0)})
		default:
			return store.DungeonRewardPlan{}, fmt.Errorf("unsupported lottery category %q", category)
		}
	}
	return plan, nil
}

func (c *Config) BountyTaskPlan(opID string, characterID int64, slotIndex int, premium bool) (BountyTaskPlan, error) {
	if slotIndex < 1 || slotIndex > 5 {
		return BountyTaskPlan{}, errors.New("bounty slot must be 1..5")
	}
	eligible := make([]BountyTaskTemplate, 0)
	for _, task := range c.Bounties.TaskTemplates {
		if premium {
			if task.Difficulty == "rare" {
				eligible = append(eligible, task)
			}
		} else if task.Difficulty == "normal" {
			eligible = append(eligible, task)
		}
	}
	if len(eligible) == 0 {
		return BountyTaskPlan{}, errors.New("no bounty template is available")
	}
	rng := rand.New(rand.NewPCG(seed(opID, "bounty", characterID, slotIndex, 0), 0x517cc1b727220a95))
	total := 0
	for _, task := range eligible {
		total += task.Weight
	}
	pick := rng.IntN(total)
	selected := eligible[len(eligible)-1]
	for _, task := range eligible {
		if pick < task.Weight {
			selected = task
			break
		}
		pick -= task.Weight
	}
	required := rollQuantity(rng, selected.QuantityMin, selected.QuantityMax)
	return BountyTaskPlan{TemplateID: selected.TemplateID, Type: selected.Type, Difficulty: selected.Difficulty, ItemID: selected.ItemID, MinRarity: selected.MinRarity, Required: required, RewardItemID: selected.RewardItemID, RewardQuantity: selected.RewardQuantity}, nil
}

func weightedString(rng *rand.Rand, rows map[string]int) string {
	total := 0
	keys := make([]string, 0, len(rows))
	for key, weight := range rows {
		keys = append(keys, key)
		total += weight
	}
	sort.Strings(keys)
	pick := rng.IntN(total)
	for _, value := range keys {
		weight := rows[value]
		if pick < weight {
			return value
		}
		pick -= weight
	}
	return ""
}

func weightedInt(rng *rand.Rand, rows map[int]int) int {
	total := 0
	keys := make([]int, 0, len(rows))
	for key, weight := range rows {
		keys = append(keys, key)
		total += weight
	}
	sort.Ints(keys)
	pick := rng.IntN(total)
	for _, value := range keys {
		weight := rows[value]
		if pick < weight {
			return value
		}
		pick -= weight
	}
	return 0
}

func (c *Config) loadMarketplace() error {
	path := filepath.Join(c.Dir, "marketplace.json")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		c.Marketplace = MarketplaceConfig{Enabled: false}
		return nil
	}
	if err := readJSON(path, &c.Marketplace); err != nil {
		return err
	}
	if strings.TrimSpace(c.Marketplace.TokenSymbol) == "" {
		c.Marketplace.TokenSymbol = "AEB"
	}
	return nil
}

func (c *Config) MarketplaceRules() store.MarketplaceRules {
	m := c.Marketplace
	return store.MarketplaceRules{
		Enabled:                       m.Enabled,
		MinListPrice:                  m.MinListPrice,
		FeeBps:                        m.FeeBps,
		FeeBurnBps:                    m.FeeBurnBps,
		FeeTreasuryBps:                m.FeeTreasuryBps,
		FeeRewardsBps:                 m.FeeRewardsBps,
		ListingDepositBps:             m.ListingDepositBps,
		BaseListingSlots:              m.BaseListingSlots,
		MaterialExpandSlots:           m.MaterialExpandSlots,
		MaterialExpandMaxTimes:        m.MaterialExpandMaxTimes,
		MaterialExpandItemID:          m.MaterialExpandItemID,
		MaterialExpandItemQuantity:    m.MaterialExpandItemQuantity,
		WalletExpandSlots:             m.WalletExpandSlots,
		WalletExpandMaxTimes:          m.WalletExpandMaxTimes,
		WalletExpandPriceToken:        m.WalletExpandPriceToken,
		WalletExpandPaymentTimeoutSec: m.WalletExpandPaymentTimeoutSeconds,
		MaxListingsCreatedPerDay:      m.MaxListingsCreatedPerAccountPerDay,
		MaxCancelsPerDay:              m.MaxCancelsPerAccountPerDay,
		MaxPurchasesPerDay:            m.MaxPurchasesPerAccountPerDay,
		PurchaseCooldownSeconds:       m.PurchaseCooldownSeconds,
		DailyLimitTimezone:            m.DailyLimitTimezone,
		DefaultCooldownHours:          m.DefaultCooldownHours,
	}
}

func (c *Config) loadDungeons() error {
	var file struct {
		Chapters []struct {
			ChapterID int            `json:"chapterId"`
			Name      string         `json:"name"`
			Floors    []DungeonFloor `json:"floors"`
		} `json:"chapters"`
	}
	if err := readJSON(filepath.Join(c.Dir, "dungeons.json"), &file); err != nil {
		return err
	}
	for _, chapter := range file.Chapters {
		for _, floor := range chapter.Floors {
			floor.ChapterID = chapter.ChapterID
			c.Floors[floorKey(floor.ChapterID, floor.FloorID)] = floor
		}
	}
	return nil
}

func (c *Config) loadLootPools() error {
	var file struct {
		LootPools []LootPool `json:"lootPools"`
	}
	if err := readJSON(filepath.Join(c.Dir, "loot_pools.json"), &file); err != nil {
		return err
	}
	for _, row := range file.LootPools {
		row.LootPoolID = strings.TrimSpace(row.LootPoolID)
		if row.LootPoolID == "" {
			return errors.New("loot_pools.json contains empty lootPoolId")
		}
		c.LootPools[row.LootPoolID] = row
	}
	return nil
}

func (c *Config) loadAffixes() error {
	var file struct {
		AffixPools []AffixPool `json:"affixPools"`
	}
	if err := readJSON(filepath.Join(c.Dir, "equipment_affixes.json"), &file); err != nil {
		return err
	}
	for _, row := range file.AffixPools {
		row.AffixPoolID = strings.TrimSpace(row.AffixPoolID)
		if row.AffixPoolID == "" {
			return errors.New("equipment_affixes.json contains empty affixPoolId")
		}
		c.AffixPools[row.AffixPoolID] = row
	}
	return nil
}

func (c *Config) loadGathering() error {
	var file struct {
		GatheringNodes []GatheringNode `json:"gatheringNodes"`
	}
	if err := readJSON(filepath.Join(c.Dir, "gathering.json"), &file); err != nil {
		return err
	}
	for _, row := range file.GatheringNodes {
		c.GatherNodes[row.NodeID] = row
	}
	return nil
}

func (c *Config) loadFarming() error {
	var file struct {
		Crops []Crop `json:"crops"`
	}
	if err := readJSON(filepath.Join(c.Dir, "farming.json"), &file); err != nil {
		return err
	}
	for _, row := range file.Crops {
		c.Crops[row.CropID] = row
	}
	return nil
}

func (c *Config) loadBosses() error {
	var file struct {
		Bosses []BossDefinition `json:"bosses"`
	}
	if err := readJSON(filepath.Join(c.Dir, "bosses.json"), &file); err != nil {
		return err
	}
	for _, row := range file.Bosses {
		row.BossID = strings.TrimSpace(row.BossID)
		if row.BossID == "" {
			return errors.New("bosses.json contains empty bossId")
		}
		c.Bosses[row.BossID] = row
	}
	return nil
}

func (c *Config) loadRecipes() error {
	var file struct {
		Recipes []struct {
			RecipeID    string             `json:"recipeId"`
			DisplayName string             `json:"displayName"`
			Inputs      []RecipeMaterial   `json:"inputs"`
			Outputs     []recipeOutputJSON `json:"outputs"`
		} `json:"recipes"`
	}
	if err := readJSON(filepath.Join(c.Dir, "recipes.json"), &file); err != nil {
		return err
	}
	for _, row := range file.Recipes {
		recipeID := strings.TrimSpace(row.RecipeID)
		if recipeID == "" {
			return errors.New("recipes.json contains empty recipeId")
		}
		recipe := Recipe{
			RecipeID:    recipeID,
			DisplayName: row.DisplayName,
			Inputs:      row.Inputs,
			Outputs:     make([]store.DungeonRewardGrant, 0, len(row.Outputs)),
		}
		for _, output := range row.Outputs {
			recipe.Outputs = append(recipe.Outputs, store.DungeonRewardGrant{
				RewardType:  output.RewardType,
				ItemID:      output.ItemID,
				Quantity:    output.Quantity,
				Rarity:      output.Rarity,
				AffixPoolID: output.AffixPoolID,
			})
		}
		c.Recipes[recipeID] = recipe
	}
	return nil
}

type recipeOutputJSON struct {
	RewardType  string `json:"rewardType"`
	ItemID      string `json:"itemId"`
	Quantity    int64  `json:"quantity"`
	Rarity      int    `json:"rarity"`
	AffixPoolID string `json:"affixPoolId"`
}

func (c *Config) validate() error {
	for itemID, item := range c.Items {
		if item.BuyCurrency < 0 || item.BuyPrice < 0 || item.SellPrice < 0 || item.GrantGold < 0 || item.GrantLockedAEB < 0 {
			return fmt.Errorf("item %q has invalid shop pricing fields", itemID)
		}
		if item.BuyCurrency > 1 {
			return fmt.Errorf("item %q has unsupported buyCurrency %d", itemID, item.BuyCurrency)
		}
		if item.WeaponType != 0 || strings.TrimSpace(item.WeaponTypeKey) != "" {
			if _, _, ok := normalizeWeaponType(item.WeaponType, item.WeaponTypeKey); !ok {
				return fmt.Errorf("item %q has invalid weaponType/weaponTypeKey", itemID)
			}
		}
	}
	for shopID, shop := range c.Shops {
		seenSellSlots := map[int]bool{}
		for _, sellItem := range shop.SellItems {
			itemID := strings.TrimSpace(sellItem.ItemID)
			if itemID == "" {
				return fmt.Errorf("shop %q contains an empty sellItems entry", shopID)
			}
			if _, ok := c.Items[itemID]; !ok {
				return fmt.Errorf("shop %q references unknown item %q", shopID, itemID)
			}
			if sellItem.MinLevel < 0 || sellItem.MaxLevel < 0 || sellItem.Rarity < 0 || sellItem.BuyPrice < 0 || sellItem.DailyLimit < 0 {
				return fmt.Errorf("shop %q item %q has invalid sell item fields", shopID, itemID)
			}
			if shop.Mystery == nil {
				if sellItem.SlotIndex <= 0 || sellItem.DailyLimit <= 0 {
					return fmt.Errorf("shop %q item %q requires slotIndex and dailyLimit", shopID, itemID)
				}
				if seenSellSlots[sellItem.SlotIndex] {
					return fmt.Errorf("shop %q duplicates slot %d", shopID, sellItem.SlotIndex)
				}
				seenSellSlots[sellItem.SlotIndex] = true
			}
			if sellItem.MaxLevel > 0 && sellItem.MinLevel > sellItem.MaxLevel {
				return fmt.Errorf("shop %q item %q has invalid level range", shopID, itemID)
			}
			if sellItem.Rarity > 0 {
				if _, ok := c.Equipment.Rarities[sellItem.Rarity]; !ok {
					return fmt.Errorf("shop %q item %q has unknown rarity %d", shopID, itemID, sellItem.Rarity)
				}
			}
		}
		if shop.Mystery != nil {
			if shop.Mystery.ManualRefreshTokenBaseCost < 0 || shop.Mystery.ManualRefreshTokenStepCost < 0 || shop.Mystery.ManualRefreshTokenMaxCost < 0 || shop.Mystery.MaxSlots <= 0 {
				return fmt.Errorf("mystery shop %q has invalid refresh cost or maxSlots", shopID)
			}
			if shop.Mystery.ManualRefreshTokenMaxCost > 0 && shop.Mystery.ManualRefreshTokenMaxCost < shop.Mystery.ManualRefreshTokenBaseCost {
				return fmt.Errorf("mystery shop %q has invalid refresh max cost", shopID)
			}
			if strings.TrimSpace(shop.Mystery.DailyLimitTimezone) == "" {
				shop.Mystery.DailyLimitTimezone = "Asia/Shanghai"
			}
			if len(shop.Mystery.Slots) == 0 || len(shop.Mystery.Slots) > shop.Mystery.MaxSlots {
				return fmt.Errorf("mystery shop %q has invalid slot count", shopID)
			}
			discountWeight := 0
			for _, discount := range shop.Mystery.Discounts {
				if discount.Bps <= 0 || discount.Bps > 10000 || discount.Weight <= 0 {
					return fmt.Errorf("mystery shop %q has invalid discount", shopID)
				}
				discountWeight += discount.Weight
			}
			if discountWeight <= 0 {
				return fmt.Errorf("mystery shop %q requires discount weights", shopID)
			}
			seenSlots := map[int]bool{}
			for _, slot := range shop.Mystery.Slots {
				if slot.SlotIndex <= 0 || slot.SlotIndex > shop.Mystery.MaxSlots || slot.UnlockLevel < 0 || len(slot.Pools) == 0 {
					return fmt.Errorf("mystery shop %q has invalid slot %d", shopID, slot.SlotIndex)
				}
				if seenSlots[slot.SlotIndex] {
					return fmt.Errorf("mystery shop %q duplicates slot %d", shopID, slot.SlotIndex)
				}
				seenSlots[slot.SlotIndex] = true
				poolWeight := 0
				for _, entry := range slot.Pools {
					itemID := strings.TrimSpace(entry.ItemID)
					if itemID == "" || entry.Weight <= 0 || entry.Quantity <= 0 {
						return fmt.Errorf("mystery shop %q slot %d has invalid pool entry", shopID, slot.SlotIndex)
					}
					if entry.BaseGold <= 0 && entry.BaseToken <= 0 {
						return fmt.Errorf("mystery shop %q slot %d item %q requires a price", shopID, slot.SlotIndex, itemID)
					}
					item, ok := c.Items[itemID]
					if !ok {
						return fmt.Errorf("mystery shop %q slot %d references unknown item %q", shopID, slot.SlotIndex, itemID)
					}
					if mysteryEntryRequiresToken(item, itemID) && entry.BaseToken <= 0 {
						return fmt.Errorf("mystery shop %q slot %d item %q must use token pricing", shopID, slot.SlotIndex, itemID)
					}
					if entry.MinDiscount < 0 || entry.MinDiscount > 10000 {
						return fmt.Errorf("mystery shop %q slot %d item %q has invalid minDiscount", shopID, slot.SlotIndex, itemID)
					}
					poolWeight += entry.Weight
				}
				if poolWeight <= 0 {
					return fmt.Errorf("mystery shop %q slot %d requires positive pool weight", shopID, slot.SlotIndex)
				}
			}
		}
	}
	if len(c.Equipment.StageMultipliers) != 7 || len(c.Equipment.Rarities) != 6 {
		return errors.New("equipment_templates.json must define seven stages and six rarities")
	}
	for stage, multiplier := range c.Equipment.StageMultipliers {
		if stage <= 0 || multiplier <= 0 {
			return errors.New("equipment stage multipliers must be positive")
		}
	}
	for rarity, row := range c.Equipment.Rarities {
		if rarity <= 0 || row.Multiplier <= 0 || row.AffixCount <= 0 {
			return errors.New("equipment rarities must have positive multiplier and affixCount")
		}
	}
	for itemID, template := range c.Equipment.ByItemID {
		if _, ok := c.Equipment.StageMultipliers[template.Stage]; !ok {
			return fmt.Errorf("equipment template %q has unsupported stage %d", itemID, template.Stage)
		}
		if _, ok := c.AffixPools[template.AffixPoolID]; !ok {
			return fmt.Errorf("equipment template %q references unknown affix pool %q", itemID, template.AffixPoolID)
		}
		if len(template.NPCRecycleGold) != 6 {
			return fmt.Errorf("equipment template %q must define six npcRecycleGold values", itemID)
		}
		for rarity, gold := range template.NPCRecycleGold {
			if _, ok := c.Equipment.Rarities[rarity]; !ok || gold < 0 {
				return fmt.Errorf("equipment template %q has invalid npcRecycleGold", itemID)
			}
		}
	}
	for _, mount := range c.Equipment.Mounts {
		if mount.EquipSlot != 7 || mount.FinalHPBps != 500 || mount.FinalAtkBps != 500 || mount.MoveSpeedBps != 2500 {
			return fmt.Errorf("mount %q must use the fixed current mount bonuses", mount.ItemID)
		}
	}
	enhancement := c.Equipment.Enhancement
	if enhancement.MaxLevel != 10 || len(enhancement.StoneProgressionByLevel) != enhancement.MaxLevel {
		return errors.New("equipment enhancement must define the current +10 progression")
	}
	for stage := range c.Equipment.StageMultipliers {
		if len(enhancement.GoldCostByStage[stage]) != enhancement.MaxLevel || len(enhancement.StoneBaseCostByStageRarity[stage]) != len(c.Equipment.Rarities) {
			return fmt.Errorf("equipment enhancement is incomplete for stage %d", stage)
		}
		stoneID := enhancement.StoneItemByStage[stage]
		if _, ok := c.Items[stoneID]; !ok {
			return fmt.Errorf("equipment enhancement stage %d references unknown stone %q", stage, stoneID)
		}
	}
	if c.Lottery.PriceAEB <= 0 || c.Lottery.MaxCount < 1 || len(c.Lottery.CategoryWeight) == 0 || len(c.Lottery.EquipmentRarityWeight) == 0 {
		return errors.New("lottery.json is incomplete")
	}
	if err := validateWeights(c.Lottery.CategoryWeight); err != nil {
		return fmt.Errorf("lottery category weights: %w", err)
	}
	if err := validateWeights(c.Lottery.EquipmentRarityWeight); err != nil {
		return fmt.Errorf("lottery equipment rarity weights: %w", err)
	}
	for _, itemID := range append(append([]string{}, c.Lottery.RareMaterialItemIDs...), append(c.Lottery.BossTicketItemIDs, c.Lottery.MountItemIDs...)...) {
		if _, ok := c.Items[itemID]; !ok {
			return fmt.Errorf("lottery.json references unknown item %q", itemID)
		}
	}
	if len(c.Bounties.Slots) != 5 || c.Bounties.Refresh.FreeCooldownSeconds <= 0 {
		return errors.New("bounties.json is incomplete")
	}
	for _, task := range c.Bounties.TaskTemplates {
		if strings.TrimSpace(task.TemplateID) == "" || task.QuantityMin <= 0 || task.QuantityMax < task.QuantityMin || task.Weight <= 0 {
			return errors.New("bounties.json contains an invalid task template")
		}
		if task.ItemID != "" {
			if _, ok := c.Items[task.ItemID]; !ok {
				return fmt.Errorf("bounty task %q references unknown item %q", task.TemplateID, task.ItemID)
			}
		}
		if _, ok := c.Items[task.RewardItemID]; !ok {
			return fmt.Errorf("bounty task %q references unknown reward %q", task.TemplateID, task.RewardItemID)
		}
	}
	for badge, lottery := range c.Bounties.BadgeLottery {
		if _, ok := c.Items[lottery.CostItemID]; !ok {
			return fmt.Errorf("bounty badge lottery %q references unknown cost item %q", badge, lottery.CostItemID)
		}
		weights := map[string]int{}
		for index, reward := range lottery.Rewards {
			if reward.Weight <= 0 || reward.Min < 0 || reward.Max < reward.Min {
				return fmt.Errorf("bounty badge lottery %q has invalid reward", badge)
			}
			if reward.Type == "item" {
				if _, ok := c.Items[reward.ItemID]; !ok {
					return fmt.Errorf("bounty badge lottery %q references unknown reward %q", badge, reward.ItemID)
				}
			}
			weights[fmt.Sprintf("%d", index)] = reward.Weight
		}
		if err := validateWeights(weights); err != nil {
			return fmt.Errorf("bounty badge lottery %q: %w", badge, err)
		}
	}
	for poolID, pool := range c.LootPools {
		for _, entry := range pool.Entries {
			if entry.RewardType != "token" {
				if _, ok := c.Items[entry.ItemID]; !ok {
					return fmt.Errorf("loot pool %q references unknown item %q", poolID, entry.ItemID)
				}
			}
			if entry.AffixPoolID != "" {
				if _, ok := c.AffixPools[entry.AffixPoolID]; !ok {
					return fmt.Errorf("loot pool %q references unknown affix pool %q", poolID, entry.AffixPoolID)
				}
			}
		}
	}
	for poolID, pool := range c.AffixPools {
		if len(pool.Affixes) == 0 {
			return fmt.Errorf("affix pool %q has no affixes", poolID)
		}
		if err := validateAffixPool(pool, c.Equipment.Rarities); err != nil {
			return fmt.Errorf("affix pool %q: %w", poolID, err)
		}
	}
	requiredBossTickets := map[string]string{
		floorKey(0, 10): "boss_ticket_ashen_threshold",
		floorKey(1, 20): "boss_ticket_gloomwood",
		floorKey(2, 30): "boss_ticket_voidscar",
	}
	if len(c.Floors) != 30 {
		return fmt.Errorf("dungeons.json must define 30 floors, got %d", len(c.Floors))
	}
	for floorID := 1; floorID <= 30; floorID++ {
		chapterID := (floorID - 1) / 10
		key := floorKey(chapterID, floorID)
		floor, ok := c.Floors[key]
		if !ok {
			return fmt.Errorf("dungeons.json missing floor %s", key)
		}
		if floor.MaxExp <= 0 {
			return fmt.Errorf("dungeon floor %s has invalid maxExp", key)
		}
		if strings.TrimSpace(floor.LootPoolID) == "" {
			return fmt.Errorf("dungeon floor %s is missing lootPoolId", key)
		}
		if _, ok := c.LootPools[floor.LootPoolID]; !ok {
			return fmt.Errorf("dungeon floor %s references unknown loot pool %q", key, floor.LootPoolID)
		}
		if ticketID, isBoss := requiredBossTickets[key]; isBoss {
			if !floor.IsBoss {
				return fmt.Errorf("dungeon floor %s must be marked isBoss", key)
			}
			if !enterCostHasItem(floor.EnterCost, ticketID, 1) {
				return fmt.Errorf("dungeon floor %s must require %s", key, ticketID)
			}
		}
	}
	for nodeID, node := range c.GatherNodes {
		if _, ok := c.LootPools[node.LootPoolID]; !ok {
			return fmt.Errorf("gathering node %q references unknown loot pool %q", nodeID, node.LootPoolID)
		}
	}
	for cropID, crop := range c.Crops {
		if _, ok := c.Items[crop.SeedItemID]; !ok {
			return fmt.Errorf("crop %q references unknown seed item %q", cropID, crop.SeedItemID)
		}
		if _, ok := c.LootPools[crop.HarvestLootPoolID]; !ok {
			return fmt.Errorf("crop %q references unknown harvest loot pool %q", cropID, crop.HarvestLootPoolID)
		}
	}
	for bossID, boss := range c.Bosses {
		if boss.ParticipationLootPoolID != "" {
			if _, ok := c.LootPools[boss.ParticipationLootPoolID]; !ok {
				return fmt.Errorf("boss %q references unknown participation loot pool %q", bossID, boss.ParticipationLootPoolID)
			}
		}
		if boss.LootPoolID != "" {
			if _, ok := c.LootPools[boss.LootPoolID]; !ok {
				return fmt.Errorf("boss %q references unknown loot pool %q", bossID, boss.LootPoolID)
			}
		}
		for _, tier := range boss.ContributionTiers {
			if tier.BonusLootPoolID != "" {
				if _, ok := c.LootPools[tier.BonusLootPoolID]; !ok {
					return fmt.Errorf("boss %q references unknown tier loot pool %q", bossID, tier.BonusLootPoolID)
				}
			}
		}
	}
	for recipeID, recipe := range c.Recipes {
		for _, input := range recipe.Inputs {
			if _, ok := c.Items[input.ItemID]; !ok {
				return fmt.Errorf("recipe %q references unknown input item %q", recipeID, input.ItemID)
			}
		}
		for _, output := range recipe.Outputs {
			if strings.ToLower(output.RewardType) != "token" {
				if _, ok := c.Items[output.ItemID]; !ok {
					return fmt.Errorf("recipe %q references unknown output item %q", recipeID, output.ItemID)
				}
			}
			if output.AffixPoolID != "" {
				if _, ok := c.AffixPools[output.AffixPoolID]; !ok {
					return fmt.Errorf("recipe %q references unknown affix pool %q", recipeID, output.AffixPoolID)
				}
			}
		}
	}
	if c.Marketplace.Enabled {
		itemID := strings.TrimSpace(c.Marketplace.MaterialExpandItemID)
		if itemID == "" {
			itemID = "market_stall_permit"
		}
		if _, ok := c.Items[itemID]; !ok {
			return fmt.Errorf("marketplace materialExpandItemId %q is not configured in items.json", itemID)
		}
	}
	return nil
}

func mysteryEntryRequiresToken(item Item, itemID string) bool {
	category := strings.TrimSpace(item.Category)
	if item.Rarity >= 3 {
		return true
	}
	if category == "boss_ticket" || category == "aeb_voucher" {
		return true
	}
	itemID = strings.TrimSpace(itemID)
	return strings.HasPrefix(itemID, "enhancement_stone_t10") ||
		strings.HasPrefix(itemID, "enhancement_stone_t15") ||
		strings.HasPrefix(itemID, "enhancement_stone_t20") ||
		strings.HasPrefix(itemID, "enhancement_stone_t25") ||
		strings.HasPrefix(itemID, "enhancement_stone_t30")
}

func validateWeights[K ~string | ~int](rows map[K]int) error {
	total := 0
	for _, weight := range rows {
		if weight < 0 {
			return errors.New("weight must not be negative")
		}
		total += weight
	}
	if total <= 0 {
		return errors.New("at least one weight must be positive")
	}
	return nil
}

func normalizeWeaponType(value int, key string) (int, string, bool) {
	key = strings.TrimSpace(key)
	if key == "" {
		key = "none"
	}
	keyValue, ok := weaponTypeKeys[key]
	if !ok {
		return 0, "", false
	}
	if value == 0 {
		value = keyValue
	}
	if value != keyValue {
		return 0, "", false
	}
	return value, key, true
}

func (c *Config) rollAffixes(rng *rand.Rand, affixPoolID string) []store.EquipmentAffix {
	pool, ok := c.AffixPools[affixPoolID]
	if !ok || len(pool.Affixes) == 0 {
		return []store.EquipmentAffix{}
	}
	rolls := pool.Rolls.Min
	if pool.Rolls.Max > pool.Rolls.Min {
		rolls += rng.IntN(pool.Rolls.Max - pool.Rolls.Min + 1)
	}
	if rolls <= 0 {
		return []store.EquipmentAffix{}
	}
	out := make([]store.EquipmentAffix, 0, rolls)
	for i := 0; i < rolls; i++ {
		affix := weightedAffix(rng, pool.Affixes)
		value := affix.Min
		if affix.Max > affix.Min {
			value += rng.Float64() * (affix.Max - affix.Min)
		}
		out = append(out, store.EquipmentAffix{
			AffixID: affix.AffixID,
			Stat:    affix.Stat,
			Value:   value,
		})
	}
	return out
}

func (c *Config) rollRewardAffixes(rng *rand.Rand, itemID string, rarity int, fallbackPoolID string) []store.EquipmentAffix {
	if _, ok := c.EquipmentTemplate(itemID); ok {
		return c.rollEquipmentAffixes(rng, itemID, rarity)
	}
	return c.rollAffixes(rng, fallbackPoolID)
}

// rollEquipmentAffixes rolls the semantic form used by all new equipment. A
// key can occur at most twice; duplicate keys have the same initial config
// value and are independently strengthen-able through InstanceID.
func (c *Config) rollEquipmentAffixes(rng *rand.Rand, itemID string, rarity int) []store.EquipmentAffix {
	template, ok := c.EquipmentTemplate(itemID)
	if !ok {
		return []store.EquipmentAffix{}
	}
	rarityConfig, ok := c.EquipmentRarity(rarity)
	if !ok {
		return []store.EquipmentAffix{}
	}
	pool, ok := c.AffixPools[template.AffixPoolID]
	if !ok {
		return []store.EquipmentAffix{}
	}
	counts := map[string]int{}
	out := make([]store.EquipmentAffix, 0, rarityConfig.AffixCount)
	for i := 0; i < rarityConfig.AffixCount; i++ {
		affix, ok := weightedEligibleAffix(rng, pool.Affixes, counts)
		if !ok {
			break
		}
		counts[affix.AffixID]++
		out = append(out, store.EquipmentAffix{AffixID: affix.AffixID, InstanceID: fmt.Sprintf("a%d", i+1)})
	}
	return out
}

// ResolveEquipment performs live stat calculation. It intentionally accepts
// only semantic instance fields for new equipment; changing JSON therefore
// immediately revalues existing equipment and NFT views.
func (c *Config) ResolveEquipment(itemID string, rarity int, affixes []store.EquipmentAffix) (ResolvedEquipment, error) {
	itemID = strings.TrimSpace(itemID)
	for _, mount := range c.Equipment.Mounts {
		if mount.ItemID == itemID {
			return ResolvedEquipment{
				ItemID: itemID, Rarity: 5, IsMount: true, WeaponType: WeaponTypeNone, WeaponTypeKey: "none",
				FinalBonuses: map[string]float64{"finalMaxHp": float64(mount.FinalHPBps) / 10000, "finalAttack": float64(mount.FinalAtkBps) / 10000, "moveSpeed": float64(mount.MoveSpeedBps) / 10000},
			}, nil
		}
	}
	template, ok := c.EquipmentTemplate(itemID)
	if !ok {
		return ResolvedEquipment{}, fmt.Errorf("equipment template %q is not configured", itemID)
	}
	rarityConfig, ok := c.EquipmentRarity(rarity)
	if !ok {
		return ResolvedEquipment{}, fmt.Errorf("equipment rarity %d is not configured", rarity)
	}
	stageMultiplier := c.Equipment.StageMultipliers[template.Stage]
	resolved := ResolvedEquipment{
		ItemID: itemID, Rarity: rarity, WeaponType: template.WeaponType, WeaponTypeKey: template.WeaponTypeKey,
		BaseFlat: map[string]float64{}, BasePercent: cloneStats(template.BasePercent),
		TotalFlat: map[string]float64{}, TotalPercent: cloneStats(template.BasePercent),
		Affixes: make([]ResolvedEquipmentAffix, 0, len(affixes)),
	}
	for stat, value := range template.BaseFlat {
		resolved.BaseFlat[stat] = value * stageMultiplier * rarityConfig.Multiplier
		resolved.TotalFlat[stat] = resolved.BaseFlat[stat]
	}
	pool := c.AffixPools[template.AffixPoolID]
	byID := map[string]AffixConfig{}
	for _, affix := range pool.Affixes {
		byID[affix.AffixID] = affix
	}
	for index, instance := range affixes {
		affix, exists := byID[instance.AffixID]
		if !exists {
			// Legacy items retain their stored display value until migrated.
			if instance.Stat != "" {
				resolved.Affixes = append(resolved.Affixes, ResolvedEquipmentAffix{AffixID: instance.AffixID, InstanceID: instance.InstanceID, EnhanceHits: instance.EnhanceHits, Stat: instance.Stat, Value: instance.Value})
				addResolvedStat(&resolved, instance.Stat, instance.Value)
				continue
			}
			return ResolvedEquipment{}, fmt.Errorf("equipment %q contains unknown affix %q", itemID, instance.AffixID)
		}
		value, err := affixValue(affix, template.Stage, rarity)
		if err != nil {
			return ResolvedEquipment{}, err
		}
		if instance.EnhanceHits < 0 {
			return ResolvedEquipment{}, fmt.Errorf("equipment %q affix %q has negative enhancement", itemID, instance.AffixID)
		}
		value *= 1 + float64(instance.EnhanceHits)*0.10
		instanceID := instance.InstanceID
		if instanceID == "" {
			instanceID = fmt.Sprintf("legacy-%d", index+1)
		}
		resolved.Affixes = append(resolved.Affixes, ResolvedEquipmentAffix{AffixID: affix.AffixID, InstanceID: instanceID, EnhanceHits: instance.EnhanceHits, Stat: affix.Stat, Value: value})
		addResolvedStat(&resolved, affix.Stat, value)
	}
	return resolved, nil
}

// ResolveEquipmentItem enriches a response object only. The returned Affixes
// contain display values, but persistence always receives the original
// semantic form produced by rollEquipmentAffixes.
func (c *Config) ResolveEquipmentItem(item store.EquipmentItem) (store.EquipmentItem, error) {
	if item.WeaponTypeKey == "" {
		item.WeaponTypeKey = "none"
	}
	if _, template := c.EquipmentTemplate(item.ItemID); !template {
		isMount := false
		for _, mount := range c.Equipment.Mounts {
			if mount.ItemID == item.ItemID {
				isMount = true
				break
			}
		}
		if !isMount {
			if itemDef, ok := c.Items[item.ItemID]; ok {
				if weaponType, weaponTypeKey, ok := normalizeWeaponType(itemDef.WeaponType, itemDef.WeaponTypeKey); ok && weaponType != WeaponTypeNone {
					item.WeaponType = weaponType
					item.WeaponTypeKey = weaponTypeKey
				}
			}
			return item, nil // legacy equipment keeps its legacy payload.
		}
	}
	resolved, err := c.ResolveEquipment(item.ItemID, item.Rarity, item.Affixes)
	if err != nil {
		return store.EquipmentItem{}, err
	}
	item.ResolvedBaseFlat = resolved.BaseFlat
	item.ResolvedBasePercent = resolved.BasePercent
	item.ResolvedFlatStats = resolved.TotalFlat
	item.ResolvedPercentStats = resolved.TotalPercent
	item.FinalBonuses = resolved.FinalBonuses
	item.WeaponType = resolved.WeaponType
	item.WeaponTypeKey = resolved.WeaponTypeKey
	item.Affixes = make([]store.EquipmentAffix, 0, len(resolved.Affixes))
	for _, affix := range resolved.Affixes {
		item.Affixes = append(item.Affixes, store.EquipmentAffix{
			AffixID: affix.AffixID, InstanceID: affix.InstanceID, EnhanceHits: affix.EnhanceHits,
			Stat: affix.Stat, Value: affix.Value,
		})
	}
	return item, nil
}

func validateAffixPool(pool AffixPool, rarities map[int]EquipmentRarity) error {
	if err := validateWeights(map[string]int{"pool": totalAffixWeight(pool.Affixes)}); err != nil {
		return err
	}
	for _, affix := range pool.Affixes {
		if strings.TrimSpace(affix.AffixID) == "" || strings.TrimSpace(affix.Stat) == "" || affix.Weight < 0 {
			return errors.New("contains invalid affix identity or weight")
		}
		if len(affix.ValueByStage) == 0 {
			continue // legacy random-value pools
		}
		for stage, values := range affix.ValueByStage {
			if stage <= 0 || len(values) != len(rarities) {
				return fmt.Errorf("affix %q must define one value per rarity at every stage", affix.AffixID)
			}
			for _, value := range values {
				if value < 0 {
					return fmt.Errorf("affix %q has negative configured value", affix.AffixID)
				}
			}
		}
	}
	return nil
}

func totalAffixWeight(rows []AffixConfig) int {
	total := 0
	for _, row := range rows {
		total += row.Weight
	}
	return total
}

func weightedEligibleAffix(rng *rand.Rand, rows []AffixConfig, counts map[string]int) (AffixConfig, bool) {
	total := 0
	for _, row := range rows {
		if row.Weight > 0 && counts[row.AffixID] < 2 {
			total += row.Weight
		}
	}
	if total <= 0 {
		return AffixConfig{}, false
	}
	pick := rng.IntN(total)
	for _, row := range rows {
		if row.Weight <= 0 || counts[row.AffixID] >= 2 {
			continue
		}
		if pick < row.Weight {
			return row, true
		}
		pick -= row.Weight
	}
	return AffixConfig{}, false
}

func affixValue(affix AffixConfig, stage, rarity int) (float64, error) {
	values, ok := affix.ValueByStage[stage]
	if !ok || rarity < 1 || rarity > len(values) {
		return 0, fmt.Errorf("affix %q has no configured value for stage %d rarity %d", affix.AffixID, stage, rarity)
	}
	return values[rarity-1], nil
}

func cloneStats(src map[string]float64) map[string]float64 {
	dst := map[string]float64{}
	for stat, value := range src {
		dst[stat] = value
	}
	return dst
}

func addResolvedStat(resolved *ResolvedEquipment, stat string, value float64) {
	switch stat {
	case "attack", "defense", "maxHp", "regen":
		resolved.TotalFlat[stat] += value
	default:
		resolved.TotalPercent[stat] += value
	}
}

func weightedAffix(rng *rand.Rand, rows []AffixConfig) AffixConfig {
	total := 0
	for _, row := range rows {
		if row.Weight > 0 {
			total += row.Weight
		}
	}
	if total <= 0 {
		return rows[rng.IntN(len(rows))]
	}
	pick := rng.IntN(total)
	for _, row := range rows {
		if row.Weight <= 0 {
			continue
		}
		if pick < row.Weight {
			return row
		}
		pick -= row.Weight
	}
	return rows[len(rows)-1]
}

func readJSON(path string, dst any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func rollQuantity(rng *rand.Rand, min, max int64) int64 {
	if min <= 0 {
		min = 1
	}
	if max < min {
		max = min
	}
	if max == min {
		return min
	}
	return min + int64(rng.Int64N(max-min+1))
}

func itemRarity(items map[string]Item, itemID string, fallback int) int {
	if fallback > 0 {
		return fallback
	}
	return items[itemID].Rarity
}

func itemCategory(items map[string]Item, itemID string) string {
	category := items[itemID].Category
	if category == "" {
		return "misc"
	}
	return category
}

func seed(opID, dungeonRunID string, characterID int64, chapterID, floorID int) uint64 {
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s:%s:%d:%d:%d", opID, dungeonRunID, characterID, chapterID, floorID)
	return h.Sum64()
}

func equipmentUID(prefix, opID, dungeonRunID string, entryIndex, rollIndex int) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "equipment"
	}
	h := fnv.New64a()
	_, _ = fmt.Fprintf(h, "%s:%s:%d:%d", opID, dungeonRunID, entryIndex, rollIndex)
	return fmt.Sprintf("%s-%016x", prefix, h.Sum64())
}

func floorKey(chapterID, floorID int) string {
	return fmt.Sprintf("%d:%d", chapterID, floorID)
}

func enterCostHasItem(cost store.DungeonCost, itemID string, quantity int64) bool {
	for _, item := range cost.Items {
		if item.ItemID == itemID && item.Quantity >= quantity {
			return true
		}
	}
	return false
}
