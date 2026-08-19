import QtQuick
import QtQuick.Layouts
import "Model.js" as Model

// One team in the search results or the follow list.
Rectangle {
    id: row

    property var team: null
    property bool followed: false
    property bool compact: false

    signal toggleFollow
    signal inspect

    implicitHeight: compact ? 40 : 48
    radius: Theme.radius
    color: hover.hovered ? Theme.alpha(Theme.foreground, 0.08) : Theme.alpha(Theme.foreground, 0.04)

    Behavior on color { ColorAnimation { duration: 100 } }

    HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
    TapHandler { onTapped: row.inspect() }

    RowLayout {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.leftMargin: 12
        anchors.rightMargin: 8
        spacing: 10

        Image {
            Layout.preferredWidth: 24
            Layout.preferredHeight: 24
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            cache: true
            sourceSize.width: 48
            sourceSize.height: 48
            source: row.team ? Model.logoFor(row.team, Theme.dark) : ""
            visible: status === Image.Ready
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 0

            Text {
                Layout.fillWidth: true
                text: row.team ? row.team.name : ""
                color: Theme.foreground
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontBody
                font.bold: row.followed
                elide: Text.ElideRight
            }
            Text {
                Layout.fillWidth: true
                text: {
                    if (!row.team) return ""
                    var bits = []
                    if (row.team.short && row.team.short !== row.team.name) bits.push(row.team.short)
                    if (row.team.game) bits.push(row.team.game)
                    // Distinguish a team with fixtures from a directory entry,
                    // so an empty logo reads as "not playing" not "broken".
                    if (row.team.playing) bits.push("playing")
                    return bits.join(" · ")
                }
                color: Theme.muted
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontCaption
                elide: Text.ElideRight
            }
        }

        AppButton {
            text: row.followed ? "Following" : "Follow"
            accentuated: !row.followed
            subtle: row.followed
            onClicked: row.toggleFollow()
        }
    }
}
