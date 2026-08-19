.pragma library

// Pure formatting helpers for the esports bar widget. Kept free of QML types
// so the same logic can be reasoned about (and unit-tested) in isolation.

var STATE_LIVE = "live"
var STATE_UPCOMING = "upcoming"
var STATE_FINISHED = "finished"

// parseState decodes the daemon's state.json, tolerating a missing or
// half-written file: the widget must never throw on startup.
function parseState(text) {
  var empty = { matches: [], updatedAt: "", spoilers: "strict", teams: [], errors: [], attribution: "", ok: false }
  if (!text || !String(text).trim()) return empty
  try {
    var doc = JSON.parse(text)
    if (!doc || !Array.isArray(doc.matches)) return empty
    return {
      matches: doc.matches,
      updatedAt: doc.updatedAt || "",
      spoilers: doc.spoilers || "strict",
      teams: Array.isArray(doc.teams) ? doc.teams : [],
      errors: Array.isArray(doc.errors) ? doc.errors : [],
      attribution: doc.attribution || "",
      ok: true
    }
  } catch (e) {
    return empty
  }
}

function startMs(match) {
  var t = Date.parse(match.startsAt)
  return isNaN(t) ? 0 : t
}

// countdown renders the time until a match in a width-stable way, so the bar
// label does not jitter as the clock ticks.
function countdown(match, nowMs) {
  var delta = startMs(match) - nowMs
  if (delta <= 0) return "now"
  var mins = Math.floor(delta / 60000)
  if (mins < 60) return mins + "m"
  var hours = Math.floor(mins / 60)
  if (hours < 24) return hours + "h" + pad(mins % 60) + "m"
  var days = Math.floor(hours / 24)
  return days + "d" + (hours % 24) + "h"
}

function pad(n) { return n < 10 ? "0" + n : String(n) }

// clockTime renders a local wall-clock start time.
function clockTime(match) {
  var ms = startMs(match)
  if (!ms) return ""
  var d = new Date(ms)
  return pad(d.getHours()) + ":" + pad(d.getMinutes())
}

function dayLabel(match, nowMs) {
  var ms = startMs(match)
  if (!ms) return ""
  var d = new Date(ms)
  var now = new Date(nowMs)
  var sameDay = d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth() && d.getDate() === now.getDate()
  if (sameDay) return "Today"
  var tomorrow = new Date(nowMs + 86400000)
  if (d.getFullYear() === tomorrow.getFullYear() && d.getMonth() === tomorrow.getMonth() && d.getDate() === tomorrow.getDate()) return "Tomorrow"
  return ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"][d.getDay()] + " " + d.getDate() + " " +
    ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"][d.getMonth()]
}

function opponentName(o) {
  if (!o) return "?"
  return o.short || o.name || "?"
}

function fullName(o) {
  if (!o) return "?"
  return o.name || o.short || "?"
}

function title(match) {
  return opponentName(match.opponents[0]) + " vs " + opponentName(match.opponents[1])
}

// scoreLabel returns the series score only when the daemon actually supplied
// one. Under a strict spoiler policy an unrevealed match arrives with no score
// at all, so there is nothing here to leak.
function scoreLabel(match) {
  if (match.redacted) return ""
  var s = match.score
  if (!s || (s[0] === 0 && s[1] === 0)) return ""
  return s[0] + "–" + s[1]
}

function isLive(match) { return match.state === STATE_LIVE }
function isFinished(match) { return match.state === STATE_FINISHED }

function bestOfLabel(match) {
  return match.bestOf > 0 ? "Bo" + match.bestOf : ""
}

// preferredStream picks what to open when the user clicks a match: an English
// Twitch feed where available, then any English feed, then anything.
function preferredStream(match) {
  var streams = match.streams || []
  if (!streams.length) return null
  var twitchEN = null, anyEN = null, anyTwitch = null
  for (var i = 0; i < streams.length; i++) {
    var s = streams[i]
    if (s.platform === "twitch" && s.language === "en" && !twitchEN) twitchEN = s
    else if (s.language === "en" && !anyEN) anyEN = s
    else if (s.platform === "twitch" && !anyTwitch) anyTwitch = s
  }
  return twitchEN || anyEN || anyTwitch || streams[0]
}

