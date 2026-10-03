package logosource

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"
	"sync"
)

// Game artwork.
//
// Same rule as team logos, for the same reasons: this ships a map of URLs, not
// images. A game's logo is the publisher's trademark, so the file is fetched to
// each user's own machine and cached there, and nothing is redistributed with
// the project.
//
// Keyed by the catalog's wiki slug rather than a display name, because the slug
// is the stable identifier everything else already uses.

//go:embed data/game-sources.json
var gameSourcesJSON []byte

type gameDocument struct {
	Version int              `json:"version"`
	Games   map[string]Entry `json:"games"`
}

var (
	gamesOnce sync.Once
	bySlug    map[string]Entry
)

func loadGames() {
	gamesOnce.Do(func() {
		bySlug = map[string]Entry{}
		var doc gameDocument
		if err := json.Unmarshal(gameSourcesJSON, &doc); err != nil {
			return
		}
		for slug, e := range doc.Games {
			if e.URL == "" {
				continue
			}
			bySlug[strings.ToLower(strings.TrimSpace(slug))] = e
		}
	})
}

// GameFor returns the curated artwork source for a wiki slug.
func GameFor(slug string) (Entry, bool) {
	loadGames()
	e, ok := bySlug[strings.ToLower(strings.TrimSpace(slug))]
	return e, ok
}

// GameURLFor returns the artwork URL for a wiki slug, or "".
func GameURLFor(slug string) string {
	e, ok := GameFor(slug)
	if !ok {
		return ""
	}
	return e.URL
}

// GameCount reports how many games have curated artwork.
func GameCount() int {
	loadGames()
	return len(bySlug)
}

// GameAttributions lists the credits that game artwork sources require, for
// the UI to display. Most publishers state no terms at all, so this is usually
// short.
func GameAttributions() []string {
	loadGames()
	seen := map[string]bool{}
	var out []string
	for _, e := range bySlug {
		if e.Attribution == "" || seen[e.Attribution] {
			continue
		}
		seen[e.Attribution] = true
		out = append(out, e.Attribution)
	}
	sort.Strings(out)
	return out
}
