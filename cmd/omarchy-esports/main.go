// Command omarchy-esports tracks upcoming esports matches, keeps a
// spoiler-free view of finished ones, and feeds the omarchy bar plugin.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/contra/omarchy-esports/internal/config"
	"github.com/contra/omarchy-esports/internal/daemon"
	"github.com/contra/omarchy-esports/internal/fuzzy"
	"github.com/contra/omarchy-esports/internal/liquipedia"
	"github.com/contra/omarchy-esports/internal/match"
	"github.com/contra/omarchy-esports/internal/notify"
	"github.com/contra/omarchy-esports/internal/store"
)

// version is stamped at build time; it also identifies us to Liquipedia.
var version = "0.1.0"

func main() {
	log.SetFlags(0)
	log.SetPrefix("")

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "daemon":
		err = cmdDaemon(args)
	case "status":
		err = cmdStatus(args)
	case "next":
		err = cmdNext(args)
	case "teams":
		err = cmdTeams(args)
	case "reveal":
		err = cmdReveal(args, true)
	case "hide":
		err = cmdReveal(args, false)
	case "watched":
		err = cmdWatched(args, true)
	case "unwatch":
		err = cmdWatched(args, false)
	case "setup":
		err = cmdSetup(args)
	case "logos":
		err = cmdLogos(args)
	case "games":
		err = cmdGames(args)
	case "search":
		err = cmdSearch(args)
	case "team":
		err = cmdTeam(args)
	case "refresh":
		err = cmdRefresh(args)
	case "open":
		err = cmdOpen(args)
	case "config":
		err = cmdConfig(args)
	case "version", "--version", "-v":
		fmt.Println("omarchy-esports " + version)
	case "help", "--help", "-h":
		usage()
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		log.Fatalf("omarchy-esports %s: %v", cmd, err)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `omarchy-esports — spoiler-free esports schedule for omarchy

usage: omarchy-esports <command> [flags]

commands:
  daemon           poll Liquipedia and YouTube, notify, maintain state
                     --once      run a single refresh and exit
                     --dry-run   log notifications instead of sending them
  status           show the schedule
                     --json      machine-readable
                     --all       include matches without a followed team
  next             print the next followed match, for the bar widget
                     --json
  teams            manage the follow list
                     list | add <name>... | remove <name>...
                     --game <slug>  scope to one game, e.g. --game dota2
  reveal <id>      unblind one match's result
  hide <id>        re-blind a revealed match
  watched <id>     mark a match watched, advancing the catch-up queue
  unwatch <id>     mark it unwatched again
  setup            first-run setup: pick games and teams
                     --reset   show the wizard again next time the app opens
                     --done    mark setup complete without the wizard
  logos            inspect or fill the local team-artwork cache
                     status | warm | path
  games            list known games, and turn them on or off
                     list | on <slug>... | off <slug>...
  search <query>   fuzzy-search the team index
                     --json  --game <slug>
  team <name>      show a team's upcoming matches
                     --json  --game <slug>
  refresh          force an immediate refresh
  open <id>        open a match
                     --stream    open the live stream (default)
                     --vod       open the VOD
  config           show or edit configuration
                     path | edit | show
                     set <key> <value>   e.g. set spoilers balanced
                                              set catchUp.window 72h
                                              set notifications.vodReady false
                     wiki <slug> on|off  e.g. wiki starcraft2 off

Match data from Liquipedia (CC BY-SA 3.0).
`)
}

// hoistFlags moves flags ahead of positional arguments.
//
// Go's flag package stops parsing at the first non-flag argument, so
// `search gamer --game dota2` would treat "--game" and "dota2" as search
// terms rather than as a flag. Nobody writes `search --game dota2 gamer`, so
// reorder the arguments instead of making the user do it.
//
// valueFlags names the flags that take a following value, so their argument
// travels with them.
func hoistFlags(args []string, valueFlags map[string]bool) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(a, "-") || a == "-" {
			positional = append(positional, a)
			continue
		}
		flags = append(flags, a)
		name := strings.TrimLeft(a, "-")
		// "--game=dota2" already carries its value.
		if strings.Contains(a, "=") {
			continue
		}
		if valueFlags[name] && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	// Emit an explicit terminator so the flag package treats everything after
	// it as literal — including a team name that begins with a dash.
	if len(positional) > 0 {
		flags = append(flags, "--")
	}
	return append(flags, positional...)
}

