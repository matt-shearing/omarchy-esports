// Package daemon orchestrates polling, enrichment, notification and state
// persistence.
package daemon

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/contra/omarchy-esports/internal/config"
	"github.com/contra/omarchy-esports/internal/liquipedia"
	"github.com/contra/omarchy-esports/internal/match"
	"github.com/contra/omarchy-esports/internal/notify"
	"github.com/contra/omarchy-esports/internal/spoiler"
	"github.com/contra/omarchy-esports/internal/store"
	"github.com/contra/omarchy-esports/internal/youtube"
)

// Daemon polls Liquipedia and YouTube and maintains the state files.
type Daemon struct {
	cfg      config.Config
	lp       *liquipedia.Client
	yt       *youtube.Client
	store    *store.Store
	notifier *notify.Sender
	logger   *log.Logger

	// maxTournamentFetches bounds how many tournament pages one refresh may
	// fetch. Each costs a 30-second rate-limit slot, so an unbounded sweep on
	// first run would take many minutes.
	maxTournamentFetches int
}

// Options configures a Daemon.
type Options struct {
	Config  config.Config
	Store   *store.Store
	Version string
	Logger  *log.Logger
	DryRun  bool
}

// New builds a Daemon.
func New(o Options) *Daemon {
	logger := o.Logger
	if logger == nil {
		logger = log.Default()
	}
	notifier := notify.NewSender()
	notifier.DryRun = o.DryRun
	notifier.Log = func(s string) { logger.Print(s) }

	return &Daemon{
		cfg:                  o.Config,
		lp:                   liquipedia.New(o.Version, o.Config.ContactEmail, store.CacheDir()),
		yt:                   youtube.New(15 * time.Minute),
		store:                o.Store,
		notifier:             notifier,
		logger:               logger,
		maxTournamentFetches: 4,
	}
}

// Run polls until the context is cancelled.
func (d *Daemon) Run(ctx context.Context) error {
	interval := time.Duration(d.cfg.PollInterval)
	d.logger.Printf("starting: %d wiki(s), poll every %s, spoilers=%s",
		len(d.cfg.EnabledWikis()), interval, d.cfg.Spoilers)

	if err := d.RefreshOnce(ctx); err != nil {
		d.logger.Printf("initial refresh: %v", err)
	}

	poll := time.NewTicker(interval)
	defer poll.Stop()
	// Notifications and lifecycle transitions are time-sensitive, so re-evaluate
	// far more often than we re-fetch. This costs nothing: it works purely on
	// already-fetched state.
	tick := time.NewTicker(30 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-poll.C:
			if err := d.RefreshOnce(ctx); err != nil {
				d.logger.Printf("refresh: %v", err)
			}
		case <-tick.C:
			if err := d.Reevaluate(ctx); err != nil {
				d.logger.Printf("reevaluate: %v", err)
			}
		}
	}
}

// RefreshOnce performs a full poll: fetch tickers, enrich, notify, persist.
func (d *Daemon) RefreshOnce(ctx context.Context) error {
	priv, err := d.store.LoadPrivate()
	if err != nil {
		return fmt.Errorf("loading state: %w", err)
	}

	var all []match.Match
	var errs []string
	for _, w := range d.cfg.EnabledWikis() {
		// Cache for most of the poll interval so a manual refresh right after
		// an automatic one does not re-request the same page.
		html, err := d.lp.ParsePage(ctx, w.Slug, w.TickerPage, time.Duration(d.cfg.PollInterval)-time.Minute)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", w.Slug, err))
			d.logger.Printf("fetching %s ticker: %v", w.Slug, err)
			continue
		}
		ms, err := liquipedia.ParseTicker(html, w.Slug, w.Game)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", w.Slug, err))
			continue
		}
		d.logger.Printf("%s: parsed %d matches", w.Slug, len(ms))
		all = append(all, ms...)
	}

	if len(all) == 0 && len(errs) > 0 {
		// Every source failed. Keep the previous state rather than blanking
		// the UI, but surface the errors.
		if err := d.publish(priv, errs); err != nil {
			return err
		}
		return fmt.Errorf("all sources failed: %s", strings.Join(errs, "; "))
	}

	all = d.filterAndAnnotate(all, time.Now())
	all = d.mergeWithKnown(priv.Matches, all)

	if err := d.enrich(ctx, all, &priv); err != nil {
		d.logger.Printf("enrichment: %v", err)
	}

	priv.Matches = all
	priv.UpdatedAt = time.Now()
	if err := d.store.SavePrivate(priv); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}
	d.dispatchNotifications(&priv)
	if err := d.store.SavePrivate(priv); err != nil {
		return fmt.Errorf("saving notification state: %w", err)
	}
	return d.publish(priv, errs)
}

// Reevaluate refreshes derived state and fires time-based notifications
// without hitting the network.
func (d *Daemon) Reevaluate(ctx context.Context) error {
	priv, err := d.store.LoadPrivate()
	if err != nil {
		return err
	}
	if len(priv.Matches) == 0 {
		return nil
	}
	now := time.Now()
	for i := range priv.Matches {
		priv.Matches[i].State = priv.Matches[i].DeriveState(now, time.Duration(d.cfg.LiveWindow))
	}
	d.dispatchNotifications(&priv)
	if err := d.store.SavePrivate(priv); err != nil {
		return err
	}
	return d.publish(priv, nil)
}

// publish writes the redacted view the UI reads.
func (d *Daemon) publish(priv store.Private, errs []string) error {
	visible := spoiler.RedactAll(priv.Matches, d.cfg.Spoilers, priv.Revealed)
	return d.store.SavePublic(store.Public{
		UpdatedAt: priv.UpdatedAt,
		Matches:   visible,
		Spoilers:  string(d.cfg.Spoilers),
		Teams:     d.cfg.Teams,
		Errors:    errs,
	})
}

