package liquipedia

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixture loads a captured Liquipedia ticker page. These are real responses
// recorded from api.php, so the tests exercise the markup we actually receive
// rather than a hand-written approximation of it.
func fixture(t *testing.T, name string) string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name+".html.gz"))
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gunzip fixture: %v", err)
	}
	defer gz.Close()
	data, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	return string(data)
}

func TestParseTickerAllWikis(t *testing.T) {
	cases := []struct {
		fixture  string
		wiki     string
		game     string
		minCount int
	}{
		{"dota2", "dota2", "Dota 2", 40},
		{"counterstrike", "counterstrike", "Counter-Strike", 60},
		{"starcraft2", "starcraft2", "StarCraft II", 15},
	}
	for _, tc := range cases {
		t.Run(tc.wiki, func(t *testing.T) {
			matches, err := ParseTicker(fixture(t, tc.fixture), tc.wiki, tc.game)
			if err != nil {
				t.Fatalf("ParseTicker: %v", err)
			}
			if len(matches) < tc.minCount {
				t.Fatalf("got %d matches, want >= %d", len(matches), tc.minCount)
			}

			ids := map[string]bool{}
			for i, m := range matches {
				if m.StartsAt.IsZero() {
					t.Errorf("match %d: zero start time", i)
				}
				if m.Opponents[0].Display() == "" || m.Opponents[1].Display() == "" {
					t.Errorf("match %d: empty opponent: %+v", i, m.Opponents)
				}
				if m.Tournament.Name == "" {
					t.Errorf("match %d (%s): missing tournament", i, m.Title())
				}
				if m.ID == "" {
					t.Errorf("match %d: missing id", i)
				}
				if ids[m.ID] {
					t.Errorf("match %d: duplicate id %s for %s", i, m.ID, m.Title())
				}
				ids[m.ID] = true
				if m.Wiki != tc.wiki {
					t.Errorf("match %d: wiki = %q, want %q", i, m.Wiki, tc.wiki)
				}
			}

			// The ticker mixes recent results with upcoming fixtures, so a
			// healthy parse finds a spread of best-of formats.
			bo := map[int]int{}
			for _, m := range matches {
				bo[m.BestOf]++
			}
			if bo[0] == len(matches) {
				t.Errorf("no match had a parsed bestOf; format extraction is broken")
			}
			t.Logf("%s: %d matches, bestOf distribution %v", tc.wiki, len(matches), bo)
		})
	}
}

// TestParseTickerLogos guards the light/dark artwork extraction, which the UI
// relies on to follow the active omarchy theme.
func TestParseTickerLogos(t *testing.T) {
	matches, err := ParseTicker(fixture(t, "dota2"), "dota2", "Dota 2")
	if err != nil {
		t.Fatal(err)
	}
	withLogo, withDark := 0, 0
	for _, m := range matches {
		for _, o := range m.Opponents {
			if o.Logo.Light != "" || o.Logo.Dark != "" {
				withLogo++
			}
			if o.Logo.Dark != "" {
				withDark++
			}
		}
	}
	if withLogo == 0 {
		t.Fatal("no opponent logos parsed")
	}
	if withDark == 0 {
		t.Error("no darkmode logo variants parsed; theme-aware artwork will not work")
	}
	t.Logf("logos: %d opponents with art, %d with a dark variant", withLogo, withDark)
}

// TestParseLegacyTicker covers the archived table-based ticker format.
func TestParseLegacyTicker(t *testing.T) {
	matches, err := ParseTicker(fixture(t, "starcraft2-legacy"), "starcraft2", "StarCraft II")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) < 30 {
		t.Fatalf("legacy parse got %d matches, want >= 30", len(matches))
	}
	// The legacy layout is the one that carries inline stream links.
	streams := 0
	for _, m := range matches {
		streams += len(m.Streams)
	}
	if streams == 0 {
		t.Error("legacy ticker parsed no streams, but the format embeds them inline")
	}
	t.Logf("legacy: %d matches, %d streams", len(matches), streams)
}

// TestComputeIDStability ensures ids survive re-parsing, since reveal state
// and notification de-duplication are keyed on them.
func TestComputeIDStability(t *testing.T) {
	a, err := ParseTicker(fixture(t, "counterstrike"), "counterstrike", "Counter-Strike")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseTicker(fixture(t, "counterstrike"), "counterstrike", "Counter-Strike")
	if err != nil {
		t.Fatal(err)
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Fatalf("id unstable across parses: %s vs %s", a[i].ID, b[i].ID)
		}
	}
}

func TestNewStream(t *testing.T) {
	cases := []struct {
		platform, channel, wantURL, wantLang string
	}{
		{"twitch", "dota2ti", "https://www.twitch.tv/dota2ti", "en"},
		{"twitch", "dota2ti_ru", "https://www.twitch.tv/dota2tiru", "en"},
		{"youtube", "UCTQKT5QqO3h7y32G8VzuySQ", "https://www.youtube.com/channel/UCTQKT5QqO3h7y32G8VzuySQ/live", "en"},
		{"kick", "somechannel", "https://kick.com/somechannel", "en"},
		{"unknownplatform", "x", "", ""},
	}
	for _, tc := range cases {
		got := NewStream(tc.platform, tc.channel)
		if got.URL != tc.wantURL {
			t.Errorf("NewStream(%q,%q).URL = %q, want %q", tc.platform, tc.channel, got.URL, tc.wantURL)
		}
	}
}

func TestDeriveState(t *testing.T) {
	now := time.Date(2026, 8, 18, 20, 0, 0, 0, time.UTC)
	matches, err := ParseTicker(fixture(t, "dota2"), "dota2", "Dota 2")
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for i := range matches {
		counts[string(matches[i].DeriveState(now, 4*time.Hour))]++
	}
	if counts["upcoming"] == 0 {
		t.Error("expected some upcoming matches in the fixture")
	}
	t.Logf("state distribution at %s: %v", now.Format(time.RFC3339), counts)
}
