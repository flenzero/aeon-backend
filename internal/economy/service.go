package economy

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/chain"
	"github.com/flenzero/aeon-backend/internal/economy/rules"
	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type Service struct {
	cfg          config.Config
	store        store.Repository
	economyRules *rules.Config
	rulesErr     error
}

func NewService(cfg config.Config, st store.Repository) *Service {
	economyRules, err := rules.LoadDir(cfg.EconomyConfigDir)
	return &Service{cfg: cfg, store: st, economyRules: economyRules, rulesErr: err}
}

func (s *Service) Ready(context.Context) error {
	return s.rulesErr
}

func (s *Service) Snapshot(accountID, characterID int64) (store.EconomySnapshot, error) {
	snapshot, err := s.store.EconomySnapshot(accountID, characterID)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	return s.resolveSnapshot(snapshot)
}

func (s *Service) WarehouseDeposit(req store.EconomyActionRequest) (store.EconomySnapshot, error) {
	snapshot, err := s.store.WarehouseDeposit(req)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	return s.resolveSnapshot(snapshot)
}

func (s *Service) WarehouseWithdraw(req store.EconomyActionRequest) (store.EconomySnapshot, error) {
	snapshot, err := s.store.WarehouseWithdraw(req)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	return s.resolveSnapshot(snapshot)
}

func (s *Service) EquipItem(req store.EconomyActionRequest) (store.EconomySnapshot, error) {
	snapshot, err := s.store.EquipItem(req)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	return s.resolveSnapshot(snapshot)
}

func (s *Service) UnequipItem(req store.EconomyActionRequest) (store.EconomySnapshot, error) {
	snapshot, err := s.store.UnequipItem(req)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	return s.resolveSnapshot(snapshot)
}

func (s *Service) EquipmentRepair(req store.EquipmentRepairRequest) (store.EquipmentRepairResult, error) {
	if s.rulesErr != nil {
		return store.EquipmentRepairResult{}, s.rulesErr
	}
	req.Rules = s.economyRules.EquipmentRules()
	result, err := s.store.EquipmentRepair(req)
	if err != nil {
		return store.EquipmentRepairResult{}, err
	}
	result.Snapshot, err = s.resolveSnapshot(result.Snapshot)
	if err != nil {
		return store.EquipmentRepairResult{}, err
	}
	return result, nil
}

func (s *Service) EquipmentEnhance(opID string, accountID, characterID int64, equipmentUID string) (store.EquipmentEnhanceResult, error) {
	if s.rulesErr != nil {
		return store.EquipmentEnhanceResult{}, s.rulesErr
	}
	snapshot, err := s.store.EconomySnapshot(accountID, characterID)
	if err != nil {
		return store.EquipmentEnhanceResult{}, err
	}
	var equipment *store.EquipmentItem
	for index := range snapshot.Equipment {
		if snapshot.Equipment[index].EquipmentUID == strings.TrimSpace(equipmentUID) {
			equipment = &snapshot.Equipment[index]
			break
		}
	}
	if equipment == nil {
		return store.EquipmentEnhanceResult{}, store.ErrNotFound
	}
	if equipment.NFTContract != nil {
		return store.EquipmentEnhanceResult{}, errors.New("nft-linked equipment cannot be enhanced")
	}
	gold, stoneID, stoneQuantity, err := s.economyRules.EnhancementRule(equipment.ItemID, equipment.Rarity, equipment.EnhanceLevel+1)
	if err != nil {
		return store.EquipmentEnhanceResult{}, err
	}
	result, err := s.store.EquipmentEnhance(store.EquipmentEnhanceRequest{
		OpID: opID, AccountID: accountID, CharacterID: characterID, EquipmentUID: equipmentUID,
		MaxLevel: s.economyRules.Equipment.Enhancement.MaxLevel, GoldCost: gold, StoneItemID: stoneID, StoneQuantity: stoneQuantity,
	})
	if err != nil {
		return store.EquipmentEnhanceResult{}, err
	}
	result.Equipment, err = s.economyRules.ResolveEquipmentItem(result.Equipment)
	if err != nil {
		return store.EquipmentEnhanceResult{}, err
	}
	result.Snapshot, err = s.resolveSnapshot(result.Snapshot)
	if err != nil {
		return store.EquipmentEnhanceResult{}, err
	}
	return result, nil
}

