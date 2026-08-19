# Esports — spoiler-free match schedule for the Omarchy bar

A bar widget showing the next match for the teams you follow, with a dropdown
listing what is live, what is coming, and what you still have to watch — and a
spoiler blackout that is enforced by the daemon rather than by the UI.

![Esports widget](preview.png)

- **Next match on the bar** with a live countdown.
- **Click a match** for an inline detail panel: full team names, kickoff, format,
  stream links, VOD, and links to the relevant Liquipedia pages.
- **Spoiler-free by construction.** Finished matches show the fixture and
  nothing else until you reveal them. The score is not merely hidden — it is
  absent from the file this widget reads.
- **Catch-up masking.** If you are behind on a team's matches, their later
  fixtures have the *opponent* withheld too, because knowing who they play next
  tells you they won.
- **Filter to your teams** with the ★ Mine toggle in the panel header.
- Covers **Dota 2**, **Counter-Strike** and **StarCraft II** out of the box,
  with 19 games verified and one command away.

## Requires the daemon

This plugin is the front end. It reads a state file and never touches the
network. The `omarchy-esports` daemon does the fetching, the spoiler redaction
and the notifications, and you need it for the widget to show anything:

```bash
git clone https://github.com/matt-shearing/omarchy-esports
cd omarchy-esports && ./install.sh
```

That also installs the companion app and a systemd user service. Until the
daemon is running the widget shows a panel telling you so.

## Install

```bash
omarchy plugin add https://github.com/matt-shearing/omarchy-esports-plugin --enable
```

Or by hand:

```bash
git clone https://github.com/matt-shearing/omarchy-esports-plugin \
  ~/.config/omarchy/plugins/contra.esports
omarchy-shell shell rescanPlugins
omarchy plugin enable contra.esports --section left
```

## Remove

```bash
omarchy plugin disable contra.esports
omarchy plugin remove contra.esports
```

Or by hand:

```bash
omarchy plugin disable contra.esports
rm -r ~/.config/omarchy/plugins/contra.esports
omarchy restart shell
```

Removing the plugin leaves the daemon and its state alone. To remove those too:

```bash
systemctl --user disable --now omarchy-esports
rm -r ~/.local/state/omarchy-esports ~/.config/omarchy-esports
rm ~/.local/bin/omarchy-esports ~/.local/bin/omarchy-esports-app
```

## Settings

Widget options live on the bar entry in `~/.config/omarchy/shell.json`:

```bash
omarchy bar set contra.esports showLabel false   # icon only
omarchy bar set contra.esports maxRows 14        # rows in the dropdown
omarchy bar set contra.esports showFinished false
omarchy bar set contra.esports hideWhenEmpty true
```

## Keybinding

```lua
-- ~/.config/hypr/bindings.lua
o.bind("SUPER + ALT + E", "Esports panel", "omarchy-shell shell toggle contra.esports")
```

Bind through `shell toggle` rather than a plugin IPC target: a bar widget is
instantiated once per monitor and an IPC target reaches only one of them, so an
IPC binding opens the panel on the wrong screen. `shell toggle` routes through
the bar, which picks the focused one.

## Licence

MIT. Match data from [Liquipedia](https://liquipedia.net), CC BY-SA 3.0.
