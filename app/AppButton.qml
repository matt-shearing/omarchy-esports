import QtQuick
import QtQuick.Controls

// A small button matching omarchy's control chrome, since this app cannot
// import the shell's Ui components.
Rectangle {
    id: button

    property string text: ""
    property bool accentuated: false
    property bool subtle: false
    // Icon-only: a square glyph for actions that are incidental to the row.
    // A row can carry four buttons, and spelling out every one of them cost
    // more width than the team names it was crowding out.
    property bool iconOnly: false
    // What the glyph means, since a glyph alone does not say.
    property string tooltip: ""

    signal clicked

    // Secondary actions are physically smaller, not merely dimmer. A row can
    // carry four of these, and at equal size the incidental ones ("Liquipedia",
    // "Reveal") carried the same visual weight as the thing the row is for.
    implicitWidth: button.iconOnly ? implicitHeight
        : label.implicitWidth + (button.subtle ? 16 : 22)
    implicitHeight: (button.subtle || button.iconOnly) ? 22 : 26
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

    ToolTip.visible: button.tooltip !== "" && hover.hovered
    ToolTip.text: button.tooltip
    ToolTip.delay: 400

    Text {
        id: label
        anchors.centerIn: parent
        text: button.text
        color: button.subtle ? Theme.muted : Theme.foreground
        font.family: Theme.fontFamily
        font.pixelSize: button.iconOnly ? Theme.fontBody
            : (button.subtle ? Theme.fontCaption - 1 : Theme.fontCaption)
    }
}
