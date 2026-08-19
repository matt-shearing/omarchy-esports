package daemon

import (
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/contra/omarchy-esports/internal/config"
	"github.com/contra/omarchy-esports/internal/store"
)

func testDaemon(t *testing.T, cfg config.Config) *Daemon {
	t.Helper()
	st, err := store.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &Daemon{cfg: cfg, store: st, logger: log.New(io.Discard, "", 0)}
}

func teamIndex(entries ...store.TeamEntry) map[string]store.TeamEntry {
	out := map[string]store.TeamEntry{}
	for _, e := range entries {
		out[e.Key] = e
	}
	return out
}

// Profiles cost the most rate-limited call the daemon makes, so the set of
// teams they are fetched for has to stay exactly the followed ones.
func TestProfileTargets(t *testing.T) {
	cfg := config.Config{
		Teams: []config.Follow{
			{Name: "Team Spirit"},                // every game
			{Name: "GamerLegion", Wiki: "dota2"}, // one game
		},
		Wikis: []config.Wiki{
			{Slug: "dota2", Enabled: true},
			{Slug: "counterstrike", Enabled: true},
			{Slug: "valorant", Enabled: false},
		},
	}
	priv := store.Private{Teams: teamIndex(
		store.TeamEntry{Key: "dota2/team spirit", Name: "Team Spirit", Wiki: "dota2"},
		store.TeamEntry{Key: "counterstrike/team spirit", Name: "Team Spirit", Wiki: "counterstrike"},
		store.TeamEntry{Key: "valorant/team spirit", Name: "Team Spirit", Wiki: "valorant"},
		store.TeamEntry{Key: "dota2/gamerlegion", Name: "GamerLegion", Wiki: "dota2"},
		store.TeamEntry{Key: "counterstrike/gamerlegion", Name: "GamerLegion", Wiki: "counterstrike"},
		store.TeamEntry{Key: "dota2/og", Name: "OG", Wiki: "dota2"},
	)}

	got := map[string]bool{}
	for _, e := range testDaemon(t, cfg).profileTargets(&priv) {
		if got[e.Key] {
			t.Errorf("duplicate target %q", e.Key)
		}
		got[e.Key] = true
	}

	want := []string{"dota2/team spirit", "counterstrike/team spirit", "dota2/gamerlegion"}
	for _, k := range want {
		if !got[k] {
			t.Errorf("missing target %q from %v", k, got)
		}
	}
	// A game-scoped follow must not pull in the org's other rosters, a
	// disabled game must not be fetched at all, and an unfollowed team never.
	for _, k := range []string{"counterstrike/gamerlegion", "valorant/team spirit", "dota2/og"} {
		if got[k] {
			t.Errorf("unwanted target %q", k)
		}
	}
}

func TestProfileTargetsEmptyFollowList(t *testing.T) {
	priv := store.Private{Teams: teamIndex(
		store.TeamEntry{Key: "dota2/og", Name: "OG", Wiki: "dota2"},
	)}
	cfg := config.Config{Wikis: []config.Wiki{{Slug: "dota2", Enabled: true}}}
	if got := testDaemon(t, cfg).profileTargets(&priv); len(got) != 0 {
		t.Errorf("following nobody still wanted %d profiles: %+v", len(got), got)
	}
}

