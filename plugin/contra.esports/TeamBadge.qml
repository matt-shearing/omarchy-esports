import QtQuick
import QtQuick.Layouts
import qs.Commons
import "Model.js" as Model

// A team's logo and short name. Liquipedia publishes light and dark artwork
// variants, so the badge follows the active omarchy theme rather than baking
// in one background assumption.
RowLayout {
    id: badge

    property var opponent: null
    property bool darkTheme: true
    property QtObject bar: null
    property bool followed: false

    spacing: Style.space(4)

    readonly property string logoSource: Model.logoFor(opponent, darkTheme)

    Image {
        Layout.preferredWidth: Style.space(16)
        Layout.preferredHeight: Style.space(16)
        Layout.alignment: Qt.AlignVCenter
        fillMode: Image.PreserveAspectFit
        // Remote artwork must never block the UI thread, and a missing logo
        // should degrade to just the name rather than an error box.
        asynchronous: true
        cache: true
        source: badge.logoSource
        visible: status === Image.Ready
        sourceSize.width: Style.space(32)
        sourceSize.height: Style.space(32)
    }

    Text {
        Layout.alignment: Qt.AlignVCenter
        text: Model.opponentName(badge.opponent)
        color: badge.bar ? badge.bar.foreground : Color.popups.text
        font.family: badge.bar ? badge.bar.fontFamily : Style.font.family
        font.pixelSize: Style.font.bodySmall
        font.bold: badge.followed
        elide: Text.ElideRight
        maximumLineCount: 1
    }
}
