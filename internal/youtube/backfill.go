package youtube

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/contra/omarchy-esports/internal/match"
)

// Backfiller recovers VODs that have already scrolled out of the RSS feed.
//
// A channel's Atom feed carries only its ~15 most recent uploads. For an
// ordinary schedule that is plenty, but a large event breaks it badly: the
// official Dota 2 channel posts every game of every series in four languages,
// so 15 entries is under a day of coverage and a Tuesday fixture is
// unreachable by Thursday. No amount of polling recovers those — they were
// never in a feed we saw.
//
// yt-dlp reads the channel's video tab directly and reaches back hundreds of
// uploads with no API key and no quota. The catch is that its cheap listing
// mode (--flat-playlist, one request per page) returns ids and titles but no
// publish dates, and Match needs a date to tie a video to a fixture. Fetching
// dates for hundreds of videos would mean hundreds of requests.
//
// So this runs in two phases: list titles cheaply, shortlist only the handful
// that name both sides of a fixture we still want, then fetch dates for that
// shortlist alone. Recovering a full event costs a few dozen requests rather
// than several hundred.
//
// yt-dlp is an optional dependency. Without it VOD discovery still works for
// anything currently in the feed, which is the common case.
type Backfiller struct {
	bin string
	// limit bounds how many uploads deep the channel listing goes.
	limit int
	// timeout bounds a single yt-dlp invocation.
	timeout time.Duration
}

// unitSep separates fields in yt-dlp's output. Video titles routinely contain
// pipes, dashes and brackets, so the separator has to be a byte a title cannot
// plausibly hold.
const unitSep = "\x1f"

// NewBackfiller locates yt-dlp. The returned Backfiller is always non-nil;
// call Available to find out whether it can do anything.
func NewBackfiller(limit int) *Backfiller {
	if limit <= 0 {
		limit = 400
	}
	bin, err := exec.LookPath("yt-dlp")
	if err != nil {
		bin = ""
	}
	return &Backfiller{bin: bin, limit: limit, timeout: 5 * time.Minute}
}

// Available reports whether yt-dlp was found.
func (b *Backfiller) Available() bool { return b != nil && b.bin != "" }

// List enumerates a channel's recent uploads, newest first.
//
// Published is left zero: the listing mode that makes this cheap does not
// carry dates. Call Dates for the ones that turn out to matter.
func (b *Backfiller) List(ctx context.Context, channelID string) ([]Video, error) {
	if !b.Available() {
		return nil, fmt.Errorf("yt-dlp not installed")
	}
	url := "https://www.youtube.com/channel/" + channelID + "/videos"
	out, err := b.run(ctx, []string{
		"--flat-playlist",
		"--playlist-end", strconv.Itoa(b.limit),
		"--ignore-errors", "--no-warnings",
		"--print", "%(id)s" + unitSep + "%(title)s" + unitSep + "%(channel)s",
		url,
	})
	if err != nil {
		return nil, err
	}

	var videos []Video
	for _, line := range out {
		f := strings.Split(line, unitSep)
		if len(f) < 2 || f[0] == "" || f[0] == "NA" {
			continue
		}
		v := Video{
			ID:    f[0],
			Title: f[1],
			// Flat listing carries no thumbnail, but YouTube's derived URL is
			// stable for every video id.
			Thumbnail: "https://i.ytimg.com/vi/" + f[0] + "/hqdefault.jpg",
			Lang:      "en",
		}
		if len(f) > 2 && f[2] != "NA" {
			v.Channel = f[2]
		}
		if m := langTagRe.FindStringSubmatch(v.Title); m != nil {
			v.Lang = strings.ToLower(strings.SplitN(m[1], "-", 2)[0])
		}
		videos = append(videos, v)
	}
	return videos, nil
}

