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
	"sort"
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
  reveal <id>      unblind one match's result
  hide <id>        re-blind a revealed match
  watched <id>     mark a match watched, advancing the catch-up queue
  unwatch <id>     mark it unwatched again
  search <query>   fuzzy-search the team index
                     --json
  team <name>      show a team's upcoming matches
                     --json
  refresh          force an immediate refresh
  open <id>        open a match
                     --stream    open the live stream (default)
                     --vod       open the VOD
  config           show or edit configuration
                     path | edit | show

Match data from Liquipedia (CC BY-SA 3.0).
`)
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
		sorted := append([]string(nil), cfg.Teams...)
		sort.Strings(sorted)
		for _, t := range sorted {
			fmt.Println(t)
		}
		return nil
	}

	action, names := args[0], args[1:]
	if len(names) == 0 {
		return fmt.Errorf("%s needs at least one team name", action)
	}
	switch action {
	case "add":
		for _, n := range names {
			if !cfg.Follows(n) {
				cfg.Teams = append(cfg.Teams, n)
				fmt.Println("following", n)
			}
		}
	case "remove", "rm":
		keep := cfg.Teams[:0]
		for _, t := range cfg.Teams {
			drop := false
			for _, n := range names {
				if strings.EqualFold(strings.TrimSpace(t), strings.TrimSpace(n)) {
					drop = true
					fmt.Println("unfollowed", t)
				}
			}
			if !drop {
				keep = append(keep, t)
			}
		}
		cfg.Teams = keep
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

func cmdSearch(args []string) error {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "machine-readable output")
	limit := fs.Int("limit", 15, "maximum results")
	if err := fs.Parse(args); err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if query == "" {
		return errors.New("need a search query")
	}
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
		if s := fuzzy.Best(query, t.Name, t.Short); s != fuzzy.NoMatch {
			hits = append(hits, scored{TeamEntry: t, Score: s, Followed: cfg.Follows(t.Name)})
		}
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
	fmt.Fprintln(w, "TEAM\tSHORT\tGAME\tFOLLOWED")
	for _, h := range hits {
		star := ""
		if h.Followed {
			star = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", h.Name, h.Short, h.Game, star)
	}
	return w.Flush()
}

func cmdTeam(args []string) error {
	fs := flag.NewFlagSet("team", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "machine-readable output")
	if err := fs.Parse(args); err != nil {
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
	var out []match.Match
	for _, m := range pub.Matches {
		if m.Involves([]string{name}) {
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
	if err := fs.Parse(args); err != nil {
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
	default:
		return fmt.Errorf("unknown config action %q", action)
	}
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
