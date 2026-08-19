import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "Model.js" as Model

// First-run setup.
//
// Shown instead of the schedule until the user has been through it, because an
// empty follow list and two default games are a poor first impression of what
// the tool does. Every step writes through the CLI immediately rather than
// batching at the end, so quitting halfway still leaves a working config.
Item {
    id: wizard

    property var config: null
    property var teamIndex: []
    property var followed: []
    property int step: 0

    signal apply(string key, string value)
    signal applyWiki(string slug, bool on)
    signal followTeam(string name, string wiki)
    signal finished

    readonly property int stepCount: 3

    function isEnabled(slug) {
        if (!config) return false
        for (var i = 0; i < config.wikis.length; i++) {
            if (config.wikis[i].slug === slug) return config.wikis[i].enabled === true
        }
        return false
    }

    function enabledCount() {
        if (!config) return 0
        var n = 0
        for (var i = 0; i < config.wikis.length; i++) if (config.wikis[i].enabled) n++
        return n
    }

    ColumnLayout {
        anchors.fill: parent
        anchors.margins: 8
        spacing: 16

        // ---- header ----
        ColumnLayout {
            Layout.fillWidth: true
            spacing: 4

            Text {
                text: "Welcome"
                color: Theme.foreground
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontDisplay
                font.bold: true
            }
            Text {
                Layout.fillWidth: true
                text: "A spoiler-free esports schedule. Three quick questions and it will stay out of your way."
                color: Theme.muted
                wrapMode: Text.WordWrap
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontBody
            }
        }

        // ---- progress ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 6

            Repeater {
                model: ["Games", "Teams", "Spoilers"]
                delegate: RowLayout {
                    required property var modelData
                    required property int index
                    spacing: 6

                    Rectangle {
                        Layout.preferredWidth: 22
                        Layout.preferredHeight: 22
                        radius: 11
                        color: index <= wizard.step ? Theme.accent : Theme.alpha(Theme.foreground, 0.1)
                        Text {
                            anchors.centerIn: parent
                            text: index < wizard.step ? "✓" : String(index + 1)
                            color: index <= wizard.step ? Theme.background : Theme.muted
                            font.family: Theme.fontFamily
                            font.pixelSize: Theme.fontCaption
                            font.bold: true
                        }
                    }
                    Text {
                        text: modelData
                        color: index === wizard.step ? Theme.foreground : Theme.muted
                        font.family: Theme.fontFamily
                        font.pixelSize: Theme.fontCaption
                        font.bold: index === wizard.step
                    }
                    Rectangle {
                        visible: index < 2
                        Layout.preferredWidth: 20
                        Layout.preferredHeight: 1
                        color: Theme.alpha(Theme.foreground, 0.15)
                    }
                }
            }
            Item { Layout.fillWidth: true }
        }

        Rectangle { Layout.fillWidth: true; implicitHeight: 1; color: Theme.alpha(Theme.foreground, 0.12) }

        // ---- steps ----
        StackLayout {
            Layout.fillWidth: true
            Layout.fillHeight: true
            currentIndex: wizard.step

            // 0 — games
            ColumnLayout {
                spacing: 10

                Text {
                    Layout.fillWidth: true
                    text: "Which games do you follow?"
                    color: Theme.foreground
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontTitle
                }
                Text {
                    Layout.fillWidth: true
                    text: "Counter-Strike and Dota 2 are on to start with. Each game adds about 30 seconds to a refresh, so pick the ones you actually watch — you can change this any time in Settings."
                    color: Theme.muted
                    wrapMode: Text.WordWrap
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontCaption
                }

                ScrollView {
                    id: gamesScroll
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    clip: true
                    ScrollBar.horizontal.policy: ScrollBar.AlwaysOff

                    Flow {
                        // Bind to the ScrollView's available width, not the
                        // content item's, or the grid overflows to the right.
                        width: gamesScroll.availableWidth
                        spacing: 8

                        Repeater {
                            model: wizard.config ? wizard.config.wikis : []
                            delegate: GameChip {
                                required property var modelData
                                wiki: modelData
                                enabled_: modelData.enabled === true
                                onToggled: wizard.applyWiki(modelData.slug, !(modelData.enabled === true))
                            }
                        }
                    }
                }
            }

            // 1 — teams
            ColumnLayout {
                spacing: 10

                Text {
                    Layout.fillWidth: true
                    text: "Any teams to follow?"
                    color: Theme.foreground
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontTitle
                }
                Text {
                    Layout.fillWidth: true
                    text: "Followed teams get notifications, a countdown on the bar, and a catch-up queue of matches you have not watched. You can skip this and add them later."
                    color: Theme.muted
                    wrapMode: Text.WordWrap
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontCaption
                }

                TextField {
                    id: search
                    Layout.fillWidth: true
                    placeholderText: wizard.teamIndex.length > 0
                        ? "Search " + wizard.teamIndex.length + " teams — try navi, spirit, g2"
                        : "Waiting for the first refresh to build the team list…"
                    enabled: wizard.teamIndex.length > 0
                    color: Theme.foreground
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontBody
                    background: Rectangle {
                        color: Theme.alpha(Theme.foreground, 0.05)
                        radius: Theme.radius - 2
                        border.width: 1
                        border.color: search.activeFocus ? Theme.accent : Theme.alpha(Theme.foreground, 0.2)
                    }
                }

                ListView {
                    Layout.fillWidth: true
                    Layout.fillHeight: true
                    clip: true
                    spacing: 6
                    boundsBehavior: Flickable.StopAtBounds
                    ScrollBar.vertical: ScrollBar { policy: ScrollBar.AsNeeded }

                    model: Model.searchTeams(wizard.teamIndex, search.text, { minChars: 2, limit: 14 })

                    delegate: TeamResultRow {
                        required property var modelData
                        width: ListView.view.width
                        team: modelData
                        compact: true
                        followed: {
                            for (var i = 0; i < wizard.followed.length; i++) {
                                var t = wizard.followed[i]
                                var n = (t && t.name !== undefined) ? t.name : t
                                if (String(n).toLowerCase() === String(modelData.name).toLowerCase()) return true
                            }
                            return false
                        }
                        onToggleFollow: wizard.followTeam(modelData.name, modelData.wiki)
                        onInspect: wizard.followTeam(modelData.name, modelData.wiki)
                    }
                }

                Text {
                    Layout.fillWidth: true
                    text: wizard.followed.length === 0
                        ? "No teams yet — that is fine, the schedule still works."
                        : "Following " + wizard.followed.length + ": " + wizard.followed.map(Model.followLabel).join(", ")
                    color: wizard.followed.length === 0 ? Theme.muted : Theme.accent
                    wrapMode: Text.WordWrap
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontCaption
                }
            }

            // 2 — spoilers
            ColumnLayout {
                spacing: 12

                Text {
                    Layout.fillWidth: true
                    text: "How much do you want hidden?"
                    color: Theme.foreground
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontTitle
                }
                Text {
                    Layout.fillWidth: true
                    text: "Results are withheld by the daemon, not just undrawn — a score you have not asked for is absent from the file this app reads."
                    color: Theme.muted
                    wrapMode: Text.WordWrap
                    font.family: Theme.fontFamily
                    font.pixelSize: Theme.fontCaption
                }

                Repeater {
                    model: [
                        { id: "strict", title: "Strict — recommended",
                          body: "Finished matches show the fixture and nothing else. Score, winner, VOD title, artwork and runtime all wait until you reveal them." },
                        { id: "balanced", title: "Balanced",
                          body: "Hides scores and artwork, but admits a match finished and how long its VOD runs. Quicker to browse; a short VOD still hints at a sweep." },
                        { id: "off", title: "Off",
                          body: "Show everything. Best if you mostly watch live." }
                    ]
                    delegate: Rectangle {
                        required property var modelData
                        Layout.fillWidth: true
                        implicitHeight: choice.implicitHeight + 20
                        radius: Theme.radius
                        readonly property bool active: wizard.config && wizard.config.spoilers === modelData.id
                        color: active ? Theme.alpha(Theme.accent, 0.12) : Theme.alpha(Theme.foreground, 0.04)
                        border.width: 1
                        border.color: active ? Theme.accent : Theme.alpha(Theme.foreground, 0.12)

                        HoverHandler { cursorShape: Qt.PointingHandCursor }
                        TapHandler { onTapped: wizard.apply("spoilers", modelData.id) }

                        ColumnLayout {
                            id: choice
                            anchors.left: parent.left
                            anchors.right: parent.right
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.leftMargin: 14
                            anchors.rightMargin: 14
                            spacing: 2

                            Text {
                                text: modelData.title
                                color: Theme.foreground
                                font.family: Theme.fontFamily
                                font.pixelSize: Theme.fontBody
                                font.bold: parent.parent.active
                            }
                            Text {
                                Layout.fillWidth: true
                                text: modelData.body
                                color: Theme.muted
                                wrapMode: Text.WordWrap
                                font.family: Theme.fontFamily
                                font.pixelSize: Theme.fontCaption
                            }
                        }
                    }
                }

                Item { Layout.fillHeight: true }
            }
        }

        Rectangle { Layout.fillWidth: true; implicitHeight: 1; color: Theme.alpha(Theme.foreground, 0.12) }

        // ---- navigation ----
        RowLayout {
            Layout.fillWidth: true
            spacing: 8

            AppButton {
                text: "Back"
                visible: wizard.step > 0
                subtle: true
                onClicked: wizard.step = Math.max(0, wizard.step - 1)
            }

            AppButton {
                text: "Skip setup"
                subtle: true
                onClicked: wizard.finished()
            }

            Item { Layout.fillWidth: true }

            Text {
                visible: wizard.step === 0
                text: wizard.enabledCount() + " game(s) on"
                color: wizard.enabledCount() === 0 ? Theme.urgent : Theme.muted
                font.family: Theme.fontFamily
                font.pixelSize: Theme.fontCaption
            }

            AppButton {
                text: wizard.step === wizard.stepCount - 1 ? "Finish" : "Next"
                accentuated: true
                onClicked: {
                    if (wizard.step === wizard.stepCount - 1) wizard.finished()
                    else wizard.step++
                }
            }
        }
    }
}
