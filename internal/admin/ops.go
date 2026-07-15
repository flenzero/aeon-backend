package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/httpx"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

const serverHeartbeatWindow = 30 * time.Second

type serverOperation struct {
	OpID   string `json:"opId"`
	Reason string `json:"reason"`
}

type opsServerView struct {
	ServerID       string     `json:"serverId"`
	Name           string     `json:"name"`
	Host           string     `json:"host"`
	Port           int        `json:"port"`
	MaxPlayers     int        `json:"maxPlayers"`
	CurPlayers     int        `json:"curPlayers"`
	CapacityLimit  int        `json:"capacityLimit"`
	HasSlot        bool       `json:"hasSlot"`
	Status         string     `json:"status"`
	PublicStatus   string     `json:"publicStatus"`
	Live           bool       `json:"live"`
	Region         string     `json:"region,omitempty"`
	PublicEndpoint string     `json:"publicEndpoint,omitempty"`
	LastPing       *time.Time `json:"lastPing,omitempty"`
}

func validServerStatus(value string) (string, error) {
	status := strings.ToUpper(strings.TrimSpace(value))
	switch status {
	case "STARTING", "ONLINE", "DRAINING", "OFFLINE", "MAINTENANCE", "DISABLED":
		return status, nil
	default:
		return "", errors.New("status must be STARTING, ONLINE, DRAINING, OFFLINE, MAINTENANCE, or DISABLED")
	}
}

func serverView(server store.GameServer, now time.Time) opsServerView {
	live := server.LastHeartbeatAt != nil && !server.LastHeartbeatAt.Before(now.Add(-serverHeartbeatWindow))
	publicStatus := "offline"
	if server.Status == "ONLINE" && live {
		if server.MaxPlayers > 0 && server.OnlinePlayers >= opsServerFullThreshold(server.MaxPlayers) {
			publicStatus = "full"
		} else {
			publicStatus = "online"
		}
	}
	return opsServerView{
		ServerID: server.ServerID, Name: server.DisplayName, Host: server.Host, Port: server.Port,
		MaxPlayers: server.MaxPlayers, CurPlayers: server.OnlinePlayers, CapacityLimit: server.MaxPlayers,
		HasSlot: publicStatus == "online", Status: strings.ToLower(server.Status), PublicStatus: publicStatus,
		Live: live, Region: server.Region, PublicEndpoint: server.PublicEndpoint, LastPing: server.LastHeartbeatAt,
	}
}

func opsServerFullThreshold(maxPlayers int) int {
	if maxPlayers <= 0 {
		return 0
	}
	return (maxPlayers*95 + 99) / 100
}

func (h *Handler) findServer(serverID string) (store.GameServer, error) {
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return store.GameServer{}, errors.New("serverId is required")
	}
	rows, err := h.store.ListGameServers("")
	if err != nil {
		return store.GameServer{}, err
	}
	for _, row := range rows {
		if row.ServerID == serverID {
			return row, nil
		}
	}
	return store.GameServer{}, store.ErrNotFound
}

func requireOperation(w http.ResponseWriter, op serverOperation) bool {
	if strings.TrimSpace(op.OpID) == "" {
		httpx.Error(w, http.StatusBadRequest, 400, "opId is required")
		return false
	}
	if strings.TrimSpace(op.Reason) == "" {
		httpx.Error(w, http.StatusBadRequest, 400, "reason is required")
		return false
	}
	return true
}

// replayOperation returns true when it has already written the saved response.
func (h *Handler) replayOperation(w http.ResponseWriter, r *http.Request, opID, action string) bool {
	previous, err := h.store.AdminOperation(opID)
	if errors.Is(err, store.ErrNotFound) {
		return false
	}
	if err != nil {
		writeStoreErr(w, err)
		return true
	}
	actor := authenticatedAdminID(r)
	if previous.AdminID != actor || previous.Action != action {
		httpx.Error(w, http.StatusBadRequest, 4001, "opId is already bound to a different admin action")
		return true
	}
	httpx.OK(w, previous.Response)
	return true
}

func (h *Handler) completeOperation(w http.ResponseWriter, r *http.Request, opID, action, target string, data any) {
	raw, err := json.Marshal(data)
	if err != nil {
		httpx.Error(w, http.StatusInternalServerError, 500, "cannot encode admin operation result")
		return
	}
	row, err := h.store.SaveAdminOperation(store.AdminOperation{
		OpID: opID, AdminID: authenticatedAdminID(r), Action: action, Target: target, Response: raw,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, row.Response)
}

func (h *Handler) listOpsServers(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" {
		var err error
		status, err = validServerStatus(status)
		if err != nil {
			httpx.Error(w, http.StatusBadRequest, 400, err.Error())
			return
		}
	}
	rows, err := h.store.ListGameServers(status)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	region := strings.TrimSpace(r.URL.Query().Get("region"))
	items := make([]opsServerView, 0, len(rows))
	now := time.Now().UTC()
	for _, row := range rows {
		if region != "" && !strings.EqualFold(row.Region, region) {
			continue
		}
		items = append(items, serverView(row, now))
	}
	httpx.OK(w, map[string]any{"items": items})
}

func (h *Handler) listOnlineOpsServers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.ListGameServers("ONLINE")
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	region := strings.TrimSpace(r.URL.Query().Get("region"))
	now := time.Now().UTC()
	items := make([]opsServerView, 0, len(rows))
	for _, row := range rows {
		view := serverView(row, now)
		if view.Live && (region == "" || strings.EqualFold(row.Region, region)) {
			items = append(items, view)
		}
	}
	httpx.OK(w, map[string]any{"items": items})
}

