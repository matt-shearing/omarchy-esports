package spoiler

import (
	"strings"
	"time"

	"github.com/contra/omarchy-esports/internal/match"
)

// Catch-up masking.
//
// The problem this solves is subtler than hiding a scoreline. Suppose you
// follow Team Spirit, they played three matches yesterday, and you have not
// watched any of them. Simply listing all three tells you they kept winning.
// Worse, their *next* fixture is itself a spoiler: if the schedule says they
// play in the Upper Bracket Final tomorrow, you know how yesterday went.
//
// So the rule is: for each followed team, the earliest unwatched finished
// match in the catch-up window is the "queue head" and is shown in full. Every
// later match involving that team — finished or upcoming — has the *other*
// side withheld, along with the score and the bracket round. You still see
// that a match exists and when it starts, so the schedule remains useful, but
// you cannot infer the result of the one you have not watched yet.
//
// Masking happens here, in the daemon, so the withheld opponent is absent from
// the published state rather than merely undrawn by the UI.

// CatchUpOptions configures masking.
type CatchUpOptions struct {
	// Teams is the follow list.
	Teams []string
	// Watched marks matches the user has already seen.
	Watched map[string]bool
	// Revealed marks matches the user explicitly unblinded; these are never
	// masked and never cause masking.
	Revealed map[string]bool
	// Window bounds how far back an unwatched match still counts as a backlog.
	// Beyond it we assume the user has moved on.
	Window time.Duration
	// Now is the reference time.
	Now time.Time
}

// ApplyCatchUp masks matches according to the rule above and returns the
// modified slice. It also sets QueueHead and Watched on the matches it visits.
func ApplyCatchUp(ms []match.Match, opts CatchUpOptions) []match.Match {
	if len(opts.Teams) == 0 || opts.Window <= 0 {
		return ms
	}
	if opts.Watched == nil {
		opts.Watched = map[string]bool{}
	}
	if opts.Revealed == nil {
		opts.Revealed = map[string]bool{}
	}

	// Record watched state on every match first, so the UI can show it.
	for i := range ms {
		if opts.Watched[ms[i].ID] {
			ms[i].Watched = true
		}
	}

	// hideSide[matchIndex][side] accumulates which opponents must be withheld.
	// Using a set means two followed teams facing each other, both with a
	// backlog, correctly ends up hiding both sides rather than neither.
	hideSide := map[int][2]bool{}
	maskedFor := map[int]string{}

	cutoff := opts.Now.Add(-opts.Window)

	// Scope each backlog to one team within one wiki. An org like Team Spirit
	// fields separate Dota 2 and Counter-Strike rosters, and being behind on
	// one game says nothing about the other, so a Dota backlog must not hide
	// their CS fixtures.
	for _, team := range opts.Teams {
		team = strings.TrimSpace(team)
		if team == "" {
			continue
		}
		for _, wiki := range wikisForTeam(ms, team) {
			applyTeamQueue(ms, team, wiki, opts, cutoff, hideSide, maskedFor)
		}
	}

	for i, sides := range hideSide {
		applyMask(&ms[i], sides, maskedFor[i])
	}
	return ms
}

// wikisForTeam lists the wikis a team appears in.
func wikisForTeam(ms []match.Match, team string) []string {
	seen := map[string]bool{}
	var out []string
	for i := range ms {
		if sideOf(&ms[i], team) < 0 {
			continue
		}
		if !seen[ms[i].Wiki] {
			seen[ms[i].Wiki] = true
			out = append(out, ms[i].Wiki)
		}
	}
	return out
}

// applyTeamQueue finds one team's queue head within one wiki and masks
// everything that team plays afterwards.
func applyTeamQueue(ms []match.Match, team, wiki string, opts CatchUpOptions,
	cutoff time.Time, hideSide map[int][2]bool, maskedFor map[int]string) {
	{
		idxs := indicesForTeam(ms, team, wiki)
		if len(idxs) == 0 {
			return
		}

		head := -1
		for _, i := range idxs {
			m := &ms[i]
			if m.State != match.StateFinished {
				continue
			}
			if m.Watched || opts.Revealed[m.ID] {
				continue
			}
			if m.StartsAt.Before(cutoff) {
				// Older than the window: assume it is no longer a backlog.
				continue
			}
			head = i
			break
		}
		if head < 0 {
			return
		}
		ms[head].QueueHead = true

		// Everything this team plays after the queue head gets the other side
		// withheld.
		for _, i := range idxs {
			if i == head || !ms[i].StartsAt.After(ms[head].StartsAt) {
				continue
			}
			if opts.Revealed[ms[i].ID] {
				continue
			}
			side := sideOf(&ms[i], team)
			if side < 0 {
				continue
			}
			other := 1 - side
			cur := hideSide[i]
			cur[other] = true
			hideSide[i] = cur
			if maskedFor[i] == "" {
				maskedFor[i] = ms[i].Opponents[side].Display()
			}
		}
	}
}

// indicesForTeam returns the indices of matches involving a team within one
// wiki, in start order. ms is already sorted by start time when the daemon
// calls this.
func indicesForTeam(ms []match.Match, team, wiki string) []int {
	var out []int
	for i := range ms {
		if wiki != "" && ms[i].Wiki != wiki {
			continue
		}
		if sideOf(&ms[i], team) >= 0 {
			out = append(out, i)
		}
	}
	return out
}

// sideOf returns 0 or 1 for the side a team plays, or -1 if absent.
func sideOf(m *match.Match, team string) int {
	t := strings.ToLower(strings.TrimSpace(team))
	for i, o := range m.Opponents {
		if o.Hidden {
			continue
		}
		if strings.ToLower(strings.TrimSpace(o.Name)) == t ||
			strings.ToLower(strings.TrimSpace(o.Short)) == t {
			return i
		}
	}
	return -1
}

// applyMask withholds the given sides and everything else that would betray
// the result.
func applyMask(m *match.Match, sides [2]bool, forTeam string) {
	for i, hide := range sides {
		if !hide {
			continue
		}
		m.Opponents[i] = match.Opponent{Hidden: true}
	}
	m.Masked = true
	m.MaskedFor = forTeam
	m.Score = [2]int{}
	m.Winner = 0
	m.Redacted = true
	m.Tournament.Name = seriesName(m.Tournament.Name)

	// A VOD title names both teams, so it defeats the mask on its own.
	if m.VOD != nil {
		v := *m.VOD
		v.Title = ""
		v.Thumbnail = ""
		m.VOD = &v
	}
}

// seriesName strips the stage suffix from a tournament label, because the
// stage is itself a progression spoiler: "TI 2026 - Upper Bracket Final" says
// more about yesterday's result than the fixture does.
func seriesName(name string) string {
	if i := strings.Index(name, " - "); i > 0 {
		return strings.TrimSpace(name[:i])
	}
	return name
}
