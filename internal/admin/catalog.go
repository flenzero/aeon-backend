package admin

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/flenzero/aeon-backend/internal/economy/rules"
	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type adminCatalogCategory struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type adminCatalogItem struct {
	ItemID               string  `json:"itemId"`
	DisplayName          string  `json:"displayName"`
	Category             string  `json:"category"`
	CategoryLabel        string  `json:"categoryLabel"`
	Rarity               int     `json:"rarity"`
	RarityLabel          string  `json:"rarityLabel"`
	IsEquipment          bool    `json:"isEquipment"`
	EquipmentSlot        *int    `json:"equipmentSlot"`
	EquipmentSlotLabel   *string `json:"equipmentSlotLabel"`
	SeriesID             string  `json:"seriesId,omitempty"`
	Stage                int     `json:"stage,omitempty"`
	DisplayType          string  `json:"displayType,omitempty"`
	Stackable            bool    `json:"stackable"`
	MaxGrantQuantity     int64   `json:"maxGrantQuantity"`
	EnabledForAdminGrant bool    `json:"enabledForAdminGrant"`
}

type adminCatalogGroup struct {
	Key   string             `json:"key"`
	Label string             `json:"label"`
	Count int                `json:"count"`
	Items []adminCatalogItem `json:"items"`
}

type adminRewardItemView struct {
	ItemID      string `json:"itemId"`
	DisplayName string `json:"displayName"`
	Category    string `json:"category"`
	Rarity      int    `json:"rarity"`
	Quantity    int64  `json:"quantity"`
	IsEquipment bool   `json:"isEquipment"`
}

type adminRewardView struct {
	Gold            int64                 `json:"gold,omitempty"`
	WithdrawableAEB int64                 `json:"withdrawableAeb,omitempty"`
	LockedAEB       int64                 `json:"lockedAeb,omitempty"`
	Items           []adminRewardItemView `json:"items,omitempty"`
}

var adminCatalogCategories = []adminCatalogCategory{
	{Key: "material", Label: "材料"},
	{Key: "rare_material", Label: "稀有材料"},
	{Key: "consumable", Label: "消耗品"},
	{Key: "aeb_voucher", Label: "AEB 凭证"},
	{Key: "seed", Label: "种子"},
	{Key: "crop", Label: "作物"},
	{Key: "boss_ticket", Label: "Boss 门票"},
	{Key: "bounty_badge", Label: "悬赏徽章"},
	{Key: "enhancement_stone", Label: "强化石"},
	{Key: "weapon", Label: "武器"},
	{Key: "armor", Label: "防具"},
	{Key: "accessory", Label: "饰品"},
	{Key: "mount", Label: "坐骑"},
}

func categoryLabel(category string) string {
	category = strings.TrimSpace(category)
	for _, row := range adminCatalogCategories {
		if row.Key == category {
			return row.Label
		}
	}
	if category == "" {
		return "未分类"
	}
	return category
}

func equipmentSlotLabel(slot int) string {
	switch slot {
	case 0:
		return "手套"
	case 1:
		return "鞋子"
	case 2:
		return "头盔"
	case 3:
		return "胸甲"
	case 4:
		return "武器"
	case 5:
		return "披风"
	case 6:
		return "坐骑"
	case 7:
		return "饰品"
	default:
		return "未知"
	}
}

func rarityLabel(cfg *rules.Config, rarity int, isEquipment bool) string {
	if isEquipment && cfg != nil {
		if row, ok := cfg.EquipmentRarity(rarity); ok && strings.TrimSpace(row.Name) != "" {
			return row.Name
		}
	}
	switch rarity {
	case 1:
		return "普通"
	case 2:
		return "优秀"
	case 3:
		return "稀有"
	case 4:
		return "史诗"
	case 5:
		return "传说"
	case 6:
		return "神话"
	default:
		if rarity <= 0 {
			return "未分级"
		}
		return fmt.Sprintf("R%d", rarity)
	}
}

