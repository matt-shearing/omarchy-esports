// Tests for shared/Model.js, the formatting and filtering logic shared by the
// bar plugin and the app.
//
// It is QML-flavoured JavaScript (a `.pragma library` header, no exports), so
// it is loaded by stripping the pragma and evaluating it in a function scope.
// Run with: node tests/model.test.mjs

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, join } from "node:path";

const here = dirname(fileURLToPath(import.meta.url));
const source = readFileSync(join(here, "..", "shared", "Model.js"), "utf8")
  .replace(/^\s*\.pragma\s+library\s*$/m, "");

// Evaluate the library and hand back its top-level functions.
const names = [...source.matchAll(/^function\s+([A-Za-z0-9_]+)\s*\(/gm)].map((m) => m[1]);
const Model = new Function(`${source}\nreturn {${names.join(",")}};`)();

let failures = 0;
function check(name, fn) {
  try {
    fn();
    console.log(`  ok   ${name}`);
  } catch (err) {
    failures++;
    console.log(`  FAIL ${name}\n       ${err.message}`);
  }
}
function eq(actual, expected, msg) {
  const a = JSON.stringify(actual), b = JSON.stringify(expected);
  if (a !== b) throw new Error(`${msg || ""} got ${a}, want ${b}`);
}
function ok(cond, msg) {
  if (!cond) throw new Error(msg || "expected true");
}

console.log("shared/Model.js");

// --- team search ---------------------------------------------------------
// The bug this guards: an org fields rosters in several games, so two entries
// share a name AND a score. Tie-breaking on name alone left their order
// undefined, so the rows could swap between refreshes and a second click would
// follow the wrong game.
check("searchTeams orders multi-game orgs deterministically", () => {
  const index = [
    { name: "FlyQuest", short: "FLY", wiki: "valorant", game: "VALORANT" },
    { name: "FlyQuest", short: "FLY", wiki: "counterstrike", game: "Counter-Strike" },
  ];
  const a = Model.searchTeams(index, "flyquest", { minChars: 2 });
  const b = Model.searchTeams(index.slice().reverse(), "flyquest", { minChars: 2 });
  eq(a.map((t) => t.wiki), b.map((t) => t.wiki), "order must not depend on input order:");
  eq(a.map((t) => t.wiki), ["counterstrike", "valorant"]);
});

check("searchTeams respects the minimum query length", () => {
  const index = [{ name: "Team Spirit", short: "TS", wiki: "dota2" }];
  eq(Model.searchTeams(index, "t", { minChars: 2 }).length, 0);
  eq(Model.searchTeams(index, "te", { minChars: 2 }).length, 1);
});

check("searchTeams filters by game", () => {
  const index = [
    { name: "FlyQuest", wiki: "valorant" },
    { name: "FlyQuest", wiki: "counterstrike" },
  ];
  const hits = Model.searchTeams(index, "fly", { minChars: 2, wiki: "counterstrike" });
  eq(hits.length, 1);
  eq(hits[0].wiki, "counterstrike");
});

check("searchTeams ranks an active roster above a dormant one", () => {
  const index = [
    { name: "Team Liquid", wiki: "counterstrike" },
    { name: "Team Liquid", wiki: "dota2", playing: true },
  ];
  eq(Model.searchTeams(index, "liquid", { minChars: 2 })[0].wiki, "dota2");
});

// --- follow matching -----------------------------------------------------
check("isFollowedTeam honours the game scope", () => {
  const teams = [{ name: "GamerLegion", wiki: "dota2" }];
  const opp = { name: "GamerLegion", short: "GL" };
  ok(Model.isFollowedTeam(opp, teams, "dota2"), "should match its own game");
  ok(!Model.isFollowedTeam(opp, teams, "counterstrike"), "must not match another game");
});

check("isFollowedTeam treats a bare string as every game", () => {
  const teams = ["Team Spirit"];
  const opp = { name: "Team Spirit", short: "TS" };
  ok(Model.isFollowedTeam(opp, teams, "dota2"));
  ok(Model.isFollowedTeam(opp, teams, "counterstrike"));
});

// --- masking -------------------------------------------------------------
check("a withheld opponent renders as hidden, never as a name", () => {
  const hidden = { hidden: true };
  ok(Model.isHiddenOpponent(hidden));
  eq(Model.opponentLabel(hidden), "?");
  ok(Model.fullOpponentLabel(hidden).toLowerCase().includes("hidden"));
});

check("scoreLabel withholds a redacted score", () => {
  eq(Model.scoreLabel({ redacted: true, score: [2, 1] }), "");
  eq(Model.scoreLabel({ score: [2, 1] }), "2–1");
  eq(Model.scoreLabel({ score: [0, 0] }), "");
});

// --- vods ----------------------------------------------------------------
check("vodSections only queues what the daemon marked", () => {
  const matches = [
    { state: "finished", queueHead: true, tournament: {}, startsAt: "2026-01-01T00:00:00Z" },
    { state: "finished", masked: true, tournament: {}, startsAt: "2026-01-02T00:00:00Z" },
    { state: "finished", tournament: {}, startsAt: "2026-01-03T00:00:00Z",
      vod: { videoId: "x" } },
    { state: "upcoming", tournament: {}, startsAt: "2026-01-04T00:00:00Z" },
  ];
  const s = Model.vodSections(matches);
  eq(s.queue.length, 2, "queue:");
  eq(s.rest.length, 1, "archive:");
});

check("vodSections filters by followed team and tournament", () => {
  const matches = [
    { state: "finished", followed: true, queueHead: true,
      tournament: { name: "EWC" }, startsAt: "2026-01-01T00:00:00Z" },
    { state: "finished", followed: false, queueHead: true,
      tournament: { name: "Other" }, startsAt: "2026-01-02T00:00:00Z" },
  ];
  eq(Model.vodSections(matches, { followedOnly: true }).queue.length, 1);
  eq(Model.vodSections(matches, { tournament: "EWC" }).queue.length, 1);
});

// --- monograms -----------------------------------------------------------
check("initialsFor prefers the team's own tag", () => {
  // The ticker abbreviation wins when we have it.
  eq(Model.initialsFor({ name: "Team Spirit", short: "TSpirit" }), "TSPI");
  eq(Model.initialsFor({ name: "Natus Vincere", short: "NAVI" }), "NAVI");
});

check("initialsFor derives something sensible without a tag", () => {
  // Teams known only from a wiki category have no short name.
  eq(Model.initialsFor({ name: "Team Liquid" }), "TL");
  eq(Model.initialsFor({ name: "G2 Esports" }), "G2");   // digit = already a tag
  eq(Model.initialsFor({ name: "NAVI" }), "NAVI");
  eq(Model.initialsFor({ name: "Natus Vincere" }), "NV");
  eq(Model.initialsFor({ name: "" }), "?");
  eq(Model.initialsFor(null), "?");
});

check("monogramHue is stable per name", () => {
  eq(Model.monogramHue("Team Liquid"), Model.monogramHue("Team Liquid"));
  ok(Model.monogramHue("Team Liquid") !== Model.monogramHue("G2 Esports"));
});

// --- config --------------------------------------------------------------
check("parseConfig normalises the follow list", () => {
  const c = Model.parseConfig(JSON.stringify({
    teams: ["Team Spirit", { name: "GamerLegion", wiki: "dota2" }],
  }));
  eq(c.teams.length, 2);
  eq(c.teams[0].name, "Team Spirit");
  eq(c.teams[1].wiki, "dota2");
});

check("parseConfig survives junk", () => {
  ok(!Model.parseConfig("not json").ok);
  ok(!Model.parseConfig("").ok);
  // An existing install must never be shown the wizard by an upgrade.
  ok(Model.parseConfig("{}").setupComplete);
});

check("parseState survives junk", () => {
  eq(Model.parseState("").matches, []);
  eq(Model.parseState("{oops").matches, []);
  ok(!Model.parseState("").ok);
});

console.log(failures === 0 ? "\nall passed" : `\n${failures} failed`);
process.exit(failures === 0 ? 0 : 1);