// openStore builds the state store.
func openStore() (*store.Store, error) { return store.New("") }

func cmdDaemon(args []string) error {
	fs := flag.NewFlagSet("daemon", flag.ExitOnError)
	once := fs.Bool("once", false, "run a single refresh and exit")
	dryRun := fs.Bool("dry-run", false, "log notifications instead of sending them")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	st, err := openStore()
	if err != nil {
		return err
	}

	logger := log.New(os.Stderr, "", log.Ltime)
	d := daemon.New(daemon.Options{
		Config:  cfg,
		Store:   st,
		Version: version,
		Logger:  logger,
		DryRun:  *dryRun,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if *once {
		return d.RefreshOnce(ctx)
	}
	err = d.Run(ctx)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func cmdRefresh(args []string) error {
	// A refresh is just a one-shot daemon run; the file lock is implicit in
	// atomic writes, so this is safe alongside a running daemon.
	return cmdDaemon(append([]string{"--once"}, args...))
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "machine-readable output")
	all := fs.Bool("all", false, "include matches without a followed team")
	limit := fs.Int("limit", 25, "maximum matches to show")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	pub, err := st.LoadPublic()
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("no state yet — run `omarchy-esports refresh` first")
		}
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(pub)
	}

	ms := pub.Matches
	if !*all {
		var followed []match.Match
		for _, m := range ms {
			if m.Followed {
				followed = append(followed, m)
			}
		}
		// Falling back rather than printing nothing keeps the command useful
		// before the user has set up a follow list.
		if len(followed) > 0 {
			ms = followed
		}
	}
	if len(ms) == 0 {
		fmt.Println("no matches")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "WHEN\tMATCH\tFORMAT\tTOURNAMENT\tID")
	shown := 0
	for _, m := range ms {
		if shown >= *limit {
			break
		}
		shown++
		star := " "
		if m.Followed {
			star = "*"
		}
		fmt.Fprintf(w, "%s%s\t%s\t%s\t%s\t%s\n",
			star, whenLabel(m), matchLabel(m), formatLabel(m), truncate(m.Tournament.Name, 34), m.ID[:8])
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if pub.Spoilers != string(config.SpoilerOff) {
		fmt.Printf("\nspoilers: %s — results hidden; `omarchy-esports reveal <id>` to unblind\n", pub.Spoilers)
	}
	for _, e := range pub.Errors {
		fmt.Fprintf(os.Stderr, "warning: %s\n", e)
	}
	fmt.Printf("updated %s · %s\n", humanAgo(pub.UpdatedAt), store.Attribution)
	return nil
}

// matchLabel renders the fixture, adding the score only when it is present,
// which under a strict policy means only when the user revealed it.
func matchLabel(m match.Match) string {
	a, b := m.Opponents[0].Display(), m.Opponents[1].Display()
	if m.Redacted {
		return fmt.Sprintf("%s vs %s", a, b)
	}
	if m.State == match.StateFinished && (m.Score[0] > 0 || m.Score[1] > 0) {
		return fmt.Sprintf("%s %d–%d %s", a, m.Score[0], m.Score[1], b)
	}
	return fmt.Sprintf("%s vs %s", a, b)
}

func formatLabel(m match.Match) string {
	if m.BestOf > 0 {
		return fmt.Sprintf("Bo%d", m.BestOf)
	}
	return ""
}

func whenLabel(m match.Match) string {
	switch m.State {
	case match.StateLive:
		return "LIVE"
	case match.StateFinished:
		if m.VOD != nil {
			return "VOD"
		}
		return "done"
	}
	d := time.Until(m.StartsAt)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
	default:
		return m.StartsAt.Local().Format("Mon 15:04")
	}
}

