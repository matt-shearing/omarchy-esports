#!/usr/bin/env bash
# Build the standalone plugin repo tree for publication.
#
# `omarchy plugin add <git-url>` clones a repo and validates
# "$clone/manifest.json" — it only ever looks at the repository ROOT, with no
# option for a subdirectory. This repo keeps the plugin under plugin/ alongside
# the daemon and the app, so publishing needs a separate tree with the manifest
# promoted to the top level.
#
# Usage:
#   ./package-plugin.sh [outdir]        # default: dist/plugin-repo
#
# The result is a directory you can `git init` and push as its own public repo,
# which is what gets submitted to omarchyplugins.com.
set -euo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="${1:-$REPO_DIR/dist/plugin-repo}"
PLUGIN_ID="$(jq -r '.id' "$REPO_DIR/plugin/contra.esports/manifest.json")"
VERSION="$(jq -r '.version' "$REPO_DIR/plugin/contra.esports/manifest.json")"

log() { printf '\033[1;35m==>\033[0m %s\n' "$*"; }

command -v jq >/dev/null || { echo "jq is required" >&2; exit 1; }

log "Syncing shared sources"
"$REPO_DIR/sync-shared.sh" >/dev/null

log "Building $PLUGIN_ID $VERSION into $OUT"
if [[ -d "$OUT" ]]; then
  find "$OUT" -mindepth 1 -delete
fi
mkdir -p "$OUT"

# The plugin's own files, promoted to the root. No symlinks: the validator
# rejects any symlink inside a plugin folder.
cp "$REPO_DIR"/plugin/contra.esports/*.qml "$OUT/"
cp "$REPO_DIR"/plugin/contra.esports/*.js "$OUT/"
cp "$REPO_DIR"/plugin/contra.esports/manifest.json "$OUT/"
cp "$REPO_DIR"/LICENSE "$OUT/"

# The marketplace looks for a root screenshot with this exact basename.
if [[ -f "$REPO_DIR/docs/preview.png" ]]; then
  cp "$REPO_DIR/docs/preview.png" "$OUT/preview.png"
else
  echo "note: docs/preview.png not found — add one before submitting" >&2
fi

cp "$REPO_DIR/docs/plugin-README.md" "$OUT/README.md"

cat > "$OUT/.gitignore" <<'EOF'
.qmlc
*.qmlc
.DS_Store
*.swp
*~
EOF

log "Validating"
if command -v omarchy-plugin-validate >/dev/null; then
  omarchy-plugin-validate "$OUT"
  echo "    validation passed"
else
  echo "    omarchy-plugin-validate not on PATH; skipped" >&2
fi

if find "$OUT" -type l | grep -q .; then
  echo "error: symlinks found in the packaged tree; the validator rejects them" >&2
  exit 1
fi

log "Done"
cat <<DONE

  Packaged: $OUT

  Publish it as its own repo:

    cd $OUT
    git init -b main && git add -A
    git commit -m "$PLUGIN_ID $VERSION"
    gh repo create omarchy-esports-plugin --public --source=. --push

  Then submit to the community catalog (see docs/PUBLISHING.md):

    gh issue create --repo HANCORE-linux/omarchy-plugin-marketplace \\
      --title "[Plugin]: Esports"

DONE
