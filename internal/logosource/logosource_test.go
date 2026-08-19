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
