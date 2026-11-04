#!/usr/bin/env bash
# (Re)create the dependencies of the test-ingestion fixtures (docs/design-tests.md):
# a second Python venv with pytest-cov (contexts) for fixtures/python-tests, and
# node_modules for the TS/JS ones and fixtures/react (`coderefinery react`).
# The harness fixtures (setup.sh) are untouched.
# Installs stay under .local/ and the fixture dirs; deletes use literal paths only.
set -euo pipefail
source "$(dirname "$0")/../harness/env.sh"
F="$REPO_ROOT/fixtures"
TESTS_VENV="$REPO_LOCAL/venvs/fixture-python-tests"
if [ ! -x "$TESTS_VENV/bin/python" ]; then
  python3 -m venv "$TESTS_VENV"
fi
"$TESTS_VENV/bin/pip" install -q pytest==9.2.0 coverage==7.17.1 pytest-cov==7.0.0
ln -sfn "$TESTS_VENV" "$F/python-tests/.venv"
for d in ts-tests js-jest-tests react; do
  if [ -f "$F/$d/package-lock.json" ]; then (cd "$F/$d" && npm ci --no-audit --no-fund); fi
done
cd "$F" || exit 1
find python-tests -name '*.pyc' -delete
find python-tests -name __pycache__ -type d -empty -delete
rm -rf python-tests/.pytest_cache python-tests/.metrics python-tests/target
rm -rf go-tests/.metrics go-tests/target
rm -rf ts-tests/.metrics ts-tests/target ts-tests/coverage ts-tests/node_modules/.vite
rm -rf js-jest-tests/.metrics js-jest-tests/target js-jest-tests/coverage
rm -rf react/.metrics react/target react/coverage react/node_modules/.vite
echo "test fixtures ready"
