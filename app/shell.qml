import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import "Model.js" as Model

// The esports app: a standalone Quickshell instance showing the full schedule,
// results and follow list.
//
// Like the bar plugin, it reads the daemon's redacted state file and never
// talks to the network itself, so results the user has not revealed are not
// present in anything this process can read. Mutations go back through the CLI.
ShellRoot {
    id: app

    property var model: Model.parseState("")
    property double nowMs: Date.now()
    property string tab: "upcoming"
    property string busy: ""

    readonly property string home: Quickshell.env("HOME") || ""
    readonly property string statePath: home + "/.local/state/omarchy-esports/state.json"

    FileView {
        id: stateFile
        path: app.statePath
        watchChanges: true
        printErrors: false
        onLoaded: app.model = Model.parseState(text())
        onLoadFailed: app.model = Model.parseState("")
        onFileChanged: reload()
    }

    Timer {
        interval: 1000
        running: true
        repeat: true
        onTriggered: app.nowMs = Date.now()
    }

    // The state file may not exist before the daemon's first run.
    Timer {
        interval: 4000
        running: !app.model.ok
        repeat: true
        onTriggered: stateFile.reload()
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
        }
    }

    function watch(m) {
        if (!m) return
        if (m.vod && m.vod.url) { Qt.openUrlExternally(m.vod.url); return }
        var s = Model.preferredStream(m)
        if (s && s.url) Qt.openUrlExternally(s.url)
    }

    // visible is the filtered list for the active tab. Bound as a property
    // rather than called per-delegate so the list rebuilds only when the tab,
    // the filter or the data changes — not on every clock tick.
    readonly property var visible: computeVisible(model, tab, followedOnly.checked)

    function computeVisible(stateModel, activeTab, onlyFollowed) {
        var out = []
        var all = stateModel.matches
        for (var i = 0; i < all.length; i++) {
            var m = all[i]
            if (activeTab === "live" && !Model.isLive(m)) continue
            if (activeTab === "upcoming" && Model.isFinished(m)) continue
            if (activeTab === "results" && !Model.isFinished(m)) continue
            if (onlyFollowed && !m.followed) continue
            out.push(m)
        }
        if (activeTab === "results") out.sort(function (a, b) { return Model.startMs(b) - Model.startMs(a) })
        else out.sort(function (a, b) { return Model.startMs(a) - Model.startMs(b) })
        return out
    }

    FloatingWindow {
        id: win
        title: "Esports"
        implicitWidth: 1120
        implicitHeight: 720
        minimumSize.width: 780
        minimumSize.height: 420
        color: Theme.background
        visible: true

        // Closing the window ends the app, since it is its own process.
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
                        { id: "results", label: "Results" },
                        { id: "teams", label: "Teams" }
                    ]
                    delegate: AppButton {
                        required property var modelData
                        text: modelData.label
                        accentuated: app.tab === modelData.id
                        onClicked: app.tab = modelData.id
                    }
                }

                Item { Layout.fillWidth: true }

                CheckBox {
                    id: followedOnly
                    visible: app.tab !== "teams"
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
                currentIndex: app.tab === "teams" ? 1 : 0

                // Match list
                Item {
                    Text {
                        anchors.centerIn: parent
                        visible: matchList.count === 0
                        text: app.model.ok
                            ? (app.tab === "live" ? "Nothing live right now." : "Nothing here yet.")
                            : "Waiting for the daemon.\n\nStart it with:  systemctl --user start omarchy-esports\nOr run once:    omarchy-esports refresh"
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
                            // Delegates take their width from the view; a
                            // Layout attached property would do nothing here.
                            width: matchList.width
                            match: modelData
                            teams: app.model.teams
                            nowMs: app.nowMs
                            onWatch: app.watch(modelData)
                            onReveal: app.run(["reveal", modelData.id], "revealing…")
                        }
                    }
                }

                // Teams tab
                ColumnLayout {
                    spacing: 12

                    RowLayout {
                        Layout.fillWidth: true
                        spacing: 8

                        TextField {
                            id: teamInput
                            Layout.fillWidth: true
                            placeholderText: "Team name as Liquipedia spells it, e.g. Team Spirit"
                            color: Theme.foreground
                            font.family: Theme.fontFamily
                            font.pixelSize: Theme.fontBody
                            background: Rectangle {
                                color: Theme.alpha(Theme.foreground, 0.05)
                                radius: Theme.radius - 2
                                border.width: 1
                                border.color: teamInput.activeFocus ? Theme.accent : Theme.alpha(Theme.foreground, 0.2)
                            }
                            onAccepted: if (text.trim() !== "") { app.run(["teams", "add", text.trim()], "adding…"); text = "" }
                        }

                        AppButton {
                            text: "Follow"
                            accentuated: true
                            onClicked: if (teamInput.text.trim() !== "") {
                                app.run(["teams", "add", teamInput.text.trim()], "adding…")
                                teamInput.text = ""
                            }
                        }
                    }

                    Text {
                        text: "Following " + app.model.teams.length + " team(s). Notifications and the bar widget follow this list."
                        color: Theme.muted
                        font.family: Theme.fontFamily
                        font.pixelSize: Theme.fontCaption
                    }

                    ListView {
                        Layout.fillWidth: true
                        Layout.fillHeight: true
                        clip: true
                        spacing: 6
                        model: app.model.teams
                        boundsBehavior: Flickable.StopAtBounds
                        ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }

                        delegate: Rectangle {
                            required property var modelData
                            width: ListView.view.width
                            implicitHeight: 40
                            radius: Theme.radius
                            color: Theme.alpha(Theme.foreground, 0.04)

                            RowLayout {
                                anchors.left: parent.left
                                anchors.right: parent.right
                                anchors.verticalCenter: parent.verticalCenter
                                anchors.leftMargin: 12
                                anchors.rightMargin: 8
                                spacing: 8

                                Text {
                                    Layout.fillWidth: true
                                    text: modelData
                                    color: Theme.foreground
                                    font.family: Theme.fontFamily
                                    font.pixelSize: Theme.fontBody
                                }

                                AppButton {
                                    text: "Unfollow"
                                    subtle: true
                                    onClicked: app.run(["teams", "remove", modelData], "removing…")
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
