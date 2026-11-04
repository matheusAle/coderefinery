#!/usr/bin/env bash
# Benchmark matrix: ubgate vs the upstream Python tools (crapper, dryer, mutator).
#
#   bench/run.sh [phase ...]        default: fetch build env help crap dry mutate-scan mutate-full cover parity report
#
# Phases
#   fetch        clone the pinned repos (bench/fetch.sh) and make one work copy per implementation
#   build        build bench and a pinned ubgate snapshot (git HEAD of ubgate/, or UBGATE_SRC=worktree)
#   env          record machine / toolchain info
#   help         startup cost: `--help` of each tool, both implementations
#   crap         crap --no-coverage on every repo
#   dry          dry (default flags) on every repo
#   mutate-scan  mutate --scan on every repo
#   mutate-full  full mutation run (--no-coverage) on fixtures/python and fixtures/go
#   cover        crap --use-existing-coverage on fzf (Go profile produced once, untimed)
#   parity       diff stdout and .metrics between upstream and ubgate for every case
#   report       results.jsonl + parity.jsonl -> summary.md + summary.csv
#   profile      CPU + heap profile of ubgate on PROFILE_CASE (default "dry typescript")
#
# Knobs (environment)
#   RUNS=3 WARMUP=1       measured runs and warm-up runs per case
#   TIMEOUT=30m           per-run timeout (process group killed; reported as "> timeout")
#   SLOW=120s             if the warm-up takes longer, it becomes the single measured run
#   MAX_LOAD=4            wait (up to LOAD_WAIT=3m) for the 1-min load average to drop below this
#   REPOS="django ..."    subset of bench/repos.txt names (default: all)
#   IMPLS="upstream ubgate"
#   OUT=.local/bench/results/<name>   results directory (default: .local/bench/results/latest)
set -euo pipefail
UBMODE="${UBMODE:-/Users/matheusale/workspaces/ub-mode}"
# shellcheck disable=SC1091
source "$UBMODE/harness/env.sh"

# Recursive deletes below use literal paths relative to the ub-mode root only.
cd "$UBMODE"
BENCH_LOCAL="$UBMODE_LOCAL/bench"
export GOTMPDIR="$UBMODE_LOCAL/tmp"   # go build scratch stays inside ub-mode too
mkdir -p "$GOTMPDIR"
BIN="$BENCH_LOCAL/bin"
WORK="$BENCH_LOCAL/work"
OUT="${OUT:-$BENCH_LOCAL/results/latest}"
RUNS="${RUNS:-3}"
WARMUP="${WARMUP:-1}"
TIMEOUT="${TIMEOUT:-30m}"
SLOW="${SLOW:-120s}"
MAX_LOAD="${MAX_LOAD:-4}"
LOAD_WAIT="${LOAD_WAIT:-3m}"
IMPLS="${IMPLS:-upstream ubgate}"
ALL_REPOS=$(grep -v '^\s*#' "$UBMODE/bench/repos.txt" | awk 'NF {print $1}' | tr '\n' ' ')
REPOS="${REPOS:-$ALL_REPOS}"
PROFILE_CASE="${PROFILE_CASE:-dry typescript}"
mkdir -p "$OUT/out" "$BIN" "$WORK"

log() { printf '\n== %s\n' "$*" >&2; }

# upstream_cmd TOOL -> argv prefix of the upstream tool; ubgate_cmd TOOL likewise.
upstream_cmd() {
  case "$1" in
    crap) echo "$UPSTREAM/crapper/crapper" ;;
    dry) echo "$UPSTREAM/dryer/dryer" ;;
    mutate) echo "$UPSTREAM/mutator/mutator" ;;
  esac
}
impl_cmd() { # impl tool
  if [ "$1" = upstream ]; then upstream_cmd "$2"; else echo "$BIN/ubgate $2"; fi
}