func (h *Handler) opsServerDetail(w http.ResponseWriter, r *http.Request) {
	server, err := h.findServer(r.PathValue("serverId"))
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	online, err := h.store.ListOnlineByServer(server.ServerID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	httpx.OK(w, map[string]any{"server": serverView(server, time.Now().UTC()), "onlinePlayers": online})
}

func (h *Handler) upsertOpsServer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		serverOperation
		Host           string `json:"host"`
		Port           int    `json:"port"`
		MaxPlayers     int    `json:"maxPlayers"`
		Status         string `json:"status"`
		Region         string `json:"region"`
		Name           string `json:"name"`
		PublicEndpoint string `json:"publicEndpoint"`
		MarkOnline     bool   `json:"markOnline"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if !requireOperation(w, body.serverOperation) {
		return
	}
	if h.replayOperation(w, r, body.OpID, "ops_server_upsert") {
		return
	}
	serverID := strings.TrimSpace(r.PathValue("serverId"))
	if serverID == "" || strings.TrimSpace(body.Host) == "" || body.Port < 1 || body.Port > 65535 || body.MaxPlayers <= 0 {
		httpx.Error(w, http.StatusBadRequest, 400, "serverId, host, port (1-65535), and positive maxPlayers are required")
		return
	}
	status := body.Status
	if body.MarkOnline {
		status = "ONLINE"
	}
	if status == "" {
		status = "MAINTENANCE"
	}
	status, err := validServerStatus(status)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	_, findErr := h.findServer(serverID)
	created := errors.Is(findErr, store.ErrNotFound)
	if findErr != nil && !created {
		writeStoreErr(w, findErr)
		return
	}
	server, err := h.store.UpsertGameServer(store.GameServer{
		ServerID: serverID, DisplayName: strings.TrimSpace(body.Name), Region: strings.TrimSpace(body.Region),
		Host: strings.TrimSpace(body.Host), Port: body.Port, PublicEndpoint: strings.TrimSpace(body.PublicEndpoint),
		MaxPlayers: body.MaxPlayers, Status: status,
	})
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	audit := h.store.AuditTarget(authenticatedAdminID(r), "ops_server_upsert", "game_server", serverID, body.Reason+" [opId="+body.OpID+"]")
	h.completeOperation(w, r, body.OpID, "ops_server_upsert", "game_server:"+serverID, map[string]any{"server": serverView(server, time.Now().UTC()), "created": created, "audit": audit})
}

func (h *Handler) setOpsServerStatus(w http.ResponseWriter, r *http.Request) {
	var body struct {
		serverOperation
		Status string `json:"status"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if !requireOperation(w, body.serverOperation) {
		return
	}
	if h.replayOperation(w, r, body.OpID, "ops_server_set_status") {
		return
	}
	status, err := validServerStatus(body.Status)
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, 400, err.Error())
		return
	}
	serverID := r.PathValue("serverId")
	server, err := h.findServer(serverID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	server.Status = status
	server, err = h.store.UpsertGameServer(server)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	audit := h.store.AuditTarget(authenticatedAdminID(r), "ops_server_set_status", "game_server", server.ServerID, body.Reason+" [opId="+body.OpID+"]")
	h.completeOperation(w, r, body.OpID, "ops_server_set_status", "game_server:"+server.ServerID, map[string]any{"server": serverView(server, time.Now().UTC()), "audit": audit})
}

func (h *Handler) listOpsOnlinePlayers(w http.ResponseWriter, r *http.Request) {
	serverID := strings.TrimSpace(r.URL.Query().Get("serverId"))
	var rows []store.OnlineSession
	var err error
	if serverID != "" {
		rows, err = h.store.ListOnlineByServer(serverID)
	} else {
		servers, listErr := h.store.ListGameServers("")
		if listErr != nil {
			writeStoreErr(w, listErr)
			return
		}
		for _, server := range servers {
			items, listErr := h.store.ListOnlineByServer(server.ServerID)
			if listErr != nil {
				writeStoreErr(w, listErr)
				return
			}
			rows = append(rows, items...)
		}
	}
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].LastSeenAt.After(rows[j].LastSeenAt) })
	httpx.OK(w, map[string]any{"items": rows})
}

func (h *Handler) kickOpsOnlinePlayer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		serverOperation
		RevokeSession bool `json:"revokeSession"`
	}
	if !httpx.Decode(r, &body) {
		httpx.Error(w, http.StatusBadRequest, 400, "invalid JSON body")
		return
	}
	if !requireOperation(w, body.serverOperation) {
		return
	}
	if h.replayOperation(w, r, body.OpID, "ops_online_player_kick") {
		return
	}
	accountID, err := strconv.ParseInt(r.PathValue("accountId"), 10, 64)
	if err != nil || accountID <= 0 {
		httpx.Error(w, http.StatusBadRequest, 400, "accountId is invalid")
		return
	}
	previous, err := h.store.GetOnlineSession(accountID)
	if err != nil {
		writeStoreErr(w, err)
		return
	}
	if _, err := h.store.LeaveOnlineSession(accountID, previous.ConnectionID); err != nil {
		writeStoreErr(w, err)
		return
	}
	if body.RevokeSession {
		if err := h.store.RevokeAccountSession(previous.SessionID, time.Now().UTC()); err != nil {
			writeStoreErr(w, err)
			return
		}
	}
	audit := h.store.AuditTarget(authenticatedAdminID(r), "ops_online_player_kick", "account", fmt.Sprint(accountID), body.Reason+" [opId="+body.OpID+"]")
	h.completeOperation(w, r, body.OpID, "ops_online_player_kick", "account:"+fmt.Sprint(accountID), map[string]any{
		"accountId": accountID, "clearedOnline": true, "revokedSession": body.RevokeSession, "previousOnline": previous, "audit": audit,
	})
}