func cmdNext(args []string) error {
	fs := flag.NewFlagSet("next", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	pub, err := st.LoadPublic()
	if err != nil {
		return err
	}
	now := time.Now()
	var best *match.Match
	for i := range pub.Matches {
		m := &pub.Matches[i]
		if m.State == match.StateFinished {
			continue
		}
		if !m.Followed && len(pub.Teams) > 0 {
			continue
		}
		if m.State == match.StateLive {
			best = m
			break
		}
		if m.StartsAt.After(now) && (best == nil || m.StartsAt.Before(best.StartsAt)) {
			best = m
		}
	}
	if best == nil {
		if *asJSON {
			fmt.Println(`{"empty":true}`)
			return nil
		}
		fmt.Println("no upcoming matches")
		return nil
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		return enc.Encode(best)
	}
	fmt.Printf("%s  %s  %s\n", whenLabel(*best), matchLabel(*best), best.Tournament.Name)
	return nil
}

func cmdTeams(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(args) == 0 || args[0] == "list" {
		if len(cfg.Teams) == 0 {
			fmt.Println("no teams followed — `omarchy-esports teams add \"Team Spirit\"`")
			return nil
		}
		sorted := append([]config.Follow(nil), cfg.Teams...)
		sort.SliceStable(sorted, func(i, j int) bool {
			if sorted[i].Name != sorted[j].Name {
				return sorted[i].Name < sorted[j].Name
			}
			return sorted[i].Wiki < sorted[j].Wiki
		})
		for _, t := range sorted {
			fmt.Println(t.Label())
		}
		return nil
	}

	action := args[0]
	fs := flag.NewFlagSet("teams", flag.ExitOnError)
	// Orgs field rosters in several games, so a follow can be scoped to one.
	game := fs.String("game", "", "restrict to one wiki slug (dota2, counterstrike, starcraft2)")
	if err := fs.Parse(hoistFlags(args[1:], map[string]bool{"game": true})); err != nil {
		return err
	}
	names := fs.Args()
	if len(names) == 0 {
		return fmt.Errorf("%s needs at least one team name", action)
	}
	wiki := strings.ToLower(strings.TrimSpace(*game))
	if wiki != "" {
		// Following a team in a game that is not currently polled is
		// legitimate: search spans every game, and enabling the game later
		// should not require re-adding the team. Only reject a slug we have
		// never heard of.
		if !cfg.KnownWiki(wiki) {
			var slugs []string
			for _, w := range cfg.Wikis {
				slugs = append(slugs, w.Slug)
			}
			return fmt.Errorf("unknown game %q (known: %s)", wiki, strings.Join(slugs, ", "))
		}
		if action == "add" && !cfg.WikiEnabled(wiki) {
			fmt.Fprintf(os.Stderr,
				"note: %s is not currently polled — enable it to see fixtures: omarchy-esports games on %s\n",
				wiki, wiki)
		}
	}

	switch action {
	case "add":
		for _, n := range names {
			n = strings.TrimSpace(n)
			if cfg.FollowIndex(n, wiki) >= 0 {
				continue
			}
			cfg.Teams = append(cfg.Teams, config.Follow{Name: n, Wiki: wiki})
			fmt.Println("following", config.Follow{Name: n, Wiki: wiki}.Label())
		}
	case "remove", "rm":
		for _, n := range names {
			n = strings.TrimSpace(n)
			kept := cfg.Teams[:0]
			for _, t := range cfg.Teams {
				// With no --game, remove every scope for that name; with one,
				// remove only the matching entry.
				drop := strings.EqualFold(strings.TrimSpace(t.Name), n) &&
					(wiki == "" || strings.EqualFold(t.Wiki, wiki))
				if drop {
					fmt.Println("unfollowed", t.Label())
					continue
				}
				kept = append(kept, t)
			}
			cfg.Teams = kept
		}
	default:
		return fmt.Errorf("unknown teams action %q (want list, add or remove)", action)
	}
	return config.Save(cfg)
}

func cmdReveal(args []string, reveal bool) error {
	if len(args) == 0 {
		return errors.New("need a match id (see `omarchy-esports status`)")
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	id, err := resolveID(st, args[0])
	if err != nil {
		return err
	}
	if err := st.SetRevealed(id, reveal); err != nil {
		return err
	}
	// Re-publish immediately so the UI updates without waiting for a poll.
	return cmdRefreshQuiet()
}

// cmdRefreshQuiet re-publishes the redacted view from existing data.
func cmdRefreshQuiet() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	d := daemon.New(daemon.Options{
		Config:  cfg,
		Store:   st,
		Version: version,
		Logger:  log.New(os.Stderr, "", 0),
	})
	return d.Reevaluate(context.Background())
}

// resolveID accepts a full id or an unambiguous prefix.
func resolveID(st *store.Store, prefix string) (string, error) {
	pub, err := st.LoadPublic()
	if err != nil {
		return "", err
	}
	var hits []string
	for _, m := range pub.Matches {
		if strings.HasPrefix(m.ID, prefix) {
			hits = append(hits, m.ID)
		}
	}
	switch len(hits) {
	case 0:
		return "", fmt.Errorf("no match with id starting %q", prefix)
	case 1:
		return hits[0], nil
	default:
		return "", fmt.Errorf("%q is ambiguous (%d matches)", prefix, len(hits))
	}
}

func cmdWatched(args []string, watched bool) error {
	if len(args) == 0 {
		return errors.New("need a match id (see `omarchy-esports status`)")
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	id, err := resolveID(st, args[0])
	if err != nil {
		return err
	}
	if err := st.SetWatched(id, watched); err != nil {
		return err
	}
	// Republish so the queue advances immediately rather than at the next poll.
	return cmdRefreshQuiet()
}

// teamIndex is the shape of teams.json.
type teamIndex struct {
	Teams []store.TeamEntry `json:"teams"`
}

func loadTeamIndex(st *store.Store) ([]store.TeamEntry, error) {
	data, err := os.ReadFile(st.TeamsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, errors.New("no team index yet — run `omarchy-esports refresh` first")
		}
		return nil, err
	}
	var idx teamIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return idx.Teams, nil
}

// cmdSetup drives first-run setup from a terminal, and controls whether the
// app shows its wizard.
func cmdSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	reset := fs.Bool("reset", false, "show the setup wizard again")
	done := fs.Bool("done", false, "mark setup complete without running it")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	switch {
	case *reset:
		cfg.SetupComplete = false
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Println("setup will run again next time the app opens")
		return nil
	case *done:
		cfg.SetupComplete = true
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Println("setup marked complete")
		return nil
	}

	// Plain summary plus the commands to finish, rather than a prompt-driven
	// flow: this runs in scripts and over ssh as often as interactively.
	fmt.Println("omarchy-esports setup")
	fmt.Println()
	fmt.Println("Games currently enabled:")
	for _, w := range cfg.EnabledWikis() {
		fmt.Printf("  %-6s %s\n", w.Short, w.Game)
	}
	fmt.Printf("\n%d more available — `omarchy-esports games list`\n", len(cfg.Wikis)-len(cfg.EnabledWikis()))
	fmt.Println()
	if len(cfg.Teams) == 0 {
		fmt.Println("No teams followed yet. Find some:")
		fmt.Println("  omarchy-esports search navi")
		fmt.Println("  omarchy-esports teams add \"Natus Vincere\" --game counterstrike")
	} else {
		fmt.Println("Teams followed:")
		for _, t := range cfg.Teams {
			fmt.Println("  " + t.Label())
		}
	}
	fmt.Println()
	fmt.Println("The app has a guided version of this: omarchy-esports-app")
	fmt.Println("Mark this done with: omarchy-esports setup --done")
	return nil
}

