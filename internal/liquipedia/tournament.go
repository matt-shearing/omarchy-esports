package liquipedia

import (
	"regexp"
	"strings"

	"golang.org/x/net/html"

	"github.com/contra/omarchy-esports/internal/match"
)

// Broadcast is what a tournament page tells us about how to watch an event.
type Broadcast struct {
	Streams []match.Stream
	// YouTubeChannels are channel ids harvested from the same broadcast table.
	// These are the feeds most likely to publish the event's VODs, which makes
	// the tournament page the bridge between "what is on" and "what can I
	// watch afterwards".
	YouTubeChannels []string
}

var (
	twitchURLRe    = regexp.MustCompile(`(?i)twitch\.tv/([A-Za-z0-9_]{3,40})`)
	ytChannelURLRe = regexp.MustCompile(`(?i)youtube\.com/channel/(UC[A-Za-z0-9_-]{20,30})`)
	ytHandleURLRe  = regexp.MustCompile(`(?i)youtube\.com/@([A-Za-z0-9_.-]{3,40})`)
	kickURLRe      = regexp.MustCompile(`(?i)kick\.com/([A-Za-z0-9_-]{3,40})`)
)

// ParseTournament extracts broadcast channels from a rendered tournament page.
//
// Liquipedia lists a tournament's broadcasters in an infobox and, for larger
// events, a per-language broadcast table. We harvest every external link that
// looks like a stream and de-duplicate, because the same channel is usually
// referenced several times on one page.
func ParseTournament(pageHTML string) (Broadcast, error) {
	var b Broadcast
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return b, err
	}

	seenStream := map[string]bool{}
	seenYT := map[string]bool{}
	addStream := func(s match.Stream) {
		if s.URL == "" || seenStream[s.URL] {
			return
		}
		seenStream[s.URL] = true
		b.Streams = append(b.Streams, s)
	}
	addYT := func(id string) {
		if id == "" || seenYT[id] {
			return
		}
		seenYT[id] = true
		b.YouTubeChannels = append(b.YouTubeChannels, id)
	}

	for _, a := range findAll(doc, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "a"
	}) {
		href := attr(a, "href")
		if href == "" {
			continue
		}

		// Liquipedia's own stream redirector, which also appears inline in
		// some tickers.
		if sm := streamHrefRe.FindStringSubmatch(href); sm != nil {
			addStream(NewStream(sm[1], sm[2]))
		}
		if sm := twitchURLRe.FindStringSubmatch(href); sm != nil {
			addStream(NewStream("twitch", sm[1]))
		}
		if sm := kickURLRe.FindStringSubmatch(href); sm != nil {
			addStream(NewStream("kick", sm[1]))
		}
		if sm := ytChannelURLRe.FindStringSubmatch(href); sm != nil {
			addStream(NewStream("youtube", sm[1]))
			addYT(sm[1])
		}
		if sm := ytHandleURLRe.FindStringSubmatch(href); sm != nil {
			// A handle is not a channel id, so it cannot seed an RSS feed, but
			// it still yields a watchable live URL.
			addStream(NewStream("youtube", sm[1]))
		}
	}
	return b, nil
}

// PreferredStream picks the best stream to surface on a match card: an
// English-language Twitch channel where possible, since that is what most
// viewers of an English UI want, falling back to anything watchable.
func PreferredStream(streams []match.Stream) *match.Stream {
	if len(streams) == 0 {
		return nil
	}
	var twitchEN, anyEN, anyTwitch *match.Stream
	for i := range streams {
		s := &streams[i]
		switch {
		case s.Platform == "twitch" && s.Language == "en" && twitchEN == nil:
			twitchEN = s
		case s.Language == "en" && anyEN == nil:
			anyEN = s
		case s.Platform == "twitch" && anyTwitch == nil:
			anyTwitch = s
		}
	}
	switch {
	case twitchEN != nil:
		return twitchEN
	case anyEN != nil:
		return anyEN
	case anyTwitch != nil:
		return anyTwitch
	}
	return &streams[0]
}
