package spoiler

import (
	"strings"
	"testing"
	"time"

	"github.com/contra/omarchy-esports/internal/config"
	"github.com/contra/omarchy-esports/internal/match"
)

// TestScanLeaks covers titles in the styles esports channels actually publish.
func TestScanLeaks(t *testing.T) {
	leaky := []string{
		"NAVI 2-0 G2 | IEM Katowice 2026 Grand Final",
		"Team Spirit 2:1 Falcons - The International 2026",
		"FaZe DESTROY Vitality in Bo3 thriller",
		"G2 beat NAVI to take the title",
		"Team Liquid ELIMINATED from The International 2026",
		"Spirit advance to the Grand Final",
		"Gaimin Gladiators are CHAMPIONS",
		"Insane comeback from Vitality",
		"Falcons def. Spirit | ESL One",
		"Heroic qualify for playoffs",
	}
	for _, title := range leaky {
		if !IsSpoilery(title) {
			t.Errorf("missed leak in %q", title)
		}
	}
}

// TestScanSafe guards against over-flagging. A tool that blacks out every
// title becomes useless, so neutral scheduling language must survive.
func TestScanSafe(t *testing.T) {
	safe := []string{
		"G2 vs NAVI | BLAST Premier World Final 2026 | Grand Final",
		"The International 2026 - Main Event Day 5",
		"ESL One Birmingham 2026 - Group Stage",
		"Team Spirit vs Falcons - Bo3 - IEM Cologne",
		"PGL Wallachia Season 6 | Playoffs",
		"Serral vs Clem | IEM Katowice StarCraft II",
	}
	for _, title := range safe {
		if sigs := Scan(title); len(sigs) > 0 {
			t.Errorf("false positive on %q: %v", title, sigs)
		}
	}
}

// TestRedactStrictWithholdsEverything is the core guarantee: under the strict
// policy a finished match hands the UI no result data at all.
func TestRedactStrictWithholdsEverything(t *testing.T) {
	m := match.Match{
		ID:     "abc",
		State:  match.StateFinished,
		Score:  [2]int{2, 0},
		Winner: 1,
		VOD: &match.VOD{
			VideoID:   "xyz",
			Title:     "NAVI 2-0 G2 | Grand Final",
			Thumbnail: "https://i.ytimg.com/vi/xyz/hq.jpg",
			Published: time.Now(),
		},
	}
	got := Redact(m, config.SpoilerStrict, false)

	if got.Score != [2]int{0, 0} {
		t.Errorf("score leaked: %v", got.Score)
	}
	if got.Winner != 0 {
		t.Errorf("winner leaked: %d", got.Winner)
	}
	if !got.Redacted {
		t.Error("Redacted flag not set")
	}
	if got.VOD.Title != "" {
		t.Errorf("VOD title leaked: %q", got.VOD.Title)
	}
	if got.VOD.Thumbnail != "" {
		t.Errorf("VOD thumbnail leaked: %q", got.VOD.Thumbnail)
	}
	if got.VOD.VideoID == "" {
		t.Error("video id should survive redaction so the VOD stays playable")
	}
	if len(got.VOD.Spoilery) == 0 {
		t.Error("expected the reason for the blackout to be recorded")
	}
}

// TestRedactRevealed confirms an explicit reveal restores everything.
func TestRedactRevealed(t *testing.T) {
	m := match.Match{
		State:  match.StateFinished,
		Score:  [2]int{2, 1},
		Winner: 1,
		VOD:    &match.VOD{Title: "NAVI 2-1 G2"},
	}
	got := Redact(m, config.SpoilerStrict, true)
	if got.Score != [2]int{2, 1} || got.Winner != 1 {
		t.Errorf("reveal did not restore the result: %v winner=%d", got.Score, got.Winner)
	}
	if got.VOD.Title == "" {
		t.Error("reveal did not restore the VOD title")
	}
	if !got.Revealed {
		t.Error("Revealed flag not set")
	}
}

// TestRedactLeavesUpcomingAlone ensures the blackout does not swallow the
// schedule, which is the feature's main job.
func TestRedactLeavesUpcomingAlone(t *testing.T) {
	for _, st := range []match.State{match.StateUpcoming, match.StateLive} {
		m := match.Match{State: st, Opponents: [2]match.Opponent{{Name: "G2"}, {Name: "NAVI"}}}
		got := Redact(m, config.SpoilerStrict, false)
		if got.Redacted {
			t.Errorf("state %s was redacted but has no result to hide", st)
		}
	}
}

// TestRedactBalanced keeps a scrubbed title and drops the artwork.
func TestRedactBalanced(t *testing.T) {
	m := match.Match{
		State: match.StateFinished,
		Score: [2]int{2, 0},
		VOD: &match.VOD{
			Title:     "NAVI 2-0 G2 | IEM Katowice Grand Final",
			Thumbnail: "https://i.ytimg.com/vi/x/hq.jpg",
		},
	}
	got := Redact(m, config.SpoilerBalanced, false)
	if got.Score != [2]int{0, 0} {
		t.Errorf("balanced mode leaked the score: %v", got.Score)
	}
	if got.VOD.Thumbnail != "" {
		t.Error("balanced mode should still drop the thumbnail")
	}
	if strings.Contains(got.VOD.Title, "2-0") {
		t.Errorf("balanced mode left a scoreline in the title: %q", got.VOD.Title)
	}
	if got.VOD.Title == "" {
		t.Error("balanced mode should keep a scrubbed title, not blank it")
	}
	t.Logf("scrubbed title: %q", got.VOD.Title)
}

