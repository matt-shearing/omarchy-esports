import QtQuick
import QtQuick.Layouts
import qs.Commons
import qs.Ui
import "Model.js" as Model

// One match in the dropdown: time on the left, the fixture in the middle,
// and whatever action is available on the right.
Rectangle {
    id: row

    property var match: null
    property QtObject bar: null
    property bool darkTheme: true
    property bool hasCursor: false
    property double nowMs: 0
    property var teams: []

    signal activated
    signal revealRequested

    readonly property color fg: bar ? bar.foreground : Color.popups.text
    readonly property bool live: match ? Model.isLive(match) : false
    readonly property bool finished: match ? Model.isFinished(match) : false
    readonly property bool blacked: match ? (match.redacted === true) : false

    implicitHeight: content.implicitHeight + Style.space(14)
    radius: Style.cornerRadius
    color: hasCursor || hover.hovered ? Style.hoverFill : "transparent"

    Behavior on color {
        enabled: row.bar ? row.bar.foregroundAnimationEnabled : false
        ColorAnimation { duration: 120 }
    }

    HoverHandler { id: hover }

    TapHandler {
        acceptedButtons: Qt.LeftButton | Qt.RightButton
        onTapped: function (point, button) {
            if (button === Qt.RightButton && row.blacked) row.revealRequested()
            else row.activated()
        }
    }

    RowLayout {
        id: content
        // Horizontal anchors only: the row's implicitHeight is derived from
        // this layout, so filling the parent would create a binding loop.
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.leftMargin: Style.space(10)
        anchors.rightMargin: Style.space(10)
        spacing: Style.space(10)

        // Time / status column. Fixed width so rows align into a clean
        // gutter regardless of how long the countdown string is.
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
                opacity: row.live ? 1.0 : 0.9
            }

            Text {
                visible: !row.live && !row.finished && text !== ""
                text: Model.countdown(row.match, row.nowMs)
                color: row.fg
                opacity: 0.55
                font.family: row.bar ? row.bar.fontFamily : Style.font.family
                font.pixelSize: Style.font.caption
            }

            Text {
                visible: row.finished
                text: row.match && row.match.vod ? "VOD" : "done"
                color: row.fg
                opacity: 0.55
                font.family: row.bar ? row.bar.fontFamily : Style.font.family
                font.pixelSize: Style.font.caption
            }
        }

        // Team logos and names.
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

        // Tournament and format.
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

        // Action affordance: a stream glyph when watchable now, a blackout
        // marker when the result is being withheld.
        Text {
            Layout.alignment: Qt.AlignVCenter
            text: {
                if (row.blacked) return "󰈉"
                if (row.match && row.match.vod) return "󰕧"
                if (row.match && Model.preferredStream(row.match)) return "󰐊"
                return ""
            }
            color: row.blacked ? row.fg : (row.live ? (row.bar ? row.bar.urgent : Color.accent) : row.fg)
            opacity: row.blacked ? 0.4 : 0.8
            font.family: row.bar ? row.bar.fontFamily : Style.font.family
            font.pixelSize: Style.font.icon
        }
    }
}
