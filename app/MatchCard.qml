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

    readonly property bool live: match ? Model.isLive(match) : false
    readonly property bool finished: match ? Model.isFinished(match) : false
    readonly property bool blacked: match ? match.redacted === true : false
    readonly property bool hasVod: match ? (match.vod !== undefined && match.vod !== null) : false
    readonly property bool highlightsOnly: hasVod && match.vod.kind === "highlights"

    implicitHeight: layout.implicitHeight + Theme.gap * 2
    radius: Theme.radius
    color: hover.hovered ? Theme.alpha(Theme.foreground, 0.06) : Theme.alpha(Theme.foreground, 0.03)
    border.width: live ? 1 : 0
    border.color: Theme.accent

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
                followed: Model.isFollowedTeam(card.match ? card.match.opponents[0] : null, card.teams)
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
                followed: Model.isFollowedTeam(card.match ? card.match.opponents[1] : null, card.teams)
                mirrored: true
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
                    return parts.join(" · ")
                }
                color: Theme.muted
                horizontalAlignment: Text.AlignRight
                elide: Text.ElideRight
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontCaption
            }
        }

        // Action
        ColumnLayout {
            Layout.minimumWidth: 96
            Layout.alignment: Qt.AlignVCenter
            spacing: 4

            AppButton {
                Layout.alignment: Qt.AlignRight
                visible: card.live || (!card.finished && card.match && Model.preferredStream(card.match) !== null) || card.hasVod
                text: card.hasVod ? (card.highlightsOnly ? "Highlights" : "Watch VOD") : "Watch"
                accentuated: card.live
                onClicked: card.watch()
            }

            // Revealing is a deliberate act, so it gets its own control rather
            // than happening as a side effect of opening a video.
            AppButton {
                Layout.alignment: Qt.AlignRight
                visible: card.blacked
                text: "Reveal result"
                subtle: true
                onClicked: card.reveal()
            }
        }
    }
}
