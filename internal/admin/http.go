package admin

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/economy/rules"
	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/readiness"
	"github.com/flenzero/aeon-backend/internal/platform/security"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

type Handler struct {
	cfg      config.Config
	store    store.Repository
	ready    readiness.Probe
	rules    *rules.Config
	rulesErr error
}

func NewHandler(cfg config.Config, st store.Repository) *Handler {
	economyRules, err := rules.LoadDir(cfg.EconomyConfigDir)
	return &Handler{cfg: cfg, store: st, ready: readiness.New(cfg.ServiceName, readiness.PersistenceChecks(cfg, st)...), rules: economyRules, rulesErr: err}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", httpx.Health(h.cfg.ServiceName))
	mux.Handle("GET /ready", h.ready.Handler())

	mux.HandleFunc("GET /api/admin/auth/nonce", h.adminLoginNonce)
	mux.HandleFunc("POST /api/admin/auth/login", h.adminLogin)
	mux.HandleFunc("POST /api/admin/admin-users", httpx.RequireSuperAdmin(h.cfg, h.store, h.createAdminUser))
	mux.HandleFunc("GET /api/admin/admin-users", httpx.RequireSuperAdmin(h.cfg, h.store, h.listAdminUsers))
	mux.HandleFunc("DELETE /api/admin/admin-users/{adminId}", httpx.RequireSuperAdmin(h.cfg, h.store, h.disableAdminUser))

	mux.HandleFunc("GET /api/admin/accounts/selector", httpx.RequireAdmin(h.cfg, h.store, h.listAccountSelector))
	mux.HandleFunc("GET /api/admin/accounts", httpx.RequireAdmin(h.cfg, h.store, h.getAccount))
	mux.HandleFunc("POST /api/admin/accounts/ban", httpx.RequireAdmin(h.cfg, h.store, h.banAccount))
	mux.HandleFunc("POST /api/admin/accounts/risk-level", httpx.RequireAdmin(h.cfg, h.store, h.setRiskLevel))
	mux.HandleFunc("POST /api/admin/accounts/license", httpx.RequireSuperAdmin(h.cfg, h.store, h.setLicense))
	mux.HandleFunc("POST /api/admin/accounts/sessions/revoke", httpx.RequireAdmin(h.cfg, h.store, h.revokeSessions))
	mux.HandleFunc("GET /api/admin/characters", httpx.RequireAdmin(h.cfg, h.store, h.listCharacters))
	mux.HandleFunc("GET /api/admin/characters/{characterId}", httpx.RequireAdmin(h.cfg, h.store, h.getCharacter))
	mux.HandleFunc("GET /api/admin/characters/{characterId}/ledger", httpx.RequireAdmin(h.cfg, h.store, h.listCharacterLedger))
	mux.HandleFunc("GET /api/admin/characters/{characterId}/audits", httpx.RequireAdmin(h.cfg, h.store, h.listCharacterAudits))
	mux.HandleFunc("GET /api/admin/characters/{characterId}/timeline", httpx.RequireAdmin(h.cfg, h.store, h.listCharacterTimeline))
	mux.HandleFunc("GET /api/admin/equipment/{equipmentUid}", httpx.RequireAdmin(h.cfg, h.store, h.getEquipment))
	mux.HandleFunc("GET /api/admin/catalog/items", httpx.RequireAdmin(h.cfg, h.store, h.listCatalogItems))

	mux.HandleFunc("GET /api/admin/announcements", httpx.RequireAdmin(h.cfg, h.store, h.listAnnouncements))
	mux.HandleFunc("GET /api/admin/announcements/templates", httpx.RequireAdmin(h.cfg, h.store, h.listAnnouncementTemplates))
	mux.HandleFunc("PUT /api/admin/announcements/templates/{code}", httpx.RequireAdmin(h.cfg, h.store, h.upsertAnnouncementTemplate))
	mux.HandleFunc("POST /api/admin/announcements/notices", httpx.RequireAdmin(h.cfg, h.store, h.createOpsAnnouncement))
	mux.HandleFunc("PUT /api/admin/announcements/notices/{announcementId}", httpx.RequireAdmin(h.cfg, h.store, h.updateOpsAnnouncement))
	mux.HandleFunc("POST /api/admin/announcements/{announcementId}/revoke", httpx.RequireAdmin(h.cfg, h.store, h.revokeAnnouncement))

	mux.HandleFunc("GET /api/admin/market/restrictions", httpx.RequireAdmin(h.cfg, h.store, h.listMarketRestrictions))
	mux.HandleFunc("POST /api/admin/market/restrictions", httpx.RequireAdmin(h.cfg, h.store, h.createMarketRestriction))
	mux.HandleFunc("POST /api/admin/market/restrictions/revoke", httpx.RequireAdmin(h.cfg, h.store, h.revokeMarketRestriction))

	mux.HandleFunc("GET /api/admin/risk/events", httpx.RequireAdmin(h.cfg, h.store, h.listRiskEvents))
	mux.HandleFunc("POST /api/admin/risk/events", httpx.RequireAdmin(h.cfg, h.store, h.createRiskEvent))

	mux.HandleFunc("GET /api/admin/audits", httpx.RequireAdmin(h.cfg, h.store, h.listAudits))
	mux.HandleFunc("GET /api/admin/ledger", httpx.RequireAdmin(h.cfg, h.store, h.listLedger))

	mux.HandleFunc("GET /api/admin/withdrawals", httpx.RequireAdmin(h.cfg, h.store, h.listWithdrawals))
	mux.HandleFunc("POST /api/admin/withdrawals/review", httpx.RequireSuperAdmin(h.cfg, h.store, h.reviewWithdrawal))

	mux.HandleFunc("GET /api/admin/payments", httpx.RequireAdmin(h.cfg, h.store, h.listPayments))
	mux.HandleFunc("GET /api/admin/nft/requests", httpx.RequireAdmin(h.cfg, h.store, h.listNFTRequests))
	mux.HandleFunc("POST /api/admin/nft/mint/confirm", httpx.RequireSuperAdmin(h.cfg, h.store, h.confirmNFTMint))

	mux.HandleFunc("GET /api/admin/hot-wallet", httpx.RequireAdmin(h.cfg, h.store, h.getHotWallet))
	mux.HandleFunc("POST /api/admin/hot-wallet/pause", httpx.RequireSuperAdmin(h.cfg, h.store, h.pauseHotWallet))
	mux.HandleFunc("POST /api/admin/service-identities", httpx.RequireSuperAdmin(h.cfg, h.store, h.createServiceIdentity))
	mux.HandleFunc("GET /api/admin/service-identities", httpx.RequireAdmin(h.cfg, h.store, h.listServiceIdentities))
	mux.HandleFunc("DELETE /api/admin/service-identities/{serviceId}", httpx.RequireSuperAdmin(h.cfg, h.store, h.disableServiceIdentity))

	// Super-admin-only operations are intentionally isolated from ordinary
	// support/risk controls. Every mutation requires an opId and a reason.
	mux.HandleFunc("GET /api/admin/ops/servers", httpx.RequireSuperAdmin(h.cfg, h.store, h.listOpsServers))
	mux.HandleFunc("GET /api/admin/ops/servers/online", httpx.RequireSuperAdmin(h.cfg, h.store, h.listOnlineOpsServers))
	mux.HandleFunc("GET /api/admin/ops/servers/online-players", httpx.RequireSuperAdmin(h.cfg, h.store, h.listOpsOnlinePlayers))
	mux.HandleFunc("GET /api/admin/ops/servers/{serverId}", httpx.RequireSuperAdmin(h.cfg, h.store, h.opsServerDetail))
	mux.HandleFunc("PUT /api/admin/ops/servers/{serverId}", httpx.RequireSuperAdmin(h.cfg, h.store, h.upsertOpsServer))
	mux.HandleFunc("POST /api/admin/ops/servers/{serverId}/status", httpx.RequireSuperAdmin(h.cfg, h.store, h.setOpsServerStatus))
	mux.HandleFunc("POST /api/admin/ops/servers/online-players/{accountId}/kick", httpx.RequireSuperAdmin(h.cfg, h.store, h.kickOpsOnlinePlayer))
	mux.HandleFunc("POST /api/admin/ops/characters/{characterId}/grants/rewards", httpx.RequireSuperAdmin(h.cfg, h.store, h.grantOpsRewards))
	mux.HandleFunc("POST /api/admin/ops/characters/{characterId}/lottery/draw", httpx.RequireSuperAdmin(h.cfg, h.store, h.drawOpsLottery))
	mux.HandleFunc("POST /api/admin/ops/characters/{characterId}/lottery/commit-preview", httpx.RequireSuperAdmin(h.cfg, h.store, h.commitOpsLotteryPreview))
	mux.HandleFunc("POST /api/admin/ops/compensation/preview", httpx.RequireSuperAdmin(h.cfg, h.store, h.previewOpsCompensation))
	mux.HandleFunc("GET /api/admin/ops/compensation/previews/{previewId}", httpx.RequireSuperAdmin(h.cfg, h.store, h.getOpsCompensationPreview))
	mux.HandleFunc("GET /api/admin/ops/compensation/previews/{previewId}/targets", httpx.RequireSuperAdmin(h.cfg, h.store, h.listOpsCompensationPreviewTargets))
	mux.HandleFunc("POST /api/admin/ops/compensation/commit", httpx.RequireSuperAdmin(h.cfg, h.store, h.commitOpsCompensation))
	mux.HandleFunc("GET /api/admin/ops/payments/economy-orders/{orderId}/trace", httpx.RequireSuperAdmin(h.cfg, h.store, h.traceOpsPayment))
	mux.HandleFunc("POST /api/admin/ops/payments/economy-orders/{orderId}/recover", httpx.RequireSuperAdmin(h.cfg, h.store, h.recoverOpsPayment))

	return httpx.Recover(httpx.WithCORS(mux))
}

