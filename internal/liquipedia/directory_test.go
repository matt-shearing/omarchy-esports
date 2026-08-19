package liquipedia

import "testing"

func names(ts []DirectoryTeam) []string {
	out := make([]string, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Name)
	}
	return out
}

func has(ts []DirectoryTeam, want string) bool {
	for _, n := range names(ts) {
		if n == want {
			return true
		}
	}
	return false
}

// TestDropSubPagesKeepsSlashedTeamNames guards a real trap: Brawl Stars has
// orgs whose names contain a slash, so filtering on "/" alone deletes them.
func TestDropSubPagesKeepsSlashedTeamNames(t *testing.T) {
	in := []DirectoryTeam{
		{Name: "6ix F/A"},            // a real team; "6ix" is not a page
		{Name: "F/A Bobby"},          // likewise
		{Name: "FaZe Clan"},          // parent org
		{Name: "FaZe Clan/Warzone"},  // roster sub-page of an org that exists
		{Name: "Denial eSports/One"}, // roster suffix, parent absent
		{Name: "Team Liquid"},
		{Name: "Team Liquid/Results"}, // archive sub-page
	}
	out := dropSubPages(in)

	for _, keep := range []string{"6ix F/A", "F/A Bobby", "FaZe Clan", "Team Liquid"} {
		if !has(out, keep) {
			t.Errorf("dropped a real team: %q (got %v)", keep, names(out))
		}
	}
	for _, drop := range []string{"FaZe Clan/Warzone", "Denial eSports/One", "Team Liquid/Results"} {
		if has(out, drop) {
			t.Errorf("kept a sub-page: %q", drop)
		}
	}
}

func TestUsableTeamTitle(t *testing.T) {
	bad := []string{"", "(disambiguation)", "Foo (disambiguation)", "(page does not exist)"}
	for _, b := range bad {
		if usableTeamTitle(b) {
			t.Errorf("accepted junk title %q", b)
		}
	}
	// A colon can appear in a legitimate team name; the request's namespace
	// filter is what excludes wiki-infrastructure pages.
	for _, g := range []string{"Team Liquid", "6ix F/A", "compLexity Gaming"} {
		if !usableTeamTitle(g) {
			t.Errorf("rejected a real team %q", g)
		}
	}
}
