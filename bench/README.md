# coderefinery benchmarks

Times `coderefinery` against Uncle Bob's Python originals (crapper, dryer, mutator)
on pinned open-source repos: wall time, CPU time (user+sys), peak memory (max
RSS), and whether the outputs still match. Results and analysis:
[RESULTS.md](RESULTS.md).

Everything stays inside `/Users/matheusale/workspaces/ub-mode`: repos are
cloned under `.local/bench/repos`, work copies live in `.local/bench/work`,
results in `.local/bench/results/<name>` (all gitignored), and every script
sources `harness/env.sh`.

```
bench/
  repos.txt        corpus: name, URL, pinned SHA, language, size class, sparse subtree
  fetch.sh         shallow, pinned clones (partial + sparse for giant repos)
  run.sh           matrix driver (phases below)
  cmd/bench/       stdlib-only Go runner: run / report / parity / count
  RESULTS.md       the write-up of the last full run
```

## Rerun

Prerequisites: the upstream tools and their venvs (`harness/fetch-upstream.sh`
if `upstream/` is missing) and `fixtures/setup.sh` (for the mutate-full phase).

```bash
cd /Users/matheusale/workspaces/ub-mode
OUT=.local/bench/results/$(date +%Y%m%d) bench/run.sh          # everything (about 1.5 h on an M4, dominated by dry)
bench/run.sh build crap report                                  # one workload into .local/bench/results/latest
REPOS="flask fzf" RUNS=5 bench/run.sh crap dry parity report    # a subset
PROFILE_CASE="dry typescript" bench/run.sh profile              # CPU + heap profile of coderefinery
```

Phases, in default order: `fetch build env help crap dry mutate-scan
mutate-full cover parity report` (plus `profile`, not in the default set).
Each timing phase appends to `$OUT/results.jsonl`; rerunning a case appends a
new batch and the report keeps the latest batch per case.

| phase | what is timed |
|---|---|
| `help` | `--help` of each tool (startup cost), 2 warm-up + 10 runs |
| `crap` | `crap --no-coverage`: discovery, parsing, complexity, report. No test suite |
| `dry` | `dry` with default flags |
| `mutate-scan` | `mutate --scan`: site discovery and naming, no test runs |
| `mutate-full` | full `mutate --no-coverage` on `fixtures/python` and `fixtures/go`: the tool's overhead around the fixtures' own tests |
| `cover` | `crap --use-existing-coverage` on fzf with a Go profile produced once, untimed |

Knobs (environment variables): `RUNS=3`, `WARMUP=1`, `TIMEOUT=30m`,
`SLOW=120s` (a warm-up slower than this becomes the single measured run),
`MAX_LOAD=4` / `LOAD_WAIT=3m` (wait for the 1-minute load average before
each run, then run anyway and record it), `REPOS`, `IMPLS`, `OUT`,
`CODEREFINERY_SRC=worktree` (benchmark the working tree instead of a `git archive`
of `HEAD`).

Outputs in `$OUT`: `results.jsonl` (one line per run), `parity.jsonl`,
`summary.md`, `summary.csv`, `env.txt`, `counts.jsonl`, and
`out/<workload>/<repo>/{upstream,coderefinery}.{stdout,stderr,root,metricsdir,metrics.<stamp>/}`
from the last run of each case.

## How a run is measured

`bench run` starts the command in its own process group, waits for it with
`wait4` (Go's `ProcessState.SysUsage()`), and records:

* **wall**: `time.Now` around start/wait.
* **CPU**: `ru_utime + ru_stime` from the child's rusage. On macOS and Linux
  this includes every descendant the child waited for (pytest, `go test` in
  mutate-full), so it is the CPU of the whole process tree. Orphaned
  grandchildren that are never waited for are not counted. The runner's own
  `RUSAGE_CHILDREN` delta is recorded as `children_cpu_s` as a cross-check.
* **max RSS**: `ru_maxrss`, bytes on macOS and KiB on Linux, normalised to
  bytes. This is the peak of the **single largest process** in the tree, not
  a sum: rusage merges children with `max`, not `+`. For single-process
  workloads (everything except mutate-full) that is exactly the tool's peak.
  For mutate-full it is the largest of mutator/coderefinery and the test processes.
* **load**: 1- and 5-minute load average just before the run.

Each case gets a warm-up run that is recorded but excluded from the stats
(it warms the OS page cache and Python's `__pycache__`), then `RUNS`
measured runs; the report shows the median and min–max. Inputs are restored
before every run (untimed), and nothing is deleted by a variable path:

* crap, dry, mutate-scan run in a persistent per-implementation APFS clone
  (`.local/bench/work/<impl>/<repo>`). The `-pre` step removes exactly the
  files those tools write, `.metrics/crap.edn` and `.metrics/dry.edn`, with a
  non-recursive `rm -f` (mutate `--scan` writes nothing), so snapshots never
  carry over.
* mutate-full uses `bench run -fresh fixtures/<fx>`: the runner clones the
  pristine fixture into `.local/bench/runs/<batch>/run-<n>/<fx>` before every
  run. The runner never deletes anything. `run.sh` clears
  `.local/bench/runs` with one literal `rm -rf` at the start of the phase.
* The last run's `.metrics` is copied to a new
  `out/<workload>/<repo>/<impl>.metrics.<stamp>/` per batch, and
  `<impl>.metricsdir` points at it.
* The only recursive deletes in `bench/` are literal paths under
  `.local/bench/` (`work`, `src`, `runs`, `cover/fzf`, `profile/src`,
  `profile/work`), run after `cd` to the repo root. `fetch.sh` updates
  existing checkouts in place.

Each implementation runs on its own copy, upstream first, then coderefinery, never
at the same time. The load wait happens once per case (`-wait-each` makes it
wait before every run).

## Parity

`bench parity` compares upstream and coderefinery output of the last run of each
case: stdout and every file under `.metrics/`, byte for byte after masking
each side's root path as `<ROOT>`. A mismatch also reports a line-level
match rate (multiset intersection / longer side) and the first differing
lines. mutator's `:tested-at` timestamp is masked (as the harness does;
`-keep-tested-at` turns that off). The fixture parity gate with structural EDN
comparison remains `harness/run.sh`.
