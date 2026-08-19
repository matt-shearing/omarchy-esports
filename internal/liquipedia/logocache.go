package liquipedia

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
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
	dir string
	// seed is an optional read-only directory of pre-supplied artwork,
	// consulted before the network. It lets a build ship or side-load a logo
	// pack so a cold install shows real logos immediately, and it rescues
	// users on networks where Liquipedia's image hosts are unreachable or
	// rate limited.
	seed string
	http *http.Client

	mu sync.Mutex
	// pace serialises downloads so a first run does not burst.
	last time.Time
	// backoffUntil pauses downloads PER HOST after a 429. Artwork is
	// decoration; continuing to ask while being told to stop is both rude and
	// pointless, and the cache fills in on a later refresh.
	//
	// Per host, not global: a 429 from liquipedia.net says nothing about an
	// org's own CDN, and a single global pause meant one refusal blocked every
	// other source too.
	backoffUntil map[string]time.Time
}

// hostOf extracts the host for backoff bookkeeping.
func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return rawURL
	}
	return u.Host
}

// ErrBackoff reports that downloads are paused after a rate-limit response.
var ErrBackoff = errors.New("logo downloads paused after a 429")

// BackingOff reports whether every known host is currently paused. It is a
// coarse check used to skip an artwork sweep entirely.
func (c *LogoCache) BackingOff() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.backoffUntil) == 0 {
		return false
	}
	now := time.Now()
	for _, t := range c.backoffUntil {
		if now.After(t) {
			return false
		}
	}
	return true
}

// BackingOffHost reports whether one host is paused.
func (c *LogoCache) BackingOffHost(rawURL string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().Before(c.backoffUntil[hostOf(rawURL)])
}

// BackoffUntil returns the furthest pause across hosts, for persistence.
func (c *LogoCache) BackoffUntil() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	var latest time.Time
	for _, t := range c.backoffUntil {
		if t.After(latest) {
			latest = t
		}
	}
	return latest
}

// SetBackoffUntil restores a persisted pause for Liquipedia, which is the only
// host whose pause is worth carrying across restarts.
func (c *LogoCache) SetBackoffUntil(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	const h = "liquipedia.net"
	if t.After(c.backoffUntil[h]) {
		c.backoffUntil[h] = t
	}
}

// NewLogoCache opens a cache rooted at dir.
func NewLogoCache(dir string) *LogoCache {
	return &LogoCache{
		dir:          filepath.Join(dir, "logos"),
		seed:         seedDir(),
		http:         &http.Client{Timeout: 30 * time.Second},
		backoffUntil: map[string]time.Time{},
	}
}

// seedDir locates a pre-supplied logo pack, if one is installed.
func seedDir() string {
	if d := os.Getenv("OMARCHY_ESPORTS_LOGO_PACK"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "omarchy-esports", "logos")
}

// SeedPath returns the pack location, for diagnostics.
func (c *LogoCache) SeedPath() string { return c.seed }

// logoInterval paces downloads. Artwork is static and cached forever, so this
// only ever applies to files never seen before.
const logoInterval = 400 * time.Millisecond

// logoBackoff is how long to stop asking after a 429.
const logoBackoff = 30 * time.Minute

// Path returns the on-disk path for a remote URL, whether or not it exists.
func (c *LogoCache) Path(rawURL string) string {
	sum := sha1.Sum([]byte(rawURL))
	name := hex.EncodeToString(sum[:]) + extensionOf(rawURL)
	return filepath.Join(c.dir, name)
}

func extensionOf(rawURL string) string {
	if i := strings.LastIndex(rawURL, "."); i >= 0 && len(rawURL)-i <= 5 {
		ext := rawURL[i:]
		if !strings.ContainsAny(ext, "/?&") {
			return ext
		}
	}
	return ".png"
}

// Has reports whether a URL is available locally, from the cache or a pack.
func (c *LogoCache) Has(rawURL string) bool {
	return c.Resolve(rawURL) != ""
}

// Resolve returns the local file backing a URL, preferring the downloaded
// cache and falling back to a supplied pack. Empty when neither has it.
func (c *LogoCache) Resolve(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	if p := c.Path(rawURL); fileHasContent(p) {
		return p
	}
	if c.seed != "" {
		if p := filepath.Join(c.seed, filepath.Base(c.Path(rawURL))); fileHasContent(p) {
			return p
		}
	}
	return ""
}

func fileHasContent(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Size() > 0
}

// Fetch downloads a logo unless it is already cached, and returns its path.
func (c *LogoCache) Fetch(ctx context.Context, rawURL, userAgent string) (string, error) {
	if rawURL == "" {
		return "", fmt.Errorf("empty logo url")
	}
	if local := c.Resolve(rawURL); local != "" {
		return local, nil
	}
	path := c.Path(rawURL)

	c.mu.Lock()
	if time.Now().Before(c.backoffUntil[hostOf(rawURL)]) {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
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
		c.backoffUntil[hostOf(rawURL)] = time.Now().Add(logoBackoff)
		c.mu.Unlock()
		return "", ErrBackoff
	}
	if resp.StatusCode == http.StatusNotFound {
		// MediaWiki will not upscale: asking for a 128px thumbnail of an
		// original narrower than that 404s. Fall back to the original file,
		// which is small anyway for such logos.
		if orig := originalOf(rawURL); orig != "" && orig != rawURL {
			return c.Fetch(ctx, orig, userAgent)
		}
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("logo %s: %s", rawURL, resp.Status)
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

// Canonical thumbnail size. Every logo is cached once at this width and scaled
// locally, so a team appearing at 35px in one fixture and 50px in another
// costs one download rather than two.
//
// 128px is chosen over the original file because originals are full-resolution
// artwork — the League of Legends mark is 610KB — while a 128px thumbnail is a
// few kilobytes and is still sharp at every size this UI draws.
const CanonicalLogoWidth = 128

var thumbPathRe = regexp.MustCompile(`^(https?://[^/]+/commons/images)/thumb/([0-9a-f])/([0-9a-f]{2})/(.+?)/\d+px-[^/]+$`)
var originalPathRe = regexp.MustCompile(`^(https?://[^/]+/commons/images)/([0-9a-f])/([0-9a-f]{2})/([^/]+)$`)

// originalOf turns a commons thumbnail URL back into the original file.
func originalOf(rawURL string) string {
	m := thumbPathRe.FindStringSubmatch(rawURL)
	if m == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s/%s", m[1], m[2], m[3], m[4])
}

// CanonicalLogoURL rewrites any commons image URL — a thumbnail of any width,
// or the original — to one fixed-width thumbnail.
//
// Liquipedia hands out per-size thumbnail URLs, so the same logo appears under
// several URLs depending on how wide the ticker drew it. Collapsing them means
// the cache key is the logo, not the rendering.
func CanonicalLogoURL(url string) string {
	if m := thumbPathRe.FindStringSubmatch(url); m != nil {
		return fmt.Sprintf("%s/thumb/%s/%s/%s/%dpx-%s",
			m[1], m[2], m[3], m[4], CanonicalLogoWidth, m[4])
	}
	if m := originalPathRe.FindStringSubmatch(url); m != nil {
		return fmt.Sprintf("%s/thumb/%s/%s/%s/%dpx-%s",
			m[1], m[2], m[3], m[4], CanonicalLogoWidth, m[4])
	}
	// Not a commons image path; leave it alone.
	return url
}
