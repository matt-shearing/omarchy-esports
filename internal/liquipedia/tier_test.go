package liquipedia

import "testing"

// TestNormaliseTier covers the per-wiki encodings actually observed:
// Counter-Strike writes "S-Tier", Dota 2 writes "1".
func TestNormaliseTier(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"S-Tier", TierS},
		{"s-tier", TierS},
		{"A-Tier", TierA},
		{"B-Tier", TierB},
		{"C-Tier", TierC},
		{"D-Tier", TierD},
		{"1", TierS},
		{"3", TierB},
		{"Tier 1", TierS},
		{" 2 ", TierA},
		// Not tiers.
		{"", TierUnknown},
		{"Misc", TierUnknown},
		{"Show Match", TierUnknown},
		{"7", TierUnknown},
		{"0", TierUnknown},
	}
	for _, c := range cases {
		if got := NormaliseTier(c.in); got != c.want {
			t.Errorf("NormaliseTier(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestTierName(t *testing.T) {
	if TierName(TierS) != "S-Tier" {
		t.Error("tier 1 should render as S-Tier")
	}
	if TierName(TierUnknown) != "" {
		t.Error("unknown tier should render empty")
	}
}
