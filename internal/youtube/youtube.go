// Package youtube discovers match VODs without an API key.
//
// The YouTube Data API charges 100 quota units per search.list call against a
// 10,000-unit daily budget, which makes result-scanning across several esports
// channels expensive and requires the user to obtain and paste a key. The
// per-channel RSS feed at /feeds/videos.xml needs no key, no quota and no
// setup, and carries everything we need: video id, title, publish time and
// thumbnail for the 15 most recent uploads.
//
// The cost is that we only see recent uploads, which suits a tool that tracks
// matches as they happen.
package youtube

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/contra/omarchy-esports/internal/match"
)

// FeedURL builds the keyless RSS feed URL for a channel id.
func FeedURL(channelID string) string {
	return "https://www.youtube.com/feeds/videos.xml?channel_id=" + channelID
}

// UploadsFeedURL builds the feed for a channel's uploads playlist. Every
// channel id of the form UC... has a matching uploads playlist UU..., so
// swapping the prefix gives a second, equivalent feed with no API call. It is
// a useful fallback when the channel feed is empty or throttled.
func UploadsFeedURL(channelID string) string {
	if !strings.HasPrefix(channelID, "UC") {
		return ""
	}
	return "https://www.youtube.com/feeds/videos.xml?playlist_id=UU" + strings.TrimPrefix(channelID, "UC")
}

// Video is one uploaded video.
type Video struct {
	ID        string
	Title     string
	Published time.Time
	Thumbnail string
	Channel   string
	// Lang is the broadcast language parsed from a leading tag such as "[EN]".
	Lang string
}

// feed mirrors the Atom document YouTube serves.
type feed struct {
	Title   string      `xml:"title"`
	Entries []feedEntry `xml:"entry"`
}

type feedEntry struct {
	VideoID   string    `xml:"videoId"`
	Title     string    `xml:"title"`
	Published time.Time `xml:"published"`
	Group     struct {
		Thumbnail struct {
			URL string `xml:"url,attr"`
		} `xml:"thumbnail"`
	} `xml:"group"`
}

// Client fetches and caches channel feeds.
type Client struct {
	http *http.Client

	mu    sync.Mutex
	cache map[string]cached
	// ttl bounds how often a given feed is refetched.
	ttl time.Duration
}

type cached struct {
	at     time.Time
	videos []Video
}

// New builds a Client. ttl controls feed cache lifetime.
func New(ttl time.Duration) *Client {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &Client{
		http:  &http.Client{Timeout: 30 * time.Second},
		cache: map[string]cached{},
		ttl:   ttl,
	}
}

// langTagRe captures the leading language marker official broadcasters use,
// e.g. "[EN] Team A vs Team B" or "[EN-A]" for an alternate English feed.
var langTagRe = regexp.MustCompile(`^\s*\[([A-Za-z]{2}(?:-[A-Za-z])?)\]\s*`)

// Videos returns recent uploads for a channel, using the cache when fresh.
func (c *Client) Videos(ctx context.Context, channelID string) ([]Video, error) {
	c.mu.Lock()
	if hit, ok := c.cache[channelID]; ok && time.Since(hit.at) < c.ttl {
		c.mu.Unlock()
		return hit.videos, nil
	}
	c.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, FeedURL(channelID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "omarchy-esports (+https://github.com/contra/omarchy-esports)")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("youtube feed %s: %s", channelID, resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}

	var f feed
	if err := xml.Unmarshal(body, &f); err != nil {
		return nil, fmt.Errorf("parsing feed %s: %w", channelID, err)
	}

	videos := make([]Video, 0, len(f.Entries))
	for _, e := range f.Entries {
		if e.VideoID == "" {
			continue
		}
		v := Video{
			ID:        e.VideoID,
			Title:     e.Title,
			Published: e.Published,
			Thumbnail: e.Group.Thumbnail.URL,
			Channel:   f.Title,
			Lang:      "en",
		}
		if m := langTagRe.FindStringSubmatch(e.Title); m != nil {
			v.Lang = strings.ToLower(strings.SplitN(m[1], "-", 2)[0])
		}
		videos = append(videos, v)
	}

	c.mu.Lock()
	c.cache[channelID] = cached{at: time.Now(), videos: videos}
	c.mu.Unlock()
	return videos, nil
}

