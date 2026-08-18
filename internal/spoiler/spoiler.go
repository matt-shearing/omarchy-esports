// Package spoiler withholds match results.
//
// The guiding rule is that spoiler-freedom is enforced by construction, not by
// convention: the daemon writes a redacted view to the file the UI reads, so a
// score the user has not asked to see is absent from that file entirely. A UI
// bug, a stray log line, or someone running `jq` over the state file therefore
// cannot leak a result. Full data lives in a separate private file that only
// the daemon reads.
package spoiler

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/contra/omarchy-esports/internal/config"
	"github.com/contra/omarchy-esports/internal/match"
)

// Signal names a category of result leak found in a piece of text.
type Signal string

const (
	SignalScore       Signal = "score"       // "2-0", "16:14"
	SignalOutcome     Signal = "outcome"     // "beat", "def.", "wins"
	SignalElimination Signal = "elimination" // "eliminated", "knocked out"
	SignalAdvance     Signal = "advance"     // "advance", "qualify"
	SignalTitle       Signal = "title"       // "champion", "lifts the trophy"
	SignalReaction    Signal = "reaction"    // "insane comeback", "shocking upset"
	// SignalSeriesLength covers leaks that a plain scoreline regex misses: the
	// mere existence of a late game reveals the score before it. See
	// SeriesLengthLeaks.
	SignalSeriesLength Signal = "series-length"
)

type pattern struct {
	signal Signal
	re     *regexp.Regexp
}

// patterns are the result-leaking constructions common in English esports VOD
// titles. Ordered roughly by how reliably they indicate a spoiler.
var patterns = []pattern{
	// A bare scoreline is the most common and most damaging leak. Require word
	// boundaries so "CS2" or "Bo3" do not trip it, and cap each side at two
	// digits so dates and years do not either.
	{SignalScore, regexp.MustCompile(`(?i)\b(\d{1,2})\s*[-–:]\s*(\d{1,2})\b`)},
	{SignalOutcome, regexp.MustCompile(`(?i)\b(beats?|beaten|def\.?|defeats?|defeated|destroys?|crushes?|dominates?|sweeps?|swept|upsets?|wins?|won|loses?|lost|takes? (?:it|the series)|victor(?:y|ious))\b`)},
	{SignalElimination, regexp.MustCompile(`(?i)\b(eliminat\w*|knocked out|sent home|out of the tournament|ends? .{0,20}run)\b`)},
	{SignalAdvance, regexp.MustCompile(`(?i)\b(advanc\w*|qualif\w*|through to|into the (?:grand )?final|book(?:s|ed)? (?:their|a) (?:spot|place))\b`)},
	{SignalTitle, regexp.MustCompile(`(?i)\b(champions?|championship win|lifts? the|wins? it all|takes? the (?:title|trophy|crown)|crowned)\b`)},
	{SignalReaction, regexp.MustCompile(`(?i)\b(insane|unbelievable|shocking|stunning|incredible) (?:comeback|upset|finish|ending|reverse)\b`)},
}

// gameNumberRe finds "Game 3" / "Map 2" markers in a VOD title.
var gameNumberRe = regexp.MustCompile(`(?i)\b(?:game|map)\s*(\d{1,2})\b`)

// SeriesLengthLeaks reports whether a title leaks the score purely by naming
// which game of the series it covers.
//
// In a best-of-N, a side needs ceil(N/2) wins to close the series out, so game
// k can only exist if neither side had got there after k-1 games. Game 3 of a
// best-of-three therefore means it was 1-1, and game 5 of a best-of-five means
// it was 2-2. Titles like "[EN] LGD vs Yandex - Game 3 - The International"
// carry no scoreline at all and read as perfectly safe, which is exactly why
// this is worth catching.
//
// bestOf of 0 means the format is unknown, in which case we cannot reason
// about it and report no leak.
func SeriesLengthLeaks(title string, bestOf int) bool {
	if bestOf <= 1 {
		return false
	}
	m := gameNumberRe.FindStringSubmatch(title)
	if m == nil {
		return false
	}
	game, err := strconv.Atoi(m[1])
	if err != nil {
		return false
	}
	winsNeeded := (bestOf + 1) / 2
	return game > winsNeeded
}

// ScanVOD scans a VOD title in the context of its series format, catching the
// format-dependent leaks that Scan alone cannot see.
func ScanVOD(title string, bestOf int) []Signal {
	sigs := Scan(title)
	if SeriesLengthLeaks(title, bestOf) {
		sigs = append(sigs, SignalSeriesLength)
	}
	return sigs
}

