package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type Handler struct {
	cfg   config.Config
	store store.Repository
}

func NewHandler(cfg config.Config, st store.Repository) *Handler {
	return &Handler{cfg: cfg, store: st}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", httpx.Health(h.cfg.ServiceName))

	mux.HandleFunc("GET /api/admin/accounts", httpx.RequireAdmin(h.cfg, h.getAccount))
	mux.HandleFunc("POST /api/admin/accounts/ban", httpx.RequireAdmin(h.cfg, h.banAccount))
	mux.HandleFunc("POST /api/admin/accounts/risk-level", httpx.RequireAdmin(h.cfg, h.setRiskLevel))
	mux.HandleFunc("POST /api/admin/accounts/license", httpx.RequireAdmin(h.cfg, h.setLicense))
	mux.HandleFunc("POST /api/admin/accounts/sessions/revoke", httpx.RequireAdmin(h.cfg, h.revokeSessions))

	mux.HandleFunc("GET /api/admin/market/restrictions", httpx.RequireAdmin(h.cfg, h.listMarketRestrictions))
	mux.HandleFunc("POST /api/admin/market/restrictions", httpx.RequireAdmin(h.cfg, h.createMarketRestriction))
	mux.HandleFunc("POST /api/admin/market/restrictions/revoke", httpx.RequireAdmin(h.cfg, h.revokeMarketRestriction))

	mux.HandleFunc("GET /api/admin/risk/events", httpx.RequireAdmin(h.cfg, h.listRiskEvents))
	mux.HandleFunc("POST /api/admin/risk/events", httpx.RequireAdmin(h.cfg, h.createRiskEvent))

	mux.HandleFunc("GET /api/admin/audits", httpx.RequireAdmin(h.cfg, h.listAudits))
	mux.HandleFunc("GET /api/admin/ledger", httpx.RequireAdmin(h.cfg, h.listLedger))

	mux.HandleFunc("GET /api/admin/withdrawals", httpx.RequireAdmin(h.cfg, h.listWithdrawals))
	mux.HandleFunc("POST /api/admin/withdrawals/review", httpx.RequireAdmin(h.cfg, h.reviewWithdrawal))

	mux.HandleFunc("GET /api/admin/payments", httpx.RequireAdmin(h.cfg, h.listPayments))
	mux.HandleFunc("GET /api/admin/nft/requests", httpx.RequireAdmin(h.cfg, h.listNFTRequests))
	mux.HandleFunc("POST /api/admin/nft/mint/confirm", httpx.RequireAdmin(h.cfg, h.confirmNFTMint))

	mux.HandleFunc("GET /api/admin/hot-wallet", httpx.RequireAdmin(h.cfg, h.getHotWallet))
	mux.HandleFunc("POST /api/admin/hot-wallet/pause", httpx.RequireAdmin(h.cfg, h.pauseHotWallet))

	return httpx.Recover(httpx.WithCORS(mux))
}

func adminID(bodyAdmin string) string {
	if strings.TrimSpace(bodyAdmin) == "" {
		return "admin"
	}
	return strings.TrimSpace(bodyAdmin)
}

func queryInt64(r *http.Request, key string) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
	return v
}

func queryInt(r *http.Request, key string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

func writeStoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, 404, err.Error())
		return
	}
	httpx.Error(w, http.StatusBadRequest, 4001, err.Error())
}

func (h *Handler) getAccount(w http.ResponseWriter, r *http.Request) {
	accountID := queryInt64(r, "accountId")
	if accountID == 0 {
		accountID = queryInt64(r, "id")
	}
	wallet := strings.TrimSpace(r.URL.Query().Get("wallet"))
	row, err := h.store.AdminGetAccount(accountID, wallet)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, row)
}

func (h *Handler) banAccount(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID int64  `json:"accountId"`
		Banned    bool   `json:"banned"`
		Reason    string `json:"reason"`
		AdminID   string `json:"adminId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if body.AccountID == 0 {
		body.AccountID = queryInt64(r, "accountId")
	}
	if body.AccountID == 0 {
		httpx.Error(w, http.StatusBadRequest, 400, "accountId is required")
		return
	}
	if err := h.store.SetAccountBan(body.AccountID, body.Banned, body.Reason); err != nil {
		writeStoreErr(w, err)
		return
	}
	aid := adminID(body.AdminID)
	action := "account_unban"
	if body.Banned {
		action = "account_ban"
		_, _ = h.store.CreateRiskEvent(store.CreateRiskEventInput{
			AccountID: body.AccountID,
			EventType: "ADMIN_BAN",
			Severity:  80,
			Detail:    map[string]any{"reason": body.Reason, "adminId": aid},
		})
	}
	audit := h.store.AuditTarget(aid, action, "account", fmt.Sprint(body.AccountID), body.Reason)
	httpx.OK(w, map[string]any{"ok": true, "audit": audit})
}

func (h *Handler) setRiskLevel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID int64  `json:"accountId"`
		RiskLevel int    `json:"riskLevel"`
		Reason    string `json:"reason"`
		AdminID   string `json:"adminId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if body.AccountID == 0 {
		httpx.Error(w, http.StatusBadRequest, 400, "accountId is required")
		return
	}
	if err := h.store.SetAccountRiskLevel(body.AccountID, body.RiskLevel); err != nil {
		writeStoreErr(w, err)
		return
	}
	aid := adminID(body.AdminID)
	_, _ = h.store.CreateRiskEvent(store.CreateRiskEventInput{
		AccountID: body.AccountID,
		EventType: "ADMIN_RISK_LEVEL",
		Severity:  body.RiskLevel,
		Detail:    map[string]any{"reason": body.Reason, "adminId": aid, "riskLevel": body.RiskLevel},
	})
	audit := h.store.AuditTarget(aid, "account_risk_level", "account", fmt.Sprint(body.AccountID), body.Reason)
	httpx.OK(w, map[string]any{"ok": true, "riskLevel": body.RiskLevel, "audit": audit})
}

