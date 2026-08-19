# TODO

## Planned: team profile pages

Clicking a team should open a page showing who they are, not just a fixture
list. Wanted: club info, current roster, recent results (respecting the spoiler
setting), recent VODs, and upcoming fixtures — kept up to date and stored
locally for followed teams.

Roughly half of this already exists. `Model.teamMatches` returns a team's
upcoming and past fixtures and the detail view renders them, artwork is already
cached on disk, and filtering VODs by team is a call the VODs view already
makes. Three pieces are missing.

**Info and roster.** These live on the team's own Liquipedia page and need
`action=parse`, which is rate limited to **one request per thirty seconds** —
the single most expensive call in the project, and the one that got the whole
IP blocked when it was run concurrently. That is affordable precisely because
the request is scoped to followed teams: five teams is two and a half minutes
of budget, spread across a refresh and cached for a week. It must never be
issued for a team merely being looked at in search, or browsing the index would
walk straight into the rate limiter.

`ParseTournament` is the working model for the parsing — infobox for region,
founded date and socials; the roster wikitable for player id, name, position
and join date. Note rosters carry *former* players in a second table; only the
active one belongs on the page.

**Storage.** A `TeamProfiles map[string]TeamProfile` in the private state, with
`FetchedAt`, refreshed weekly, following the same shape as `TournamentStreams`
and `DirectorySweeps`. Profiles for teams no longer followed should age out
rather than accumulate.

**Spoiler handling.** Results already redact correctly, but a roster page adds
two new leaks worth thinking about before building, not after:

- A recent-results list on one page is a much denser spoiler surface than the
  same matches scattered through a schedule. Catch-up masking already withholds
  the opponent for anything after the queue head, and the profile page has to
  respect that rather than re-deriving the fixture list from the private state.
- Placement and prize-money fields in a team's infobox state tournament results
  outright ("2nd, The International 2026"). Either omit those fields or gate
  them behind the same reveal the scoreline uses.

## Confirm the follow list

`Team Spirit`, `G2 Esports` and `Falcons` were seeded for testing rather than
because you asked for them; `GamerLegion` and `Team Liquid` are scoped to their
Dota rosters. Unfollowing now works from the app, so this is a few clicks —
worth doing before the catch-up queue is tuned around teams you do not watch.

## Resolved

- Contact address is `contra13@pm.me`, sent to Liquipedia in the User-Agent on
  every request.
- Plugin id is `contra.esports`, matching your other local plugins. Ids are
  permanent once listed, so this is now fixed.
- Publishing is prepared and not submitted — `docs/SUBMISSION.md` has the exact
  commands, field values, and the five form confirmations mapped to where each
  is already satisfied. Both gating decisions above are now made.
- The stray `gamerlegion` follow-list entry is gone. The org is followed as
  `{"name": "GamerLegion", "wiki": "dota2"}` — their Dota roster only.
