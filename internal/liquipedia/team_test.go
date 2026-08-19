package liquipedia

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fixtures are stored gzipped: a rendered team page is a quarter of a megabyte
// of mostly boilerplate, and the interesting parts do not compress away.
func loadTeamPage(t *testing.T, name string) string {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name+".html.gz"))
	if err != nil {
		t.Skipf("fixture %s missing: %v", name, err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	data, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestParseTeamDota(t *testing.T) {
	p, err := ParseTeam(loadTeamPage(t, "team-falcons-dota2"))
	if err != nil {
		t.Fatal(err)
	}

	fields := map[string]string{}
	for _, f := range p.Fields {
		fields[f.Label] = f.Value
	}
	if got := fields["Location"]; got != "Saudi Arabia" {
		t.Errorf("Location = %q", got)
	}
	if got := fields["Region"]; got != "Europe" {
		t.Errorf("Region = %q", got)
	}
	if _, ok := fields["Approx. Total Winnings"]; !ok {
		t.Errorf("winnings missing from %v", fields)
	}

	if len(p.Roster) != 5 {
		t.Fatalf("roster has %d players, want 5: %+v", len(p.Roster), p.Roster)
	}
	first := p.Roster[0]
	if first.ID != "skiter" {
		t.Errorf("first player ID = %q, want skiter", first.ID)
	}
	if first.Name != "Oliver Lepko" {
		t.Errorf("first player name = %q", first.Name)
	}
	if first.Country != "Slovakia" {
		t.Errorf("first player country = %q", first.Country)
	}
	if first.Position != "1" {
		t.Errorf("first player position = %q", first.Position)
	}
	// The footnote marker on a join date is not part of the date.
	if first.Joined != "2023-11-11" {
		t.Errorf("join date = %q, want a bare date with no reference number", first.Joined)
	}
	if !strings.Contains(first.Page, "/dota2/") {
		t.Errorf("player page = %q, want a wiki link", first.Page)
	}

	var captains []string
	for _, r := range p.Roster {
		if r.Captain {
			captains = append(captains, r.ID)
		}
	}
	if len(captains) != 1 || captains[0] != "ATF" {
		t.Errorf("captains = %v, want exactly [ATF]", captains)
	}
}

// The roster template differs per wiki, and several tables on the page share
// the same headers. Anchoring on the heading structure has to hold across both.
func TestParseTeamCounterStrike(t *testing.T) {
	p, err := ParseTeam(loadTeamPage(t, "team-falcons-cs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Roster) == 0 {
		t.Fatal("no roster parsed")
	}
	if len(p.Roster) != 5 {
		t.Errorf("roster has %d players, want the active five: %+v", len(p.Roster), p.Roster)
	}
	for _, r := range p.Roster {
		if r.ID == "" {
			t.Errorf("player with no ID: %+v", r)
		}
		// This wiki lists the coach inside the active roster table, unlike
		// Dota 2 which gives staff their own section.
		if strings.Contains(strings.ToLower(r.Position), "coach") {
			t.Errorf("coaching staff leaked into the roster: %+v", r)
		}
	}
	// This wiki's infobox uses different labels, which is why fields are a
	// list rather than a fixed struct.
	var labels []string
	for _, f := range p.Fields {
		labels = append(labels, f.Label)
	}
	joined := strings.Join(labels, ",")
	if !strings.Contains(joined, "In-Game Leader") {
		t.Errorf("labels = %v, want the wiki-specific ones preserved", labels)
	}
}

// Many pages name the team in their headings ("Players of Team Spirit" then
// "Active Roster") instead of using the generic ids. Matching those literally
// shipped a followed team with an empty roster, and the same page carries
// coaching, inactive and former tables that must all stay out.
func TestParseTeamHeadingVariant(t *testing.T) {
	p, err := ParseTeam(loadTeamPage(t, "team-spirit-dota2"))
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Roster) == 0 {
		t.Fatal("no roster parsed from a page using team-named headings")
	}
	ids := map[string]bool{}
	for _, r := range p.Roster {
		ids[r.ID] = true
	}
	// The active five, not the coaching staff or the inactive list.
	if len(p.Roster) > 6 {
		t.Errorf("roster has %d entries, which suggests another table leaked in: %v",
			len(p.Roster), ids)
	}
	for _, r := range p.Roster {
		if strings.Contains(strings.ToLower(r.Position), "coach") {
			t.Errorf("coaching staff leaked into the roster: %+v", r)
		}
	}
}

func TestIsActiveRosterSection(t *testing.T) {
	cases := []struct {
		section, sub string
		want         bool
	}{
		{"Player_Roster", "Active", true},
		{"Players_of_Team_Spirit", "Active_Roster", true},
		{"Player_Roster", "", true},
		{"Player_Roster", "Former", false},
		{"Players_of_Team_Spirit", "Inactive_Roster", false},
		{"Players_of_Team_Spirit", "Coaching_Staff", false},
		{"Players_of_Team_Spirit", "Former_Players_of_Team_Spirit", false},
		// The staff table under Organization has identical headers and an
		// "Active" subsection of its own.
		{"Organization", "Active_2", false},
		{"Results", "", false},
	}
	for _, c := range cases {
		if got := isActiveRosterSection(c.section, c.sub); got != c.want {
			t.Errorf("isActiveRosterSection(%q, %q) = %v, want %v",
				c.section, c.sub, got, c.want)
		}
	}
}

// A page we cannot parse must yield nothing rather than panic.
func TestParseTeamJunk(t *testing.T) {
	for _, in := range []string{"", "<html><body>nothing here</body></html>", "<<<"} {
		p, err := ParseTeam(in)
		if err != nil {
			t.Errorf("ParseTeam(%q) errored: %v", in, err)
		}
		if len(p.Roster) != 0 || len(p.Fields) != 0 {
			t.Errorf("ParseTeam(%q) invented data: %+v", in, p)
		}
	}
}