// logoFor picks the artwork variant matching the active theme.
function logoFor(opponent, darkTheme) {
  if (!opponent || !opponent.logo) return ""
  var l = opponent.logo
  if (darkTheme && l.dark) return l.dark
  if (!darkTheme && l.light) return l.light
  return l.light || l.dark || ""
}

// sections groups matches for display: live first, then upcoming by day, then
// recently finished. Returns [{ header, matches }].
function sections(matches, nowMs, opts) {
  opts = opts || {}
  var live = [], upcoming = [], finished = []
  for (var i = 0; i < matches.length; i++) {
    var m = matches[i]
    if (opts.followedOnly && !m.followed) continue
    if (isLive(m)) live.push(m)
    else if (isFinished(m)) finished.push(m)
    else upcoming.push(m)
  }
  upcoming.sort(function (a, b) { return startMs(a) - startMs(b) })
  finished.sort(function (a, b) { return startMs(b) - startMs(a) })

  var out = []
  if (live.length) out.push({ header: "LIVE NOW", matches: live, kind: "live" })

  var currentDay = ""
  var bucket = null
  for (var j = 0; j < upcoming.length; j++) {
    var label = dayLabel(upcoming[j], nowMs)
    if (label !== currentDay) {
      currentDay = label
      bucket = { header: label.toUpperCase(), matches: [], kind: "upcoming" }
      out.push(bucket)
    }
    bucket.matches.push(upcoming[j])
  }

  if (opts.showFinished && finished.length) {
    out.push({ header: "RECENT", matches: finished, kind: "finished" })
  }
  return out
}

// barLabel builds the text shown on the bar itself: the most relevant match,
// which is a live one if there is one, else the next to start.
function barLabel(matches, nowMs, showLabel) {
  var best = null
  for (var i = 0; i < matches.length; i++) {
    var m = matches[i]
    if (isFinished(m)) continue
    if (!m.followed) continue
    if (isLive(m)) { best = m; break }
    if (startMs(m) > nowMs && (!best || startMs(m) < startMs(best))) best = m
  }
  if (!best) return { text: "", match: null, live: false }
  if (!showLabel) return { text: "", match: best, live: isLive(best) }
  var lead = isLive(best) ? "LIVE" : countdown(best, nowMs)
  return { text: title(best) + "  " + lead, match: best, live: isLive(best) }
}

// truncate keeps the bar from growing without bound on long team names.
function truncate(s, n) {
  if (!s || s.length <= n) return s || ""
  return s.substring(0, n - 1) + "…"
}

// isFollowedTeam reports whether a specific opponent is on the follow list, so
// the UI can emphasise the side the user actually cares about rather than both
// sides of a followed fixture.
//
// Follow entries carry an optional game scope: an org can be followed in Dota
// but not Counter-Strike, so the match's wiki decides whether a scoped entry
// applies.
function isFollowedTeam(opponent, teams, wiki) {
  if (!opponent || !teams || !teams.length) return false
  var name = String(opponent.name || "").toLowerCase().trim()
  var short = String(opponent.short || "").toLowerCase().trim()
  if (!name && !short) return false
  for (var i = 0; i < teams.length; i++) {
    var t = teams[i]
    var tn = String((t && t.name !== undefined) ? t.name : t).toLowerCase().trim()
    if (!tn || (tn !== name && tn !== short)) continue
    var scope = (t && t.wiki) ? String(t.wiki).toLowerCase() : ""
    if (!scope || !wiki || scope === String(wiki).toLowerCase()) return true
  }
  return false
}

// followLabel renders a follow entry, e.g. "GamerLegion (Dota 2)".
function followLabel(entry) {
  if (!entry) return ""
  var name = (entry.name !== undefined) ? entry.name : entry
  var game = entry.game || entry.wiki || ""
  return game ? name + " (" + game + ")" : String(name)
}

// gamesInIndex lists the distinct games present in the team index, for the
// search filter chips.
function gamesInIndex(index) {
  var seen = {}, out = []
  for (var i = 0; i < index.length; i++) {
    var t = index[i]
    if (!t.wiki || seen[t.wiki]) continue
    seen[t.wiki] = true
    out.push({ wiki: t.wiki, game: t.game || t.wiki })
  }
  out.sort(function (a, b) { return String(a.game).localeCompare(String(b.game)) })
  return out
}