func (s *Service) EquipmentNPCRecycle(opID string, accountID, characterID int64, equipmentUID string) (store.EquipmentNPCRecycleResult, error) {
	if s.rulesErr != nil {
		return store.EquipmentNPCRecycleResult{}, s.rulesErr
	}
	snapshot, err := s.store.EconomySnapshot(accountID, characterID)
	if err != nil {
		return store.EquipmentNPCRecycleResult{}, err
	}
	for _, equipment := range snapshot.Equipment {
		if equipment.EquipmentUID != strings.TrimSpace(equipmentUID) {
			continue
		}
		if equipment.NFTContract != nil {
			return store.EquipmentNPCRecycleResult{}, errors.New("nft-linked equipment cannot be sold to an npc")
		}
		template, ok := s.economyRules.EquipmentTemplate(equipment.ItemID)
		if !ok {
			return store.EquipmentNPCRecycleResult{}, errors.New("only current equipment templates can be sold to an npc")
		}
		gold, ok := template.NPCRecycleGold[equipment.Rarity]
		if !ok {
			return store.EquipmentNPCRecycleResult{}, errors.New("npc recycle value is not configured")
		}
		result, err := s.store.EquipmentNPCRecycle(store.EquipmentNPCRecycleRequest{OpID: opID, AccountID: accountID, CharacterID: characterID, EquipmentUID: equipmentUID, GoldCredit: gold})
		if err != nil {
			return store.EquipmentNPCRecycleResult{}, err
		}
		result.Snapshot, err = s.resolveSnapshot(result.Snapshot)
		if err != nil {
			return store.EquipmentNPCRecycleResult{}, err
		}
		return result, nil
	}
	return store.EquipmentNPCRecycleResult{}, store.ErrNotFound
}

func (s *Service) PurgeExpiredNPCRecycledEquipment(now time.Time, limit int) (int64, error) {
	return s.store.PurgeExpiredNPCRecycledEquipment(now, limit)
}

func (s *Service) ShopBuy(opID string, accountID, characterID int64, shopID, itemID string, quantity int64) (store.ShopBuyResult, error) {
	if s.rulesErr != nil {
		return store.ShopBuyResult{}, s.rulesErr
	}
	shop, ok := s.economyRules.Shop(shopID)
	if !ok {
		return store.ShopBuyResult{}, errors.New("shop does not exist")
	}
	item, ok := s.economyRules.Items[strings.TrimSpace(itemID)]
	if !ok {
		return store.ShopBuyResult{}, errors.New("item does not exist")
	}
	if !shop.SellsItem(item.ItemID) {
		return store.ShopBuyResult{}, errors.New("item is not sold by this shop")
	}
	if quantity <= 0 {
		return store.ShopBuyResult{}, errors.New("quantity must be positive")
	}
	if item.BuyPrice <= 0 {
		return store.ShopBuyResult{}, errors.New("buy price is not configured")
	}
	plan, err := s.economyRules.ShopPurchasePlan(opID, characterID, item.ItemID, quantity)
	if err != nil {
		return store.ShopBuyResult{}, err
	}
	req := store.ShopBuyRequest{
		OpID:        opID,
		AccountID:   accountID,
		CharacterID: characterID,
		ShopID:      strings.TrimSpace(shopID),
		ItemID:      item.ItemID,
		Quantity:    quantity,
		RewardPlan:  plan,
		GrantGold:   item.GrantGold * quantity,
		ConfigSnapshot: map[string]any{
			"shopId":      strings.TrimSpace(shopID),
			"itemId":      item.ItemID,
			"quantity":    quantity,
			"unitPrice":   item.BuyPrice,
			"buyCurrency": item.BuyCurrency,
		},
	}
	if item.BuyCurrency == 1 {
		req.TokenCost = item.BuyPrice * quantity
		req.ReceiverWallet = strings.TrimSpace(s.cfg.SolanaDepositWallet)
		return s.store.CreateShopBuyPayment(req)
	}
	req.GoldCost = item.BuyPrice * quantity
	result, err := s.store.ShopBuyGold(req)
	if err != nil {
		return store.ShopBuyResult{}, err
	}
	result.Snapshot, err = s.resolveSnapshot(result.Snapshot)
	if err != nil {
		return store.ShopBuyResult{}, err
	}
	return result, nil
}

func (s *Service) ShopSell(opID string, accountID, characterID int64, shopID string, slotIndex int, quantity int64, equipmentUID string) (store.ShopSellResult, error) {
	if s.rulesErr != nil {
		return store.ShopSellResult{}, s.rulesErr
	}
	if _, ok := s.economyRules.Shop(shopID); !ok {
		return store.ShopSellResult{}, errors.New("shop does not exist")
	}
	if strings.TrimSpace(equipmentUID) != "" {
		return s.shopSellEquipment(opID, accountID, characterID, shopID, equipmentUID)
	}
	if slotIndex < 0 {
		return store.ShopSellResult{}, errors.New("slotIndex is required")
	}
	if quantity <= 0 {
		return store.ShopSellResult{}, errors.New("quantity must be positive")
	}
	snapshot, err := s.store.EconomySnapshot(accountID, characterID)
	if err != nil {
		return store.ShopSellResult{}, err
	}
	var inventory *store.InventoryItem
	for index := range snapshot.Inventory {
		if snapshot.Inventory[index].Slot == slotIndex {
			inventory = &snapshot.Inventory[index]
			break
		}
	}
	if inventory == nil {
		return store.ShopSellResult{}, store.ErrNotFound
	}
	item, ok := s.economyRules.Items[inventory.ItemID]
	if !ok {
		return store.ShopSellResult{}, errors.New("item is not configured")
	}
	if item.SellPrice <= 0 {
		return store.ShopSellResult{}, errors.New("sell price is not configured")
	}
	result, err := s.store.ShopSell(store.ShopSellRequest{
		OpID:        opID,
		AccountID:   accountID,
		CharacterID: characterID,
		ShopID:      strings.TrimSpace(shopID),
		SlotIndex:   slotIndex,
		Quantity:    quantity,
		GoldCredit:  item.SellPrice * quantity,
	})
	if err != nil {
		return store.ShopSellResult{}, err
	}
	result.Snapshot, err = s.resolveSnapshot(result.Snapshot)
	if err != nil {
		return store.ShopSellResult{}, err
	}
	return result, nil
}

