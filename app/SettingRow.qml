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
        // Capped so the help paragraph gets the same measure on every row.
        // Letting it take whatever the controls left over meant one
        // description wrapped 150px later than its neighbour.
        Layout.maximumWidth: row.width * 0.62
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

    // A fixed box with its controls pinned right, so a lone toggle starts on
    // the same x as a four-way choice instead of floating 157px off to the
    // side of it.
    Item {
        Layout.preferredWidth: 264
        Layout.preferredHeight: holder.implicitHeight
        Layout.alignment: Qt.AlignRight | Qt.AlignVCenter

        RowLayout {
            id: holder
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            spacing: 6
        }
    }
}
