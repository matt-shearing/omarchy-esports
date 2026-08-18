package liquipedia

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/contra/omarchy-esports/internal/match"
)

// ParseTicker extracts matches from a rendered Liquipedia ticker page.
//
// Every wiki we support (dota2, counterstrike, starcraft2) renders the modern
// "match-info" ticker component, though they host it on different pages. An
// older table-based ticker still exists on some archived pages; ParseTicker
// falls back to it so a mis-pointed TickerPage degrades rather than returning
// nothing.
func ParseTicker(pageHTML, wiki, game string) ([]match.Match, error) {
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return nil, err
	}
	blocks := findAllByClass(doc, "match-info")
	out := make([]match.Match, 0, len(blocks))
	for _, b := range blocks {
		if m, ok := parseModernBlock(b, wiki, game); ok {
			out = append(out, m)
		}
	}
	if len(out) == 0 {
		out = parseLegacyTicker(doc, wiki, game)
	}
	return out, nil
}

var bestOfRe = regexp.MustCompile(`(?i)bo\s*(\d+)`)
var scoreRe = regexp.MustCompile(`^(\d+)\s*[:\-–]\s*(\d+)$`)

func parseModernBlock(b *html.Node, wiki, game string) (match.Match, bool) {
	var m match.Match
	m.Wiki = wiki
	m.Game = game

	timer := findByClass(b, "timer-object")
	if timer == nil {
		return m, false
	}
	tsRaw := attr(timer, "data-timestamp")
	ts, err := strconv.ParseInt(tsRaw, 10, 64)
	if err != nil || ts <= 0 {
		return m, false
	}
	m.StartsAt = time.Unix(ts, 0).UTC()

	opps := findAllByClass(b, "match-info-header-opponent")
	if len(opps) < 2 {
		return m, false
	}
	m.Opponents[0] = parseOpponent(opps[0], wiki)
	m.Opponents[1] = parseOpponent(opps[1], wiki)
	if m.Opponents[0].Display() == "" || m.Opponents[1].Display() == "" {
		return m, false
	}

	// The score holder shows "vs" before a match and the series score after.
	if upper := findByClass(b, "match-info-header-scoreholder-upper"); upper != nil {
		if sm := scoreRe.FindStringSubmatch(strings.TrimSpace(text(upper))); sm != nil {
			m.Score[0], _ = strconv.Atoi(sm[1])
			m.Score[1], _ = strconv.Atoi(sm[2])
			switch {
			case m.Score[0] > m.Score[1]:
				m.Winner = 1
			case m.Score[1] > m.Score[0]:
				m.Winner = 2
			}
		}
	}
	if lower := findByClass(b, "match-info-header-scoreholder-lower"); lower != nil {
		if bm := bestOfRe.FindStringSubmatch(text(lower)); bm != nil {
			m.BestOf, _ = strconv.Atoi(bm[1])
		}
	}

	m.Tournament = parseTournament(b)
	m.Streams = parseStreams(b)

	m.ID = m.ComputeID()
	return m, true
}

// redlinkTitleRe strips MediaWiki's "(page does not exist)" suffix, which the
// ticker emits for teams that have no wiki page yet.
var redlinkTitleRe = regexp.MustCompile(`\s*\(page does not exist\)\s*$`)

func parseOpponent(n *html.Node, wiki string) match.Opponent {
	var o match.Opponent

	// The canonical name lives in the title attribute of the team link. Prefer
	// the logo anchor, which points at the real page even when the name span
	// has been rendered as a redlink.
	for _, a := range findAll(n, func(c *html.Node) bool {
		return c.Type == html.ElementNode && c.Data == "a"
	}) {
		title := redlinkTitleRe.ReplaceAllString(attr(a, "title"), "")
		href := attr(a, "href")
		if title == "" {
			continue
		}
		if o.Name == "" {
			o.Name = title
		}
		// A redlink href is an edit URL; recover the page name from its query
		// rather than storing a link that would open the wiki editor.
		if strings.Contains(href, "redlink=1") {
			if u, err := url.Parse(href); err == nil {
				if t := u.Query().Get("title"); t != "" && o.Page == "" {
					o.Page = "/" + wiki + "/" + strings.ReplaceAll(t, " ", "_")
				}
			}
			continue
		}
		if o.Page == "" && href != "" {
			o.Page = href
		}
	}

	if name := findByClass(n, "name"); name != nil {
		o.Short = text(name)
	}
	if o.Short == "" {
		o.Short = o.Name
	}
	if o.Name == "" {
		o.Name = o.Short
	}

	o.Logo = parseLogo(n)
	return o
}