// filterAndAnnotate applies the horizon, drops noise and marks followed teams.
func (d *Daemon) filterAndAnnotate(ms []match.Match, now time.Time) []match.Match {
	horizon := now.Add(time.Duration(d.cfg.Horizon))
	// Keep finished matches around for a while so VODs have time to appear.
	past := now.Add(-7 * 24 * time.Hour)

	out := make([]match.Match, 0, len(ms))
	seen := map[string]bool{}
	for _, m := range ms {
		if seen[m.ID] {
			continue
		}
		if m.StartsAt.After(horizon) || m.StartsAt.Before(past) {
			continue
		}
		if d.cfg.HideTBD && m.TBD() {
			continue
		}
		m.State = m.DeriveState(now, time.Duration(d.cfg.LiveWindow))
		m.Followed = m.Involves(d.cfg.Teams)
		if d.cfg.FollowedOnly && !m.Followed {
			continue
		}
		seen[m.ID] = true
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartsAt.Before(out[j].StartsAt) })
	return out
}

// mergeWithKnown carries forward enrichment (streams, VODs) that the ticker
// does not itself provide, so a refresh does not discard work.
func (d *Daemon) mergeWithKnown(known, fresh []match.Match) []match.Match {
	byID := make(map[string]match.Match, len(known))
	for _, m := range known {
		byID[m.ID] = m
	}
	for i := range fresh {
		old, ok := byID[fresh[i].ID]
		if !ok {
			continue
		}
		if len(fresh[i].Streams) == 0 && len(old.Streams) > 0 {
			fresh[i].Streams = old.Streams
		}
		if fresh[i].VOD == nil && old.VOD != nil {
			fresh[i].VOD = old.VOD
		}
		// The ticker drops the score once a match scrolls out of the recent
		// window; keep what we already recorded.
		if fresh[i].Winner == 0 && old.Winner != 0 {
			fresh[i].Score, fresh[i].Winner = old.Score, old.Winner
		}
	}
	return fresh
}

// enrich fills in streams from tournament pages and VODs from YouTube.
func (d *Daemon) enrich(ctx context.Context, ms []match.Match, priv *store.Private) error {
	now := time.Now()

	// Liquipedia only renders stream links within two hours of a match start,
	// so the tournament page is the reliable source. Fetch for the matches
	// that need it most: followed and imminent first.
	type need struct {
		idx      int
		priority int
	}
	var needs []need
	for i, m := range ms {
		if len(m.Streams) > 0 || m.Tournament.Page == "" {
			continue
		}
		if m.State == match.StateFinished {
			continue
		}
		until := m.StartsAt.Sub(now)
		if until > 48*time.Hour {
			continue
		}
		p := 0
		if m.Followed {
			p += 100
		}
		if m.State == match.StateLive {
			p += 50
		}
		if until < 3*time.Hour {
			p += 25
		}
		needs = append(needs, need{idx: i, priority: p})
	}
	sort.SliceStable(needs, func(a, b int) bool { return needs[a].priority > needs[b].priority })

	fetches := 0
	for _, n := range needs {
		page := ms[n.idx].Tournament.Page
		info, cached := priv.TournamentStreams[page]
		if !cached || time.Since(info.FetchedAt) > 24*time.Hour {
			if fetches >= d.maxTournamentFetches {
				continue
			}
			html, err := d.lp.ParsePage(ctx, ms[n.idx].Wiki, strings.TrimPrefix(strings.TrimPrefix(page, "/"+ms[n.idx].Wiki+"/"), "/"), 24*time.Hour)
			if err != nil {
				d.logger.Printf("tournament %s: %v", page, err)
				continue
			}
			fetches++
			b, err := liquipedia.ParseTournament(html)
			if err != nil {
				continue
			}
			info = store.TournamentInfo{
				FetchedAt:       time.Now(),
				Streams:         b.Streams,
				YouTubeChannels: b.YouTubeChannels,
			}
			priv.TournamentStreams[page] = info
			d.logger.Printf("tournament %s: %d streams, %d yt channels", page, len(b.Streams), len(b.YouTubeChannels))
		}
		ms[n.idx].Streams = info.Streams
	}

	// VOD discovery for finished matches.
	if d.cfg.YouTube.Enabled {
		d.discoverVODs(ctx, ms, priv)
	}
	return nil
}

// discoverVODs looks for recorded video of finished matches on the channels
// associated with their tournament.
func (d *Daemon) discoverVODs(ctx context.Context, ms []match.Match, priv *store.Private) {
	maxAge := time.Duration(d.cfg.YouTube.MaxAge)

	// Collect the channels worth checking, and which matches each serves.
	channels := map[string][]int{}
	for i, m := range ms {
		if m.State != match.StateFinished || m.VOD != nil {
			continue
		}
		if time.Since(m.StartsAt) > maxAge {
			continue
		}
		if info, ok := priv.TournamentStreams[m.Tournament.Page]; ok {
			for _, c := range info.YouTubeChannels {
				channels[c] = append(channels[c], i)
			}
		}
		for _, c := range d.cfg.YouTube.Channels {
			channels[c] = append(channels[c], i)
		}
	}

	for channelID, idxs := range channels {
		videos, err := d.yt.Videos(ctx, channelID)
		if err != nil {
			d.logger.Printf("youtube %s: %v", channelID, err)
			continue
		}
		for _, i := range idxs {
			if ms[i].VOD != nil {
				continue
			}
			if v := youtube.Match(ms[i], videos, "en", maxAge); v != nil {
				ms[i].VOD = v
				d.logger.Printf("vod for %s: %s", ms[i].Title(), v.VideoID)
			}
		}
	}
}
