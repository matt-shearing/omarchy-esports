import QtQuick
import QtQuick.Layouts

// A selectable game tile: short badge, full name, and how busy the scene was
// when the game was verified. Used by both the setup wizard and settings, so
// turning a game on looks the same in both places.
Rectangle {
    id: chip

    property var wiki: null
    property bool enabled_: false

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

        // The badge doubles as the visual key used elsewhere in the app.
        Rectangle {
            Layout.preferredWidth: 40
            Layout.preferredHeight: 24
            radius: 4
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
