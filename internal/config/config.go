// Package config handles user configuration for omarchy-esports.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SpoilerMode controls how aggressively results are withheld.
type SpoilerMode string

const (
	// SpoilerStrict withholds score, winner, VOD title, thumbnail and duration
	// until the user reveals a match explicitly.
	SpoilerStrict SpoilerMode = "strict"
	// SpoilerBalanced withholds scores and thumbnails but admits that a match
	// finished and how long its VOD runs.
	SpoilerBalanced SpoilerMode = "balanced"
	// SpoilerOff shows everything.
	SpoilerOff SpoilerMode = "off"
)

// Wiki describes one Liquipedia wiki we poll.
type Wiki struct {
	// Slug is the Liquipedia path segment, e.g. "dota2".
	Slug string `json:"slug"`
	// Game is the display name, e.g. "Dota 2".
	Game string `json:"game"`
	// TickerPage is the page carrying the upcoming-match ticker. This differs
	// per wiki: most use "Liquipedia:Matches", but starcraft2's copy of that
	// page is a stale archive and its live ticker lives on Main_Page.
	TickerPage string `json:"tickerPage"`
	// Enabled allows keeping a wiki configured but dormant.
	Enabled bool `json:"enabled"`
}

// Notifications selects which events raise a desktop notification.
type Notifications struct {
	// MatchStarting fires ahead of a followed team's match.
	MatchStarting bool `json:"matchStarting"`
	// LeadTime is how far ahead of the start MatchStarting fires.
	LeadTime Duration `json:"leadTime"`
	// MatchLive fires when a followed match actually crosses its start time.
	MatchLive bool `json:"matchLive"`
	// VODReady fires when a spoiler-safe VOD appears for a followed match.
	VODReady bool `json:"vodReady"`
	// TournamentStarting fires a day ahead of a followed tournament.
	TournamentStarting bool `json:"tournamentStarting"`
	// Quiet suppresses all notifications without losing the settings above.
	Quiet bool `json:"quiet"`
}

// Follow is one entry on the follow list: a team, optionally scoped to a
// single game.
//
// Orgs field rosters in several games — GamerLegion, NAVI, Team Spirit and
// Team Falcons all appear in both the Counter-Strike and Dota 2 wikis — and
// following the org by name alone cannot express "their Dota roster, not their
// CS one". An empty Wiki means every game, which is what a bare string in the
// config file decodes to.
type Follow struct {
	Name string `json:"name"`
	Wiki string `json:"wiki,omitempty"`
}

// UnmarshalJSON accepts either a bare string or an object, so existing configs
// keep working and the game-scoped form is opt-in:
//
//	"teams": ["Team Spirit", {"name": "GamerLegion", "wiki": "dota2"}]
func (f *Follow) UnmarshalJSON(b []byte) error {
	var name string
	if err := json.Unmarshal(b, &name); err == nil {
		f.Name, f.Wiki = name, ""
		return nil
	}
	// Alias avoids recursing back into this method.
	type raw Follow
	var r raw
	if err := json.Unmarshal(b, &r); err != nil {
		return fmt.Errorf("invalid team entry %s: %w", string(b), err)
	}
	*f = Follow(r)
	return nil
}

// MarshalJSON writes the bare string form when the entry is not game-scoped,
// so a config edited by hand stays readable.
func (f Follow) MarshalJSON() ([]byte, error) {
	if f.Wiki == "" {
		return json.Marshal(f.Name)
	}
	type raw Follow
	return json.Marshal(raw(f))
}

// Label renders the entry for display, e.g. "GamerLegion (dota2)".
func (f Follow) Label() string {
	if f.Wiki == "" {
		return f.Name
	}
	return f.Name + " (" + f.Wiki + ")"
}

// Config is the whole user configuration.
type Config struct {
	// Teams is the follow list. Names are matched case-insensitively against
	// both canonical Liquipedia names and ticker abbreviations.
	Teams []Follow `json:"teams"`

	// Wikis are the Liquipedia wikis to poll.
	Wikis []Wiki `json:"wikis"`

	// Spoilers is the blackout policy.
	Spoilers SpoilerMode `json:"spoilers"`

	// FollowedOnly restricts the ticker to matches involving followed teams.
	FollowedOnly bool `json:"followedOnly"`

	// HideTBD drops bracket slots whose opponents are not seeded yet.
	HideTBD bool `json:"hideTBD"`

	// CatchUp controls the backlog masking described in package spoiler: when
	// you have unwatched matches for a followed team, that team's later
	// fixtures have their opponent withheld, because knowing who they play
	// next reveals that they won.
	CatchUp CatchUp `json:"catchUp"`

	// Horizon is how far ahead to keep matches.
	Horizon Duration `json:"horizon"`

	// PollInterval is how often to refresh the tickers. Liquipedia's terms
	// require caching; values below MinPollInterval are clamped.
	PollInterval Duration `json:"pollInterval"`

	// LiveWindow is how long after its start a match is presumed live.
	LiveWindow Duration `json:"liveWindow"`

	Notifications Notifications `json:"notifications"`

	// LiquipediaAPIKey enables the structured LPDB v3 API. Optional — the
	// keyless ticker path is used when this is empty.
	LiquipediaAPIKey string `json:"liquipediaApiKey,omitempty"`

	// ContactEmail is embedded in the User-Agent, as Liquipedia's API terms
	// of use require a way to contact the operator.
	ContactEmail string `json:"contactEmail,omitempty"`

	// YouTube holds VOD discovery settings.
	YouTube YouTube `json:"youtube"`
}

