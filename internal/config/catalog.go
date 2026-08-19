package config

// Catalog of known Liquipedia wikis.
//
// Every entry was verified against the live API rather than taken from a list
// of games: the slug resolves, the named ticker page exists, and parsing it
// yields matches in the modern `div.match-info` format this tool reads.
//
// Existence alone is not evidence. starcraft2 has a
// `Liquipedia:Upcoming_and_ongoing_matches` page that looks exactly right and
// serves matches from 2024-25; its live ticker is on `Main_Page`. StarCraft
// Brood War is the same. A wiki whose ticker could not be confirmed is left
// out entirely, because a wrong entry is worse than a missing one — it ships a
// game that silently shows nothing.
//
// The `Fixtures` figure records what the verification run saw, as a sanity
// check when a game later looks empty: a game with a low count was quiet at
// the time of checking, not broken.

// CatalogEntry is a known wiki, ready to be enabled.
type CatalogEntry struct {
	Slug       string
	Game       string
	TickerPage string
	// Fixtures is how many upcoming matches the ticker held when this entry
	// was verified. Zero means the format parsed correctly but the game was
	// between events — the entry works, there was simply nothing scheduled.
	Fixtures int
}

// Catalog lists the wikis the tool knows how to poll, most active first.
var Catalog = []CatalogEntry{
	// Enabled by default.
	{Slug: "counterstrike", Game: "Counter-Strike", TickerPage: "Liquipedia:Matches", Fixtures: 42},
	{Slug: "dota2", Game: "Dota 2", TickerPage: "Liquipedia:Matches", Fixtures: 14},
	{Slug: "starcraft2", Game: "StarCraft II", TickerPage: "Main_Page", Fixtures: 10},

	// Verified, available to enable.
	{Slug: "leagueoflegends", Game: "League of Legends", TickerPage: "Liquipedia:Matches", Fixtures: 48},
	{Slug: "valorant", Game: "VALORANT", TickerPage: "Liquipedia:Matches", Fixtures: 49},
	{Slug: "rocketleague", Game: "Rocket League", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "overwatch", Game: "Overwatch", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "apexlegends", Game: "Apex Legends", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "pubg", Game: "PUBG", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "mobilelegends", Game: "Mobile Legends: Bang Bang", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "honorofkings", Game: "Honor of Kings", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "marvelrivals", Game: "Marvel Rivals", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "tft", Game: "Teamfight Tactics", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "deadlock", Game: "Deadlock", TickerPage: "Liquipedia:Matches", Fixtures: 35},
	{Slug: "callofduty", Game: "Call of Duty", TickerPage: "Liquipedia:Matches", Fixtures: 16},
	{Slug: "warcraft", Game: "Warcraft", TickerPage: "Liquipedia:Matches", Fixtures: 4},
	{Slug: "starcraft", Game: "StarCraft: Brood War", TickerPage: "Main_Page", Fixtures: 1},

	// Format verified, but the ticker held only finished matches at the time
	// of checking — these scenes were between events, not broken.
	{Slug: "rainbowsix", Game: "Rainbow Six", TickerPage: "Liquipedia:Matches", Fixtures: 0},
	{Slug: "easportsfc", Game: "EA Sports FC", TickerPage: "Liquipedia:Matches", Fixtures: 0},
}

// Deliberately absent, with the reason, so nobody re-adds them hopefully:
//
//	smash         — Main_Page has no ticker markup at all (0 timestamps,
//	                0 match-info blocks); Liquipedia:Matches does not exist.
//	formula1      — Main_Page carries timestamps but no match-info blocks, so
//	                it uses a different widget this parser does not read.
//	ageofempires  — Liquipedia:Matches parsed with no timestamps or blocks.
//	streetfighter — not a Liquipedia slug (HTTP 404 on siteinfo).
//	sixsiege      — not a Liquipedia slug; Rainbow Six is `rainbowsix`.
//
// Existence confirmed but ticker never parse-tested, so unverified and
// omitted: brawlstars, clashroyale, freefire, wildrift, naraka. Their team
// categories are populated, so they are likely fine — but "likely" is not the
// bar for shipping a game.

// CatalogFor returns the catalog entry for a slug.
func CatalogFor(slug string) (CatalogEntry, bool) {
	for _, e := range Catalog {
		if e.Slug == slug {
			return e, true
		}
	}
	return CatalogEntry{}, false
}

// Wiki builds a config entry from the catalog.
func (e CatalogEntry) Wiki(enabled bool) Wiki {
	return Wiki{
		Slug:       e.Slug,
		Game:       e.Game,
		TickerPage: e.TickerPage,
		Enabled:    enabled,
	}
}
