package daemon

import (
	"testing"

	"github.com/contra/omarchy-esports/internal/config"
	"github.com/contra/omarchy-esports/internal/match"
)

func TestMatchesInterestTopTierProtoss(t *testing.T) {
	in := config.Interest{Wiki: "starcraft2", MaxTier: 2, Race: "Protoss", MainEventOnly: true}
	mk := func(wiki string, tier int, tierType string, races ...string) *match.Match {
		m := &match.Match{Wiki: wiki}
		m.Tournament.Tier = tier
		m.Tournament.TierType = tierType
		for i, r := range races {
			m.Opponents[i] = match.Opponent{Name: "p", Race: r}
		}
		return m
	}
	cases := []struct {
		name string
		m    *match.Match
		want bool
	}{
		{"S-tier PvZ", mk("starcraft2", 1, "", "Protoss", "Zerg"), true},
		{"A-tier ZvP", mk("starcraft2", 2, "", "Zerg", "protoss"), true},
		{"B-tier PvT", mk("starcraft2", 3, "", "Protoss", "Terran"), false},
		{"unknown tier", mk("starcraft2", 0, "", "Protoss", "Terran"), false},
		{"S-tier qualifier", mk("starcraft2", 1, "Qualifier", "Protoss", "Terran"), false},
		{"S-tier TvZ", mk("starcraft2", 1, "", "Terran", "Zerg"), false},
		{"other game", mk("dota2", 1, "", "Protoss", ""), false},
	}
	for _, c := range cases {
		if got := MatchesInterest(c.m, in); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