// cmdLogos reports on, and can force-fill, the artwork cache.
//
// Artwork is fetched once and served from disk. When Liquipedia is rate
// limiting, the cache fills in over subsequent refreshes and the UI shows
// monograms meanwhile — this command makes that state visible rather than
// leaving the user wondering why logos are missing.
func cmdLogos(args []string) error {
	action := "status"
	if len(args) > 0 {
		action = args[0]
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	cache := liquipedia.NewLogoCache(store.CacheDir())

	switch action {
	case "path":
		fmt.Println(filepath.Join(store.CacheDir(), "logos"))
		return nil

	case "status", "warm":
		pub, err := st.LoadPublic()
		if err != nil {
			return errors.New("no state yet — run `omarchy-esports refresh` first")
		}
		// Collapse to canonical files: the same logo appears under several
		// URLs depending on the size it was drawn at.
		wanted := map[string]bool{}
		for _, m := range pub.Matches {
			for _, o := range m.Opponents {
				for _, u := range []string{o.Logo.Light, o.Logo.Dark} {
					if u == "" || strings.HasPrefix(u, "file://") {
						continue
					}
					wanted[liquipedia.CanonicalLogoURL(u)] = true
				}
			}
		}
		var missing []string
		for u := range wanted {
			if !cache.Has(u) {
				missing = append(missing, u)
			}
		}
		sort.Strings(missing)

		have := len(wanted) - len(missing)
		fmt.Printf("artwork cache: %d of %d file(s) present\n", have, len(wanted))
		fmt.Println("  " + filepath.Join(store.CacheDir(), "logos"))

		if action == "status" {
			if len(missing) > 0 {
				fmt.Printf("\n%d missing. Fill them now with: omarchy-esports logos warm\n", len(missing))
			}
			return nil
		}

		if len(missing) == 0 {
			fmt.Println("nothing to fetch")
			return nil
		}

		cfg, _ := config.Load()
		ua := liquipedia.New(version, cfg.ContactEmail, store.CacheDir()).UserAgent()
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		fmt.Printf("fetching %d file(s)…\n", len(missing))
		done := 0
		for _, u := range missing {
			if _, err := cache.Fetch(ctx, u, ua); err != nil {
				if errors.Is(err, liquipedia.ErrBackoff) {
					fmt.Printf("\nLiquipedia is rate limiting this address; stopped after %d.\n", done)
					fmt.Println("The daemon retries automatically, and the UI shows monograms meanwhile.")
					return nil
				}
				fmt.Fprintf(os.Stderr, "  %v\n", err)
				continue
			}
			done++
		}
		fmt.Printf("cached %d file(s)\n", done)
		return nil

	default:
		return fmt.Errorf("unknown logos action %q (want status, warm or path)", action)
	}
}

// cmdGames lists and toggles the games polled.
func cmdGames(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	action := "list"
	if len(args) > 0 {
		action = args[0]
	}

	if action == "list" {
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "GAME\tSLUG\tTICKER PAGE\tSTATE")
		configured := map[string]bool{}
		for _, wiki := range cfg.Wikis {
			configured[wiki.Slug] = true
			state := "off"
			if wiki.Enabled {
				state = "on"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", wiki.Game, wiki.Slug, wiki.TickerPage, state)
		}
		// Catalog entries the user's config predates.
		for _, e := range config.Catalog {
			if !configured[e.Slug] {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", e.Game, e.Slug, e.TickerPage, "available")
			}
		}
		if err := w.Flush(); err != nil {
			return err
		}
		fmt.Println("\nEach enabled game costs one page parse per refresh, and Liquipedia")
		fmt.Println("allows one every 30 seconds — so more games means slower refreshes.")
		return nil
	}

	slugs := args[1:]
	if len(slugs) == 0 {
		return fmt.Errorf("%s needs at least one game slug (see `omarchy-esports games list`)", action)
	}
	on := action == "on"
	if action != "on" && action != "off" {
		return fmt.Errorf("unknown games action %q (want list, on or off)", action)
	}

	for _, slug := range slugs {
		slug = strings.ToLower(strings.TrimSpace(slug))
		found := false
		for i := range cfg.Wikis {
			if strings.EqualFold(cfg.Wikis[i].Slug, slug) {
				cfg.Wikis[i].Enabled = on
				found = true
			}
		}
		if !found {
			// Adopt it from the catalog rather than making the user hand-write
			// a wiki entry.
			e, ok := config.CatalogFor(slug)
			if !ok {
				return fmt.Errorf("unknown game %q — `omarchy-esports games list` shows what is available", slug)
			}
			cfg.Wikis = append(cfg.Wikis, e.Wiki(on))
		}
		fmt.Printf("%s %s\n", slug, map[bool]string{true: "enabled", false: "disabled"}[on])
	}
	return config.Save(cfg)
}

func cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "machine-readable output")
	limit := fs.Int("limit", 15, "maximum results")
	game := fs.String("game", "", "restrict to one wiki slug (dota2, counterstrike, starcraft2)")
	if err := fs.Parse(hoistFlags(args, map[string]bool{"game": true, "limit": true})); err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if query == "" {
		return errors.New("need a search query")
	}
	gameFilter := strings.ToLower(strings.TrimSpace(*game))
	st, err := openStore()
	if err != nil {
		return err
	}
	teams, err := loadTeamIndex(st)
	if err != nil {
		return err
	}
	cfg, _ := config.Load()

	type scored struct {
		store.TeamEntry
		Score    int  `json:"score"`
		Followed bool `json:"followed"`
	}
	var hits []scored
	for _, t := range teams {
		if gameFilter != "" && !strings.EqualFold(t.Wiki, gameFilter) {
			continue
		}
		sc := fuzzy.Best(query, t.Name, t.Short)
		if sc == fuzzy.NoMatch {
			continue
		}
		// A team with a fixture in the current window is far more likely to be
		// the one being searched for than a dormant directory entry.
		if t.Playing {
			sc += 50
		}
		hits = append(hits, scored{TeamEntry: t, Score: sc, Followed: cfg.Follows(t.Name, t.Wiki)})
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > *limit {
		hits = hits[:*limit]
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(hits)
	}
	if len(hits) == 0 {
		fmt.Printf("no team matching %q in the index (%d known)\n", query, len(teams))
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "TEAM\tSHORT\tGAME\tSTATUS\tFOLLOWED")
	for _, h := range hits {
		star := ""
		if h.Followed {
			star = "*"
		}
		status := ""
		if h.Playing {
			status = "playing"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", h.Name, h.Short, h.Game, status, star)
	}
	if err := w.Flush(); err != nil {
		return err
	}
	if len(hits) > 0 && hits[0].Wiki != "" {
		fmt.Println("\nfollow one game only:  omarchy-esports teams add \"" +
			hits[0].Name + "\" --game " + hits[0].Wiki)
	}
	return nil
}

func cmdTeam(args []string) error {
	fs := flag.NewFlagSet("team", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "machine-readable output")
	game := fs.String("game", "", "restrict to one wiki slug")
	if err := fs.Parse(hoistFlags(args, map[string]bool{"game": true})); err != nil {
		return err
	}
	name := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if name == "" {
		return errors.New("need a team name")
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	pub, err := st.LoadPublic()
	if err != nil {
		return err
	}
	wiki := strings.ToLower(strings.TrimSpace(*game))
	var out []match.Match
	for _, m := range pub.Matches {
		if m.InvolvesTeam(name, wiki) {
			out = append(out, m)
		}
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	if len(out) == 0 {
		fmt.Printf("no scheduled matches for %q\n", name)
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "WHEN\tMATCH\tTOURNAMENT\tID")
	for _, m := range out {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			whenLabel(m), matchLabel(m), truncate(m.Tournament.Name, 34), m.ID[:8])
	}
	return w.Flush()
}

func cmdOpen(args []string) error {
	fs := flag.NewFlagSet("open", flag.ExitOnError)
	wantVOD := fs.Bool("vod", false, "open the VOD")
	wantStream := fs.Bool("stream", false, "open the live stream")
	if err := fs.Parse(hoistFlags(args, nil)); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("need a match id")
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	pub, err := st.LoadPublic()
	if err != nil {
		return err
	}
	id, err := resolveID(st, fs.Arg(0))
	if err != nil {
		return err
	}
	var m *match.Match
	for i := range pub.Matches {
		if pub.Matches[i].ID == id {
			m = &pub.Matches[i]
			break
		}
	}
	if m == nil {
		return fmt.Errorf("match %s not found", id)
	}

	url := ""
	switch {
	case *wantVOD:
		if m.VOD == nil {
			return errors.New("no VOD known for this match yet")
		}
		url = m.VOD.URL
	case *wantStream:
		if s := liquipedia.PreferredStream(m.Streams); s != nil {
			url = s.URL
		}
	default:
		// Default to whatever is watchable now.
		if s := liquipedia.PreferredStream(m.Streams); s != nil {
			url = s.URL
		} else if m.VOD != nil {
			url = m.VOD.URL
		}
	}
	if url == "" {
		return errors.New("nothing to open for this match yet")
	}
	cmd := notify.OpenCommand(url)
	if err := exec.Command("bash", "-lc", cmd).Start(); err != nil {
		return err
	}
	// Opening the recording is the natural signal that this match has been
	// watched, which advances the catch-up queue. `unwatch` undoes it.
	if m.VOD != nil && url == m.VOD.URL {
		if err := st.SetWatched(m.ID, true); err != nil {
			return err
		}
		return cmdRefreshQuiet()
	}
	return nil
}

func cmdConfig(args []string) error {
	action := "show"
	if len(args) > 0 {
		action = args[0]
	}
	switch action {
	case "path":
		fmt.Println(config.Path())
	case "edit":
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "nvim"
		}
		if _, err := config.Load(); err != nil {
			return err
		}
		c := exec.Command(editor, config.Path())
		c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
		return c.Run()
	case "show":
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cfg)
	case "set":
		if len(args) < 3 {
			return errors.New("usage: config set <key> <value>   e.g. config set spoilers balanced")
		}
		return configSet(args[1], strings.Join(args[2:], " "))
	case "wiki":
		if len(args) < 3 {
			return errors.New("usage: config wiki <slug> on|off")
		}
		return configWiki(args[1], args[2])
	default:
		return fmt.Errorf("unknown config action %q (want path, edit, show, set or wiki)", action)
	}
	return nil
}

