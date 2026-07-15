package economy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/chain"
	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/readiness"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type Handler struct {
	cfg     config.Config
	service *Service
	store   store.Repository
	ready   readiness.Probe
}

func NewHandler(cfg config.Config, st store.Repository) *Handler {
	service := NewService(cfg, st)
	checks := readiness.PersistenceChecks(cfg, st)
	checks = append(checks, readiness.Required("economy-rules", service.Ready))
	if cfg.StubMode == config.StubDisabled {
		rpc := chain.NewHTTPClient(cfg.SolanaRPCURL)
		checks = append(checks, readiness.Required("solana-rpc", func(ctx context.Context) error {
			_, err := rpc.GetSlot(ctx)
			return err
		}))
	}
	return &Handler{cfg: cfg, service: service, store: st, ready: readiness.New(cfg.ServiceName, checks...)}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", httpx.Health(h.cfg.ServiceName))
	mux.Handle("GET /ready", h.ready.Handler())
	mux.HandleFunc("GET /api/announcements/active", h.publicActiveAnnouncements)
	gameplay := func(next http.HandlerFunc) http.HandlerFunc {
		return httpx.RequireService(h.cfg, h.store, "economy.gameplay", next)
	}
	worker := func(next http.HandlerFunc) http.HandlerFunc {
		return httpx.RequireService(h.cfg, h.store, "economy.worker", next)
	}
	mint := func(next http.HandlerFunc) http.HandlerFunc {
		return httpx.RequireService(h.cfg, h.store, "economy.mint", next)
	}
	bossOps := func(next http.HandlerFunc) http.HandlerFunc {
		return httpx.RequireService(h.cfg, h.store, "economy.boss_ops", next)
	}
	rewards := func(next http.HandlerFunc) http.HandlerFunc {
		return httpx.RequireService(h.cfg, h.store, "economy.rewards", next)
	}
	payments := func(next http.HandlerFunc) http.HandlerFunc {
		return httpx.RequireService(h.cfg, h.store, "economy.payments", next)
	}
	mux.HandleFunc("GET /api/economy/snapshot", gameplay(h.snapshot))
	mux.HandleFunc("GET /api/economy/announcements/active", gameplay(h.activeAnnouncements))
	mux.HandleFunc("POST /api/economy/warehouse/deposit", gameplay(h.warehouseDeposit))
	mux.HandleFunc("POST /api/economy/warehouse/withdraw", gameplay(h.warehouseWithdraw))
	mux.HandleFunc("POST /api/economy/equipment/equip", gameplay(h.equipItem))
	mux.HandleFunc("POST /api/economy/equipment/unequip", gameplay(h.unequipItem))
	mux.HandleFunc("POST /api/economy/equipment/repair", gameplay(h.equipmentRepair))
	mux.HandleFunc("POST /api/economy/equipment/enhance", gameplay(h.equipmentEnhance))
	mux.HandleFunc("POST /api/economy/equipment/npc-recycle", gameplay(h.equipmentNPCRecycle))
	mux.HandleFunc("POST /api/economy/shop/buy", gameplay(h.shopBuy))
	mux.HandleFunc("POST /api/economy/shop/sell", gameplay(h.shopSell))
	mux.HandleFunc("POST /api/economy/lottery/draw", gameplay(h.lotteryDraw))
	mux.HandleFunc("GET /api/economy/bounty/board", gameplay(h.bountyBoard))
	mux.HandleFunc("POST /api/economy/bounty/slots/unlock-gold", gameplay(h.bountyUnlockGold))
	mux.HandleFunc("POST /api/economy/bounty/slots/unlock-aeb", gameplay(h.bountyUnlockAEB))
	mux.HandleFunc("POST /api/economy/bounty/refresh", gameplay(h.bountyRefresh))
	mux.HandleFunc("POST /api/economy/bounty/progress/combat", gameplay(h.bountyCombatProgress))
	mux.HandleFunc("POST /api/economy/bounty/submit-equipment", gameplay(h.bountySubmitEquipment))
	mux.HandleFunc("POST /api/economy/bounty/claim", gameplay(h.bountyClaim))
	mux.HandleFunc("POST /api/economy/bounty/badges/draw", gameplay(h.bountyBadgeDraw))
	mux.HandleFunc("POST /api/economy/nft/mint/request", gameplay(h.nftMintRequest))
	mux.HandleFunc("POST /api/economy/nft/mint/cancel", gameplay(h.nftMintCancel))
	mux.HandleFunc("POST /api/economy/internal/nft/mint/confirm", mint(h.nftMintConfirm))
	mux.HandleFunc("GET /api/economy/nft/assets", gameplay(h.nftListAssets))
	mux.HandleFunc("POST /api/economy/dungeon/enter", gameplay(h.dungeonEnter))
	mux.HandleFunc("POST /api/economy/dungeon/finish", gameplay(h.dungeonFinish))
	mux.HandleFunc("POST /api/economy/loot/claim-player", gameplay(h.lootClaim))
	mux.HandleFunc("POST /api/economy/loot/claim-all", gameplay(h.lootClaimAll))
	mux.HandleFunc("POST /api/economy/loot/discard", gameplay(h.lootDiscard))
	mux.HandleFunc("POST /api/economy/gathering/settle", gameplay(h.gatheringSettle))
	mux.HandleFunc("POST /api/economy/farming/harvest", gameplay(h.farmingHarvest))
	mux.HandleFunc("POST /api/economy/boss/contribute", gameplay(h.bossContribute))
	mux.HandleFunc("POST /api/economy/boss/settle", gameplay(h.bossSettle))
	mux.HandleFunc("POST /api/economy/internal/boss/events/open", bossOps(h.bossOpenEvent))
	mux.HandleFunc("POST /api/economy/internal/boss/events/close", bossOps(h.bossCloseEvent))
	mux.HandleFunc("POST /api/economy/internal/boss/events/settle", bossOps(h.bossMarkSettled))
	mux.HandleFunc("GET /api/economy/internal/boss/events/active", bossOps(h.bossListActiveEvents))
	mux.HandleFunc("POST /api/economy/inventory/organize", gameplay(h.inventoryOrganize))
	mux.HandleFunc("POST /api/economy/warehouse/organize", gameplay(h.warehouseOrganize))
	mux.HandleFunc("POST /api/economy/inventory/discard", gameplay(h.inventoryDiscard))
	mux.HandleFunc("POST /api/economy/inventory/synthesize", gameplay(h.inventorySynthesize))
	mux.HandleFunc("POST /api/economy/inventory/bag/expand", gameplay(h.bagExpand))
	mux.HandleFunc("POST /api/economy/license/purchase", gameplay(h.licensePurchase))
	mux.HandleFunc("POST /api/economy/license/buy", gameplay(h.licensePurchase))
	mux.HandleFunc("GET /api/economy/marketplace/listings", gameplay(h.marketplaceListings))
	mux.HandleFunc("GET /api/economy/marketplace/listings/mine", gameplay(h.marketplaceMyListings))
	mux.HandleFunc("GET /api/economy/marketplace/slots", gameplay(h.marketplaceSlots))
	mux.HandleFunc("POST /api/economy/marketplace/list", gameplay(h.marketplaceList))
	mux.HandleFunc("POST /api/economy/marketplace/listings/{listingId}/buy", gameplay(h.marketplaceBuy))
	mux.HandleFunc("POST /api/economy/marketplace/listings/{listingId}/cancel", gameplay(h.marketplaceCancel))
	mux.HandleFunc("POST /api/economy/marketplace/slots/expand-material", gameplay(h.marketplaceExpandMaterial))
	mux.HandleFunc("POST /api/economy/marketplace/slots/expand-wallet", gameplay(h.marketplaceExpandWallet))
	mux.HandleFunc("POST /api/economy/marketplace/slots/expand-wallet/submit", gameplay(h.marketplaceExpandWalletSubmit))
	mux.HandleFunc("POST /api/economy/internal/payments/submit", payments(h.marketplaceExpandWalletSubmit))
	mux.HandleFunc("POST /api/economy/rewards/grant-locked", rewards(h.grantLocked))
	mux.HandleFunc("POST /api/chain/token/claim", gameplay(h.requestWithdrawal))
	mux.HandleFunc("GET /api/chain/token/ledger", gameplay(h.ledger))
	mux.HandleFunc("POST /api/economy/internal/unlocks/settle", worker(h.settleUnlocks))
	mux.HandleFunc("POST /api/economy/internal/withdrawals/process", worker(h.processWithdrawals))
	mux.HandleFunc("POST /api/economy/internal/chain/deposits/scan", worker(h.scanDeposits))
	mux.HandleFunc("POST /api/economy/internal/chain/payouts/submit", worker(h.submitPayouts))
	mux.HandleFunc("POST /api/economy/internal/chain/payouts/confirm", worker(h.confirmPayouts))
	mux.HandleFunc("POST /api/economy/internal/equipment/npc-recycle/purge", worker(h.purgeNPCRecycledEquipment))
	mux.HandleFunc("POST /api/economy/internal/payments/confirm", payments(h.confirmPayment))
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

func (h *Handler) activeAnnouncements(w http.ResponseWriter, r *http.Request) {
	h.writeActiveAnnouncements(w, r, "")
}

func (h *Handler) publicActiveAnnouncements(w http.ResponseWriter, r *http.Request) {
	h.writeActiveAnnouncements(w, r, store.AnnouncementKindOpsNotice)
}

func (h *Handler) writeActiveAnnouncements(w http.ResponseWriter, r *http.Request, kind string) {
	limit, _, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	afterID := int64(0)
	if raw := strings.TrimSpace(r.URL.Query().Get("afterId")); raw != "" {
		afterID, err = strconv.ParseInt(raw, 10, 64)
		if err != nil || afterID < 0 {
			httpx.Error(w, http.StatusBadRequest, 400, "afterId must be non-negative")
			return
		}
	}
	rows, err := h.store.ListActiveAnnouncements(kind, time.Now().UTC(), afterID, limit)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"items": rows, "count": len(rows), "afterId": afterID})
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