// URL returns the watch URL for a video id.
func URL(videoID string) string { return "https://www.youtube.com/watch?v=" + videoID }

// nonAlnum strips punctuation so "PSG.LGD" and "PSG LGD" compare equal.
var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// tokens reduces a name to comparable lowercase tokens, dropping the corporate
// filler words that appear inconsistently between Liquipedia and VOD titles.
func tokens(name string) []string {
	name = nonAlnum.ReplaceAllString(strings.ToLower(name), " ")
	var out []string
	for _, f := range strings.Fields(name) {
		switch f {
		case "team", "esports", "esport", "gaming", "club", "the", "org":
			continue
		}
		out = append(out, f)
	}
	if len(out) == 0 {
		// A name made entirely of filler still needs something to match on.
		out = strings.Fields(nonAlnum.ReplaceAllString(strings.ToLower(name), " "))
	}
	return out
}

// mentions reports whether a title refers to an opponent, comparing against
// both the canonical name and the ticker abbreviation.
func mentions(title string, o match.Opponent) bool {
	lt := " " + nonAlnum.ReplaceAllString(strings.ToLower(title), " ") + " "
	for _, candidate := range []string{o.Name, o.Short} {
		toks := tokens(candidate)
		if len(toks) == 0 {
			continue
		}
		all := true
		for _, tok := range toks {
			// Require a whole-token match so "OG" does not match "Gaming".
			if !strings.Contains(lt, " "+tok+" ") {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// highlightRe identifies cut-down uploads. Branded org channels increasingly
// post clips and highlight reels rather than full matches, and a highlights
// package is both the wrong video for someone wanting to watch the series and
// far more likely to give the result away in its title and artwork.
var highlightRe = regexp.MustCompile(`(?i)\b(highlights?|best of|top ?plays|recap|clips?|shorts?|in \d+ minutes|montage|funniest|plays of the)\b`)

// IsHighlight reports whether a title looks like a cut-down upload rather
// than a full match VOD.
func IsHighlight(title string) bool { return highlightRe.MatchString(title) }

// gameNumberRe finds "Game 3" / "Map 2" style markers.
var gameNumberRe = regexp.MustCompile(`(?i)\b(?:game|map|g)\s*([2-9])\b`)

// Match finds the best VOD for a match among candidate videos.
//
// A video qualifies when it names both opponents and was published in the
// window running from shortly before the match start to maxAge afterwards.
// Among qualifying videos we prefer the configured language, then the earliest
// upload, which is typically the full match rather than a later highlight cut.
func Match(m match.Match, videos []Video, preferLang string, maxAge time.Duration) *match.VOD {
	if preferLang == "" {
		preferLang = "en"
	}
	var best *Video
	var bestScore int

	for i := range videos {
		v := &videos[i]
		if v.Published.Before(m.StartsAt.Add(-2*time.Hour)) || v.Published.After(m.StartsAt.Add(maxAge)) {
			continue
		}
		if !mentions(v.Title, m.Opponents[0]) || !mentions(v.Title, m.Opponents[1]) {
			continue
		}
		score := 1
		if v.Lang == preferLang {
			score += 10
		}
		// Prefer a full match over a highlights package.
		if !IsHighlight(v.Title) {
			score += 20
		}
		// Prefer the earliest qualifying upload of the preferred language.
		if best == nil || score > bestScore ||
			(score == bestScore && v.Published.Before(best.Published)) {
			best, bestScore = v, score
		}
	}
	if best == nil {
		return nil
	}
	kind := "full"
	if IsHighlight(best.Title) {
		kind = "highlights"
	}
	return &match.VOD{
		Kind:      kind,
		VideoID:   best.ID,
		Title:     best.Title,
		URL:       URL(best.ID),
		Channel:   best.Channel,
		Published: best.Published,
		Thumbnail: best.Thumbnail,
	}
}

// GameNumberLeak reports the highest game number named in a title. This is a
// spoiler that scoring regexes miss: "Game 3" of a best-of-three tells you the
// series was tied 1-1, and "Game 5" of a best-of-five tells you it was 2-2.
func GameNumberLeak(title string) int {
	m := gameNumberRe.FindStringSubmatch(title)
	if m == nil {
		return 0
	}
	n := 0
	fmt.Sscanf(m[1], "%d", &n)
	return n
}
