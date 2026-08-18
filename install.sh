#!/usr/bin/env bash
# Install omarchy-esports: the daemon binary, the bar plugin, and the app.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BIN_DIR="${HOME}/.local/bin"
PLUGIN_DIR="${HOME}/.config/omarchy/plugins"
APP_DIR="${HOME}/.local/share/omarchy-esports"
DESKTOP_DIR="${HOME}/.local/share/applications"

log() { printf '\033[1;35m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33mwarning:\033[0m %s\n' "$*" >&2; }

command -v go >/dev/null || { echo "go is required to build omarchy-esports" >&2; exit 1; }

log "Syncing shared sources"
"$REPO_DIR/sync-shared.sh" >/dev/null

log "Building omarchy-esports"
mkdir -p "$BIN_DIR"
(cd "$REPO_DIR" && go build -trimpath -ldflags "-s -w" -o "$BIN_DIR/omarchy-esports" ./cmd/omarchy-esports)

log "Installing the bar plugin"
mkdir -p "$PLUGIN_DIR"
target="$PLUGIN_DIR/contra.esports"
# Clear any previous install. Scoped to our own plugin directory, and only
# when it is exactly that path.
if [[ -L "$target" ]]; then
  unlink "$target"
elif [[ -d "$target" ]]; then
  find "$target" -mindepth 1 -delete
  rmdir "$target"
fi

# A symlink keeps a dev checkout live, but the shell's inotify watcher does not
# follow symlinks, so edits then need `omarchy restart shell` rather than the
# usual automatic reload. A copy is the better default for a plain install.
if [[ "${DEV_LINK:-0}" == "1" ]]; then
  ln -sfn "$REPO_DIR/plugin/contra.esports" "$target"
  log "Linked plugin (dev mode) — reload edits with: omarchy restart shell"
else
  cp -r "$REPO_DIR/plugin/contra.esports" "$target"
fi

log "Installing the app"
mkdir -p "$APP_DIR"
cp -r "$REPO_DIR/app/." "$APP_DIR/"

cat > "$BIN_DIR/omarchy-esports-app" <<LAUNCHER
#!/usr/bin/env bash
# Launch the esports app as its own Quickshell instance.
exec qs -p "${APP_DIR}" -n "\$@"
LAUNCHER
chmod +x "$BIN_DIR/omarchy-esports-app"

mkdir -p "$DESKTOP_DIR"
cat > "$DESKTOP_DIR/omarchy-esports.desktop" <<DESKTOP
[Desktop Entry]
Type=Application
Name=Esports
Comment=Spoiler-free esports schedule, streams and VODs
Exec=${BIN_DIR}/omarchy-esports-app
Icon=applications-games
Terminal=false
Categories=Game;Network;
DESKTOP

log "Installing the user service"
mkdir -p "${HOME}/.config/systemd/user"
cp "$REPO_DIR/systemd/omarchy-esports.service" "${HOME}/.config/systemd/user/"
systemctl --user daemon-reload
systemctl --user enable --now omarchy-esports.service 2>/dev/null \
  || warn "could not enable the service; start it with: systemctl --user start omarchy-esports"

log "Enabling the bar widget"
omarchy plugin enable contra.esports --section left 2>/dev/null \
  || warn "enable it yourself with: omarchy plugin enable contra.esports"
omarchy restart shell >/dev/null 2>&1 || true

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) warn "$BIN_DIR is not on your PATH" ;;
esac

cat <<'DONE'

Installed.

  Follow some teams:   omarchy-esports teams add "Team Spirit" "G2 Esports"
  See the schedule:    omarchy-esports status
  Open the app:        omarchy-esports-app
  Toggle the panel:    omarchy-shell shell toggle contra.esports

Match data from Liquipedia (CC BY-SA 3.0).
DONE
