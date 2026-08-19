import QtQuick
import QtQuick.Layouts

// One labelled setting: a description on the left, controls on the right.
RowLayout {
    id: row

    property string label: ""
    property string help: ""
    default property alias content: holder.data

    spacing: 16
    Layout.fillWidth: true

    ColumnLayout {
        Layout.fillWidth: true
        spacing: 1

        Text {
            text: row.label
            color: Theme.foreground
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontBody
        }
        Text {
            visible: row.help !== ""
            Layout.fillWidth: true
            text: row.help
            color: Theme.muted
            wrapMode: Text.WordWrap
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontCaption
        }
    }

    RowLayout {
        id: holder
        Layout.alignment: Qt.AlignRight | Qt.AlignVCenter
        spacing: 6
    }
}
