# omarchy-esports

Spoiler-free esports schedule for [Omarchy](https://omarchy.org): a bar widget,
a desktop app, and a daemon that tracks upcoming matches, ties them to live
streams, and finds VODs afterwards — without telling you who won.

**Counter-Strike** and **Dota 2** on first run, with **37 games** verified and
one toggle away — League of Legends, VALORANT, Rocket League, Overwatch, Apex,
PUBG, Call of Duty, Deadlock, Marvel Rivals, Halo, Hearthstone, osu! and more.

---

## What it does

- **Bar widget** — the next match for a team you follow, with a live countdown.
  Click any match for an inline detail panel with stream links, the VOD, and
  links to the relevant Liquipedia pages. A button opens the full app.
- **App** — schedule, a catch-up queue of what you still have to watch, VODs,
  fuzzy team search with logos, and a per-team view of upcoming fixtures.
- **Spoiler-free by construction** — see below. This is the point of the thing.
- **Stream links** — resolved from the tournament's broadcast table, so a match
  card can send you straight to the right Twitch channel.
- **VOD discovery** — finds the recording on YouTube once a match is done, with
  no API key and no quota.
- **Notifications** — followed team starting, going live, VOD ready, tournament
  starting. Clicking the notification opens the stream.

## Install

```bash
git clone https://github.com/matt-shearing/omarchy-esports
cd omarchy-esports
./install.sh
```

Then follow some teams and you are done:

```bash
omarchy-esports teams add "Team Spirit" "G2 Esports"
omarchy-esports status
```

`install.sh` builds the binary into `~/.local/bin`, installs the bar plugin,
enables a systemd user service for the daemon, and adds the widget to your bar.
Use `DEV_LINK=1 ./install.sh` to symlink the plugin from a working copy instead
of copying it. If the plugin is already a git checkout from
`omarchy plugin add`, the installer leaves that checkout alone.

The bar widget is also a standalone repo so the catalog's
`omarchy plugin add` works (it only reads `manifest.json` at the repository
root):

```bash
omarchy plugin add https://github.com/matt-shearing/omarchy-esports-plugin.git --enable
```

You still need this repo — the daemon — for the widget to show anything.

### Uninstall

```bash
omarchy plugin remove contra.esports
systemctl --user disable --now omarchy-esports
rm -r ~/.local/state/omarchy-esports ~/.config/omarchy-esports
rm ~/.local/bin/omarchy-esports ~/.local/bin/omarchy-esports-app
rm -r ~/.local/share/omarchy-esports
rm ~/.local/share/applications/omarchy-esports.desktop
```

### Keybindings

The installer does not touch your keybindings. These are the suggested ones —
add them to `~/.config/hypr/bindings.lua`:

```lua
o.bind("SUPER + ALT + E", "Esports panel", "omarchy-shell shell toggle contra.esports")
o.bind("SUPER + SHIFT + G", "Esports app", "omarchy-esports-app")
```

Bind the panel through `shell toggle`, not through a plugin IPC target. A bar
widget is instantiated once per monitor and an IPC target only ever reaches one
of those instances, so an IPC binding opens the panel on whichever monitor
claimed the target rather than the one you are looking at. `shell toggle` routes
through the bar, which picks the focused screen.

## Spoiler-freedom is structural, not cosmetic

Most spoiler-free modes are a UI setting: the app knows the score and agrees not
to draw it. One rendering bug, one debug log, one notification preview, and the
result is out.

This one splits state across two files:

| file | mode | contents |
|---|---|---|
| `~/.local/state/omarchy-esports/state.json` | `0644` | what the UI reads — **redacted** |
| `~/.local/state/omarchy-esports/full.json`  | `0600` | what the daemon knows — everything |

Under the default strict policy, a finished match you have not revealed has no
score, no winner, no VOD title and no thumbnail **in the file the UI reads**.
The UI is not being trusted to hide anything, because it never receives it.

```bash
# The redacted view the widget and app consume:
jq '[.matches[] | select(.score != null and (.score[0] > 0))] | length' \
  ~/.local/state/omarchy-esports/state.json
# => 0

omarchy-esports reveal <id>   # deliberately unblind one match
```

### What counts as a spoiler

Scorelines are the obvious leak. The subtler ones this also catches:

- **Outcome verbs** — "beat", "def.", "eliminated", "advance", "champions".
- **Thumbnails** — celebration shots and score overlays give away results
  before you have read a word. Withheld under strict.
- **VOD runtime** — a 45-minute best-of-three can only have been a 2–0.
- **Series length.** This one is nastier than it looks. A title like
  `[EN] LGD vs Yandex - Game 3 - The International` contains no score and reads
  as perfectly safe. But in a best-of-three, a third game can only exist if it
  was 1–1. In general, in a best-of-*N* a side needs `ceil(N/2)` wins to close
  it out, so any game beyond that number leaks the state of the series. The
  daemon knows each match's format, so it can reason about this and black the
  title out — and because such a title cannot be scrubbed into something both
  safe and still descriptive, it is withheld rather than edited.
- **Highlights packages** are flagged as such. They are the wrong video if you
  wanted to watch the series, and their titles and artwork spoil far more
  eagerly than a full VOD's.

Three policies: `strict` (default), `balanced`, `off`. Set `spoilers` in
`~/.config/omarchy-esports/config.json`.

### Catch-up masking: the spoiler hiding in your own schedule

Hiding scores is not enough once you are behind.

Say you follow Team Spirit, they played three matches yesterday, and you have
watched none of them. Listing all three tells you they kept winning. Worse,
their *next* fixture is itself the spoiler: if the schedule says they play in
tomorrow's Upper Bracket Final, you already know how yesterday went.

So for each followed team, the earliest unwatched finished match is the **queue
head** and is shown in full — it is the one to watch next. Every later match
that team plays, finished or upcoming, has the **opponent withheld**, along
with the score and the bracket stage:

```
CATCH UP
  ended  Thu 13 Aug   Team Spirit   vs   JiJieHao      ← queue head, full detail
  ended  Thu 13 Aug   Team Spirit   vs   ? Hidden      ← opponent withheld
  18:00  Fri 14 Aug   Team Spirit   vs   ? Hidden      ← upcoming, also withheld
```

You still see that a match exists and when it starts, so the schedule stays
useful; you cannot work out the result of the one you have not watched.

Marking one watched — or just opening its VOD — advances the queue and reveals
the next opponent. As with everything else here, the withheld opponent is
absent from the published state file, not merely undrawn.

Details worth knowing:

- **Queues are scoped per team per wiki.** An org like Team Spirit fields
  separate Dota 2 and Counter-Strike rosters; being behind on one says nothing
  about the other, so a Dota backlog never masks their CS fixtures.
- **The tournament stage is stripped** on a masked fixture, because
  "TI 2026 — Upper Bracket Final" says more about yesterday than the fixture
  does. You see "TI 2026".
- **A backlog expires.** `catchUp.window` (default 48h) bounds how far back an
  unwatched match still counts, so a match you skipped last month does not hide
  your schedule forever.
- If two followed teams meet and both are behind, **both** sides are withheld.
- `omarchy-esports reveal <id>` overrides it per match, and
  `catchUp.enabled: false` turns it off entirely.

## How the data works

Everything is keyless. No account, no API key, no quota.

```
Liquipedia:Matches ticker  (api.php action=parse)
        │  match → tournament page link
        ▼
Tournament page  ──► Twitch channels ──► stream link on the match card
        │
        └────────► YouTube channel id ──► RSS feed ──► VOD discovery
```

The ticker gives fixtures, times, formats, tournaments and team artwork —
including separate light and dark logo variants, which is why the widget's
artwork follows your omarchy theme. It does *not* give stream links except
within two hours of a start, so those come from the tournament page, which is
fetched once and cached hard.

That same page yields the event's YouTube channel ids, which feed the
`/feeds/videos.xml` RSS endpoint — no key, no quota — to find VODs afterwards.

### Team artwork: fetched once, cached forever, never redistributed

Logos are downloaded once into `~/.cache/omarchy-esports/logos/` and served
from disk. The UI originally pointed `Image` elements straight at
liquipedia.net, so every panel open fired dozens of requests for files that
never change — enough to earn **HTTP 429**, at which point logos silently
stopped rendering.

A typical fixture window needs **77 files**. All commons image URLs collapse to
one 128px thumbnail per logo, so a team drawn at 35px in one row and 50px in
another is a single download rather than two. After that it is disk reads
forever, and artwork works offline.

**Why not ship the logos with the app, or host them?** Because that is the one
option that is actually risky. Liquipedia carries team logos under a fair-use
claim explicitly scoped to *their own* use — the file pages say so in terms
("this file's transformative educational use **on Liquipedia**"), and their
copyright policy states plainly that images are "not automatically licensed
under CC-BY-SA 3.0". Some, such as Fnatic's, are held under a permission grant
made to Liquipedia specifically, which their policy says does not extend to
third parties.

Fetching to a user's own machine is analogous to a browser cache. Bundling them
in a release, or re-hosting them, would make this project the entity
reproducing and distributing thousands of trademarked marks — the one plausible
takedown vector, and worse in an MIT repo where downstream users reasonably
assume everything shipped is freely reusable.

Freely-licensed substitutes were investigated and are not viable: of Team
Liquid, G2, Fnatic, NAVI and Cloud9, **none** has a logo on Wikimedia Commons
with a clean, rights-holder-granted free licence.

So: fetch once per user, cache forever, back off when asked to stop, and fall
back to a monogram. This is also what every comparable open-source project
converged on independently.

**When artwork is unavailable** — a rate limit, a datacentre or VPN egress IP,
or simply a team the ticker has never shown — the UI draws a monogram from the
team's own tag. Nothing looks broken.

```bash
omarchy-esports logos status    # how much artwork is cached
omarchy-esports logos warm      # fetch what is missing now
omarchy-esports logos export    # copy the cache into a portable folder
```

`logos export` exists for moving *your own* cache between *your own* machines —
useful if the machine running this sits behind a VPN whose egress Liquipedia
rate-limits. A pack is loaded from
`~/.local/share/omarchy-esports/logos/`, or `$OMARCHY_ESPORTS_LOGO_PACK`. It is
deliberately not something this project ships or hosts.

### Playing nicely with Liquipedia

Liquipedia's [API terms](https://liquipedia.net/api-terms-of-use) are strict and
enforced with automated IP bans, so compliance is built into the client rather
than left to callers:

- **Gzip is mandatory.** A request without `Accept-Encoding: gzip` is rejected
  outright with `406`.
- **`action=parse` is limited to one request per 30 seconds** — it is the
  expensive action, not the cheap one. Everything else is one per 2 seconds. All
  traffic is serialised through a single limiter, so adding wikis lowers the
  refresh rate rather than multiplying the request rate.
- **A descriptive User-Agent with contact details is required.** Set
  `contactEmail` in the config so yours identifies you.
- **Responses are cached on disk**, and stale cache is served if the network is
  down, so a disconnected laptop still shows the last known schedule.
- Only `api.php` is used. Automated access to rendered wiki pages is against the
  terms.
- Match data is CC BY-SA 3.0, attributed in the widget, the app, and the CLI.

An optional LiquipediaDB (LPDB) API key gives structured JSON instead of parsed
HTML. Nothing requires one — put it in `liquipediaApiKey` if you have one.

## Usage

```bash
omarchy-esports status              # the schedule; * marks followed teams
omarchy-esports status --all        # everything, not just your teams
omarchy-esports next                # next followed match, for scripting
omarchy-esports teams add "NAVI"
omarchy-esports teams remove "NAVI"
omarchy-esports reveal <id>         # unblind one result
omarchy-esports hide <id>           # re-blind it
omarchy-esports watched <id>        # advance the catch-up queue
omarchy-esports unwatch <id>
omarchy-esports search spir         # fuzzy-search the team index
omarchy-esports search gamer --game dota2
omarchy-esports team "Team Spirit"  # one team's fixtures
omarchy-esports config set spoilers balanced
omarchy-esports config set catchUp.window 72h
omarchy-esports config wiki starcraft2 off
omarchy-esports open <id> --stream  # open the stream
omarchy-esports open <id> --vod     # open the VOD
omarchy-esports refresh             # force a poll
omarchy-esports config edit
```

Team names are matched case-insensitively against both Liquipedia's canonical
name and the ticker abbreviation, so `NAVI` and `Natus Vincere` both work.

### Finding teams, and following one game not another

The app's Teams tab searches as you type, from two characters, showing each
team's logo and game, with filter chips for each game. The index is built from
every team seen in a ticker, so it costs no extra API calls, works offline, and
naturally covers the teams that are actually competing.

The index has two sources. Teams seen in a fetched fixture arrive with their
artwork and short name and are marked **playing**; the rest come from each
wiki's `Category:Teams`, which lists every team page on that wiki — around
2,900 teams across the three default games. Without that second source the
index only knew teams with a fixture in the current window, so an org between
events was invisible: Team Liquid has Dota fixtures today and none in
Counter-Strike, and could not be followed for CS at all.

Enumerating a category is an `action=query` call, rate-limited at one per two
seconds rather than the one per thirty seconds a page parse costs, so a full
sweep is a few seconds per wiki and is repeated weekly.

**Orgs are indexed per game, not per name.** GamerLegion, NAVI, Team Spirit,
Team Falcons and Aurora Gaming all field rosters in more than one wiki, so each
appears once per game with its own artwork and its own follow state:

```bash
omarchy-esports search "team liquid"
# TEAM         SHORT   GAME            STATUS   FOLLOWED
# Team Liquid  Liquid  Dota 2          playing  *
# Team Liquid          Counter-Strike
# Team Liquid          StarCraft II

omarchy-esports search gamer
# TEAM         SHORT  GAME            STATUS   FOLLOWED
# GamerLegion  GL     Counter-Strike
# GamerLegion  GL     Dota 2          playing  *

omarchy-esports teams add "GamerLegion" --game dota2   # their Dota roster only
omarchy-esports teams add "Team Spirit"                # every game
```

A scoped entry only matches fixtures from that wiki — including for
notifications and catch-up masking, so being behind on GamerLegion's Dota
matches never hides their Counter-Strike ones. In the config the two forms sit
side by side:

```jsonc
"teams": ["Team Spirit", { "name": "GamerLegion", "wiki": "dota2" }]
```

Ranking prefers exact matches, then prefixes, then word prefixes, then
substrings, and finally subsequences, so `spir` finds Team Spirit before Team
Spirit Academy, and `navi` finds Natus Vincere by its abbreviation. A team with
a current fixture outranks a dormant one, which is why the Dota row comes first
above.

Teams known only from the directory have no logo until they next play — the
ticker is where artwork comes from — so they render as a monogram derived from
the team's own tag where one is known ("TSpirit", "NAVI") and from its name
otherwise. Artwork that is temporarily unavailable degrades the same way, which
is why a missing logo looks deliberate rather than broken.

## First run

The app opens a short wizard the first time: which games, which teams, how much
to hide. Every step writes immediately, so quitting halfway still leaves a
working config. `omarchy-esports setup --reset` brings it back;
`--done` skips it.

Only Counter-Strike and Dota 2 are on to begin with. Each enabled game costs a
30-second parse slot per refresh, so starting narrow keeps a fresh install
quick, and the wizard is where you widen it.

## Tournament tiers

Liquipedia rates every tournament, and the rating is what separates a major
from a Tuesday-night cup:

```bash
omarchy-esports config set minTier 1          # S-Tier only
omarchy-esports config set hideMinorEvents true
```

The ticker shows tier filter buttons but carries no tier on the match rows, so
it comes from each tournament's infobox — a cheap query rather than a page
parse. Two details make it work honestly:

- **Ticker links point at stage sub-pages.** "The International/2026/Main Event"
  uses a HiddenDataBox and has no tier of its own, so the lookup walks up to the
  parent tournament. Without that, coverage was 124 of 198 matches; with it, 154
  and rising as more tournaments are resolved.
- **The encoding differs per wiki.** Counter-Strike writes `S-Tier`, Dota 2
  writes `1`. Both normalise to Liquipedia's numeric scale.
- **Matches whose tier is not yet known are always kept.** Tiers resolve
  lazily and are bounded per refresh, so hiding unknowns would make fixtures
  flicker in and out of the schedule.

`hideMinorEvents` is a separate axis — event format rather than prestige —
dropping anything tagged Qualifier, Weekly, Monthly or Showmatch. A weekly cup
can still carry a respectable tier, so the two filters work best together.

## Settings

The app has a Settings tab covering the blackout policy, catch-up masking and
its window, which games to poll, the notification toggles, and the refresh
interval. Every control writes through the CLI rather than editing the file
directly, so validation and clamping happen in one place — setting a poll
interval below the floor, or a malformed duration, is rejected rather than
silently corrected later.

The same settings from a terminal:

```bash
omarchy-esports config set spoilers balanced
omarchy-esports config set catchUp.enabled false
omarchy-esports config set notifications.vodReady false
omarchy-esports config wiki starcraft2 off
omarchy-esports config show
```

The daemon notices config changes within about 30 seconds — no restart.

## Configuration

`~/.config/omarchy-esports/config.json`:

```jsonc
{
  "teams": ["Team Spirit", "G2 Esports"],
  "wikis": [
    { "slug": "dota2",         "game": "Dota 2",        "tickerPage": "Liquipedia:Matches", "enabled": true },
    { "slug": "counterstrike", "game": "Counter-Strike","tickerPage": "Liquipedia:Matches", "enabled": true },
    { "slug": "starcraft2",    "game": "StarCraft II",  "tickerPage": "Main_Page",          "enabled": true }
    // ...plus the rest of the catalog, disabled. `games on <slug>` flips one.
  ],
  "spoilers": "strict",        // strict | balanced | off
  "catchUp": {
    "enabled": true,           // withhold opponents while you are behind
    "window": "48h0m0s"        // how far back an unwatched match counts
  },
  "followedOnly": false,       // restrict the ticker to your teams
  "hideTBD": true,             // drop unseeded bracket slots
  "horizon": "336h0m0s",       // how far ahead to track
  "pollInterval": "15m0s",     // floored at 5m
  "contactEmail": "you@example.com",
  "notifications": {
    "matchStarting": true, "leadTime": "10m0s",
    "matchLive": true, "vodReady": true, "tournamentStarting": true,
    "quiet": false
  }
}
```

## Games

```bash
omarchy-esports games list
omarchy-esports games on valorant
omarchy-esports games off starcraft2
```

The catalog holds 37 wikis, each verified against the live API: the slug
resolves, the ticker page exists, and parsing it returns fixtures in the format
this tool reads.

Existence alone does not qualify a game. StarCraft II has a
`Liquipedia:Upcoming_and_ongoing_matches` page that looks exactly right and
serves matches from 2024-25 — which is why its entry points at `Main_Page`,
as does StarCraft: Brood War's. Games whose ticker could not be confirmed are
left out with the reason recorded in `internal/config/catalog.go`: Smash and
Age of Empires have no ticker markup, Formula 1 uses a widget this parser does
not read, and `streetfighter` and `sixsiege` are not Liquipedia slugs at all.
A wrong entry is worse than a missing one — it adds a game that silently shows
nothing.

Two entries (Rainbow Six, EA Sports FC) parsed correctly but held only finished
matches when checked; their scenes were between events, and the catalog records
the fixture count seen at verification so a quiet game is distinguishable from
a broken one.

**Each enabled game costs a 30-second parse slot per refresh**, as does each
tournament page fetched for stream links. The daemon warns at startup when the
enabled games cannot be fetched inside the poll interval, and suggests either a
longer interval or one fewer game.

> StarCraft II is the odd one out: its `Liquipedia:Matches` page is a stale
> archive that still serves matches from 2024–25, so its ticker is read from
> `Main_Page` instead. If you add a wiki and get nothing, check that page first.

Widget settings live on the bar entry in `~/.config/omarchy/shell.json`:

```bash
omarchy bar set contra.esports showLabel false
omarchy bar set contra.esports maxRows 14
```

`showLabel`, `maxRows`, `showFinished`, `hideWhenEmpty`.

## Layout

```
cmd/omarchy-esports/     CLI and daemon entry point
internal/liquipedia/     rate-limited API client, ticker + tournament parsers
internal/youtube/        keyless RSS VOD discovery and match association
internal/spoiler/        redaction engine and leak detection
internal/store/          the two-file public/private state split
internal/fuzzy/          team-search ranking (mirrored in shared/Model.js)
internal/liquipedia/directory.go   per-wiki team catalogue enumeration
internal/daemon/         polling, enrichment, notifications
internal/config/         user configuration
shared/Model.js          formatting logic shared by widget and app
plugin/contra.esports/   Quickshell bar widget
app/                     standalone Quickshell app
```

`shared/Model.js` is mirrored into both QML targets by `sync-shared.sh`, since
neither a plugin nor an app can import from outside its own directory. Edit the
copy in `shared/`.

## Development

```bash
./run-tests.sh                      # Go suites, the shared JS model, manifest validation
go test ./...                       # real captured Liquipedia HTML, masking rules, redaction
node tests/model.test.mjs           # shared/Model.js
DEV_LINK=1 ./install.sh             # symlink the plugin from this checkout
omarchy restart shell               # reload after a plugin edit
journalctl -t omarchy-shell -f      # QML errors
journalctl --user -u omarchy-esports -f
qs -p ./app -n                      # run the app against the working copy
```

The parser tests run against real captured `api.php` responses in
`internal/liquipedia/testdata/`, so they exercise the markup Liquipedia
actually serves rather than an idealised version of it.

Two things that will cost you an hour if you do not know them:

- **A symlinked plugin does not hot-reload.** The shell watches
  `~/.config/omarchy/plugins` with `inotifywait -r`, which does not follow
  symlinks, so `rescanPlugins` silently keeps serving the old code. Use
  `omarchy restart shell` while developing against a symlink.
- **A bar widget must set `implicitWidth` and `implicitHeight` on its root.**
  An `Item` does not take its size from its children, so a widget that omits
  them is zero-width and simply never appears — with no error to tell you why.
  And do not `anchors.fill: parent` the layout you derive that size from, or the
  two will wait on each other and every column collapses onto the same x.

## Publishing the plugin

`./package-plugin.sh` builds the standalone tree the community catalog needs.
`omarchy plugin add` clones a repo and only ever validates `manifest.json` at
the **repository root**, so the plugin cannot be published from its
subdirectory here. See [docs/PUBLISHING.md](docs/PUBLISHING.md) for the
submission process, the plugin-id decision, and the pre-submission checklist.

## Licence

MIT.

Match and team data is sourced from [Liquipedia](https://liquipedia.net) under
the [Liquipedia API Terms of Use](https://liquipedia.net/api-terms-of-use);
their text content is available under
[CC BY-SA 3.0](https://creativecommons.org/licenses/by-sa/3.0/).

Team and organisation logos are trademarks of their respective owners, fetched
at runtime to each user's own machine for identification purposes only. They
are **not** covered by Liquipedia's CC BY-SA licence and are **not**
redistributed as part of this software. Their use here does not imply
endorsement by, or affiliation with, the teams, the organisations, or
Liquipedia. If you are a rights holder and want a logo removed, please open an
issue.
