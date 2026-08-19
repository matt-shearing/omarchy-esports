import QtQuick
import QtQuick.Layouts
import "Model.js" as Model

// Team logo and name. Falls back to a monogram, because most of the index
// comes from wiki team categories rather than fixtures and so has no artwork —
// and remote artwork can be temporarily unavailable besides.
RowLayout {
    id: badge

    property var opponent: null
    property bool followed: false
    property bool mirrored: false

    signal clicked(string name)

    readonly property bool hidden: Model.isHiddenOpponent(opponent)
    readonly property string logoSource: hidden ? "" : Model.logoFor(opponent, Theme.dark)

    spacing: 8
    layoutDirection: mirrored ? Qt.RightToLeft : Qt.LeftToRight

    Item {
        Layout.preferredWidth: 26
        Layout.preferredHeight: 26

        Image {
            id: art
            anchors.fill: parent
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            cache: true
            sourceSize.width: 52
            sourceSize.height: 52
            source: badge.logoSource
            visible: !badge.hidden && status === Image.Ready
        }

        // Shown when the side is withheld, when there is no artwork, and while
        // a remote image is loading or has failed.
        Rectangle {
            anchors.fill: parent
            visible: !art.visible
            radius: badge.hidden ? width / 2 : 5
            color: badge.hidden
                ? "transparent"
                : Qt.hsla(Model.monogramHue(badge.opponent) / 360, 0.45, Theme.dark ? 0.32 : 0.72, 1.0)
            border.width: badge.hidden ? 1 : 0
            border.color: Theme.alpha(Theme.foreground, 0.3)

            Text {
                anchors.centerIn: parent
                text: badge.hidden ? "?" : Model.initialsFor(badge.opponent)
                color: badge.hidden ? Theme.muted : (Theme.dark ? "#ffffff" : "#101010")
                font.family: Theme.fontFamily
                font.pixelSize: badge.hidden ? Theme.fontBody : Theme.fontCaption - 1
                font.bold: !badge.hidden
            }
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
