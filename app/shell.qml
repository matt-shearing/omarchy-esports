import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import "Model.js" as Model

// The esports app: schedule, catch-up queue, VODs and follow-list management.
//
// Like the bar plugin it reads the daemon's redacted state file and never
// talks to the network itself, so results and opponents the daemon withheld
// are not present in anything this process can read. Mutations go back through
// the CLI, and the daemon notices the config change within a tick.
ShellRoot {
    id: app

    property var model: Model.parseState("")
    property var teamIndex: []
    property double nowMs: Date.now()
    property string tab: "upcoming"
    property string busy: ""

    // Non-empty when the team detail view is open, replacing the tab content.
    property string selectedTeam: ""
    property string teamQuery: ""
    // Empty means every game; otherwise a wiki slug.
    property string gameFilter: ""
    // VODs view filters.
    property bool vodFollowedOnly: false
    property string vodTournament: ""
    property var config: Model.parseConfig("")
    // Shown until first-run setup is done. Held locally as well so finishing
    // the wizard takes effect immediately rather than waiting for the config
    // file to round-trip through the CLI.
    property bool setupDone: false

    readonly property string home: Quickshell.env("HOME") || ""
    readonly property string stateDir: home + "/.local/state/omarchy-esports"
    readonly property string configPath: home + "/.config/omarchy-esports/config.json"

    FileView {
        id: stateFile
        path: app.stateDir + "/state.json"
        watchChanges: true
        printErrors: false
        onLoaded: app.model = Model.parseState(text())
        onLoadFailed: app.model = Model.parseState("")
        onFileChanged: reload()
    }

    FileView {
        id: teamsFile
        path: app.stateDir + "/teams.json"
        watchChanges: true
        printErrors: false
        onLoaded: app.teamIndex = Model.parseTeamIndex(text())
        onLoadFailed: app.teamIndex = []
        onFileChanged: reload()
    }

    // Read-only: the settings view writes through the CLI so validation and
    // clamping stay in one place.
    FileView {
        id: configFile
        path: app.configPath
        watchChanges: true
        printErrors: false
        onLoaded: {
            app.config = Model.parseConfig(text())
            if (app.config.setupComplete) app.setupDone = true
        }
        onLoadFailed: app.config = Model.parseConfig("")
        onFileChanged: reload()
    }

    Timer {
        interval: 1000
        running: true
        repeat: true
        onTriggered: app.nowMs = Date.now()
    }

    // Neither file exists before the daemon's first run, and FileView cannot
    // watch a path that is not there yet.
    Timer {
        interval: 4000
        running: !app.model.ok || app.teamIndex.length === 0
        repeat: true
        onTriggered: { stateFile.reload(); teamsFile.reload() }
    }

    Process { id: proc }

    // Commands used to be dropped whenever one was already in flight, so a
    // second click within the same second silently did nothing. Queue instead.
    property var pending: []

    function run(args, label) {
        if (proc.running) {
            var q = app.pending.slice()
            q.push({ args: args, label: label || "" })
            app.pending = q
            return
        }
        app.busy = label || ""
        proc.command = ["omarchy-esports"].concat(args)
        proc.running = true
    }

    function runNext() {
        if (app.pending.length === 0) {
            app.busy = ""
            return
        }
        var q = app.pending.slice()
        var next = q.shift()
        app.pending = q
        app.busy = next.label
        proc.command = ["omarchy-esports"].concat(next.args)
        proc.running = true
    }

    Connections {
        target: proc
        function onExited() {
            configFile.reload()
            stateFile.reload()
            teamsFile.reload()
            app.runNext()
        }
    }

    function applySetting(key, value) {
        if (key === "__logos") {
            app.run(["logos", "warm"], "fetching artwork…")
            return
        }
        if (key === "__edit") {
            // No terminal is guaranteed, so open the file with the desktop
            // handler rather than assuming $EDITOR.
            Qt.openUrlExternally("file://" + app.configPath)
            return
        }
        app.run(["config", "set", key, value], "saving…")
    }

    function applyWiki(slug, on) {
        app.run(["config", "wiki", slug, on ? "on" : "off"], "saving…")
    }

    function finishSetup() {
        app.setupDone = true
        app.run(["setup", "--done"], "saving…")
    }

    function watch(m) {
        if (!m) return
        if (Model.hasVod(m)) {
            // Route through the CLI so opening a recording also advances the
            // catch-up queue.
            app.run(["open", m.id, "--vod"], "opening…")
            return
        }
        var s = Model.preferredStream(m)
        var u = s ? Model.safeExternalUrl(s.url) : ""
        if (u) Qt.openUrlExternally(u)
    }

    // isFollowed answers for a specific game scope. Passing an empty wiki asks
    // "followed in any game?".
    function isFollowed(name, wiki) {
        var n = String(name || "").toLowerCase().trim()
        // config.teams, not model.teams: the CLI writes the config file
        // synchronously, while the daemon's published state lags by a tick.
        var list = app.config.ok ? app.config.teams : app.model.teams
        for (var i = 0; i < list.length; i++) {
            var t = list[i]
            var tn = String((t && t.name !== undefined) ? t.name : t).toLowerCase().trim()
            if (tn !== n) continue
            var scope = (t && t.wiki) ? String(t.wiki).toLowerCase() : ""
            if (!scope || !wiki || scope === String(wiki).toLowerCase()) return true
        }
        return false
    }

    // toggleFollow scopes to one game when given a wiki, so following an org's
    // Dota roster does not also follow their Counter-Strike one.
    function toggleFollow(name, wiki) {
        if (!name) return
        var on = isFollowed(name, wiki)
        var args = ["teams", on ? "remove" : "add", name]
        if (wiki) args = args.concat(["--game", wiki])
        app.run(args, on ? "unfollowing…" : "following…")
    }

    // visible is the filtered list for the active tab. Bound as a property so
    // it rebuilds only when the tab, filter or data changes — not every tick.
    readonly property var visible: computeVisible(model, tab, followedOnly.checked)

    function computeVisible(stateModel, activeTab, onlyFollowed) {
        var out = []
        var all = stateModel.matches
        for (var i = 0; i < all.length; i++) {
            var m = all[i]
            if (activeTab === "live" && !Model.isLive(m)) continue
            if (activeTab === "upcoming" && Model.isFinished(m)) continue
            if (onlyFollowed && !m.followed) continue
            out.push(m)
        }
        out.sort(function (a, b) { return Model.startMs(a) - Model.startMs(b) })
        return out
    }

    readonly property var vods: Model.vodSections(model.matches, {
        followedOnly: vodFollowedOnly,
        tournament: vodTournament
    })
    readonly property var vodTournaments: Model.tournamentsWithVods(model.matches)
    readonly property var searchResults: Model.searchTeams(teamIndex, teamQuery,
        { minChars: 2, limit: 20, wiki: gameFilter })
    readonly property var indexGames: Model.filterGames(config, teamIndex)
    property string selectedTeamWiki: ""
    readonly property var teamDetail: Model.teamMatches(model.matches, selectedTeam, selectedTeamWiki)

    FloatingWindow {
        id: win
        title: "Esports"
        implicitWidth: 1120
        implicitHeight: 760
        minimumSize.width: 780
        minimumSize.height: 460
        color: Theme.background
        visible: true

        onClosed: Qt.quit()

        SetupWizard {
            anchors.fill: parent
            anchors.margins: 18
            visible: app.config.ok && !app.setupDone
            config: app.config
            teamIndex: app.teamIndex
            followed: app.config.ok ? app.config.teams : app.model.teams
            onApply: function (key, value) { app.applySetting(key, value) }
            onApplyWiki: function (slug, on) { app.applyWiki(slug, on) }
            onFollowTeam: function (name, wiki) { app.toggleFollow(name, wiki) }
            onFinished: app.finishSetup()
        }

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: 18
            spacing: 14
            visible: !(app.config.ok && !app.setupDone)

            // ---- header ----
            RowLayout {
                Layout.fillWidth: true
                spacing: 12

                Text {
                    text: "ESPORTS"
                    color: Theme.foreground
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontHeading
                    font.bold: true
                    font.letterSpacing: 2
                }

                Rectangle {
                    visible: app.model.spoilers !== "off"
                    implicitWidth: spoilerLabel.implicitWidth + 16
                    implicitHeight: 22
                    radius: 11
                    color: Theme.alpha(Theme.accent, 0.15)
                    border.width: 1
                    border.color: Theme.alpha(Theme.accent, 0.5)
                    Text {
                        id: spoilerLabel
                        anchors.centerIn: parent
                        text: "spoiler-free · " + app.model.spoilers
                        color: Theme.accent
                        font.family: Theme.fontFamily
                        font.pixelSize: Theme.fontCaption
                    }
                }

                Item { Layout.fillWidth: true }

                Text {
                    text: app.busy !== "" ? app.busy : (app.model.ok ? "" : "waiting for daemon")
                    color: Theme.muted
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontCaption
                }

                AppButton {
                    text: "Refresh"
                    onClicked: app.run(["refresh"], "refreshing…")
                }
            }

            // ---- tabs ----
            RowLayout {
                Layout.fillWidth: true
                spacing: 8

                Repeater {
                    model: [
                        { id: "upcoming", label: "Upcoming" },
                        { id: "live", label: "Live" },
                        { id: "vods", label: "VODs" },
                        { id: "teams", label: "Teams" },
                        { id: "settings", label: "Settings" }
                    ]
                    delegate: AppButton {
                        required property var modelData
                        text: modelData.label
                        accentuated: app.tab === modelData.id && app.selectedTeam === ""
                        onClicked: { app.tab = modelData.id; app.selectedTeam = "" }
                    }
                }

                Item { Layout.fillWidth: true }

                CheckBox {
                    id: followedOnly
                    visible: app.tab === "upcoming" || app.tab === "live"
                    text: "My teams only"
                    checked: false
                    contentItem: Text {
                        text: followedOnly.text
                        color: Theme.muted
                        font.family: Theme.fontFamily
                        font.pixelSize: Theme.fontCaption
                        leftPadding: followedOnly.indicator.width + 6
                        verticalAlignment: Text.AlignVCenter
                    }
                }
            }

            Rectangle { Layout.fillWidth: true; implicitHeight: 1; color: Theme.alpha(Theme.foreground, 0.12) }

            // ---- content ----
            StackLayout {
                Layout.fillWidth: true
                Layout.fillHeight: true
                currentIndex: {
                    if (app.selectedTeam !== "") return 3
                    if (app.tab === "settings") return 4
                    if (app.tab === "teams") return 2
                    if (app.tab === "vods") return 1
                    return 0
                }

                // 0 — schedule
                Item {
                    Text {
                        anchors.centerIn: parent
                        visible: matchList.count === 0
                        text: app.model.ok
                            ? (app.tab === "live" ? "Nothing live right now." : "Nothing scheduled.")
                            : "Waiting for the daemon.\n\nStart it:  systemctl --user start omarchy-esports\nOr run once:  omarchy-esports refresh"
                        color: Theme.muted
                        horizontalAlignment: Text.AlignHCenter
                        font.family: Theme.fontFamily
                        font.pixelSize: Theme.fontBody
                    }

                    ListView {
                        id: matchList
                        anchors.fill: parent
                        clip: true
                        spacing: 8
                        model: app.visible
                        boundsBehavior: Flickable.StopAtBounds
                        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }

                        delegate: MatchCard {
                            required property var modelData
                            width: matchList.width
                            match: modelData
                            teams: app.model.teams
                            nowMs: app.nowMs
                            onWatch: app.watch(modelData)
                            onReveal: app.run(["reveal", modelData.id], "revealing…")
                            onMarkWatched: app.run(["watched", modelData.id], "updating…")
                            onInspectTeam: function (name) {
                                app.selectedTeam = name
                                app.selectedTeamWiki = modelData.wiki || ""
                            }
                        }
                    }
                }

                // 1 — VODs and the catch-up queue
                ColumnLayout {
                    spacing: 10

                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 6

                        AppButton {
                            text: "My teams"
                            accentuated: app.vodFollowedOnly
                            onClicked: app.vodFollowedOnly = !app.vodFollowedOnly
                        }

                        Rectangle {
                            Layout.preferredWidth: 1
                            Layout.preferredHeight: 18
                            color: Theme.alpha(Theme.foreground, 0.15)
                        }

                        AppButton {
                            text: "All events"
                            accentuated: app.vodTournament === ""
                            onClicked: app.vodTournament = ""
                        }

                        // The active tournament filter, set by clicking a
                        // tournament name on any card below.
                        AppButton {
                            visible: app.vodTournament !== ""
                            text: "✕ " + app.vodTournament
                            accentuated: true
                            onClicked: app.vodTournament = ""
                        }

                        Item { Layout.fillWidth: true }

                        Text {
                            text: app.vodTournament === "" && app.vodTournaments.length > 0
                                ? "click an event name to filter"
                                : ""
                            color: Theme.muted
                            font.family: Theme.fontFamily
                            font.pixelSize: Theme.fontCaption
                        }
                    }

                    Item {
                    Layout.fillWidth: true
                    Layout.fillHeight: true

                    Text {
                        anchors.centerIn: parent
                        visible: vodList.count === 0
                        text: {
                            if (app.vodTournament !== "")
                                return "No recordings for " + app.vodTournament + "."
                            if (app.vodFollowedOnly)
                                return "No recordings for your teams yet.\n\nTurn off \"My teams\" to see everything."
                            return "No recordings yet.\n\nVODs appear once a match finishes and the\nbroadcaster uploads it."
                        }
                        color: Theme.muted
                        horizontalAlignment: Text.AlignHCenter
                        font.family: Theme.fontFamily
                        font.pixelSize: Theme.fontBody
                    }

                    ListView {
                        id: vodList
                        anchors.fill: parent
                        clip: true
                        spacing: 8
                        boundsBehavior: Flickable.StopAtBounds
                        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }

                        // A flat model of section headers and cards, so the
                        // queue and the archive scroll as one list.
                        model: {
                            var rows = []
                            if (app.vods.queue.length > 0) {
                                rows.push({ kind: "header", text: "CATCH UP",
                                    hint: "Oldest first. Watching one unlocks the next." })
                                for (var i = 0; i < app.vods.queue.length; i++)
                                    rows.push({ kind: "match", match: app.vods.queue[i] })
                            }
                            if (app.vods.rest.length > 0) {
                                rows.push({ kind: "header", text: "RECENT RECORDINGS", hint: "" })
                                for (var j = 0; j < app.vods.rest.length; j++)
                                    rows.push({ kind: "match", match: app.vods.rest[j] })
                            }
                            return rows
                        }

                        delegate: Loader {
                            required property var modelData
                            width: vodList.width
                            sourceComponent: modelData.kind === "header" ? headerComponent : cardComponent

                            Component {
                                id: headerComponent
                                ColumnLayout {
                                    spacing: 2
                                    Text {
                                        text: modelData.text
                                        color: Theme.foreground
                                        opacity: 0.5
                                        font.family: Theme.fontFamily
                                        font.pixelSize: Theme.fontCaption
                                        font.bold: true
                                        font.letterSpacing: 1
                                        Layout.topMargin: 10
                                    }
                                    Text {
                                        visible: modelData.hint !== ""
                                        text: modelData.hint
                                        color: Theme.muted
                                        font.family: Theme.fontFamily
                                        font.pixelSize: Theme.fontCaption
                                    }
                                }
                            }

                            Component {
                                id: cardComponent
                                MatchCard {
                                    match: modelData.match
                                    teams: app.model.teams
                                    nowMs: app.nowMs
                                    tournamentClickable: true
                                    onWatch: app.watch(modelData.match)
                                    onReveal: app.run(["reveal", modelData.match.id], "revealing…")
                                    onMarkWatched: app.run(["watched", modelData.match.id], "updating…")
                                    onInspectTeam: function (name) {
                                        app.selectedTeam = name
                                        app.selectedTeamWiki = modelData.match.wiki || ""
                                    }
                                    onInspectTournament: function (name) { app.vodTournament = name }
                                }
                            }
                        }
                    }
                    }
                }

                // 2 — teams
                ColumnLayout {
                    spacing: 12

                    TextField {
                        id: teamInput
                        Layout.fillWidth: true
                        placeholderText: "Search teams — start typing, e.g. spir, navi, g2"
                        color: Theme.foreground
                        font.family: Theme.fontFamily
                        font.pixelSize: Theme.fontBody
                        onTextChanged: app.teamQuery = text
                        background: Rectangle {
                            color: Theme.alpha(Theme.foreground, 0.05)
                            radius: Theme.radius - 2
                            border.width: 1
                            border.color: teamInput.activeFocus ? Theme.accent : Theme.alpha(Theme.foreground, 0.2)
                        }
                    }

                    // Filter chips. An org can field rosters in several games,
                    // so search results are per game and this narrows them.
                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 6

                        Text {
                            text: "Game"
                            color: Theme.muted
                            font.family: Theme.fontFamily
                            font.pixelSize: Theme.fontCaption
                        }

                        AppButton {
                            text: "All"
                            accentuated: app.gameFilter === ""
                            onClicked: app.gameFilter = ""
                        }

                        Repeater {
                            model: app.indexGames
                            delegate: AppButton {
                                required property var modelData
                                // A game switched on but not yet catalogued
                                // says so, rather than looking broken.
                                text: modelData.indexed ? modelData.game
                                    : modelData.game + " (indexing…)"
                                subtle: !modelData.indexed
                                accentuated: app.gameFilter === modelData.wiki
                                onClicked: app.gameFilter = modelData.wiki
                            }
                        }

                        Item { Layout.fillWidth: true }

                        Text {
                            visible: app.indexGames.length < 2
                            text: "Add games in Settings"
                            color: Theme.muted
                            font.family: Theme.fontFamily
                            font.pixelSize: Theme.fontCaption
                        }
                    }

                    Text {
                        text: {
                            var q = app.teamQuery.trim()
                            if (q.length === 0)
                                return "Following " + app.model.teams.length + " team(s) · " +
                                       app.teamIndex.length + " teams indexed from recent matches"
                            if (q.length < 2) return "Keep typing…"
                            if (app.searchResults.length === 0) {
                                // Distinguish "no such team" from "that game
                                // has not been catalogued yet".
                                for (var i = 0; i < app.indexGames.length; i++) {
                                    var g = app.indexGames[i]
                                    if (g.wiki === app.gameFilter && !g.indexed)
                                        return g.game + " is still being indexed — its teams appear after the next refresh"
                                }
                                return "No match for \"" + q + "\""
                            }
                            return app.searchResults.length + " result(s)"
                        }
                        color: Theme.muted
                        font.family: Theme.fontFamily
                        font.pixelSize: Theme.fontCaption
                    }

                    ListView {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        clip: true
                        spacing: 6
                        boundsBehavior: Flickable.StopAtBounds
                        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }

                        // With no query this is the follow list; with one it is
                        // the search results.
                        model: {
                            if (app.teamQuery.trim().length >= 2) return app.searchResults
                            // With no query this is the follow list, resolved
                            // against the index so each entry gets its artwork.
                            var out = []
                            for (var i = 0; i < app.model.teams.length; i++) {
                                var entry = app.model.teams[i]
                                var name = (entry && entry.name !== undefined) ? entry.name : entry
                                var scope = (entry && entry.wiki) ? entry.wiki : ""
                                var found = null
                                for (var j = 0; j < app.teamIndex.length; j++) {
                                    var t = app.teamIndex[j]
                                    var sameName = String(t.name).toLowerCase() === String(name).toLowerCase() ||
                                        String(t.short || "").toLowerCase() === String(name).toLowerCase()
                                    if (!sameName) continue
                                    if (scope && String(t.wiki).toLowerCase() !== String(scope).toLowerCase()) continue
                                    found = t
                                    break
                                }
                                out.push(found ? found
                                    : { name: name, short: "", game: scope, wiki: scope, logo: {} })
                            }
                            return out
                        }

                        delegate: TeamResultRow {
                            required property var modelData
                            width: ListView.view.width
                            team: modelData
                            followed: app.isFollowed(modelData.name, modelData.wiki)
                            onToggleFollow: app.toggleFollow(modelData.name, modelData.wiki)
                            onInspect: {
                                app.selectedTeam = modelData.name
                                app.selectedTeamWiki = modelData.wiki || ""
                            }
                        }
                    }
                }

                // 3 — team detail
                ColumnLayout {
                    spacing: 12

                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 12

                        AppButton {
                            text: "← Back"
                            onClicked: app.selectedTeam = ""
                        }

                        Text {
                            text: app.selectedTeam +
                                (app.selectedTeamWiki !== "" ? "  ·  " + app.selectedTeamWiki : "")
                            color: Theme.foreground
                            font.family: Theme.fontFamily
                            font.pixelSize: Theme.fontTitle
                            font.bold: true
                        }

                        Item { Layout.fillWidth: true }

                        AppButton {
                            text: app.isFollowed(app.selectedTeam, app.selectedTeamWiki) ? "Unfollow" : "Follow"
                            accentuated: !app.isFollowed(app.selectedTeam, app.selectedTeamWiki)
                            onClicked: app.toggleFollow(app.selectedTeam, app.selectedTeamWiki)
                        }
                    }

                    Text {
                        text: app.teamDetail.upcoming.length + " upcoming · " +
                              app.teamDetail.past.length + " played"
                        color: Theme.muted
                        font.family: Theme.fontFamily
                        font.pixelSize: Theme.fontCaption
                    }

                    ListView {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        clip: true
                        spacing: 8
                        boundsBehavior: Flickable.StopAtBounds
                        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }

                        model: {
                            var rows = []
                            if (app.teamDetail.upcoming.length) {
                                rows.push({ kind: "header", text: "UPCOMING", hint: "" })
                                for (var i = 0; i < app.teamDetail.upcoming.length; i++)
                                    rows.push({ kind: "match", match: app.teamDetail.upcoming[i] })
                            }
                            if (app.teamDetail.past.length) {
                                rows.push({ kind: "header", text: "PLAYED", hint: "" })
                                for (var j = 0; j < app.teamDetail.past.length; j++)
                                    rows.push({ kind: "match", match: app.teamDetail.past[j] })
                            }
                            return rows
                        }

                        delegate: Loader {
                            required property var modelData
                            width: ListView.view.width
                            sourceComponent: modelData.kind === "header" ? detailHeader : detailCard

                            Component {
                                id: detailHeader
                                Text {
                                    text: modelData.text
                                    color: Theme.foreground
                                    opacity: 0.5
                                    font.family: Theme.fontFamily
                                    font.pixelSize: Theme.fontCaption
                                    font.bold: true
                                    font.letterSpacing: 1
                                    topPadding: 8
                                }
                            }

                            Component {
                                id: detailCard
                                MatchCard {
                                    match: modelData.match
                                    teams: app.model.teams
                                    nowMs: app.nowMs
                                    onWatch: app.watch(modelData.match)
                                    onReveal: app.run(["reveal", modelData.match.id], "revealing…")
                                    onMarkWatched: app.run(["watched", modelData.match.id], "updating…")
                                }
                            }
                        }
                    }
                }

                // 4 — settings
                SettingsView {
                    config: app.config
                    teamIndex: app.teamIndex
                    onApply: function (key, value) { app.applySetting(key, value) }
                    onApplyWiki: function (slug, on) { app.applyWiki(slug, on) }
                }
            }

            // ---- footer ----
            RowLayout {
                Layout.fillWidth: true

                Text {
                    text: (app.model.attribution !== "" ? app.model.attribution
                        : "Data via Liquipedia (CC BY-SA 3.0)") + " · Team logos © their respective owners"
                    color: Theme.muted
                    opacity: 0.7
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontCaption
                }

                Item { Layout.fillWidth: true }

                Text {
                    visible: app.model.errors.length > 0
                    text: "⚠ " + app.model.errors.length + " source error(s)"
                    color: Theme.urgent
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontCaption
                }
            }
        }
    }
}
