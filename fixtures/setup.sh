#!/usr/bin/env bash
# (Re)create the fixture dependencies inside the repo: the shared Python venv
# (pytest + coverage), the python fixture's .venv symlink, and node_modules.
set -euo pipefail
source "$(dirname "$0")/../harness/env.sh"
F="$REPO_ROOT/fixtures"
if [ ! -x "$FIXTURE_VENV/bin/python" ]; then
  python3 -m venv "$FIXTURE_VENV"
fi
"$FIXTURE_VENV/bin/pip" install -q pytest==9.2.0 coverage==7.17.1
ln -sfn "$FIXTURE_VENV" "$F/python/.venv"
(cd "$F/typescript" && npm ci --no-audit --no-fund)
(cd "$F/javascript" && npm ci --no-audit --no-fund)
(cd "$F/go" && go mod verify >/dev/null 2>&1 || true)
# Leave no caches that would change test ordering or coverage inside the pristine trees.
# Deletes use literal paths only, relative to the fixtures dir (never a variable).
cd "$F" || exit 1
find python -name '*.pyc' -delete
find python -name __pycache__ -type d -empty -delete
rm -rf python/.pytest_cache python/.metrics python/target python/coverage
rm -rf typescript/node_modules/.vite typescript/.metrics typescript/target typescript/coverage
rm -rf javascript/node_modules/.vite javascript/.metrics javascript/target javascript/coverage
rm -rf go/.metrics go/target go/coverage
echo "fixtures ready"