func (s *Service) shopSellEquipment(opID string, accountID, characterID int64, shopID, equipmentUID string) (store.ShopSellResult, error) {
	snapshot, err := s.store.EconomySnapshot(accountID, characterID)
	if err != nil {
		return store.ShopSellResult{}, err
	}
	var equipment *store.EquipmentItem
	for index := range snapshot.Equipment {
		if snapshot.Equipment[index].EquipmentUID == strings.TrimSpace(equipmentUID) {
			equipment = &snapshot.Equipment[index]
			break
		}
	}
	if equipment == nil || equipment.Status != "IN_BAG" {
		return store.ShopSellResult{}, store.ErrNotFound
	}
	if equipment.NFTContract != nil {
		return store.ShopSellResult{}, errors.New("nft-linked equipment cannot be sold to a shop")
	}
	item, ok := s.economyRules.Items[equipment.ItemID]
	if !ok {
		return store.ShopSellResult{}, errors.New("equipment item is not configured")
	}
	if item.SellPrice <= 0 {
		return store.ShopSellResult{}, errors.New("sell price is not configured")
	}
	result, err := s.store.ShopSell(store.ShopSellRequest{
		OpID:         opID,
		AccountID:    accountID,
		CharacterID:  characterID,
		ShopID:       strings.TrimSpace(shopID),
		EquipmentUID: strings.TrimSpace(equipmentUID),
		Quantity:     1,
		GoldCredit:   item.SellPrice,
	})
	if err != nil {
		return store.ShopSellResult{}, err
	}
	result.Snapshot, err = s.resolveSnapshot(result.Snapshot)
	if err != nil {
		return store.ShopSellResult{}, err
	}
	return result, nil
}

func (s *Service) RequestNFTMint(req store.NFTMintRequestInput) (store.NFTMintRequestResult, error) {
	if s.rulesErr != nil {
		return store.NFTMintRequestResult{}, s.rulesErr
	}
	req.Rules = s.economyRules.NFTRules()
	result, err := s.store.RequestNFTMint(req)
	if err != nil {
		return store.NFTMintRequestResult{}, err
	}
	result.Snapshot, err = s.resolveSnapshot(result.Snapshot)
	if err != nil {
		return store.NFTMintRequestResult{}, err
	}
	return result, nil
}

func (s *Service) ConfirmNFTMint(req store.NFTMintConfirmInput) (store.NFTMintRequestResult, error) {
	if s.cfg.StubMode == config.StubDisabled {
		return store.NFTMintRequestResult{}, errors.New("Metaplex Core mint verification adapter is not configured")
	}
	result, err := s.store.ConfirmNFTMint(req)
	if err != nil {
		return store.NFTMintRequestResult{}, err
	}
	result.Snapshot, err = s.resolveSnapshot(result.Snapshot)
	if err != nil {
		return store.NFTMintRequestResult{}, err
	}
	return result, nil
}

func (s *Service) CancelNFTMint(opID string, accountID, requestID int64) (store.NFTMintRequestResult, error) {
	result, err := s.store.CancelNFTMint(opID, accountID, requestID)
	if err != nil {
		return store.NFTMintRequestResult{}, err
	}
	result.Snapshot, err = s.resolveSnapshot(result.Snapshot)
	if err != nil {
		return store.NFTMintRequestResult{}, err
	}
	return result, nil
}

func (s *Service) ListNFTAssets(accountID int64) ([]store.NFTAsset, error) {
	return s.store.ListNFTAssets(accountID)
}

func (s *Service) DungeonEnter(req store.DungeonEnterRequest) (store.DungeonResult, error) {
	if s.rulesErr != nil {
		return store.DungeonResult{}, s.rulesErr
	}
	if floor, ok := s.economyRules.DungeonFloor(req.ChapterID, req.FloorID); ok {
		req.IsBoss = floor.IsBoss
		req.Cost = floor.EnterCost
	}
	return s.store.DungeonEnter(req)
}

func (s *Service) DungeonFinish(req store.DungeonFinishRequest) (store.DungeonResult, error) {
	if s.rulesErr != nil {
		return store.DungeonResult{}, s.rulesErr
	}
	req.Result = strings.ToLower(strings.TrimSpace(req.Result))
	plan, err := s.economyRules.DungeonRewards(req)
	if err != nil {
		return store.DungeonResult{}, err
	}
	req.RewardPlan = plan
	eq := s.economyRules.EquipmentRules()
	req.EquipmentWearPoints = eq.DungeonWearPoints
	req.DefaultMaxDurability = eq.DefaultMaxDurability
	result, err := s.store.DungeonFinish(req)
	if err != nil {
		return store.DungeonResult{}, err
	}
	return s.resolveDungeonResult(result)
}