// publishProfiles is the only thing standing between a wiki's infobox and a
// world-readable file, so what it drops matters more than what it keeps.
func TestPublishProfilesWithholdsResults(t *testing.T) {
	d := testDaemon(t, config.Config{})
	priv := store.Private{TeamProfiles: map[string]store.TeamProfile{
		"dota2/team spirit": {
			Name: "Team Spirit", Wiki: "dota2",
			Fields: []store.ProfileField{
				{Label: "Region", Value: "CIS"},
				{Label: "Created", Value: "2015-12-06"},
				{Label: "Approx. Total Winnings", Value: "$31,523,780"},
				{Label: "Achievements", Value: "Winner"},
				{Label: "Best Placement", Value: "3rd-4th"},
				{Label: "Notes", Value: "beat Falcons 2-0"},
			},
			Roster: []store.ProfilePlayer{{ID: "Yatoro", Position: "1"}},
		},
	}}

	if err := d.publishProfiles(priv); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(d.store.ProfilesPath()))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Profiles []store.TeamProfile `json:"profiles"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Profiles) != 1 {
		t.Fatalf("want one profile, got %d", len(doc.Profiles))
	}

	kept := map[string]bool{}
	for _, f := range doc.Profiles[0].Fields {
		kept[f.Label] = true
	}
	// A founding date is not a scoreline, however much "12-06" looks like one.
	for _, label := range []string{"Region", "Created", "Approx. Total Winnings"} {
		if !kept[label] {
			t.Errorf("withheld a harmless field: %q", label)
		}
	}
	for _, label := range []string{"Achievements", "Best Placement", "Notes"} {
		if kept[label] {
			t.Errorf("published a result-bearing field: %q", label)
		}
	}
	// The private cache must be left intact — publishing is a filtered view of
	// it, not a destructive edit.
	if n := len(priv.TeamProfiles["dota2/team spirit"].Fields); n != 6 {
		t.Errorf("publishing mutated the private cache: %d fields left", n)
	}
	if len(doc.Profiles[0].Roster) != 1 {
		t.Error("roster was dropped")
	}
}

// A profile that failed or parsed to nothing must not be retried on every
// refresh: each attempt spends a thirty-second rate-limit slot before the
// request is even sent.
func TestProfileRetryBackoffIsHonoured(t *testing.T) {
	cfg := config.Config{
		Teams: []config.Follow{{Name: "Ghost Org", Wiki: "dota2"}},
		Wikis: []config.Wiki{{Slug: "dota2", Enabled: true}},
	}
	priv := store.Private{
		Teams: teamIndex(store.TeamEntry{
			Key: "dota2/ghost org", Name: "Ghost Org", Wiki: "dota2",
		}),
		ProfileRetryAfter: map[string]time.Time{
			"dota2/ghost org": time.Now().Add(time.Hour),
		},
	}
	d := testDaemon(t, cfg)

	// A nil client would panic on any fetch, so surviving this call is the
	// assertion: the backoff has to be consulted before the request.
	d.fetchTeamProfiles(t.Context(), &priv)

	if _, ok := priv.ProfileRetryAfter["dota2/ghost org"]; !ok {
		t.Error("backoff was dropped for a team still followed")
	}
}

// Unfollowing a team should reclaim its cached page and its backoff.
func TestProfilesPrunedWhenUnfollowed(t *testing.T) {
	cfg := config.Config{
		Teams: []config.Follow{{Name: "Team Spirit", Wiki: "dota2"}},
		Wikis: []config.Wiki{{Slug: "dota2", Enabled: true}},
	}
	priv := store.Private{
		Teams: teamIndex(store.TeamEntry{
			Key: "dota2/team spirit", Name: "Team Spirit", Wiki: "dota2",
		}),
		TeamProfiles: map[string]store.TeamProfile{
			"dota2/team spirit": {Name: "Team Spirit", Wiki: "dota2", FetchedAt: time.Now()},
			"dota2/old team":    {Name: "Old Team", Wiki: "dota2", FetchedAt: time.Now()},
		},
		ProfileRetryAfter: map[string]time.Time{
			"dota2/old team": time.Now().Add(time.Hour),
		},
	}
	testDaemon(t, cfg).fetchTeamProfiles(t.Context(), &priv)

	if _, ok := priv.TeamProfiles["dota2/old team"]; ok {
		t.Error("profile kept for a team no longer followed")
	}
	if _, ok := priv.ProfileRetryAfter["dota2/old team"]; ok {
		t.Error("backoff kept for a team no longer followed")
	}
	if _, ok := priv.TeamProfiles["dota2/team spirit"]; !ok {
		t.Error("profile dropped for a team still followed")
	}
}