func (h *Handler) adminLoginNonce(w http.ResponseWriter, r *http.Request) {
	adminID := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("adminId")))
	if adminID == "" {
		httpx.Error(w, http.StatusBadRequest, 400, "adminId is required")
		return
	}
	admin, err := h.store.AdminUser(adminID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if admin.Status != store.AdminUserActive {
		httpx.Error(w, http.StatusUnauthorized, 401, "admin user is disabled")
		return
	}
	nonce := security.RandomToken("admin_nonce")
	expiresAt := time.Now().UTC().Add(5 * time.Minute)
	message := security.AdminLoginMessage(admin.AdminID, nonce)
	h.store.SaveAdminLoginNonce(store.AdminLoginNonce{
		Nonce: nonce, AdminID: admin.AdminID, Message: message,
		Status: "PENDING", ExpiresAt: expiresAt, CreatedAt: time.Now().UTC(),
	})
	httpx.OK(w, map[string]any{"adminId": admin.AdminID, "nonce": nonce, "message": message, "expiresAt": expiresAt})
}

func (h *Handler) adminLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AdminID   string `json:"adminId"`
		Nonce     string `json:"nonce"`
		Signature string `json:"signature"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	adminID := strings.ToLower(strings.TrimSpace(body.AdminID))
	if adminID == "" || strings.TrimSpace(body.Nonce) == "" || strings.TrimSpace(body.Signature) == "" {
		httpx.Error(w, http.StatusBadRequest, 400, "adminId, nonce, and signature are required")
		return
	}
	admin, err := h.store.AdminUser(adminID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if admin.Status != store.AdminUserActive {
		httpx.Error(w, http.StatusUnauthorized, 401, "admin user is disabled")
		return
	}
	if admin.Role == "SUPER_ADMIN" {
		httpx.Error(w, http.StatusForbidden, 403, "super admin must use the super-admin operations key")
		return
	}
	now := time.Now().UTC()
	nonce, err := h.store.AdminLoginNonce(body.Nonce, admin.AdminID, now)
	if err != nil {
		httpx.Error(w, http.StatusUnauthorized, 401, "admin login nonce is invalid or expired")
		return
	}
	if err := security.VerifyEd25519Signature(admin.PublicKey, nonce.Message, body.Signature); err != nil {
		httpx.Error(w, http.StatusUnauthorized, 401, err.Error())
		return
	}
	if err := h.store.ConsumeAdminLoginNonce(nonce.Nonce, admin.AdminID, now); err != nil {
		httpx.Error(w, http.StatusUnauthorized, 401, "admin login nonce is invalid or already consumed")
		return
	}
	admin, _ = h.store.TouchAdminUserLogin(admin.AdminID, now)
	token, expiresAt, err := security.SignAdminAccessToken(h.cfg.JWTSecret, admin.AdminID, admin.Username, admin.Role, h.cfg.AdminSessionTTL())
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, 500, err.Error())
		return
	}
	httpx.OK(w, map[string]any{"admin": admin, "accessToken": token, "expiresAt": expiresAt})
}

func (h *Handler) createAdminUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		AdminID   string `json:"adminId"`
		Username  string `json:"username"`
		PublicKey string `json:"publicKey"`
		Role      string `json:"role"`
		Reason    string `json:"reason"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	actor, _ := httpx.ContextAdminActor(r)
	row, err := h.store.CreateAdminUser(store.CreateAdminUserInput{
		AdminID: body.AdminID, Username: body.Username, PublicKey: body.PublicKey,
		Role: body.Role, CreatedBy: actor.ID, Reason: body.Reason,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.Created(w, row)
}

func (h *Handler) listAdminUsers(w http.ResponseWriter, r *http.Request) {
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && status != store.AdminUserActive && status != store.AdminUserDisabled {
		httpx.Error(w, http.StatusBadRequest, 400, "status must be ACTIVE or DISABLED")
		return
	}
	limit, offset, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	rows, err := h.store.ListAdminUsers(status, limit, offset)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"items": rows, "count": len(rows)})
}

