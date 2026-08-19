package config

import (
	"encoding/json"
	"testing"
	"time"
)

// TestPollIntervalIsClamped protects Liquipedia rate-limit compliance: a
// hand-edited config must not be able to make the daemon poll aggressively.
func TestPollIntervalIsClamped(t *testing.T) {
	c := Default()
	c.PollInterval = Duration(5 * time.Second)
	c.clamp()
	if time.Duration(c.PollInterval) < MinPollInterval {
		t.Errorf("poll interval %s was not clamped to %s", time.Duration(c.PollInterval), MinPollInterval)
	}
}

func TestUnknownSpoilerModeFallsBackToStrict(t *testing.T) {
	c := Default()
	c.Spoilers = SpoilerMode("nonsense")
	c.clamp()
	// Falling back to strict rather than off means a typo cannot silently
	// start showing results.
	if c.Spoilers != SpoilerStrict {
		t.Errorf("spoiler mode fell back to %q, want strict", c.Spoilers)
	}
}

func TestDurationJSONRoundTrip(t *testing.T) {
	type holder struct {
		D Duration `json:"d"`
	}
	out, err := json.Marshal(holder{Duration(15 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"d":"15m0s"}` {
		t.Errorf("marshalled as %s, want a human-readable string", out)
	}

	for _, in := range []string{`{"d":"90s"}`, `{"d":90}`} {
		var h holder
		if err := json.Unmarshal([]byte(in), &h); err != nil {
			t.Fatalf("%s: %v", in, err)
		}
		if time.Duration(h.D) != 90*time.Second {
			t.Errorf("%s decoded to %s, want 90s", in, time.Duration(h.D))
		}
	}
}

// TestDefaultsCoverTheShippedWikis guards the starcraft2 ticker-page quirk:
// its Liquipedia:Matches page is a stale archive, so the default must point at
// Main_Page instead.
func TestDefaultsCoverTheShippedWikis(t *testing.T) {
	c := Default()
	want := map[string]string{
		"dota2":         "Liquipedia:Matches",
		"counterstrike": "Liquipedia:Matches",
		"starcraft2":    "Main_Page",
	}
	got := map[string]string{}
	for _, w := range c.EnabledWikis() {
		got[w.Slug] = w.TickerPage
	}
	for slug, page := range want {
		if got[slug] != page {
			t.Errorf("wiki %s ticker page = %q, want %q", slug, got[slug], page)
		}
	}
}

func TestFollows(t *testing.T) {
	c := Config{Teams: []Follow{{Name: "Team Spirit"}, {Name: "  G2 Esports  "}}}
	for _, name := range []string{"team spirit", "TEAM SPIRIT", "G2 Esports"} {
		if !c.Follows(name, "") {
			t.Errorf("Follows(%q) = false, want true", name)
		}
	}
	if c.Follows("Astralis", "") {
		t.Error("Follows returned true for an unfollowed team")
	}
}

// TestFollowsIsGameScoped covers the case that prompted the feature: an org
// fields rosters in several games and you may want only one of them.
func TestFollowsIsGameScoped(t *testing.T) {
	c := Config{Teams: []Follow{{Name: "GamerLegion", Wiki: "dota2"}}}

	if !c.Follows("GamerLegion", "dota2") {
		t.Error("should follow the scoped game")
	}
	if c.Follows("GamerLegion", "counterstrike") {
		t.Error("must not follow a game the entry is not scoped to")
	}
	// An unscoped query asks "followed anywhere?".
	if !c.Follows("GamerLegion", "") {
		t.Error("unscoped query should match a scoped entry")
	}

	// An unscoped entry follows the org everywhere.
	all := Config{Teams: []Follow{{Name: "Team Spirit"}}}
	for _, w := range []string{"dota2", "counterstrike", ""} {
		if !all.Follows("Team Spirit", w) {
			t.Errorf("unscoped follow should match wiki %q", w)
		}
	}
}

// TestFollowJSON covers both accepted forms and the compact round-trip.
func TestFollowJSON(t *testing.T) {
	var f Follow
	if err := json.Unmarshal([]byte(`"Team Spirit"`), &f); err != nil {
		t.Fatalf("bare string form should decode: %v", err)
	}
	if f.Name != "Team Spirit" || f.Wiki != "" {
		t.Errorf("bare string decoded to %+v", f)
	}

	if err := json.Unmarshal([]byte(`{"name":"GamerLegion","wiki":"dota2"}`), &f); err != nil {
		t.Fatalf("object form should decode: %v", err)
	}
	if f.Name != "GamerLegion" || f.Wiki != "dota2" {
		t.Errorf("object decoded to %+v", f)
	}

	// An unscoped entry marshals back to a bare string so hand-edited configs
	// stay readable.
	out, _ := json.Marshal(Follow{Name: "Team Spirit"})
	if string(out) != `"Team Spirit"` {
		t.Errorf("unscoped follow marshalled as %s, want a bare string", out)
	}
	out, _ = json.Marshal(Follow{Name: "GamerLegion", Wiki: "dota2"})
	if string(out) != `{"name":"GamerLegion","wiki":"dota2"}` {
		t.Errorf("scoped follow marshalled as %s", out)
	}
}

// TestConfigWithMixedTeamForms is the realistic file: some entries scoped,
// some not.
func TestConfigWithMixedTeamForms(t *testing.T) {
	var c Config
	raw := `{"teams":["Team Spirit",{"name":"GamerLegion","wiki":"dota2"}]}`
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Teams) != 2 {
		t.Fatalf("got %d teams", len(c.Teams))
	}
	if c.Teams[0].Wiki != "" || c.Teams[1].Wiki != "dota2" {
		t.Errorf("mixed forms decoded wrong: %+v", c.Teams)
	}
	if c.Teams[1].Label() != "GamerLegion (dota2)" {
		t.Errorf("Label() = %q", c.Teams[1].Label())
	}
}
