package economy

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type withdrawalBoundaryRepository struct {
	store.Repository
	called bool
}

type shopSellEquipmentRepository struct {
	store.Repository
	snapshot store.EconomySnapshot
	sellReq  store.ShopSellRequest
}

func (r *shopSellEquipmentRepository) EconomySnapshot(accountID, characterID int64) (store.EconomySnapshot, error) {
	return r.snapshot, nil
}

func (r *shopSellEquipmentRepository) ShopSell(req store.ShopSellRequest) (store.ShopSellResult, error) {
	r.sellReq = req
	return store.ShopSellResult{Snapshot: r.snapshot}, nil
}

func TestShopCatalogReturnsDailyLimitedNormalMerchantStock(t *testing.T) {
	repo := store.New()
	account := repo.UpsertAccountByWallet("shop-catalog-wallet")
	character, err := repo.CreateCharacter(account.ID, "Catalog")
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	service := NewService(config.Config{EconomyConfigDir: "../../configs/economy"}, repo)
	catalog, err := service.ShopCatalog(account.ID, character.ID, "general_merchant", time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("shop catalog: %v", err)
	}
	if catalog.ShopID != "general_merchant" || catalog.BusinessDate != "2026-07-16" {
		t.Fatalf("catalog header=%+v", catalog)
	}
	if len(catalog.Items) == 0 {
		t.Fatal("general merchant catalog is empty")
	}
	first := catalog.Items[0]
	if first.SlotIndex != 1 || first.DailyLimit <= 0 || first.RemainingToday != first.DailyLimit || !first.Available {
		t.Fatalf("first catalog item=%+v", first)
	}
}

func TestEquipmentMerchantCatalogFiltersByCharacterLevel(t *testing.T) {
	repo := store.New()
	account := repo.UpsertAccountByWallet("equipment-catalog-wallet")
	character, err := repo.CreateCharacter(account.ID, "Equipment")
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	service := NewService(config.Config{EconomyConfigDir: "../../configs/economy"}, repo)
	catalog, err := service.ShopCatalog(account.ID, character.ID, "equipment_merchant", time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("shop catalog: %v", err)
	}
	if len(catalog.Items) != 6 {
		t.Fatalf("level 1 equipment rows=%d, want 6", len(catalog.Items))
	}
	for _, item := range catalog.Items {
		if item.Rarity != 3 || item.MinLevel != 0 || item.MaxLevel != 4 || item.DailyLimit != 1 {
			t.Fatalf("unexpected equipment catalog item=%+v", item)
		}
	}
}

func TestShopSellEquipmentUsesTemplateStageRarityPrice(t *testing.T) {
	repo := &shopSellEquipmentRepository{
		Repository: store.New(),
		snapshot: store.EconomySnapshot{
			Equipment: []store.EquipmentItem{{
				EquipmentUID: "eq-template-sell",
				ItemID:       "shadowiron_sword_t10",
				Rarity:       2,
				Status:       "IN_BAG",
			}},
		},
	}
	service := NewService(config.Config{EconomyConfigDir: "../../configs/economy"}, repo)
	if _, err := service.ShopSell("sell-template-equipment", 1, 2, "general_merchant", -1, 0, "eq-template-sell"); err != nil {
		t.Fatalf("shop sell equipment: %v", err)
	}
	if repo.sellReq.GoldCredit != 310 {
		t.Fatalf("equipment sell gold = %d, want 310", repo.sellReq.GoldCredit)
	}
}

func TestShopSellRejectsLegacyEquipmentRows(t *testing.T) {
	repo := &shopSellEquipmentRepository{
		Repository: store.New(),
		snapshot: store.EconomySnapshot{
			Equipment: []store.EquipmentItem{{
				EquipmentUID: "eq-legacy-sell",
				ItemID:       "rusted_saber",
				Rarity:       1,
				Status:       "IN_BAG",
			}},
		},
	}
	service := NewService(config.Config{EconomyConfigDir: "../../configs/economy"}, repo)
	_, err := service.ShopSell("sell-legacy-equipment", 1, 2, "general_merchant", -1, 0, "eq-legacy-sell")
	if err == nil || !strings.Contains(err.Error(), "current equipment templates") {
		t.Fatalf("legacy equipment sell err=%v", err)
	}
	if repo.sellReq.GoldCredit != 0 {
		t.Fatalf("legacy equipment reached sell persistence: %+v", repo.sellReq)
	}
}

