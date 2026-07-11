package rules

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"

	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type Config struct {
	Dir         string
	Items       map[string]Item
	Floors      map[string]DungeonFloor
	LootPools   map[string]LootPool
	AffixPools  map[string]AffixPool
	GatherNodes map[string]GatheringNode
	Crops       map[string]Crop
	Bosses      map[string]BossDefinition
	Recipes     map[string]Recipe
	Marketplace MarketplaceConfig
	Rules       EconomyRules
}

type Item struct {
	ItemID          string `json:"itemId"`
	DisplayName     string `json:"displayName"`
	Category        string `json:"category"`
	Rarity          int    `json:"rarity"`
	MaxStack        int64  `json:"maxStack"`
	IsEquipment     bool   `json:"isEquipment"`
	EquipSlot       int    `json:"equipSlot"`
	Tradable        bool   `json:"tradable"`
	DefaultBindType string `json:"defaultBindType"`
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
	DefaultTokenCooldownHours           int    `json:"defaultTokenCooldownHours"`
	EquipmentUIDPrefix                  string `json:"equipmentUidPrefix"`
	BagSlots                            int    `json:"bagSlots"`
	WarehouseSlots                      int    `json:"warehouseSlots"`
	BagExpandSlots                      int    `json:"bagExpandSlots"`
	BagExpandMaxTimes                   int    `json:"bagExpandMaxTimes"`
	BagExpandPriceToken                 int64  `json:"bagExpandPriceToken"`
	BagExpandPaymentTimeoutSeconds      int    `json:"bagExpandPaymentTimeoutSeconds"`
	TradingLicensePriceToken            int64  `json:"tradingLicensePriceToken"`
	TradingLicensePaymentTimeoutSeconds int    `json:"tradingLicensePaymentTimeoutSeconds"`
	EquipmentDefaultMaxDurability       int    `json:"equipmentDefaultMaxDurability"`
	EquipmentRepairCostPerPointToken    int64  `json:"equipmentRepairCostPerPointToken"`
	EquipmentRepairMinCostToken         int64  `json:"equipmentRepairMinCostToken"`
	EquipmentRepairBurnBps              int64  `json:"equipmentRepairBurnBps"`
	EquipmentRepairRecycleBps           int64  `json:"equipmentRepairRecycleBps"`
	EquipmentRepairRewardBps            int64  `json:"equipmentRepairRewardBps"`
	EquipmentDungeonWearPoints          int    `json:"equipmentDungeonWearPoints"`
	EquipmentBossWearPoints             int    `json:"equipmentBossWearPoints"`
	NFTMintFeeToken                     int64  `json:"nftMintFeeToken"`
	NFTMintMinRarity                    int    `json:"nftMintMinRarity"`
}

type DungeonFloor struct {
	ChapterID  int
	FloorID    int               `json:"floorId"`
	IsBoss     bool              `json:"isBoss"`
	EnterCost  store.DungeonCost `json:"enterCost"`
	MaxExp     int64             `json:"maxExp"`
	LootPoolID string            `json:"lootPoolId"`
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
	AffixID string  `json:"affixId"`
	Stat    string  `json:"stat"`
	Min     float64 `json:"min"`
	Max     float64 `json:"max"`
	Weight  int     `json:"weight"`
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
					Affixes:      c.rollAffixes(rng, output.AffixPoolID),
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
		MintFeeToken: c.Rules.NFTMintFeeToken,
		MinRarity:    c.Rules.NFTMintMinRarity,
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
					Affixes:      c.rollAffixes(rng, entry.AffixPoolID),
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

func (c *Config) loadRules() error {
	return readJSON(filepath.Join(c.Dir, "economy_rules.json"), &c.Rules)
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
	for key, floor := range c.Floors {
		if floor.LootPoolID != "" {
			if _, ok := c.LootPools[floor.LootPoolID]; !ok {
				return fmt.Errorf("dungeon floor %q references unknown loot pool %q", key, floor.LootPoolID)
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
