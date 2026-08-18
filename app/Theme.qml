pragma Singleton

import QtQuick
import Quickshell
import Quickshell.Io

// Theme reads omarchy's own theme files rather than defining a palette of its
// own, so the app follows a theme switch live: omarchy rewrites colors.toml
// when the theme changes, FileView notices, and every binding updates.
//
// The bar plugin gets this for free from the shell's Color singleton, but this
// app runs as a separate Quickshell instance and cannot import qs.Commons, so
// it reads the same source of truth directly.
QtObject {
    id: theme

    readonly property string themeDir: (Quickshell.env("HOME") || "") + "/.local/state/omarchy/current/theme"

    // Sensible fallbacks, used until the file loads and if a key is absent.
    property color background: "#191c2a"
    property color surface: "#12141f"
    property color surfaceAlt: "#282d3c"
    property color foreground: "#e6ebf5"
    property color muted: "#6a7080"
    property color accent: "#ff0080"
    property color urgent: "#ff5555"
    property color success: "#50ffb4"
    property bool dark: true

    readonly property string fontFamily: "monospace"

    // Type scale, mirroring the shell's Style.font tokens closely enough that
    // the app reads as part of the same system.
    readonly property int fontCaption: 11
    readonly property int fontBody: 13
    readonly property int fontSubtitle: 15
    readonly property int fontTitle: 17
    readonly property int fontHeading: 21
    readonly property int fontDisplay: 28

    readonly property int radius: 8
    readonly property int gap: 10

    property FileView colorsFile: FileView {
        path: theme.themeDir + "/colors.toml"
        watchChanges: true
        printErrors: false
        onLoaded: theme.applyColors(text())
        onFileChanged: reload()
    }

    // parseToml pulls `key = "value"` pairs out of the theme file. The file is
    // a flat list of colour assignments with # comments, so a full TOML parser
    // would be more machinery than the format warrants.
    function parseToml(text) {
        var out = {}
        var lines = String(text || "").split("\n")
        for (var i = 0; i < lines.length; i++) {
            var line = lines[i].trim()
            if (line === "" || line.charAt(0) === "#" || line.charAt(0) === "[") continue
            var eq = line.indexOf("=")
            if (eq < 0) continue
            var key = line.substring(0, eq).trim()
            var value = line.substring(eq + 1).trim()
            // Strip a trailing comment, then surrounding quotes.
            var hash = value.indexOf("#", value.charAt(0) === '"' ? value.indexOf('"', 1) : 0)
            if (hash > 0) value = value.substring(0, hash).trim()
            value = value.replace(/^["']|["']$/g, "")
            out[key] = value
        }
        return out
    }

    function applyColors(text) {
        var t = parseToml(text)
        if (t.background) theme.background = t.background
        if (t.dark_background) theme.surface = t.dark_background
        if (t.lighter_background) theme.surfaceAlt = t.lighter_background
        if (t.foreground) theme.foreground = t.foreground
        if (t.muted) theme.muted = t.muted
        else if (t.dark_foreground) theme.muted = t.dark_foreground
        if (t.accent) theme.accent = t.accent
        if (t.red) theme.urgent = t.red
        if (t.green) theme.success = t.green
        if (t.mode) theme.dark = (t.mode === "dark")
    }

    // alpha returns a colour with the given opacity, for subtle fills.
    function alpha(c, a) {
        return Qt.rgba(c.r, c.g, c.b, a)
    }
}
