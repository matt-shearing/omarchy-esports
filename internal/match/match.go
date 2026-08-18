// Package match defines the core domain types shared by the daemon, the CLI,
// and (via JSON) the Quickshell UI.
package match

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// State is where a match sits in its lifecycle.
type State string

const (
	StateUpcoming State = "upcoming"
	StateLive     State = "live"
	StateFinished State = "finished"
)

// Opponent is one side of a match. For team games this is an org; for the SC2
// team leagues it is also an org, and for 1v1 wikis it is a player.
type Opponent struct {
	Name  string `json:"name"`           // canonical name, e.g. "Team Spirit"
	Short string `json:"short"`          // ticker abbreviation, e.g. "TSpirit"
	Page  string `json:"page,omitempty"` // liquipedia page path
	Logo  Logo   `json:"logo,omitempty"` // light/dark variants

	// Hidden marks a side withheld by catch-up masking. When it is set every
	// other field on this opponent is empty: knowing who a followed team plays
	// next reveals that they won their previous match, so the identity is
	// removed from the published state rather than merely not drawn.
	Hidden bool `json:"hidden,omitempty"`
}

// URL returns the opponent's Liquipedia page, or "" when unknown or hidden.
func (o Opponent) URL() string {
	if o.Hidden || o.Page == "" {
		return ""
	}
	if strings.HasPrefix(o.Page, "http") {
		return o.Page
	}
	return "https://liquipedia.net" + o.Page
}

// Logo carries the light/dark artwork Liquipedia publishes for a team. The UI
// picks a variant based on the active omarchy theme.
type Logo struct {
	Light string `json:"light,omitempty"`
	Dark  string `json:"dark,omitempty"`
	// Local is a cached on-disk path, filled in by the logo fetcher.
	Local string `json:"local,omitempty"`
}

// Pick returns the URL for the requested theme, falling back to whichever
// variant exists. Liquipedia often publishes only an "allmode" logo.
func (l Logo) Pick(dark bool) string {
	if dark && l.Dark != "" {
		return l.Dark
	}
	if !dark && l.Light != "" {
		return l.Light
	}
	if l.Light != "" {
		return l.Light
	}
	return l.Dark
}

// Tournament is the event a match belongs to.
type Tournament struct {
	Name string `json:"name"`
	Page string `json:"page,omitempty"`
	Icon string `json:"icon,omitempty"`
	Tier string `json:"tier,omitempty"`
}

// Stream is a live broadcast channel.
type Stream struct {
	Platform string `json:"platform"` // "twitch" | "youtube"
	Channel  string `json:"channel"`
	URL      string `json:"url"`
	Language string `json:"language,omitempty"` // ISO-ish hint parsed from the channel name
	Primary  bool   `json:"primary,omitempty"`  // best guess at the main English feed
}

// VOD is a recorded video for a finished match.
type VOD struct {
	VideoID   string    `json:"videoId"`
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Channel   string    `json:"channel,omitempty"`
	Published time.Time `json:"published"`
	Thumbnail string    `json:"thumbnail,omitempty"`
	// Kind distinguishes a full match recording from a cut-down upload.
	// Highlight packages are both the wrong video for someone wanting to
	// watch the series and far more likely to spoil it in their artwork, so
	// the UI labels them rather than presenting them as the match.
	Kind string `json:"kind,omitempty"` // "full" | "highlights"
	// Spoilery records why this VOD is considered result-leaking, so the UI can
	// explain the blackout instead of silently hiding things.
	Spoilery []string `json:"spoilery,omitempty"`
}

