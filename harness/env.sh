# Source this file before running anything in ub-mode.
# Keeps every cache, venv, module and package download inside the ub-mode folder.
UBMODE="${UBMODE:-/Users/matheusale/workspaces/ub-mode}"
export UBMODE
export UBMODE_LOCAL="$UBMODE/.local"

# Python / pip
export PIP_CACHE_DIR="$UBMODE_LOCAL/cache/pip"
export PIP_DISABLE_PIP_VERSION_CHECK=1
export PIP_REQUIRE_VIRTUALENV=1          # refuse any pip install outside a venv
export XDG_CACHE_HOME="$UBMODE_LOCAL/cache/xdg"

# Node / npm
export npm_config_cache="$UBMODE_LOCAL/cache/npm"
export npm_config_prefix="$UBMODE_LOCAL/npm-global"
export npm_config_update_notifier=false
export npm_config_fund=false
export npm_config_audit=false
export NO_UPDATE_NOTIFIER=1

# Go
export GOPATH="$UBMODE_LOCAL/go"
export GOCACHE="$UBMODE_LOCAL/cache/go-build"
export GOMODCACHE="$UBMODE_LOCAL/go/pkg/mod"
export GOENV="$UBMODE_LOCAL/go/env"
export GOTOOLCHAIN=local
export GOFLAGS="-modcacherw"

# Upstream tools and the shared fixture interpreter (pytest + coverage).
export UPSTREAM="$UBMODE/upstream"
export FIXTURE_VENV="$UBMODE_LOCAL/venvs/fixture-python"
export SCRATCH_ROOT="$UBMODE_LOCAL/scratch"

# Deterministic, colourless tool output.
export NO_COLOR=1
export FORCE_COLOR=0
export CI=1
export TZ=UTC
export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8

# Go from asdf (.tool-versions at the ub-mode root), behind a wrapper that
# points HOME inside ub-mode for the go process only: Go writes telemetry
# counters to os.UserConfigDir() (~/Library/Application Support/go), which no
# env var redirects. GOPATH/GOCACHE/GOMODCACHE/GOENV above are explicit, so
# the fake HOME changes nothing else.
UBMODE_GOROOT_BIN="${UBMODE_GOROOT_BIN:-$HOME/.asdf/installs/golang/1.26.1/go/bin}"
mkdir -p "$UBMODE_LOCAL/bin" "$UBMODE_LOCAL/home"
if [ ! -x "$UBMODE_LOCAL/bin/go" ] || ! grep -q "$UBMODE_GOROOT_BIN" "$UBMODE_LOCAL/bin/go"; then
  printf '#!/bin/sh\nHOME="%s" exec "%s/go" "$@"\n' "$UBMODE_LOCAL/home" "$UBMODE_GOROOT_BIN" > "$UBMODE_LOCAL/bin/go"
  chmod +x "$UBMODE_LOCAL/bin/go"
fi
# The fixture venv's python3 wins, so crapper's `python3 -m coverage` never touches the system Python.
PATH="$UBMODE_LOCAL/bin:$FIXTURE_VENV/bin:$UBMODE_LOCAL/npm-global/bin:$GOPATH/bin:$PATH"
export PATH
mkdir -p "$PIP_CACHE_DIR" "$npm_config_cache" "$GOCACHE" "$GOMODCACHE" "$SCRATCH_ROOT" "$XDG_CACHE_HOME"
