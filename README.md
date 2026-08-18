# omarchy-esports

Spoiler-free esports schedule for [Omarchy](https://omarchy.org): a bar widget,
a desktop app, and a daemon that tracks upcoming matches, ties them to live
streams, and finds VODs afterwards — without telling you who won.

Covers **Dota 2**, **Counter-Strike** and **StarCraft II** out of the box, and
any other Liquipedia wiki you add.

---

## What it does

- **Bar widget** — the next match for a team you follow, with a live countdown.
  Click for a panel of live / upcoming / recent matches.
- **App** — a fuller view with team artwork, tabs, and follow-list management.
- **Spoiler-free by construction** — see below. This is the point of the thing.
- **Stream links** — resolved from the tournament's broadcast table, so a match
  card can send you straight to the right Twitch channel.
- **VOD discovery** — finds the recording on YouTube once a match is done, with
  no API key and no quota.
- **Notifications** — followed team starting, going live, VOD ready, tournament
  starting. Clicking the notification opens the stream.

## Install

```bash
git clone https://github.com/contra/omarchy-esports
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
of copying it.

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
omarchy-esports open <id> --stream  # open the stream
omarchy-esports open <id> --vod     # open the VOD
omarchy-esports refresh             # force a poll
omarchy-esports config edit
```

Team names are matched case-insensitively against both Liquipedia's canonical
name and the ticker abbreviation, so `NAVI` and `Natus Vincere` both work.

## Configuration

`~/.config/omarchy-esports/config.json`:

```jsonc
{
  "teams": ["Team Spirit", "G2 Esports"],
  "wikis": [
    { "slug": "dota2",         "game": "Dota 2",        "tickerPage": "Liquipedia:Matches", "enabled": true },
    { "slug": "counterstrike", "game": "Counter-Strike","tickerPage": "Liquipedia:Matches", "enabled": true },
    { "slug": "starcraft2",    "game": "StarCraft II",  "tickerPage": "Main_Page",          "enabled": true }
  ],
  "spoilers": "strict",        // strict | balanced | off
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

Adding a wiki is just another entry — `valorant`, `leagueoflegends`,
`rocketleague`, `apexlegends` and friends all use `Liquipedia:Matches`.

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
go test ./...                       # 34 tests, incl. real captured Liquipedia HTML
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

## Licence

MIT. Match data from [Liquipedia](https://liquipedia.net), CC BY-SA 3.0.