# bench_case WORKLOAD REPO IMPL DIR PRE [extra bench flags...] -- ARGS...
# DIR is the working directory, or the source of per-run clones when the
# extra flags contain -fresh DIR.
bench_case() {
  local workload=$1 repo=$2 impl=$3 dir=$4 pre=$5
  shift 5
  local flags=()
  while [ "$1" != "--" ]; do flags+=("$1"); shift; done
  shift
  local tool=${workload#help-}
  tool=${tool%%-*}
  local o="$OUT/out/$workload/$repo"
  mkdir -p "$o"
  # shellcheck disable=SC2046
  "$BIN/bench" run -workload "$workload" -repo "$repo" -impl "$impl" -dir "$dir" \
    -runs "$RUNS" -warmup "$WARMUP" -timeout "$TIMEOUT" -slow "$SLOW" \
    -max-load "$MAX_LOAD" -load-wait "$LOAD_WAIT" \
    -pre "$pre" -out "$OUT/results.jsonl" -stdout "$o/$impl.stdout" -stderr "$o/$impl.stderr" -dir-out "$o/$impl.root" \
    ${flags[@]+"${flags[@]}"} -- $(impl_cmd "$impl" "$tool") "$@"
  # keep the last run's .metrics for the parity phase: a new directory per
  # batch (nothing is deleted); $impl.metricsdir points at the latest one
  local last stamp
  last=$(cat "$o/$impl.root")
  stamp=$(date -u +%Y%m%dT%H%M%S)-$$
  if [ -d "$last/.metrics" ]; then
    cp -R "$last/.metrics" "$o/$impl.metrics.$stamp"
    echo "$o/$impl.metrics.$stamp" > "$o/$impl.metricsdir"
  else
    : > "$o/$impl.metricsdir"
  fi
}

# clean_metrics DIR: the untimed -pre step for crap/dry/mutate-scan. Those
# tools write only .metrics/crap.edn and .metrics/dry.edn (mutate --scan
# writes nothing), so remove exactly those files, non-recursively.
clean_metrics() { printf "rm -f '%s/.metrics/crap.edn' '%s/.metrics/dry.edn'; rmdir '%s/.metrics' 2>/dev/null || true" "$1" "$1" "$1"; }

phase_fetch() {
  log fetch
  # shellcheck disable=SC2086
  "$UBMODE/bench/fetch.sh" $REPOS
  # fresh work copies of every repo: clear the whole (literal) work dir first
  rm -rf .local/bench/work
  for impl in upstream ubgate; do
    mkdir -p "$WORK/$impl"
    for r in $ALL_REPOS; do
      cp -c -R "$BENCH_LOCAL/repos/$r" "$WORK/$impl/$r" 2>/dev/null || cp -R "$BENCH_LOCAL/repos/$r" "$WORK/$impl/$r"
    done
  done
}

phase_build() {
  log build
  (cd "$UBMODE/bench" && go build -o "$BIN/bench" ./cmd/bench)
  if [ "${UBGATE_SRC:-head}" = worktree ]; then
    (cd "$UBMODE/ubgate" && go build -trimpath -o "$BIN/ubgate" ./cmd/ubgate)
    echo "worktree $(git -C "$UBMODE" rev-parse HEAD) dirty=$(git -C "$UBMODE" status --porcelain -- ubgate | wc -l | tr -d ' ')" > "$BIN/ubgate.version"
  else
    rm -rf .local/bench/src
    mkdir -p "$BENCH_LOCAL/src"
    git -C "$UBMODE" archive HEAD ubgate | tar -x -C "$BENCH_LOCAL/src"
    (cd "$BENCH_LOCAL/src/ubgate" && go build -trimpath -o "$BIN/ubgate" ./cmd/ubgate)
    echo "head $(git -C "$UBMODE" rev-parse HEAD)" > "$BIN/ubgate.version"
  fi
  cat "$BIN/ubgate.version"
}

phase_env() {
  log env
  {
    echo "date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "cpu: $(sysctl -n machdep.cpu.brand_string 2>/dev/null || grep -m1 'model name' /proc/cpuinfo | cut -d: -f2)"
    echo "cores: $(sysctl -n hw.ncpu 2>/dev/null || nproc) logical ($(sysctl -n hw.perflevel0.physicalcpu 2>/dev/null || echo ?) performance + $(sysctl -n hw.perflevel1.physicalcpu 2>/dev/null || echo ?) efficiency)"
    echo "ram: $(( $(sysctl -n hw.memsize 2>/dev/null || echo 0) / 1073741824 )) GiB"
    echo "os: $(sw_vers -productName 2>/dev/null) $(sw_vers -productVersion 2>/dev/null) ($(uname -srm))"
    echo "go: $(go version)"
    echo "python (upstream venvs): $("$UPSTREAM/crapper/.venv/bin/python" --version 2>&1)"
    echo "tree-sitter (upstream): $("$UPSTREAM/crapper/.venv/bin/python" -c 'import importlib.metadata as m; print(m.version("tree-sitter"), "/ language-pack", m.version("tree-sitter-language-pack"))' 2>&1)"
    echo "ubgate: $(cat "$BIN/ubgate.version" 2>/dev/null)"
    echo "upstream: crapper $(git -C "$UPSTREAM/crapper" rev-parse --short HEAD), dryer $(git -C "$UPSTREAM/dryer" rev-parse --short HEAD), mutator $(git -C "$UPSTREAM/mutator" rev-parse --short HEAD)"
    echo "load (start): $(sysctl -n vm.loadavg 2>/dev/null || cat /proc/loadavg)"
    echo "top CPU processes (start):"
    (ps -Aceo pcpu,rss,comm -r || true) | sed -n '1,8s/^/  /p'
  } | tee "$OUT/env.txt"
  for r in $REPOS; do "$BIN/bench" count -json -name "$r" -root "$BENCH_LOCAL/repos/$r"; done > "$OUT/counts.jsonl"
}

phase_help() {
  log "help (startup)"
  for tool in crap dry mutate; do
    for impl in $IMPLS; do
      RUNS=10 WARMUP=2 SLOW=0 bench_case "help-$tool" "$tool" "$impl" "$WORK" "" -- --help
    done
  done
}

repo_case() { # workload pre-args... ; runs workload on every repo, every impl
  local workload=$1; shift
  for r in $REPOS; do
    for impl in $IMPLS; do
      local d="$WORK/$impl/$r"
      bench_case "$workload" "$r" "$impl" "$d" "$(clean_metrics "$d")" -- "$@"
    done
  done
}

phase_crap() { log crap; repo_case crap --no-coverage; }
phase_dry() { log dry; repo_case dry; }
phase_mutate_scan() { log "mutate --scan"; repo_case mutate-scan --scan; }

phase_mutate_full() {
  log "mutate (full, --no-coverage) on fixtures"
  # The runner clones the pristine fixture into .local/bench/runs/<batch>/run-<n>/<fixture> before
  # every run (-fresh), so mutator's snapshots and differential selection never carry over.
  # Those clones are cleared here, by literal path.
  rm -rf .local/bench/runs
  for fx in python go; do
    for impl in $IMPLS; do
      bench_case mutate-full "$fx" "$impl" "$UBMODE/fixtures/$fx" "" -fresh "$UBMODE/fixtures/$fx" -- --no-coverage
    done
  done
}

phase_cover() {
  log "crap --use-existing-coverage on fzf (Go profile)"
  local prof="$BENCH_LOCAL/cover/fzf.coverage.out"
  if [ ! -s "$prof" ]; then
    mkdir -p "$BENCH_LOCAL/cover"
    rm -rf .local/bench/cover/fzf
    cp -c -R "$BENCH_LOCAL/repos/fzf" "$BENCH_LOCAL/cover/fzf"
    (cd "$BENCH_LOCAL/cover/fzf" && go test ./... -coverprofile="$prof" > "$BENCH_LOCAL/cover/fzf.test.log" 2>&1) || echo "fzf go test failed (see cover/fzf.test.log); profile kept if written" >&2
  fi
  for impl in $IMPLS; do
    local d="$WORK/$impl/fzf"
    cp "$prof" "$d/coverage.out"
    bench_case crap-cover fzf "$impl" "$d" "$(clean_metrics "$d")" -- --use-existing-coverage
    rm -f "$d/coverage.out"
  done
}

phase_parity() {
  log parity
  : > "$OUT/parity.jsonl"
  for o in "$OUT"/out/*/*; do
    [ -f "$o/upstream.stdout" ] && [ -f "$o/ubgate.stdout" ] || continue
    local workload repo ra rb
    workload=$(basename "$(dirname "$o")"); repo=$(basename "$o")
    case "$workload" in help-*) continue ;; esac
    ra=$(cat "$o/upstream.root"); rb=$(cat "$o/ubgate.root")
    "$BIN/bench" parity -workload "$workload" -repo "$repo" -artifact stdout \
      -a "$o/upstream.stdout" -b "$o/ubgate.stdout" -mask-a "$ra" -mask-b "$rb" -out "$OUT/parity.jsonl"
    local files ma mb
    ma=$(cat "$o/upstream.metricsdir" 2>/dev/null || true); mb=$(cat "$o/ubgate.metricsdir" 2>/dev/null || true)
    files=$( { [ -n "$ma" ] && (cd "$ma" && find . -type f); [ -n "$mb" ] && (cd "$mb" && find . -type f); true; } | sed 's#^\./##' | sort -u)
    for f in $files; do
      "$BIN/bench" parity -workload "$workload" -repo "$repo" -artifact ".metrics/$f" \
        -a "${ma:-/nonexistent}/$f" -b "${mb:-/nonexistent}/$f" -mask-a "$ra" -mask-b "$rb" -out "$OUT/parity.jsonl"
    done
  done
}

phase_report() {
  log report
  "$BIN/bench" report -order help-crap,help-dry,help-mutate,crap,crap-cover,dry,mutate-scan,mutate-full \
    -in "$OUT/results.jsonl" -parity "$OUT/parity.jsonl" -md "$OUT/summary.md" -csv "$OUT/summary.csv"
  cat "$OUT/summary.md"
}

phase_profile() {
  # A throwaway copy of ubgate's cmd with a pprof wrapper, built under .local (ubgate/ is not touched).
  log "profile: ubgate $PROFILE_CASE"
  local tool=${PROFILE_CASE%% *} repo=${PROFILE_CASE#* }
  local p="$BENCH_LOCAL/profile"
  rm -rf .local/bench/profile/src .local/bench/profile/work
  mkdir -p "$p/src"
  git -C "$UBMODE" archive HEAD ubgate | tar -x -C "$p/src"
  local m="$p/src/ubgate/cmd/ubgate"
  # rename main() and wrap it
  perl -pi -e 's/^func main\(\) \{/func realMain() {/; s/os\.Exit\(/exitHook(/g' "$m/main.go"
  cat > "$m/zz_pprof.go" <<'GO'
package main

import (
	"bytes"
	"os"
	"runtime"
	"runtime/metrics"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"
)

var (
	cpuFile  *os.File
	peakMu   sync.Mutex
	peakHeap uint64
	peakProf bytes.Buffer
	stopPeak = make(chan struct{})
)

// watchPeak snapshots the heap profile whenever live heap objects reach a new
// maximum (sampled every 100ms), so the inuse view shows the peak, not the exit.
func watchPeak() {
	s := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-stopPeak:
			return
		case <-t.C:
			metrics.Read(s)
			if v := s[0].Value.Uint64(); v > peakHeap*11/10 {
				peakMu.Lock()
				peakHeap = v
				peakProf.Reset()
				pprof.Lookup("heap").WriteTo(&peakProf, 0)
				peakMu.Unlock()
			}
		}
	}
}

func stopProfiles() {
	close(stopPeak)
	if cpuFile != nil {
		pprof.StopCPUProfile()
		cpuFile.Close()
	}
	if p := os.Getenv("UBGATE_MEMPROFILE"); p != "" {
		f, _ := os.Create(p)
		pprof.Lookup("allocs").WriteTo(f, 0)
		f.Close()
	}
	if p := os.Getenv("UBGATE_HEAPPROFILE"); p != "" {
		peakMu.Lock()
		os.WriteFile(p, peakProf.Bytes(), 0o644)
		peakMu.Unlock()
		os.WriteFile(p+".peak", []byte(fmtMiB(peakHeap)), 0o644)
	}
}

func fmtMiB(b uint64) string {
	return strconv.FormatUint(b>>20, 10) + " MiB live heap objects at the peak sample\n"
}

func exitHook(code int) { stopProfiles(); os.Exit(code) }

func main() {
	runtime.MemProfileRate = 64 * 1024
	if p := os.Getenv("UBGATE_CPUPROFILE"); p != "" {
		cpuFile, _ = os.Create(p)
		pprof.StartCPUProfile(cpuFile)
	}
	go watchPeak()
	realMain()
	stopProfiles()
}
GO
  (cd "$p/src/ubgate" && go build -o "$p/ubgate-pprof" ./cmd/ubgate)
  local c="$p/$tool-$repo" d="$p/work/$repo" args=()
  mkdir -p "$c" "$p/work"
  cp -c -R "$BENCH_LOCAL/repos/$repo" "$d" 2>/dev/null || cp -R "$BENCH_LOCAL/repos/$repo" "$d"
  [ "$tool" = crap ] && args=(--no-coverage)
  [ "$tool" = mutate ] && args=(--scan)
  (cd "$d" && UBGATE_CPUPROFILE="$c/cpu.pprof" UBGATE_MEMPROFILE="$c/allocs.pprof" UBGATE_HEAPPROFILE="$c/heap.pprof" \
    "$p/ubgate-pprof" "$tool" ${args[@]+"${args[@]}"} > /dev/null)
  go tool pprof -top -nodecount=25 "$p/ubgate-pprof" "$c/cpu.pprof" > "$c/cpu.top.txt" 2>/dev/null
  go tool pprof -top -cum -nodecount=40 "$p/ubgate-pprof" "$c/cpu.pprof" > "$c/cpu.cum.txt" 2>/dev/null
  go tool pprof -top -sample_index=alloc_space -nodecount=25 "$p/ubgate-pprof" "$c/allocs.pprof" > "$c/alloc_space.top.txt" 2>/dev/null
  go tool pprof -top -sample_index=inuse_space -nodecount=20 "$p/ubgate-pprof" "$c/heap.pprof" > "$c/inuse.top.txt" 2>/dev/null
  head -35 "$c/cpu.top.txt"; head -30 "$c/alloc_space.top.txt"
}

phases=("$@")
[ ${#phases[@]} -eq 0 ] && phases=(fetch build env help crap dry mutate-scan mutate-full cover parity report)
for ph in "${phases[@]}"; do
  case "$ph" in
    fetch | build | env | help | crap | dry | cover | parity | report | profile) "phase_$ph" ;;
    mutate-scan) phase_mutate_scan ;;
    mutate-full) phase_mutate_full ;;
    *) echo "unknown phase: $ph" >&2; exit 2 ;;
  esac
done
