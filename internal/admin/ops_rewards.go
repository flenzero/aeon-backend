package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type opsRewardItem struct {
	ItemID   string `json:"itemId"`
	Quantity int64  `json:"quantity"`
	Rarity   int    `json:"rarity"`
}

func (h *Handler) opsCharacterID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	characterID, err := strconv.ParseInt(r.PathValue("characterId"), 10, 64)
	if err != nil || characterID <= 0 {
		httpx.Error(w, http.StatusBadRequest, 400, "characterId is invalid")
		return 0, false
	}
	return characterID, true
}

func (h *Handler) configuredRewardPlan(opID string, items []opsRewardItem) (store.DungeonRewardPlan, error) {
	if h.rulesErr != nil || h.rules == nil {
		return store.DungeonRewardPlan{}, fmt.Errorf("economy rules are unavailable: %w", h.rulesErr)
	}
	plan := store.DungeonRewardPlan{}
	for index, item := range items {
		itemID := strings.TrimSpace(item.ItemID)
		if itemID == "" || item.Quantity <= 0 {
			return store.DungeonRewardPlan{}, fmt.Errorf("items[%d] requires itemId and positive quantity", index)
		}
		definition, ok := h.rules.Items[itemID]
		if !ok {
			return store.DungeonRewardPlan{}, fmt.Errorf("items[%d] references unknown configured item %q", index, itemID)
		}
		reward := store.DungeonRewardGrant{
			ItemID: itemID, Quantity: item.Quantity, Rarity: definition.Rarity, Category: definition.Category, RewardType: "item",
		}
		if definition.IsEquipment {
			if item.Quantity != 1 {
				return store.DungeonRewardPlan{}, fmt.Errorf("equipment %q quantity must be 1", itemID)
			}
			if item.Rarity > 0 {
				if _, ok := h.rules.EquipmentRarity(item.Rarity); !ok {
					return store.DungeonRewardPlan{}, fmt.Errorf("items[%d] rarity is not configured", index)
				}
				reward.Rarity = item.Rarity
			}
			reward.RewardType = "equipment"
			reward.EquipmentUID = fmt.Sprintf("admin-%s-%d", strings.TrimSpace(opID), index)
		}
		plan.Items = append(plan.Items, reward)
	}
	return plan, nil
}

func (h *Handler) grantOpsRewards(w http.ResponseWriter, r *http.Request) {
	var body struct {
		serverOperation
		Gold               int64           `json:"gold"`
		WithdrawableAEB    int64           `json:"withdrawableAeb"`
		LockedAEB          int64           `json:"lockedAeb"`
		Items              []opsRewardItem `json:"items"`
		AnnounceRare       bool            `json:"announceRare"`
		AnnouncementSource string          `json:"announcementSource"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if !requireOperation(w, body.serverOperation) {
		return
	}
	characterID, ok := h.opsCharacterID(w, r)
	if !ok {
		return
	}
	plan, err := h.configuredRewardPlan(body.OpID, body.Items)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	result, err := h.store.AdminGrantRewards(store.AdminRewardGrant{
		OpID: body.OpID, AdminID: authenticatedAdminID(r), CharacterID: characterID, Reason: body.Reason,
		Gold: body.Gold, WithdrawableAEB: body.WithdrawableAEB, LockedAEB: body.LockedAEB, Items: plan.Items,
		AnnounceRare: body.AnnounceRare, AnnouncementSource: body.AnnouncementSource,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, result)
}

func (h *Handler) drawOpsLottery(w http.ResponseWriter, r *http.Request) {
	var body struct {
		serverOperation
		Count  int  `json:"count"`
		DryRun bool `json:"dryRun"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if !requireOperation(w, body.serverOperation) {
		return
	}
	characterID, ok := h.opsCharacterID(w, r)
	if !ok {
		return
	}
	if h.replayOperation(w, r, body.OpID, "ops_lottery_preview") {
		return
	}
	if h.rulesErr != nil || h.rules == nil {
		httpx.Error(w, http.StatusServiceUnavailable, 4002, "economy rules are unavailable")
		return
	}
	if body.Count < 1 || body.Count > h.rules.Lottery.MaxCount {
		httpx.Error(w, http.StatusBadRequest, 400, fmt.Sprintf("count must be 1..%d", h.rules.Lottery.MaxCount))
		return
	}
	level, err := h.store.AdminCharacterLevel(characterID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if body.DryRun {
		plan, err := h.rules.LotteryPlan(fmt.Sprintf("preview-%d", time.Now().UnixNano()), characterID, level, body.Count)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		preview, err := h.store.CreateAdminLotteryPreview(authenticatedAdminID(r), characterID, plan)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		audit := h.store.AuditTarget(authenticatedAdminID(r), "ops_lottery_preview", "character", strconv.FormatInt(characterID, 10), body.Reason+" [opId="+body.OpID+"]")
		h.completeOperation(w, r, body.OpID, "ops_lottery_preview", "character:"+strconv.FormatInt(characterID, 10), map[string]any{"dryRun": true, "preview": preview, "audit": audit})
		return
	}
	httpx.Error(w, http.StatusBadRequest, 400, "lottery draw requires dryRun=true; commit a persisted preview instead")
}

func (h *Handler) commitOpsLotteryPreview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PreviewID string `json:"previewId"`
		OpID      string `json:"opId"`
		Reason    string `json:"reason"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if !requireOperation(w, serverOperation{OpID: body.OpID, Reason: body.Reason}) {
		return
	}
	characterID, ok := h.opsCharacterID(w, r)
	if !ok {
		return
	}
	result, err := h.store.CommitAdminLotteryPreview(store.AdminLotteryCommit{PreviewID: body.PreviewID, OpID: body.OpID, AdminID: authenticatedAdminID(r), CharacterID: characterID, Reason: body.Reason})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, result)
}
