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
	Slug string
	Game string
	// Short is the badge shown against a fixture, in the form fans actually
	// use — "CS2" rather than "Counter-Strike". Kept to five characters so a
	// row of them stays aligned.
	Short      string
	TickerPage string
	// Fixtures is how many upcoming matches the ticker held when this entry
	// was verified. Zero means the format parsed correctly but the game was
	// between events — the entry works, there was simply nothing scheduled.
	Fixtures int
}

// Catalog lists the wikis the tool knows how to poll, most active first.
var Catalog = []CatalogEntry{
	// Enabled by default.
	{Slug: "counterstrike", Game: "Counter-Strike", Short: "CS2", TickerPage: "Liquipedia:Matches", Fixtures: 42},
	{Slug: "dota2", Game: "Dota 2", Short: "DOTA2", TickerPage: "Liquipedia:Matches", Fixtures: 14},
	{Slug: "starcraft2", Game: "StarCraft II", Short: "SC2", TickerPage: "Main_Page", Fixtures: 10},

	// Verified, available to enable.
	{Slug: "leagueoflegends", Game: "League of Legends", Short: "LOL", TickerPage: "Liquipedia:Matches", Fixtures: 48},
	{Slug: "valorant", Game: "VALORANT", Short: "VAL", TickerPage: "Liquipedia:Matches", Fixtures: 49},
	{Slug: "rocketleague", Game: "Rocket League", Short: "RL", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "overwatch", Game: "Overwatch", Short: "OW2", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "apexlegends", Game: "Apex Legends", Short: "APEX", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "pubg", Game: "PUBG", Short: "PUBG", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "mobilelegends", Game: "Mobile Legends: Bang Bang", Short: "MLBB", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "honorofkings", Game: "Honor of Kings", Short: "HOK", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "marvelrivals", Game: "Marvel Rivals", Short: "MR", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "tft", Game: "Teamfight Tactics", Short: "TFT", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "deadlock", Game: "Deadlock", Short: "DL", TickerPage: "Liquipedia:Matches", Fixtures: 35},
	{Slug: "callofduty", Game: "Call of Duty", Short: "COD", TickerPage: "Liquipedia:Matches", Fixtures: 16},
	{Slug: "warcraft", Game: "Warcraft", Short: "WC3", TickerPage: "Liquipedia:Matches", Fixtures: 4},
	{Slug: "starcraft", Game: "StarCraft: Brood War", Short: "BW", TickerPage: "Main_Page", Fixtures: 1},

	{Slug: "naraka", Game: "Naraka", Short: "NRK", TickerPage: "Liquipedia:Matches", Fixtures: 50},
	{Slug: "freefire", Game: "Free Fire", Short: "FF", TickerPage: "Liquipedia:Matches", Fixtures: 38},
	{Slug: "halo", Game: "Halo", Short: "HALO", TickerPage: "Liquipedia:Matches", Fixtures: 24},
	{Slug: "brawlstars", Game: "Brawl Stars", Short: "BS", TickerPage: "Liquipedia:Matches", Fixtures: 23},
	{Slug: "osu", Game: "osu!", Short: "OSU", TickerPage: "Liquipedia:Matches", Fixtures: 16},
	{Slug: "trackmania", Game: "TrackMania", Short: "TM", TickerPage: "Main_Page", Fixtures: 12},
	{Slug: "heroes", Game: "Heroes of the Storm", Short: "HOTS", TickerPage: "Liquipedia:Matches", Fixtures: 1},

	{Slug: "crossfire", Game: "CrossFire", Short: "CF", TickerPage: "Liquipedia:Matches", Fixtures: 20},
	{Slug: "teamfortress", Game: "Team Fortress", Short: "TF2", TickerPage: "Liquipedia:Matches", Fixtures: 1},

	// Format verified, but the ticker held only finished matches at the time
	// of checking — these scenes were between events, not broken.
	{Slug: "pokemon", Game: "Pokémon", Short: "PKMN", TickerPage: "Liquipedia:Matches", Fixtures: 0},
	{Slug: "criticalops", Game: "Critical Ops", Short: "COPS", TickerPage: "Liquipedia:Matches", Fixtures: 0},
	{Slug: "autochess", Game: "Auto Chess", Short: "AC", TickerPage: "Liquipedia:Matches", Fixtures: 0},
	{Slug: "splatoon", Game: "Splatoon", Short: "SPL", TickerPage: "Liquipedia:Matches", Fixtures: 0},
	{Slug: "clashroyale", Game: "Clash Royale", Short: "CR", TickerPage: "Liquipedia:Matches", Fixtures: 0},
	{Slug: "wildrift", Game: "Wild Rift", Short: "WR", TickerPage: "Liquipedia:Matches", Fixtures: 0},
	{Slug: "hearthstone", Game: "Hearthstone", Short: "HS", TickerPage: "Liquipedia:Matches", Fixtures: 0},
	{Slug: "worldofwarcraft", Game: "World of Warcraft", Short: "WOW", TickerPage: "Liquipedia:Matches", Fixtures: 0},
	{Slug: "simracing", Game: "Sim Racing", Short: "SIM", TickerPage: "Liquipedia:Matches", Fixtures: 0},
	{Slug: "rainbowsix", Game: "Rainbow Six", Short: "R6", TickerPage: "Liquipedia:Matches", Fixtures: 0},
	{Slug: "easportsfc", Game: "EA Sports FC", Short: "FC", TickerPage: "Liquipedia:Matches", Fixtures: 0},
}

// Deliberately absent, with the reason, so nobody re-adds them hopefully:
//
//	smash         — Main_Page has no ticker markup at all (0 timestamps,
//	                0 match-info blocks); Liquipedia:Matches does not exist.
//	formula1      — Main_Page carries timestamps but no match-info blocks, so
//	                it uses a different widget this parser does not read.
//	ageofempires  — Liquipedia:Matches parsed with no timestamps or blocks.
//	fortnite      — Main_Page has no ticker markup (0 blocks); no
//	                Liquipedia:Matches page exists.
//	streetfighter — not a Liquipedia slug (HTTP 404 on siteinfo).
//	sixsiege      — not a Liquipedia slug; Rainbow Six is `rainbowsix`.
//
// Verification is ongoing for the longer tail of wikis; anything not listed
// above has simply not been parse-tested yet, and "likely fine" is not the bar
// for shipping a game.

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
		Short:      e.Short,
		TickerPage: e.TickerPage,
		Enabled:    enabled,
	}
}
