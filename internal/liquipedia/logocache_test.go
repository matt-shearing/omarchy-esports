package liquipedia

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

// TestLogoCacheStoresAndReuses covers the whole point of the cache: a file is
// fetched once and then served from disk.
func TestLogoCacheStoresAndReuses(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Write([]byte("\x89PNG fake"))
	}))
	defer srv.Close()

	c := NewLogoCache(t.TempDir())
	url := srv.URL + "/Team_Logo.png"

	path, err := c.Fetch(context.Background(), url, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("logo not written: %v", err)
	}
	if !c.Has(url) {
		t.Error("Has should report a cached file")
	}

	if _, err := c.Fetch(context.Background(), url, "test"); err != nil {
		t.Fatal(err)
	}
	if hits != 1 {
		t.Errorf("server hit %d times, want 1 — the cache is not being reused", hits)
	}
}

// TestLogoCacheBacksOffOn429 is the behaviour that matters when Liquipedia
// says stop: further downloads pause rather than hammering.
func TestLogoCacheBacksOffOn429(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := NewLogoCache(t.TempDir())

	_, err := c.Fetch(context.Background(), srv.URL+"/a.png", "test")
	if !errors.Is(err, ErrBackoff) {
		t.Fatalf("first 429 gave %v, want ErrBackoff", err)
	}
	if !c.BackingOff() {
		t.Error("cache should be backing off after a 429")
	}

	// A second request must not reach the server at all.
	_, err = c.Fetch(context.Background(), srv.URL+"/b.png", "test")
	if !errors.Is(err, ErrBackoff) {
		t.Errorf("second attempt gave %v, want ErrBackoff", err)
	}
	if hits != 1 {
		t.Errorf("server hit %d times, want 1 — backoff is not preventing requests", hits)
	}
}

func TestLogoCachePathIsStable(t *testing.T) {
	c := NewLogoCache(t.TempDir())
	url := "https://liquipedia.net/commons/images/a/a1/Team.png"
	if c.Path(url) != c.Path(url) {
		t.Error("path should be deterministic")
	}
	if c.Path(url) == c.Path(url+"x") {
		t.Error("different URLs must not collide")
	}
	if got := c.Path(url); got[len(got)-4:] != ".png" {
		t.Errorf("extension not preserved: %s", got)
	}
}

// TestCanonicalLogoURL collapses per-size thumbnails onto one cache entry.
func TestCanonicalLogoURL(t *testing.T) {
	const base = "https://liquipedia.net/commons/images"
	cases := []struct{ in, want string }{
		// Different rendered widths of the same logo must collapse.
		{base + "/thumb/a/a2/Luminosity_Gaming_2018_allmode.png/50px-Luminosity_Gaming_2018_allmode.png",
			base + "/thumb/a/a2/Luminosity_Gaming_2018_allmode.png/128px-Luminosity_Gaming_2018_allmode.png"},
		{base + "/thumb/a/a2/Luminosity_Gaming_2018_allmode.png/35px-Luminosity_Gaming_2018_allmode.png",
			base + "/thumb/a/a2/Luminosity_Gaming_2018_allmode.png/128px-Luminosity_Gaming_2018_allmode.png"},
		// The original resolves to the same canonical thumbnail.
		{base + "/a/a2/Luminosity_Gaming_2018_allmode.png",
			base + "/thumb/a/a2/Luminosity_Gaming_2018_allmode.png/128px-Luminosity_Gaming_2018_allmode.png"},
		// Anything else is left alone.
		{"https://example.com/logo.png", "https://example.com/logo.png"},
		{"", ""},
	}
	for _, c := range cases {
		if got := CanonicalLogoURL(c.in); got != c.want {
			t.Errorf("CanonicalLogoURL(%q)\n got %s\nwant %s", c.in, got, c.want)
		}
	}

	// The whole point: two renderings share one cache entry.
	c := NewLogoCache(t.TempDir())
	a := c.Path(CanonicalLogoURL(base + "/thumb/a/a2/X.png/50px-X.png"))
	b := c.Path(CanonicalLogoURL(base + "/thumb/a/a2/X.png/35px-X.png"))
	if a != b {
		t.Errorf("two sizes of one logo mapped to different cache files:\n %s\n %s", a, b)
	}
}

// TestBackoffSurvivesRestore covers persistence: a restart must not resume
// hammering an endpoint that just asked us to stop.
func TestBackoffSurvivesRestore(t *testing.T) {
	c := NewLogoCache(t.TempDir())
	if c.BackingOff() {
		t.Fatal("a fresh cache should not be backing off")
	}
	until := time.Now().Add(20 * time.Minute)
	c.SetBackoffUntil(until)
	if !c.BackingOff() {
		t.Error("restored backoff should be in effect")
	}
	// An earlier time must not shorten an active pause.
	c.SetBackoffUntil(time.Now().Add(time.Minute))
	if got := c.BackoffUntil(); !got.Equal(until) {
		t.Errorf("backoff shortened to %s, want %s", got, until)
	}
}

// TestClientRateLimitBackoff: a 429 from the API must pause all traffic, and
// the pause must be restorable so a restart does not resume hammering.
func TestClientRateLimitBackoff(t *testing.T) {
	c := New("test", "test@example.com", t.TempDir())
	if c.RateLimited() {
		t.Fatal("a fresh client should not be paused")
	}
	until := time.Now().Add(10 * time.Minute)
	c.SetRateLimitedUntil(until)
	if !c.RateLimited() {
		t.Error("restored pause should be in effect")
	}
	// An earlier time must not shorten an active pause.
	c.SetRateLimitedUntil(time.Now().Add(time.Minute))
	if got := c.RateLimitedUntil(); !got.Equal(until) {
		t.Errorf("pause shortened to %s, want %s", got, until)
	}
}

// TestOriginalOf supports the 404 fallback: MediaWiki refuses to upscale, so a
// 128px request for a narrower original must retry the original file.
func TestOriginalOf(t *testing.T) {
	const base = "https://liquipedia.net/commons/images"
	got := originalOf(base + "/thumb/a/a2/Team_X.png/128px-Team_X.png")
	want := base + "/a/a2/Team_X.png"
	if got != want {
		t.Errorf("originalOf\n got %s\nwant %s", got, want)
	}
	if originalOf("https://example.com/logo.png") != "" {
		t.Error("a non-thumbnail URL has no original to fall back to")
	}
}
