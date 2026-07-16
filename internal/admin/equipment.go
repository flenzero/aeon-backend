package admin

import (
	"net/http"
	"strings"

	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func (h *Handler) getEquipment(w http.ResponseWriter, r *http.Request) {
	equipmentUID := strings.TrimSpace(r.PathValue("equipmentUid"))
	if equipmentUID == "" {
		httpx.Error(w, http.StatusBadRequest, 400, "equipmentUid is required")
		return
	}
	detail, err := h.store.AdminEquipmentDetail(equipmentUID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	detail.Equipment = h.resolveEquipmentItem(detail.Equipment)
	httpx.OK(w, detail)
}

func (h *Handler) resolveEquipmentItem(item store.EquipmentItem) store.EquipmentItem {
	if h.rules == nil || h.rulesErr != nil {
		return item
	}
	resolved, err := h.rules.ResolveEquipmentItem(item)
	if err != nil {
		return item
	}
	return resolved
}

func (h *Handler) resolveEconomySnapshot(snapshot store.EconomySnapshot) store.EconomySnapshot {
	for index := range snapshot.Equipment {
		snapshot.Equipment[index] = h.resolveEquipmentItem(snapshot.Equipment[index])
	}
	return snapshot
}
