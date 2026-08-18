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
	c := Config{Teams: []string{"Team Spirit", "  G2 Esports  "}}
	for _, name := range []string{"team spirit", "TEAM SPIRIT", "G2 Esports"} {
		if !c.Follows(name) {
			t.Errorf("Follows(%q) = false, want true", name)
		}
	}
	if c.Follows("Astralis") {
		t.Error("Follows returned true for an unfollowed team")
	}
}