func (h *Handler) disableAdminUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reason string `json:"reason"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	actor, _ := httpx.ContextAdminActor(r)
	row, err := h.store.DisableAdminUser(r.PathValue("adminId"), actor.ID, body.Reason)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, row)
}

func (h *Handler) createServiceIdentity(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ServiceID    string   `json:"serviceId"`
		Name         string   `json:"name"`
		Kind         string   `json:"kind"`
		SubjectID    string   `json:"subjectId"`
		PublicKey    string   `json:"publicKey"`
		Capabilities []string `json:"capabilities"`
		Reason       string   `json:"reason"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	actor, _ := httpx.ContextAdminActor(r)
	row, err := h.store.CreateServiceIdentity(store.CreateServiceIdentityInput{
		ServiceID:    body.ServiceID,
		Name:         body.Name,
		Kind:         body.Kind,
		SubjectID:    body.SubjectID,
		PublicKey:    body.PublicKey,
		Capabilities: body.Capabilities,
		CreatedBy:    actor.ID,
		Reason:       body.Reason,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.Created(w, row)
}

func (h *Handler) disableServiceIdentity(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Reason string `json:"reason"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	actor, _ := httpx.ContextAdminActor(r)
	row, err := h.store.DisableServiceIdentity(r.PathValue("serviceId"), actor.ID, body.Reason)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, row)
}

func (h *Handler) listServiceIdentities(w http.ResponseWriter, r *http.Request) {
	status := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && status != store.ServiceIdentityActive && status != store.ServiceIdentityDisabled {
		httpx.Error(w, http.StatusBadRequest, 400, "status must be ACTIVE or DISABLED")
		return
	}
	limit, offset := 50, 0
	var err error
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 200 {
			httpx.Error(w, http.StatusBadRequest, 400, "limit must be between 1 and 200")
			return
		}
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			httpx.Error(w, http.StatusBadRequest, 400, "offset must be non-negative")
			return
		}
	}
	rows, err := h.store.ListServiceIdentities(status, limit, offset)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"items": rows, "count": len(rows)})
}

func authenticatedAdminID(r *http.Request) string {
	actor, ok := httpx.ContextAdminActor(r)
	if !ok || strings.TrimSpace(actor.ID) == "" {
		return "unknown-admin"
	}
	return strings.TrimSpace(actor.ID)
}

func queryInt64(r *http.Request, key string) int64 {
	v, _ := strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
	return v
}

func optionalBoolQuery(r *http.Request, key string) (bool, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return value, nil
}

func writeStoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		httpx.Error(w, http.StatusNotFound, 404, err.Error())
		return
	}
	if errors.Is(err, store.ErrForbidden) {
		httpx.Error(w, http.StatusForbidden, 403, err.Error())
		return
	}
	httpx.Error(w, http.StatusBadRequest, 4001, err.Error())
}

func (h *Handler) listAccountSelector(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	status, ok := validateAccountStatus(r.URL.Query().Get("status"))
	if !ok {
		httpx.Error(w, http.StatusBadRequest, 400, "status must be ACTIVE, BANNED, FROZEN, or DELETED")
		return
	}
	rows, err := h.store.ListAdminAccountSelector(store.AdminAccountSelectorFilter{
		Keyword: r.URL.Query().Get("keyword"),
		Status:  status,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"items": rows, "count": len(rows), "limit": limit, "offset": offset})
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
	aid := authenticatedAdminID(r)
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
	aid := authenticatedAdminID(r)
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
	aid := authenticatedAdminID(r)
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
	aid := authenticatedAdminID(r)
	audit := h.store.AuditTarget(aid, "account_sessions_revoke", "account", fmt.Sprint(body.AccountID), body.Reason)
	httpx.OK(w, map[string]any{"revoked": n, "audit": audit})
}

func (h *Handler) listMarketRestrictions(w http.ResponseWriter, r *http.Request) {
	accountID, _, err := httpx.OptionalPositiveInt64(r, "accountId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	limit, offset, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	activeOnly, err := optionalBoolQuery(r, "activeOnly")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	rows, err := h.store.ListMarketRestrictions(accountID, activeOnly, limit, offset)
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
	aid := authenticatedAdminID(r)
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
	aid := authenticatedAdminID(r)
	row, err := h.store.RevokeMarketRestriction(body.ID, aid, body.Reason)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	audit := h.store.AuditTarget(aid, "market_restriction_revoke", "market_restriction", fmt.Sprint(row.ID), body.Reason)
	httpx.OK(w, map[string]any{"restriction": row, "audit": audit})
}

func (h *Handler) listRiskEvents(w http.ResponseWriter, r *http.Request) {
	accountID, _, err := httpx.OptionalPositiveInt64(r, "accountId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	limit, offset, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	rows, err := h.store.ListRiskEvents(accountID, limit, offset)
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
	aid := authenticatedAdminID(r)
	audit := h.store.AuditTarget(aid, "risk_event_create", "risk_event", fmt.Sprint(row.ID), body.Reason)
	httpx.OK(w, map[string]any{"event": row, "audit": audit})
}

func (h *Handler) listAudits(w http.ResponseWriter, r *http.Request) {
	limit, offset, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	rows, err := h.store.ListAudits(limit, offset)
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
	row, err := h.store.ReviewWithdrawal(body.ID, body.Approve, authenticatedAdminID(r), body.Reason)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, row)
}

func (h *Handler) listPayments(w http.ResponseWriter, r *http.Request) {
	accountID, _, err := httpx.OptionalPositiveInt64(r, "accountId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	limit, offset, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	rows, err := h.store.ListPaymentOrdersAdmin(store.AdminListFilter{
		AccountID: accountID,
		Status:    r.URL.Query().Get("status"),
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"payments": rows})
}

func (h *Handler) listNFTRequests(w http.ResponseWriter, r *http.Request) {
	accountID, _, err := httpx.OptionalPositiveInt64(r, "accountId")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	limit, offset, err := httpx.Pagination(r, 50, 200)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	rows, err := h.store.ListNFTMintRequests(store.AdminListFilter{
		AccountID: accountID,
		Status:    r.URL.Query().Get("status"),
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"requests": rows})
}

func (h *Handler) confirmNFTMint(w http.ResponseWriter, r *http.Request) {
	if h.cfg.StubMode == config.StubDisabled {
		httpx.Error(w, http.StatusServiceUnavailable, 4002, "Metaplex Core mint verification adapter is not configured")
		return
	}
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
	aid := authenticatedAdminID(r)
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
	aid := authenticatedAdminID(r)
	action := "hot_wallet_resume"
	if body.Paused {
		action = "hot_wallet_pause"
	}
	audit := h.store.AuditTarget(aid, action, "hot_wallet", wallet, body.Reason)
	httpx.OK(w, map[string]any{"status": row, "audit": audit})
}
