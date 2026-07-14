package admin

import (
	"net/http"
	"strings"

	"github.com/flenzero/aeon-backend/internal/platform/httpx"
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
	httpx.OK(w, detail)
}
