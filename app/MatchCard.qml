import QtQuick
import QtQuick.Layouts
import "Model.js" as Model

// A match as a card: bigger artwork and an explicit action, in contrast to the
// bar panel's compact rows.
Rectangle {
    id: card

    property var match: null
    property var teams: []
    property double nowMs: 0

    signal watch
    signal reveal
    signal markWatched
    signal inspectTeam(string name)

    readonly property bool live: match ? Model.isLive(match) : false
    readonly property bool finished: match ? Model.isFinished(match) : false
    readonly property bool blacked: match ? match.redacted === true : false
    readonly property bool hasVod: match ? (match.vod !== undefined && match.vod !== null) : false
    readonly property bool highlightsOnly: hasVod && match.vod.kind === "highlights"
    readonly property bool masked: match ? Model.isMasked(match) : false
    readonly property bool watched: match ? match.watched === true : false
    readonly property bool queueHead: match ? match.queueHead === true : false

    implicitHeight: layout.implicitHeight + Theme.gap * 2
    radius: Theme.radius
    color: hover.hovered ? Theme.alpha(Theme.foreground, 0.06) : Theme.alpha(Theme.foreground, 0.03)
    border.width: (live || queueHead) ? 1 : 0
    border.color: queueHead && !live ? Theme.success : Theme.accent
    opacity: card.watched && !card.queueHead ? 0.55 : 1.0

    Behavior on color { ColorAnimation { duration: 120 } }

    HoverHandler { id: hover }

    RowLayout {
        id: layout
        // Deliberately not anchors.fill: the card takes its height from this
        // layout, so filling the card would make each depend on the other and
        // the resulting binding loop collapses every column onto the same x.
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.leftMargin: Theme.gap
        anchors.rightMargin: Theme.gap
        spacing: Theme.gap * 1.4

        // Time column
        ColumnLayout {
            Layout.preferredWidth: 74
            Layout.alignment: Qt.AlignVCenter
            spacing: 2

            Text {
                text: card.live ? "LIVE" : (card.finished ? (card.hasVod ? "VOD" : "ended") : Model.clockTime(card.match))
                color: card.live ? Theme.accent : Theme.foreground
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontSubtitle
                font.bold: card.live
            }
            Text {
                visible: !card.live && !card.finished
                text: Model.countdown(card.match, card.nowMs)
                color: Theme.muted
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontCaption
            }
            Text {
                visible: card.finished
                text: card.match ? Model.dayLabel(card.match, card.nowMs) : ""
                color: Theme.muted
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontCaption
            }
        }

        // Teams
        RowLayout {
            Layout.fillWidth: true
            Layout.minimumWidth: 220
            spacing: Theme.gap

            AppTeamBadge {
                Layout.fillWidth: true
                opponent: card.match ? card.match.opponents[0] : null
                followed: Model.isFollowedTeam(card.match ? card.match.opponents[0] : null, card.teams, card.match ? card.match.wiki : "")
                onClicked: function (name) { card.inspectTeam(name) }
            }

            Text {
                text: Model.scoreLabel(card.match) !== "" ? Model.scoreLabel(card.match) : "vs"
                color: Model.scoreLabel(card.match) !== "" ? Theme.foreground : Theme.muted
                font.family: Theme.fontFamily
                font.pixelSize: Model.scoreLabel(card.match) !== "" ? Theme.fontSubtitle : Theme.fontCaption
                font.bold: Model.scoreLabel(card.match) !== ""
                Layout.alignment: Qt.AlignVCenter
            }

            AppTeamBadge {
                Layout.fillWidth: true
                opponent: card.match ? card.match.opponents[1] : null
                followed: Model.isFollowedTeam(card.match ? card.match.opponents[1] : null, card.teams, card.match ? card.match.wiki : "")
                mirrored: true
                onClicked: function (name) { card.inspectTeam(name) }
            }
        }

        // Tournament
        ColumnLayout {
            Layout.preferredWidth: 190
            Layout.minimumWidth: 130
            Layout.maximumWidth: 240
            Layout.alignment: Qt.AlignVCenter
            spacing: 2

            Text {
                Layout.fillWidth: true
                text: card.match ? card.match.tournament.name : ""
                color: Theme.foreground
                opacity: 0.8
                elide: Text.ElideRight
                horizontalAlignment: Text.AlignRight
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontCaption
            }
            Text {
                Layout.fillWidth: true
                text: {
                    var parts = []
                    if (card.match && Model.bestOfLabel(card.match)) parts.push(Model.bestOfLabel(card.match))
                    if (card.match && card.match.game) parts.push(card.match.game)
                    if (card.masked) parts.push("opponent hidden")
                    return parts.join(" · ")
                }
                color: card.masked ? Theme.accent : Theme.muted
                horizontalAlignment: Text.AlignRight
                elide: Text.ElideRight
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontCaption
            }
        }

        // Action
        RowLayout {
            Layout.minimumWidth: 96
            Layout.alignment: Qt.AlignVCenter
            spacing: 6

            AppButton {
                visible: card.live || (!card.finished && card.match && Model.preferredStream(card.match) !== null) || card.hasVod
                text: card.hasVod ? (card.highlightsOnly ? "Highlights" : "Watch VOD") : "Watch"
                accentuated: card.live || card.queueHead
                onClicked: card.watch()
            }

            AppButton {
                visible: card.finished && card.match && card.match.followed && !card.watched
                text: "Watched"
                subtle: true
                onClicked: card.markWatched()
            }

            AppButton {
                visible: Model.tournamentUrl(card.match) !== ""
                text: "Liquipedia"
                subtle: true
                onClicked: Qt.openUrlExternally(Model.tournamentUrl(card.match))
            }

            // Revealing is a deliberate act, so it gets its own control rather
            // than happening as a side effect of opening a video.
            AppButton {
                visible: card.blacked || card.masked
                text: "Reveal"
                subtle: true
                onClicked: card.reveal()
            }
        }
    }
}
