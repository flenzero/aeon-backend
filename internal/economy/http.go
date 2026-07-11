package economy

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type Handler struct {
	cfg     config.Config
	service *Service
	store   store.Repository
}

func NewHandler(cfg config.Config, st store.Repository) *Handler {
	return &Handler{cfg: cfg, service: NewService(cfg, st), store: st}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", httpx.Health(h.cfg.ServiceName))
	mux.HandleFunc("GET /api/economy/snapshot", httpx.RequireInternal(h.cfg, h.snapshot))
	mux.HandleFunc("POST /api/economy/warehouse/deposit", httpx.RequireInternal(h.cfg, h.warehouseDeposit))
	mux.HandleFunc("POST /api/economy/warehouse/withdraw", httpx.RequireInternal(h.cfg, h.warehouseWithdraw))
	mux.HandleFunc("POST /api/economy/equipment/equip", httpx.RequireInternal(h.cfg, h.equipItem))
	mux.HandleFunc("POST /api/economy/equipment/unequip", httpx.RequireInternal(h.cfg, h.unequipItem))
	mux.HandleFunc("POST /api/economy/equipment/repair", httpx.RequireInternal(h.cfg, h.equipmentRepair))
	mux.HandleFunc("POST /api/economy/nft/mint/request", httpx.RequireInternal(h.cfg, h.nftMintRequest))
	mux.HandleFunc("POST /api/economy/nft/mint/cancel", httpx.RequireInternal(h.cfg, h.nftMintCancel))
	mux.HandleFunc("POST /api/economy/internal/nft/mint/confirm", httpx.RequireInternal(h.cfg, h.nftMintConfirm))
	mux.HandleFunc("GET /api/economy/nft/assets", httpx.RequireInternal(h.cfg, h.nftListAssets))
	mux.HandleFunc("POST /api/economy/dungeon/enter", httpx.RequireInternal(h.cfg, h.dungeonEnter))
	mux.HandleFunc("POST /api/economy/dungeon/finish", httpx.RequireInternal(h.cfg, h.dungeonFinish))
	mux.HandleFunc("POST /api/economy/loot/claim-player", httpx.RequireInternal(h.cfg, h.lootClaim))
	mux.HandleFunc("POST /api/economy/loot/claim-all", httpx.RequireInternal(h.cfg, h.lootClaimAll))
	mux.HandleFunc("POST /api/economy/loot/discard", httpx.RequireInternal(h.cfg, h.lootDiscard))
	mux.HandleFunc("POST /api/economy/gathering/settle", httpx.RequireInternal(h.cfg, h.gatheringSettle))
	mux.HandleFunc("POST /api/economy/farming/harvest", httpx.RequireInternal(h.cfg, h.farmingHarvest))
	mux.HandleFunc("POST /api/economy/boss/contribute", httpx.RequireInternal(h.cfg, h.bossContribute))
	mux.HandleFunc("POST /api/economy/boss/settle", httpx.RequireInternal(h.cfg, h.bossSettle))
	mux.HandleFunc("POST /api/economy/internal/boss/events/open", httpx.RequireInternal(h.cfg, h.bossOpenEvent))
	mux.HandleFunc("POST /api/economy/internal/boss/events/close", httpx.RequireInternal(h.cfg, h.bossCloseEvent))
	mux.HandleFunc("POST /api/economy/internal/boss/events/settle", httpx.RequireInternal(h.cfg, h.bossMarkSettled))
	mux.HandleFunc("GET /api/economy/internal/boss/events/active", httpx.RequireInternal(h.cfg, h.bossListActiveEvents))
	mux.HandleFunc("POST /api/economy/inventory/organize", httpx.RequireInternal(h.cfg, h.inventoryOrganize))
	mux.HandleFunc("POST /api/economy/warehouse/organize", httpx.RequireInternal(h.cfg, h.warehouseOrganize))
	mux.HandleFunc("POST /api/economy/inventory/discard", httpx.RequireInternal(h.cfg, h.inventoryDiscard))
	mux.HandleFunc("POST /api/economy/inventory/synthesize", httpx.RequireInternal(h.cfg, h.inventorySynthesize))
	mux.HandleFunc("POST /api/economy/inventory/bag/expand", httpx.RequireInternal(h.cfg, h.bagExpand))
	mux.HandleFunc("POST /api/economy/license/purchase", httpx.RequireInternal(h.cfg, h.licensePurchase))
	mux.HandleFunc("GET /api/economy/marketplace/listings", httpx.RequireInternal(h.cfg, h.marketplaceListings))
	mux.HandleFunc("GET /api/economy/marketplace/listings/mine", httpx.RequireInternal(h.cfg, h.marketplaceMyListings))
	mux.HandleFunc("GET /api/economy/marketplace/slots", httpx.RequireInternal(h.cfg, h.marketplaceSlots))
	mux.HandleFunc("POST /api/economy/marketplace/list", httpx.RequireInternal(h.cfg, h.marketplaceList))
	mux.HandleFunc("POST /api/economy/marketplace/listings/{listingId}/buy", httpx.RequireInternal(h.cfg, h.marketplaceBuy))
	mux.HandleFunc("POST /api/economy/marketplace/listings/{listingId}/cancel", httpx.RequireInternal(h.cfg, h.marketplaceCancel))
	mux.HandleFunc("POST /api/economy/marketplace/slots/expand-material", httpx.RequireInternal(h.cfg, h.marketplaceExpandMaterial))
	mux.HandleFunc("POST /api/economy/marketplace/slots/expand-wallet", httpx.RequireInternal(h.cfg, h.marketplaceExpandWallet))
	mux.HandleFunc("POST /api/economy/marketplace/slots/expand-wallet/submit", httpx.RequireInternal(h.cfg, h.marketplaceExpandWalletSubmit))
	mux.HandleFunc("POST /api/economy/internal/payments/submit", httpx.RequireInternal(h.cfg, h.marketplaceExpandWalletSubmit))
	mux.HandleFunc("POST /api/economy/rewards/grant-locked", httpx.RequireInternal(h.cfg, h.grantLocked))
	mux.HandleFunc("POST /api/chain/token/claim", httpx.RequireInternal(h.cfg, h.requestWithdrawal))
	mux.HandleFunc("GET /api/chain/token/ledger", httpx.RequireInternal(h.cfg, h.ledger))
	mux.HandleFunc("POST /api/economy/internal/unlocks/settle", httpx.RequireInternal(h.cfg, h.settleUnlocks))
	mux.HandleFunc("POST /api/economy/internal/withdrawals/process", httpx.RequireInternal(h.cfg, h.processWithdrawals))
	mux.HandleFunc("POST /api/economy/internal/chain/deposits/scan", httpx.RequireInternal(h.cfg, h.scanDeposits))
	mux.HandleFunc("POST /api/economy/internal/chain/payouts/submit", httpx.RequireInternal(h.cfg, h.submitPayouts))
	mux.HandleFunc("POST /api/economy/internal/chain/payouts/confirm", httpx.RequireInternal(h.cfg, h.confirmPayouts))
	mux.HandleFunc("POST /api/economy/internal/payments/confirm", httpx.RequireInternal(h.cfg, h.confirmPayment))
	return httpx.Recover(httpx.WithCORS(mux))
}

