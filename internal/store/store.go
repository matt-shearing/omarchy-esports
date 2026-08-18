// Package store persists daemon state.
//
// State is deliberately split across two files:
//
//	state.json  (0644) — the redacted view the UI reads. Under a strict policy
//	                     this file physically does not contain the score of an
//	                     unrevealed match.
//	full.json   (0600) — everything the daemon knows, including results,
//	                     the reveal set and notification bookkeeping.
//
// Keeping them apart is what makes the spoiler guarantee structural. If the UI
// can only ever read state.json, then no UI bug, log line, or curious `jq`
// invocation can surface a result the user has not asked for.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/contra/omarchy-esports/internal/match"
)

// Public is the redacted document the UI consumes.
type Public struct {
	// Version lets the QML side detect an incompatible daemon.
	Version int `json:"version"`
	// UpdatedAt is when the daemon last completed a refresh.
	UpdatedAt time.Time `json:"updatedAt"`
	// Matches are sorted by start time, ascending.
	Matches []match.Match `json:"matches"`
	// Spoilers echoes the active policy so the UI can label its blackouts.
	Spoilers string `json:"spoilers"`
	// Teams is the follow list, so the UI can render it without re-reading config.
	Teams []string `json:"teams"`
	// Errors records per-wiki failures from the last refresh, so the UI can
	// show a degraded state instead of silently displaying stale data.
	Errors []string `json:"errors,omitempty"`
	// Attribution is required by Liquipedia's CC-BY-SA 3.0 terms.
	Attribution string `json:"attribution"`
}

// Private is the daemon's own record.
type Private struct {
	Version   int             `json:"version"`
	UpdatedAt time.Time       `json:"updatedAt"`
	Matches   []match.Match   `json:"matches"`
	Revealed  map[string]bool `json:"revealed"`
	// Watched advances the catch-up queue: once a match is watched, the next
	// unwatched one becomes the visible queue head.
	Watched  map[string]bool      `json:"watched"`
	Notified map[string]time.Time `json:"notified"`
	// Teams is a cumulative index of every team seen in a ticker, used to power
	// the app's follow-list search without extra API calls. It is kept across
	// refreshes so it grows into a useful directory over time.
	Teams map[string]TeamEntry `json:"teams"`
	// TournamentStreams caches broadcast channels discovered from tournament
	// pages, keyed by tournament page path. These are expensive to fetch
	// (one rate-limited parse each) and change rarely.
	TournamentStreams map[string]TournamentInfo `json:"tournamentStreams"`
}

// TeamEntry is one team in the searchable index.
type TeamEntry struct {
	Name     string     `json:"name"`
	Short    string     `json:"short,omitempty"`
	Page     string     `json:"page,omitempty"`
	Logo     match.Logo `json:"logo,omitempty"`
	Wiki     string     `json:"wiki,omitempty"`
	Game     string     `json:"game,omitempty"`
	LastSeen time.Time  `json:"lastSeen"`
}

// TournamentInfo is the cached result of scraping a tournament page.
type TournamentInfo struct {
	FetchedAt       time.Time      `json:"fetchedAt"`
	Streams         []match.Stream `json:"streams"`
	YouTubeChannels []string       `json:"youtubeChannels"`
}

// CurrentVersion is the state schema version.
const CurrentVersion = 1

// Attribution is the credit line Liquipedia's CC-BY-SA 3.0 licence requires.
const Attribution = "Match data from Liquipedia (CC BY-SA 3.0)"

// Store reads and writes daemon state.
type Store struct {
	dir string
	mu  sync.Mutex
}

// Dir returns the state directory, honouring XDG.
func Dir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "omarchy-esports")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "omarchy-esports")
}

// CacheDir returns the cache directory, honouring XDG.
func CacheDir() string {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" {
		return filepath.Join(d, "omarchy-esports")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".cache", "omarchy-esports")
}

