package store

import "testing"

func TestNFTRulesSelectAEBMintFeeByRarity(t *testing.T) {
	rules := NFTRules{}.WithDefaults()
	for rarity, want := range map[int]int64{3: 500, 4: 2000, 5: 10000, 6: 10000} {
		if got := rules.MintFeeForRarity(rarity); got != want {
			t.Errorf("MintFeeForRarity(%d) = %d, want %d AEB", rarity, got, want)
		}
	}
}