// configSet writes a dotted key into the config file.
//
// It edits the file as generic JSON and then re-decodes it into Config, so
// every value still goes through the same validation and clamping as a
// hand-edited file — a bad duration or spoiler mode is rejected here rather
// than silently accepted and corrected at load time.
func configSet(key, raw string) error {
	if _, err := config.Load(); err != nil { // ensure the file exists
		return err
	}
	data, err := os.ReadFile(config.Path())
	if err != nil {
		return err
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing %s: %w", config.Path(), err)
	}

	parts := strings.Split(key, ".")
	cur := doc
	for i, p := range parts[:len(parts)-1] {
		next, ok := cur[p].(map[string]any)
		if !ok {
			if cur[p] != nil {
				return fmt.Errorf("%s is not a section", strings.Join(parts[:i+1], "."))
			}
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
	cur[parts[len(parts)-1]] = parseScalar(raw)

	merged, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	// Decode into a zero Config, not the defaults: unmarshalling an array into
	// a slice that already has elements reuses those elements, so absent
	// fields would inherit values from whichever default sat at that index.
	var cfg config.Config
	if err := json.Unmarshal(merged, &cfg); err != nil {
		return fmt.Errorf("%s = %q is not valid: %w", key, raw, err)
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Printf("%s = %v\n", key, cur[parts[len(parts)-1]])
	return nil
}

// parseScalar turns a command-line value into the JSON type it looks like, so
// `config set catchUp.enabled false` stores a boolean rather than a string.
func parseScalar(raw string) any {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "yes", "on":
		return true
	case "false", "no", "off":
		return false
	}
	if n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
		return n
	}
	return raw
}

// configWiki enables or disables one game.
func configWiki(slug, state string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	on := parseScalar(state) == true
	found := false
	for i := range cfg.Wikis {
		if strings.EqualFold(cfg.Wikis[i].Slug, slug) {
			cfg.Wikis[i].Enabled = on
			found = true
		}
	}
	if !found {
		return fmt.Errorf("no wiki %q in the config; add it under \"wikis\" first", slug)
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Printf("%s %s\n", slug, map[bool]string{true: "enabled", false: "disabled"}[on])
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func humanAgo(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
}
