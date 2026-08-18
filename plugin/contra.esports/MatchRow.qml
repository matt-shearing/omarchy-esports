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

    implicitHeight: body.implicitHeight + Style.space(14)
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
                Layout.preferredWidth: Style.space(52)
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
                    followed: Model.isFollowedTeam(row.match ? row.match.opponents[0] : null, row.teams)
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
                    followed: Model.isFollowedTeam(row.match ? row.match.opponents[1] : null, row.teams)
                }

                Item { Layout.fillWidth: true }
            }

            ColumnLayout {
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
                Text {
                    Layout.fillWidth: true
                    text: row.match ? Model.bestOfLabel(row.match) : ""
                    color: row.fg
                    opacity: 0.45
                    horizontalAlignment: Text.AlignRight
                    font.family: row.bar ? row.bar.fontFamily : Style.font.family
                    font.pixelSize: Style.font.caption
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
            Layout.topMargin: Style.space(4)
            visible: row.expanded
            spacing: Style.space(6)

            PanelSeparator { foreground: row.fg }

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

            Text {
                Layout.fillWidth: true
                text: {
                    if (!row.match) return ""
                    var bits = []
                    bits.push(Model.dayLabel(row.match, row.nowMs) + " " + Model.clockTime(row.match))
                    if (Model.bestOfLabel(row.match)) bits.push(Model.bestOfLabel(row.match))
                    if (row.match.game) bits.push(row.match.game)
                    bits.push(row.match.tournament.name)
                    return bits.join("  ·  ")
                }
                color: row.fg
                opacity: 0.6
                wrapMode: Text.WordWrap
                font.family: row.bar ? row.bar.fontFamily : Style.font.family
                font.pixelSize: Style.font.caption
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
                spacing: Style.space(6)

                Button {
                    visible: row.match && Model.preferredStream(row.match) !== null
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
                    visible: row.finished && row.match && row.match.followed && !row.match.watched
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
                    visible: row.match && Model.opponentUrl(row.match.opponents[0]) !== ""
                    text: Model.opponentName(row.match ? row.match.opponents[0] : null)
                    fontSize: Style.font.caption
                    foreground: row.fg
                    fontFamily: row.bar ? row.bar.fontFamily : Style.font.family
                    horizontalPadding: Style.spacing.controlPaddingX
                    verticalPadding: Style.spacing.controlPaddingY
                    bordered: true
                    onClicked: row.openUrlRequested(Model.opponentUrl(row.match.opponents[0]))
                }

                Button {
                    visible: row.match && Model.opponentUrl(row.match.opponents[1]) !== ""
                    text: Model.opponentName(row.match ? row.match.opponents[1] : null)
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