func catalogLimitOffset(r *http.Request) (int, int, error) {
	limit := 200
	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 500 {
			return 0, 0, fmt.Errorf("limit must be between 1 and 500")
		}
		limit = value
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return 0, 0, fmt.Errorf("offset must be non-negative")
		}
		offset = value
	}
	return limit, offset, nil
}

func equipmentDisplayTypes(cfg *rules.Config) map[string]string {
	out := map[string]string{}
	if cfg == nil {
		return out
	}
	for _, series := range cfg.Equipment.Series {
		for _, stage := range series.Stages {
			out[strings.TrimSpace(stage.ItemID)] = strings.TrimSpace(series.DisplayType)
		}
	}
	for _, mount := range cfg.Equipment.Mounts {
		out[strings.TrimSpace(mount.ItemID)] = "坐骑"
	}
	return out
}

func (h *Handler) catalogVersion() string {
	if h.rules == nil {
		return ""
	}
	raw, _ := json.Marshal(struct {
		Version   string
		Items     map[string]rules.Item
		Equipment map[string]rules.EquipmentTemplate
	}{
		Version:   h.rules.Rules.Version,
		Items:     h.rules.Items,
		Equipment: h.rules.Equipment.ByItemID,
	})
	sum := sha256.Sum256(raw)
	version := strings.TrimSpace(h.rules.Rules.Version)
	if version == "" {
		return fmt.Sprintf("%x", sum[:8])
	}
	return fmt.Sprintf("%s-%x", version, sum[:8])
}

func (h *Handler) catalogItems() []adminCatalogItem {
	displayTypes := equipmentDisplayTypes(h.rules)
	items := make([]adminCatalogItem, 0, len(h.rules.Items))
	for _, item := range h.rules.Items {
		item.ItemID = strings.TrimSpace(item.ItemID)
		if item.ItemID == "" {
			continue
		}
		displayName := strings.TrimSpace(item.DisplayName)
		if displayName == "" {
			displayName = item.ItemID
		}
		stackable := !item.IsEquipment && item.MaxStack != 1
		maxGrantQuantity := int64(999999)
		if item.IsEquipment {
			maxGrantQuantity = 1
			stackable = false
		}
		row := adminCatalogItem{
			ItemID:               item.ItemID,
			DisplayName:          displayName,
			Category:             item.Category,
			CategoryLabel:        categoryLabel(item.Category),
			Rarity:               item.Rarity,
			RarityLabel:          rarityLabel(h.rules, item.Rarity, item.IsEquipment),
			IsEquipment:          item.IsEquipment,
			Stackable:            stackable,
			MaxGrantQuantity:     maxGrantQuantity,
			EnabledForAdminGrant: true,
		}
		if item.IsEquipment {
			slot := item.EquipSlot
			label := equipmentSlotLabel(slot)
			row.EquipmentSlot = &slot
			row.EquipmentSlotLabel = &label
			row.DisplayType = displayTypes[item.ItemID]
			if template, ok := h.rules.EquipmentTemplate(item.ItemID); ok {
				row.SeriesID = template.SeriesID
				row.Stage = template.Stage
				row.Category = template.Category
				row.CategoryLabel = categoryLabel(template.Category)
				slot = template.EquipSlot
				label = equipmentSlotLabel(slot)
				row.EquipmentSlot = &slot
				row.EquipmentSlotLabel = &label
			}
		}
		items = append(items, row)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Category == items[j].Category {
			if items[i].DisplayName == items[j].DisplayName {
				return items[i].ItemID < items[j].ItemID
			}
			return items[i].DisplayName < items[j].DisplayName
		}
		return items[i].Category < items[j].Category
	})
	return items
}