// Scan reports which result-leak signals appear in a piece of text.
func Scan(text string) []Signal {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	var out []Signal
	seen := map[Signal]bool{}
	for _, p := range patterns {
		if p.re.MatchString(text) && !seen[p.signal] {
			seen[p.signal] = true
			out = append(out, p.signal)
		}
	}
	return out
}

// IsSpoilery reports whether text leaks a result.
func IsSpoilery(text string) bool { return len(Scan(text)) > 0 }

// Strings renders signals for display or JSON.
func Strings(sigs []Signal) []string {
	out := make([]string, 0, len(sigs))
	for _, s := range sigs {
		out = append(out, string(s))
	}
	return out
}

// Redact returns a copy of m safe to hand to the UI under the given policy.
//
// revealed reports whether the user has explicitly unblinded this match.
// A revealed match is returned untouched.
func Redact(m match.Match, mode config.SpoilerMode, revealed bool) match.Match {
	m.Revealed = revealed
	if revealed || mode == config.SpoilerOff {
		return m
	}
	// Only finished matches carry results worth hiding. An upcoming or live
	// match has nothing to spoil, and blacking it out would make the ticker
	// useless for its main job.
	if m.State != match.StateFinished {
		return m
	}

	m.Redacted = true
	m.Score = [2]int{}
	m.Winner = 0

	if m.VOD != nil {
		vod := *m.VOD
		sigs := ScanVOD(vod.Title, m.BestOf)
		vod.Spoilery = Strings(sigs)

		// A series-length leak cannot be scrubbed away: removing "Game 3"
		// would leave a title that no longer describes the video. Withhold the
		// title outright even under the balanced policy.
		if containsSignal(sigs, SignalSeriesLength) && mode == config.SpoilerBalanced {
			vod.Title = ""
			vod.Thumbnail = ""
			m.VOD = &vod
			return m
		}

		switch mode {
		case config.SpoilerStrict:
			// Withhold the title, the artwork and the runtime. Runtime matters:
			// a 45-minute Bo3 VOD can only be a 2-0. Kind survives: knowing a
			// video is a highlights package reveals no result, and the user
			// needs the warning before clicking one.
			vod.Title = ""
			vod.Thumbnail = ""
		case config.SpoilerBalanced:
			// Keep a scrubbed title and the runtime, drop the artwork, which
			// routinely shows celebrations and score overlays.
			vod.Title = Scrub(vod.Title)
			vod.Thumbnail = ""
		}
		m.VOD = &vod
	}
	return m
}

// scrubbers rewrite leaking fragments into neutral placeholders, so a title
// stays informative ("Grand Final — Day 3") without carrying the result.
var scrubbers = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`(?i)\b\d{1,2}\s*[-–:]\s*\d{1,2}\b`), "–"},
	{regexp.MustCompile(`(?i)\s*[|\-–—]?\s*\b(?:beats?|def\.?|defeats?|defeated|destroys?|crushes?|dominates?|sweeps?|upsets?)\b\s*`), " vs "},
	{regexp.MustCompile(`(?i)\b(eliminat\w*|advanc\w*|qualif\w*|champions?|crowned|wins? it all)\b`), "…"},
}

// Scrub removes result-leaking fragments from a title while keeping the rest.
func Scrub(title string) string {
	out := title
	for _, s := range scrubbers {
		out = s.re.ReplaceAllString(out, s.with)
	}
	// Collapse the whitespace and separator debris the substitutions leave.
	out = regexp.MustCompile(`\s{2,}`).ReplaceAllString(out, " ")
	out = regexp.MustCompile(`(?:\s*[|\-–—]\s*){2,}`).ReplaceAllString(out, " — ")
	out = strings.Trim(out, " |-–—…")
	return strings.TrimSpace(out)
}

func containsSignal(sigs []Signal, want Signal) bool {
	for _, s := range sigs {
		if s == want {
			return true
		}
	}
	return false
}

// RedactAll applies Redact across a slice, consulting revealed for each id.
func RedactAll(ms []match.Match, mode config.SpoilerMode, revealed map[string]bool) []match.Match {
	out := make([]match.Match, 0, len(ms))
	for _, m := range ms {
		out = append(out, Redact(m, mode, revealed[m.ID]))
	}
	return out
}
