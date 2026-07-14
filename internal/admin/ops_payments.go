package admin

import (
	"errors"
	"net/http"

	"github.com/flenzero/aeon-backend/internal/platform/httpx"
)

func (h *Handler) traceOpsPayment(w http.ResponseWriter, r *http.Request) {
	trace, err := h.store.AdminPaymentTrace(r.PathValue("orderId"))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, trace)
}

func (h *Handler) recoverOpsPayment(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OpID   string `json:"opId"`
		Reason string `json:"reason"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if !requireOperation(w, serverOperation{OpID: body.OpID, Reason: body.Reason}) {
		return
	}
	if h.replayOperation(w, r, body.OpID, "ops_payment_recover") {
		return
	}
	trace, err := h.store.AdminPaymentTrace(r.PathValue("orderId"))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if trace.Order.Status == "FULFILLED" {
		writeStoreErr(w, errors.New("fulfilled payment orders cannot be recovered; use an explicit admin reward grant for compensation"))
		return
	}
	order, err := h.store.ConfirmPaymentOrder(r.Context(), r.PathValue("orderId"), "admin_recovery: "+body.Reason)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	audit := h.store.AuditTarget(authenticatedAdminID(r), "ops_payment_recover", "economy_payment_order", order.ID, body.Reason+" [opId="+body.OpID+"]")
	h.completeOperation(w, r, body.OpID, "ops_payment_recover", "economy_payment_order:"+order.ID, map[string]any{"order": order, "audit": audit})
}
