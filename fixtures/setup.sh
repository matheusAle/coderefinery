#!/usr/bin/env bash
# (Re)create the fixture dependencies inside ub-mode: the shared Python venv
# (pytest + coverage), the python fixture's .venv symlink, and node_modules.
set -euo pipefail
source "$(dirname "$0")/../harness/env.sh"
F="$UBMODE/fixtures"
if [ ! -x "$FIXTURE_VENV/bin/python" ]; then
  python3 -m venv "$FIXTURE_VENV"
fi
"$FIXTURE_VENV/bin/pip" install -q pytest==9.2.0 coverage==7.17.1
ln -sfn "$FIXTURE_VENV" "$F/python/.venv"
(cd "$F/typescript" && npm ci --no-audit --no-fund)
(cd "$F/javascript" && npm ci --no-audit --no-fund)
(cd "$F/go" && go mod verify >/dev/null 2>&1 || true)
# Leave no caches that would change test ordering or coverage inside the pristine trees.
find "$F" -name __pycache__ -prune -exec rm -rf {} +
rm -rf "$F"/*/.pytest_cache "$F"/*/node_modules/.vite "$F"/*/.metrics "$F"/*/target "$F"/*/coverage
echo "fixtures ready"