func (h *Handler) setLicense(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID int64  `json:"accountId"`
		Granted   bool   `json:"granted"`
		Reason    string `json:"reason"`
		AdminID   string `json:"adminId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if body.AccountID == 0 {
		httpx.Error(w, http.StatusBadRequest, 400, "accountId is required")
		return
	}
	if err := h.store.SetTradingLicense(body.AccountID, body.Granted); err != nil {
		writeStoreErr(w, err)
		return
	}
	aid := adminID(body.AdminID)
	action := "trading_license_revoke"
	if body.Granted {
		action = "trading_license_grant"
	}
	audit := h.store.AuditTarget(aid, action, "account", fmt.Sprint(body.AccountID), body.Reason)
	httpx.OK(w, map[string]any{"ok": true, "granted": body.Granted, "audit": audit})
}

func (h *Handler) revokeSessions(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID int64  `json:"accountId"`
		Reason    string `json:"reason"`
		AdminID   string `json:"adminId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if body.AccountID == 0 {
		httpx.Error(w, http.StatusBadRequest, 400, "accountId is required")
		return
	}
	n, err := h.store.AdminRevokeAccountSessions(body.AccountID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	aid := adminID(body.AdminID)
	audit := h.store.AuditTarget(aid, "account_sessions_revoke", "account", fmt.Sprint(body.AccountID), body.Reason)
	httpx.OK(w, map[string]any{"revoked": n, "audit": audit})
}

func (h *Handler) listMarketRestrictions(w http.ResponseWriter, r *http.Request) {
	activeOnly := strings.EqualFold(r.URL.Query().Get("activeOnly"), "true") || r.URL.Query().Get("activeOnly") == "1"
	rows, err := h.store.ListMarketRestrictions(queryInt64(r, "accountId"), activeOnly, queryInt(r, "limit", 50), queryInt(r, "offset", 0))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"restrictions": rows})
}

func (h *Handler) createMarketRestriction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID       int64  `json:"accountId"`
		RestrictionType string `json:"restrictionType"`
		Reason          string `json:"reason"`
		AdminID         string `json:"adminId"`
		ExpiresAt       string `json:"expiresAt"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	var expiresAt *time.Time
	if strings.TrimSpace(body.ExpiresAt) != "" {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(body.ExpiresAt))
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 400, "expiresAt must be RFC3339")
			return
		}
		utc := t.UTC()
		expiresAt = &utc
	}
	aid := adminID(body.AdminID)
	row, err := h.store.CreateMarketRestriction(store.CreateMarketRestrictionInput{
		AccountID:       body.AccountID,
		RestrictionType: body.RestrictionType,
		Reason:          body.Reason,
		CreatedBy:       aid,
		ExpiresAt:       expiresAt,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	_, _ = h.store.CreateRiskEvent(store.CreateRiskEventInput{
		AccountID: body.AccountID,
		EventType: "MARKET_RESTRICTION",
		Severity:  50,
		Detail: map[string]any{
			"restrictionType": row.RestrictionType,
			"restrictionId":   row.ID,
			"reason":          body.Reason,
			"adminId":         aid,
		},
	})
	audit := h.store.AuditTarget(aid, "market_restriction_create", "market_restriction", fmt.Sprint(row.ID), body.Reason)
	httpx.OK(w, map[string]any{"restriction": row, "audit": audit})
}

func (h *Handler) revokeMarketRestriction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID      int64  `json:"id"`
		Reason  string `json:"reason"`
		AdminID string `json:"adminId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	aid := adminID(body.AdminID)
	row, err := h.store.RevokeMarketRestriction(body.ID, aid, body.Reason)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	audit := h.store.AuditTarget(aid, "market_restriction_revoke", "market_restriction", fmt.Sprint(row.ID), body.Reason)
	httpx.OK(w, map[string]any{"restriction": row, "audit": audit})
}

func (h *Handler) listRiskEvents(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListRiskEvents(queryInt64(r, "accountId"), queryInt(r, "limit", 50), queryInt(r, "offset", 0))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"events": rows})
}

