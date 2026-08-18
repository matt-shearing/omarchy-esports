#!/usr/bin/env bash
# The bar plugin and the app both need Model.js, and neither can import from
# outside its own directory, so the canonical copy lives in shared/ and is
# mirrored into both. Run this after editing shared/Model.js.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")"
cp shared/Model.js plugin/contra.esports/Model.js
cp shared/Model.js app/Model.js
echo "synced shared/Model.js -> plugin/, app/"
