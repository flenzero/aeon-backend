package economy

import (
	"fmt"
	"testing"

	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type withdrawalBoundaryRepository struct {
	store.Repository
	called bool
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
