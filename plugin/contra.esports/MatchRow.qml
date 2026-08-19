import QtQuick
import QtQuick.Layouts
import qs.Commons
import qs.Ui
import "Model.js" as Model

// One match in the dropdown. Clicking expands an inline detail panel beneath
// the row rather than replacing the list, so you keep your place.
Rectangle {
    id: row

    property var match: null
    property QtObject bar: null
    property bool darkTheme: true
    property bool hasCursor: false
    property double nowMs: 0
    property var teams: []
    // Game catalog, for the artwork badge. Empty just means a text badge.
    property var games: []
    property bool expanded: false

    signal toggleRequested
    signal watchRequested
    signal revealRequested
    signal watchedRequested
    signal openUrlRequested(string url)

    readonly property color fg: bar ? bar.foreground : Color.popups.text
    readonly property bool live: match ? Model.isLive(match) : false
    readonly property bool finished: match ? Model.isFinished(match) : false
    readonly property bool blacked: match ? (match.redacted === true) : false
    readonly property bool masked: match ? Model.isMasked(match) : false

    // The kickoff line is rendered once here so `facts` below can depend on
    // the string rather than on nowMs: the panel's one-second clock would
    // otherwise hand the Repeater a fresh model array every tick and rebuild
    // every delegate for a label that changes at most once a day.
    readonly property string whenLabel: match
        ? Model.dayLabel(match, nowMs) + " " + Model.clockTime(match)
        : ""

    // Detail lines as label/value data, so every open row lines its values up
    // on one column and a field the daemon did not supply drops its whole line
    // instead of leaving a dangling separator in a run-on string.
    readonly property var facts: {
        if (!match) return []
        var out = [{ label: "When", value: whenLabel }]
        if (Model.bestOfLabel(match)) out.push({ label: "Format", value: Model.bestOfLabel(match) })
        if (match.game) out.push({ label: "Game", value: match.game })
        out.push({ label: "Event", value: match.tournament.name })
        return out
    }

    // Shared by the summary's clock column and the detail's label column, so
    // the open block's values start on the same edge as the team badges above
    // them instead of drifting when either width is tweaked.
    readonly property int gutter: Style.space(52)

    // A collapsed row stays dense so the list packs; an open one reads as a
    // card, so its vertical padding grows to match the horizontal margin
    // `body` already uses and the block sits evenly inside its own fill.
    readonly property int padY: expanded ? Style.space(10) : Style.space(7)

    implicitHeight: body.implicitHeight + padY * 2
    radius: Style.cornerRadius
    color: expanded ? Style.selectedFill
        : (hasCursor || hover.hovered ? Style.hoverFill : "transparent")

    Behavior on color {
        enabled: row.bar ? row.bar.foregroundAnimationEnabled : false
        ColorAnimation { duration: 120 }
    }

    HoverHandler { id: hover }

    TapHandler {
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        onTapped: function (point, button) {
            if (button === Qt.RightButton) row.watchRequested()
            else row.toggleRequested()
        }
    }

    ColumnLayout {
        id: body
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.leftMargin: Style.space(10)
        anchors.rightMargin: Style.space(10)
        spacing: Style.space(6)

        // ---- summary line ----
        RowLayout {
            Layout.fillWidth: true
            spacing: Style.space(10)

            ColumnLayout {
                Layout.preferredWidth: row.gutter
                Layout.alignment: Qt.AlignVCenter
                spacing: 0

                Text {
                    text: row.live ? "LIVE" : (row.finished ? "" : Model.clockTime(row.match))
                    color: row.live ? (row.bar ? row.bar.urgent : Color.accent) : row.fg
                    font.family: row.bar ? row.bar.fontFamily : Style.font.family
                    font.pixelSize: Style.font.bodySmall
                    font.bold: row.live
                }
                Text {
                    visible: !row.live && !row.finished
                    text: Model.countdown(row.match, row.nowMs)
                    color: row.fg
                    opacity: 0.55
                    font.family: row.bar ? row.bar.fontFamily : Style.font.family
                    font.pixelSize: Style.font.caption
                }
                Text {
                    visible: row.finished
                    text: Model.hasVod(row.match) ? "VOD" : "done"
                    color: row.fg
                    opacity: 0.55
                    font.family: row.bar ? row.bar.fontFamily : Style.font.family
                    font.pixelSize: Style.font.caption
                }
            }

            RowLayout {
                Layout.fillWidth: true
                spacing: Style.space(6)

                TeamBadge {
                    opponent: row.match ? row.match.opponents[0] : null
                    darkTheme: row.darkTheme
                    bar: row.bar
                    followed: Model.isFollowedTeam(row.match ? row.match.opponents[0] : null, row.teams, row.match ? row.match.wiki : "")
                }

                Text {
                    text: Model.scoreLabel(row.match) !== "" ? Model.scoreLabel(row.match) : "v"
                    color: row.fg
                    opacity: 0.45
                    font.family: row.bar ? row.bar.fontFamily : Style.font.family
                    font.pixelSize: Style.font.caption
                    Layout.alignment: Qt.AlignVCenter
                }

                TeamBadge {
                    opponent: row.match ? row.match.opponents[1] : null
                    darkTheme: row.darkTheme
                    bar: row.bar
                    followed: Model.isFollowedTeam(row.match ? row.match.opponents[1] : null, row.teams, row.match ? row.match.wiki : "")
                }

                Item { Layout.fillWidth: true }
            }

            ColumnLayout {
                // A Layout never shrinks a child that does not fill, so
                // without this the tournament block held its full width on a
                // narrow panel and pushed the trailing icon off the card.
                // The maximum still caps it once there is room to spare.
                Layout.fillWidth: true
                Layout.maximumWidth: Style.space(150)
                Layout.alignment: Qt.AlignVCenter
                spacing: 0

                Text {
                    Layout.fillWidth: true
                    text: row.match ? Model.truncate(row.match.tournament.name, 28) : ""
                    color: row.fg
                    opacity: 0.7
                    elide: Text.ElideRight
                    horizontalAlignment: Text.AlignRight
                    font.family: row.bar ? row.bar.fontFamily : Style.font.family
                    font.pixelSize: Style.font.caption
                }
                // Game and format, dimmer than the tournament above it: useful
                // orientation when several games share one list, but never
                // competing with the fixture itself.
                RowLayout {
                    Layout.fillWidth: true
                    spacing: 4

                    Item { Layout.fillWidth: true }

                    // The artwork carries the same 0.4 opacity as the text it
                    // sits beside: this line is orientation, and must not
                    // out-shout the fixture above it.
                    Image {
                        source: Model.gameIconForMatch(row.games, row.match)
                        visible: status === Image.Ready
                        Layout.preferredWidth: Style.font.caption
                        Layout.preferredHeight: Style.font.caption
                        fillMode: Image.PreserveAspectFit
                        smooth: true
                        mipmap: true
                        asynchronous: true
                        sourceSize.width: 24
                        sourceSize.height: 24
                        opacity: 0.4
                    }

                    Text {
                        text: {
                            if (!row.match) return ""
                            var bits = []
                            var badge = Model.gameBadge(row.match)
                            if (badge) bits.push(badge)
                            if (Model.bestOfLabel(row.match)) bits.push(Model.bestOfLabel(row.match))
                            return bits.join(" · ")
                        }
                        color: row.fg
                        opacity: 0.4
                        horizontalAlignment: Text.AlignRight
                        font.family: row.bar ? row.bar.fontFamily : Style.font.family
                        font.pixelSize: Style.font.caption
                    }
                }
            }

            Text {
                Layout.alignment: Qt.AlignVCenter
                text: {
                    if (row.masked) return "󰛑"
                    if (row.blacked) return "󰈉"
                    if (Model.hasVod(row.match)) return "󰕧"
                    if (row.match && Model.preferredStream(row.match)) return "󰐊"
                    return ""
                }
                color: (row.masked || row.blacked) ? row.fg
                    : (row.live ? (row.bar ? row.bar.urgent : Color.accent) : row.fg)
                opacity: (row.masked || row.blacked) ? 0.4 : 0.8
                font.family: row.bar ? row.bar.fontFamily : Style.font.family
                font.pixelSize: Style.font.icon
            }
        }

        // ---- expandable detail ----
        ColumnLayout {
            Layout.fillWidth: true
            Layout.topMargin: Style.space(2)
            visible: row.expanded
            spacing: Style.space(8)

            // A Layout hands out width imperatively, which overwrites the
            // parent-width binding PanelSeparator carries; without fillWidth
            // the rule draws at its 100px implicit width and reads as a stub.
            PanelSeparator {
                Layout.fillWidth: true
                foreground: row.fg
            }

            Text {
                Layout.fillWidth: true
                text: {
                    if (!row.match) return ""
                    var a = Model.fullOpponentLabel(row.match.opponents[0])
                    var b = Model.fullOpponentLabel(row.match.opponents[1])
                    return a + "  vs  " + b
                }
                color: row.fg
                wrapMode: Text.WordWrap
                font.family: row.bar ? row.bar.fontFamily : Style.font.family
                font.pixelSize: Style.font.bodySmall
            }

            ColumnLayout {
                Layout.fillWidth: true
                spacing: Style.space(3)

                Repeater {
                    model: row.facts

                    RowLayout {
                        id: fact

                        required property var modelData

                        Layout.fillWidth: true
                        spacing: Style.space(10)

                        Text {
                            Layout.preferredWidth: row.gutter
                            Layout.alignment: Qt.AlignTop
                            text: fact.modelData.label
                            color: row.fg
                            opacity: 0.4
                            font.family: row.bar ? row.bar.fontFamily : Style.font.family
                            font.pixelSize: Style.font.caption
                        }

                        Text {
                            Layout.fillWidth: true
                            text: fact.modelData.value
                            color: row.fg
                            opacity: 0.75
                            wrapMode: Text.WordWrap
                            font.family: row.bar ? row.bar.fontFamily : Style.font.family
                            font.pixelSize: Style.font.caption
                        }
                    }
                }
            }

            // Explains a blackout instead of leaving the user guessing.
            Text {
                Layout.fillWidth: true
                visible: row.masked || row.blacked
                text: row.masked ? Model.maskExplanation(row.match)
                    : "Result hidden — spoiler-free mode"
                color: row.bar ? row.bar.urgent : Color.accent
                opacity: 0.85
                wrapMode: Text.WordWrap
                font.family: row.bar ? row.bar.fontFamily : Style.font.family
                font.pixelSize: Style.font.caption
            }

            Flow {
                Layout.fillWidth: true
                spacing: Style.spacing.controlGap

                Button {
                    visible: !!(row.match && Model.preferredStream(row.match))
                    text: "󰐊 Watch"
                    fontSize: Style.font.caption
                    foreground: row.fg
                    fontFamily: row.bar ? row.bar.fontFamily : Style.font.family
                    horizontalPadding: Style.spacing.controlPaddingX
                    verticalPadding: Style.spacing.controlPaddingY
                    bordered: true
                    onClicked: row.watchRequested()
                }

                Button {
                    visible: Model.hasVod(row.match)
                    text: Model.isHighlightVod(row.match) ? "󰕧 Highlights" : "󰕧 VOD"
                    fontSize: Style.font.caption
                    foreground: row.fg
                    fontFamily: row.bar ? row.bar.fontFamily : Style.font.family
                    horizontalPadding: Style.spacing.controlPaddingX
                    verticalPadding: Style.spacing.controlPaddingY
                    bordered: true
                    onClicked: row.openUrlRequested(Model.vodUrl(row.match))
                }

                Button {
                    visible: row.blacked || row.masked
                    text: "Reveal"
                    fontSize: Style.font.caption
                    foreground: row.fg
                    fontFamily: row.bar ? row.bar.fontFamily : Style.font.family
                    horizontalPadding: Style.spacing.controlPaddingX
                    verticalPadding: Style.spacing.controlPaddingY
                    bordered: true
                    onClicked: row.revealRequested()
                }

                Button {
                    visible: !!(row.finished && row.match && row.match.followed && !row.match.watched)
                    text: "Mark watched"
                    fontSize: Style.font.caption
                    foreground: row.fg
                    fontFamily: row.bar ? row.bar.fontFamily : Style.font.family
                    horizontalPadding: Style.spacing.controlPaddingX
                    verticalPadding: Style.spacing.controlPaddingY
                    bordered: true
                    onClicked: row.watchedRequested()
                }

                Button {
                    visible: Model.tournamentUrl(row.match) !== ""
                    text: "Liquipedia"
                    fontSize: Style.font.caption
                    foreground: row.fg
                    fontFamily: row.bar ? row.bar.fontFamily : Style.font.family
                    horizontalPadding: Style.spacing.controlPaddingX
                    verticalPadding: Style.spacing.controlPaddingY
                    bordered: true
                    onClicked: row.openUrlRequested(Model.tournamentUrl(row.match))
                }

                Button {
                    visible: !!row.match && Model.opponentUrl(row.match.opponents[0]) !== ""
                    // Flow can wrap between buttons but not inside one, so a
                    // team with no short name is capped rather than allowed to
                    // grow a button wider than a narrow panel.
                    text: Model.truncate(Model.opponentName(row.match ? row.match.opponents[0] : null), 20)
                    fontSize: Style.font.caption
                    foreground: row.fg
                    fontFamily: row.bar ? row.bar.fontFamily : Style.font.family
                    horizontalPadding: Style.spacing.controlPaddingX
                    verticalPadding: Style.spacing.controlPaddingY
                    bordered: true
                    onClicked: row.openUrlRequested(Model.opponentUrl(row.match.opponents[0]))
                }

                Button {
                    visible: !!row.match && Model.opponentUrl(row.match.opponents[1]) !== ""
                    text: Model.truncate(Model.opponentName(row.match ? row.match.opponents[1] : null), 20)
                    fontSize: Style.font.caption
                    foreground: row.fg
                    fontFamily: row.bar ? row.bar.fontFamily : Style.font.family
                    horizontalPadding: Style.spacing.controlPaddingX
                    verticalPadding: Style.spacing.controlPaddingY
                    bordered: true
                    onClicked: row.openUrlRequested(Model.opponentUrl(row.match.opponents[1]))
                }
            }
        }
    }
}
