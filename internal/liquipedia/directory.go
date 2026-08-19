package liquipedia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Team directory.
//
// The match ticker only names teams with a fixture in the fetched window, so
// an index built from tickers alone cannot find an org that is between events
// — Team Liquid has Dota fixtures today and none in Counter-Strike, and was
// therefore unfindable for CS.
//
// Liquipedia's `Category:Teams` lists every team page on a wiki (1407 of them
// on counterstrike), and enumerating a category is a plain `action=query`
// call, which is rate-limited at one per 2 seconds rather than the one per 30
// seconds that page parsing costs. A full wiki is therefore a handful of cheap
// requests, and the result changes slowly enough to cache for days.

// DirectoryTeam is one team page from a category listing.
type DirectoryTeam struct {
	Name string
	Page string
	Wiki string
}

type categoryResponse struct {
	Query struct {
		CategoryMembers []struct {
			Title string `json:"title"`
			NS    int    `json:"ns"`
		} `json:"categorymembers"`
	} `json:"query"`
	Continue struct {
		CmContinue string `json:"cmcontinue"`
	} `json:"continue"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

// maxDirectoryPages bounds a single sweep. At 500 titles per page this allows
// 5000 teams per wiki, comfortably above the largest category observed.
const maxDirectoryPages = 10

// ListTeams enumerates a wiki's team pages.
//
// category defaults to "Teams". Pagination follows the API's continuation
// token; every request goes through the shared rate limiter.
func (c *Client) ListTeams(ctx context.Context, wiki, category string) ([]DirectoryTeam, error) {
	if strings.TrimSpace(category) == "" {
		category = "Teams"
	}
	var out []DirectoryTeam
	seen := map[string]bool{}
	cont := ""

	for page := 0; page < maxDirectoryPages; page++ {
		q := url.Values{
			"action":      {"query"},
			"list":        {"categorymembers"},
			"cmtitle":     {"Category:" + category},
			"cmlimit":     {"500"},
			"cmnamespace": {"0"}, // article namespace only: no User:, Category:, Template:
			"format":      {"json"},
		}
		if cont != "" {
			q.Set("cmcontinue", cont)
		}
		endpoint := fmt.Sprintf("https://liquipedia.net/%s/api.php?%s", url.PathEscape(wiki), q.Encode())

		body, err := c.get(ctx, endpoint, false)
		if err != nil {
			// Return what we have: a partial directory still beats none.
			if len(out) > 0 {
				return out, nil
			}
			return nil, err
		}
		var resp categoryResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return out, fmt.Errorf("decoding category listing for %s: %w", wiki, err)
		}
		if resp.Error != nil {
			return out, fmt.Errorf("liquipedia %s: %s: %s", wiki, resp.Error.Code, resp.Error.Info)
		}

		for _, m := range resp.Query.CategoryMembers {
			name := strings.TrimSpace(m.Title)
			if !usableTeamTitle(name) || seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, DirectoryTeam{
				Name: name,
				Page: "/" + wiki + "/" + strings.ReplaceAll(name, " ", "_"),
				Wiki: wiki,
			})
		}

		cont = resp.Continue.CmContinue
		if cont == "" {
			break
		}
	}
	// Sub-page detection needs the whole listing, since it asks whether a
	// title's parent is itself a team.
	return dropSubPages(out), nil
}

// usableTeamTitle rejects entries that are obviously not teams.
//
// The namespace filter on the request already excludes User:, Template: and
// friends, so this is a light second pass.
func usableTeamTitle(title string) bool {
	if title == "" || strings.HasPrefix(title, "(") {
		return false
	}
	lower := strings.ToLower(title)
	for _, bad := range []string{"(disambiguation)", "(page does not exist)"} {
		if strings.Contains(lower, bad) {
			return false
		}
	}
	return true
}

// rosterSuffixes are the sub-page names Liquipedia uses for a division or an
// archive hanging off an org's page.
var rosterSuffixes = map[string]bool{
	"warzone": true, "mobile": true, "codol": true, "one": true, "online": true,
	"results": true, "matches": true, "history": true, "training squad": true,
	"academy": true, "youth": true,
}

// dropSubPages removes roster and archive sub-pages while keeping teams whose
// name genuinely contains a slash.
//
// A blanket "contains /" filter is wrong in both directions. It drops real
// teams — Brawl Stars has orgs actually named "6ix F/A" and "F/A Bobby", where
// "6ix" is not a page at all — and it is also too coarse for Call of Duty,
// where roughly one in eight entries is a "/Warzone" or "/Mobile" roster of an
// org that also has its own page.
//
// So a slashed title is only dropped when the part before the slash is itself
// a team in the same listing, or the suffix is a known roster or archive name.
func dropSubPages(teams []DirectoryTeam) []DirectoryTeam {
	parents := make(map[string]bool, len(teams))
	for _, t := range teams {
		parents[strings.ToLower(t.Name)] = true
	}

	out := teams[:0]
	for _, t := range teams {
		idx := strings.LastIndex(t.Name, "/")
		if idx > 0 {
			parent := strings.ToLower(strings.TrimSpace(t.Name[:idx]))
			suffix := strings.ToLower(strings.TrimSpace(t.Name[idx+1:]))
			if parents[parent] || rosterSuffixes[suffix] {
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

// DirectoryTTL is how long a fetched directory stays fresh. Team rosters come
// and go slowly, and a sweep costs several requests per wiki, so this is
// deliberately long.
const DirectoryTTL = 7 * 24 * time.Hour
