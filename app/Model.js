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

// isFollowedTeam reports whether a specific opponent is on the follow list,
// so the UI can emphasise the side the user actually cares about rather than
// both sides of a followed fixture.
function isFollowedTeam(opponent, teams) {
  if (!opponent || !teams || !teams.length) return false
  var name = String(opponent.name || "").toLowerCase().trim()
  var short = String(opponent.short || "").toLowerCase().trim()
  for (var i = 0; i < teams.length; i++) {
    var t = String(teams[i] || "").toLowerCase().trim()
    if (t && (t === name || t === short)) return true
  }
  return false
}
