#!/usr/bin/env bash
# Run the coderefinery-vs-upstream matrix.
#
#   harness/run.sh record   [compare flags]   run upstream, rewrite golden/
#   harness/run.sh check    [compare flags]   coderefinery vs golden (default)
#   harness/run.sh live     [compare flags]   coderefinery vs upstream, side by side
#   harness/run.sh selftest [compare flags]   upstream vs golden (harness sanity / nondeterminism)
#   harness/run.sh selflive [compare flags]   upstream vs upstream, side by side
#   harness/run.sh test                       unit tests of the harness itself
#
# Any extra flags go to cmd/compare, e.g. -tool crap -fixture go -variant default,
# -ours '/path/to/coderefinery {tool} {args}', -float-tol 1e-4, -strict-stderr, -max-diffs 20.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
source "$HERE/env.sh"
mode="${1:-check}"
[ $# -gt 0 ] && shift
(cd "$HERE" && go build -o bin/compare ./cmd/compare)
case "$mode" in
  record)   exec "$HERE/bin/compare" -record "$@" ;;
  check)    exec "$HERE/bin/compare" "$@" ;;
  live)     exec "$HERE/bin/compare" -live "$@" ;;
  selftest) exec "$HERE/bin/compare" -ours upstream "$@" ;;
  selflive) exec "$HERE/bin/compare" -live -ours upstream "$@" ;;
  test)     cd "$HERE" && exec go test ./... ;;
  *) echo "usage: $0 {record|check|live|selftest|selflive|test} [compare flags]" >&2; exit 2 ;;
esac