func (h *Handler) snapshot(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	characterID, err := httpx.CharacterID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
		return
	}
	snapshot, err := h.service.Snapshot(accountID, characterID)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, 2008, err.Error())
		return
	}
	httpx.OK(w, snapshot)
}

type economyActionBody struct {
	OpID         string `json:"opId"`
	CharacterID  int64  `json:"characterId"`
	SlotIndex    *int   `json:"slotIndex"`
	Quantity     int64  `json:"quantity"`
	EquipmentUID string `json:"equipmentUid"`
	EquipSlot    *int   `json:"equipSlot"`
}

type dungeonEnterBody struct {
	OpID        string `json:"opId"`
	CharacterID int64  `json:"characterId"`
	ChapterID   int    `json:"chapterId"`
	FloorID     int    `json:"floorId"`
}

type dungeonFinishBody struct {
	OpID         string              `json:"opId"`
	CharacterID  int64               `json:"characterId"`
	DungeonRunID string              `json:"dungeonRunId"`
	ChapterID    int                 `json:"chapterId"`
	FloorID      int                 `json:"floorId"`
	Result       string              `json:"result"`
	Exp          int64               `json:"exp"`
	Kills        []store.DungeonKill `json:"kills"`
	Progress     map[string]any      `json:"progress"`
}

