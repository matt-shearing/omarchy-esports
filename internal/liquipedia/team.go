package liquipedia

import (
	"strings"

	"golang.org/x/net/html"
)

// Team profile parsing.
//
// A team's own Liquipedia page carries the two things a fixture list cannot:
// who the organisation is, and who currently plays for it. Both come from one
// rendered page, which costs a single action=parse — the most rate-limited
// call in the project at one request per thirty seconds. That budget is why
// the daemon only ever asks for teams the user actually follows.
//
// Deliberately not parsed: the Results section. It is a table of tournament
// placements, which states outcomes outright for events the user may be part
// way through watching. There is no redaction that makes a placement table
// safe, so it is never read in the first place.

// TeamProfile is what a team's page tells us about the organisation.
type TeamProfile struct {
	// Fields are the infobox rows in page order, e.g. "Region" / "Europe".
	// Kept as a list rather than named fields because the template varies by
	// wiki: Counter-Strike pages carry "In-Game Leader" and "Games", Dota 2
	// pages carry "Team Captain" and "Director".
	Fields []InfoField
	// Roster is the active playing roster only.
	Roster []RosterPlayer
}

// InfoField is one infobox row.
type InfoField struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// RosterPlayer is one member of the active roster.
type RosterPlayer struct {
	// ID is the in-game handle, which is how players are known.
	ID string `json:"id"`
	// Name is the player's real name, when the page gives one.
	Name string `json:"name,omitempty"`
	// Position is the wiki's own notion of role: a lane number in Dota 2, a
	// role name elsewhere.
	Position string `json:"position,omitempty"`
	Joined   string `json:"joined,omitempty"`
	Country  string `json:"country,omitempty"`
	Captain  bool   `json:"captain,omitempty"`
	// Page is the player's Liquipedia page.
	Page string `json:"page,omitempty"`
}

// ParseTeam extracts the infobox and active roster from a rendered team page.
func ParseTeam(pageHTML string) (TeamProfile, error) {
	var p TeamProfile
	doc, err := html.Parse(strings.NewReader(pageHTML))
	if err != nil {
		return p, err
	}
	p.Fields = parseInfoboxFields(doc)
	p.Roster = parseActiveRoster(doc)
	return p, nil
}

// parseInfoboxFields reads the label/value rows of the team infobox.
func parseInfoboxFields(doc *html.Node) []InfoField {
	var out []InfoField
	for _, label := range findAllByClass(doc, "infobox-description") {
		name := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(text(label)), ":"))
		if name == "" {
			continue
		}
		value := text(nextElement(label))
		// Some rows are headers with no value, and some carry only an icon.
		if value == "" {
			continue
		}
		out = append(out, InfoField{Label: name, Value: value})
	}
	return out
}

// nextElement returns the next sibling that is an element, skipping the text
// nodes that formatting leaves between them.
func nextElement(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}
	for s := n.NextSibling; s != nil; s = s.NextSibling {
		if s.Type == html.ElementNode {
			return s
		}
	}
	return nil
}

// parseActiveRoster finds the active playing roster.
//
// A team page holds several tables with identical headers — the active roster,
// coaching staff, inactive players, and one or more former-player tables.
// Picking by index would return ex-players the moment a page gains a section,
// so this reads the heading structure instead.
//
// The headings themselves cannot be matched literally. Most pages use generic
// ids ("Player_Roster" then "Active"), but plenty name the team instead
// ("Players_of_Team_Spirit" then "Active_Roster"), and that variant is common
// enough to have shipped a team with an empty roster. So a section qualifies
// on what its heading means rather than what it says: it must be about players
// and it must say they are current.
func parseActiveRoster(doc *html.Node) []RosterPlayer {
	nodes := findAll(doc, func(n *html.Node) bool {
		if n.Type != html.ElementNode {
			return false
		}
		switch n.Data {
		case "h2", "h3":
			return true
		case "table":
			return hasClass(n, "table2__table")
		}
		return false
	})

	var section, sub string
	for _, n := range nodes {
		switch n.Data {
		case "h2":
			section, sub = attr(n, "id"), ""
		case "h3":
			sub = attr(n, "id")
		case "table":
			if !isActiveRosterSection(section, sub) {
				continue
			}
			// Take the first section that actually has players, so an empty
			// table cannot shadow a populated one further down.
			if players := parseRosterTable(n); len(players) > 0 {
				return players
			}
		}
	}
	return nil
}

