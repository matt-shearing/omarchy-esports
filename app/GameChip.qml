import QtQuick
import QtQuick.Layouts

// A selectable game tile: artwork or short badge, full name, and whether the
// game is on. Used by both the setup wizard and settings, so turning a game on
// looks the same in both places.
Rectangle {
    id: chip

    property var wiki: null
    property bool enabled_: false
    // Local artwork path, or "" to fall back to the short text badge.
    property string icon: ""

    signal toggled

    implicitWidth: 168
    implicitHeight: 56
    radius: Theme.radius

    color: enabled_ ? Theme.alpha(Theme.accent, 0.14)
        : (hover.hovered ? Theme.alpha(Theme.foreground, 0.08) : Theme.alpha(Theme.foreground, 0.04))
    border.width: 1
    border.color: enabled_ ? Theme.accent : Theme.alpha(Theme.foreground, 0.15)

    Behavior on color { ColorAnimation { duration: 110 } }
    Behavior on border.color { ColorAnimation { duration: 110 } }

    HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
    TapHandler { onTapped: chip.toggled() }

    RowLayout {
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.verticalCenter: parent.verticalCenter
        anchors.leftMargin: 10
        anchors.rightMargin: 10
        spacing: 9

        // Artwork where we have it, the short badge otherwise. The text badge
        // is not merely a placeholder: a game with no curated source, or one
        // whose artwork has not downloaded yet, stays in this state for good,
        // so it has to look deliberate rather than broken.
        Item {
            Layout.preferredWidth: 40
            Layout.preferredHeight: 30

            readonly property bool artworkShown: art.status === Image.Ready

            Image {
                id: art
                anchors.fill: parent
                source: chip.icon
                visible: status === Image.Ready
                fillMode: Image.PreserveAspectFit
                smooth: true
                mipmap: true
                asynchronous: true
                // Bound the decode: some publisher icons are 1024px square and
                // this draws at 40.
                sourceSize.width: 80
                sourceSize.height: 80
                opacity: chip.enabled_ ? 1.0 : 0.55
                Behavior on opacity { NumberAnimation { duration: 110 } }
            }

            Rectangle {
                anchors.centerIn: parent
                width: 40
                height: 24
                radius: 4
                visible: !parent.artworkShown
                color: chip.enabled_ ? Theme.accent : Theme.alpha(Theme.foreground, 0.12)

                Text {
                    anchors.centerIn: parent
                    text: chip.wiki ? (chip.wiki.short || "?") : "?"
                    color: chip.enabled_ ? Theme.background : Theme.foreground
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontCaption - 1
                    font.bold: true
                }
            }
        }

        ColumnLayout {
            Layout.fillWidth: true
            spacing: 0

            Text {
                Layout.fillWidth: true
                text: chip.wiki ? chip.wiki.game : ""
                color: Theme.foreground
                elide: Text.ElideRight
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontCaption
                font.bold: chip.enabled_
            }
            Text {
                Layout.fillWidth: true
                text: chip.enabled_ ? "on" : "off"
                color: chip.enabled_ ? Theme.accent : Theme.muted
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontCaption - 1
            }
        }
    }
}
