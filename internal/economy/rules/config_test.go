package rules

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/flenzero/aeon-backend/internal/platform/store"
)

func TestLoadDirAndDungeonRewards(t *testing.T) {
	cfg, err := LoadDir(filepath.Join("..", "..", "..", "configs", "economy"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	floor, ok := cfg.DungeonFloor(0, 1)
	if !ok {
		t.Fatal("expected dungeon floor 0/1")
	}
	if floor.MaxExp != 30 {
		t.Fatalf("max exp = %d, want 30", floor.MaxExp)
	}
	req := store.DungeonFinishRequest{
		OpID:         "reward-op-1",
		AccountID:    1,
		CharacterID:  2,
		DungeonRunID: "run-1",
		ChapterID:    0,
		FloorID:      1,
		Result:       "victory",
		Exp:          20,
	}
	first, err := cfg.DungeonRewards(req)
	if err != nil {
		t.Fatalf("dungeon rewards: %v", err)
	}
	second, err := cfg.DungeonRewards(req)
	if err != nil {
		t.Fatalf("dungeon rewards replay: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("reward generation is not deterministic:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestDungeonRewardsRejectsExpAboveCap(t *testing.T) {
	cfg, err := LoadDir(filepath.Join("..", "..", "..", "configs", "economy"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	_, err = cfg.DungeonRewards(store.DungeonFinishRequest{
		OpID:         "reward-op-2",
		CharacterID:  2,
		DungeonRunID: "run-2",
		ChapterID:    0,
		FloorID:      1,
		Result:       "victory",
		Exp:          31,
	})
	if err == nil {
		t.Fatal("expected exp cap error")
	}
}

func TestGatheringAndFarmingRewardsUseConfiguredPools(t *testing.T) {
	cfg, err := LoadDir(filepath.Join("..", "..", "..", "configs", "economy"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	gathering, err := cfg.GatheringRewards(store.ActivitySettlementRequest{
		OpID:        "gather-op-1",
		CharacterID: 2,
		ActivityID:  "shadow_woods_iron_vein",
	})
	if err != nil {
		t.Fatalf("gathering rewards: %v", err)
	}
	if len(gathering.Items) == 0 && gathering.TokenReward == 0 {
		t.Fatalf("expected gathering rewards, got %+v", gathering)
	}
	farming, err := cfg.FarmingRewards(store.ActivitySettlementRequest{
		OpID:        "farm-op-1",
		CharacterID: 2,
		ActivityID:  "emberroot",
	})
	if err != nil {
		t.Fatalf("farming rewards: %v", err)
	}
	if len(farming.Items) == 0 {
		t.Fatalf("expected farming item rewards, got %+v", farming)
	}
}

func TestRecipePlanProducesCraftOutput(t *testing.T) {
	cfg, err := LoadDir(filepath.Join("..", "..", "..", "configs", "economy"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	plan, inputs, err := cfg.RecipePlan("compress_aeon_shard", "craft-op-1", 2, 1)
	if err != nil {
		t.Fatalf("recipe plan: %v", err)
	}
	if len(inputs) != 1 || inputs[0].ItemID != "gloomcap_spore" || inputs[0].Quantity != 3 {
		t.Fatalf("inputs = %+v", inputs)
	}
	if len(plan.Items) != 1 || plan.Items[0].ItemID != "aeon_shard" {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestBossRewardsIncludeParticipationAndTierPools(t *testing.T) {
	cfg, err := LoadDir(filepath.Join("..", "..", "..", "configs", "economy"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	low, err := cfg.BossRewards(store.BossSettleRequest{
		OpID:        "boss-op-low",
		CharacterID: 2,
		BossEventID: 10,
		BossKey:     "shadow_leviathan",
	}, 0)
	if err != nil {
		t.Fatalf("boss rewards low contribution: %v", err)
	}
	if len(low.Items) == 0 {
		t.Fatalf("expected participation rewards, got %+v", low)
	}
	high, err := cfg.BossRewards(store.BossSettleRequest{
		OpID:        "boss-op-high",
		CharacterID: 2,
		BossEventID: 10,
		BossKey:     "shadow_leviathan",
	}, 6000)
	if err != nil {
		t.Fatalf("boss rewards high contribution: %v", err)
	}
	if len(high.Items) <= len(low.Items) && high.TokenReward <= low.TokenReward {
		t.Fatalf("expected higher tier rewards to expand plan:\nlow=%+v\nhigh=%+v", low, high)
	}
}
