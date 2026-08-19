// Package daemon orchestrates polling, enrichment, notification and state
// persistence.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
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

	// configModTime tracks the config file so edits are picked up without a
	// restart. The app and the CLI both mutate the follow list by writing this
	// file, and a daemon holding a startup snapshot would silently ignore them.
	configModTime time.Time

	// mu serialises state mutation. The first refresh runs concurrently with
	// the tick loop, so both paths can reach the store at once.
	mu sync.Mutex
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

	var modTime time.Time
	if fi, err := os.Stat(config.Path()); err == nil {
		modTime = fi.ModTime()
	}

	return &Daemon{
		cfg:                  o.Config,
		configModTime:        modTime,
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

	// Run the first refresh concurrently. It is slow by design — Liquipedia
	// allows one parse per 30 seconds, so three wikis take a couple of minutes
	// — and doing it inline would leave the shorter tick unserviced for that
	// whole time, delaying config reloads and republishes at exactly the
	// moment a new user is setting things up.
	go func() {
		if err := d.RefreshOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			d.logger.Printf("initial refresh: %v", err)
		}
	}()

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
			d.reloadConfigIfChanged()
			if err := d.Reevaluate(ctx); err != nil {
				d.logger.Printf("reevaluate: %v", err)
			}
		}
	}
}

// RefreshOnce performs a full poll: fetch tickers, enrich, notify, persist.
func (d *Daemon) RefreshOnce(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

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

	d.indexTeams(all, &priv)

	priv.Matches = all
	priv.UpdatedAt = time.Now()
	if err := d.store.SavePrivate(priv); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}
	d.dispatchNotifications(&priv)
	if err := d.store.SavePrivate(priv); err != nil {
		return fmt.Errorf("saving notification state: %w", err)
	}
	if err := d.publishTeams(priv); err != nil {
		d.logger.Printf("writing team index: %v", err)
	}
	return d.publish(priv, errs)
}

// publicTeams renders the follow list for the UI, resolving each entry's game
// label from the team index where possible.
func (d *Daemon) publicTeams() []store.PublicTeam {
	priv, err := d.store.LoadPrivate()
	var index map[string]store.TeamEntry
	if err == nil {
		index = priv.Teams
	}
	out := make([]store.PublicTeam, 0, len(d.cfg.Teams))
	for _, f := range d.cfg.Teams {
		pt := store.PublicTeam{Name: f.Name, Wiki: f.Wiki}
		if f.Wiki != "" && index != nil {
			if e, ok := index[f.Wiki+"/"+strings.ToLower(f.Name)]; ok {
				pt.Game = e.Game
			}
		}
		out = append(out, pt)
	}
	return out
}

