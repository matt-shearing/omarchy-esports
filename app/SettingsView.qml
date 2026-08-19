import QtQuick
import QtQuick.Controls
import QtQuick.Layouts
import "Model.js" as Model

// Settings. Every change is written through the CLI rather than by editing
// config.json here, so validation, clamping and the daemon's reload path stay
// in one place — the app never has to know that a poll interval has a floor.
ScrollView {
    id: view

    property var config: null
    property var teamIndex: []

    signal apply(string key, string value)
    signal applyWiki(string slug, bool on)

    clip: true
    ScrollBar.horizontal.policy: ScrollBar.AlwaysOff

    ColumnLayout {
        width: view.availableWidth
        spacing: 18

        // ---- spoilers ----
        Text {
            text: "SPOILERS"
            color: Theme.foreground
            opacity: 0.5
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontCaption
            font.bold: true
            font.letterSpacing: 1
        }

        SettingRow {
            label: "Blackout policy"
            help: "Strict withholds the score, the winner, the VOD title, its artwork and its runtime until you reveal a match. Balanced keeps a scrubbed title and the runtime."
            Repeater {
                model: ["strict", "balanced", "off"]
                delegate: AppButton {
                    required property var modelData
                    text: modelData
                    accentuated: view.config && view.config.spoilers === modelData
                    onClicked: view.apply("spoilers", modelData)
                }
            }
        }

        SettingRow {
            label: "Catch-up masking"
            help: "While you are behind on a followed team, hide the opponent of their later matches — knowing who they play next reveals that they won."
            AppButton {
                text: (view.config && view.config.catchUp.enabled) ? "On" : "Off"
                accentuated: view.config && view.config.catchUp.enabled
                onClicked: view.apply("catchUp.enabled",
                    (view.config && view.config.catchUp.enabled) ? "false" : "true")
            }
        }

        SettingRow {
            label: "Backlog window"
            help: "How far back an unwatched match still counts as a backlog. Beyond this it is treated as history and stops hiding anything."
            Repeater {
                model: ["24h", "48h", "72h", "168h"]
                delegate: AppButton {
                    required property var modelData
                    text: modelData
                    accentuated: view.config &&
                        String(view.config.catchUp.window).indexOf(modelData.replace("h", "h")) === 0
                    onClicked: view.apply("catchUp.window", modelData)
                }
            }
        }

        Rectangle { Layout.fillWidth: true; implicitHeight: 1; color: Theme.alpha(Theme.foreground, 0.1) }

        // ---- games ----
        Text {
            text: "GAMES"
            color: Theme.foreground
            opacity: 0.5
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontCaption
            font.bold: true
            font.letterSpacing: 1
        }

        Text {
            Layout.fillWidth: true
            text: "Which Liquipedia wikis to poll. Each enabled game costs one request per refresh, and Liquipedia allows one page parse every 30 seconds — so turning games off makes refreshes quicker, not just quieter."
            color: Theme.muted
            wrapMode: Text.WordWrap
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontCaption
        }

        Repeater {
            model: view.config ? view.config.wikis : []
            delegate: SettingRow {
                required property var modelData
                label: modelData.game || modelData.slug
                help: modelData.slug + " · ticker: " + (modelData.tickerPage || "")
                AppButton {
                    text: modelData.enabled ? "On" : "Off"
                    accentuated: modelData.enabled
                    onClicked: view.applyWiki(modelData.slug, !modelData.enabled)
                }
            }
        }

        Rectangle { Layout.fillWidth: true; implicitHeight: 1; color: Theme.alpha(Theme.foreground, 0.1) }

        // ---- notifications ----
        Text {
            text: "NOTIFICATIONS"
            color: Theme.foreground
            opacity: 0.5
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontCaption
            font.bold: true
            font.letterSpacing: 1
        }

        Repeater {
            model: [
                { key: "matchStarting", label: "Match starting", help: "Ahead of a followed team's match, with the lead time below." },
                { key: "matchLive", label: "Match live", help: "The moment a followed match crosses its start time." },
                { key: "vodReady", label: "VOD ready", help: "When a recording appears. The message names the fixture only, never the result." },
                { key: "tournamentStarting", label: "Tournament starting", help: "A day ahead of an event featuring a team you follow." },
                { key: "quiet", label: "Quiet mode", help: "Suppress everything without losing the settings above." }
            ]
            delegate: SettingRow {
                required property var modelData
                label: modelData.label
                help: modelData.help
                AppButton {
                    readonly property bool on: view.config &&
                        view.config.notifications[modelData.key] === true
                    text: on ? "On" : "Off"
                    accentuated: on && modelData.key !== "quiet"
                    onClicked: view.apply("notifications." + modelData.key, on ? "false" : "true")
                }
            }
        }

        Rectangle { Layout.fillWidth: true; implicitHeight: 1; color: Theme.alpha(Theme.foreground, 0.1) }

        // ---- schedule ----
        Text {
            text: "SCHEDULE"
            color: Theme.foreground
            opacity: 0.5
            font.family: Theme.fontFamily
            font.pixelSize: Theme.fontCaption
            font.bold: true
            font.letterSpacing: 1
        }

        SettingRow {
            label: "Only my teams"
            help: "Drop matches with no followed team from the schedule entirely."
            AppButton {
                text: (view.config && view.config.followedOnly) ? "On" : "Off"
                accentuated: view.config && view.config.followedOnly
                onClicked: view.apply("followedOnly", (view.config && view.config.followedOnly) ? "false" : "true")
            }
        }

        SettingRow {
            label: "Hide unseeded fixtures"
            help: "Liquipedia publishes bracket slots before the teams are known. These are noise in a ticker."
            AppButton {
                text: (view.config && view.config.hideTBD) ? "On" : "Off"
                accentuated: view.config && view.config.hideTBD
                onClicked: view.apply("hideTBD", (view.config && view.config.hideTBD) ? "false" : "true")
            }
        }

        SettingRow {
            label: "Refresh every"
            help: "Floored at 5 minutes. Liquipedia's terms require caching rather than re-fetching unchanged pages."
            Repeater {
                model: ["10m", "15m", "30m", "1h"]
                delegate: AppButton {
                    required property var modelData
                    text: modelData
                    accentuated: view.config && String(view.config.pollInterval).indexOf(modelData) === 0
                    onClicked: view.apply("pollInterval", modelData)
                }
            }
        }

        SettingRow {
            label: "Contact address"
            help: view.config && view.config.contactEmail !== ""
                ? "Sent to Liquipedia in the User-Agent on every request, as their API terms require. Currently: " + view.config.contactEmail
                : "Not set. Liquipedia's API terms ask for a contact address; without one the client falls back to the project's issues URL."
            AppButton {
                text: "Edit config"
                subtle: true
                onClicked: view.apply("__edit", "")
            }
        }

        Item { Layout.preferredHeight: 8 }
    }
}