// ---------------------------------------------------------------------------
// Catch-up masking
//
// The daemon withholds the opponent of any match a followed team plays after
// an unwatched one, because knowing who they face next reveals that they won.
// The UI's job is only to render that absence honestly.
// ---------------------------------------------------------------------------

function isMasked(match) { return !!(match && match.masked) }

function isHiddenOpponent(o) { return !!(o && o.hidden) }

// opponentLabel renders a side, showing a placeholder when it was withheld.
function opponentLabel(o) {
  if (isHiddenOpponent(o)) return "?"
  return opponentName(o)
}

function fullOpponentLabel(o) {
  if (isHiddenOpponent(o)) return "Hidden until you catch up"
  return fullName(o)
}

// maskExplanation says why a fixture is hidden, in the UI's own words.
function maskExplanation(match) {
  if (!isMasked(match)) return ""
  var who = match.maskedFor || "a team you follow"
  return "Opponent hidden until you catch up on " + who
}

// maskNote is the compact form, shown alongside the tournament rather than
// replacing it, so a masked card keeps its context.
function maskNote(match) {
  return isMasked(match) ? "opponent hidden" : ""
}

// ---------------------------------------------------------------------------
// Liquipedia links
// ---------------------------------------------------------------------------

var LIQUIPEDIA = "https://liquipedia.net"

function absoluteUrl(path) {
  if (!path) return ""
  if (path.indexOf("http") === 0) return path
  return LIQUIPEDIA + path
}

function tournamentUrl(match) {
  return match && match.tournament ? absoluteUrl(match.tournament.page) : ""
}

function opponentUrl(o) {
  if (!o || o.hidden) return ""
  return absoluteUrl(o.page)
}

// matchUrl is the best page to read about a fixture. Liquipedia has no stable
// per-match page for ticker entries, so the tournament page is the target.
function matchUrl(match) { return tournamentUrl(match) }

// ---------------------------------------------------------------------------
// VODs and the catch-up queue
// ---------------------------------------------------------------------------

function hasVod(match) { return !!(match && match.vod && match.vod.videoId) }

function isHighlightVod(match) {
  return hasVod(match) && match.vod.kind === "highlights"
}

function vodUrl(match) {
  if (!hasVod(match)) return ""
  return match.vod.url || ("https://www.youtube.com/watch?v=" + match.vod.videoId)
}

// vodSections splits recorded matches into the catch-up queue and the archive.
//
// The queue mirrors what the daemon actually computed: a match is in it only
// if the daemon marked it as a queue head or masked it, which happens only
// inside the configured catch-up window. Listing every unwatched match a
// followed team ever played would bury the two or three that matter.
//
// The queue is ordered oldest first, because watching out of order is exactly
// what spoils a bracket.
function vodSections(matches) {
  var queue = [], rest = []
  for (var i = 0; i < matches.length; i++) {
    var m = matches[i]
    if (!isFinished(m)) continue
    if (m.queueHead === true || isMasked(m)) queue.push(m)
    else if (hasVod(m)) rest.push(m)
  }
  queue.sort(function (a, b) { return startMs(a) - startMs(b) })
  rest.sort(function (a, b) { return startMs(b) - startMs(a) })
  return { queue: queue, rest: rest }
}

// teamMatches returns everything involving a team, upcoming first.
function teamMatches(matches, teamName, wiki) {
  var name = String(teamName || "").toLowerCase().trim()
  if (!name) return { upcoming: [], past: [] }
  var scope = String(wiki || "").toLowerCase()
  var upcoming = [], past = []
  for (var i = 0; i < matches.length; i++) {
    var m = matches[i]
    if (scope && String(m.wiki || "").toLowerCase() !== scope) continue
    var hit = false
    for (var j = 0; j < 2; j++) {
      var o = m.opponents[j]
      if (isHiddenOpponent(o)) continue
      if (String(o.name || "").toLowerCase().trim() === name ||
          String(o.short || "").toLowerCase().trim() === name) hit = true
    }
    if (!hit) continue
    if (isFinished(m)) past.push(m); else upcoming.push(m)
  }
  upcoming.sort(function (a, b) { return startMs(a) - startMs(b) })
  past.sort(function (a, b) { return startMs(b) - startMs(a) })
  return { upcoming: upcoming, past: past }
}

