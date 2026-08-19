package liquipedia

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Local logo cache.
//
// The UI used to point Image elements straight at liquipedia.net, which meant
// every panel open fired dozens of requests for artwork that never changes.
// Liquipedia started replying 429, and logos silently failed to render.
//
// Downloading each file once and serving it from disk fixes the rendering, and
// is the behaviour their terms ask for anyway: cache rather than re-request.
// It also means artwork still appears with no network.

// LogoCache stores team artwork on disk.
type LogoCache struct {
	dir  string
	http *http.Client

	mu sync.Mutex
	// pace serialises downloads so a first run does not burst.
	last time.Time
	// backoffUntil pauses all downloads after a 429. Artwork is decoration;
	// continuing to ask for it while being told to stop is both rude and
	// pointless, and the cache simply fills in on a later refresh.
	backoffUntil time.Time
}

// ErrBackoff reports that downloads are paused after a rate-limit response.
var ErrBackoff = errors.New("logo downloads paused after a 429")

// BackingOff reports whether downloads are currently paused.
func (c *LogoCache) BackingOff() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().Before(c.backoffUntil)
}

// NewLogoCache opens a cache rooted at dir.
func NewLogoCache(dir string) *LogoCache {
	return &LogoCache{
		dir:  filepath.Join(dir, "logos"),
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

// logoInterval paces downloads. Artwork is static and cached forever, so this
// only ever applies to files never seen before.
const logoInterval = 400 * time.Millisecond

// logoBackoff is how long to stop asking after a 429.
const logoBackoff = 30 * time.Minute

// Path returns the on-disk path for a remote URL, whether or not it exists.
func (c *LogoCache) Path(url string) string {
	sum := sha1.Sum([]byte(url))
	name := hex.EncodeToString(sum[:]) + extensionOf(url)
	return filepath.Join(c.dir, name)
}

func extensionOf(url string) string {
	if i := strings.LastIndex(url, "."); i >= 0 && len(url)-i <= 5 {
		ext := url[i:]
		if !strings.ContainsAny(ext, "/?&") {
			return ext
		}
	}
	return ".png"
}

// Has reports whether a URL is already cached.
func (c *LogoCache) Has(url string) bool {
	if url == "" {
		return false
	}
	fi, err := os.Stat(c.Path(url))
	return err == nil && fi.Size() > 0
}

// Fetch downloads a logo unless it is already cached, and returns its path.
func (c *LogoCache) Fetch(ctx context.Context, url, userAgent string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("empty logo url")
	}
	path := c.Path(url)
	if c.Has(url) {
		return path, nil
	}

	c.mu.Lock()
	if time.Now().Before(c.backoffUntil) {
		c.mu.Unlock()
		return "", ErrBackoff
	}
	wait := time.Until(c.last.Add(logoInterval))
	if wait < 0 {
		wait = 0
	}
	c.last = time.Now().Add(wait)
	c.mu.Unlock()
	if wait > 0 {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(wait):
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.mu.Lock()
		c.backoffUntil = time.Now().Add(logoBackoff)
		c.mu.Unlock()
		return "", ErrBackoff
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("logo %s: %s", url, resp.Status)
	}

	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return "", err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	// 4MB is generous for a team logo and bounds a misbehaving response.
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 4<<20)); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		return "", err
	}
	return path, nil
}

// UserAgent exposes the client's identifying header, so the logo cache
// presents the same identity as the API calls.
func (c *Client) UserAgent() string { return c.ua }
