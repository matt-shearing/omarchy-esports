import QtQuick
import QtQuick.Layouts
import qs.Commons
import "Model.js" as Model

// Team logo and short name. Falls back to a monogram: most teams in the index
// come from wiki team categories rather than fixtures and have no artwork, and
// remote artwork can be temporarily unavailable besides. A blank square reads
// as broken; two letters read as deliberate.
RowLayout {
    id: badge

    property var opponent: null
    property bool darkTheme: true
    property QtObject bar: null
    property bool followed: false

    readonly property bool hidden: Model.isHiddenOpponent(opponent)
    readonly property string logoSource: hidden ? "" : Model.logoFor(opponent, darkTheme)
    readonly property color fg: bar ? bar.foreground : Color.popups.text

    spacing: Style.space(4)

    Item {
        Layout.preferredWidth: Style.space(16)
        Layout.preferredHeight: Style.space(16)
        Layout.alignment: Qt.AlignVCenter

        Image {
            id: art
            anchors.fill: parent
            fillMode: Image.PreserveAspectFit
            asynchronous: true
            cache: true
            sourceSize.width: Style.space(32)
            sourceSize.height: Style.space(32)
            source: badge.logoSource
            visible: !badge.hidden && status === Image.Ready
        }

        Rectangle {
            anchors.fill: parent
            visible: !art.visible
            radius: badge.hidden ? width / 2 : Style.space(3)
            color: badge.hidden ? "transparent" : Qt.rgba(badge.fg.r, badge.fg.g, badge.fg.b, 0.14)
            border.width: badge.hidden ? 1 : 0
            border.color: badge.fg
            opacity: badge.hidden ? 0.35 : 1.0

            Text {
                textFormat: Text.PlainText
                anchors.centerIn: parent
                text: badge.hidden ? "?" : Model.initialsFor(badge.opponent)
                color: badge.fg
                opacity: badge.hidden ? 1.0 : 0.75
                font.family: badge.bar ? badge.bar.fontFamily : Style.font.family
                font.pixelSize: Style.font.caption - 1
                font.bold: !badge.hidden
            }
        }
    }

    Text {
        textFormat: Text.PlainText
        Layout.alignment: Qt.AlignVCenter
        text: Model.opponentLabel(badge.opponent)
        color: badge.fg
        font.family: badge.bar ? badge.bar.fontFamily : Style.font.family
        font.pixelSize: Style.font.bodySmall
        font.bold: badge.followed
        opacity: badge.hidden ? 0.45 : 1.0
        elide: Text.ElideRight
        maximumLineCount: 1
    }
}
