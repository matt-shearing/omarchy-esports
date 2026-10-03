package liquipedia

import "testing"

// StarCraft players carry a race icon beside their name; team opponents (the
// GSTL team league) carry one per player and must be left without a race.
func TestParseTickerReadsStarCraftRaces(t *testing.T) {
	ms, err := ParseTicker(fixture(t, "starcraft2"), "starcraft2", "StarCraft II")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, m := range ms {
		for _, o := range m.Opponents {
			counts[o.Race]++
		}
	}
	if counts["Protoss"] == 0 || counts["Terran"] == 0 || counts["Zerg"] == 0 {
		t.Fatalf("expected all three races, got %v", counts)
	}
	for r := range counts {
		switch r {
		case "", "Protoss", "Terran", "Zerg", "Random":
		default:
			t.Fatalf("unexpected race %q", r)
		}
	}
	// Other games never get a race.
	cs, _ := ParseTicker(fixture(t, "counterstrike"), "counterstrike", "Counter-Strike")
	for _, m := range cs {
		for _, o := range m.Opponents {
			if o.Race != "" {
				t.Fatalf("CS opponent %q has race %q", o.Name, o.Race)
			}
		}
	}
}
