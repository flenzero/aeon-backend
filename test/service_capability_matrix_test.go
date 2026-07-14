package test

import (
	"crypto/ed25519"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flenzero/aeon-backend/internal/account"
	"github.com/flenzero/aeon-backend/internal/chain"
	"github.com/flenzero/aeon-backend/internal/economy"
	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/security"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type capabilityRoute struct {
	service    string
	method     string
	target     string
	capability string
}

type testServiceSigner struct {
	id         string
	privateKey ed25519.PrivateKey
}

func TestEveryInternalRouteRejectsWrongServiceCapability(t *testing.T) {
	cfg := testConfig()
	cfg.Profile = config.ProfileProduction
	st := store.New()
	signers := map[string]testServiceSigner{}
	register := func(kind, serviceID, subject string, capabilities ...string) {
		t.Helper()
		publicKey, privateKey, err := ed25519.GenerateKey(nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := st.CreateServiceIdentity(store.CreateServiceIdentityInput{
			ServiceID: serviceID, Name: serviceID, Kind: kind, SubjectID: subject,
			PublicKey: chain.EncodeBase58(publicKey), Capabilities: capabilities,
			CreatedBy: "bootstrap-super-admin", Reason: "capability matrix",
		}); err != nil {
			t.Fatal(err)
		}
		for _, capability := range capabilities {
			signers[capability] = testServiceSigner{id: serviceID, privateKey: privateKey}
		}
	}
	register("GAME_SERVER", "game-server-capability-test", "capability-test", "account.gameplay", "economy.gameplay")
	register("WORKER", "worker-capability-test", "", "economy.worker")
	register("CHAIN_OPERATOR", "chain-capability-test", "", "economy.payments")
	register("MINT_OPERATOR", "mint-capability-test", "", "economy.mint")
	register("OPS", "ops-capability-test", "", "account.ops", "economy.boss_ops", "economy.rewards")

	accountHandler := account.NewHandler(cfg, st).Routes()
	economyHandler := economy.NewHandler(cfg, st).Routes()
	routes := serviceCapabilityRoutes()
	if len(routes) != 61 {
		t.Fatalf("capability route manifest has %d routes, want 61", len(routes))
	}
	for i, route := range routes {
		name := fmt.Sprintf("%s_%s_%s", route.service, route.method, route.target)
		t.Run(name, func(t *testing.T) {
			wrongCapability := "economy.worker"
			if route.capability == wrongCapability {
				wrongCapability = "economy.gameplay"
			}
			signer := signers[wrongCapability]
			var body *strings.Reader
			if route.method == http.MethodPost || route.method == http.MethodDelete {
				body = strings.NewReader(`{}`)
			} else {
				body = strings.NewReader("")
			}
			req := httptest.NewRequest(route.method, route.target, body)
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-Account-Id", "1")
			req.Header.Set("X-Character-Id", "1")
			if err := security.SignServiceRequest(req, signer.id, signer.privateKey, time.Now().UTC(), fmt.Sprintf("nonce-route-%04d-xxxx", i)); err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			if route.service == "account" {
				accountHandler.ServeHTTP(rec, req)
			} else {
				economyHandler.ServeHTTP(rec, req)
			}
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status=%d want=%d capability=%s body=%s", rec.Code, http.StatusForbidden, route.capability, rec.Body.String())
			}
		})
	}
}