// CatchUp configures backlog masking.
type CatchUp struct {
	// Enabled turns masking on.
	Enabled bool `json:"enabled"`
	// Window bounds how far back an unwatched match still counts as a backlog,
	// so a match missed last month does not hide the schedule forever.
	Window Duration `json:"window"`
}

// YouTube configures VOD discovery.
type YouTube struct {
	// Enabled turns VOD discovery on.
	Enabled bool `json:"enabled"`
	// Channels is an explicit extra list of channel IDs to watch, on top of
	// those discovered from tournament broadcast tables.
	Channels []string `json:"channels,omitempty"`
	// MaxAge bounds how far back a VOD can be published and still be matched.
	MaxAge Duration `json:"maxAge"`
}

// MinPollInterval is the floor on polling, to stay well inside Liquipedia's
// rate limits and caching expectations even if a user edits the config.
const MinPollInterval = 5 * time.Minute

// Default returns the shipped configuration.
func Default() Config {
	return Config{
		Teams: []Follow{},
		Wikis: []Wiki{
			{Slug: "dota2", Game: "Dota 2", TickerPage: "Liquipedia:Matches", Enabled: true},
			{Slug: "counterstrike", Game: "Counter-Strike", TickerPage: "Liquipedia:Matches", Enabled: true},
			// starcraft2's Liquipedia:Matches is a stale archive (verified: it
			// returns matches from 2024-2025). Its live ticker is on Main_Page.
			{Slug: "starcraft2", Game: "StarCraft II", TickerPage: "Main_Page", Enabled: true},
		},
		Spoilers: SpoilerStrict,
		CatchUp: CatchUp{
			Enabled: true,
			Window:  Duration(48 * time.Hour),
		},
		FollowedOnly: false,
		HideTBD:      true,
		Horizon:      Duration(14 * 24 * time.Hour),
		PollInterval: Duration(15 * time.Minute),
		LiveWindow:   Duration(4 * time.Hour),
		Notifications: Notifications{
			MatchStarting:      true,
			LeadTime:           Duration(10 * time.Minute),
			MatchLive:          true,
			VODReady:           true,
			TournamentStarting: true,
		},
		YouTube: YouTube{
			Enabled: true,
			MaxAge:  Duration(7 * 24 * time.Hour),
		},
	}
}

// Dir returns the configuration directory, honouring XDG.
func Dir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return filepath.Join(d, "omarchy-esports")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "omarchy-esports")
}

// Path returns the config file path.
func Path() string { return filepath.Join(Dir(), "config.json") }

// Load reads the config, creating it with defaults when absent.
func Load() (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		if err := Save(cfg); err != nil {
			return cfg, fmt.Errorf("writing default config: %w", err)
		}
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	// Unmarshal over the defaults so a partial config file keeps sane values
	// for anything it omits.
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing %s: %w", Path(), err)
	}
	cfg.clamp()
	return cfg, nil
}

func (c *Config) clamp() {
	if time.Duration(c.PollInterval) < MinPollInterval {
		c.PollInterval = Duration(MinPollInterval)
	}
	if time.Duration(c.Horizon) <= 0 {
		c.Horizon = Duration(14 * 24 * time.Hour)
	}
	if time.Duration(c.LiveWindow) <= 0 {
		c.LiveWindow = Duration(4 * time.Hour)
	}
	if time.Duration(c.CatchUp.Window) <= 0 {
		c.CatchUp.Window = Duration(48 * time.Hour)
	}
	switch c.Spoilers {
	case SpoilerStrict, SpoilerBalanced, SpoilerOff:
	default:
		c.Spoilers = SpoilerStrict
	}
}

// Save atomically writes the config.
func Save(c Config) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := Path() + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, Path())
}

// EnabledWikis returns only the wikis marked enabled.
func (c Config) EnabledWikis() []Wiki {
	var out []Wiki
	for _, w := range c.Wikis {
		if w.Enabled {
			out = append(out, w)
		}
	}
	return out
}

// Follows reports whether a team is followed in the given wiki. An empty wiki
// asks "followed in any game?".
func (c Config) Follows(name, wiki string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, t := range c.Teams {
		if strings.ToLower(strings.TrimSpace(t.Name)) != name {
			continue
		}
		// An unscoped entry follows the team everywhere.
		if t.Wiki == "" || wiki == "" || strings.EqualFold(t.Wiki, wiki) {
			return true
		}
	}
	return false
}

// FollowIndex returns the position of an exact (name, wiki) entry, or -1.
func (c Config) FollowIndex(name, wiki string) int {
	for i, t := range c.Teams {
		if strings.EqualFold(strings.TrimSpace(t.Name), strings.TrimSpace(name)) &&
			strings.EqualFold(t.Wiki, wiki) {
			return i
		}
	}
	return -1
}

// TeamNames returns the follow list as plain names, for the paths that only
// need to know which teams matter.
func (c Config) TeamNames() []string {
	out := make([]string, 0, len(c.Teams))
	for _, t := range c.Teams {
		out = append(out, t.Name)
	}
	return out
}

// WikiEnabled reports whether a wiki slug is configured and enabled.
func (c Config) WikiEnabled(slug string) bool {
	for _, w := range c.Wikis {
		if strings.EqualFold(w.Slug, slug) {
			return w.Enabled
		}
	}
	return false
}

// Duration is a time.Duration that marshals as a human string ("15m", "4h")
// so the config file stays readable and hand-editable.
type Duration time.Duration

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch x := v.(type) {
	case string:
		parsed, err := time.ParseDuration(x)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", x, err)
		}
		*d = Duration(parsed)
	case float64:
		// Bare numbers are seconds, matching omarchy's shell.json idle values.
		*d = Duration(time.Duration(x) * time.Second)
	default:
		return fmt.Errorf("invalid duration %v", v)
	}
	return nil
}
