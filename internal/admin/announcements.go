package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func (h *Handler) listAnnouncementTemplates(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListAnnouncementTemplates(r.URL.Query().Get("kind"))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"items": rows, "count": len(rows)})
}

func (h *Handler) upsertAnnouncementTemplate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kind            string `json:"kind"`
		TitleTemplate   string `json:"titleTemplate"`
		BodyTemplate    string `json:"bodyTemplate"`
		DisplayMode     string `json:"displayMode"`
		Priority        int    `json:"priority"`
		DurationSeconds int    `json:"durationSeconds"`
		Enabled         *bool  `json:"enabled"`
		Reason          string `json:"reason"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	row, err := h.store.UpsertAnnouncementTemplate(store.UpsertAnnouncementTemplateInput{
		Code: r.PathValue("code"), Kind: body.Kind, TitleTemplate: body.TitleTemplate, BodyTemplate: body.BodyTemplate,
		DisplayMode: body.DisplayMode, Priority: body.Priority, DurationSeconds: body.DurationSeconds,
		Enabled: body.Enabled, UpdatedBy: authenticatedAdminID(r), Reason: body.Reason,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, row)
}

func (h *Handler) listAnnouncements(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	rows, err := h.store.ListAnnouncements(store.AnnouncementFilter{
		Kind: r.URL.Query().Get("kind"), Status: r.URL.Query().Get("status"), Limit: limit, Offset: offset,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"items": rows, "count": len(rows), "limit": limit, "offset": offset})
}

func (h *Handler) createOpsAnnouncement(w http.ResponseWriter, r *http.Request) {
	in, ok := h.decodeOpsAnnouncement(w, r)
	if !ok {
		return
	}
	row, err := h.store.CreateOpsAnnouncement(store.CreateOpsAnnouncementInput{
		AdminID: authenticatedAdminID(r), Title: in.title, Body: in.body, DisplayMode: in.displayMode,
		Priority: in.priority, StartsAt: in.startsAt, EndsAt: in.endsAt, Reason: in.reason,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.Created(w, row)
}

func (h *Handler) updateOpsAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("announcementId")), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, 400, "announcementId is invalid")
		return
	}
	in, ok := h.decodeOpsAnnouncement(w, r)
	if !ok {
		return
	}
	row, err := h.store.UpdateOpsAnnouncement(id, store.UpdateOpsAnnouncementInput{
		AdminID: authenticatedAdminID(r), Title: in.title, Body: in.body, DisplayMode: in.displayMode,
		Priority: in.priority, StartsAt: in.startsAt, EndsAt: in.endsAt, Reason: in.reason,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, row)
}

func (h *Handler) revokeAnnouncement(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("announcementId")), 10, 64)
	if err != nil || id <= 0 {
		httpx.Error(w, http.StatusBadRequest, 400, "announcementId is invalid")
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	row, err := h.store.RevokeAnnouncement(id, authenticatedAdminID(r), body.Reason)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, row)
}

type opsAnnouncementBody struct {
	title       string
	body        string
	displayMode string
	priority    int
	startsAt    time.Time
	endsAt      *time.Time
	reason      string
}

func (h *Handler) decodeOpsAnnouncement(w http.ResponseWriter, r *http.Request) (opsAnnouncementBody, bool) {
	var body struct {
		Title       string `json:"title"`
		Body        string `json:"body"`
		DisplayMode string `json:"displayMode"`
		Priority    int    `json:"priority"`
		StartsAt    string `json:"startsAt"`
		EndsAt      string `json:"endsAt"`
		Reason      string `json:"reason"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return opsAnnouncementBody{}, false
	}
	startsAt, err := parseAdminOptionalTime(body.StartsAt)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, "startsAt must be RFC3339")
		return opsAnnouncementBody{}, false
	}
	endsAt, err := parseAdminOptionalTime(body.EndsAt)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, "endsAt must be RFC3339")
		return opsAnnouncementBody{}, false
	}
	var ends *time.Time
	if !endsAt.IsZero() {
		ends = &endsAt
	}
	return opsAnnouncementBody{
		title: strings.TrimSpace(body.Title), body: strings.TrimSpace(body.Body), displayMode: body.DisplayMode,
		priority: body.Priority, startsAt: startsAt, endsAt: ends, reason: body.Reason,
	}, true
}

func parseAdminOptionalTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339, value)
}
