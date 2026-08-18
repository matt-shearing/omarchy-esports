import QtQuick
import QtQuick.Layouts
import "Model.js" as Model

// Team logo plus name, sized for the app's cards.
RowLayout {
    id: badge

    property var opponent: null
    property bool followed: false
    property bool mirrored: false

    signal clicked(string name)

    readonly property bool hidden: Model.isHiddenOpponent(opponent)

    spacing: 8
    layoutDirection: mirrored ? Qt.RightToLeft : Qt.LeftToRight

    Image {
        Layout.preferredWidth: 26
        Layout.preferredHeight: 26
        fillMode: Image.PreserveAspectFit
        asynchronous: true
        cache: true
        sourceSize.width: 52
        sourceSize.height: 52
        source: badge.hidden ? "" : Model.logoFor(badge.opponent, Theme.dark)
        visible: !badge.hidden && status === Image.Ready
    }

    // A withheld side reads as deliberately hidden, not as missing data.
    Rectangle {
        visible: badge.hidden
        Layout.preferredWidth: 26
        Layout.preferredHeight: 26
        radius: 13
        color: "transparent"
        border.width: 1
        border.color: Theme.alpha(Theme.foreground, 0.3)
        Text {
            anchors.centerIn: parent
            text: "?"
            color: Theme.muted
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontBody
        }
    }

    Text {
        Layout.fillWidth: true
        text: badge.hidden ? "Hidden" : Model.fullName(badge.opponent)
        color: Theme.foreground
        opacity: badge.followed ? 1.0 : 0.85
        font.family: Theme.fontFamily
        font.pixelSize: Theme.fontBody
        font.bold: badge.followed
        elide: Text.ElideRight
        horizontalAlignment: badge.mirrored ? Text.AlignRight : Text.AlignLeft

        HoverHandler {
            enabled: !badge.hidden
            cursorShape: Qt.PointingHandCursor
        }
        TapHandler {
            enabled: !badge.hidden
            onTapped: badge.clicked(Model.fullName(badge.opponent))
        }
    }
}
