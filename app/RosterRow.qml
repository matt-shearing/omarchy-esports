import QtQuick
import QtQuick.Layouts
import "Model.js" as Model

// One player on a team's active roster.
//
// The handle is what players are known by, so it leads; the real name is
// secondary. Position is the wiki's own notion of role and varies by game — a
// lane number in Dota 2, a role name elsewhere — so it is rendered as given
// rather than interpreted.
Rectangle {
    id: row

    property var player: null
    // Liquipedia page for the player, opened on click.
    readonly property string page: player && player.page ? String(player.page) : ""

    implicitHeight: line.implicitHeight + 14
    radius: Theme.radius - 3
    color: hover.hovered ? Theme.alpha(Theme.foreground, 0.06) : "transparent"

    HoverHandler { id: hover }

    TapHandler {
        enabled: row.page !== ""
        onTapped: Qt.openUrlExternally(Model.absoluteUrl(row.page))
    }

    RowLayout {
        id: line
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.leftMargin: 8
        anchors.rightMargin: 8
        anchors.verticalCenter: parent.verticalCenter
        spacing: 10

        Text {
            text: row.player && row.player.captain ? "★" : ""
            visible: text !== ""
            color: Theme.accent
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontCaption
            // Reserve the width either way so handles stay aligned.
            Layout.preferredWidth: 10
        }
        Item { visible: !(row.player && row.player.captain); Layout.preferredWidth: 10 }

        Text {
            text: row.player ? String(row.player.id || "") : ""
            color: Theme.foreground
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontBody
            font.bold: true
            Layout.preferredWidth: 130
            elide: Text.ElideRight
        }

        Text {
            text: row.player ? String(row.player.name || "") : ""
            color: Theme.muted
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontCaption
            Layout.fillWidth: true
            elide: Text.ElideRight
        }

        Text {
            text: row.player && row.player.country ? String(row.player.country) : ""
            color: Theme.muted
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontCaption
            horizontalAlignment: Text.AlignRight
            Layout.preferredWidth: 110
            elide: Text.ElideRight
        }

        Text {
            // A Dota lane is a bare number and needs the label to make sense;
            // a role name like "Coach" reads worse with one.
            text: {
                if (!row.player || !row.player.position) return ""
                var p = String(row.player.position)
                return /^[0-9]+$/.test(p) ? "pos " + p : p
            }
            color: Theme.muted
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontCaption
            horizontalAlignment: Text.AlignRight
            Layout.preferredWidth: 64
        }

        Text {
            text: row.player && row.player.joined ? String(row.player.joined) : ""
            color: Theme.alpha(Theme.muted, 0.8)
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontCaption
            horizontalAlignment: Text.AlignRight
            Layout.preferredWidth: 90
        }
    }
}