// Dates fetches publish times for specific video ids. Each id costs a request,
// so pass only ids already known to be worth resolving.
func (b *Backfiller) Dates(ctx context.Context, ids []string) (map[string]time.Time, error) {
	if !b.Available() {
		return nil, fmt.Errorf("yt-dlp not installed")
	}
	if len(ids) == 0 {
		return map[string]time.Time{}, nil
	}
	args := []string{
		"--skip-download", "--ignore-errors", "--no-warnings",
		"--print", "%(id)s" + unitSep + "%(timestamp)s",
	}
	for _, id := range ids {
		args = append(args, URL(id))
	}
	out, err := b.run(ctx, args)
	// A single unavailable video must not discard the ids that did resolve,
	// so partial output is kept even on a non-zero exit.
	dates := map[string]time.Time{}
	for _, line := range out {
		f := strings.Split(line, unitSep)
		if len(f) != 2 {
			continue
		}
		ts, cerr := strconv.ParseInt(f[1], 10, 64)
		if cerr != nil {
			continue
		}
		dates[f[0]] = time.Unix(ts, 0).UTC()
	}
	if len(dates) == 0 && err != nil {
		return nil, err
	}
	return dates, nil
}

// run executes yt-dlp and returns its stdout lines.
func (b *Backfiller) run(ctx context.Context, args []string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, b.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, b.bin, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	var lines []string
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		if line := strings.TrimRight(sc.Text(), "\r"); line != "" {
			lines = append(lines, line)
		}
	}
	err = cmd.Wait()
	return lines, err
}

// gameNoRe reads which game of a series a title covers.
var gameNoRe = regexp.MustCompile(`(?i)\b(?:game|map)\s*(\d{1,2})\b`)

// gameNo returns the game number a title names, or 1 when it names none.
func gameNo(title string) int {
	m := gameNoRe.FindStringSubmatch(title)
	if m == nil {
		return 1
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n < 1 {
		return 1
	}
	return n
}

// ShortlistFor picks the videos worth resolving a date for: those naming both
// sides of one of the given fixtures.
//
// It keeps at most perMatch candidates per fixture rather than everything that
// matches, because a single series on an official channel is many videos — one
// per game, times one per broadcast language. The International puts out
// twelve uploads for one best-of-three, and eleven of them are videos we would
// never choose. Ranking here by the same preferences Match applies later means
// the requests we spend are spent on the video that will actually be picked.
func ShortlistFor(ms []match.Match, videos []Video, preferLang string, perMatch int) []Video {
	if preferLang == "" {
		preferLang = "en"
	}
	if perMatch <= 0 {
		perMatch = 3
	}

	ranked := make([][]Video, 0, len(ms))
	for j := range ms {
		m := &ms[j]
		var cands []Video
		for i := range videos {
			if mentions(videos[i].Title, m.Opponents[0]) && mentions(videos[i].Title, m.Opponents[1]) {
				cands = append(cands, videos[i])
			}
		}
		// Preferred language first, full matches over highlight cuts, then the
		// earliest game of the series — which is also the one that cannot leak
		// the score by existing.
		sort.SliceStable(cands, func(a, b int) bool {
			va, vb := cands[a], cands[b]
			if (va.Lang == preferLang) != (vb.Lang == preferLang) {
				return va.Lang == preferLang
			}
			if IsHighlight(va.Title) != IsHighlight(vb.Title) {
				return !IsHighlight(va.Title)
			}
			return gameNo(va.Title) < gameNo(vb.Title)
		})
		if len(cands) > perMatch {
			cands = cands[:perMatch]
		}
		if len(cands) > 0 {
			ranked = append(ranked, cands)
		}
	}

	// Emit round-robin — every fixture's best candidate, then every fixture's
	// second, and so on. The caller caps this list at what it can afford to
	// resolve, and a cap applied to fixture-ordered output would strand the
	// fixtures at the tail entirely while spending three requests each on the
	// ones at the head. Breadth first means a truncated sweep still gives
	// every fixture its single best shot.
	var out []Video
	taken := map[string]bool{}
	for depth := 0; depth < perMatch; depth++ {
		for _, cands := range ranked {
			if depth >= len(cands) {
				continue
			}
			if v := cands[depth]; !taken[v.ID] {
				taken[v.ID] = true
				out = append(out, v)
			}
		}
	}
	return out
}