// ---------------------------------------------------------------------------
// Fuzzy team search
//
// Mirrors internal/fuzzy in Go. It runs in-process so results appear while
// typing rather than after spawning the CLI on every keystroke.
// ---------------------------------------------------------------------------

var NO_MATCH = -1

function fuzzyScore(query, target) {
  var q = String(query || "").toLowerCase().trim()
  var t = String(target || "").toLowerCase().trim()
  if (!q || !t) return NO_MATCH
  if (q === t) return 1000
  if (t.indexOf(q) === 0) return 800 - Math.min(t.length - q.length, 99)
  if (wordPrefix(t, q)) return 600 - Math.min(t.length - q.length, 99)
  if (t.indexOf(q) >= 0) return 400 - Math.min(t.length - q.length, 99)
  var spread = subsequenceSpread(t, q)
  if (spread >= 0) return 200 - Math.min(spread, 199)
  return NO_MATCH
}

function wordPrefix(target, query) {
  var words = target.split(/[\s._-]+/)
  for (var i = 0; i < words.length; i++) {
    if (words[i].indexOf(query) === 0) return true
  }
  return false
}

function subsequenceSpread(target, query) {
  var ti = 0, first = -1, last = -1
  for (var qi = 0; qi < query.length; qi++) {
    var found = false
    while (ti < target.length) {
      if (target.charAt(ti) === query.charAt(qi)) {
        if (first < 0) first = ti
        last = ti
        ti++
        found = true
        break
      }
      ti++
    }
    if (!found) return -1
  }
  return (last - first + 1) - query.length
}

// searchTeams ranks the index against a query. minChars keeps the list from
// flashing up the entire directory on the first keystroke.
function searchTeams(index, query, opts) {
  opts = opts || {}
  var minChars = opts.minChars === undefined ? 2 : opts.minChars
  var limit = opts.limit || 12
  var wiki = opts.wiki || ""
  var q = String(query || "").trim()
  if (q.length < minChars) return []

  var hits = []
  for (var i = 0; i < index.length; i++) {
    var t = index[i]
    if (wiki && String(t.wiki || "").toLowerCase() !== String(wiki).toLowerCase()) continue
    var score = Math.max(fuzzyScore(q, t.name), fuzzyScore(q, t.short || ""))
    if (score === NO_MATCH) continue
    // A team with a fixture in the current window is far more likely to be the
    // one being searched for than a dormant directory entry.
    if (t.playing) score += 50
    hits.push({ team: t, score: score })
  }
  hits.sort(function (a, b) {
    if (b.score !== a.score) return b.score - a.score
    return String(a.team.name).localeCompare(String(b.team.name))
  })
  return hits.slice(0, limit).map(function (h) { return h.team })
}

// parseConfig decodes config.json for the settings view. The app only reads
// it; every write goes through the CLI so validation and clamping happen in
// one place.
function parseConfig(text) {
  var empty = {
    ok: false, spoilers: "strict", followedOnly: false, hideTBD: true,
    catchUp: { enabled: true, window: "48h0m0s" },
    wikis: [], notifications: {}, pollInterval: "15m0s", contactEmail: ""
  }
  if (!text || !String(text).trim()) return empty
  try {
    var d = JSON.parse(text)
    return {
      ok: true,
      spoilers: d.spoilers || "strict",
      followedOnly: d.followedOnly === true,
      hideTBD: d.hideTBD !== false,
      catchUp: d.catchUp || { enabled: true, window: "48h0m0s" },
      wikis: Array.isArray(d.wikis) ? d.wikis : [],
      notifications: d.notifications || {},
      pollInterval: d.pollInterval || "15m0s",
      contactEmail: d.contactEmail || ""
    }
  } catch (e) {
    return empty
  }
}

// parseTeamIndex decodes teams.json defensively.
function parseTeamIndex(text) {
  if (!text || !String(text).trim()) return []
  try {
    var doc = JSON.parse(text)
    return Array.isArray(doc.teams) ? doc.teams : []
  } catch (e) {
    return []
  }
}