func (s *Service) LootClaim(req store.LootActionRequest) (store.EconomySnapshot, error) {
	snapshot, err := s.store.LootClaim(req)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	return s.resolveSnapshot(snapshot)
}

func (s *Service) LootClaimAll(req store.LootActionRequest) (store.EconomySnapshot, error) {
	snapshot, err := s.store.LootClaimAll(req)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	return s.resolveSnapshot(snapshot)
}

func (s *Service) LootDiscard(req store.LootActionRequest) (store.EconomySnapshot, error) {
	snapshot, err := s.store.LootDiscard(req)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	return s.resolveSnapshot(snapshot)
}

func (s *Service) GatheringSettle(req store.ActivitySettlementRequest) (store.ActivitySettlementResult, error) {
	if s.rulesErr != nil {
		return store.ActivitySettlementResult{}, s.rulesErr
	}
	req.ActivityType = "gathering"
	plan, err := s.economyRules.GatheringRewards(req)
	if err != nil {
		return store.ActivitySettlementResult{}, err
	}
	req.RewardPlan = plan
	result, err := s.store.GatheringSettle(req)
	if err != nil {
		return store.ActivitySettlementResult{}, err
	}
	result.Snapshot, err = s.resolveSnapshot(result.Snapshot)
	if err != nil {
		return store.ActivitySettlementResult{}, err
	}
	return result, nil
}

func (s *Service) FarmingHarvest(req store.ActivitySettlementRequest) (store.ActivitySettlementResult, error) {
	if s.rulesErr != nil {
		return store.ActivitySettlementResult{}, s.rulesErr
	}
	req.ActivityType = "farming"
	plan, err := s.economyRules.FarmingRewards(req)
	if err != nil {
		return store.ActivitySettlementResult{}, err
	}
	req.RewardPlan = plan
	result, err := s.store.FarmingHarvest(req)
	if err != nil {
		return store.ActivitySettlementResult{}, err
	}
	result.Snapshot, err = s.resolveSnapshot(result.Snapshot)
	if err != nil {
		return store.ActivitySettlementResult{}, err
	}
	return result, nil
}

func (s *Service) BossContribute(req store.BossContributeRequest) (store.BossContributeResult, error) {
	return s.store.BossContribute(req)
}

func (s *Service) BossSettle(req store.BossSettleRequest) (store.BossSettleResult, error) {
	if s.rulesErr != nil {
		return store.BossSettleResult{}, s.rulesErr
	}
	contribution, bossKey, err := s.store.BossContribution(req.AccountID, req.BossEventID)
	if err != nil {
		return store.BossSettleResult{}, err
	}
	if strings.TrimSpace(req.BossKey) == "" {
		req.BossKey = bossKey
	}
	plan, err := s.economyRules.BossRewards(req, contribution)
	if err != nil {
		return store.BossSettleResult{}, err
	}
	req.RewardPlan = plan
	eq := s.economyRules.EquipmentRules()
	req.EquipmentWearPoints = eq.BossWearPoints
	req.DefaultMaxDurability = eq.DefaultMaxDurability
	result, err := s.store.BossSettle(req)
	if err != nil {
		return store.BossSettleResult{}, err
	}
	result.Snapshot, err = s.resolveSnapshot(result.Snapshot)
	if err != nil {
		return store.BossSettleResult{}, err
	}
	return result, nil
}

func (s *Service) BossOpenEvent(req store.BossOpenEventRequest) (store.BossEvent, error) {
	if s.rulesErr != nil {
		return store.BossEvent{}, s.rulesErr
	}
	bossKey := strings.TrimSpace(req.BossKey)
	if bossKey == "" {
		return store.BossEvent{}, errors.New("bossKey is required")
	}
	if _, ok := s.economyRules.Bosses[bossKey]; !ok {
		return store.BossEvent{}, errors.New("bossKey is not configured")
	}
	req.BossKey = bossKey
	if req.StartsAt.IsZero() {
		req.StartsAt = time.Now().UTC()
	}
	if req.EndsAt.IsZero() {
		req.EndsAt = req.StartsAt.Add(2 * time.Hour)
	}
	return s.store.BossOpenEvent(req)
}

func (s *Service) BossCloseEvent(req store.BossCloseEventRequest) (store.BossEvent, error) {
	return s.store.BossCloseEvent(req)
}

func (s *Service) BossMarkSettled(req store.BossMarkSettledRequest) (store.BossEvent, error) {
	return s.store.BossMarkSettled(req)
}

func (s *Service) BossListActiveEvents() ([]store.BossEvent, error) {
	return s.store.BossListActiveEvents()
}

