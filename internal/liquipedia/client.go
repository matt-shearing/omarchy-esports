// Package liquipedia talks to Liquipedia's MediaWiki API and, when an API key
// is configured, the structured LPDB v3 API.
//
// Liquipedia's API terms of use (https://liquipedia.net/api-terms-of-use)
// impose three requirements that this client enforces structurally rather than
// by convention, because violating them gets the user's IP banned:
//
//  1. Requests MUST be gzip-encoded. Omitting Accept-Encoding returns
//     "406 Gzip encoding is required for API requests".
//  2. Requests MUST carry a descriptive User-Agent naming the application and
//     a contact address.
//  3. action=parse is limited to one request per 30s (it is the expensive
//     action); all other actions to one per 2s. Responses must be cached.
//
// Note the direction of that last rule: it is easy to assume parse is the
// cheap one. It is not, and getting it backwards is exactly the kind of
// violation that earns an automated IP ban.
package liquipedia

import (
	"compress/gzip"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// parseInterval is Liquipedia's documented floor for action=parse, which
	// is rate-limited far harder than other actions because rendering a page
	// is expensive for them.
	parseInterval = 30 * time.Second
	// otherInterval is the floor for every other action.
	otherInterval = 2 * time.Second
	// userAgentTmpl embeds the contact address the terms of use require.
	userAgentTmpl = "omarchy-esports/%s (https://github.com/contra/omarchy-esports; %s) go-http"
)

// Client is a rate-limited, gzip-speaking Liquipedia API client.
//
// A single Client must be shared across all wikis: the rate limits are
// per-IP, not per-wiki, so independent clients would multiply the request
// rate by the number of games the user follows.
type Client struct {
	http     *http.Client
	ua       string
	cacheDir string

	mu        sync.Mutex
	lastParse time.Time
	lastOther time.Time
}

// New builds a Client. contact should be a working email or URL; version is
// embedded in the User-Agent.
func New(version, contact, cacheDir string) *Client {
	if strings.TrimSpace(contact) == "" {
		// Still identifies the software and gives maintainers somewhere to
		// look, which is better than a generic Go user agent.
		contact = "https://github.com/contra/omarchy-esports/issues"
	}
	return &Client{
		http:     &http.Client{Timeout: 45 * time.Second},
		ua:       fmt.Sprintf(userAgentTmpl, version, contact),
		cacheDir: cacheDir,
	}
}

// throttle blocks until the relevant rate limit allows another request.
func (c *Client) throttle(ctx context.Context, isParse bool) error {
	c.mu.Lock()
	var last *time.Time
	var interval time.Duration
	if isParse {
		last, interval = &c.lastParse, parseInterval
	} else {
		last, interval = &c.lastOther, otherInterval
	}
	wait := time.Until(last.Add(interval))
	// Reserve our slot before unlocking so concurrent callers queue up behind
	// us rather than all computing the same (already elapsed) wait.
	if wait < 0 {
		wait = 0
	}
	*last = time.Now().Add(wait)
	c.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
}

// get performs a rate-limited, gzip-encoded, identified GET.
func (c *Client) get(ctx context.Context, rawURL string, isParse bool) ([]byte, error) {
	if err := c.throttle(ctx, isParse); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.ua)
	// Set explicitly rather than relying on Go's transparent gzip, which only
	// applies under conditions we would rather not depend on. Liquipedia hard
	// -rejects requests without it.
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var reader io.Reader = resp.Body
	if strings.Contains(strings.ToLower(resp.Header.Get("Content-Encoding")), "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip: %w", err)
		}
		defer gz.Close()
		reader = gz
	}
	body, err := io.ReadAll(io.LimitReader(reader, 32<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(body)
		if len(snippet) > 300 {
			snippet = snippet[:300]
		}
		return nil, fmt.Errorf("liquipedia %s: %s", resp.Status, strings.TrimSpace(snippet))
	}
	return body, nil
}

// parseResponse is the shape of action=parse&prop=text.
type parseResponse struct {
	Parse struct {
		Title string `json:"title"`
		Text  struct {
			Star string `json:"*"`
		} `json:"text"`
	} `json:"parse"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

// ParsePage fetches the rendered HTML of a wiki page, using an on-disk cache.
// maxAge governs cache reuse; Liquipedia's terms require that we cache rather
// than re-fetch unchanged pages.
func (c *Client) ParsePage(ctx context.Context, wiki, page string, maxAge time.Duration) (string, error) {
	endpoint := fmt.Sprintf("https://liquipedia.net/%s/api.php?%s", url.PathEscape(wiki), url.Values{
		"action": {"parse"},
		"page":   {page},
		"format": {"json"},
		"prop":   {"text"},
	}.Encode())

	if html, ok := c.readCache(endpoint, maxAge); ok {
		return html, nil
	}

	body, err := c.get(ctx, endpoint, true)
	if err != nil {
		// Serve stale cache rather than failing outright: an offline laptop
		// should still show the schedule it last knew about.
		if html, ok := c.readCache(endpoint, 30*24*time.Hour); ok {
			return html, nil
		}
		return "", err
	}
	var pr parseResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return "", fmt.Errorf("decoding parse response for %s/%s: %w", wiki, page, err)
	}
	if pr.Error != nil {
		return "", fmt.Errorf("liquipedia %s/%s: %s: %s", wiki, page, pr.Error.Code, pr.Error.Info)
	}
	html := pr.Parse.Text.Star
	if strings.TrimSpace(html) == "" {
		return "", fmt.Errorf("liquipedia %s/%s: empty page content", wiki, page)
	}
	c.writeCache(endpoint, html)
	return html, nil
}

func (c *Client) cachePath(key string) string {
	sum := sha1.Sum([]byte(key))
	return filepath.Join(c.cacheDir, "pages", hex.EncodeToString(sum[:])+".html")
}

func (c *Client) readCache(key string, maxAge time.Duration) (string, bool) {
	if c.cacheDir == "" || maxAge <= 0 {
		return "", false
	}
	p := c.cachePath(key)
	fi, err := os.Stat(p)
	if err != nil || time.Since(fi.ModTime()) > maxAge {
		return "", false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", false
	}
	return string(data), true
}

func (c *Client) writeCache(key, html string) {
	if c.cacheDir == "" {
		return
	}
	p := c.cachePath(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, []byte(html), 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, p)
}

// AbsoluteURL turns a Liquipedia-relative href into an absolute URL.
func AbsoluteURL(href string) string {
	if href == "" || strings.HasPrefix(href, "http") {
		return href
	}
	return "https://liquipedia.net" + href
}
