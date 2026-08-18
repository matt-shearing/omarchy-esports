import QtQuick
import QtQuick.Layouts
import Quickshell
import Quickshell.Io
import qs.Commons
import qs.Ui
import "Model.js" as Model

// Esports bar widget: shows the next followed match on the bar and opens a
// schedule panel listing live, upcoming and recently finished matches.
//
// The widget never talks to the network. It reads the redacted state file the
// daemon publishes, which is what makes the spoiler guarantee hold: a result
// the user has not revealed is simply not present in the file this reads.
Panel {
    id: root

    moduleName: "contra.esports"
    ipcTarget: "contra.esports"
    manageIpc: false

    // ---- settings (see manifest.json barWidget.schema) ----
    readonly property bool showLabel: setting("showLabel", true) === true
    readonly property int maxRows: Math.max(3, Number(setting("maxRows", 10)))
    readonly property bool showFinished: setting("showFinished", true) === true
    readonly property bool hideWhenEmpty: setting("hideWhenEmpty", false) === true

    // ---- state ----
    property var model: Model.parseState("")
    property double nowMs: Date.now()
    property int focusIndex: 0
    property bool cursorActive: false

    readonly property string home: (bar && bar.shell && bar.shell.home) ? bar.shell.home : ""
    readonly property string statePath: home + "/.local/state/omarchy-esports/state.json"

    // Omarchy themes can be light or dark; pick artwork to match by measuring
    // the bar background rather than guessing from the theme name.
    readonly property bool darkTheme: {
        var c = bar ? bar.background : Color.background
        if (!c) return true
        return (0.299 * c.r + 0.587 * c.g + 0.114 * c.b) < 0.5
    }

    readonly property var barInfo: Model.barLabel(model.matches, nowMs, showLabel)
    readonly property var groups: Model.sections(model.matches, nowMs, {
        showFinished: root.showFinished,
        followedOnly: false
    })

    // ModuleSlot sizes a bar widget from these. An Item does not take its
    // size from its children, so omitting them collapses the widget to zero
    // width and it never renders.
    implicitWidth: button.implicitWidth
    implicitHeight: button.implicitHeight

    visible: !(hideWhenEmpty && model.matches.length === 0)

    // ---- data source ----
    FileView {
        id: stateFile
        path: root.statePath
        watchChanges: true
        printErrors: false
        onLoaded: root.model = Model.parseState(text())
        onLoadFailed: root.model = Model.parseState("")
        // text() is stale inside the change signal, so reload and let onLoaded
        // deliver the new contents.
        onFileChanged: reload()
    }

    // The state file may not exist before the daemon's first run; FileView
    // cannot watch a missing path, so re-probe until it appears.
    Timer {
        interval: 5000
        running: !root.model.ok
        repeat: true
        onTriggered: stateFile.reload()
    }

    // Clock for countdowns. One second while the panel is open so the numbers
    // move; a slow tick otherwise, since the bar only shows minutes.
    Timer {
        interval: root.opened ? 1000 : 20000
        running: true
        repeat: true
        onTriggered: root.nowMs = Date.now()
    }

    // No IpcHandler here on purpose. A bar widget is instantiated once per
    // monitor, and an IPC target routes to exactly one of those instances, so
    // a hotkey bound to a per-target handler opens the panel on whichever
    // monitor happened to claim the target rather than the focused one.
    // Omarchy routes hotkeys through the bar instead, which picks the focused
    // screen. Bind keys to:
    //     omarchy-shell shell toggle contra.esports
    // That works because Ui.Panel gives this root the open()/close()/opened
    // interface Bar.findPanelWidget looks for.

    // ---- actions ----
    Process { id: actionProc }

    function run(args) {
        if (actionProc.running) return
        actionProc.command = ["omarchy-esports"].concat(args)
        actionProc.running = true
    }

    function refresh() { run(["refresh"]) }
    function revealMatch(id) { run(["reveal", id]) }

    function activate(m) {
        if (!m) return
        if (m.vod && m.vod.url) {
            Qt.openUrlExternally(m.vod.url)
            root.close()
            return
        }
        var s = Model.preferredStream(m)
        if (s && s.url) {
            Qt.openUrlExternally(s.url)
            root.close()
        }
    }

    onOpenedChanged: if (opened) { stateFile.reload(); focusIndex = 0; cursorActive = false }

    // ---- bar button ----
    BarIconButton {
        id: button
        bar: root.bar
        active: root.barInfo.live
        text: {
            var glyph = root.barInfo.live ? "󰐊" : "󰊴"
            if (!root.showLabel || root.barInfo.text === "") return glyph
            return glyph + "  " + Model.truncate(root.barInfo.text, 32)
        }
        slotSize: Style.bar.iconSlot
        fixedWidth: (root.showLabel && root.barInfo.text !== "") ? -1 : slotSize
        tooltipText: {
            if (!root.model.ok) return "Esports · waiting for daemon"
            if (!root.barInfo.match) return "Esports · nothing scheduled"
            var m = root.barInfo.match
            return Model.fullName(m.opponents[0]) + " vs " + Model.fullName(m.opponents[1]) +
                "\n" + m.tournament.name +
                "\n" + (Model.isLive(m) ? "live now" : "starts " + Model.clockTime(m))
        }
        onPressed: function (b) {
            if (b === Qt.RightButton) root.refresh()
            else root.toggle()
        }
    }

    // ---- dropdown ----
    KeyboardPanel {
        id: panel
        anchorItem: button
        owner: root
        bar: root.bar
        open: root.opened
        focusTarget: keys
        contentWidth: panel.fittedContentWidth(Style.space(560))
        contentHeight: panel.fittedContentHeight(column.implicitHeight)

        PanelKeyCatcher {
            id: keys
            anchors.fill: parent

            onMoveRequested: function (dx, dy) {
                root.cursorActive = true
                var step = dy !== 0 ? dy : dx
                var count = flatMatches.length
                if (count === 0) return
                root.focusIndex = Math.max(0, Math.min(count - 1, root.focusIndex + step))
            }
            onActivateRequested: {
                if (!root.cursorActive) return
                var m = flatMatches[root.focusIndex]
                if (m) root.activate(m)
            }
            onCloseRequested: root.close()
            onTabRequested: function (direction) { root.switchPanel(direction) }

            // A flat index over every visible row, so arrow keys walk the list
            // continuously across section headers.
            readonly property var flatMatches: {
                var out = []
                for (var i = 0; i < root.groups.length; i++) {
                    var g = root.groups[i]
                    for (var j = 0; j < g.matches.length; j++) out.push(g.matches[j])
                }
                return out.slice(0, root.maxRows)
            }

            ColumnLayout {
                id: column
                anchors.left: parent.left
                anchors.right: parent.right
                anchors.top: parent.top
                spacing: Style.space(8)

                // Header
                RowLayout {
                    Layout.fillWidth: true
                    spacing: Style.space(8)

                    PanelSectionHeader {
                        text: "ESPORTS"
                        foreground: root.bar ? root.bar.foreground : Color.popups.text
                        fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
                    }

                    Item { Layout.fillWidth: true }

                    Text {
                        visible: root.model.spoilers !== "off"
                        text: "󰈉 spoiler-free"
                        color: root.bar ? root.bar.foreground : Color.popups.text
                        opacity: 0.5
                        font.family: root.bar ? root.bar.fontFamily : Style.font.family
                        font.pixelSize: Style.font.caption
                    }
                }

                PanelSeparator { foreground: root.bar ? root.bar.foreground : Color.popups.text }

                // Empty / error states
                Text {
                    visible: !root.model.ok
                    Layout.fillWidth: true
                    text: "Waiting for the esports daemon.\nRun: omarchy-esports refresh"
                    color: root.bar ? root.bar.foreground : Color.popups.text
                    opacity: 0.6
                    wrapMode: Text.WordWrap
                    font.family: root.bar ? root.bar.fontFamily : Style.font.family
                    font.pixelSize: Style.font.bodySmall
                }

                Text {
                    visible: root.model.ok && root.groups.length === 0
                    Layout.fillWidth: true
                    text: root.model.teams.length === 0
                        ? "No matches scheduled.\nFollow a team: omarchy-esports teams add \"Team Spirit\""
                        : "No matches scheduled for your teams."
                    color: root.bar ? root.bar.foreground : Color.popups.text
                    opacity: 0.6
                    wrapMode: Text.WordWrap
                    font.family: root.bar ? root.bar.fontFamily : Style.font.family
                    font.pixelSize: Style.font.bodySmall
                }

                // Sections
                Repeater {
                    model: root.groups
                    delegate: ColumnLayout {
                        required property var modelData
                        required property int index

                        Layout.fillWidth: true
                        spacing: Style.space(2)

                        // Rows already consumed by earlier sections, so the
                        // maxRows budget applies across the whole list.
                        readonly property int offset: {
                            var n = 0
                            for (var i = 0; i < index; i++) n += root.groups[i].matches.length
                            return n
                        }
                        readonly property var visibleMatches: {
                            var remaining = root.maxRows - offset
                            if (remaining <= 0) return []
                            return modelData.matches.slice(0, remaining)
                        }

                        visible: visibleMatches.length > 0

                        Text {
                            text: modelData.header
                            color: modelData.kind === "live"
                                ? (root.bar ? root.bar.urgent : Color.accent)
                                : (root.bar ? root.bar.foreground : Color.popups.text)
                            opacity: modelData.kind === "live" ? 0.9 : 0.45
                            font.family: root.bar ? root.bar.fontFamily : Style.font.family
                            font.pixelSize: Style.font.caption
                            font.bold: true
                            Layout.topMargin: Style.space(4)
                            Layout.leftMargin: Style.space(4)
                        }

                        Repeater {
                            model: visibleMatches
                            delegate: MatchRow {
                                required property var modelData
                                required property int index

                                Layout.fillWidth: true
                                match: modelData
                                bar: root.bar
                                teams: root.model.teams
                                darkTheme: root.darkTheme
                                nowMs: root.nowMs
                                hasCursor: root.cursorActive &&
                                    root.focusIndex === (parent.offset + index)
                                onActivated: root.activate(modelData)
                                onRevealRequested: root.revealMatch(modelData.id)
                            }
                        }
                    }
                }

                PanelSeparator {
                    visible: root.model.ok
                    foreground: root.bar ? root.bar.foreground : Color.popups.text
                }

                // Footer: attribution is required by Liquipedia's CC BY-SA licence.
                RowLayout {
                    visible: root.model.ok
                    Layout.fillWidth: true
                    spacing: Style.space(8)

                    Text {
                        text: root.model.attribution !== "" ? root.model.attribution : "Data from Liquipedia (CC BY-SA 3.0)"
                        color: root.bar ? root.bar.foreground : Color.popups.text
                        opacity: 0.35
                        font.family: root.bar ? root.bar.fontFamily : Style.font.family
                        font.pixelSize: Style.font.caption
                    }

                    Item { Layout.fillWidth: true }

                    Button {
                        text: "Refresh"
                        fontSize: Style.font.caption
                        foreground: root.bar ? root.bar.foreground : Color.popups.text
                        fontFamily: root.bar ? root.bar.fontFamily : Style.font.family
                        horizontalPadding: Style.spacing.controlPaddingX
                        verticalPadding: Style.spacing.controlPaddingY
                        bordered: true
                        onClicked: root.refresh()
                    }
                }
            }
        }
    }
}