// New opens a store rooted at dir. An empty dir uses the XDG default.
func New(dir string) (*Store, error) {
	if dir == "" {
		dir = Dir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// PublicPath is the path the UI watches.
func (s *Store) PublicPath() string { return filepath.Join(s.dir, "state.json") }

// PrivatePath is the daemon-only record.
func (s *Store) PrivatePath() string { return filepath.Join(s.dir, "full.json") }

// LoadPrivate reads the daemon record, returning an empty one when absent.
func (s *Store) LoadPrivate() (Private, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	p := Private{
		Version:           CurrentVersion,
		Revealed:          map[string]bool{},
		Watched:           map[string]bool{},
		Notified:          map[string]time.Time{},
		Teams:             map[string]TeamEntry{},
		TournamentStreams: map[string]TournamentInfo{},
	}
	data, err := os.ReadFile(s.PrivatePath())
	if os.IsNotExist(err) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(data, &p); err != nil {
		// A corrupt state file must not brick the daemon; start over rather
		// than refusing to run.
		return Private{
			Version:           CurrentVersion,
			Revealed:          map[string]bool{},
			Watched:           map[string]bool{},
			Notified:          map[string]time.Time{},
			Teams:             map[string]TeamEntry{},
			TournamentStreams: map[string]TournamentInfo{},
		}, nil
	}
	if p.Revealed == nil {
		p.Revealed = map[string]bool{}
	}
	if p.Watched == nil {
		p.Watched = map[string]bool{}
	}
	if p.Teams == nil {
		p.Teams = map[string]TeamEntry{}
	}
	if p.Notified == nil {
		p.Notified = map[string]time.Time{}
	}
	if p.TournamentStreams == nil {
		p.TournamentStreams = map[string]TournamentInfo{}
	}
	return p, nil
}

// SavePrivate writes the daemon record with owner-only permissions.
func (s *Store) SavePrivate(p Private) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.Version = CurrentVersion
	return writeJSON(s.PrivatePath(), p, 0o600)
}

// SavePublic writes the redacted view the UI reads.
func (s *Store) SavePublic(p Public) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p.Version = CurrentVersion
	p.Attribution = Attribution
	sort.SliceStable(p.Matches, func(i, j int) bool {
		return p.Matches[i].StartsAt.Before(p.Matches[j].StartsAt)
	})
	return writeJSON(s.PublicPath(), p, 0o644)
}

// LoadPublic reads the redacted view, for the CLI's status output.
func (s *Store) LoadPublic() (Public, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var p Public
	data, err := os.ReadFile(s.PublicPath())
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return p, fmt.Errorf("parsing %s: %w", s.PublicPath(), err)
	}
	return p, nil
}

// writeJSON writes atomically so a reader never observes a half-written file.
// The UI watches these paths for changes, so a torn read would be visible.
func writeJSON(path string, v any, perm os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), perm); err != nil {
		return err
	}
	// Ensure the mode sticks even if the temp file already existed.
	if err := os.Chmod(tmp, perm); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// SetWatched records that the user has seen a match, advancing the catch-up
// queue for whichever followed teams played in it.
func (s *Store) SetWatched(id string, watched bool) error {
	p, err := s.LoadPrivate()
	if err != nil {
		return err
	}
	if watched {
		p.Watched[id] = true
	} else {
		delete(p.Watched, id)
	}
	return s.SavePrivate(p)
}

// TeamsPath is the searchable team index the app reads.
func (s *Store) TeamsPath() string { return filepath.Join(s.dir, "teams.json") }

// SaveTeams publishes the team index. Team names and artwork reveal no
// results, so this file needs no redaction.
func (s *Store) SaveTeams(teams []TeamEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	sort.SliceStable(teams, func(i, j int) bool { return teams[i].Name < teams[j].Name })
	return writeJSON(s.TeamsPath(), map[string]any{
		"version": CurrentVersion,
		"teams":   teams,
	}, 0o644)
}

// SetRevealed records an explicit reveal and persists it.
func (s *Store) SetRevealed(id string, revealed bool) error {
	p, err := s.LoadPrivate()
	if err != nil {
		return err
	}
	if revealed {
		p.Revealed[id] = true
	} else {
		delete(p.Revealed, id)
	}
	return s.SavePrivate(p)
}