// headingWords normalises a MediaWiki heading id into lowercase words.
func headingWords(id string) string {
	return strings.ToLower(strings.NewReplacer("_", " ", "-", " ").Replace(id))
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// isActiveRosterSection reports whether a heading pair marks the current
// playing roster.
func isActiveRosterSection(section, sub string) bool {
	sec, sb := headingWords(section), headingWords(sub)
	ctx := strings.TrimSpace(sec + " " + sb)

	// It has to be about players at all. This is what keeps the identically
	// shaped staff table under "Organization" out.
	if !containsAny(ctx, "player", "roster", "squad", "lineup") {
		return false
	}
	// And it must not be one of the tables that looks the same but is not the
	// current lineup. "inactive" is checked before "active" below, because
	// "Inactive_Roster" contains the word "active".
	if containsAny(ctx, "former", "inactive", "coach", "staff", "organization",
		"organisation", "substitute", "trial", "retired", "loan") {
		return false
	}
	// Where a page subdivides the roster section, the subsection has to say
	// the roster is current. Where it does not, the section is the roster.
	if sb != "" {
		return strings.Contains(sb, "active")
	}
	return true
}

// parseRosterTable reads the ID / Name / Position / Join Date columns.
func parseRosterTable(table *html.Node) []RosterPlayer {
	var out []RosterPlayer
	for _, tr := range findAll(table, func(n *html.Node) bool {
		return n.Type == html.ElementNode && n.Data == "tr" && hasClass(n, "table2__row--body")
	}) {
		var cells []*html.Node
		for c := tr.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && c.Data == "td" {
				cells = append(cells, c)
			}
		}
		if len(cells) == 0 {
			continue
		}

		var p RosterPlayer
		id := cells[0]
		if a := firstTag(id, "a"); a != nil {
			p.ID = text(a)
			p.Page = attr(a, "href")
		} else {
			p.ID = text(id)
		}
		if img := firstTag(id, "img"); img != nil {
			p.Country = attr(img, "alt")
		}
		// The captain is marked with a crown icon rather than a column.
		p.Captain = find(id, func(n *html.Node) bool {
			return n.Type == html.ElementNode && strings.EqualFold(attr(n, "title"), "Captain")
		}) != nil

		if len(cells) > 1 {
			p.Name = text(cells[1])
		}
		if len(cells) > 2 {
			p.Position = text(cells[2])
		}
		if len(cells) > 3 {
			// Join dates carry a footnote marker; the reference number is not
			// part of the date.
			p.Joined = textWithout(cells[3], func(n *html.Node) bool {
				return n.Type == html.ElementNode && (n.Data == "sup" || hasClass(n, "reference"))
			})
		}
		if p.ID == "" || isStaffRole(p.Position) {
			continue
		}
		out = append(out, p)
	}
	return out
}

// isStaffRole reports whether a roster row is support staff rather than a
// player.
//
// Counter-Strike pages list the coach inside the active roster table rather
// than under Organization, so the section guard cannot exclude them and the
// position column is the only thing that distinguishes them. They already
// appear in the infobox as "Coaches", so keeping them here would both
// duplicate that and contradict what this roster claims to be.
func isStaffRole(position string) bool {
	return containsAny(strings.ToLower(position),
		"coach", "manager", "analyst", "staff", "director")
}

// textWithout is text() with whole subtrees excluded.
func textWithout(n *html.Node, skip func(*html.Node) bool) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(cur *html.Node) {
		if skip(cur) {
			return
		}
		if cur.Type == html.TextNode {
			b.WriteString(cur.Data)
		}
		for c := cur.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