func filterCatalogItems(items []adminCatalogItem, r *http.Request) ([]adminCatalogItem, error) {
	keyword := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("keyword")))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	isEquipment, err := optionalBoolPtrQuery(r, "isEquipment")
	if err != nil {
		return nil, fmt.Errorf("isEquipment must be true or false")
	}
	rarity := -1
	if raw := strings.TrimSpace(r.URL.Query().Get("rarity")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return nil, fmt.Errorf("rarity must be a non-negative integer")
		}
		rarity = value
	}
	out := make([]adminCatalogItem, 0, len(items))
	for _, item := range items {
		if keyword != "" {
			haystack := strings.ToLower(strings.Join([]string{
				item.ItemID,
				item.DisplayName,
				item.Category,
				item.CategoryLabel,
				item.RarityLabel,
				item.DisplayType,
			}, " "))
			if !strings.Contains(haystack, keyword) {
				continue
			}
		}
		if category != "" && item.Category != category {
			continue
		}
		if isEquipment != nil && item.IsEquipment != *isEquipment {
			continue
		}
		if rarity >= 0 && item.Rarity != rarity {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func groupedCatalogItems(items []adminCatalogItem) []adminCatalogGroup {
	byCategory := map[string][]adminCatalogItem{}
	for _, item := range items {
		byCategory[item.Category] = append(byCategory[item.Category], item)
	}
	groups := []adminCatalogGroup{}
	seen := map[string]bool{}
	for _, category := range adminCatalogCategories {
		seen[category.Key] = true
		if rows := byCategory[category.Key]; len(rows) > 0 {
			groups = append(groups, adminCatalogGroup{Key: category.Key, Label: category.Label, Count: len(rows), Items: rows})
		}
	}
	extras := make([]string, 0)
	for key := range byCategory {
		if !seen[key] {
			extras = append(extras, key)
		}
	}
	sort.Strings(extras)
	for _, key := range extras {
		rows := byCategory[key]
		groups = append(groups, adminCatalogGroup{Key: key, Label: categoryLabel(key), Count: len(rows), Items: rows})
	}
	return groups
}

func (h *Handler) listCatalogItems(w http.ResponseWriter, r *http.Request) {
	if h.rulesErr != nil || h.rules == nil {
		httpx.Error(w, http.StatusServiceUnavailable, 4002, "economy rules are unavailable")
		return
	}
	limit, offset, err := catalogLimitOffset(r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	items, err := filterCatalogItems(h.catalogItems(), r)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	total := len(items)
	if offset > len(items) {
		items = []adminCatalogItem{}
	} else {
		end := offset + limit
		if end > len(items) {
			end = len(items)
		}
		items = items[offset:end]
	}
	grouped, err := optionalBoolQuery(r, "grouped")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	if grouped {
		httpx.OK(w, map[string]any{
			"configVersion": h.catalogVersion(),
			"groups":        groupedCatalogItems(items),
			"categories":    adminCatalogCategories,
			"count":         len(items),
			"total":         total,
			"limit":         limit,
			"offset":        offset,
		})
		return
	}
	httpx.OK(w, map[string]any{
		"configVersion": h.catalogVersion(),
		"items":         items,
		"categories":    adminCatalogCategories,
		"count":         len(items),
		"total":         total,
		"limit":         limit,
		"offset":        offset,
	})
}

func (h *Handler) rewardView(rewards store.AdminCompensationRewards) adminRewardView {
	out := adminRewardView{
		Gold:            rewards.Gold,
		WithdrawableAEB: rewards.WithdrawableAEB,
		LockedAEB:       rewards.LockedAEB,
		Items:           []adminRewardItemView{},
	}
	byID := map[string]adminCatalogItem{}
	if h.rulesErr == nil && h.rules != nil {
		for _, item := range h.catalogItems() {
			byID[item.ItemID] = item
		}
	}
	for _, item := range rewards.Items {
		def := byID[item.ItemID]
		displayName := def.DisplayName
		if displayName == "" {
			displayName = item.ItemID
		}
		category := def.Category
		if category == "" {
			category = item.Category
		}
		rarity := def.Rarity
		if item.Rarity > 0 {
			rarity = item.Rarity
		}
		isEquipment := def.IsEquipment || strings.EqualFold(item.RewardType, "equipment")
		out.Items = append(out.Items, adminRewardItemView{
			ItemID:      item.ItemID,
			DisplayName: displayName,
			Category:    category,
			Rarity:      rarity,
			Quantity:    item.Quantity,
			IsEquipment: isEquipment,
		})
	}
	return out
}