// indexTeams accumulates every team seen in a ticker into a searchable index.
//
// Building it from matches we have already fetched means the app's follow-list
// search costs no extra API calls and works offline, and it naturally covers
// the teams that are actually competing. The index is cumulative: teams are
// kept after their matches age out, so it grows into a directory over time.
func (d *Daemon) indexTeams(ms []match.Match, priv *store.Private) {
	if priv.Teams == nil {
		priv.Teams = map[string]store.TeamEntry{}
	}
	// Drop entries written before the index was keyed per game. Those keys
	// were the bare team name, so an org playing two games collapsed into one
	// entry carrying whichever game was seen last.
	for k := range priv.Teams {
		if !strings.Contains(k, "/") {
			delete(priv.Teams, k)
		}
	}
	now := time.Now()
	for _, m := range ms {
		for _, o := range m.Opponents {
			name := strings.TrimSpace(o.Name)
			if name == "" || o.Hidden || strings.EqualFold(name, "TBD") {
				continue
			}
			// Key per game: one org can field rosters in several wikis and
			// each needs its own entry, artwork and follow state.
			key := m.Wiki + "/" + strings.ToLower(name)
			entry, ok := priv.Teams[key]
			if !ok {
				entry = store.TeamEntry{Key: key, Name: name}
			}
			entry.Key = key
			entry.Short = firstNonEmpty(o.Short, entry.Short)
			entry.Page = firstNonEmpty(o.Page, entry.Page)
			entry.Wiki = firstNonEmpty(m.Wiki, entry.Wiki)
			entry.Game = firstNonEmpty(m.Game, entry.Game)
			// Keep whichever artwork variants we have seen; a later ticker may
			// only carry one of them.
			if o.Logo.Light != "" {
				entry.Logo.Light = o.Logo.Light
			}
			if o.Logo.Dark != "" {
				entry.Logo.Dark = o.Logo.Dark
			}
			entry.LastSeen = now
			priv.Teams[key] = entry
		}
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// publishTeams writes the searchable index.
func (d *Daemon) publishTeams(priv store.Private) error {
	out := make([]store.TeamEntry, 0, len(priv.Teams))
	for _, t := range priv.Teams {
		out = append(out, t)
	}
	return d.store.SaveTeams(out)
}

// reloadConfigIfChanged picks up edits to the config file.
//
// Following a team from the app writes the config and expects the change to
// take effect; without this the daemon would keep publishing against the
// follow list it read at startup, and the button would appear to do nothing.
func (d *Daemon) reloadConfigIfChanged() {
	fi, err := os.Stat(config.Path())
	if err != nil {
		return
	}
	if !fi.ModTime().After(d.configModTime) {
		return
	}
	cfg, err := config.Load()
	if err != nil {
		d.logger.Printf("reloading config: %v", err)
		return
	}
	d.configModTime = fi.ModTime()
	d.cfg = cfg
	d.logger.Printf("config reloaded: %d team(s), spoilers=%s, catch-up=%v",
		len(cfg.Teams), cfg.Spoilers, cfg.CatchUp.Enabled)
}

// Reevaluate refreshes derived state and fires time-based notifications
// without hitting the network.
func (d *Daemon) Reevaluate(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

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
		// Re-derive rather than trusting the stored flag: the follow list can
		// change between refreshes, and a refresh is minutes away because of
		// Liquipedia's rate limit. Without this, following or unfollowing a
		// team appears to do nothing until the next poll.
		priv.Matches[i].Followed = d.isFollowed(&priv.Matches[i])
	}
	d.dispatchNotifications(&priv)
	if err := d.store.SavePrivate(priv); err != nil {
		return err
	}
	return d.publish(priv, nil)
}

// publish writes the redacted view the UI reads.
//
// Order matters. Catch-up masking runs first, on a copy, because it needs the
// real opponents to decide what to hide; the score redaction then runs over
// the already-masked records. Both happen here rather than in the UI so the
// withheld data is absent from the published file.
func (d *Daemon) publish(priv store.Private, errs []string) error {
	staged := make([]match.Match, len(priv.Matches))
	copy(staged, priv.Matches)

	if d.cfg.CatchUp.Enabled {
		staged = spoiler.ApplyCatchUp(staged, spoiler.CatchUpOptions{
			Teams:    d.cfg.Teams,
			Watched:  priv.Watched,
			Revealed: priv.Revealed,
			Window:   time.Duration(d.cfg.CatchUp.Window),
			Now:      time.Now(),
		})
	}

	visible := spoiler.RedactAll(staged, d.cfg.Spoilers, priv.Revealed)
	return d.store.SavePublic(store.Public{
		UpdatedAt: priv.UpdatedAt,
		Matches:   visible,
		Spoilers:  string(d.cfg.Spoilers),
		Teams:     d.publicTeams(),
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
		m.Followed = d.isFollowed(&m)
		if d.cfg.FollowedOnly && !m.Followed {
			continue
		}
		seen[m.ID] = true
		out = append(out, m)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].StartsAt.Before(out[j].StartsAt) })
	return out
}

// isFollowed reports whether a match involves a followed team, honouring the
// game scope on each follow entry.
func (d *Daemon) isFollowed(m *match.Match) bool {
	for _, f := range d.cfg.Teams {
		if m.InvolvesTeam(f.Name, f.Wiki) {
			return true
		}
	}
	return false
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