func (s *Service) InventoryOrganize(req store.EconomyActionRequest) (store.EconomySnapshot, error) {
	bagSlots := 25
	if s.economyRules != nil {
		snap, err := s.store.EconomySnapshot(req.AccountID, req.CharacterID)
		if err != nil {
			return store.EconomySnapshot{}, err
		}
		bagSlots = s.economyRules.EffectiveBagSlots(snap.BagExpandCount)
	}
	snapshot, err := s.store.InventoryOrganize(req, bagSlots)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	return s.resolveSnapshot(snapshot)
}

func (s *Service) WarehouseOrganize(req store.EconomyActionRequest) (store.EconomySnapshot, error) {
	warehouseSlots := 50
	if s.economyRules != nil {
		warehouseSlots = s.economyRules.WarehouseSlots()
	}
	snapshot, err := s.store.WarehouseOrganize(req, warehouseSlots)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	return s.resolveSnapshot(snapshot)
}

func (s *Service) InventoryDiscard(req store.InventoryDiscardRequest) (store.EconomySnapshot, error) {
	snapshot, err := s.store.InventoryDiscard(req)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	return s.resolveSnapshot(snapshot)
}

func (s *Service) Synthesize(req store.SynthesizeRequest) (store.EconomySnapshot, error) {
	if s.rulesErr != nil {
		return store.EconomySnapshot{}, s.rulesErr
	}
	if req.BatchCount <= 0 {
		req.BatchCount = 1
	}
	plan, inputs, err := s.economyRules.RecipePlan(req.RecipeID, req.OpID, req.CharacterID, req.BatchCount)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	req.RewardPlan = plan
	req.Inputs = make([]store.MaterialCost, len(inputs))
	for i, input := range inputs {
		req.Inputs[i] = store.MaterialCost{ItemID: input.ItemID, Quantity: input.Quantity}
	}
	snapshot, err := s.store.Synthesize(req)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	return s.resolveSnapshot(snapshot)
}

func (s *Service) resolveSnapshot(snapshot store.EconomySnapshot) (store.EconomySnapshot, error) {
	if s.economyRules == nil {
		return snapshot, s.rulesErr
	}
	snapshot.BagSlots = s.economyRules.EffectiveBagSlots(snapshot.BagExpandCount)
	for index, equipment := range snapshot.Equipment {
		resolved, err := s.economyRules.ResolveEquipmentItem(equipment)
		if err != nil {
			return store.EconomySnapshot{}, err
		}
		snapshot.Equipment[index] = resolved
	}
	return snapshot, nil
}

func (s *Service) resolveDungeonResult(result store.DungeonResult) (store.DungeonResult, error) {
	var err error
	result.Snapshot, err = s.resolveSnapshot(result.Snapshot)
	if err != nil {
		return store.DungeonResult{}, err
	}
	for index, equipment := range result.Rewards.EquipmentItems {
		resolved, err := s.economyRules.ResolveEquipmentItem(equipment)
		if err != nil {
			return store.DungeonResult{}, err
		}
		result.Rewards.EquipmentItems[index] = resolved
	}
	return result, nil
}

func (s *Service) marketplaceRules() store.MarketplaceRules {
	if s.economyRules == nil {
		return store.MarketplaceRules{Enabled: false}
	}
	rules := s.economyRules.MarketplaceRules()
	rules.DepositReceiverWallet = strings.TrimSpace(s.cfg.SolanaDepositWallet)
	return rules
}

func (s *Service) chainConfig() store.ChainScanConfig {
	return store.ChainScanConfig{
		Network:          s.cfg.SolanaNetwork,
		TokenMint:        s.cfg.SolanaTokenMint,
		TokenDecimals:    s.cfg.SolanaTokenDecimals,
		DepositWallet:    s.cfg.SolanaDepositWallet,
		PayoutWallet:     firstNonEmpty(s.cfg.SolanaPayoutWallet, s.cfg.SolanaDepositWallet),
		PayoutPrivateKey: s.cfg.SolanaPayoutPrivateKey,
		PayoutMode:       s.cfg.SolanaPayoutMode,
		ScanLimit:        s.cfg.SolanaDepositScanLimit,
		LowBalanceRaw:    s.cfg.SolanaPayoutLowBalanceRaw,
	}
}

