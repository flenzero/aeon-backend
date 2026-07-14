package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func pathCharacterID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	characterID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("characterId")), 10, 64)
	if err != nil || characterID <= 0 {
		httpx.Error(w, http.StatusBadRequest, 400, "characterId is invalid")
		return 0, false
	}
	return characterID, true
}

func optionalIntQuery(r *http.Request, key string) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0, strconv.ErrSyntax
	}
	return value, nil
}

func optionalBoolPtrQuery(r *http.Request, key string) (*bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func validateAccountStatus(raw string) (string, bool) {
	status := strings.ToUpper(strings.TrimSpace(raw))
	switch status {
	case "", "ACTIVE", "BANNED", "FROZEN", "DELETED":
		return status, true
	default:
		return "", false
	}
}

func (h *Handler) listCharacters(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	accountID, _, err := httpx.OptionalPositiveInt64(r, "accountId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	minLevel, err := optionalIntQuery(r, "minLevel")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, "minLevel must be a non-negative integer")
		return
	}
	maxLevel, err := optionalIntQuery(r, "maxLevel")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, "maxLevel must be a non-negative integer")
		return
	}
	hasLicense, err := optionalBoolPtrQuery(r, "hasTradingLicense")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, "hasTradingLicense must be true or false")
		return
	}
	onlineOnly, err := optionalBoolQuery(r, "onlineOnly")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	status, ok := validateAccountStatus(r.URL.Query().Get("status"))
	if !ok {
		httpx.Error(w, http.StatusBadRequest, 400, "status must be ACTIVE, BANNED, FROZEN, or DELETED")
		return
	}
	rows, err := h.store.ListAdminCharacters(store.AdminCharacterListFilter{
		Keyword:           r.URL.Query().Get("keyword"),
		AccountID:         accountID,
		Wallet:            r.URL.Query().Get("wallet"),
		MinLevel:          minLevel,
		MaxLevel:          maxLevel,
		HasTradingLicense: hasLicense,
		Status:            status,
		OnlineOnly:        onlineOnly,
		ServerID:          r.URL.Query().Get("serverId"),
		Limit:             limit,
		Offset:            offset,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"items": rows, "count": len(rows), "limit": limit, "offset": offset})
}

func characterInclude(raw string) map[string]bool {
	if strings.TrimSpace(raw) == "" {
		return map[string]bool{"account": true, "snapshot": true, "online": true}
	}
	out := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		key := strings.ToLower(strings.TrimSpace(part))
		if key == "all" {
			return map[string]bool{"account": true, "snapshot": true, "economy": true, "online": true, "ledger": true, "audits": true}
		}
		if key != "" {
			out[key] = true
		}
	}
	return out
}

func (h *Handler) getCharacter(w http.ResponseWriter, r *http.Request) {
	characterID, ok := pathCharacterID(w, r)
	if !ok {
		return
	}
	detail, err := h.store.AdminCharacterDetail(characterID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	include := characterInclude(r.URL.Query().Get("include"))
	data := map[string]any{"character": detail.Character}
	if include["account"] {
		data["account"] = detail.Account
	}
	if include["snapshot"] || include["economy"] {
		data["economy"] = detail.Economy
	}
	if include["online"] {
		data["online"] = detail.Online
	}
	if include["ledger"] {
		rows, err := h.store.ListCharacterLedger(characterID, "", 50, 0)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		data["ledger"] = rows
	}
	if include["audits"] {
		rows, err := h.store.ListCharacterAudits(characterID, 50, 0)
		if err != nil {
			writeStoreErr(w, err)
			return
		}
		data["audits"] = rows
	}
	httpx.OK(w, data)
}

func (h *Handler) listCharacterLedger(w http.ResponseWriter, r *http.Request) {
	characterID, ok := pathCharacterID(w, r)
	if !ok {
		return
	}
	limit, offset, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	rows, err := h.store.ListCharacterLedger(characterID, r.URL.Query().Get("kind"), limit, offset)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"ledger": rows, "count": len(rows), "limit": limit, "offset": offset})
}

func (h *Handler) listCharacterAudits(w http.ResponseWriter, r *http.Request) {
	characterID, ok := pathCharacterID(w, r)
	if !ok {
		return
	}
	limit, offset, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	rows, err := h.store.ListCharacterAudits(characterID, limit, offset)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"audits": rows, "count": len(rows), "limit": limit, "offset": offset})
}

func (h *Handler) listCharacterTimeline(w http.ResponseWriter, r *http.Request) {
	characterID, ok := pathCharacterID(w, r)
	if !ok {
		return
	}
	limit, offset, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	page, err := h.store.AdminCharacterTimeline(characterID, store.AdminCharacterTimelineFilter{
		Types:  r.URL.Query()["types"],
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, page)
}
