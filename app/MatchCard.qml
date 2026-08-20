import QtQuick
import QtQuick.Layouts
import "Model.js" as Model

// A match as a card: bigger artwork and an explicit action, in contrast to the
// bar panel's compact rows.
Rectangle {
    id: card

    property var match: null
    property var teams: []
    // Game catalog, for the artwork badge. Empty just means a text badge.
    property var games: []
    property double nowMs: 0

    signal watch
    signal reveal
    signal markWatched
    signal inspectTeam(string name)
    signal inspectTournament(string name)

    // Set by the VODs view, where clicking a tournament filters to it.
    property bool tournamentClickable: false

    readonly property bool live: match ? Model.isLive(match) : false
    readonly property bool finished: match ? Model.isFinished(match) : false
    readonly property bool blacked: match ? match.redacted === true : false
    readonly property bool hasVod: match ? (match.vod !== undefined && match.vod !== null) : false
    readonly property bool highlightsOnly: hasVod && match.vod.kind === "highlights"
    readonly property bool masked: match ? Model.isMasked(match) : false
    readonly property bool watched: match ? match.watched === true : false
    readonly property bool queueHead: match ? match.queueHead === true : false
    // Below this the fixed columns cannot all fit, and something has to give
    // before content starts escaping the card.
    readonly property bool compact: card.width > 0 && card.width < 900

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
            Layout.fillWidth: false
            Layout.preferredWidth: 66
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

        // Game badge: small and dim, a visual key rather than a label to read.
        Rectangle {
            Layout.alignment: Qt.AlignVCenter
            // Fixed, because this sits before every other column: letting it
            // size to "DOTA2" vs "CS2" shifted the logos, the names and the
            // "vs" divider by 12px on alternating rows.
            Layout.preferredWidth: 62
            Layout.preferredHeight: 18
            radius: 4
            visible: badgeText.text !== ""
            color: Theme.alpha(Theme.foreground, 0.07)

            RowLayout {
                id: badgeRow
                anchors.centerIn: parent
                spacing: 4

                // No artwork at this size. Several publisher marks are white
                // on transparent, and at 13px on a light theme they read as
                // Qt's broken-image glyph rather than as a logo. The chips in
                // settings render the same artwork at 40px where it works.
                Image {
                    source: ""
                    visible: false
                    Layout.preferredWidth: 13
                    Layout.preferredHeight: 13
                    fillMode: Image.PreserveAspectFit
                    smooth: true
                    mipmap: true
                    asynchronous: true
                    sourceSize.width: 26
                    sourceSize.height: 26
                }

                Text {
                    id: badgeText
                    text: Model.gameBadge(card.match)
                    color: Theme.muted
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontCaption - 1
                    font.bold: true
                    font.letterSpacing: 0.5
                }
            }
        }

        // Teams
        //
        // Both sides take an equal share rather than sizing to their names, so
        // "vs" lands on the same x in every row. Sizing to content let the
        // divider wander by a couple of hundred pixels down a list, which is
        // what stopped the column reading as a column at all.
        RowLayout {
            Layout.fillWidth: true
            Layout.minimumWidth: 220
            spacing: Theme.gap

            AppTeamBadge {
                Layout.fillWidth: true
                Layout.preferredWidth: 0
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
                horizontalAlignment: Text.AlignHCenter
                Layout.preferredWidth: 44
                Layout.alignment: Qt.AlignVCenter
            }

            AppTeamBadge {
                Layout.fillWidth: true
                Layout.preferredWidth: 0
                opponent: card.match ? card.match.opponents[1] : null
                followed: Model.isFollowedTeam(card.match ? card.match.opponents[1] : null, card.teams, card.match ? card.match.wiki : "")
                mirrored: true
                onClicked: function (name) { card.inspectTeam(name) }
            }
        }

        // Tournament
        //
        // One fixed width for every row: a column that sized between 130 and
        // 240 put each row's event name in a different place. It is dropped
        // rather than squeezed when the window is too narrow to hold it, since
        // a half-elided event name is worth less than the space it costs.
        ColumnLayout {
            visible: !card.compact
            // Exactly one column in this row may be flexible, and it has to be
            // the teams — they carry the content the row exists for. Every
            // other column being fillWidth by default is what starved them.
            Layout.fillWidth: false
            Layout.preferredWidth: 210
            Layout.maximumWidth: 210
            Layout.alignment: Qt.AlignVCenter
            spacing: 2

            Text {
                id: tournamentLabel
                Layout.fillWidth: true
                text: card.match ? card.match.tournament.name : ""
                color: (card.tournamentClickable && tournamentHover.hovered)
                    ? Theme.accent : Theme.foreground
                opacity: 0.8
                elide: Text.ElideRight
                horizontalAlignment: Text.AlignRight
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontCaption
                font.underline: card.tournamentClickable && tournamentHover.hovered

                HoverHandler {
                    id: tournamentHover
                    enabled: card.tournamentClickable
                    cursorShape: Qt.PointingHandCursor
                }
                TapHandler {
                    enabled: card.tournamentClickable
                    onTapped: card.inspectTournament(card.match.tournament.name)
                }
            }
            Text {
                Layout.fillWidth: true
                text: {
                    var parts = []
                    if (card.match && Model.bestOfLabel(card.match)) parts.push(Model.bestOfLabel(card.match))
                    if (card.match && card.match.game) parts.push(card.match.game)
                    if (card.highlightsOnly) parts.push("highlights only")
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

        // Actions
        //
        // A fixed box with the buttons pushed to its right edge. How many
        // appear varies by row — a followed finished match adds "Watched", a
        // masked one adds "Reveal" — so a cluster that sized to its contents
        // started somewhere different on every row, which read as the whole
        // list being ragged.
        RowLayout {
            // fillWidth defaults to TRUE for a Layout item, only false for a
            // plain one. Without pinning it off, this box ignored its declared
            // width and swallowed every row's surplus — several hundred pixels
            // of void sitting next to elided team names.
            Layout.fillWidth: false
            Layout.preferredWidth: 96
            Layout.maximumWidth: 96
            Layout.alignment: Qt.AlignVCenter
            spacing: 6

            Item { Layout.fillWidth: true }

            AppButton {
                visible: !!(card.finished && card.match && card.match.followed && !card.watched)
                text: "\u2713"
                iconOnly: true
                subtle: true
                tooltip: "Mark as watched"
                onClicked: card.markWatched()
            }

            AppButton {
                visible: Model.tournamentUrl(card.match) !== ""
                text: "\u2197"
                iconOnly: true
                subtle: true
                tooltip: "Open on Liquipedia"
                onClicked: Qt.openUrlExternally(Model.tournamentUrl(card.match))
            }

            // Revealing is a deliberate act, so it gets its own control rather
            // than happening as a side effect of opening a video.
            AppButton {
                visible: card.blacked || card.masked
                text: "\u25c9"
                iconOnly: true
                subtle: true
                tooltip: "Reveal the result"
                onClicked: card.reveal()
            }

            // The primary action last and in a fixed slot, so it lands on the
            // same x in every row. Right-aligning a cluster whose button count
            // varies by row moved it by over a hundred pixels between
            // neighbours, and the eye had to hunt for it each time.
            Item {
                Layout.preferredWidth: 26
                Layout.preferredHeight: 26
                Layout.alignment: Qt.AlignVCenter

                AppButton {
                    anchors.right: parent.right
                    anchors.verticalCenter: parent.verticalCenter
                    visible: card.live || (!card.finished && card.match && Model.preferredStream(card.match) !== null) || card.hasVod
                    text: "\u25b6"
                    iconOnly: true
                    accentuated: true
                    // What it opens is spelled out on the meta line rather
                    // than in the button, so the control stays one width.
                    tooltip: card.hasVod
                        ? (card.highlightsOnly ? "Watch highlights" : "Watch the VOD")
                        : (card.live ? "Watch live" : "Open the stream")
                    onClicked: card.watch()
                }
            }
        }
    }
}