func (s *Service) chainRPC() chain.RPC {
	if !s.cfg.SolanaRPCEnabled {
		return nil
	}
	return chain.NewHTTPClient(s.cfg.SolanaRPCURL)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (s *Service) MarketplaceExpandWalletSlots(req store.MarketplaceExpandWalletRequest) (store.MarketplaceExpandWalletResult, error) {
	if s.rulesErr != nil {
		return store.MarketplaceExpandWalletResult{}, s.rulesErr
	}
	req.Rules = s.marketplaceRules()
	return s.store.MarketplaceExpandWalletSlots(req)
}

func (s *Service) MarketplaceSubmitWalletExpandPayment(req store.MarketplaceSubmitPaymentRequest) (store.PaymentOrder, error) {
	if s.cfg.SolanaRPCEnabled {
		rpc := s.chainRPC()
		if rpc == nil {
			return store.PaymentOrder{}, errors.New("solana rpc unavailable for payment verification")
		}
		return s.store.SubmitPaymentOrderVerified(context.Background(), rpc, s.chainConfig(), req)
	}
	return s.store.MarketplaceSubmitWalletExpandPayment(req)
}

func (s *Service) growthPaymentRules() store.GrowthPaymentRules {
	if s.economyRules == nil {
		return store.GrowthPaymentRules{DepositReceiverWallet: strings.TrimSpace(s.cfg.SolanaDepositWallet)}
	}
	return s.economyRules.GrowthPaymentRules(s.cfg.SolanaDepositWallet)
}

func (s *Service) CreateBagExpandPayment(req store.GrowthPaymentRequest) (store.GrowthPaymentResult, error) {
	if s.rulesErr != nil {
		return store.GrowthPaymentResult{}, s.rulesErr
	}
	req.Rules = s.growthPaymentRules()
	return s.store.CreateBagExpandPayment(req)
}

func (s *Service) CreateTradingLicensePayment(req store.GrowthPaymentRequest) (store.GrowthPaymentResult, error) {
	if s.rulesErr != nil {
		return store.GrowthPaymentResult{}, s.rulesErr
	}
	req.Rules = s.growthPaymentRules()
	return s.store.CreateTradingLicensePayment(req)
}

func (s *Service) CreateLotteryPayment(opID string, accountID, characterID int64, count int) (store.LotteryPaymentResult, error) {
	if s.rulesErr != nil {
		return store.LotteryPaymentResult{}, s.rulesErr
	}
	snapshot, err := s.store.EconomySnapshot(accountID, characterID)
	if err != nil {
		return store.LotteryPaymentResult{}, err
	}
	plan, err := s.economyRules.LotteryPlan(opID, characterID, snapshot.Level, count)
	if err != nil {
		return store.LotteryPaymentResult{}, err
	}
	lottery := s.economyRules.Lottery
	result, err := s.store.CreateLotteryPayment(store.LotteryPaymentRequest{
		OpID: opID, AccountID: accountID, CharacterID: characterID, Count: count,
		Amount: lottery.PriceAEB * int64(count), ReceiverWallet: s.cfg.SolanaDepositWallet,
		ConfigSnapshot: map[string]any{"priceAeb": lottery.PriceAEB, "count": count, "categoryWeights": lottery.CategoryWeight, "equipmentRarityWeights": lottery.EquipmentRarityWeight}, RewardPlan: plan,
	})
	if err != nil {
		return store.LotteryPaymentResult{}, err
	}
	return result, nil
}

func (s *Service) BountyBoard(accountID, characterID int64) (store.BountyBoard, error) {
	if s.rulesErr != nil {
		return store.BountyBoard{}, s.rulesErr
	}
	return s.store.BountyBoard(store.BountyBoardRequest{AccountID: accountID, CharacterID: characterID, Plans: s.bountyPlans("bounty-board", characterID, false)})
}

func (s *Service) UnlockBountyGoldSlot(opID string, accountID, characterID int64) (store.BountyBoard, error) {
	if s.rulesErr != nil {
		return store.BountyBoard{}, s.rulesErr
	}
	var cost int64
	for _, slot := range s.economyRules.Bounties.Slots {
		if slot.SlotIndex == 2 {
			cost = slot.GoldCost
		}
	}
	return s.store.UnlockBountyGoldSlot(store.BountyGoldUnlockRequest{OpID: opID, AccountID: accountID, CharacterID: characterID, GoldCost: cost, Plans: s.bountyPlans(opID, characterID, false)})
}

func (s *Service) CreateBountySlotPayment(opID string, accountID, characterID int64, slotIndex int) (store.BountyPaymentResult, error) {
	if s.rulesErr != nil {
		return store.BountyPaymentResult{}, s.rulesErr
	}
	var price int64
	for _, slot := range s.economyRules.Bounties.Slots {
		if slot.SlotIndex == slotIndex {
			price = slot.AEBPrice
		}
	}
	return s.store.CreateBountyPayment(store.BountyPaymentRequest{OpID: opID, AccountID: accountID, CharacterID: characterID, Purpose: store.PaymentPurposeBountySlotUnlock, SlotIndex: slotIndex, Amount: price, ReceiverWallet: s.cfg.SolanaDepositWallet})
}

func (s *Service) RefreshBounty(opID string, accountID, characterID int64, mode string) (store.BountyBoard, *store.PaymentOrder, error) {
	if s.rulesErr != nil {
		return store.BountyBoard{}, nil, s.rulesErr
	}
	board, err := s.BountyBoard(accountID, characterID)
	if err != nil {
		return store.BountyBoard{}, nil, err
	}
	plans := map[int]store.BountyTaskPlan{}
	for _, slot := range board.Slots {
		if slot.Task != nil && slot.Task.Status == "ACTIVE" && slot.Task.Difficulty == "normal" {
			plans[slot.SlotIndex] = s.bountyPlan(opID, characterID, slot.SlotIndex, false)
		}
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "premium" {
		if len(plans) == 0 {
			return store.BountyBoard{}, nil, errors.New("no normal active bounty task can be refreshed")
		}
		first := 6
		for slot := range plans {
			if slot < first {
				first = slot
			}
		}
		plans[first] = s.bountyPlan(opID, characterID, first, true)
		result, err := s.store.CreateBountyPayment(store.BountyPaymentRequest{OpID: opID, AccountID: accountID, CharacterID: characterID, Purpose: store.PaymentPurposeBountyPremiumRefresh, Amount: s.economyRules.Bounties.Refresh.PremiumAEBPrice, ReceiverWallet: s.cfg.SolanaDepositWallet, RefreshPlans: plans})
		return store.BountyBoard{}, &result.Order, err
	}
	if mode != "free" && mode != "gold" {
		return store.BountyBoard{}, nil, errors.New("refresh mode must be free, gold, or premium")
	}
	result, err := s.store.RefreshBounty(store.BountyRefreshRequest{OpID: opID, AccountID: accountID, CharacterID: characterID, Mode: mode, GoldCost: s.economyRules.Bounties.Refresh.GoldCost, CooldownSeconds: int64(s.economyRules.Bounties.Refresh.FreeCooldownSeconds), Plans: plans})
	return result, nil, err
}

func (s *Service) SubmitBountyEquipment(opID string, accountID, characterID int64, slotIndex int, equipmentUID string) (store.BountyTask, error) {
	return s.store.SubmitBountyEquipment(store.BountyEquipmentSubmitRequest{OpID: opID, AccountID: accountID, CharacterID: characterID, SlotIndex: slotIndex, EquipmentUID: equipmentUID})
}
func (s *Service) ClaimBounty(opID string, accountID, characterID int64, slotIndex int) (store.BountyTask, error) {
	return s.store.ClaimBounty(store.BountyClaimRequest{OpID: opID, AccountID: accountID, CharacterID: characterID, SlotIndex: slotIndex})
}
func (s *Service) ProgressBountyCombat(opID string, accountID, characterID int64, dungeonRunID, serverID string) ([]store.BountyTask, error) {
	return s.store.ProgressBountyCombat(store.BountyCombatProgressRequest{OpID: opID, AccountID: accountID, CharacterID: characterID, DungeonRunID: dungeonRunID, ServerID: serverID})
}

func (s *Service) DrawBountyBadge(opID string, accountID, characterID int64, badge string) (store.BountyBadgeDrawResult, error) {
	if s.rulesErr != nil {
		return store.BountyBadgeDrawResult{}, s.rulesErr
	}
	lottery, ok := s.economyRules.Bounties.BadgeLottery[strings.ToLower(strings.TrimSpace(badge))]
	if !ok {
		return store.BountyBadgeDrawResult{}, errors.New("unknown bounty badge")
	}
	if len(lottery.Rewards) == 0 {
		return store.BountyBadgeDrawResult{}, errors.New("empty bounty badge lottery")
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("%s:%d:%d", opID, accountID, characterID)))
	pick := int(h.Sum64() % uint64(bountyWeight(lottery.Rewards)))
	chosen := lottery.Rewards[len(lottery.Rewards)-1]
	for _, reward := range lottery.Rewards {
		if pick < reward.Weight {
			chosen = reward
			break
		}
		pick -= reward.Weight
	}
	span := chosen.Max - chosen.Min + 1
	amount := chosen.Min
	if span > 1 {
		amount += int64((h.Sum64() >> 16) % uint64(span))
	}
	return s.store.DrawBountyBadge(store.BountyBadgeDrawRequest{OpID: opID, AccountID: accountID, CharacterID: characterID, BadgeItemID: lottery.CostItemID, RewardType: chosen.Type, RewardItemID: chosen.ItemID, Amount: amount})
}

func bountyWeight(rows []rules.BountyBadgeReward) int {
	total := 0
	for _, row := range rows {
		total += row.Weight
	}
	return total
}
func (s *Service) bountyPlans(opID string, characterID int64, premium bool) map[int]store.BountyTaskPlan {
	out := map[int]store.BountyTaskPlan{}
	for slot := 1; slot <= 5; slot++ {
		out[slot] = s.bountyPlan(opID, characterID, slot, premium)
	}
	return out
}
func (s *Service) bountyPlan(opID string, characterID int64, slot int, premium bool) store.BountyTaskPlan {
	plan, err := s.economyRules.BountyTaskPlan(opID, characterID, slot, premium)
	if err != nil {
		return store.BountyTaskPlan{}
	}
	return store.BountyTaskPlan{TemplateID: plan.TemplateID, Type: plan.Type, Difficulty: plan.Difficulty, ItemID: plan.ItemID, MinRarity: plan.MinRarity, RequiredQuantity: plan.Required, RewardItemID: plan.RewardItemID, RewardQuantity: plan.RewardQuantity}
}

func (s *Service) ConfirmPaymentOrder(orderID, reason string) (store.PaymentOrder, error) {
	return s.store.ConfirmPaymentOrder(context.Background(), orderID, reason)
}

func (s *Service) ScanDeposits() (store.DepositScanResult, error) {
	if !s.cfg.SolanaRPCEnabled {
		return store.DepositScanResult{Disabled: true, Message: "SOLANA_RPC_ENABLED=false"}, nil
	}
	rpc := s.chainRPC()
	if rpc == nil {
		return store.DepositScanResult{Disabled: true, Message: "rpc unavailable"}, nil
	}
	return s.store.ScanAndCreditDeposits(context.Background(), rpc, s.chainConfig())
}

func (s *Service) ProcessWithdrawals(limit int) []store.Withdrawal {
	return s.store.ProcessAutoWithdrawalsWithChain(
		time.Now().UTC(),
		s.cfg.AutoWithdrawSingleMax,
		s.cfg.UserDailyWithdrawMax,
		s.cfg.GlobalHourlyMax,
		s.cfg.GlobalDailyMax,
		limit,
		s.chainConfig(),
	)
}

func (s *Service) SubmitPayouts(limit int) (store.PayoutJobResult, error) {
	cfg := s.chainConfig()
	if cfg.PayoutMode == "stub" {
		return store.PayoutJobResult{Disabled: true, Message: "stub mode"}, nil
	}
	return s.store.SubmitSolanaPayouts(context.Background(), s.chainRPC(), cfg, limit)
}

func (s *Service) ConfirmPayouts(limit int) (store.PayoutJobResult, error) {
	return s.store.ConfirmSolanaPayouts(context.Background(), s.chainRPC(), s.chainConfig(), limit)
}

func (s *Service) MarketplaceCreateListing(req store.MarketplaceListRequest) (store.MarketplaceListResult, error) {
	if s.rulesErr != nil {
		return store.MarketplaceListResult{}, s.rulesErr
	}
	req.Rules = s.marketplaceRules()
	return s.store.MarketplaceCreateListing(req)
}

func (s *Service) MarketplaceBuy(req store.MarketplaceBuyRequest) (store.MarketplaceBuyResult, error) {
	if s.rulesErr != nil {
		return store.MarketplaceBuyResult{}, s.rulesErr
	}
	req.Rules = s.marketplaceRules()
	return s.store.MarketplaceBuy(req)
}

func (s *Service) MarketplaceCancel(req store.MarketplaceCancelRequest) (store.MarketplaceCancelResult, error) {
	if s.rulesErr != nil {
		return store.MarketplaceCancelResult{}, s.rulesErr
	}
	req.Rules = s.marketplaceRules()
	return s.store.MarketplaceCancel(req)
}

func (s *Service) MarketplaceExpandMaterialSlots(req store.MarketplaceExpandSlotsRequest) (store.MarketplaceExpandResult, error) {
	if s.rulesErr != nil {
		return store.MarketplaceExpandResult{}, s.rulesErr
	}
	req.Rules = s.marketplaceRules()
	return s.store.MarketplaceExpandMaterialSlots(req)
}

func (s *Service) MarketplaceSlots(accountID int64) (store.MarketplaceSlots, error) {
	if s.rulesErr != nil {
		return store.MarketplaceSlots{}, s.rulesErr
	}
	return s.store.MarketplaceSlots(accountID, s.marketplaceRules())
}

func (s *Service) MarketplaceListListings(filter store.MarketplaceListFilter) ([]store.MarketplaceListing, error) {
	return s.store.MarketplaceListListings(filter)
}

func (s *Service) MarketplaceMyListings(accountID int64, status string, limit, offset int) ([]store.MarketplaceListing, error) {
	return s.store.MarketplaceMyListings(accountID, status, limit, offset)
}

func (s *Service) GrantLocked(accountID, amount int64, source, ref string, cooldownHours int) (store.LockedGame, error) {
	if cooldownHours <= 0 {
		cooldownHours = 74
	}
	return s.store.GrantLocked(accountID, amount, defaultSource(source), ref, time.Now().UTC().Add(time.Duration(cooldownHours)*time.Hour))
}

func (s *Service) RequestWithdrawal(accountID, amount int64, wallet string) (store.Withdrawal, error) {
	if amount <= 0 {
		return store.Withdrawal{}, errors.New("amount must be positive")
	}
	wallet = strings.TrimSpace(wallet)
	if wallet == "" {
		account, ok := s.store.Account(accountID)
		if !ok {
			return store.Withdrawal{}, store.ErrNotFound
		}
		wallet = account.WalletAddress
	}
	if wallet == "" {
		return store.Withdrawal{}, errors.New("wallet is required")
	}
	manual := amount > s.cfg.AutoWithdrawSingleMax
	return s.store.CreateWithdrawal(accountID, amount, wallet, manual)
}

func (s *Service) SettleUnlocks(limit int) []store.LockedGame {
	return s.store.SettleUnlocks(time.Now().UTC(), limit)
}

func defaultSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "manual_reward"
	}
	return source
}
