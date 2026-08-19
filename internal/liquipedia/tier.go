package liquipedia

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Tournament tiers.
//
// The match ticker carries tier filter buttons but no tier on the match rows
// themselves, so tier has to come from the tournament page. Its infobox
// exposes `liquipediatier`, and reading section 0 is a cheap `action=query`
// call rather than a page parse.
//
// The encoding is not consistent between wikis, which is the trap here:
// Counter-Strike writes "S-Tier"/"C-Tier" while Dota 2 writes "1". Both are
// normalised to Liquipedia's numeric scale, where 1 is the top.

// Tier values, following Liquipedia's own numbering.
const (
	TierUnknown = 0
	TierS       = 1 // premier events: majors, The International, EWC
	TierA       = 2
	TierB       = 3
	TierC       = 4
	TierD       = 5
)

// TierName renders a tier for display.
func TierName(tier int) string {
	switch tier {
	case TierS:
		return "S-Tier"
	case TierA:
		return "A-Tier"
	case TierB:
		return "B-Tier"
	case TierC:
		return "C-Tier"
	case TierD:
		return "D-Tier"
	}
	return ""
}

var letterTierRe = regexp.MustCompile(`(?i)^([sabcd])[\s-]*tier$`)

// NormaliseTier converts an infobox value to the numeric scale.
//
// Accepts "S-Tier", "s tier", "1", "Tier 1" and returns TierUnknown for
// anything it does not recognise — including "Misc" and "Show Match", which
// some events use in place of a tier.
func NormaliseTier(raw string) int {
	v := strings.TrimSpace(raw)
	if v == "" {
		return TierUnknown
	}
	if n, err := strconv.Atoi(v); err == nil {
		if n >= 1 && n <= 5 {
			return n
		}
		return TierUnknown
	}
	if m := letterTierRe.FindStringSubmatch(v); m != nil {
		switch strings.ToLower(m[1]) {
		case "s":
			return TierS
		case "a":
			return TierA
		case "b":
			return TierB
		case "c":
			return TierC
		case "d":
			return TierD
		}
	}
	// "Tier 1" and similar.
	if m := regexp.MustCompile(`(?i)^tier\s*([1-5])$`).FindStringSubmatch(v); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return TierUnknown
}

var (
	tierFieldRe     = regexp.MustCompile(`(?im)^\s*\|\s*liquipediatier\s*=\s*([^\n|]*)`)
	tierTypeFieldRe = regexp.MustCompile(`(?im)^\s*\|\s*liquipediatiertype\s*=\s*([^\n|]*)`)
)

// TournamentTier is what a tournament page says about its own standing.
type TournamentTier struct {
	// Tier is the normalised numeric tier, or TierUnknown.
	Tier int
	// Type is the qualifier from `liquipediatiertype` — "Qualifier",
	// "Weekly", "Monthly", "Showmatch" and so on. Empty for a main event.
	Type string
}

// FetchTier reads a tournament's tier from its infobox.
//
// page is a Liquipedia path such as "/counterstrike/Esports_World_Cup/2026";
// the wiki prefix is stripped if present.
//
// Ticker entries often link to a stage sub-page — "The International/2026/
// Main Event" rather than "The International/2026" — and those use a
// HiddenDataBox instead of the full infobox, so they carry no tier at all.
// When the linked page has none, this walks up one path segment at a time to
// the parent tournament, which is where the tier actually lives.
func (c *Client) FetchTier(ctx context.Context, wiki, page string) (TournamentTier, error) {
	var out TournamentTier

	title := strings.TrimPrefix(strings.TrimPrefix(page, "/"+wiki+"/"), "/")
	if i := strings.Index(title, "#"); i >= 0 {
		title = title[:i]
	}
	if title == "" {
		return out, fmt.Errorf("empty tournament page")
	}

	// Two extra hops covers "Event/Year/Stage/Round"; beyond that the parent
	// is the series index page, which has no tier of its own.
	for attempt := 0; attempt < 3; attempt++ {
		tt, err := c.fetchTierPage(ctx, wiki, title)
		if err != nil {
			return out, err
		}
		if tt.Tier != TierUnknown {
			return tt, nil
		}
		// Keep any tier type found on the way up, even without a tier.
		if out.Type == "" {
			out.Type = tt.Type
		}
		idx := strings.LastIndex(title, "/")
		if idx <= 0 {
			break
		}
		title = title[:idx]
	}
	return out, nil
}

// fetchTierPage reads one page's infobox.
func (c *Client) fetchTierPage(ctx context.Context, wiki, title string) (TournamentTier, error) {
	var out TournamentTier

	endpoint := fmt.Sprintf("https://liquipedia.net/%s/api.php?%s", url.PathEscape(wiki), url.Values{
		"action":    {"query"},
		"prop":      {"revisions"},
		"titles":    {title},
		"rvprop":    {"content"},
		"rvslots":   {"main"},
		"rvsection": {"0"},
		"format":    {"json"},
	}.Encode())

	body, err := c.get(ctx, endpoint, false)
	if err != nil {
		return out, err
	}
	var resp revisionsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return out, fmt.Errorf("decoding tier for %s/%s: %w", wiki, title, err)
	}
	for _, p := range resp.Query.Pages {
		if p.Missing != nil || len(p.Revisions) == 0 {
			continue
		}
		text := p.Revisions[0].Slots.Main.Content
		if m := tierFieldRe.FindStringSubmatch(text); m != nil {
			out.Tier = NormaliseTier(m[1])
		}
		if m := tierTypeFieldRe.FindStringSubmatch(text); m != nil {
			out.Type = strings.TrimSpace(m[1])
		}
	}
	return out, nil
}
