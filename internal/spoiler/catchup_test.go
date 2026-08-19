package spoiler

import (
	"testing"
	"time"

	"github.com/contra/omarchy-esports/internal/config"
	"github.com/contra/omarchy-esports/internal/match"
)

func opp(name string) match.Opponent {
	return match.Opponent{Name: name, Short: name, Logo: match.Logo{Light: "https://x/" + name + ".png"}}
}

// scenario builds Spirit's day: three matches, none watched.
func scenario(now time.Time) []match.Match {
	return []match.Match{
		{
			ID: "m1", State: match.StateFinished, StartsAt: now.Add(-6 * time.Hour),
			Opponents: [2]match.Opponent{opp("Team Spirit"), opp("Iron Wing")},
			Score:     [2]int{2, 0}, Winner: 1, BestOf: 3,
			Tournament: match.Tournament{Name: "TI 2026 - Round 1"},
			VOD:        &match.VOD{VideoID: "v1", Title: "Team Spirit vs Iron Wing"},
		},
		{
			ID: "m2", State: match.StateFinished, StartsAt: now.Add(-3 * time.Hour),
			Opponents: [2]match.Opponent{opp("Team Spirit"), opp("Team Falcons")},
			Score:     [2]int{2, 1}, Winner: 1, BestOf: 3,
			Tournament: match.Tournament{Name: "TI 2026 - Upper Bracket Final"},
			VOD:        &match.VOD{VideoID: "v2", Title: "Team Spirit vs Team Falcons"},
		},
		{
			ID: "m3", State: match.StateUpcoming, StartsAt: now.Add(20 * time.Hour),
			Opponents:  [2]match.Opponent{opp("Team Spirit"), opp("Xtreme Gaming")},
			BestOf:     5,
			Tournament: match.Tournament{Name: "TI 2026 - Grand Final"},
		},
	}
}

func defaultOpts(now time.Time) CatchUpOptions {
	return CatchUpOptions{
		Teams:  []config.Follow{{Name: "Team Spirit"}},
		Window: 24 * time.Hour,
		Now:    now,
	}
}

// TestQueueHeadIsFullyVisible: the match you should watch next is the one
// thing that is not hidden.
func TestQueueHeadIsFullyVisible(t *testing.T) {
	now := time.Now()
	ms := ApplyCatchUp(scenario(now), defaultOpts(now))

	if !ms[0].QueueHead {
		t.Fatal("earliest unwatched match was not marked as the queue head")
	}
	if ms[0].Masked {
		t.Error("the queue head must not be masked")
	}
	if ms[0].Opponents[1].Hidden || ms[0].Opponents[1].Name != "Iron Wing" {
		t.Error("the queue head's opponent should be visible")
	}
}

// TestLaterMatchesHideTheOpponent is the headline behaviour: knowing who
// Spirit plays next would reveal that they won.
func TestLaterMatchesHideTheOpponent(t *testing.T) {
	now := time.Now()
	ms := ApplyCatchUp(scenario(now), defaultOpts(now))

	for _, i := range []int{1, 2} {
		m := ms[i]
		if !m.Masked {
			t.Errorf("match %s should be masked", m.ID)
		}
		if m.Opponents[0].Name != "Team Spirit" {
			t.Errorf("match %s hid the followed team instead of the opponent", m.ID)
		}
		if !m.Opponents[1].Hidden {
			t.Errorf("match %s left the opponent visible", m.ID)
		}
		if m.Opponents[1].Name != "" || m.Opponents[1].Logo.Light != "" {
			t.Errorf("match %s masked opponent still carries data: %+v", m.ID, m.Opponents[1])
		}
		if m.Score != [2]int{0, 0} || m.Winner != 0 {
			t.Errorf("match %s leaked a result", m.ID)
		}
	}
}