// TestRedactOff passes everything through.
func TestRedactOff(t *testing.T) {
	m := match.Match{State: match.StateFinished, Score: [2]int{2, 0}, Winner: 1}
	got := Redact(m, config.SpoilerOff, false)
	if got.Score != [2]int{2, 0} || got.Redacted {
		t.Errorf("spoilers-off should pass results through, got %v redacted=%v", got.Score, got.Redacted)
	}
}

func TestScrub(t *testing.T) {
	cases := []struct{ in, wantAbsent string }{
		{"NAVI 2-0 G2 | Grand Final", "2-0"},
		{"Spirit beat Falcons | ESL One", "beat"},
		{"Team Liquid ELIMINATED | TI 2026", "ELIMINATED"},
	}
	for _, tc := range cases {
		got := Scrub(tc.in)
		if strings.Contains(strings.ToLower(got), strings.ToLower(tc.wantAbsent)) {
			t.Errorf("Scrub(%q) = %q, still contains %q", tc.in, got, tc.wantAbsent)
		}
		if strings.TrimSpace(got) == "" {
			t.Errorf("Scrub(%q) removed everything", tc.in)
		}
	}
}

// TestSeriesLengthLeaks covers the format-dependent leak: a title naming a
// game beyond the number needed to win the series reveals the score.
func TestSeriesLengthLeaks(t *testing.T) {
	cases := []struct {
		title  string
		bestOf int
		want   bool
		why    string
	}{
		{"[EN] LGD vs Yandex - Game 3 - The International 2026", 3, true, "game 3 of a Bo3 means it was 1-1"},
		{"[EN] LGD vs Yandex - Game 2 - The International 2026", 3, false, "every Bo3 can reach game 2"},
		{"[EN] LGD vs Yandex - Game 1 - The International 2026", 3, false, "game 1 reveals nothing"},
		{"Spirit vs Falcons - Game 5 - Grand Final", 5, true, "game 5 of a Bo5 means it was 2-2"},
		{"Spirit vs Falcons - Game 4 - Grand Final", 5, true, "game 4 of a Bo5 means a side led 2-1"},
		{"Spirit vs Falcons - Game 3 - Grand Final", 5, false, "every Bo5 can reach game 3"},
		{"Serral vs Clem - Map 3", 3, true, "map wording behaves like game wording"},
		{"G2 vs NAVI - Game 3", 0, false, "unknown format means we cannot infer anything"},
		{"G2 vs NAVI - Bo1", 1, false, "a Bo1 has no later games to leak"},
	}
	for _, tc := range cases {
		if got := SeriesLengthLeaks(tc.title, tc.bestOf); got != tc.want {
			t.Errorf("SeriesLengthLeaks(%q, Bo%d) = %v, want %v — %s",
				tc.title, tc.bestOf, got, tc.want, tc.why)
		}
	}
}

// TestSeriesLengthForcesBlackout confirms an unscrubbable leak is not merely
// scrubbed under the balanced policy.
func TestSeriesLengthForcesBlackout(t *testing.T) {
	m := match.Match{
		State:  match.StateFinished,
		BestOf: 3,
		VOD:    &match.VOD{Title: "[EN] LGD Gaming vs Team Yandex - Game 3 - The International 2026"},
	}
	got := Redact(m, config.SpoilerBalanced, false)
	if got.VOD.Title != "" {
		t.Errorf("balanced mode kept a series-length leak: %q", got.VOD.Title)
	}
	if !containsSignal(ScanVOD(m.VOD.Title, 3), SignalSeriesLength) {
		t.Error("series-length signal not recorded")
	}
}

// A finishing position states an outcome without using any of the verbs the
// outcome pattern looks for, and team pages carry them in infobox rows.
func TestPlacementSignal(t *testing.T) {
	leaks := []string{
		"1st place, The International 2026",
		"Runner-up at BLAST Slam VII",
		"runners up",
		"finished 4th",
		"2nd Place",
	}
	for _, in := range leaks {
		if !IsSpoilery(in) {
			t.Errorf("missed a placement leak: %q", in)
		}
	}
	// Ordinals are everywhere on a team page and must not trip on their own:
	// a Dota position is a bare number, and dates carry ordinals too.
	safe := []string{
		"Position 3",
		"Joined 1st January 2026",
		"3rd position",
		"21st Century",
	}
	for _, in := range safe {
		if IsSpoilery(in) {
			t.Errorf("false positive on %q: %v", in, Strings(Scan(in)))
		}
	}
}

// A founding date is not a scoreline. Team infoboxes carry dates like
// "2015-12-06", whose "12-06" is indistinguishable from "2-0" to the score
// pattern — which withheld the founding date of every team page.
func TestDatesAreNotScores(t *testing.T) {
	safe := []string{
		"2015-12-06",
		"Created: 2015-12-06",
		": 2015-12-06: 2016-06-09",
		"Joined 2023-11-11",
	}
	for _, in := range safe {
		if IsSpoilery(in) {
			t.Errorf("date treated as a result leak: %q -> %v", in, Strings(Scan(in)))
		}
	}
	// Stripping dates must not blunt the scoreline pattern itself.
	leaks := map[string]bool{
		"Spirit 2-0 Falcons":               true,
		"Spirit 2-0 Falcons on 2026-08-13": true,
		"16:14 on 2026-08-13":              true,
	}
	for in := range leaks {
		if !IsSpoilery(in) {
			t.Errorf("missed a scoreline: %q", in)
		}
	}
}
