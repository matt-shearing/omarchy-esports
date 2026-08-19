# TODO

## Confirm the follow list

`Team Spirit`, `G2 Esports` and `Falcons` were seeded for testing rather than
because you asked for them; `GamerLegion` and `Team Liquid` are scoped to their
Dota rosters. Unfollowing now works from the app, so this is a few clicks —
worth doing before the catch-up queue is tuned around teams you do not watch.

## Resolved

- Team profile pages are built. Clicking a team shows club details, the active
  roster, upcoming fixtures, recordings and past matches. Profiles are fetched
  only for followed teams (two per refresh, cached a week) because each costs a
  30-second rate-limit slot; the Results section is never parsed, and infobox
  rows go through the spoiler scanner before publication.
- Contact address is `contra13@pm.me`, sent to Liquipedia in the User-Agent on
  every request.
- Plugin id is `contra.esports`, matching your other local plugins. Ids are
  permanent once listed, so this is now fixed.
- Publishing is prepared and not submitted — `docs/SUBMISSION.md` has the exact
  commands, field values, and the five form confirmations mapped to where each
  is already satisfied. Both gating decisions above are now made.
- The stray `gamerlegion` follow-list entry is gone. The org is followed as
  `{"name": "GamerLegion", "wiki": "dota2"}` — their Dota roster only.
