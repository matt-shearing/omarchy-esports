package liquipedia

import "testing"

// TestParseTournament runs against a real capture of The International 2026
// main event page, which carries a full multi-language broadcast table.
func TestParseTournament(t *testing.T) {
	b, err := ParseTournament(fixture(t, "tournament-ti2026"))
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Streams) == 0 {
		t.Fatal("no streams parsed from a tournament page that lists several")
	}
	if len(b.YouTubeChannels) == 0 {
		t.Error("no YouTube channel ids parsed; VOD discovery depends on these")
	}

	var twitch int
	for _, s := range b.Streams {
		if s.Platform == "twitch" {
			twitch++
		}
		if s.URL == "" {
			t.Errorf("stream with empty URL: %+v", s)
		}
	}
	if twitch == 0 {
		t.Error("expected Twitch channels on this event")
	}

	if p := PreferredStream(b.Streams); p == nil || p.URL == "" {
		t.Error("PreferredStream returned nothing watchable")
	} else {
		t.Logf("preferred: %s %s", p.Platform, p.URL)
	}
	t.Logf("%d streams (%d twitch), %d youtube channels", len(b.Streams), twitch, len(b.YouTubeChannels))
	for _, c := range b.YouTubeChannels {
		t.Logf("  yt channel: %s", c)
	}
}
