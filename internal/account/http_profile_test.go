package account

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/flenzero/aeon-backend/internal/platform/config"
	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func TestPlayerProfileReturnsExpToNextLevelAndDungeonProgress(t *testing.T) {
	st := store.New()
	accountRow := st.UpsertAccountByWallet("profile-exp-wallet")
	character, err := st.CreateCharacter(accountRow.ID, "ProfileHero")
	if err != nil {
		t.Fatalf("create character: %v", err)
	}
	enter, err := st.DungeonEnter(store.DungeonEnterRequest{
		OpID:        "profile-progress-enter",
		AccountID:   accountRow.ID,
		CharacterID: character.ID,
		ChapterID:   0,
		FloorID:     1,
	})
	if err != nil {
		t.Fatalf("dungeon enter: %v", err)
	}
	if _, err := st.DungeonFinish(store.DungeonFinishRequest{
		OpID:         "profile-progress-finish",
		AccountID:    accountRow.ID,
		CharacterID:  character.ID,
		DungeonRunID: enter.DungeonRunID,
		ChapterID:    0,
		FloorID:      1,
		Result:       "victory",
	}); err != nil {
		t.Fatalf("dungeon finish: %v", err)
	}
	cfg := config.Config{
		ServiceName:      "account-api-test",
		JWTSecret:        "test-jwt",
		InternalKey:      "test-internal",
		Profile:          config.ProfileTest,
		EconomyConfigDir: "../../configs/economy",
	}
	handler := NewHandler(cfg, st).Routes()
	req := httptest.NewRequest(http.MethodGet, "/api/player/profile?characterId="+strconv.FormatInt(character.ID, 10), nil)
	req.Header.Set("X-Internal-Key", cfg.InternalKey)
	req.Header.Set("X-Account-Id", strconv.FormatInt(accountRow.ID, 10))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		OK   bool `json:"ok"`
		Data struct {
			Player  store.PlayerState     `json:"player"`
			Economy store.EconomySnapshot `json:"economy"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.OK {
		t.Fatalf("response ok=false: %+v", response)
	}
	if response.Data.Economy.ExpToNextLevel != 83 || response.Data.Player.ExpToNextLevel != 83 {
		t.Fatalf("expToNextLevel economy=%d player=%d, want 83", response.Data.Economy.ExpToNextLevel, response.Data.Player.ExpToNextLevel)
	}
	if response.Data.Economy.HighestClearedChapterID != 0 || response.Data.Economy.HighestClearedFloorID != 1 {
		t.Fatalf("highest cleared economy chapter=%d floor=%d, want chapter=0 floor=1", response.Data.Economy.HighestClearedChapterID, response.Data.Economy.HighestClearedFloorID)
	}
}
