package rules

import (
	"math/rand/v2"
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
	if len(cfg.Floors) != 30 {
		t.Fatalf("floors = %d, want 30", len(cfg.Floors))
	}
	for floorID := 1; floorID <= 30; floorID++ {
		chapterID := (floorID - 1) / 10
		floor, ok := cfg.DungeonFloor(chapterID, floorID)
		if !ok {
			t.Fatalf("expected dungeon floor %d/%d", chapterID, floorID)
		}
		wantMaxExp := int64(20 + floorID*10)
		if floor.IsBoss {
			wantMaxExp = int64(float64(wantMaxExp) * 1.5)
		}
		if floor.MaxExp != wantMaxExp {
			t.Fatalf("floor %d maxExp = %d, want %d", floorID, floor.MaxExp, wantMaxExp)
		}
		if floor.EnemyHpScale <= 0 || floor.EnemyAtkScale <= 0 {
			t.Fatalf("floor %d missing combat scales: %+v", floorID, floor)
		}
	}
	floor, ok := cfg.DungeonFloor(0, 1)
	if !ok {
		t.Fatal("expected dungeon floor 0/1")
	}
	if floor.MaxExp != 30 || floor.LootPoolID != "dungeon_ch0_f1_3" {
		t.Fatalf("floor 1 = %+v", floor)
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

func TestLoadDirBuildsLiveEquipmentAndOperationsConfiguration(t *testing.T) {
	cfg, err := LoadDir(filepath.Join("..", "..", "..", "configs", "economy"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if len(cfg.Equipment.ByItemID) != 70 {
		t.Fatalf("equipment templates = %d, want 70", len(cfg.Equipment.ByItemID))
	}
	if cfg.Shops == nil || len(cfg.Shops) != 0 {
		t.Fatalf("default shops should be configured but empty: %+v", cfg.Shops)
	}
	axe, ok := cfg.EquipmentTemplate("ashbound_axe_t1")
	if !ok {
		t.Fatal("ashbound axe template missing")
	}
	if axe.BaseFlat["attack"] != 12 || axe.BasePercent["attackSpeed"] != -0.08 {
		t.Fatalf("axe template=%+v", axe)
	}
	purple, ok := cfg.EquipmentRarity(4)
	if !ok || purple.AffixCount != 4 || purple.Multiplier != 1.4 {
		t.Fatalf("purple rarity=%+v", purple)
	}
	if cfg.Lottery.PriceAEB != 30 || cfg.Lottery.MaxCount != 10 || cfg.Lottery.EquipmentRarityWeight[3] != 80 {
		t.Fatalf("lottery=%+v", cfg.Lottery)
	}
	if len(cfg.Bounties.Slots) != 5 || cfg.Bounties.Slots[1].GoldCost != 3000 || cfg.Bounties.Refresh.PremiumAEBPrice != 30 {
		t.Fatalf("bounties=%+v", cfg.Bounties)
	}
	bossFloor, ok := cfg.DungeonFloor(0, 10)
	if !ok || !bossFloor.IsBoss || len(bossFloor.EnterCost.Items) != 1 || bossFloor.EnterCost.Items[0].ItemID != "boss_ticket_ashen_threshold" {
		t.Fatalf("boss ticket cost=%+v", bossFloor)
	}
	if bossFloor.MaxExp != 180 {
		t.Fatalf("boss max exp = %d, want 180", bossFloor.MaxExp)
	}
	bossPool, ok := cfg.LootPools["dungeon_ch0_boss"]
	if !ok || len(bossPool.Entries) == 0 {
		t.Fatal("dungeon_ch0_boss loot pool missing")
	}
	foundStageGear := false
	for _, entry := range bossPool.Entries {
		if entry.RewardType == "equipment" && entry.ItemID == "gloomhide_sword_t5" {
			foundStageGear = true
			if entry.Rarity != 2 {
				t.Fatalf("boss gear rarity = %d, want 2", entry.Rarity)
			}
		}
	}
	if !foundStageGear {
		t.Fatal("expected gloomhide_sword_t5 in dungeon_ch0_boss")
	}
	finalBoss, ok := cfg.DungeonFloor(2, 30)
	if !ok || !finalBoss.IsBoss || finalBoss.EnterCost.Items[0].ItemID != "boss_ticket_voidscar" {
		t.Fatalf("final boss=%+v", finalBoss)
	}
	finalPool := cfg.LootPools["dungeon_ch2_boss"]
	foundT30 := false
	for _, entry := range finalPool.Entries {
		if entry.ItemID == "aeonblight_sword_t30" && entry.Rarity == 3 {
			foundT30 = true
		}
	}
	if !foundT30 {
		t.Fatal("expected aeonblight_sword_t30 rarity 3 in dungeon_ch2_boss")
	}
	gold, stoneID, stoneQuantity, err := cfg.EnhancementRule("ashbound_axe_t1", 3, 6)
	if err != nil || gold <= 0 || stoneID != "enhancement_stone_t1" || stoneQuantity <= 0 {
		t.Fatalf("enhancement rule gold=%d stone=%q quantity=%d err=%v", gold, stoneID, stoneQuantity, err)
	}
}

func TestNewEquipmentUsesSemanticAffixesAndLiveResolution(t *testing.T) {
	cfg, err := LoadDir(filepath.Join("..", "..", "..", "configs", "economy"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	affixes := cfg.rollEquipmentAffixes(rand.New(rand.NewPCG(11, 22)), "ashbound_axe_t1", 6)
	if len(affixes) != 6 {
		t.Fatalf("red affixes = %d, want 6", len(affixes))
	}
	counts := map[string]int{}
	for _, affix := range affixes {
		if affix.InstanceID == "" || affix.Stat != "" || affix.Value != 0 {
			t.Fatalf("new affix must be semantic only: %+v", affix)
		}
		counts[affix.AffixID]++
		if counts[affix.AffixID] > 2 {
			t.Fatalf("affix %q appeared more than twice: %+v", affix.AffixID, affixes)
		}
	}
	affixes[0].EnhanceHits = 2
	resolved, err := cfg.ResolveEquipment("ashbound_axe_t1", 6, affixes)
	if err != nil {
		t.Fatalf("resolve equipment: %v", err)
	}
	if resolved.BaseFlat["attack"] != 23.4 || resolved.BasePercent["attackSpeed"] != -0.08 {
		t.Fatalf("axe base stats = %+v", resolved)
	}
	if resolved.Affixes[0].Value <= 0 || resolved.Affixes[0].EnhanceHits != 2 {
		t.Fatalf("enhanced affix = %+v", resolved.Affixes[0])
	}
	mount, err := cfg.ResolveEquipment("mount_grimfang_wolf", 1, nil)
	if err != nil {
		t.Fatalf("resolve mount: %v", err)
	}
	if !mount.IsMount || mount.FinalBonuses["finalAttack"] != 0.05 || mount.FinalBonuses["moveSpeed"] != 0.25 {
		t.Fatalf("mount resolution = %+v", mount)
	}
}

func TestBountyTaskPlansRespectRefreshMode(t *testing.T) {
	cfg, err := LoadDir(filepath.Join("..", "..", "..", "configs", "economy"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	normal, err := cfg.BountyTaskPlan("bounty-normal", 2, 1, false)
	if err != nil || normal.Difficulty != "normal" || normal.Required <= 0 {
		t.Fatalf("normal bounty=%+v err=%v", normal, err)
	}
	premium, err := cfg.BountyTaskPlan("bounty-premium", 2, 1, true)
	if err != nil || premium.Type != "submit_equipment" || premium.MinRarity != 3 {
		t.Fatalf("premium bounty=%+v err=%v", premium, err)
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

func TestGatheringNodesAwardFourQualityMaterialsAndRareVoucher(t *testing.T) {
	cfg, err := LoadDir(filepath.Join("..", "..", "..", "configs", "economy"))
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cases := []struct {
		nodeID string
		items  []string
	}{
		{"shadow_woods_ashwood_grove", []string{"ashwood_white", "ashwood_green", "ashwood_blue", "ashwood_purple"}},
		{"shadow_woods_iron_vein", []string{"shadow_iron_white", "shadow_iron_green", "shadow_iron_blue", "shadow_iron_purple"}},
		{"shadow_woods_gloomcap_patch", []string{"gloomcap_spore_white", "gloomcap_spore_green", "gloomcap_spore_blue", "gloomcap_spore_purple"}},
		{"shadow_woods_gloomstone_outcrop", []string{"gloomstone_white", "gloomstone_green", "gloomstone_blue", "gloomstone_purple"}},
	}
	for _, tc := range cases {
		node, ok := cfg.GatherNodes[tc.nodeID]
		if !ok {
			t.Fatalf("gathering node %q is missing", tc.nodeID)
		}
		pool, ok := cfg.LootPools[node.LootPoolID]
		if !ok {
			t.Fatalf("gathering node %q references missing pool %q", tc.nodeID, node.LootPoolID)
		}
		chances := map[string]float64{}
		for _, entry := range pool.Entries {
			chances[entry.ItemID] = entry.DropChance
		}
		for _, itemID := range tc.items {
			if chances[itemID] == 0 {
				t.Fatalf("gathering node %q does not award %q: %+v", tc.nodeID, itemID, pool.Entries)
			}
		}
		if chances[tc.items[0]] != 1 || chances[tc.items[1]] != 0.25 || chances[tc.items[2]] != 0.05 || chances[tc.items[3]] != 0.005 {
			t.Fatalf("gathering node %q quality chances = %+v", tc.nodeID, chances)
		}
		if chances["aeb_exchange_voucher"] != 0.0002 {
			t.Fatalf("gathering node %q voucher chance = %v, want 0.0002", tc.nodeID, chances["aeb_exchange_voucher"])
		}
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
