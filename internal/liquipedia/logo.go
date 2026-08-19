package liquipedia

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/contra/omarchy-esports/internal/match"
)

// Team logos for teams that are not currently playing.
//
// Artwork normally arrives with a fixture: the match ticker embeds each team's
// logo. Teams known only from the wiki's team category therefore have none,
// which is most of the index.
//
// Fetching one is a single cheap `action=query` call. Asking for section 0 of
// the page returns just the infobox — about 2KB instead of the ~58KB full
// article — and `{{Infobox team}}` exposes `image=` and `imagedark=` with the
// same field names across every wiki checked. The file name is then turned
// into a URL locally, so no second round-trip is needed.

type revisionsResponse struct {
	Query struct {
		Pages map[string]struct {
			Title     string `json:"title"`
			Missing   any    `json:"missing"`
			Revisions []struct {
				Slots struct {
					Main struct {
						Content string `json:"*"`
					} `json:"main"`
				} `json:"slots"`
			} `json:"revisions"`
		} `json:"pages"`
	} `json:"query"`
}

var (
	imageFieldRe     = regexp.MustCompile(`(?im)^\s*\|\s*image\s*=\s*([^\n]+)`)
	imageDarkFieldRe = regexp.MustCompile(`(?im)^\s*\|\s*imagedark\s*=\s*([^\n]+)`)
)

// FetchLogo returns a team's artwork, or an empty Logo when the page has none.
func (c *Client) FetchLogo(ctx context.Context, wiki, team string) (match.Logo, error) {
	var logo match.Logo

	endpoint := fmt.Sprintf("https://liquipedia.net/%s/api.php?%s", url.PathEscape(wiki), url.Values{
		"action":    {"query"},
		"prop":      {"revisions"},
		"titles":    {team},
		"rvprop":    {"content"},
		"rvslots":   {"main"},
		"rvsection": {"0"}, // the infobox only
		"format":    {"json"},
	}.Encode())

	body, err := c.get(ctx, endpoint, false)
	if err != nil {
		return logo, err
	}
	var resp revisionsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return logo, fmt.Errorf("decoding infobox for %s/%s: %w", wiki, team, err)
	}

	for _, page := range resp.Query.Pages {
		if page.Missing != nil || len(page.Revisions) == 0 {
			continue
		}
		text := page.Revisions[0].Slots.Main.Content
		light := infoboxFile(imageFieldRe, text)
		dark := infoboxFile(imageDarkFieldRe, text)
		if light != "" {
			logo.Light = CommonsFileURL(light)
		}
		if dark != "" {
			logo.Dark = CommonsFileURL(dark)
		} else if light != "" {
			// A single "allmode" file serves both themes; Astralis ships only
			// one, for example.
			logo.Dark = logo.Light
		}
	}
	return logo, nil
}

// infoboxFile extracts a file name from an infobox field.
//
// Older infoboxes append an inline size to the value, as in
// `image=aspera_logo.png|10px`, so everything from the first pipe is dropped.
// Wiki comments and stray templates are trimmed the same way.
func infoboxFile(re *regexp.Regexp, text string) string {
	m := re.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	v := m[1]
	if i := strings.Index(v, "|"); i >= 0 {
		v = v[:i]
	}
	if i := strings.Index(v, "<!--"); i >= 0 {
		v = v[:i]
	}
	if i := strings.Index(v, "}}"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v)
}

// CommonsFileURL builds the URL of a file on Liquipedia's shared commons.
//
// MediaWiki stores uploads in hash buckets derived from the MD5 of the file
// name with spaces as underscores: the first hex digit is the outer directory
// and the first two are the inner one. This is standard MediaWiki layout
// rather than a documented Liquipedia contract, so a renamed file would break
// the link — the UI treats a logo that fails to load as simply absent, which
// is the same as having no artwork at all.
func CommonsFileURL(file string) string {
	file = strings.TrimSpace(strings.TrimPrefix(file, "File:"))
	if file == "" {
		return ""
	}
	name := strings.ReplaceAll(file, " ", "_")
	sum := md5.Sum([]byte(name))
	h := hex.EncodeToString(sum[:])
	return fmt.Sprintf("https://liquipedia.net/commons/images/%s/%s/%s",
		h[:1], h[:2], url.PathEscape(name))
}