type lootActionBody struct {
	OpID        string `json:"opId"`
	CharacterID int64  `json:"characterId"`
	LootID      int64  `json:"lootId"`
	SlotIndex   *int   `json:"slotIndex"`
	Quantity    int64  `json:"quantity"`
}

type gatheringSettleBody struct {
	OpID        string `json:"opId"`
	CharacterID int64  `json:"characterId"`
	NodeID      string `json:"nodeId"`
}

type farmingHarvestBody struct {
	OpID        string `json:"opId"`
	CharacterID int64  `json:"characterId"`
	CropID      string `json:"cropId"`
}

type bossContributeBody struct {
	OpID         string `json:"opId"`
	CharacterID  int64  `json:"characterId"`
	BossEventID  int64  `json:"bossEventId"`
	Contribution int64  `json:"contribution"`
}

type bossSettleBody struct {
	OpID        string `json:"opId"`
	CharacterID int64  `json:"characterId"`
	BossEventID int64  `json:"bossEventId"`
	BossKey     string `json:"bossKey"`
}

type inventoryOrganizeBody struct {
	OpID        string `json:"opId"`
	CharacterID int64  `json:"characterId"`
}

type inventoryDiscardBody struct {
	OpID         string `json:"opId"`
	CharacterID  int64  `json:"characterId"`
	SlotIndex    *int   `json:"slotIndex"`
	Quantity     int64  `json:"quantity"`
	EquipmentUID string `json:"equipmentUid"`
}

type inventorySynthesizeBody struct {
	OpID        string `json:"opId"`
	CharacterID int64  `json:"characterId"`
	RecipeID    string `json:"recipeId"`
	BatchCount  int64  `json:"batchCount"`
}

type bossOpenEventBody struct {
	OpID     string         `json:"opId"`
	BossKey  string         `json:"bossKey"`
	StartsAt string         `json:"startsAt"`
	EndsAt   string         `json:"endsAt"`
	Metadata map[string]any `json:"metadata"`
}

type bossEventIDBody struct {
	OpID        string `json:"opId"`
	BossEventID int64  `json:"bossEventId"`
}

func (h *Handler) warehouseDeposit(w http.ResponseWriter, r *http.Request) {
	h.handleEconomyAction(w, r, h.service.WarehouseDeposit)
}

func (h *Handler) warehouseWithdraw(w http.ResponseWriter, r *http.Request) {
	h.handleEconomyAction(w, r, h.service.WarehouseWithdraw)
}

func (h *Handler) equipItem(w http.ResponseWriter, r *http.Request) {
	h.handleEconomyAction(w, r, h.service.EquipItem)
}

func (h *Handler) unequipItem(w http.ResponseWriter, r *http.Request) {
	h.handleEconomyAction(w, r, h.service.UnequipItem)
}

