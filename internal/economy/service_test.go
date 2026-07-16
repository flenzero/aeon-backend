package economy

import (
	"fmt"
	"testing"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type withdrawalBoundaryRepository struct {
	store.Repository
	called bool
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
