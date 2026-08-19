package liquipedia

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
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