func (h *Handler) equipmentEnhance(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.service.EquipmentEnhance(body.OpID, accountID, characterID, body.EquipmentUID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3602, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) equipmentNPCRecycle(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.service.EquipmentNPCRecycle(body.OpID, accountID, characterID, body.EquipmentUID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3603, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) shopBuy(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body struct {
		OpID        string `json:"opId"`
		CharacterID int64  `json:"characterId"`
		ShopID      string `json:"shopId"`
		ItemID      string `json:"itemId"`
		Quantity    int64  `json:"quantity"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if body.CharacterID == 0 {
		body.CharacterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.ShopBuy(body.OpID, accountID, body.CharacterID, body.ShopID, body.ItemID, body.Quantity)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3000, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) shopSell(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body struct {
		OpID         string `json:"opId"`
		CharacterID  int64  `json:"characterId"`
		ShopID       string `json:"shopId"`
		SlotIndex    *int   `json:"slotIndex"`
		Quantity     int64  `json:"quantity"`
		EquipmentUID string `json:"equipmentUid"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if body.CharacterID == 0 {
		body.CharacterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	slotIndex := -1
	if body.SlotIndex != nil {
		slotIndex = *body.SlotIndex
	}
	result, err := h.service.ShopSell(body.OpID, accountID, body.CharacterID, body.ShopID, slotIndex, body.Quantity, body.EquipmentUID)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3000, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) lotteryDraw(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 2006, err.Error())
		return
	}
	var body struct {
		OpID        string `json:"opId"`
		CharacterID int64  `json:"characterId"`
		Count       int    `json:"count"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if body.CharacterID == 0 {
		body.CharacterID, err = httpx.CharacterID(r)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 2007, err.Error())
			return
		}
	}
	result, err := h.service.CreateLotteryPayment(body.OpID, accountID, body.CharacterID, body.Count)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3610, err.Error())
		return
	}
	httpx.Created(w, result)
}

func bountyCharacterID(r *http.Request, requested int64) (int64, error) {
	if requested != 0 {
		return requested, nil
	}
	return httpx.CharacterID(r)
}
func (h *Handler) bountyBoard(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, 400, 2006, err.Error())
		return
	}
	characterID, err := httpx.CharacterID(r)
	if err != nil {
		httpx.Error(w, 400, 2007, err.Error())
		return
	}
	result, err := h.service.BountyBoard(accountID, characterID)
	if err != nil {
		httpx.Error(w, 400, 3620, err.Error())
		return
	}
	httpx.OK(w, result)
}
func (h *Handler) bountyUnlockGold(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, 400, 2006, err.Error())
		return
	}
	var b struct {
		OpID        string `json:"opId"`
		CharacterID int64  `json:"characterId"`
	}
	if !httpx.Decode(r, &b) {
		httpx.Error(w, 400, 400, "invalid JSON body")
		return
	}
	characterID, err := bountyCharacterID(r, b.CharacterID)
	if err != nil {
		httpx.Error(w, 400, 2007, err.Error())
		return
	}
	result, err := h.service.UnlockBountyGoldSlot(b.OpID, accountID, characterID)
	if err != nil {
		httpx.Error(w, 400, 3621, err.Error())
		return
	}
	httpx.OK(w, result)
}
func (h *Handler) bountyUnlockAEB(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, 400, 2006, err.Error())
		return
	}
	var b struct {
		OpID        string `json:"opId"`
		CharacterID int64  `json:"characterId"`
		SlotIndex   int    `json:"slotIndex"`
	}
	if !httpx.Decode(r, &b) {
		httpx.Error(w, 400, 400, "invalid JSON body")
		return
	}
	characterID, err := bountyCharacterID(r, b.CharacterID)
	if err != nil {
		httpx.Error(w, 400, 2007, err.Error())
		return
	}
	result, err := h.service.CreateBountySlotPayment(b.OpID, accountID, characterID, b.SlotIndex)
	if err != nil {
		httpx.Error(w, 400, 3622, err.Error())
		return
	}
	httpx.Created(w, result)
}
func (h *Handler) bountyRefresh(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, 400, 2006, err.Error())
		return
	}
	var b struct {
		OpID        string `json:"opId"`
		CharacterID int64  `json:"characterId"`
		Mode        string `json:"mode"`
	}
	if !httpx.Decode(r, &b) {
		httpx.Error(w, 400, 400, "invalid JSON body")
		return
	}
	characterID, err := bountyCharacterID(r, b.CharacterID)
	if err != nil {
		httpx.Error(w, 400, 2007, err.Error())
		return
	}
	board, order, err := h.service.RefreshBounty(b.OpID, accountID, characterID, b.Mode)
	if err != nil {
		httpx.Error(w, 400, 3623, err.Error())
		return
	}
	if order != nil {
		httpx.Created(w, map[string]any{"order": order})
		return
	}
	httpx.OK(w, board)
}
func (h *Handler) bountyCombatProgress(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, 400, 2006, err.Error())
		return
	}
	var b struct {
		OpID         string `json:"opId"`
		CharacterID  int64  `json:"characterId"`
		DungeonRunID string `json:"dungeonRunId"`
	}
	if !httpx.Decode(r, &b) {
		httpx.Error(w, 400, 400, "invalid JSON body")
		return
	}
	serverID := gameplayServerID(r)
	if serverID == "" {
		httpx.Error(w, 403, 4030, "combat progress requires a game server identity")
		return
	}
	characterID, err := bountyCharacterID(r, b.CharacterID)
	if err != nil {
		httpx.Error(w, 400, 2007, err.Error())
		return
	}
	tasks, err := h.service.ProgressBountyCombat(b.OpID, accountID, characterID, b.DungeonRunID, serverID)
	if err != nil {
		httpx.Error(w, 400, 3624, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"tasks": tasks})
}
func (h *Handler) bountySubmitEquipment(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, 400, 2006, err.Error())
		return
	}
	var b struct {
		OpID         string `json:"opId"`
		CharacterID  int64  `json:"characterId"`
		SlotIndex    int    `json:"slotIndex"`
		EquipmentUID string `json:"equipmentUid"`
	}
	if !httpx.Decode(r, &b) {
		httpx.Error(w, 400, 400, "invalid JSON body")
		return
	}
	characterID, err := bountyCharacterID(r, b.CharacterID)
	if err != nil {
		httpx.Error(w, 400, 2007, err.Error())
		return
	}
	result, err := h.service.SubmitBountyEquipment(b.OpID, accountID, characterID, b.SlotIndex, b.EquipmentUID)
	if err != nil {
		httpx.Error(w, 400, 3625, err.Error())
		return
	}
	httpx.OK(w, result)
}
func (h *Handler) bountyClaim(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, 400, 2006, err.Error())
		return
	}
	var b struct {
		OpID        string `json:"opId"`
		CharacterID int64  `json:"characterId"`
		SlotIndex   int    `json:"slotIndex"`
	}
	if !httpx.Decode(r, &b) {
		httpx.Error(w, 400, 400, "invalid JSON body")
		return
	}
	characterID, err := bountyCharacterID(r, b.CharacterID)
	if err != nil {
		httpx.Error(w, 400, 2007, err.Error())
		return
	}
	result, err := h.service.ClaimBounty(b.OpID, accountID, characterID, b.SlotIndex)
	if err != nil {
		httpx.Error(w, 400, 3626, err.Error())
		return
	}
	httpx.OK(w, result)
}
func (h *Handler) bountyBadgeDraw(w http.ResponseWriter, r *http.Request) {
	accountID, err := httpx.AccountID(r)
	if err != nil {
		httpx.Error(w, 400, 2006, err.Error())
		return
	}
	var b struct {
		OpID        string `json:"opId"`
		CharacterID int64  `json:"characterId"`
		Badge       string `json:"badge"`
	}
	if !httpx.Decode(r, &b) {
		httpx.Error(w, 400, 400, "invalid JSON body")
		return
	}
	characterID, err := bountyCharacterID(r, b.CharacterID)
	if err != nil {
		httpx.Error(w, 400, 2007, err.Error())
		return
	}
	result, err := h.service.DrawBountyBadge(b.OpID, accountID, characterID, b.Badge)
	if err != nil {
		httpx.Error(w, 400, 3627, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) purgeNPCRecycledEquipment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Limit int `json:"limit"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	count, err := h.service.PurgeExpiredNPCRecycledEquipment(time.Now().UTC(), body.Limit)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, 5601, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"purged": count})
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
	if h.cfg.StubMode == config.StubDisabled {
		httpx.Error(w, http.StatusServiceUnavailable, 3903, "Metaplex Core mint verification adapter is not configured")
		return
	}
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
	serverID := gameplayServerID(r)
	if serverID != "" {
		online, err := h.store.GetOnlineSession(accountID)
		if err != nil || online.ServerID != serverID || online.CharacterID != characterID {
			httpx.Error(w, http.StatusForbidden, 3100, "player is not online on the calling game server")
			return
		}
	}
	result, err := h.service.DungeonEnter(store.DungeonEnterRequest{
		OpID:        body.OpID,
		AccountID:   accountID,
		CharacterID: characterID,
		ServerID:    serverID,
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
		ServerID:     gameplayServerID(r),
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

func gameplayServerID(r *http.Request) string {
	identity, ok := httpx.ContextServiceIdentity(r)
	if !ok {
		return ""
	}
	if identity.Kind == "GAME_SERVER" {
		return strings.TrimSpace(identity.SubjectID)
	}
	if identity.Kind == "LEGACY" {
		return strings.TrimSpace(r.Header.Get("X-Game-Server-Id"))
	}
	return ""
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
	limit, offset, err := httpx.Pagination(r, 20, 100)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
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
	limit, offset, err := httpx.Pagination(r, 50, 100)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
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
	if !decodeEmptyCommand(w, r) {
		return
	}
	result, err := h.service.ScanDeposits()
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3801, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) submitPayouts(w http.ResponseWriter, r *http.Request) {
	if !decodeEmptyCommand(w, r) {
		return
	}
	result, err := h.service.SubmitPayouts(50)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 3802, err.Error())
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) confirmPayouts(w http.ResponseWriter, r *http.Request) {
	if !decodeEmptyCommand(w, r) {
		return
	}
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

func (h *Handler) settleUnlocks(w http.ResponseWriter, r *http.Request) {
	if !decodeEmptyCommand(w, r) {
		return
	}
	httpx.OK(w, map[string]any{"settled": h.service.SettleUnlocks(100)})
}

func (h *Handler) processWithdrawals(w http.ResponseWriter, r *http.Request) {
	if !decodeEmptyCommand(w, r) {
		return
	}
	httpx.OK(w, map[string]any{"processed": h.service.ProcessWithdrawals(20)})
}

func decodeEmptyCommand(w http.ResponseWriter, r *http.Request) bool {
	var body struct{}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return false
	}
	return true
}
