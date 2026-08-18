package liquipedia

import (
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"

	"github.com/contra/omarchy-esports/internal/match"
)

// parseLegacyTicker handles the older table-based ticker, in which each match
// is one `table.infobox_matches_content` with a team-left / versus / team-right
// row followed by a match-filler row carrying the countdown, streams and
// tournament.
//
// No wiki we ship uses this by default, but several archived ticker pages
// still render it. Supporting it means a mis-pointed TickerPage degrades to
// partial data instead of silently returning nothing.
func parseLegacyTicker(doc *html.Node, wiki, game string) []match.Match {
	tables := findAllByClass(doc, "infobox_matches_content")
	out := make([]match.Match, 0, len(tables))
	for _, t := range tables {
		if m, ok := parseLegacyBlock(t, wiki, game); ok {
			out = append(out, m)
		}
	}
	return out
}

func parseLegacyBlock(t *html.Node, wiki, game string) (match.Match, bool) {
	var m match.Match
	m.Wiki = wiki
	m.Game = game

	timer := findByClass(t, "timer-object")
	if timer == nil {
		return m, false
	}
	ts, err := strconv.ParseInt(attr(timer, "data-timestamp"), 10, 64)
	if err != nil || ts <= 0 {
		return m, false
	}
	m.StartsAt = time.Unix(ts, 0).UTC()

	left, right := findByClass(t, "team-left"), findByClass(t, "team-right")
	if left == nil || right == nil {
		return m, false
	}
	m.Opponents[0] = parseLegacyOpponent(left, wiki)
	m.Opponents[1] = parseLegacyOpponent(right, wiki)
	if m.Opponents[0].Display() == "" || m.Opponents[1].Display() == "" {
		return m, false
	}

	if upper := findByClass(t, "versus-upper"); upper != nil {
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
	if lower := findByClass(t, "versus-lower"); lower != nil {
		if bm := bestOfRe.FindStringSubmatch(text(lower)); bm != nil {
			m.BestOf, _ = strconv.Atoi(bm[1])
		}
	}

	// The legacy layout puts the tournament in a plain link inside
	// .tournament-text-flex rather than a dedicated name element.
	if tf := findByClass(t, "tournament-text-flex"); tf != nil {
		if a := firstTag(tf, "a"); a != nil {
			m.Tournament.Name = strings.TrimSpace(attr(a, "title"))
			if m.Tournament.Name == "" {
				m.Tournament.Name = text(a)
			}
			m.Tournament.Page = attr(a, "href")
		}
	}
	m.Streams = parseStreams(t)

	m.ID = m.ComputeID()
	return m, true
}

// parseLegacyOpponent reads a legacy cell, which for 1v1 wikis holds a player
// link rather than a team template.
func parseLegacyOpponent(n *html.Node, wiki string) match.Opponent {
	var o match.Opponent
	if a := firstTag(n, "a"); a != nil {
		o.Name = redlinkTitleRe.ReplaceAllString(attr(a, "title"), "")
		o.Short = text(a)
		if href := attr(a, "href"); href != "" && !strings.Contains(href, "redlink=1") {
			o.Page = href
		}
	}
	if o.Name == "" {
		o.Name = strings.TrimSpace(text(n))
	}
	if o.Short == "" {
		o.Short = o.Name
	}
	o.Logo = parseLogo(n)
	return o
}
