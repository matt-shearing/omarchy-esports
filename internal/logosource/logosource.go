// Package logosource maps team names to curated artwork URLs.
//
// Team logos normally come from Liquipedia's copy, which has two drawbacks:
// every client fetching artwork concentrates load on one host — enough to earn
// a rate limit — and Liquipedia's copy is carried under a fair-use claim scoped
// to their own use.
//
// This package ships a curated map of URLs pointing at each org's own CDN
// where one could be verified. The map contains links, not artwork: the images
// are still fetched to each user's own machine and cached there, and nothing
// is redistributed with the project.
package logosource

import (
	_ "embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed data/logo-sources.json
var sourcesJSON []byte

// Entry is one curated source.
type Entry struct {
	URL    string `json:"url"`
	Source string `json:"source"`
	// Terms records what the source actually states about reuse. "none stated"
	// is the honest and dominant answer: almost no esports org publishes a
	// press kit, so most entries are simply the file the org's own site loads.
	Terms string `json:"terms"`
	// Attribution is required credit, where a real licence demands it.
	Attribution string `json:"attribution,omitempty"`
	Note        string `json:"note,omitempty"`
}

type document struct {
	Version int              `json:"version"`
	Teams   map[string]Entry `json:"teams"`
}

var (
	once   sync.Once
	byName map[string]Entry
)

func load() {
	once.Do(func() {
		byName = map[string]Entry{}
		var doc document
		if err := json.Unmarshal(sourcesJSON, &doc); err != nil {
			return
		}
		for name, e := range doc.Teams {
			if e.URL == "" {
				continue
			}
			byName[normalise(name)] = e
		}
	})
}

func normalise(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// For returns the curated source for a team, if one exists.
func For(team string) (Entry, bool) {
	load()
	e, ok := byName[normalise(team)]
	return e, ok
}

// URLFor returns just the URL, or "" when the team is not mapped.
func URLFor(team string) string {
	if e, ok := For(team); ok {
		return e.URL
	}
	return ""
}

// Count reports how many teams are mapped, for diagnostics.
func Count() int {
	load()
	return len(byName)
}

// Attributions lists the credits required by the curated sources, so the UI
// and docs can carry them.
func Attributions() []string {
	load()
	seen := map[string]bool{}
	var out []string
	for _, e := range byName {
		if e.Attribution == "" || seen[e.Attribution] {
			continue
		}
		seen[e.Attribution] = true
		out = append(out, e.Attribution)
	}
	return out
}
