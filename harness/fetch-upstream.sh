#!/bin/sh
# Recreates upstream/ — the reference tools the harness compares against —
# at the commits recorded in harness/UPSTREAM.md, with the same pinned
# tree-sitter versions. Only needed for `run.sh live|record|selftest`;
# golden-mode `run.sh check` works without it.
set -eu
ROOT=$(cd "$(dirname "$0")/.." && pwd)
. "$ROOT/harness/env.sh"
mkdir -p "$ROOT/upstream"

fetch() { # name sha
  dir="$ROOT/upstream/$1"
  if [ ! -d "$dir/.git" ]; then
    git clone --quiet "https://github.com/unclebob/$1.git" "$dir"
  fi
  git -C "$dir" fetch --quiet origin
  git -C "$dir" checkout --quiet "$2"
  echo "$1 @ $(git -C "$dir" rev-parse --short HEAD)"
}

install() { # name
  dir="$ROOT/upstream/$1"
  [ -x "$dir/.venv/bin/python" ] || python3 -m venv "$dir/.venv"
  "$dir/.venv/bin/pip" install --quiet tree-sitter==0.26.0 tree-sitter-language-pack==1.23.1
  "$dir/.venv/bin/pip" install --quiet -e "$dir[dev]"
}

fetch crapper 6fd92c4c9bcc56efb64a2f8743d5a2993bcfd42d
fetch dryer   66ff6d21a42c04afcad89c78a80066176d1294b0
fetch mutator a5a0cb803b66e0cdb8bf471b3b08af701d424a41
fetch ub-mode 3da62bf
for t in crapper dryer mutator; do install "$t"; done
echo "upstream ready: $ROOT/upstream"
