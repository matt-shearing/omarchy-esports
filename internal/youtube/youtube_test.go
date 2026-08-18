package youtube

import (
	"testing"
	"time"

	"github.com/contra/omarchy-esports/internal/match"
)

func TestUploadsFeedURL(t *testing.T) {
	got := UploadsFeedURL("UCTQKT5QqO3h7y32G8VzuySQ")
	want := "https://www.youtube.com/feeds/videos.xml?playlist_id=UUTQKT5QqO3h7y32G8VzuySQ"
	if got != want {
		t.Errorf("UploadsFeedURL = %q, want %q", got, want)
	}
	if UploadsFeedURL("notachannel") != "" {
		t.Error("expected empty result for a non-UC id")
	}
}

func TestIsHighlight(t *testing.T) {
	highlights := []string{
		"HIGHLIGHTS: NAVI vs G2 | EWC 2026",
		"G2 vs NAVI - Best of the Group Stage",
		"TI 2026 Day 3 Recap",
		"Top Plays of The International 2026",
		"The International in 10 minutes",
	}
	for _, h := range highlights {
		if !IsHighlight(h) {
			t.Errorf("missed highlight: %q", h)
		}
	}
	full := []string{
		"[EN] Team Spirit vs Falcons - Game 1 - The International 2026",
		"G2 Esports vs Astralis | Esports World Cup 2026 | Playoffs",
	}
	for _, f := range full {
		if IsHighlight(f) {
			t.Errorf("false positive on a full match: %q", f)
		}
	}
}

// TestMentions covers the name matching that ties a VOD to a fixture. The
// tricky cases come from Liquipedia's canonical names differing from how
// broadcasters write them.
func TestMentions(t *testing.T) {
	cases := []struct {
		title string
		opp   match.Opponent
		want  bool
	}{
		{"NAVI проти 3DMAX | EWC", match.Opponent{Name: "Natus Vincere", Short: "NAVI"}, true},
		{"Team Spirit vs Falcons", match.Opponent{Name: "Team Spirit", Short: "TSpirit"}, true},
		{"PSG.LGD vs Yandex", match.Opponent{Name: "PSG LGD", Short: "LGD"}, true},
		{"G2 Esports vs Astralis", match.Opponent{Name: "G2 Esports", Short: "G2"}, true},
		// "Gaming" is filler and must not let an unrelated org match.
		{"Aurora Gaming vs BB", match.Opponent{Name: "LGD Gaming", Short: "LGD"}, false},
		{"Falcons vs Liquid", match.Opponent{Name: "Team Spirit", Short: "TSpirit"}, false},
	}
	for _, tc := range cases {
		if got := mentions(tc.title, tc.opp); got != tc.want {
			t.Errorf("mentions(%q, %q) = %v, want %v", tc.title, tc.opp.Name, got, tc.want)
		}
	}
}

// TestMatchPrefersFullOverHighlights is the behaviour that matters when an
// org posts both a highlights reel and the full game.
func TestMatchPrefersFullOverHighlights(t *testing.T) {
	start := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	m := match.Match{
		StartsAt:  start,
		Opponents: [2]match.Opponent{{Name: "Team Spirit", Short: "TSpirit"}, {Name: "Team Falcons", Short: "Falcons"}},
	}
	videos := []Video{
		{ID: "hl", Title: "HIGHLIGHTS Team Spirit vs Team Falcons", Published: start.Add(time.Hour), Lang: "en"},
		{ID: "full", Title: "[EN] Team Spirit vs Team Falcons - Game 1", Published: start.Add(2 * time.Hour), Lang: "en"},
	}
	got := Match(m, videos, "en", 7*24*time.Hour)
	if got == nil {
		t.Fatal("no VOD matched")
	}
	if got.VideoID != "full" {
		t.Errorf("picked %q, want the full match", got.VideoID)
	}
	if got.Kind != "full" {
		t.Errorf("Kind = %q, want full", got.Kind)
	}
}

func TestMatchPrefersLanguage(t *testing.T) {
	start := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	m := match.Match{
		StartsAt:  start,
		Opponents: [2]match.Opponent{{Name: "LGD Gaming", Short: "LGD"}, {Name: "Team Yandex", Short: "Yandex"}},
	}
	videos := []Video{
		{ID: "ru", Title: "[RU] LGD vs Yandex - Игра 1", Published: start.Add(time.Hour), Lang: "ru"},
		{ID: "en", Title: "[EN] LGD vs Yandex - Game 1", Published: start.Add(90 * time.Minute), Lang: "en"},
	}
	got := Match(m, videos, "en", 7*24*time.Hour)
	if got == nil || got.VideoID != "en" {
		t.Errorf("expected the English feed, got %+v", got)
	}
}

// TestMatchRejectsOutOfWindow guards against attaching last season's video to
// this week's fixture between the same two teams.
func TestMatchRejectsOutOfWindow(t *testing.T) {
	start := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	m := match.Match{
		StartsAt:  start,
		Opponents: [2]match.Opponent{{Name: "G2 Esports", Short: "G2"}, {Name: "Astralis", Short: "Astralis"}},
	}
	videos := []Video{
		{ID: "old", Title: "G2 Esports vs Astralis - Game 1", Published: start.Add(-90 * 24 * time.Hour), Lang: "en"},
		{ID: "future", Title: "G2 Esports vs Astralis - Game 1", Published: start.Add(60 * 24 * time.Hour), Lang: "en"},
	}
	if got := Match(m, videos, "en", 7*24*time.Hour); got != nil {
		t.Errorf("matched a video outside the window: %s", got.VideoID)
	}
}

func TestGameNumberLeak(t *testing.T) {
	if GameNumberLeak("Team A vs Team B - Game 3 - TI") != 3 {
		t.Error("failed to read game number")
	}
	if GameNumberLeak("Team A vs Team B") != 0 {
		t.Error("reported a game number where there is none")
	}
}
