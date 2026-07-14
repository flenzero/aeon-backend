package account

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/security"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func TestHomeRecoveryEndpointRequiresReconnectToOriginServer(t *testing.T) {
	st := store.New()
	accountRow := st.UpsertAccountByWallet("home_resume_wallet")
	character, err := st.CreateCharacter(accountRow.ID, "Home Resume Hero")
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.DungeonEnter(store.DungeonEnterRequest{
		OpID: "home-resume-enter", AccountID: accountRow.ID, CharacterID: character.ID,
		ServerID: "world-a", ChapterID: 0, FloorID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	token, err := security.SignAccessToken("test-jwt", accountRow.ID, accountRow.Username, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{ServiceName: "account-api-test", JWTSecret: "test-jwt", InternalKey: "test-internal", Profile: config.ProfileTest}
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/game/dungeon/recovery?characterId=%d", character.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	NewHandler(cfg, st).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		OK   bool                     `json:"ok"`
		Data store.DungeonRunRecovery `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK || !response.Data.Required || response.Data.ServerID != "world-a" || response.Data.DungeonRunID != run.DungeonRunID {
		t.Fatalf("response=%+v", response)
	}
}

func TestRecoveryDecisionEndpointCanAbandonWithoutRewardSettlement(t *testing.T) {
	st := store.New()
	accountRow := st.UpsertAccountByWallet("home_abandon_wallet")
	character, err := st.CreateCharacter(accountRow.ID, "Home Abandon Hero")
	if err != nil {
		t.Fatal(err)
	}
	_, err = st.CreateAccountSession(store.CreateSessionRequest{
		SessionID: "home-abandon-session", AccountID: accountRow.ID,
		RefreshToken: "home-abandon-refresh", ExpiresAt: time.Now().UTC().Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	run, err := st.DungeonEnter(store.DungeonEnterRequest{
		OpID: "home-abandon-enter", AccountID: accountRow.ID, CharacterID: character.ID,
		ServerID: "world-a", ChapterID: 0, FloorID: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	token, err := security.SignAccessToken("test-jwt", accountRow.ID, accountRow.Username, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{ServiceName: "account-api-test", JWTSecret: "test-jwt", InternalKey: "test-internal", Profile: config.ProfileTest}
	body := fmt.Sprintf(`{"characterId":%d,"dungeonRunId":%q,"action":"abandon","sessionId":"home-abandon-session"}`, character.ID, run.DungeonRunID)
	req := httptest.NewRequest(http.MethodPost, "/api/game/dungeon/recovery", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	NewHandler(cfg, st).Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if active, err := st.ActiveDungeonRun(accountRow.ID, character.ID); err == nil {
		t.Fatalf("run remains active: %+v", active)
	}
}
