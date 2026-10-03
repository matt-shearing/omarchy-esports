package logosource

import (
	"strings"
	"testing"
)

func TestMapLoads(t *testing.T) {
	if Count() == 0 {
		t.Fatal("no curated sources loaded; the embedded file is missing or malformed")
	}
	t.Logf("%d teams mapped", Count())
}

func TestLookupIsCaseInsensitive(t *testing.T) {
	for _, name := range []string{"Team Liquid", "team liquid", "  TEAM LIQUID  "} {
		if URLFor(name) == "" {
			t.Errorf("no source found for %q", name)
		}
	}
	if URLFor("Definitely Not A Team") != "" {
		t.Error("unmapped team should return an empty URL")
	}
}

func TestEveryEntryIsUsable(t *testing.T) {
	load()
	for name, e := range byName {
		if !strings.HasPrefix(e.URL, "https://") {
			t.Errorf("%s: url is not https: %q", name, e.URL)
		}
		// Liquipedia is the fallback, not a curated source — an entry pointing
		// back at it would defeat the purpose of the map.
		if strings.Contains(e.URL, "liquipedia.net") {
			t.Errorf("%s: curated source points back at Liquipedia", name)
		}
		if e.Terms == "" {
			t.Errorf("%s: terms must be recorded, use \"none stated\" if unknown", name)
		}
	}
}

// Game artwork is optional by design: a game with no curated source falls back
// to its short text badge, so the map being incomplete must not break lookups.
func TestGameSources(t *testing.T) {
	if _, ok := GameFor("definitely-not-a-game"); ok {
		t.Error("unknown slug reported as having artwork")
	}
	if u := GameURLFor("definitely-not-a-game"); u != "" {
		t.Errorf("unknown slug returned a URL: %q", u)
	}
	// Every entry that exists must be usable: a keyed entry with no URL is
	// worse than no entry, because callers would treat it as artwork.
	for _, slug := range []string{"dota2", "counterstrike", "leagueoflegends"} {
		if e, ok := GameFor(slug); ok && e.URL == "" {
			t.Errorf("%s is present but has no URL", slug)
		}
	}
	if n := GameCount(); n < 0 {
		t.Errorf("GameCount = %d", n)
	}
}

// Slugs come from the catalog, so lookups must not care about surrounding
// whitespace or case.
func TestGameLookupNormalises(t *testing.T) {
	if GameCount() == 0 {
		t.Skip("no curated game artwork yet")
	}
	for slug := range bySlug {
		if _, ok := GameFor(" " + slug + " "); !ok {
			t.Errorf("lookup failed on padded slug %q", slug)
		}
		break
	}
}