func (h *Handler) createRiskEvent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AccountID int64          `json:"accountId"`
		EventType string         `json:"eventType"`
		Severity  int            `json:"severity"`
		DeviceID  string         `json:"deviceId"`
		IPAddress string         `json:"ipAddress"`
		Wallet    string         `json:"wallet"`
		Detail    map[string]any `json:"detail"`
		AdminID   string         `json:"adminId"`
		Reason    string         `json:"reason"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	row, err := h.store.CreateRiskEvent(store.CreateRiskEventInput{
		AccountID: body.AccountID,
		EventType: body.EventType,
		Severity:  body.Severity,
		DeviceID:  body.DeviceID,
		IPAddress: body.IPAddress,
		Wallet:    body.Wallet,
		Detail:    body.Detail,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	aid := adminID(body.AdminID)
	audit := h.store.AuditTarget(aid, "risk_event_create", "risk_event", fmt.Sprint(row.ID), body.Reason)
	httpx.OK(w, map[string]any{"event": row, "audit": audit})
}

func (h *Handler) listAudits(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListAudits(queryInt(r, "limit", 50), queryInt(r, "offset", 0))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"audits": rows})
}

func (h *Handler) listLedger(w http.ResponseWriter, r *http.Request) {
	httpx.OK(w, map[string]any{"ledger": h.store.Ledger(queryInt64(r, "accountId"))})
}

func (h *Handler) listWithdrawals(w http.ResponseWriter, r *http.Request) {
	httpx.OK(w, map[string]any{"withdrawals": h.store.ListWithdrawals(r.URL.Query().Get("status"))})
}

func (h *Handler) reviewWithdrawal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID      int64  `json:"id"`
		Approve bool   `json:"approve"`
		Reason  string `json:"reason"`
		AdminID string `json:"adminId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	row, err := h.store.ReviewWithdrawal(body.ID, body.Approve, adminID(body.AdminID), body.Reason)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, row)
}

func (h *Handler) listPayments(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListPaymentOrdersAdmin(store.AdminListFilter{
		AccountID: queryInt64(r, "accountId"),
		Status:    r.URL.Query().Get("status"),
		Limit:     queryInt(r, "limit", 50),
		Offset:    queryInt(r, "offset", 0),
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"payments": rows})
}

func (h *Handler) listNFTRequests(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListNFTMintRequests(store.AdminListFilter{
		AccountID: queryInt64(r, "accountId"),
		Status:    r.URL.Query().Get("status"),
		Limit:     queryInt(r, "limit", 50),
		Offset:    queryInt(r, "offset", 0),
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"requests": rows})
}

func (h *Handler) confirmNFTMint(w http.ResponseWriter, r *http.Request) {
	var body struct {
		OpID        string `json:"opId"`
		RequestID   int64  `json:"requestId"`
		MintAddress string `json:"mintAddress"`
		TxSignature string `json:"txSignature"`
		MetadataURI string `json:"metadataUri"`
		AdminID     string `json:"adminId"`
		Reason      string `json:"reason"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if strings.TrimSpace(body.OpID) == "" {
		body.OpID = fmt.Sprintf("admin-nft-confirm-%d-%d", body.RequestID, time.Now().UnixNano())
	}
	result, err := h.store.ConfirmNFTMint(store.NFTMintConfirmInput{
		OpID:        body.OpID,
		RequestID:   body.RequestID,
		MintAddress: body.MintAddress,
		TxSignature: body.TxSignature,
		MetadataURI: body.MetadataURI,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	aid := adminID(body.AdminID)
	audit := h.store.AuditTarget(aid, "nft_mint_confirm", "nft_mint_request", fmt.Sprint(body.RequestID), body.Reason)
	httpx.OK(w, map[string]any{"result": result, "audit": audit})
}

func (h *Handler) getHotWallet(w http.ResponseWriter, r *http.Request) {
	wallet := strings.TrimSpace(r.URL.Query().Get("wallet"))
	if wallet == "" {
		wallet = h.cfg.SolanaPayoutWallet
	}
	row, err := h.store.GetHotWalletStatus(wallet)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httpx.OK(w, map[string]any{
				"wallet":        wallet,
				"network":       h.cfg.SolanaNetwork,
				"payoutsPaused": false,
				"exists":        false,
			})
			return
		}
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"status": row, "exists": true})
}

func (h *Handler) pauseHotWallet(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Wallet  string `json:"wallet"`
		Paused  bool   `json:"paused"`
		Reason  string `json:"reason"`
		AdminID string `json:"adminId"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	wallet := strings.TrimSpace(body.Wallet)
	if wallet == "" {
		wallet = h.cfg.SolanaPayoutWallet
	}
	row, err := h.store.SetHotWalletPayoutsPaused(wallet, h.cfg.SolanaNetwork, h.cfg.SolanaTokenMint, body.Paused)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	aid := adminID(body.AdminID)
	action := "hot_wallet_resume"
	if body.Paused {
		action = "hot_wallet_pause"
	}
	audit := h.store.AuditTarget(aid, action, "hot_wallet", wallet, body.Reason)
	httpx.OK(w, map[string]any{"status": row, "audit": audit})
}