func (h *Handler) equipmentRepair(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body struct {
		OpID         string `json:"opId"`
		CharacterID  int64  `json:"characterId"`
		EquipmentUID string `json:"equipmentUid"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.EquipmentRepair(store.EquipmentRepairRequest{
		OpID:         body.OpID,
		AccountID:    accountID,
		CharacterID:  characterID,
		EquipmentUID: body.EquipmentUID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3601, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) nftMintRequest(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body struct {
		OpID         string `json:"opId"`
		CharacterID  int64  `json:"characterId"`
		EquipmentUID string `json:"equipmentUid"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.RequestNFTMint(store.NFTMintRequestInput{
		OpID: body.OpID, AccountID: accountID, CharacterID: characterID, EquipmentUID: body.EquipmentUID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3901, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) nftMintCancel(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body struct {
		OpID      string `json:"opId"`
		RequestID int64  `json:"requestId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	result, err := h.service.CancelNFTMint(body.OpID, accountID, body.RequestID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3902, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) nftMintConfirm(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OpID        string `json:"opId"`
		RequestID   int64  `json:"requestId"`
		MintAddress string `json:"mintAddress"`
		TxSignature string `json:"txSignature"`
		MetadataURI string `json:"metadataUri"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	result, err := h.service.ConfirmNFTMint(store.NFTMintConfirmInput{
		OpID: body.OpID, RequestID: body.RequestID, MintAddress: body.MintAddress,
		TxSignature: body.TxSignature, MetadataURI: body.MetadataURI,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3903, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) nftListAssets(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	items, err := h.service.ListNFTAssets(accountID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3904, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"items": items})
}

func (h *Handler) dungeonEnter(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body dungeonEnterBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.DungeonEnter(store.DungeonEnterRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
		ChapterID:   body.ChapterID,
		FloorID:     body.FloorID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3101, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) dungeonFinish(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body dungeonFinishBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.DungeonFinish(store.DungeonFinishRequest{
		OpID:         body.OpID,
		AccountID:    accountID,
		CharacterID:  characterID,
		DungeonRunID: body.DungeonRunID,
		ChapterID:    body.ChapterID,
		FloorID:      body.FloorID,
		Result:       body.Result,
		Exp:          body.Exp,
		Kills:        body.Kills,
		Progress:     body.Progress,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3102, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) lootClaim(w http.ResponseWriter, r *http.Request) {
	h.handleLootAction(w, r, h.service.LootClaim)
}

func (h *Handler) lootClaimAll(w http.ResponseWriter, r *http.Request) {
	h.handleLootAction(w, r, h.service.LootClaimAll)
}

func (h *Handler) lootDiscard(w http.ResponseWriter, r *http.Request) {
	h.handleLootAction(w, r, h.service.LootDiscard)
}

func (h *Handler) handleLootAction(w http.ResponseWriter, r *http.Request, run func(store.LootActionRequest) (store.EconomySnapshot, error)) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body lootActionBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	slotIndex := -1
	if body.SlotIndex != nil {
		slotIndex = *body.SlotIndex
	}
	snapshot, err := run(store.LootActionRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
		LootID:      body.LootID,
		SlotIndex:   slotIndex,
		Quantity:    body.Quantity,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3201, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"snapshot": snapshot})
}

func (h *Handler) gatheringSettle(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body gatheringSettleBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.GatheringSettle(store.ActivitySettlementRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
		ActivityID:  body.NodeID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3301, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) farmingHarvest(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body farmingHarvestBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.FarmingHarvest(store.ActivitySettlementRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
		ActivityID:  body.CropID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3302, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) bossContribute(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body bossContributeBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.BossContribute(store.BossContributeRequest{
		OpID:         body.OpID,
		AccountID:    accountID,
		CharacterID:  characterID,
		BossEventID:  body.BossEventID,
		Contribution: body.Contribution,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3401, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) bossSettle(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body bossSettleBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.BossSettle(store.BossSettleRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
		BossEventID: body.BossEventID,
		BossKey:     body.BossKey,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3402, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) bossOpenEvent(w http.ResponseWriter, r *http.Request) {
	var body bossOpenEventBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	startsAt, err := parseOptionalTime(body.StartsAt)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3403, "startsAt must be RFC3339")
		return
	}
	endsAt, err := parseOptionalTime(body.EndsAt)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3403, "endsAt must be RFC3339")
		return
	}
	event, err := h.service.BossOpenEvent(store.BossOpenEventRequest{
		OpID:     body.OpID,
		BossKey:  body.BossKey,
		StartsAt: startsAt,
		EndsAt:   endsAt,
		Metadata: body.Metadata,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3403, err.Error())
		return
	}
	httpx.Created(w, event)
}

func (h *Handler) bossCloseEvent(w http.ResponseWriter, r *http.Request) {
	var body bossEventIDBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	event, err := h.service.BossCloseEvent(store.BossCloseEventRequest{
		OpID:        body.OpID,
		BossEventID: body.BossEventID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3404, err.Error())
		return
	}
	httpx.OK(w, event)
}

func (h *Handler) bossMarkSettled(w http.ResponseWriter, r *http.Request) {
	var body bossEventIDBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	event, err := h.service.BossMarkSettled(store.BossMarkSettledRequest{
		OpID:        body.OpID,
		BossEventID: body.BossEventID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3405, err.Error())
		return
	}
	httpx.OK(w, event)
}

func (h *Handler) bossListActiveEvents(w http.ResponseWriter, _ *http.Request) {
	events, err := h.service.BossListActiveEvents()
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, 3406, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"events": events})
}

func parseOptionalTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}

func (h *Handler) inventoryOrganize(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body inventoryOrganizeBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	snapshot, err := h.service.InventoryOrganize(store.EconomyActionRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3501, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"snapshot": snapshot})
}

func (h *Handler) warehouseOrganize(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body inventoryOrganizeBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	snapshot, err := h.service.WarehouseOrganize(store.EconomyActionRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3504, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"snapshot": snapshot})
}

func (h *Handler) inventoryDiscard(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body inventoryDiscardBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	slotIndex := -1
	if body.SlotIndex != nil {
		slotIndex = *body.SlotIndex
	}
	snapshot, err := h.service.InventoryDiscard(store.InventoryDiscardRequest{
		OpID:         body.OpID,
		AccountID:    accountID,
		CharacterID:  characterID,
		SlotIndex:    slotIndex,
		Quantity:     body.Quantity,
		EquipmentUID: body.EquipmentUID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3502, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"snapshot": snapshot})
}

func (h *Handler) inventorySynthesize(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body inventorySynthesizeBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	snapshot, err := h.service.Synthesize(store.SynthesizeRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
		RecipeID:    body.RecipeID,
		BatchCount:  body.BatchCount,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3503, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"snapshot": snapshot})
}

type marketplaceListBody struct {
	OpID           string `json:"opId"`
	CharacterID    int64  `json:"characterId"`
	AssetType      string `json:"assetType"`
	EquipmentUID   string `json:"equipmentUid"`
	SourceLocation string `json:"sourceLocation"`
	SlotIndex      *int   `json:"slotIndex"`
	Quantity       int64  `json:"quantity"`
	PriceToken     int64  `json:"priceToken"`
}

type marketplaceBuyBody struct {
	OpID        string `json:"opId"`
	CharacterID int64  `json:"characterId"`
}

type marketplaceCancelBody struct {
	OpID string `json:"opId"`
}

type marketplaceExpandBody struct {
	OpID        string `json:"opId"`
	CharacterID int64  `json:"characterId"`
}

func (h *Handler) bagExpand(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body marketplaceExpandBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.CreateBagExpandPayment(store.GrowthPaymentRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3720, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) licensePurchase(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body marketplaceExpandBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, _ = httpx.CharacterID(r)
	}
	result, err := h.service.CreateTradingLicensePayment(store.GrowthPaymentRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3721, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) marketplaceListings(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 20)
	offset := queryInt(r, "offset", 0)
	items, err := h.service.MarketplaceListListings(store.MarketplaceListFilter{
		Status:    r.URL.Query().Get("status"),
		AssetType: r.URL.Query().Get("assetType"),
		ItemID:    r.URL.Query().Get("itemId"),
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3701, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"items": items, "limit": limit, "offset": offset})
}

func (h *Handler) marketplaceMyListings(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)
	items, err := h.service.MarketplaceMyListings(accountID, r.URL.Query().Get("status"), limit, offset)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3702, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"items": items, "limit": limit, "offset": offset})
}

