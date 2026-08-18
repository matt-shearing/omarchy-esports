import QtQuick
import QtQuick.Layouts
import "Model.js" as Model

// Team logo plus name, sized for the app's cards.
RowLayout {
    id: badge

    property var opponent: null
    property bool followed: false
    property bool mirrored: false

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
        source: Model.logoFor(badge.opponent, Theme.dark)
        visible: status === Image.Ready
    }

    Text {
        Layout.fillWidth: true
        text: Model.fullName(badge.opponent)
        color: Theme.foreground
        opacity: badge.followed ? 1.0 : 0.85
        font.family: Theme.fontFamily
        font.pixelSize: Theme.fontBody
        font.bold: badge.followed
        elide: Text.ElideRight
        horizontalAlignment: badge.mirrored ? Text.AlignRight : Text.AlignLeft
    }
}
