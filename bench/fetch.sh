#!/usr/bin/env bash
# Clone (shallow, pinned) every repo in bench/repos.txt into .local/bench/repos/<name>.
# Usage: bench/fetch.sh [name...]      (default: all)
# Repos with sha "-" are fetched at the default branch HEAD; the resolved SHA is printed so it
# can be pinned in repos.txt. Existing checkouts at the right SHA are left alone.
set -euo pipefail
UBMODE="${UBMODE:-/Users/matheusale/workspaces/ub-mode}"
# shellcheck disable=SC1091
source "$UBMODE/harness/env.sh"
REPOS_TXT="$UBMODE/bench/repos.txt"
DEST="$UBMODE_LOCAL/bench/repos"
mkdir -p "$DEST"

want=" $* "
grep -v '^\s*#' "$REPOS_TXT" | grep -v '^\s*$' | while read -r name url sha lang size sparse; do
  if [ $# -gt 0 ] && [[ "$want" != *" $name "* ]]; then continue; fi
  dir="$DEST/$name"
  if [ -d "$dir/.git" ] && [ "$sha" != "-" ] && [ "$(git -C "$dir" rev-parse HEAD)" = "$sha" ]; then
    echo "$name: already at $sha"; continue
  fi
  # Existing checkouts are updated in place (fetch + forced checkout); nothing is deleted.
  if [ ! -d "$dir/.git" ]; then
    mkdir -p "$dir"
    git -C "$dir" init -q
    git -C "$dir" remote add origin "$url"
  fi
  if [ "$sparse" != "." ]; then
    git -C "$dir" config remote.origin.partialclonefilter blob:none
    git -C "$dir" config remote.origin.promisor true
    git -C "$dir" sparse-checkout set --cone ${sparse//,/ }
  else
    git -C "$dir" sparse-checkout disable 2>/dev/null || true
  fi
  ref="$sha"; [ "$sha" = "-" ] && ref=HEAD
  filter=(); [ "$sparse" != "." ] && filter=(--filter=blob:none)
  git -C "$dir" fetch -q --depth 1 ${filter[@]+"${filter[@]}"} origin "$ref"
  git -C "$dir" -c advice.detachedHead=false checkout -q -f FETCH_HEAD
  echo "$name: $(git -C "$dir" rev-parse HEAD) ($lang, sparse=$sparse)"
done
