package config

import (
	"encoding/json"
	"os"
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
	// Check every configured wiki, not only the enabled ones: starcraft2 ships
	// available-but-off, and its Main_Page quirk must survive regardless.
	got := map[string]string{}
	for _, w := range c.Wikis {
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

// TestCatalogIsWellFormed guards the shipped game list: a malformed entry
// silently adds a game that shows nothing.
func TestCatalogIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range Catalog {
		if e.Slug == "" || e.Game == "" || e.TickerPage == "" {
			t.Errorf("incomplete catalog entry: %+v", e)
		}
		if seen[e.Slug] {
			t.Errorf("duplicate catalog slug %q", e.Slug)
		}
		seen[e.Slug] = true
		if e.Fixtures < 0 {
			t.Errorf("%s: negative fixture count", e.Slug)
		}
	}
	// Every game must carry a short badge for the UI.
	for _, e := range Catalog {
		if e.Short == "" || len(e.Short) > 5 {
			t.Errorf("%s: short badge %q must be 1-5 chars", e.Slug, e.Short)
		}
	}
	for _, slug := range []string{"counterstrike", "dota2", "starcraft2"} {
		if _, ok := CatalogFor(slug); !ok {
			t.Errorf("catalog is missing default game %q", slug)
		}
	}
	// starcraft2 and Brood War use Main_Page; their Liquipedia:Matches pages
	// are missing or stale, and regressing this silently empties the game.
	for _, slug := range []string{"starcraft2", "starcraft"} {
		e, _ := CatalogFor(slug)
		if e.TickerPage != "Main_Page" {
			t.Errorf("%s ticker page = %q, want Main_Page", slug, e.TickerPage)
		}
	}
}

// TestDefaultWikisStartNarrow keeps the catalog from silently turning on every
// game: each enabled game costs a 30-second parse slot per refresh, so a fresh
// install must start small and let the wizard widen it.
func TestDefaultWikisStartNarrow(t *testing.T) {
	on := map[string]bool{}
	for _, w := range Default().EnabledWikis() {
		on[w.Slug] = true
	}
	if len(on) != 2 {
		t.Errorf("expected 2 games enabled by default, got %d: %v", len(on), on)
	}
	for _, slug := range []string{"counterstrike", "dota2"} {
		if !on[slug] {
			t.Errorf("%s should be enabled by default", slug)
		}
	}
	if on["starcraft2"] {
		t.Error("starcraft2 should be available but off by default")
	}
	// Everything else must be present but dormant.
	if len(Default().Wikis) != len(Catalog) {
		t.Errorf("default config has %d wikis, catalog has %d", len(Default().Wikis), len(Catalog))
	}
}

// TestLoadDoesNotInheritDefaultSliceElements guards a subtle json.Unmarshal
// behaviour: decoding an array into a slice that already has elements reuses
// those elements, so fields absent from the file keep the default's value at
// that index. With defaults holding the full game catalog, a config listing
// four wikis in a different order silently paired each with the wrong short
// badge — dota2 rendered as "CS2".
func TestLoadDoesNotInheritDefaultSliceElements(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	raw := `{"wikis":[
      {"slug":"dota2","game":"Dota 2","tickerPage":"Liquipedia:Matches","enabled":true},
      {"slug":"counterstrike","game":"Counter-Strike","tickerPage":"Liquipedia:Matches","enabled":true}
    ]}`
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"dota2": "DOTA2", "counterstrike": "CS2"}
	for _, w := range cfg.Wikis {
		if exp, ok := want[w.Slug]; ok && w.Short != exp {
			t.Errorf("%s got short %q, want %q — slice elements leaked between entries",
				w.Slug, w.Short, exp)
		}
		// Every entry must agree with the catalog on its own identity.
		if e, ok := CatalogFor(w.Slug); ok && w.Game != e.Game {
			t.Errorf("%s got game %q, want %q", w.Slug, w.Game, e.Game)
		}
	}
}

// TestLoadRepairsWrongShortBadges covers configs already written with badges
// belonging to another game, from the slice-reuse bug.
func TestLoadRepairsWrongShortBadges(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	raw := `{"wikis":[
      {"slug":"dota2","game":"Dota 2","short":"CS2","tickerPage":"Liquipedia:Matches","enabled":true},
      {"slug":"valorant","game":"VALORANT","short":"LOL","tickerPage":"Liquipedia:Matches","enabled":true}
    ]}`
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"dota2": "DOTA2", "valorant": "VAL"}
	for _, w := range cfg.Wikis {
		if exp, ok := want[w.Slug]; ok && w.Short != exp {
			t.Errorf("%s kept the wrong badge %q, want %q", w.Slug, w.Short, exp)
		}
	}
}