func serviceCapabilityRoutes() []capabilityRoute {
	var out []capabilityRoute
	add := func(service, capability string, rows ...[2]string) {
		for _, row := range rows {
			out = append(out, capabilityRoute{service: service, method: row[0], target: row[1], capability: capability})
		}
	}
	add("account", "account.ops",
		[2]string{"GET", "/api/auth/session/redis"},
		[2]string{"POST", "/api/game/online/sweep"},
	)
	add("account", "account.gameplay",
		[2]string{"GET", "/api/character/list"}, [2]string{"POST", "/api/character/create"},
		[2]string{"POST", "/api/game/launch/consume"}, [2]string{"POST", "/api/game/servers/register"},
		[2]string{"POST", "/api/game/servers/heartbeat"}, [2]string{"GET", "/api/game/servers"},
		[2]string{"POST", "/api/game/online/enter"}, [2]string{"POST", "/api/game/online/heartbeat"},
		[2]string{"POST", "/api/game/online/leave"}, [2]string{"GET", "/api/game/online"},
		[2]string{"GET", "/api/game/online/server?serverId=capability-test"},
	)
	add("economy", "economy.gameplay",
		[2]string{"GET", "/api/economy/snapshot"},
		[2]string{"POST", "/api/economy/warehouse/deposit"}, [2]string{"POST", "/api/economy/warehouse/withdraw"},
		[2]string{"POST", "/api/economy/equipment/equip"}, [2]string{"POST", "/api/economy/equipment/unequip"}, [2]string{"POST", "/api/economy/equipment/repair"},
		[2]string{"POST", "/api/economy/nft/mint/request"}, [2]string{"POST", "/api/economy/nft/mint/cancel"}, [2]string{"GET", "/api/economy/nft/assets"},
		[2]string{"POST", "/api/economy/dungeon/enter"}, [2]string{"POST", "/api/economy/dungeon/finish"},
		[2]string{"POST", "/api/economy/loot/claim-player"}, [2]string{"POST", "/api/economy/loot/claim-all"}, [2]string{"POST", "/api/economy/loot/discard"},
		[2]string{"POST", "/api/economy/gathering/settle"}, [2]string{"POST", "/api/economy/farming/harvest"},
		[2]string{"POST", "/api/economy/boss/contribute"}, [2]string{"POST", "/api/economy/boss/settle"},
		[2]string{"POST", "/api/economy/inventory/organize"}, [2]string{"POST", "/api/economy/warehouse/organize"},
		[2]string{"POST", "/api/economy/inventory/discard"}, [2]string{"POST", "/api/economy/inventory/synthesize"},
		[2]string{"POST", "/api/economy/inventory/bag/expand"}, [2]string{"POST", "/api/economy/license/purchase"},
		[2]string{"GET", "/api/economy/marketplace/listings"}, [2]string{"GET", "/api/economy/marketplace/listings/mine"}, [2]string{"GET", "/api/economy/marketplace/slots"},
		[2]string{"POST", "/api/economy/marketplace/list"}, [2]string{"POST", "/api/economy/marketplace/listings/1/buy"}, [2]string{"POST", "/api/economy/marketplace/listings/1/cancel"},
		[2]string{"POST", "/api/economy/marketplace/slots/expand-material"}, [2]string{"POST", "/api/economy/marketplace/slots/expand-wallet"},
		[2]string{"POST", "/api/economy/marketplace/slots/expand-wallet/submit"},
		[2]string{"POST", "/api/chain/token/claim"}, [2]string{"GET", "/api/chain/token/ledger"},
	)
	add("economy", "economy.mint", [2]string{"POST", "/api/economy/internal/nft/mint/confirm"})
	add("economy", "economy.boss_ops",
		[2]string{"POST", "/api/economy/internal/boss/events/open"}, [2]string{"POST", "/api/economy/internal/boss/events/close"},
		[2]string{"POST", "/api/economy/internal/boss/events/settle"}, [2]string{"GET", "/api/economy/internal/boss/events/active"},
	)
	add("economy", "economy.payments",
		[2]string{"POST", "/api/economy/internal/payments/submit"}, [2]string{"POST", "/api/economy/internal/payments/confirm"},
	)
	add("economy", "economy.rewards", [2]string{"POST", "/api/economy/rewards/grant-locked"})
	add("economy", "economy.worker",
		[2]string{"POST", "/api/economy/internal/unlocks/settle"}, [2]string{"POST", "/api/economy/internal/withdrawals/process"},
		[2]string{"POST", "/api/economy/internal/chain/deposits/scan"}, [2]string{"POST", "/api/economy/internal/chain/payouts/submit"},
		[2]string{"POST", "/api/economy/internal/chain/payouts/confirm"},
	)
	return out
}
