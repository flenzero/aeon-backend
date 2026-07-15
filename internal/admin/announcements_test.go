package admin

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func TestAdminAnnouncementNoticeAndTemplateFlow(t *testing.T) {
	cfg := adminTestConfig()
	st := store.New()
	handler := NewHandler(cfg, st).Routes()
	adminHeaders := map[string]string{"Authorization": "Bearer " + cfg.AdminToken}

	var notice store.Announcement
	doAdminJSON(t, handler, http.MethodPost, "/api/admin/announcements/notices", map[string]any{
		"title": "维护通知", "body": "今晚 20:00 开始维护", "displayMode": "BANNER", "reason": "scheduled maintenance",
	}, adminHeaders, http.StatusCreated, &notice)
	if notice.ID == 0 || notice.Kind != store.AnnouncementKindOpsNotice || notice.Status != store.AnnouncementStatusActive {
		t.Fatalf("notice = %+v", notice)
	}

	var listed struct {
		Items []store.Announcement `json:"items"`
		Count int                  `json:"count"`
	}
	doAdminJSON(t, handler, http.MethodGet, "/api/admin/announcements?kind=OPS_NOTICE", nil, adminHeaders, http.StatusOK, &listed)
	if listed.Count != 1 || listed.Items[0].ID != notice.ID {
		t.Fatalf("listed = %+v", listed)
	}

	var revoked store.Announcement
	doAdminJSON(t, handler, http.MethodPost, "/api/admin/announcements/"+strconv.FormatInt(notice.ID, 10)+"/revoke", map[string]any{
		"reason": "message superseded",
	}, adminHeaders, http.StatusOK, &revoked)
	if revoked.Status != store.AnnouncementStatusRevoked {
		t.Fatalf("revoked = %+v", revoked)
	}

	enabled := true
	var template store.AnnouncementTemplate
	doAdminJSON(t, handler, http.MethodPut, "/api/admin/announcements/templates/rare_equipment", map[string]any{
		"titleTemplate": "神装现世", "bodyTemplate": "{characterName} 通过{source}获得 {itemName}",
		"displayMode": "POPUP", "priority": 999, "durationSeconds": 9, "enabled": enabled, "reason": "copy update",
	}, adminHeaders, http.StatusOK, &template)
	if template.Code != store.AnnouncementTemplateRareEquipment || template.Priority != 999 {
		t.Fatalf("template = %+v", template)
	}
}
