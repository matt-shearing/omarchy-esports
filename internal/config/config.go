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
	// Short is the compact badge shown against a fixture, e.g. "CS2".
	Short string `json:"short,omitempty"`
	// TickerPage is the page carrying the upcoming-match ticker. This differs
	// per wiki: most use "Liquipedia:Matches", but starcraft2's copy of that
	// page is a stale archive and its live ticker lives on Main_Page.
	TickerPage string `json:"tickerPage"`
	// Enabled allows keeping a wiki configured but dormant.
	Enabled bool `json:"enabled"`
	// TeamCategory overrides the category enumerated to build the team
	// directory. Empty means "Teams", which is what every wiki checked uses.
	TeamCategory string `json:"teamCategory,omitempty"`
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

	// MinTier hides matches from tournaments below the given Liquipedia tier:
	// 1 keeps only premier events, 2 adds A-Tier, and so on. Zero disables the
	// filter. Matches whose tier could not be determined are kept — see
	// HideUnknownTier.
	MinTier int `json:"minTier"`

	// HideMinorEvents drops qualifiers, weeklies, monthlies and showmatches,
	// using the tournament's `liquipediatiertype`. This is a different axis
	// from tier — event format rather than prestige — and is often the more
	// useful filter, since a weekly cup can still be tagged a decent tier.
	HideMinorEvents bool `json:"hideMinorEvents"`

	// HideUnknownTier also drops matches whose tournament has no tier yet.
	// Off by default: tier is fetched lazily, so a newly seen tournament is
	// briefly unknown, and hiding those would make matches flicker in and out.
	HideUnknownTier bool `json:"hideUnknownTier"`

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

	// SetupComplete records that the user has been through first-run setup.
	// Until then the app shows a wizard instead of the schedule, because an
	// empty follow list and two default games are a poor first impression of
	// what the tool does.
	SetupComplete bool `json:"setupComplete"`

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
		Teams:    []Follow{},
		Wikis:    defaultWikis(),
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

// defaultWikis enables the two biggest scenes on a fresh install and offers
// the rest of the catalog pre-configured but dormant, so turning one on is a
// toggle rather than a research exercise.
//
// Starting narrow matters: every enabled game costs a 30-second parse slot per
// refresh, so a default of "everything" would make a new install slow and
// noisy before the user has expressed any preference. The setup wizard is
// where they widen it.
func defaultWikis() []Wiki {
	on := map[string]bool{"counterstrike": true, "dota2": true}
	out := make([]Wiki, 0, len(Catalog))
	for _, e := range Catalog {
		out = append(out, e.Wiki(on[e.Slug]))
	}
	return out
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

	// Unmarshalling over the defaults keeps sane values for anything the file
	// omits — but Go's json.Unmarshal REUSES the elements of an existing
	// slice rather than replacing them, so a shorter array in the file leaves
	// each element carrying leftover fields from the default at that index.
	// That silently paired every game with the wrong short badge: the file's
	// first wiki (dota2) inherited "CS2" from the default catalog's first
	// entry. Clearing the slices first forces them to be rebuilt from the
	// file, and they are restored below when the file omits them entirely.
	defaults := cfg
	cfg.Wikis = nil
	cfg.Teams = nil

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing %s: %w", Path(), err)
	}
	if cfg.Wikis == nil {
		cfg.Wikis = defaults.Wikis
	}
	if cfg.Teams == nil {
		cfg.Teams = defaults.Teams
	}
	cfg.clamp()
	return cfg, nil
}

func (c *Config) clamp() {
	// Short is catalog metadata rather than a user preference, so the catalog
	// is always authoritative. Backfilling only when empty is not enough: an
	// earlier json.Unmarshal slice-reuse bug wrote badges belonging to other
	// games into existing configs, and those files need repairing rather than
	// preserving.
	for i := range c.Wikis {
		if e, ok := CatalogFor(c.Wikis[i].Slug); ok {
			c.Wikis[i].Short = e.Short
		}
	}
	// Offer catalog games the config predates, dormant.
	known := map[string]bool{}
	for _, w := range c.Wikis {
		known[w.Slug] = true
	}
	for _, e := range Catalog {
		if !known[e.Slug] {
			c.Wikis = append(c.Wikis, e.Wiki(false))
		}
	}

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
//
// Normalisation runs here rather than only in Load, so every writer goes
// through the same correction. `config set` builds its value by decoding into
// a defaults-populated struct, which is exactly the path that used to write
// mismatched game badges back to disk.
func Save(c Config) error {
	c.clamp()
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

// KnownWiki reports whether a slug is one this build knows about, whether or
// not it is currently being polled.
func (c Config) KnownWiki(slug string) bool {
	for _, w := range c.Wikis {
		if strings.EqualFold(w.Slug, slug) {
			return true
		}
	}
	_, ok := CatalogFor(strings.ToLower(slug))
	return ok
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