func TestDungeonEnterRejectsUnconfiguredFloor(t *testing.T) {
	repo := store.New()
	account := repo.UpsertAccountByWallet("dungeon-enter-wallet")
	character, err := repo.CreateCharacter(account.ID, "Dungeon")
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	service := NewService(config.Config{EconomyConfigDir: "../../configs/economy"}, repo)
	_, err = service.DungeonEnter(store.DungeonEnterRequest{
		OpID:        "dungeon-enter-unconfigured",
		AccountID:   account.ID,
		CharacterID: character.ID,
		ChapterID:   9,
		FloorID:     99,
	})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("unconfigured dungeon enter err=%v", err)
	}
}

func TestDungeonFinishReturnsExpToNextLevel(t *testing.T) {
	repo := store.New()
	account := repo.UpsertAccountByWallet("dungeon-exp-wallet")
	character, err := repo.CreateCharacter(account.ID, "ExpHero")
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	service := NewService(config.Config{EconomyConfigDir: "../../configs/economy"}, repo)
	enter, err := service.DungeonEnter(store.DungeonEnterRequest{
		OpID:        "dungeon-exp-enter",
		AccountID:   account.ID,
		CharacterID: character.ID,
		ChapterID:   0,
		FloorID:     7,
	})
	if err != nil {
		t.Fatalf("dungeon enter: %v", err)
	}
	if enter.Snapshot.ExpToNextLevel != 83 || enter.Rewards.ExpToNextLevel != 83 {
		t.Fatalf("enter expToNextLevel snapshot=%d rewards=%d, want 83", enter.Snapshot.ExpToNextLevel, enter.Rewards.ExpToNextLevel)
	}
	finish, err := service.DungeonFinish(store.DungeonFinishRequest{
		OpID:         "dungeon-exp-finish",
		AccountID:    account.ID,
		CharacterID:  character.ID,
		DungeonRunID: enter.DungeonRunID,
		ChapterID:    0,
		FloorID:      7,
		Result:       "victory",
		Exp:          90,
	})
	if err != nil {
		t.Fatalf("dungeon finish: %v", err)
	}
	if finish.Snapshot.Exp != 90 || finish.Snapshot.Level != 2 {
		t.Fatalf("finish snapshot progress exp=%d level=%d, want exp=90 level=2", finish.Snapshot.Exp, finish.Snapshot.Level)
	}
	if finish.Snapshot.ExpToNextLevel != 125 || finish.Rewards.ExpToNextLevel != 125 {
		t.Fatalf("finish expToNextLevel snapshot=%d rewards=%d, want 125", finish.Snapshot.ExpToNextLevel, finish.Rewards.ExpToNextLevel)
	}
	if finish.Snapshot.HighestClearedChapterID != 0 || finish.Snapshot.HighestClearedFloorID != 7 {
		t.Fatalf("finish highest cleared chapter=%d floor=%d, want chapter=0 floor=7", finish.Snapshot.HighestClearedChapterID, finish.Snapshot.HighestClearedFloorID)
	}
	if finish.Rewards.Level != finish.Snapshot.Level || finish.Rewards.LevelsGained != 1 {
		t.Fatalf("rewards progress level=%d levelsGained=%d, want level=%d levelsGained=1", finish.Rewards.Level, finish.Rewards.LevelsGained, finish.Snapshot.Level)
	}
}

func (r *withdrawalBoundaryRepository) CreateWithdrawal(accountID, amount int64, wallet string, manual bool) (store.Withdrawal, error) {
	r.called = true
	return store.Withdrawal{AccountID: accountID, Amount: amount, Wallet: wallet}, nil
}

func TestRequestWithdrawalRejectsNonPositiveAmountBeforePersistence(t *testing.T) {
	for _, amount := range []int64{-1, 0} {
		t.Run(fmt.Sprint(amount), func(t *testing.T) {
			repo := &withdrawalBoundaryRepository{Repository: store.New()}
			service := NewService(config.Config{EconomyConfigDir: "../../configs/economy", AutoWithdrawSingleMax: 5000}, repo)
			if _, err := service.RequestWithdrawal(1, amount, "11111111111111111111111111111111"); err == nil {
				t.Fatalf("amount %d accepted", amount)
			}
			if repo.called {
				t.Fatalf("amount %d reached persistence Adapter", amount)
			}
		})
	}
}
