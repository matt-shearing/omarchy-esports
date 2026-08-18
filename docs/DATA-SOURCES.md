# Data sources — verified findings

All findings below were verified live against the real endpoints on 2026-08-18,
not taken from documentation.

## Liquipedia

### Hard requirements (discovered by being rejected)

1. **Gzip is mandatory.** Any request to `api.php` without `Accept-Encoding: gzip`
   returns `406 Gzip encoding is required for API requests`. This is not optional
   and not documented in the obvious place.
2. **Descriptive User-Agent required**, per liquipedia.net/api-terms-of-use.
   We send: `omarchy-esports/<ver> (<repo-url>; <contact>) go-http/<ver>`
3. **Rate limits**: `action=parse` 1 req / 2s; other actions 1 req / 30s.
   The daemon serialises ALL Liquipedia traffic through a single token-bucket
   limiter so these can never be exceeded regardless of how many wikis are enabled.

### Two access paths — we implement both

| | LPDB v3 | api.php `action=parse` |
|---|---|---|
| Endpoint | `api.liquipedia.net/api/v3/match` | `liquipedia.net/<wiki>/api.php` |
| Auth | **API key required** | none |
| Verified | returns `{"error":["API key \"\" is not valid."]}` — endpoint live, key gates it | returns full match ticker HTML |
| Data | structured JSON | HTML, must be parsed |

Design decision: **the keyless path is the default** so the app works the moment
it is installed. LPDB is used automatically *if* the user supplies a key, giving
richer/structured data. No feature is gated behind having a key.

### Keyless match ticker — structure

`GET /<wiki>/api.php?action=parse&page=Liquipedia:Matches&format=json&prop=text`

Returns ~336KB HTML, 64 match blocks for dota2. Each `div.match-info` yields:

- `span.timer-object[data-timestamp]` → unix start time (authoritative, TZ-safe)
- `div.match-info-header-opponent` ×2 → team page href (`/dota2/Team_Spirit`),
  canonical name (`title=`), short display name, and logo image
  - **logos ship as `team-template-lightmode` / `team-template-darkmode` variants**,
    which we map onto the active omarchy theme
- `span.match-info-header-scoreholder-lower` → `(Bo3)` format
- `div.match-info-tournament` → tournament name, tournament page href, tournament icon
- Match id (e.g. `Match:ID_TI2026Main_R01-M001`)

Not present in the ticker: **stream links**. Those come from the tournament page.

### Tournament page → broadcasts

`GET /<wiki>/api.php?action=parse&page=<tournament-page>&format=json&prop=text`

Verified on `The_International/2026/Main_Event`, which exposed the full
multi-language broadcast list:

- Twitch: `dota2ti`, `dota2ti_ru`, `dota2ti_es`, `dota2ti_cn`, `dota2_maincast`, `arabicdota`
- YouTube: `youtube.com/channel/UCTQKT5QqO3h7y32G8VzuySQ/streams`, `@Dota2_maincast/streams`
- Liquipedia stream pages: `Special:Stream/twitch/The_International`

This is the key link in the chain: it gives us both the **live stream URL** for an
upcoming match and the **YouTube channel id** to watch for that event's VODs.

### Resolved pipeline (fully keyless)

```
Liquipedia:Matches ticker
        │  match → tournament page href
        ▼
Tournament page  ──► Twitch channels ──► stream link on the match card
        │
        └────────► YouTube channel id ──► RSS feed ──► VOD discovery
```

Tournament pages are fetched once and cached hard (they change rarely), so the
per-match stream link costs no extra requests.