// TestBracketStageIsStripped: "Upper Bracket Final" says more about
// yesterday's result than the fixture does.
func TestBracketStageIsStripped(t *testing.T) {
	now := time.Now()
	ms := ApplyCatchUp(scenario(now), defaultOpts(now))

	for _, i := range []int{1, 2} {
		if got := ms[i].Tournament.Name; got != "TI 2026" {
			t.Errorf("match %s tournament = %q, want the series name only", ms[i].ID, got)
		}
	}
	// The queue head keeps its full stage label; it is not a spoiler for a
	// match you are about to watch.
	if ms[0].Tournament.Name != "TI 2026 - Round 1" {
		t.Errorf("queue head tournament was altered: %q", ms[0].Tournament.Name)
	}
}

// TestMaskedVODTitleWithheld: a VOD title names both teams, defeating the mask.
func TestMaskedVODTitleWithheld(t *testing.T) {
	now := time.Now()
	ms := ApplyCatchUp(scenario(now), defaultOpts(now))
	if ms[1].VOD == nil {
		t.Fatal("VOD dropped entirely; it should stay playable")
	}
	if ms[1].VOD.Title != "" {
		t.Errorf("masked match leaked its opponent via the VOD title: %q", ms[1].VOD.Title)
	}
	if ms[1].VOD.VideoID == "" {
		t.Error("masked VOD lost its video id and is no longer playable")
	}
}

// TestWatchingAdvancesTheQueue: once you have seen m1, m2 becomes the head and
// its opponent is revealed, while m3 stays masked.
func TestWatchingAdvancesTheQueue(t *testing.T) {
	now := time.Now()
	opts := defaultOpts(now)
	opts.Watched = map[string]bool{"m1": true}
	ms := ApplyCatchUp(scenario(now), opts)

	if !ms[1].QueueHead {
		t.Fatal("m2 should have become the queue head")
	}
	if ms[1].Masked || ms[1].Opponents[1].Hidden {
		t.Error("the new queue head should be fully visible")
	}
	if !ms[2].Masked {
		t.Error("m3 should still be masked")
	}
}

// TestNoBacklogMeansNoMasking: someone up to date sees the normal schedule.
func TestNoBacklogMeansNoMasking(t *testing.T) {
	now := time.Now()
	opts := defaultOpts(now)
	opts.Watched = map[string]bool{"m1": true, "m2": true}
	ms := ApplyCatchUp(scenario(now), opts)

	for _, m := range ms {
		if m.Masked {
			t.Errorf("match %s was masked despite an empty backlog", m.ID)
		}
	}
	if ms[2].Opponents[1].Name != "Xtreme Gaming" {
		t.Error("upcoming opponent should be visible when caught up")
	}
}

// TestOldBacklogDoesNotMaskForever bounds the window, so a match missed last
// month does not hide the whole schedule.
func TestOldBacklogDoesNotMaskForever(t *testing.T) {
	now := time.Now()
	ms := scenario(now)
	ms[0].StartsAt = now.Add(-40 * 24 * time.Hour)
	ms[1].StartsAt = now.Add(-39 * 24 * time.Hour)
	out := ApplyCatchUp(ms, defaultOpts(now))
	for _, m := range out {
		if m.Masked {
			t.Errorf("match %s masked by a backlog outside the window", m.ID)
		}
	}
}

// TestRevealOverridesMasking lets the user opt out per match.
func TestRevealOverridesMasking(t *testing.T) {
	now := time.Now()
	opts := defaultOpts(now)
	opts.Revealed = map[string]bool{"m3": true}
	ms := ApplyCatchUp(scenario(now), opts)
	if ms[2].Masked {
		t.Error("an explicitly revealed match should not be masked")
	}
	if ms[2].Opponents[1].Name != "Xtreme Gaming" {
		t.Error("revealed match should show its opponent")
	}
}

