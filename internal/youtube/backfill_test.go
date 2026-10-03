package youtube

import (
	"testing"
	"time"

	"github.com/contra/omarchy-esports/internal/match"
)

func fixture(a, b string) match.Match {
	return match.Match{
		Opponents: [2]match.Opponent{{Name: a}, {Name: b}},
		StartsAt:  time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC),
		State:     match.StateFinished,
	}
}

func vid(id, title, lang string) Video {
	return Video{ID: id, Title: title, Lang: lang}
}

// A real International listing: one series, three games, four languages.
func tiListing() []Video {
	var out []Video
	for _, lang := range []string{"en", "es", "ru", "pt"} {
		for g := 1; g <= 3; g++ {
			id := lang + string(rune('0'+g))
			title := "[" + lang + "] Team Falcons vs GamerLegion - Game " +
				string(rune('0'+g)) + " - The International 2026"
			out = append(out, vid(id, title, lang))
		}
	}
	return out
}

func TestShortlistForPrefersLanguageAndFirstGame(t *testing.T) {
	got := ShortlistFor([]match.Match{fixture("Team Falcons", "GamerLegion")}, tiListing(), "en", 3)

	if len(got) != 3 {
		t.Fatalf("want 3 candidates from 12 uploads, got %d", len(got))
	}
	if got[0].ID != "en1" {
		t.Errorf("want the English game 1 first, got %q (%s)", got[0].ID, got[0].Title)
	}
	for _, v := range got {
		if v.Lang != "en" {
			t.Errorf("a non-preferred language reached the shortlist: %q", v.Title)
		}
	}
}

func TestShortlistForSkipsUnrelatedAndDeduplicates(t *testing.T) {
	listing := append(tiListing(),
		vid("x1", "[EN] Team Spirit vs Xtreme Gaming - Game 1", "en"),
		vid("x2", "The International 2026 - Opening Ceremony", "en"),
	)
	// The same fixture twice must not yield the same video twice.
	ms := []match.Match{
		fixture("Team Falcons", "GamerLegion"),
		fixture("Team Falcons", "GamerLegion"),
	}
	got := ShortlistFor(ms, listing, "en", 3)

	seen := map[string]bool{}
	for _, v := range got {
		if seen[v.ID] {
			t.Errorf("duplicate video %q in shortlist", v.ID)
		}
		seen[v.ID] = true
		if v.ID == "x1" || v.ID == "x2" {
			t.Errorf("unrelated upload %q reached the shortlist", v.Title)
		}
	}
}

func TestShortlistForRanksFullMatchOverHighlights(t *testing.T) {
	listing := []Video{
		vid("h", "[EN] Team Falcons vs GamerLegion - HIGHLIGHTS", "en"),
		vid("f", "[EN] Team Falcons vs GamerLegion - Game 1 - The International 2026", "en"),
	}
	got := ShortlistFor([]match.Match{fixture("Team Falcons", "GamerLegion")}, listing, "en", 2)
	if len(got) != 2 || got[0].ID != "f" {
		t.Fatalf("want the full match ranked first, got %+v", got)
	}
}

func TestGameNo(t *testing.T) {
	for title, want := range map[string]int{
		"[EN] A vs B - Game 1":  1,
		"[EN] A vs B - Game 3":  3,
		"[EN] A vs B - Map 2":   2,
		"[EN] A vs B":           1,
		"[EN] A vs B - Grand F": 1,
	} {
		if got := gameNo(title); got != want {
			t.Errorf("gameNo(%q) = %d, want %d", title, got, want)
		}
	}
}

// Without yt-dlp the backfiller must degrade rather than break.
func TestBackfillerUnavailable(t *testing.T) {
	b := &Backfiller{}
	if b.Available() {
		t.Fatal("empty Backfiller reports available")
	}
	if _, err := b.List(t.Context(), "UC123"); err == nil {
		t.Error("List should error without yt-dlp")
	}
	if _, err := b.Dates(t.Context(), []string{"a"}); err == nil {
		t.Error("Dates should error without yt-dlp")
	}
}

func TestDatesWithNoIDsMakesNoCall(t *testing.T) {
	b := &Backfiller{bin: "/nonexistent/yt-dlp"}
	got, err := b.Dates(t.Context(), nil)
	if err != nil || len(got) != 0 {
		t.Fatalf("want empty result and no error, got %v %v", got, err)
	}
}

// A cap applied to fixture-ordered output starves the fixtures at the tail.
// Breadth-first ordering must give every fixture a candidate before any
// fixture gets a second one.
func TestShortlistForIsBreadthFirstAcrossFixtures(t *testing.T) {
	var listing []Video
	ms := []match.Match{}
	for _, pair := range [][2]string{{"Alpha", "Bravo"}, {"Charlie", "Delta"}, {"Echo", "Foxtrot"}} {
		ms = append(ms, fixture(pair[0], pair[1]))
		for g := 1; g <= 3; g++ {
			id := pair[0][:1] + string(rune('0'+g))
			listing = append(listing, vid(id,
				"[EN] "+pair[0]+" vs "+pair[1]+" - Game "+string(rune('0'+g)), "en"))
		}
	}

	got := ShortlistFor(ms, listing, "en", 3)
	if len(got) != 9 {
		t.Fatalf("want all 9 candidates, got %d", len(got))
	}
	// Truncating to one-per-fixture must still cover all three fixtures.
	first := map[byte]bool{}
	for _, v := range got[:3] {
		first[v.ID[0]] = true
		if v.ID[1] != '1' {
			t.Errorf("first pass should carry game 1 only, got %q", v.ID)
		}
	}
	if len(first) != 3 {
		t.Errorf("first 3 entries cover %d fixtures, want 3: %v", len(first), got[:3])
	}
}
