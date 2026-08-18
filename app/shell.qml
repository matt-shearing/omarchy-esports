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

    readonly property string home: Quickshell.env("HOME") || ""
    readonly property string stateDir: home + "/.local/state/omarchy-esports"

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

    function run(args, label) {
        if (proc.running) return
        app.busy = label || ""
        proc.command = ["omarchy-esports"].concat(args)
        proc.running = true
    }

    Connections {
        target: proc
        function onExited() {
            app.busy = ""
            stateFile.reload()
            teamsFile.reload()
        }
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
        if (s && s.url) Qt.openUrlExternally(s.url)
    }

    function isFollowed(name) {
        var n = String(name || "").toLowerCase().trim()
        for (var i = 0; i < app.model.teams.length; i++) {
            if (String(app.model.teams[i]).toLowerCase().trim() === n) return true
        }
        return false
    }

    function toggleFollow(name) {
        if (!name) return
        app.run(["teams", isFollowed(name) ? "remove" : "add", name],
                isFollowed(name) ? "unfollowing…" : "following…")
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

    readonly property var vods: Model.vodSections(model.matches)
    readonly property var searchResults: Model.searchTeams(teamIndex, teamQuery, { minChars: 2, limit: 12 })
    readonly property var teamDetail: Model.teamMatches(model.matches, selectedTeam)

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

        ColumnLayout {
            anchors.fill: parent
            anchors.margins: 18
            spacing: 14

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
                        { id: "teams", label: "Teams" }
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
                            onInspectTeam: function (name) { app.selectedTeam = name }
                        }
                    }
                }

                // 1 — VODs and the catch-up queue
                Item {
                    Text {
                        anchors.centerIn: parent
                        visible: vodList.count === 0
                        text: "No recordings yet.\n\nVODs appear once a followed team's match finishes\nand the broadcaster uploads it."
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
                                    onWatch: app.watch(modelData.match)
                                    onReveal: app.run(["reveal", modelData.match.id], "revealing…")
                                    onMarkWatched: app.run(["watched", modelData.match.id], "updating…")
                                    onInspectTeam: function (name) { app.selectedTeam = name }
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

                    Text {
                        text: {
                            var q = app.teamQuery.trim()
                            if (q.length === 0)
                                return "Following " + app.model.teams.length + " team(s) · " +
                                       app.teamIndex.length + " teams indexed from recent matches"
                            if (q.length < 2) return "Keep typing…"
                            if (app.searchResults.length === 0)
                                return "No match for \"" + q + "\" — the index covers teams seen in recent fixtures"
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
                            var out = []
                            for (var i = 0; i < app.model.teams.length; i++) {
                                var name = app.model.teams[i]
                                var found = null
                                for (var j = 0; j < app.teamIndex.length; j++) {
                                    var t = app.teamIndex[j]
                                    if (String(t.name).toLowerCase() === String(name).toLowerCase() ||
                                        String(t.short || "").toLowerCase() === String(name).toLowerCase()) {
                                        found = t
                                        break
                                    }
                                }
                                out.push(found ? found : { name: name, short: "", game: "", logo: {} })
                            }
                            return out
                        }

                        delegate: TeamResultRow {
                            required property var modelData
                            width: ListView.view.width
                            team: modelData
                            followed: app.isFollowed(modelData.name)
                            onToggleFollow: app.toggleFollow(modelData.name)
                            onInspect: app.selectedTeam = modelData.name
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
                            text: app.selectedTeam
                            color: Theme.foreground
                            font.family: Theme.fontFamily
                            font.pixelSize: Theme.fontTitle
                            font.bold: true
                        }

                        Item { Layout.fillWidth: true }

                        AppButton {
                            text: app.isFollowed(app.selectedTeam) ? "Unfollow" : "Follow"
                            accentuated: !app.isFollowed(app.selectedTeam)
                            onClicked: app.toggleFollow(app.selectedTeam)
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
            }

            // ---- footer ----
            RowLayout {
                Layout.fillWidth: true

                Text {
                    text: app.model.attribution !== "" ? app.model.attribution : "Match data from Liquipedia (CC BY-SA 3.0)"
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