func (h *Handler) marketplaceSlots(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	slots, err := h.service.MarketplaceSlots(accountID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3703, err.Error())
		return
	}
	httpx.OK(w, slots)
}

func (h *Handler) marketplaceList(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body marketplaceListBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	slotIndex := -1
	if body.SlotIndex != nil {
		slotIndex = *body.SlotIndex
	}
	result, err := h.service.MarketplaceCreateListing(store.MarketplaceListRequest{
		OpID:           body.OpID,
		AccountID:      accountID,
		CharacterID:    characterID,
		AssetType:      body.AssetType,
		EquipmentUID:   body.EquipmentUID,
		SourceLocation: body.SourceLocation,
		SlotIndex:      slotIndex,
		Quantity:       body.Quantity,
		PriceToken:     body.PriceToken,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3704, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) marketplaceBuy(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	listingID, err := pathInt64(r, "listingId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3705, err.Error())
		return
	}
	var body marketplaceBuyBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.MarketplaceBuy(store.MarketplaceBuyRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
		ListingID:   listingID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3706, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) marketplaceCancel(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	listingID, err := pathInt64(r, "listingId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3705, err.Error())
		return
	}
	var body marketplaceCancelBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	result, err := h.service.MarketplaceCancel(store.MarketplaceCancelRequest{
		OpID:      body.OpID,
		AccountID: accountID,
		ListingID: listingID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3707, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) marketplaceExpandMaterial(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body marketplaceExpandBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.MarketplaceExpandMaterialSlots(store.MarketplaceExpandSlotsRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3708, err.Error())
		return
	}
	httpx.OK(w, result)
}

type marketplaceExpandWalletSubmitBody struct {
	OpID        string `json:"opId"`
	OrderID     string `json:"orderId"`
	TxSignature string `json:"txSignature"`
}

type paymentConfirmBody struct {
	OrderID string `json:"orderId"`
	Reason  string `json:"reason"`
}

func (h *Handler) marketplaceExpandWallet(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body marketplaceExpandBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.MarketplaceExpandWalletSlots(store.MarketplaceExpandWalletRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3709, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) marketplaceExpandWalletSubmit(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body marketplaceExpandWalletSubmitBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	order, err := h.service.MarketplaceSubmitWalletExpandPayment(store.MarketplaceSubmitPaymentRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		OrderID:     body.OrderID,
		TxSignature: body.TxSignature,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3710, err.Error())
		return
	}
	httpx.OK(w, order)
}

func (h *Handler) scanDeposits(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ScanDeposits()
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3801, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) submitPayouts(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.SubmitPayouts(50)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3802, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) confirmPayouts(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.ConfirmPayouts(50)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3803, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) confirmPayment(w http.ResponseWriter, r *http.Request) {
	var body paymentConfirmBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	order, err := h.service.ConfirmPaymentOrder(body.OrderID, body.Reason)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3804, err.Error())
		return
	}
	httpx.OK(w, order)
}

func queryInt(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	var value int
	if _, err := fmt.Sscanf(raw, "%d", &value); err != nil {
		return fallback
	}
	return value
}

func pathInt64(r *http.Request, key string) (int64, error) {
	raw := strings.TrimSpace(r.PathValue(key))
	if raw == "" {
		return 0, errors.New(key + " is required")
	}
	var value int64
	if _, err := fmt.Sscanf(raw, "%d", &value); err != nil || value <= 0 {
		return 0, errors.New(key + " is invalid")
	}
	return value, nil
}

func (h *Handler) handleEconomyAction(w http.ResponseWriter, r *http.Request, run func(store.EconomyActionRequest) (store.EconomySnapshot, error)) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body economyActionBody
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	characterID := body.CharacterID
	if characterID == 0 {
		characterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	slotIndex := -1
	if body.SlotIndex != nil {
		slotIndex = *body.SlotIndex
	}
	equipSlot := -1
	if body.EquipSlot != nil {
		equipSlot = *body.EquipSlot
	}
	snapshot, err := run(store.EconomyActionRequest{
		OpID:         body.OpID,
		AccountID:    accountID,
		CharacterID:  characterID,
		SlotIndex:    slotIndex,
		Quantity:     body.Quantity,
		EquipmentUID: body.EquipmentUID,
		EquipSlot:    equipSlot,
	})
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3001, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"snapshot": snapshot})
}

func (h *Handler) grantLocked(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body struct {
		Amount        int64  `json:"amount"`
		Source        string `json:"source"`
		Ref           string `json:"ref"`
		CooldownHours int    `json:"cooldownHours"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	row, err := h.service.GrantLocked(accountID, body.Amount, body.Source, body.Ref, body.CooldownHours)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3000, err.Error())
		return
	}
	httpx.Created(w, row)
}

func (h *Handler) requestWithdrawal(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body struct {
		Amount int64  `json:"amount"`
		Wallet string `json:"wallet"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	row, err := h.service.RequestWithdrawal(accountID, body.Amount, body.Wallet)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3600, err.Error())
		return
	}
	httpx.Created(w, row)
}

func (h *Handler) ledger(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"entries": h.store.Ledger(accountID)})
}

func (h *Handler) settleUnlocks(w http.ResponseWriter, _ *http.Request) {
	httpx.OK(w, map[string]any{"settled": h.service.SettleUnlocks(100)})
}

func (h *Handler) processWithdrawals(w http.ResponseWriter, _ *http.Request) {
	httpx.OK(w, map[string]any{"processed": h.service.ProcessWithdrawals(20)})
}