// TestBothSidesFollowedHidesBoth: if two followed teams meet and both have a
// backlog, neither identity can be shown.
func TestBothSidesFollowedHidesBoth(t *testing.T) {
	now := time.Now()
	ms := []match.Match{
		{ID: "a1", State: match.StateFinished, StartsAt: now.Add(-6 * time.Hour),
			Opponents: [2]match.Opponent{opp("Team Spirit"), opp("Iron Wing")}},
		{ID: "b1", State: match.StateFinished, StartsAt: now.Add(-5 * time.Hour),
			Opponents: [2]match.Opponent{opp("Team Falcons"), opp("Liquid")}},
		{ID: "c1", State: match.StateUpcoming, StartsAt: now.Add(10 * time.Hour),
			Opponents: [2]match.Opponent{opp("Team Spirit"), opp("Team Falcons")}},
	}
	opts := defaultOpts(now)
	opts.Teams = []config.Follow{{Name: "Team Spirit"}, {Name: "Team Falcons"}}
	out := ApplyCatchUp(ms, opts)

	if !out[2].Masked {
		t.Fatal("the shared later fixture should be masked")
	}
	if !out[2].Opponents[0].Hidden || !out[2].Opponents[1].Hidden {
		t.Errorf("both sides should be hidden when both teams have a backlog: %+v", out[2].Opponents)
	}
}

// TestUnfollowedTeamsAreUnaffected keeps the rest of the schedule intact.
func TestUnfollowedTeamsAreUnaffected(t *testing.T) {
	now := time.Now()
	ms := scenario(now)
	ms = append(ms, match.Match{
		ID: "other", State: match.StateUpcoming, StartsAt: now.Add(30 * time.Hour),
		Opponents:  [2]match.Opponent{opp("MOUZ"), opp("FaZe")},
		Tournament: match.Tournament{Name: "EWC 2026 - Playoffs"},
	})
	out := ApplyCatchUp(ms, defaultOpts(now))
	last := out[len(out)-1]
	if last.Masked || last.Opponents[0].Hidden || last.Opponents[1].Hidden {
		t.Error("a match with no followed team should never be masked")
	}
	if last.Tournament.Name != "EWC 2026 - Playoffs" {
		t.Error("unrelated tournament label was altered")
	}
}

// TestBacklogIsScopedPerWiki: an org fields separate rosters per game, so
// being behind on their Dota matches must not hide their Counter-Strike
// fixtures.
func TestBacklogIsScopedPerWiki(t *testing.T) {
	now := time.Now()
	ms := []match.Match{
		{
			ID: "dota-old", Wiki: "dota2", State: match.StateFinished,
			StartsAt:  now.Add(-6 * time.Hour),
			Opponents: [2]match.Opponent{opp("Team Spirit"), opp("Iron Wing")},
		},
		{
			ID: "dota-next", Wiki: "dota2", State: match.StateUpcoming,
			StartsAt:  now.Add(10 * time.Hour),
			Opponents: [2]match.Opponent{opp("Team Spirit"), opp("Xtreme Gaming")},
		},
		{
			ID: "cs-next", Wiki: "counterstrike", State: match.StateUpcoming,
			StartsAt:  now.Add(12 * time.Hour),
			Opponents: [2]match.Opponent{opp("Team Spirit"), opp("MOUZ")},
		},
	}
	out := ApplyCatchUp(ms, defaultOpts(now))

	if !out[1].Masked {
		t.Error("the later Dota fixture should be masked by the Dota backlog")
	}
	if out[2].Masked || out[2].Opponents[1].Hidden {
		t.Error("a Counter-Strike fixture must not be masked by a Dota backlog")
	}
	if out[2].Opponents[1].Name != "MOUZ" {
		t.Errorf("CS opponent was hidden: %+v", out[2].Opponents[1])
	}
}

// TestPerWikiQueueHeads: each game gets its own queue head.
func TestPerWikiQueueHeads(t *testing.T) {
	now := time.Now()
	ms := []match.Match{
		{ID: "d1", Wiki: "dota2", State: match.StateFinished, StartsAt: now.Add(-6 * time.Hour),
			Opponents: [2]match.Opponent{opp("Team Spirit"), opp("Iron Wing")}},
		{ID: "c1", Wiki: "counterstrike", State: match.StateFinished, StartsAt: now.Add(-5 * time.Hour),
			Opponents: [2]match.Opponent{opp("Team Spirit"), opp("MOUZ")}},
	}
	out := ApplyCatchUp(ms, defaultOpts(now))
	if !out[0].QueueHead || !out[1].QueueHead {
		t.Errorf("expected a queue head per wiki, got d1=%v c1=%v", out[0].QueueHead, out[1].QueueHead)
	}
}
