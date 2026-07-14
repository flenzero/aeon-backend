package admin

import (
	"net/http"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func compensationTime(raw string) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	value, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, err
	}
	utc := value.UTC()
	return &utc, nil
}

func (h *Handler) previewOpsCompensation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		serverOperation
		Filters struct {
			MinLevel          int    `json:"minLevel"`
			MaxLevel          int    `json:"maxLevel"`
			LastLoginFrom     string `json:"lastLoginFrom"`
			LastLoginTo       string `json:"lastLoginTo"`
			MinClearedChapter int    `json:"minClearedChapter"`
			MinClearedFloor   int    `json:"minClearedFloor"`
			MinClearCount     int64  `json:"minDungeonClearCount"`
			HasLicense        *bool  `json:"hasTradingLicense"`
		} `json:"filters"`
		Gold            int64           `json:"gold"`
		WithdrawableAEB int64           `json:"withdrawableAeb"`
		LockedAEB       int64           `json:"lockedAeb"`
		Items           []opsRewardItem `json:"items"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if !requireOperation(w, body.serverOperation) {
		return
	}
	if h.replayOperation(w, r, body.OpID, "ops_compensation_preview") {
		return
	}
	from, err := compensationTime(body.Filters.LastLoginFrom)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, "lastLoginFrom must be RFC3339")
		return
	}
	to, err := compensationTime(body.Filters.LastLoginTo)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, "lastLoginTo must be RFC3339")
		return
	}
	plan, err := h.configuredRewardPlan("compensation-preview", body.Items)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	result, err := h.store.CreateAdminCompensationPreview(authenticatedAdminID(r), store.AdminCompensationFilter{MinLevel: body.Filters.MinLevel, MaxLevel: body.Filters.MaxLevel, LastLoginFrom: from, LastLoginTo: to, MinClearedChapter: body.Filters.MinClearedChapter, MinClearedFloor: body.Filters.MinClearedFloor, MinClearCount: body.Filters.MinClearCount, HasLicense: body.Filters.HasLicense}, store.AdminCompensationRewards{Gold: body.Gold, WithdrawableAEB: body.WithdrawableAEB, LockedAEB: body.LockedAEB, Items: plan.Items})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	targetID := result.PreviewID
	if targetID == "" {
		targetID = "empty"
	}
	audit := h.store.AuditTarget(authenticatedAdminID(r), "ops_compensation_preview", "compensation_preview", targetID, body.Reason+" [opId="+body.OpID+"]")
	h.completeOperation(w, r, body.OpID, "ops_compensation_preview", "compensation_preview:"+targetID, map[string]any{"preview": result, "audit": audit})
}

func (h *Handler) getOpsCompensationPreview(w http.ResponseWriter, r *http.Request) {
	previewID := r.PathValue("previewId")
	row, err := h.store.AdminCompensationPreview(previewID, authenticatedAdminID(r))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{
		"preview": map[string]any{
			"previewId":   row.PreviewID,
			"status":      row.Status,
			"targetCount": row.TargetCount,
			"expiresAt":   row.ExpiresAt,
			"filters":     row.Filters,
			"rewards":     h.rewardView(row.Rewards),
			"createdAt":   row.CreatedAt,
			"committedAt": row.CommittedAt,
		},
	})
}

func (h *Handler) listOpsCompensationPreviewTargets(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	page, err := h.store.ListAdminCompensationPreviewTargets(r.PathValue("previewId"), authenticatedAdminID(r), r.URL.Query().Get("keyword"), limit, offset)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, page)
}

func (h *Handler) commitOpsCompensation(w http.ResponseWriter, r *http.Request) {
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
	result, err := h.store.CommitAdminCompensation(store.AdminCompensationCommit{PreviewID: body.PreviewID, OpID: body.OpID, AdminID: authenticatedAdminID(r), Reason: body.Reason})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, result)
}
