package admin

import (
	"net/http"
	"testing"

	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func TestAdminCatalogExpandsTemplateEquipmentRarities(t *testing.T) {
	cfg := adminTestConfig()
	handler := NewHandler(cfg, store.New()).Routes()
	headers := map[string]string{"Authorization": "Bearer " + cfg.AdminToken}

	var listed struct {
		Items []adminCatalogItem `json:"items"`
		Count int                `json:"count"`
		Total int                `json:"total"`
	}
	doAdminJSON(t, handler, http.MethodGet, "/api/admin/catalog/items?isEquipment=true&rarity=6&limit=500", nil, headers, http.StatusOK, &listed)
	if listed.Count == 0 || listed.Total == 0 {
		t.Fatalf("rarity=6 equipment catalog is empty: %+v", listed)
	}
	for _, item := range listed.Items {
		if !item.IsEquipment || item.Rarity != 6 {
			t.Fatalf("rarity=6 equipment filter returned unexpected item: %+v", item)
		}
	}

	for _, item := range listed.Items {
		if item.ItemID == "aeonblight_sword_t30" && item.RarityLabel == "至臻" && item.SeriesID == "sword" && item.Stage == 30 && item.SellPrice == 7850 {
			return
		}
	}
	t.Fatalf("rarity=6 catalog did not include aeonblight_sword_t30: %+v", listed.Items)
}

func TestGroupedAdminCatalogReturnsCompleteFilteredGroups(t *testing.T) {
	cfg := adminTestConfig()
	handler := NewHandler(cfg, store.New()).Routes()
	headers := map[string]string{"Authorization": "Bearer " + cfg.AdminToken}

	var listed struct {
		Groups []adminCatalogGroup `json:"groups"`
		Count  int                 `json:"count"`
		Total  int                 `json:"total"`
		Limit  int                 `json:"limit"`
		Offset int                 `json:"offset"`
	}
	doAdminJSON(t, handler, http.MethodGet, "/api/admin/catalog/items?grouped=true&limit=50", nil, headers, http.StatusOK, &listed)
	if listed.Total <= 50 {
		t.Fatalf("test fixture should exceed requested limit: %+v", listed)
	}
	if listed.Count != listed.Total || listed.Limit != listed.Total || listed.Offset != 0 {
		t.Fatalf("grouped catalog should return complete filtered groups, got count=%d total=%d limit=%d offset=%d", listed.Count, listed.Total, listed.Limit, listed.Offset)
	}

	var weaponGroup *adminCatalogGroup
	for index := range listed.Groups {
		if listed.Groups[index].Key == "weapon" {
			weaponGroup = &listed.Groups[index]
			break
		}
	}
	if weaponGroup == nil {
		t.Fatalf("grouped catalog did not include weapon group: %+v", listed.Groups)
	}
	raritiesByItemID := map[string]map[int]bool{}
	for _, item := range weaponGroup.Items {
		if _, ok := raritiesByItemID[item.ItemID]; !ok {
			raritiesByItemID[item.ItemID] = map[int]bool{}
		}
		raritiesByItemID[item.ItemID][item.Rarity] = true
	}
	if got := len(raritiesByItemID["aeonblight_sword_t30"]); got != 6 {
		t.Fatalf("aeonblight_sword_t30 rarity variants = %d, want 6", got)
	}
}