// Match is one scheduled, live, or completed series.
type Match struct {
	ID         string      `json:"id"`
	Wiki       string      `json:"wiki"`
	Game       string      `json:"game"`
	StartsAt   time.Time   `json:"startsAt"`
	Opponents  [2]Opponent `json:"opponents"`
	BestOf     int         `json:"bestOf,omitempty"`
	Tournament Tournament  `json:"tournament"`
	Streams    []Stream    `json:"streams,omitempty"`
	State      State       `json:"state"`

	// Result fields. In a redacted (spoiler-free) view these are zeroed and
	// Redacted is set — see package spoiler. They are never merely hidden by
	// the UI; they are absent from the file the UI reads.
	Score  [2]int `json:"score,omitempty"`
	Winner int    `json:"winner,omitempty"` // 1-based side, 0 = unknown

	VOD *VOD `json:"vod,omitempty"`

	// Redacted is true when result data was withheld from this record.
	Redacted bool `json:"redacted,omitempty"`
	// Revealed is true when the user has explicitly unblinded this match.
	Revealed bool `json:"revealed,omitempty"`
	// Followed is true when either opponent is on the user's follow list.
	Followed bool `json:"followed,omitempty"`

	// Watched records that the user has seen this match, which advances the
	// catch-up queue.
	Watched bool `json:"watched,omitempty"`

	// QueueHead marks the earliest unwatched finished match for a followed
	// team: the one to watch next, and the only one shown in full.
	QueueHead bool `json:"queueHead,omitempty"`

	// Masked is set when catch-up masking hid one or both opponents.
	Masked bool `json:"masked,omitempty"`

	// MaskedFor names the followed team whose backlog caused the masking, so
	// the UI can say why a fixture is hidden.
	MaskedFor string `json:"maskedFor,omitempty"`
}

// TournamentURL returns the tournament's Liquipedia page.
func (m *Match) TournamentURL() string {
	if m.Tournament.Page == "" {
		return ""
	}
	if strings.HasPrefix(m.Tournament.Page, "http") {
		return m.Tournament.Page
	}
	return "https://liquipedia.net" + m.Tournament.Page
}

// ComputeID derives a stable identifier from the properties that do not change
// as a match progresses. Liquipedia's own match ids are absent from the ticker
// for most wikis, so we synthesise one; it must stay stable across polls or
// notification de-duplication and reveal state would break.
func (m *Match) ComputeID() string {
	key := strings.Join([]string{
		m.Wiki,
		normalise(m.Opponents[0].Name),
		normalise(m.Opponents[1].Name),
		fmt.Sprint(m.StartsAt.UTC().Unix()),
	}, "|")
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:])[:16]
}

func normalise(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// Title is a short human label, e.g. "G2 vs NAVI".
func (m *Match) Title() string {
	a, b := m.Opponents[0].Display(), m.Opponents[1].Display()
	return a + " vs " + b
}

// Display prefers the canonical name but falls back to the abbreviation.
func (o Opponent) Display() string {
	if o.Name != "" {
		return o.Name
	}
	return o.Short
}

// Involves reports whether either side matches any of the supplied names,
// compared case-insensitively against both canonical and short forms.
func (m *Match) Involves(names []string) bool {
	for _, n := range names {
		n = normalise(n)
		if n == "" {
			continue
		}
		for _, o := range m.Opponents {
			if normalise(o.Name) == n || normalise(o.Short) == n {
				return true
			}
		}
	}
	return false
}

// TBD reports whether the match still has placeholder opponents. Liquipedia
// publishes bracket slots before seeding is known; these are noise in a ticker.
func (m *Match) TBD() bool {
	for _, o := range m.Opponents {
		d := normalise(o.Display())
		if d == "" || d == "tbd" || d == "tba" {
			return true
		}
	}
	return false
}

// DeriveState infers lifecycle position from the clock and known results.
// A series has no published end time, so we treat a match as live for a
// generous window after its start and finished once a score exists.
func (m *Match) DeriveState(now time.Time, liveWindow time.Duration) State {
	if m.Score[0] > 0 || m.Score[1] > 0 || m.Winner != 0 {
		return StateFinished
	}
	if now.Before(m.StartsAt) {
		return StateUpcoming
	}
	if now.Sub(m.StartsAt) < liveWindow {
		return StateLive
	}
	return StateFinished
}
