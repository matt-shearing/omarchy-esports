package daemon

import (
	"fmt"
	"time"

	"github.com/contra/omarchy-esports/internal/liquipedia"
	"github.com/contra/omarchy-esports/internal/match"
	"github.com/contra/omarchy-esports/internal/notify"
	"github.com/contra/omarchy-esports/internal/store"
)

// event names a notification kind, used as part of the de-duplication key so
// each match can raise each kind at most once.
type event string

const (
	eventStarting   event = "starting"
	eventLive       event = "live"
	eventVOD        event = "vod"
	eventTournament event = "tournament"
)

// notifyKey builds the de-duplication key for a match and event.
func notifyKey(id string, e event) string { return string(e) + ":" + id }

// dispatchNotifications raises any due notifications and records them.
//
// It is called both after a network refresh and on the fast timer, so it must
// be idempotent: the Notified map is the single source of truth for what has
// already been sent.
func (d *Daemon) dispatchNotifications(priv *store.Private) {
	n := d.cfg.Notifications
	if n.Quiet {
		return
	}
	now := time.Now()
	lead := time.Duration(n.LeadTime)

	// Suppress the backlog on a first run: without this, configuring the tool
	// mid-tournament would fire a burst of notifications for matches that
	// already happened.
	firstRun := len(priv.Notified) == 0

	for i := range priv.Matches {
		m := &priv.Matches[i]
		if !m.Followed {
			continue
		}

		if n.MatchStarting && m.State == match.StateUpcoming {
			until := m.StartsAt.Sub(now)
			if until > 0 && until <= lead {
				d.raise(priv, m, eventStarting, firstRun,
					fmt.Sprintf("%s starts in %s", m.Title(), humaniseDuration(until)),
					m.Tournament.Name, "󰥔")
			}
		}

		if n.MatchLive && m.State == match.StateLive {
			d.raise(priv, m, eventLive, firstRun,
				m.Title()+" is live",
				m.Tournament.Name, "󰐊")
		}

		if n.VODReady && m.VOD != nil {
			// The body deliberately names only the fixture, never the result,
			// so a notification cannot spoil a match the user is avoiding.
			d.raise(priv, m, eventVOD, firstRun,
				"VOD ready: "+m.Title(),
				m.Tournament.Name, "󰕧")
		}
	}

	if n.TournamentStarting {
		d.notifyTournaments(priv, now, firstRun)
	}
}

// raise sends one notification unless it has already been sent.
func (d *Daemon) raise(priv *store.Private, m *match.Match, e event, firstRun bool, title, body, glyph string) {
	key := notifyKey(m.ID, e)
	if _, done := priv.Notified[key]; done {
		return
	}
	priv.Notified[key] = time.Now()
	if firstRun {
		// Record it as sent so it does not fire later, but stay silent now.
		return
	}

	nn := notify.Notification{
		Title:   title,
		Body:    body,
		Glyph:   glyph,
		AppName: "omarchy-esports",
	}
	// Clicking should do the obvious thing: open the stream for a live match,
	// the VOD for a finished one.
	switch e {
	case eventVOD:
		if m.VOD != nil {
			nn.Exec = notify.OpenCommand(m.VOD.URL)
		}
	default:
		if s := liquipedia.PreferredStream(m.Streams); s != nil {
			nn.Exec = notify.OpenCommand(s.URL)
			nn.Body = body + " · " + s.Platform
		}
	}
	if err := d.notifier.Send(nn); err != nil {
		d.logger.Printf("notification failed: %v", err)
	}
}

// notifyTournaments raises a day-ahead heads-up for events featuring a
// followed team, keyed on the tournament rather than any single match.
func (d *Daemon) notifyTournaments(priv *store.Private, now time.Time, firstRun bool) {
	type info struct {
		earliest time.Time
		name     string
		followed bool
	}
	events := map[string]*info{}
	for i := range priv.Matches {
		m := &priv.Matches[i]
		if m.Tournament.Page == "" {
			continue
		}
		e, ok := events[m.Tournament.Page]
		if !ok {
			e = &info{earliest: m.StartsAt, name: m.Tournament.Name}
			events[m.Tournament.Page] = e
		}
		if m.StartsAt.Before(e.earliest) {
			e.earliest = m.StartsAt
		}
		if m.Followed {
			e.followed = true
		}
	}

	for page, e := range events {
		if !e.followed {
			continue
		}
		until := e.earliest.Sub(now)
		if until <= 0 || until > 24*time.Hour {
			continue
		}
		key := notifyKey(page, eventTournament)
		if _, done := priv.Notified[key]; done {
			continue
		}
		priv.Notified[key] = time.Now()
		if firstRun {
			continue
		}
		if err := d.notifier.Send(notify.Notification{
			Title:   e.name + " starts " + humaniseWhen(until),
			Body:    "A team you follow is competing",
			Glyph:   "󰆚",
			AppName: "omarchy-esports",
		}); err != nil {
			d.logger.Printf("notification failed: %v", err)
		}
	}
}

// humaniseDuration renders a short lead time, e.g. "10 min".
func humaniseDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "under a minute"
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()))
	default:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m == 0 {
			return fmt.Sprintf("%dh", h)
		}
		return fmt.Sprintf("%dh %dm", h, m)
	}
}

// humaniseWhen renders a coarse "tomorrow"-style phrase.
func humaniseWhen(d time.Duration) string {
	switch {
	case d < time.Hour:
		return "within the hour"
	case d < 6*time.Hour:
		return fmt.Sprintf("in %d hours", int(d.Hours()))
	default:
		return "tomorrow"
	}
}

// PruneNotified drops bookkeeping for matches that have aged out, so the state
// file does not grow without bound.
func PruneNotified(priv *store.Private, keep time.Duration) {
	cutoff := time.Now().Add(-keep)
	for k, at := range priv.Notified {
		if at.Before(cutoff) {
			delete(priv.Notified, k)
		}
	}
}
