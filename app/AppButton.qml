import QtQuick

// A small button matching omarchy's control chrome, since this app cannot
// import the shell's Ui components.
Rectangle {
    id: button

    property string text: ""
    property bool accentuated: false
    property bool subtle: false

    signal clicked

    implicitWidth: label.implicitWidth + 22
    implicitHeight: 26
    radius: Theme.radius - 2

    color: {
        if (mouse.pressed) return Theme.alpha(Theme.foreground, 0.16)
        if (hover.hovered) return Theme.alpha(Theme.foreground, 0.10)
        return button.accentuated ? Theme.alpha(Theme.accent, 0.18) : Theme.alpha(Theme.foreground, 0.05)
    }
    border.width: 1
    border.color: button.accentuated ? Theme.accent : Theme.alpha(Theme.foreground, button.subtle ? 0.18 : 0.35)

    Behavior on color { ColorAnimation { duration: 100 } }

    HoverHandler { id: hover; cursorShape: Qt.PointingHandCursor }
    TapHandler { id: mouse; onTapped: button.clicked() }

    Text {
        id: label
        anchors.centerIn: parent
        text: button.text
        color: button.subtle ? Theme.muted : Theme.foreground
        font.family: Theme.fontFamily
        font.pixelSize: Theme.fontCaption
    }
}