// parseLogo pulls the light and dark artwork variants. Liquipedia emits either
// a single "allmode" icon or a lightmode/darkmode pair; we keep whichever
// exists so the UI can follow the active omarchy theme.
func parseLogo(n *html.Node) match.Logo {
	var l match.Logo
	for _, span := range findAllByClass(n, "team-template-image-icon") {
		img := firstTag(span, "img")
		if img == nil {
			continue
		}
		src := AbsoluteURL(attr(img, "src"))
		if src == "" {
			continue
		}
		switch {
		case hasClass(span, "team-template-darkmode"):
			l.Dark = src
		case hasClass(span, "team-template-lightmode"):
			l.Light = src
		default:
			if l.Light == "" {
				l.Light = src
			}
		}
	}
	return l
}

func parseTournament(b *html.Node) match.Tournament {
	var t match.Tournament
	if nameNode := findByClass(b, "match-info-tournament-name"); nameNode != nil {
		t.Name = text(nameNode)
		if a := firstTag(nameNode, "a"); a != nil {
			t.Page = attr(a, "href")
		}
	}
	if icon := findByClass(b, "league-icon-small-image"); icon != nil {
		if img := firstTag(icon, "img"); img != nil {
			t.Icon = AbsoluteURL(attr(img, "src"))
		}
		if t.Page == "" {
			if a := firstTag(icon, "a"); a != nil {
				t.Page = attr(a, "href")
			}
		}
	}
	// Strip the fragment so the same event does not appear as several
	// tournaments purely because matches link to different sections.
	if i := strings.Index(t.Page, "#"); i >= 0 {
		t.Page = t.Page[:i]
	}
	return t
}

// streamHrefRe matches Liquipedia's stream redirector links, which appear
// inline in the ticker for some wikis.
var streamHrefRe = regexp.MustCompile(`/Special:Stream/([a-z0-9]+)/([^"'?#]+)`)

func parseStreams(b *html.Node) []match.Stream {
	var out []match.Stream
	seen := map[string]bool{}
	for _, a := range findAll(b, func(c *html.Node) bool {
		return c.Type == html.ElementNode && c.Data == "a"
	}) {
		sm := streamHrefRe.FindStringSubmatch(attr(a, "href"))
		if sm == nil {
			continue
		}
		s := NewStream(sm[1], sm[2])
		if s.URL == "" || seen[s.URL] {
			continue
		}
		seen[s.URL] = true
		out = append(out, s)
	}
	return out
}

// NewStream builds a Stream from a platform and channel, deriving a watchable
// URL and guessing the broadcast language from channel-name suffixes.
func NewStream(platform, channel string) match.Stream {
	channel = strings.TrimSpace(channel)
	// Liquipedia stream pages use underscores for spaces in channel names.
	channel = strings.ReplaceAll(channel, "_", "")
	platform = strings.ToLower(strings.TrimSpace(platform))
	s := match.Stream{Platform: platform, Channel: channel}
	switch platform {
	case "twitch":
		s.URL = "https://www.twitch.tv/" + channel
	case "youtube":
		if strings.HasPrefix(channel, "UC") && len(channel) > 20 {
			s.URL = "https://www.youtube.com/channel/" + channel + "/live"
		} else {
			s.URL = "https://www.youtube.com/@" + channel + "/live"
		}
	case "kick":
		s.URL = "https://kick.com/" + channel
	default:
		return match.Stream{}
	}
	s.Language = guessLanguage(channel)
	s.Primary = s.Language == "en"
	return s
}

// langSuffixes maps the channel-name conventions Liquipedia broadcasters use
// to language codes, so the UI can prefer an English feed.
var langSuffixes = map[string]string{
	"ru": "ru", "es": "es", "cn": "zh", "zh": "zh", "br": "pt", "pt": "pt",
	"de": "de", "fr": "fr", "pl": "pl", "it": "it", "tr": "tr", "kr": "ko",
	"ko": "ko", "jp": "ja", "ja": "ja", "ar": "ar", "ua": "uk", "uk": "uk",
	"vn": "vi", "th": "th", "id": "id", "cz": "cs", "nl": "nl", "se": "sv",
}

func guessLanguage(channel string) string {
	c := strings.ToLower(channel)
	for suf, lang := range langSuffixes {
		if strings.HasSuffix(c, "_"+suf) || strings.HasSuffix(c, "-"+suf) || strings.HasSuffix(c, suf) && len(c) > len(suf)+3 {
			// Require a separator or a reasonably long stem so "esl" is not
			// read as Slovenian and "navi" not as Vietnamese.
			if strings.HasSuffix(c, "_"+suf) || strings.HasSuffix(c, "-"+suf) {
				return lang
			}
		}
	}
	return "en"
}
