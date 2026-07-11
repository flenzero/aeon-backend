package economy

import (
	"context"
	"errors"
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

func (s *Service) Snapshot(accountID, characterID int64) (store.EconomySnapshot, error) {
	snapshot, err := s.store.EconomySnapshot(accountID, characterID)
	if err != nil {
		return store.EconomySnapshot{}, err
	}
	if s.economyRules != nil {
		snapshot.BagSlots = s.economyRules.EffectiveBagSlots(snapshot.BagExpandCount)
	}
	return snapshot, nil
}

func (s *Service) WarehouseDeposit(req store.EconomyActionRequest) (store.EconomySnapshot, error) {
	return s.store.WarehouseDeposit(req)
}

func (s *Service) WarehouseWithdraw(req store.EconomyActionRequest) (store.EconomySnapshot, error) {
	return s.store.WarehouseWithdraw(req)
}

func (s *Service) EquipItem(req store.EconomyActionRequest) (store.EconomySnapshot, error) {
	return s.store.EquipItem(req)
}

func (s *Service) UnequipItem(req store.EconomyActionRequest) (store.EconomySnapshot, error) {
	return s.store.UnequipItem(req)
}

func (s *Service) EquipmentRepair(req store.EquipmentRepairRequest) (store.EquipmentRepairResult, error) {
	if s.rulesErr != nil {
		return store.EquipmentRepairResult{}, s.rulesErr
	}
	req.Rules = s.economyRules.EquipmentRules()
	return s.store.EquipmentRepair(req)
}

func (s *Service) RequestNFTMint(req store.NFTMintRequestInput) (store.NFTMintRequestResult, error) {
	if s.rulesErr != nil {
		return store.NFTMintRequestResult{}, s.rulesErr
	}
	req.Rules = s.economyRules.NFTRules()
	return s.store.RequestNFTMint(req)
}

func (s *Service) ConfirmNFTMint(req store.NFTMintConfirmInput) (store.NFTMintRequestResult, error) {
	return s.store.ConfirmNFTMint(req)
}

func (s *Service) CancelNFTMint(opID string, accountID, requestID int64) (store.NFTMintRequestResult, error) {
	return s.store.CancelNFTMint(opID, accountID, requestID)
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
	return s.store.DungeonFinish(req)
}

func (s *Service) LootClaim(req store.LootActionRequest) (store.EconomySnapshot, error) {
	return s.store.LootClaim(req)
}

func (s *Service) LootClaimAll(req store.LootActionRequest) (store.EconomySnapshot, error) {
	return s.store.LootClaimAll(req)
}

func (s *Service) LootDiscard(req store.LootActionRequest) (store.EconomySnapshot, error) {
	return s.store.LootDiscard(req)
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
	return s.store.GatheringSettle(req)
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
	return s.store.FarmingHarvest(req)
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
	return s.store.BossSettle(req)
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
	return s.store.InventoryOrganize(req, bagSlots)
}

func (s *Service) WarehouseOrganize(req store.EconomyActionRequest) (store.EconomySnapshot, error) {
	warehouseSlots := 50
	if s.economyRules != nil {
		warehouseSlots = s.economyRules.WarehouseSlots()
	}
	return s.store.WarehouseOrganize(req, warehouseSlots)
}

func (s *Service) InventoryDiscard(req store.InventoryDiscardRequest) (store.EconomySnapshot, error) {
	return s.store.InventoryDiscard(req)
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
	return s.store.Synthesize(req)
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
