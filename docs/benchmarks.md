# ubgate benchmarks

This is the permanent record of how `ubgate` was benchmarked against Uncle Bob's
Python originals and what each run found. `ubgate` (`ubgate/`) is the stdlib-only
Go port. The originals are `crapper`, `dryer` and `mutator` in `upstream/`
(Python 3.12 + py-tree-sitter).

* Harness: [`bench/`](../bench/) ([README](../bench/README.md), corpus in
  [`repos.txt`](../bench/repos.txt), driver [`run.sh`](../bench/run.sh),
  clones via [`fetch.sh`](../bench/fetch.sh), runner in
  [`bench/cmd/bench/`](../bench/cmd/bench/)).
* Write-up of the last full run: [`bench/RESULTS.md`](../bench/RESULTS.md).
* Parser divergences found by the benchmarks:
  [`ubgate/docs/PARSER-BUGS.md`, "Found by benchmarks"](../ubgate/docs/PARSER-BUGS.md#found-by-benchmarks).
* Raw data: `.local/bench/results/<YYYYMMDD>/` (gitignored).

Sections 1–3 describe the method and change only when the harness changes.
Sections 4–6 hold the **current baseline** (the 2026-11-03 run). Every run,
including the baseline and any run after an optimisation, gets a dated entry
in [History](#8-history). New entries are appended there; earlier ones are
never overwritten.

---

## 1. Goal and questions answered

Before `ubgate` replaces the Python tools, we want to know whether it is at
least as good on real, large codebases, not just on the parity fixtures:

1. **Speed**: wall time per workload, per repo, and at startup.
2. **CPU**: total work done (user+sys), which matters on shared machines and
   in CI.
3. **Memory**: peak resident set size (max RSS).
4. **Output parity at scale**: do both tools produce byte-identical stdout and
   `.metrics/*` on large real repos? The fixture harness (`harness/run.sh`)
   already gates parity on small, curated inputs. This benchmark checks it on
   real ones.
5. **Where the time goes** in ubgate (profiling), so optimisation targets come
   from measurements rather than guesses.

## 2. Strategy and methodology

### 2.1 Repo selection

* **Language coverage.** Two repos for each language both tools support:
  Python, TypeScript, JavaScript, Go, Java and Rust. That makes 12 repos.
* **Medium vs large.** Each language gets one *medium* repo (roughly 10–50 k
  lines analysed: flask, axios, fzf, gson, ripgrep, excalidraw) and one *large*
  repo (130–380 k lines analysed: django, TypeScript, webpack, hugo, guava,
  tokio). The pair shows how each tool scales, which matters most for dry's
  O(n²) pair loop.
* **Subtrees for giants.** Monorepos are sparse-checked-out to the
  production source tree, so runs stay tractable and focused on real code:
  TypeScript `src/`, webpack `lib/`, guava `guava/src/`, gson `gson/src/main/`,
  axios `lib/`. Root files (`package.json`, `go.mod`, …) are always present.
  The tools' own `SKIP_DIRS` / `TEST_DIRS` / test-file rules then exclude
  tests, `node_modules`, `vendor`, `testdata`, etc.
* **Pinned commits.** Every repo is pinned to a full SHA in `bench/repos.txt`
  and fetched shallow (`git fetch --depth 1 origin <sha>`). Sparse repos use
  partial clones (`--filter=blob:none`). **TypeScript is pinned to tag
  v6.0.3** (`050880ce59e3`), the last release written in TypeScript: the
  `main` branch is now the Go port, so it would no longer be a TypeScript
  workload.
* **JavaScript and dry.** dryer has no `.js/.jsx/.mjs/.cjs` support, so dry on
  webpack and axios only walks the tree (0 and 1 dry files). Those cases
  measure startup and directory walking only, and they are kept on purpose.

### 2.2 Workloads, and why

| phase | command | why |
|---|---|---|
| `help` | `--help` for each tool | Startup cost: interpreter, imports, binary load. 2 warm-ups + 10 runs. |
| `crap` | `crap --no-coverage` | Discovery + parse + complexity + report, with no test suite involved. |
| `cover` | `crap --use-existing-coverage` on fzf | Exercises the coverage-reading path. The Go profile is made once, untimed (`go test ./... -coverprofile`). |
| `dry` | `dry` (default flags) | The full duplicate-detection pipeline, including the pair loop. |
| `mutate-scan` | `mutate --scan` | Mutation-site discovery and naming without running any tests. Writes nothing. |
| `mutate-full` | `mutate --no-coverage` on `fixtures/python` and `fixtures/go` | One end-to-end mutation run to show the tool's overhead around the test runs. |

**Excluded on purpose:**

* Coverage generation (`crap` without `--no-coverage`) and full mutation on
  the big repos. Both run the project's own test suite (once per mutant for
  mutate), so they would time pytest / `go test` / cargo / Maven rather than
  the tool. Full mutation only runs on the small fixtures, where it confirms
  that the tool adds no overhead.
* `--changed` (needs a git history setup per repo).

### 2.3 Runner design (`bench/cmd/bench`, stdlib-only Go)

* **Process isolation.** Each run starts the command in its own process group
  and waits for it with `wait4` (Go's `ProcessState.SysUsage()`).
* **wall** = monotonic clock around start/wait.
* **CPU** = `ru_utime + ru_stime` from the child's rusage. This includes every
  descendant the child waited for (pytest / `go test` in mutate-full). The
  runner's own `RUSAGE_CHILDREN` delta is recorded as `children_cpu_s` for
  cross-checking, and it matched exactly in every run of the baseline.
  Orphaned grandchildren that are never waited for would not be counted.
* **max RSS** = `ru_maxrss`, normalised to bytes. macOS reports **bytes** and
  Linux reports **KiB**. This is the peak of the **single largest process** in
  the tree, not a sum, because rusage merges children with `max`. For every
  workload except mutate-full that is the tool's own peak. For mutate-full it
  is the largest of the tool and its test processes, which in practice is the
  test process.
* **Warm-up + N runs.** Each case gets `WARMUP=1` run that is recorded but
  excluded (it warms the OS page cache and the upstream `__pycache__`),
  followed by `RUNS=3` measured runs. The report gives the **median** and the
  min–max. Startup uses 2 warm-ups + 10 runs. ubgate keeps no persistent
  cache, so "warm" only refers to the OS page cache. A truly cold cache
  (`sudo purge`) was not tested.
* **Speedup** = upstream median wall / ubgate median wall. **CPU ratio** and
  **mem ratio** are upstream / ubgate, so values > 1 favour ubgate. The
  per-workload summary is the **geometric mean** of the per-repo speedups,
  with min and max. "Total" speedups divide sums of per-case medians.
* **Ordering.** For each case, upstream runs first and ubgate right after,
  never both at once, so both see similar machine conditions.
* **Timeouts.** `TIMEOUT=30m` per run (the process group is killed and the
  run is reported as `> timeout`). `SLOW=120s`: a warm-up slower than this
  becomes the single measured run (marked `1*`). In the baseline nothing
  timed out and no case hit `SLOW` (the slowest warm-up, upstream dry guava,
  took 85 s).
* **Load gating.** Before each case the runner waits while the 1-minute load
  average is above `MAX_LOAD=4`, up to `LOAD_WAIT`, then runs anyway. It
  records `load1` / `load5` / `load_waited_s` for every run. (The baseline
  used a 20 s wait, and the load never fell below 4.)
* **ubgate build.** `git archive HEAD ubgate` is extracted under
  `.local/bench/src` and built with `go build -trimpath`, so concurrent edits
  in the working tree do not leak in. `UBGATE_SRC=worktree` benchmarks the
  working tree instead and records the dirty-file count.

### 2.4 Fresh inputs between runs

* Each implementation has its own APFS clone of every repo
  (`.local/bench/work/<impl>/<repo>`), recreated by the `fetch` phase.
* An untimed `-pre` step before **every** run removes exactly the files the
  tools write (`.metrics/crap.edn`, `.metrics/dry.edn`, then `rmdir .metrics`)
  with a non-recursive `rm -f`. Snapshots and history therefore never carry
  over between runs. `mutate --scan` writes nothing (verified: no `.metrics/`
  after a scan).
* mutate-full uses `bench run -fresh`: before each run the runner clones the
  pristine fixture into `.local/bench/runs/<batch>/run-<n>/`, so mutator's
  snapshots and differential selection start from zero every time.
* The `.metrics/` of the last run of each case is copied to
  `out/<workload>/<repo>/<impl>.metrics.<stamp>/` for the parity check.
* The only recursive deletes are of literal paths under `.local/bench/`.

### 2.5 Shared-machine noise

The baseline ran on a shared developer machine. Other agents were building
and testing, and OrbStack, Chrome, Spotify and `mediaanalysisd` were active.

* The 1-minute load during the kept runs was **4.1–13.5**, above the target
  of 4.
* One **burst to load ≈ 130** (an Android emulator plus media analysis) hit
  the first `mutate --scan` pass for axios, hugo, fzf, guava, gson, tokio and
  ripgrep, and the first mutate-full python pass. Those cases were **rerun**
  once the load was back to 6–8, and the report keeps only the latest batch
  per case. The reruns waited 50–150 s for the load to drop.
* Every tool except mutate-full is single-threaded, so one of the 10 cores was
  usually free. But the M4 has only 4 performance cores, and the scheduler can
  move a run onto an efficiency core. That cannot be pinned from user space,
  and it shows up in the min–max ranges (for example ubgate crap tokio
  374–764 ms).
* **Expect about ±10% noise per case**, and more on sub-100 ms cases. Ratios
  hold up better than absolute times because both sides ran back to back.

### 2.6 Parity check

`bench parity` compares the last run of each case on both sides: stdout and
every file under `.metrics/`, **byte for byte**. Before comparing, it masks
each side's root path as `<ROOT>` (including `/private` and symlink-resolved
forms) and masks mutator's `:tested-at "…"` timestamp. On a mismatch it
reports a line-level match rate (multiset intersection / longer side) and the
first differing lines. Each mismatch was then minimised to a small repro and
checked against tree-sitter with `go run ./tools/treediff FILE`. This is
stricter than the fixture gate in `harness/run.sh`, which compares EDN
structurally.

### 2.7 Profiling method

`bench/run.sh profile` (case chosen with `PROFILE_CASE`, default
`"dry typescript"`) copies `ubgate/cmd/ubgate` to `.local/bench/profile/src`
and adds a throwaway `zz_pprof.go` wrapper. `ubgate/` itself is not modified.
The wrapper is driven by `UBGATE_CPUPROFILE`, `UBGATE_MEMPROFILE` and
`UBGATE_HEAPPROFILE`:

* a CPU profile for the whole run;
* an `allocs` profile (total allocation by site);
* a heap **inuse** snapshot taken whenever live heap objects reach a new
  peak, sampled every 100 ms. This gives a picture of what is live at peak
  memory.

`go tool pprof -top` / `-top -cum` / `-sample_index=alloc_space` /
`-sample_index=inuse_space` output and the raw profiles go to
`.local/bench/profile/<tool>-<repo>/`. The baseline profiled `dry-typescript`,
`dry-django`, `mutate-django`, `crap-typescript` and `crap-ripgrep`.

## 3. Environment (baseline run, 2026-11-03)

| | |
|---|---|
| Date | 2026-11-03, 19:27–20:54 UTC (timed phases, reruns included) |
| CPU | Apple M4, 10 cores (4 performance + 6 efficiency) |
| RAM | 32 GiB |
| OS | macOS 26.6.1 (Darwin 25.6.0, arm64) |
| Go | 1.26.1 darwin/arm64 |
| ubgate | `12becbcaf598488a9bef6eaf5f5d9dedec9899b0` (`git archive HEAD`, `-trimpath`) |
| Python | 3.12.3 (upstream venvs) |
| tree-sitter (upstream) | py-tree-sitter 0.26.0, tree-sitter-language-pack 1.23.1 |
| upstream | crapper `6fd92c4`, dryer `66ff6d2`, mutator `a5a0cb8` |
| Load at start | 11.77 / 12.60 / 12.78 |
| Raw data | `.local/bench/results/20261103/` (`results.jsonl` 448 runs, `parity.jsonl` 70 artifacts, `summary.md`, `summary.csv`, `env.txt`, `counts.jsonl`, `out/`) |

### Corpus (pinned)

"analysed files" = what the tools walk after their own pruning (`bench count`).
"dry files" excludes JavaScript (dryer has no JS support).

| repo | upstream | pinned SHA | language | size | checkout | analysed files | lines | non-blank | dry files |
|---|---|---|---|---|---|---:|---:|---:|---:|
| django | [django/django](https://github.com/django/django) | `2382f40aed50332d2325563ea9097853561d45e8` | python | large | whole repo | 943 (python 921, javascript 22) | 167,564 | 144,922 | 921 |
| flask | [pallets/flask](https://github.com/pallets/flask) | `49943b5a94413a530761350f04e896f76e8d620e` | python | medium | whole repo | 35 (python 35) | 10,139 | 8,051 | 35 |
| typescript | [microsoft/TypeScript](https://github.com/microsoft/TypeScript) | `050880ce59e30b356b686bd3144efe24f875ebc8` (v6.0.3) | typescript | large | `src/` | 604 (typescript 601, javascript 3) | 380,278 | 347,187 | 601 |
| excalidraw | [excalidraw/excalidraw](https://github.com/excalidraw/excalidraw) | `dd2f8fc32b92cbf56f4abee6a0cf3b14681b4b55` | typescript | medium | whole repo | 549 (typescript 521, javascript 28) | 154,718 | 139,926 | 521 |
| webpack | [webpack/webpack](https://github.com/webpack/webpack) | `0b35ae904f20174d3be375a44822c9e4aedb1221` | javascript | large | `lib/` | 818 (javascript 818) | 339,946 | 318,686 | 0 |
| axios | [axios/axios](https://github.com/axios/axios) | `fb88a1b133c1a2d774713070b8ca25f7bcd382ac` | javascript | medium | `lib/` | 79 (javascript 78, typescript 1) | 10,755 | 9,218 | 1 |
| hugo | [gohugoio/hugo](https://github.com/gohugoio/hugo) | `fa5a3d48b8dcf6c0bfcd7d06726a6530b2efc174` | go | large | whole repo | 542 (go 519, javascript 23) | 133,747 | 114,612 | 519 |
| fzf | [junegunn/fzf](https://github.com/junegunn/fzf) | `99bfd5aa0f66133d7dcfbf18f8044eb765f04649` | go | medium | whole repo | 60 (go 60) | 27,262 | 25,190 | 60 |
| guava | [google/guava](https://github.com/google/guava) | `2bcd693991a6386cc22f946c9b18359ccd07e80e` | java | large | `guava/src/` | 611 (java 611) | 182,525 | 164,002 | 611 |
| gson | [google/gson](https://github.com/google/gson) | `57731049da3e415be3d5d34db1ca3b80f3c73810` | java | medium | `gson/src/main/` | 87 (java 87) | 19,538 | 17,834 | 87 |
| tokio | [tokio-rs/tokio](https://github.com/tokio-rs/tokio) | `f67444986f673811950c1c3cdb0761eedc8a88ff` | rust | large | whole repo | 509 (rust 509) | 131,535 | 118,613 | 509 |
| ripgrep | [BurntSushi/ripgrep](https://github.com/BurntSushi/ripgrep) | `91f5d22ef48bde459d90a8c7dbeab416e97d0f2b` | rust | medium | whole repo | 87 (rust 87) | 49,037 | 44,625 | 87 |

## 4. Results (baseline, ubgate `12becbc`)

All numbers are copied as measured from `.local/bench/results/20261103/summary.md`.

### 4.1 Headline

Totals over the 12 repos (sum of per-case medians), plus startup and the fixture runs:

| workload | upstream wall | ubgate wall | speedup (total) | geomean speedup (range) | upstream CPU | ubgate CPU | max RSS upstream → ubgate (typical) | parity |
|---|---:|---:|---:|---:|---:|---:|---|---|
| startup (`--help`) | 46–83 ms | 3–5 ms | 10–30× | – | 36–56 ms | 2–4 ms | 17–23 MiB → 5 MiB | – |
| crap `--no-coverage` | 13.6 s | 3.9 s | 3.5× | 3.78× (2.05–8.13×) | 12.9 s | 4.7 s | 23–168 → 12–125 MiB (ubgate 1.2–2.1× less, guava 0.93×) | 11/12 identical |
| crap `--use-existing-coverage` (fzf) | 463 ms | 134 ms | 3.5× | 3.47× | 452 ms | 162 ms | 43 → 33 MiB | identical |
| dry | 313.5 s | 162.8 s | 1.9× | 2.89× (1.29–10.2×) | 296 s | 157 s | django/excalidraw/hugo/guava/tokio: ubgate 1.2–1.3× **more** (e.g. hugo 128 → 155 MiB); typescript 364 → 346; small repos less | 11/12 identical |
| mutate `--scan` | 136.2 s | 30.5 s | 4.5× | 5.03× (1.42–13.1×) | 131 s | 32 s | 29–172 → 14–161 MiB (1.07–2.3× less) | 11/12 identical |
| mutate full, `--no-coverage` (fixtures) | 17.3 s | 17.6 s | 0.98× | 0.95× (0.91–0.99×) | 29.8 s | 29.5 s | same (the peak is the test process) | 2/2 identical |

Per-workload geometric mean of wall-time speedups:

| workload | cases | geomean speedup | min | max |
|---|---:|---:|---:|---:|
| help-crap | 1 | 29.7× | 29.7× | 29.7× |
| help-dry | 1 | 10.1× | 10.1× | 10.1× |
| help-mutate | 1 | 17.3× | 17.3× | 17.3× |
| crap | 12 | 3.78× | 2.05× | 8.13× |
| crap-cover | 1 | 3.47× | 3.47× | 3.47× |
| dry | 12 | 2.89× | 1.29× | 10.2× |
| mutate-scan | 12 | 5.03× | 1.42× | 13.1× |
| mutate-full | 2 | 0.95× | 0.91× | 0.99× |

Takeaways:

* ubgate is faster in every case except full mutation, which is bounded by the
  fixtures' own pytest / `go test` runs (one per mutant). There the two tools
  tie within noise.
* The biggest wins are startup (10–30×, no interpreter or imports) and
  `mutate --scan` on TS/JS (13.1× TypeScript, 9.12× webpack). The smallest are
  dry TypeScript (1.29×) and `mutate --scan` django (1.42×). Both are
  algorithmic hotspots inherited 1:1 from upstream (see §6).
* **Memory:** ubgate is lower on crap and `mutate --scan`, thanks to a 5 MiB
  floor against Python's 17–23 MiB. On **dry it is higher for most large
  repos** (mem ratio 0.77–0.84×; TypeScript about even at 1.05×) because of the
  `map[string]struct{}` fingerprint sets.
* **CPU vs wall:** ubgate's CPU is often a little above its wall time (GC runs
  on other cores), while upstream's single Python thread stays just under. The
  CPU ratio is therefore slightly lower than the speedup, and it is the fairer
  measure of total work.
* Warm-up vs measured median: 1.06× for ubgate and 1.09× for upstream
  (median). The largest outliers are ubgate crap tokio (2.3×) and django
  (1.6×) on first touch of the work copy.

### 4.2 Startup (`--help`, 2 warm-ups + 10 runs)

| tool | upstream wall (min–max) | ubgate wall (min–max) | speedup | upstream CPU | ubgate CPU | upstream RSS | ubgate RSS | mem ratio | load1 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|
| crap | 83 ms (44–152 ms) | 3 ms (3–6 ms) | 29.7× | 56 ms | 2 ms | 19 MiB | 5 MiB | 3.46× | 8.9–10.2 |
| dry | 46 ms (35–72 ms) | 5 ms (3–8 ms) | 10.1× | 36 ms | 4 ms | 17 MiB | 5 MiB | 3.15× | 9.2–11.4 |
| mutate | 52 ms (49–73 ms) | 3 ms (3–5 ms) | 17.3× | 46 ms | 2 ms | 23 MiB | 5 MiB | 4.21× | 9.1–10.0 |

### 4.3 crap `--no-coverage` (1 warm-up + 3 runs)

| repo | upstream wall (min–max) | ubgate wall (min–max) | speedup | upstream CPU | ubgate CPU | CPU ratio | upstream RSS | ubgate RSS | mem ratio | load1 | parity |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| django | 1.25 s (1.25–1.30 s) | 420 ms (389–485 ms) | 2.99× | 1.23 s | 538 ms | 2.28× | 36 MiB | 21 MiB | 1.73× | 8.1–9.6 | identical (2/2) |
| flask | 150 ms (139–160 ms) | 21 ms (19–38 ms) | 7.17× | 128 ms | 23 ms | 5.58× | 24 MiB | 13 MiB | 1.89× | 9.0–10.1 | identical (2/2) |
| typescript | 3.90 s (3.75–4.26 s) | 907 ms (883–1.43 s) | 4.30× | 3.70 s | 1.12 s | 3.30× | 168 MiB | 125 MiB | 1.34× | 8.9–11.2 | 0/2 identical; stdout 99.3%, crap.edn 99.3% |
| excalidraw | 1.30 s (1.25–1.33 s) | 340 ms (337–534 ms) | 3.82× | 1.25 s | 391 ms | 3.19× | 45 MiB | 29 MiB | 1.54× | 10.0–11.2 | identical (2/2) |
| webpack | 2.64 s (2.55–3.25 s) | 603 ms (570–693 ms) | 4.37× | 2.58 s | 743 ms | 3.47× | 75 MiB | 52 MiB | 1.44× | 9.2–10.8 | identical (2/2) |
| axios | 145 ms (140–154 ms) | 26 ms (25–30 ms) | 5.61× | 139 ms | 27 ms | 5.17× | 26 MiB | 12 MiB | 2.12× | 10.9–13.5 | identical (2/2) |
| hugo | 1.29 s (1.21–1.30 s) | 405 ms (360–506 ms) | 3.19× | 1.22 s | 458 ms | 2.67× | 67 MiB | 43 MiB | 1.56× | 11.2–11.6 | identical (2/2) |
| fzf | 442 ms (423–493 ms) | 54 ms (52–58 ms) | 8.13× | 357 ms | 74 ms | 4.84× | 42 MiB | 32 MiB | 1.31× | 11.2–11.6 | identical (2/2) |
| guava | 863 ms (838–917 ms) | 418 ms (383–448 ms) | 2.07× | 849 ms | 542 ms | 1.57× | 34 MiB | 36 MiB | 0.93× | 10.3–12.1 | identical (2/2) |
| gson | 138 ms (134–169 ms) | 37 ms (36–40 ms) | 3.71× | 132 ms | 40 ms | 3.31× | 23 MiB | 13 MiB | 1.76× | 10.7–10.9 | identical (2/2) |
| tokio | 1.02 s (862–1.07 s) | 499 ms (374–764 ms) | 2.05× | 964 ms | 535 ms | 1.80× | 30 MiB | 17 MiB | 1.79× | 9.4–9.7 | identical (2/2) |
| ripgrep | 438 ms (386–487 ms) | 179 ms (174–266 ms) | 2.45× | 387 ms | 227 ms | 1.71× | 38 MiB | 30 MiB | 1.23× | 9.3–10.2 | identical (2/2) |

crap `--use-existing-coverage` on fzf: 463 ms (457–614 ms) → 134 ms (102–175 ms),
3.47×. CPU 452 ms → 162 ms (2.79×), RSS 43 → 33 MiB (1.29×), load1 9.0–9.4,
identical (2/2).

### 4.4 dry (default flags, 1 warm-up + 3 runs)

| repo | upstream wall (min–max) | ubgate wall (min–max) | speedup | upstream CPU | ubgate CPU | CPU ratio | upstream RSS | ubgate RSS | mem ratio | load1 | parity |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| django | 64.30 s (63.65–65.54 s) | 28.11 s (26.57–30.65 s) | 2.29× | 61.40 s | 26.30 s | 2.33× | 99 MiB | 127 MiB | 0.78× | 4.9–13.2 | identical (2/2) |
| flask | 306 ms (290–316 ms) | 85 ms (84–108 ms) | 3.58× | 288 ms | 91 ms | 3.15× | 25 MiB | 15 MiB | 1.61× | 6.2–6.9 | identical (2/2) |
| typescript | 78.34 s (74.73–85.53 s) | 60.92 s (60.75–61.45 s) | 1.29× | 73.83 s | 58.86 s | 1.25× | 364 MiB | 346 MiB | 1.05× | 5.8–9.3 | identical (2/2) |
| excalidraw | 19.90 s (19.44–20.31 s) | 8.43 s (8.09–9.18 s) | 2.36× | 18.20 s | 8.12 s | 2.24× | 112 MiB | 139 MiB | 0.80× | 5.7–7.3 | identical (2/2) |
| webpack | 45 ms (42–50 ms) | 7 ms (7–8 ms) | 6.09× | 38 ms | 6 ms | 6.94× | 17 MiB | 6 MiB | 2.89× | 8.4–9.3 | identical (1/1) |
| axios | 66 ms (53–84 ms) | 6 ms (5–7 ms) | 10.2× | 58 ms | 6 ms | 10.0× | 23 MiB | 8 MiB | 3.05× | 6.7–7.9 | identical (2/2) |
| hugo | 35.67 s (35.50–38.44 s) | 16.02 s (15.97–16.40 s) | 2.23× | 34.00 s | 15.77 s | 2.16× | 128 MiB | 155 MiB | 0.82× | 6.7–9.2 | identical (2/2) |
| fzf | 1.44 s (1.32–1.54 s) | 548 ms (538–759 ms) | 2.63× | 1.39 s | 578 ms | 2.41× | 70 MiB | 57 MiB | 1.24× | 6.2–6.7 | identical (2/2) |
| guava | 89.95 s (88.44–104 s) | 38.83 s (37.16–39.09 s) | 2.32× | 84.45 s | 37.19 s | 2.27× | 104 MiB | 134 MiB | 0.77× | 5.4–6.8 | 0/2 identical; stdout 100.0%, dry.edn 100.0% |
| gson | 755 ms (739–1.01 s) | 297 ms (245–299 ms) | 2.54× | 726 ms | 285 ms | 2.55× | 30 MiB | 22 MiB | 1.33× | 4.9–5.2 | identical (2/2) |
| tokio | 18.52 s (18.05–18.68 s) | 8.03 s (7.87–8.63 s) | 2.31× | 17.65 s | 7.84 s | 2.25× | 72 MiB | 86 MiB | 0.84× | 5.3–6.1 | identical (2/2) |
| ripgrep | 4.14 s (3.74–4.22 s) | 1.51 s (1.42–1.76 s) | 2.74× | 3.94 s | 1.56 s | 2.52× | 55 MiB | 52 MiB | 1.07× | 4.9–5.4 | identical (2/2) |

### 4.5 mutate `--scan` (1 warm-up + 3 runs)

| repo | upstream wall (min–max) | ubgate wall (min–max) | speedup | upstream CPU | ubgate CPU | CPU ratio | upstream RSS | ubgate RSS | mem ratio | load1 | parity |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| django | 16.62 s (16.19–16.72 s) | 11.71 s (11.20–14.93 s) | 1.42× | 15.90 s | 11.67 s | 1.36× | 34 MiB | 21 MiB | 1.64× | 4.2–6.1 | identical (1/1) |
| flask | 359 ms (318–377 ms) | 86 ms (85–104 ms) | 4.19× | 325 ms | 92 ms | 3.54× | 28 MiB | 14 MiB | 2.10× | 6.4–6.6 | identical (1/1) |
| typescript | 38.62 s (38.51–38.80 s) | 2.94 s (2.92–3.74 s) | 13.1× | 37.59 s | 3.41 s | 11.0× | 172 MiB | 161 MiB | 1.07× | 4.1–6.1 | 0/1 identical; stdout 99.6% |
| excalidraw | 12.59 s (12.47–12.74 s) | 2.23 s (2.13–2.27 s) | 5.66× | 12.16 s | 2.27 s | 5.35× | 49 MiB | 27 MiB | 1.79× | 4.5–6.5 | identical (1/1) |
| webpack | 38.24 s (37.93–38.67 s) | 4.19 s (4.09–4.29 s) | 9.12× | 36.76 s | 4.47 s | 8.23× | 78 MiB | 62 MiB | 1.27× | 5.2–10.2 | identical (1/1) |
| axios | 827 ms (806–927 ms) | 120 ms (118–147 ms) | 6.92× | 819 ms | 125 ms | 6.57× | 31 MiB | 14 MiB | 2.24× | 8.0–8.0 | identical (1/1) |
| hugo | 13.24 s (13.08–14.43 s) | 5.58 s (5.51–5.81 s) | 2.37× | 12.60 s | 5.71 s | 2.21× | 71 MiB | 52 MiB | 1.36× | 6.7–8.1 | identical (1/1) |
| fzf | 1.38 s (1.33–1.38 s) | 211 ms (201–253 ms) | 6.57× | 1.37 s | 256 ms | 5.34× | 49 MiB | 34 MiB | 1.43× | 6.3–6.6 | identical (1/1) |
| guava | 5.65 s (5.57–5.80 s) | 747 ms (731–938 ms) | 7.56× | 5.33 s | 905 ms | 5.89× | 38 MiB | 34 MiB | 1.14× | 6.3–6.8 | identical (1/1) |
| gson | 484 ms (480–499 ms) | 86 ms (82–94 ms) | 5.64× | 477 ms | 96 ms | 4.99× | 29 MiB | 14 MiB | 2.02× | 6.7–6.8 | identical (1/1) |
| tokio | 5.69 s (5.30–5.79 s) | 2.12 s (2.00–2.30 s) | 2.69× | 5.35 s | 2.13 s | 2.51× | 32 MiB | 18 MiB | 1.83× | 6.2–7.4 | identical (1/1) |
| ripgrep | 2.49 s (2.44–2.90 s) | 477 ms (476–549 ms) | 5.22× | 2.40 s | 558 ms | 4.30× | 43 MiB | 33 MiB | 1.29× | 7.2–7.8 | identical (1/1) |

### 4.6 mutate full, `--no-coverage` (fixtures, fresh clone per run)

| fixture | upstream wall (min–max) | ubgate wall (min–max) | speedup | upstream CPU | ubgate CPU | CPU ratio | upstream RSS | ubgate RSS | load1 | parity |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| python | 1.89 s (1.87–2.17 s), exit 3 | 2.06 s (1.85–2.09 s), exit 3 | 0.91× | 11.07 s | 10.66 s | 1.04× | 36 MiB | 36 MiB | 7.5–9.4 | identical (5/5) |
| go | 15.46 s (15.44–15.97 s), exit 3 | 15.59 s (15.01–16.00 s), exit 3 | 0.99× | 18.69 s | 18.84 s | 0.99× | 90 MiB | 89 MiB | 6.5–11.7 | identical (4/4) |

Exit 3 is expected on both sides (surviving mutants). RSS here is the test
process (pytest ≈ 36 MiB, `go test` ≈ 90 MiB), not the tool.

## 5. Parity on the big repos

**36 of 39 timed cases** (excluding `--help`) gave byte-identical stdout and
`.metrics/*` on both sides. That is 65 of 70 compared artifacts, after root-path
and `:tested-at` masking. Every Python, Go, Rust and JavaScript repo matched
on every workload, and so did both fixture mutation runs (stdout plus all 7
`.metrics/mutate/*.edn` snapshots).

The three mismatches were all minimised and recorded in
[`ubgate/docs/PARSER-BUGS.md` → "Found by benchmarks"](../ubgate/docs/PARSER-BUGS.md#found-by-benchmarks):

| case | match | cause |
|---|---|---|
| crap typescript | 99.3% of lines (7 443 vs 7 495) | Two files fail to parse in ubgate and are dropped whole (52 functions). (1) `const d = x as a.b.C[];` is a **real parser bug**: tree-sitter parses it with no ERROR (`completionsIncomplete.ts`, 4 functions / 44 sites). (2) `interface L<in out T> {}` uses TS variance annotations: tree-sitter recovers with an ERROR node and ubgate rejects the file (deviation **A2**). That one file is `transformers/utilities.ts`, 48 functions / 166 sites. |
| mutate-scan typescript | 99.65% (210 site lines) | The same two files: 166 + 44 sites missing. |
| dry guava | 99.99% (one extra pair) | `@Nullable Object @Nullable ... args`: tree-sitter-java builds ERROR nodes, but ubgate now parses it cleanly. `checkNotNull`'s fingerprints therefore differ, and ubgate reports one extra pair, `Preconditions.java:995-1005 ~ 1062-1071` (0.84). Repro: `class C { void f(Object @A ... a) {} }`. DEVIATIONS **J1** still says ubgate rejects this input, which is now out of date. |

Two of the three are A2 inputs, where tree-sitter itself reports ERROR nodes
and ubgate does not emulate error recovery. Only the `as a.b.C[]` case is a
real ubgate parser bug.

## 6. Profiling hotspots and optimisation plan

Hotspots from the baseline profiles, in order of impact. Every planned change
must keep output **byte-identical** (re-checked with `bench parity` and
`harness/run.sh`).

### 6.1 dry: the pair loop (worst case: dry typescript, 61 s)

* `dry.FindDuplicates` → `dry.Jaccard` takes **94.6%** of CPU (93% of a 26 s
  run on django). Inside it, `runtime.mapaccess2_faststr` is 83% cumulative,
  with `aeshashbody` at 32% flat and `memequal` at 11%. `Jaccard(left, right
  map[string]struct{})` probes `right` for every element of `left`, re-hashing
  long fingerprint strings for every pair. Upstream's `len(a & b)` on
  frozensets runs in C with cached string hashes, which is why ubgate is only
  1.29× ahead here.
* Memory: peak live heap is 348 MiB on TypeScript. 80% of it is
  `strings.Builder` output (fingerprint strings from `dry.Fingerprints`,
  129 MiB), and 11% is the per-entry sets. This is why dry's RSS is 1.2–1.3×
  **above** upstream on large repos.

**Plan (being implemented now):**

1. **Fingerprint interning.** Map each language's fingerprints to dense
   `uint32` ids once, after scanning. Store each entry as a sorted `[]uint32`
   and intersect by merge. This removes the hashing (about 75% of the loop),
   keeps each distinct string only once, and should bring dry's RSS below
   upstream's.
2. **Size-bound pruning.** `J(A,B) ≤ min(|A|,|B|) / max(|A|,|B|)`, so a pair
   can be skipped when that bound is below the threshold. Sorting entries by
   set size lets the inner loop `break` early. The bound can never drop a
   qualifying pair. Iterate the smaller set.
3. **Parallel pair loop.** Split the outer loop across `GOMAXPROCS` workers
   with per-worker result slices, concatenated in `i` order before the
   existing stable sort so the output stays deterministic. dry currently uses
   1 of 10 cores.

### 6.2 `mutate --scan`: coverage bundle reloaded per file (django: 85% of a 12.8 s run)

`mutate.coveredLines` → `coverage.LoadBundle` → `coverage.globSorted` runs
**once per source file**. Each call does a `filepath.Glob` for
`root/*/*/target/coverage/go/coverage.out` (and the JaCoCo twin), reading every
directory at depth 2, `.git/objects/*` included. 816 MB of the run's 2.5 GB of
allocations come from `os.(*File).readdir`. Upstream has the same structure
(`covered_lines` calls `load_bundle` per file). The cost is highest on trees
with many depth-2 directories (django 1.42×, hugo 2.37×), while shallow trees
let the parser speedup show (TypeScript 13.1×, webpack 9.12×).

**Plan:** load the bundle (and `jacocoIndex`) **once per run** and pass it
down. This is a cache within a single run only. There is **no cross-run
cache**, so nothing is persisted or invalidated between invocations. It is
safe because nothing writes reports during a scan, and in a full run coverage
is generated before the per-file loop. Expected: django scan from about 12 s to
about 2 s.

### 6.3 crap: allocation and GC (crap typescript, 0.9 s)

Only about 37% of CPU is ubgate code. The rest is the Go runtime:
`runtime.madvise` at 28% (the scavenger returning freed tree memory), GC marking
and thread wake-ups. 65% of the 620 MB allocated are tree nodes
(`syntax.Leaf` 274 MB, `syntax.Inner` 127 MB) that die right after each file.
crap ripgrep shows the same pattern (`madvise` 30%).

**Plan:**

1. **Slab-allocate nodes**: per-file `[]syntax.Node` chunks instead of one heap
   object per node. Optionally raise `GOGC` (for example
   `debug.SetGCPercent(400)`) for these short-lived batch runs.
2. **Parallel file parsing**: files are independent, so parse across cores and
   re-sort results exactly as today.

### 6.4 Open decision: a Rust rewrite?

This question is **open**. The recommendation is:

1. Do the Go optimisations above first (dry → mutate-scan → crap).
2. Re-benchmark with the same harness and append a History entry.
3. Only then, if a hot path is still far from where it should be, prototype
   **one** hot path in Rust (the dry pair loop is the natural candidate) and
   compare it to the optimised Go on the same corpus.

The harness is language-agnostic: it times any command line through `bench
run` and compares outputs byte for byte. A Rust prototype can therefore be
measured as a third implementation without changing the method. Most of the
current gap is algorithmic (O(n²) hashing, per-file reloads, allocation
churn), not a language limit, so the expected return of a rewrite should be
judged against the optimised Go, not against this baseline.

## 7. How to reproduce

Prerequisites: the upstream tools and their venvs (`harness/fetch-upstream.sh`
if `upstream/` is missing), `fixtures/setup.sh` (for mutate-full), Go and git.
Everything stays inside the ub-mode root, under `.local/bench/` (gitignored).

```bash
cd /Users/matheusale/workspaces/ub-mode

# Full matrix (about 1.5 h on an M4, dominated by dry):
# fetch build env help crap dry mutate-scan mutate-full cover parity report
OUT=.local/bench/results/$(date +%Y%m%d) bench/run.sh

# The baseline used a 20 s load wait on a shared machine:
OUT=.local/bench/results/$(date +%Y%m%d) LOAD_WAIT=20s bench/run.sh

# Rerun cases hit by a load burst (appends a new batch; the report keeps the latest batch per case):
OUT=.local/bench/results/<YYYYMMDD> REPOS="axios hugo fzf guava gson tokio ripgrep" bench/run.sh mutate-scan parity report

# One workload, or a subset:
bench/run.sh build crap report                                   # into .local/bench/results/latest
REPOS="flask fzf" RUNS=5 bench/run.sh crap dry parity report

# Benchmark uncommitted ubgate changes (records the dirty-file count):
UBGATE_SRC=worktree OUT=.local/bench/results/$(date +%Y%m%d)-wip bench/run.sh build dry parity report

# Profile one ubgate case (CPU, allocs, peak inuse heap):
PROFILE_CASE="dry typescript" bench/run.sh profile
go tool pprof -top .local/bench/profile/ubgate-pprof .local/bench/profile/dry-typescript/cpu.pprof

# Check a parity mismatch against tree-sitter:
go run ./tools/treediff FILE
```

Knobs: `RUNS=3`, `WARMUP=1`, `TIMEOUT=30m`, `SLOW=120s`, `MAX_LOAD=4`,
`LOAD_WAIT=3m` (default), `REPOS`, `IMPLS="upstream ubgate"`, `OUT`,
`UBGATE_SRC=worktree`, `PROFILE_CASE`.

To compare against this baseline, keep the corpus pins and workloads
unchanged, run on the same machine class, and report both the absolute
numbers and the ubgate-vs-baseline-ubgate ratios. Because the machine is
shared, record the load range in the entry.

## 8. History

Newest entries go at the bottom. Each entry uses this template:

```markdown
### YYYY-MM-DD: <title> (ubgate `<short sha>`)

* **Change under test:** what changed in ubgate (or the harness) since the previous entry.
* **Run:** `.local/bench/results/<dir>/`, phases/repos run, RUNS/WARMUP, load range, reruns.
* **Results:** per-workload ubgate wall/CPU/RSS vs upstream and vs the previous entry (tables or deltas).
* **Parity:** N/M cases identical; any new or fixed mismatches (link PARSER-BUGS.md).
* **Notes / next:** surprises, new hotspots, follow-ups.
```

### 2026-11-03: Baseline (ubgate `12becbc`)

* **Change under test:** none. This is the first full comparison of the
  stdlib-only Go port against upstream crapper `6fd92c4`, dryer `66ff6d2`,
  mutator `a5a0cb8`.
* **Run:** `.local/bench/results/20261103/`, all phases, 12 repos, 1 warm-up
  + 3 runs (startup 2 + 10), 19:27–20:54 UTC. Load1 4.1–13.5 on kept runs.
  The mutate-scan cases for axios, hugo, fzf, guava, gson, tokio and ripgrep,
  plus mutate-full python, were rerun after a load ≈ 130 burst.
* **Results:** geomean wall speedups: crap 3.78× (2.05–8.13×), dry 2.89×
  (1.29–10.2×), mutate-scan 5.03× (1.42–13.1×), startup 10.1–29.7×,
  mutate-full 0.95× (tie: bounded by the fixtures' tests). Totals over 12
  repos: crap 13.6 s → 3.9 s, dry 313.5 s → 162.8 s, mutate-scan 136.2 s →
  30.5 s. Memory is lower on crap/scan, but dry is 1.2–1.3× **higher** on
  most large repos (0.77–0.84×). Full tables are in §4.
* **Parity:** 36/39 cases (65/70 artifacts) byte-identical. Mismatches: crap
  and mutate-scan on TypeScript (`as a.b.C[]` parser bug, `<in out T>` A2), and
  dry guava (J1 now parses, one extra pair). See §5 and PARSER-BUGS.md.
* **Notes / next:** dry pair loop (94.6% of dry TS CPU), per-file coverage
  bundle reload in `mutate --scan` (85% of django scan), and GC/allocation in
  crap. Optimisation plan in §6, starting with dry. The Rust question stays
  open until the Go optimisations have been re-benchmarked.

### 2026-11-03: dry optimisation (ubgate `26c7d9c` + working tree)

* **Change under test:** `internal/dry` pair loop and fingerprints (§6.1).
  Fingerprints are interned per language into dense `uint32` ids while
  scanning: atoms by kind+text, lists by their element ids. Printed strings
  are never built, and that keying matches printed-string equality exactly
  (a keyword containing a space or `]` would break it; that case is detected
  and the language is rescanned by printed string). Each entry keeps a sorted
  `[]uint32`, and Jaccard is a linear merge with early exit. Pairs are
  pruned with the size bound `|A|/|B|`, computed with the score's own float
  expression, so a pair exactly at the threshold is still reported. Each
  language group is walked by ascending set size, so a row stops at the
  first pair the bound rules out. Rows run on GOMAXPROCS workers. Hits are
  put back in upstream enumeration order before the existing stable sort.
* **Run:** `.local/opt-dry/results/` (driver `.local/opt-dry/bench.sh`,
  `bench run` directly). Dry only, 12 repos, 1 warm-up + 3 runs. ubgate
  before (`.local/opt-dry/ubgate-before`, built from the tree before the
  change) and after, each at default GOMAXPROCS (`p10`, 10 = NumCPU) and at
  `GOMAXPROCS=1` (`p1`). Before-`p1` used `-slow 20s`, so django, typescript
  and guava have one run. Upstream ran on django and fzf to calibrate drift:
  61.78 s / 1.32 s vs 64.30 s / 1.44 s in the baseline (machine about 4–8%
  faster). 21:55–22:48 UTC, load1 3.6–8.7 (one before-guava run at 38; its
  runs span 36.98–37.82 s).
* **Results** (median wall / CPU / max RSS):

  | repo | before p10 | after p10 | after p1 | wall speedup p10 / p1 | RSS before → after (p10) |
  |---|---:|---:|---:|---:|---:|
  | django | 25.48 s / 24.59 s | 513 ms / 895 ms | 807 ms / 779 ms | 49.6× / 28.0× | 124 → 31 MiB |
  | typescript | 49.63 s / 49.69 s | 1.01 s / 1.53 s | 1.36 s / 1.35 s | 49.0× / 36.7× | 375 → 135 MiB |
  | guava | 37.49 s / 36.56 s | 441 ms / 864 ms | 732 ms / 700 ms | 85.0× / 46.1× | 135 → 41 MiB |
  | hugo | 13.78 s / 13.83 s | 327 ms / 567 ms | 439 ms / 434 ms | 42.1× / 37.6× | 157 → 27 MiB |
  | tokio | 7.41 s / 7.39 s | 396 ms / 571 ms | 509 ms / 500 ms | 18.7× / 14.3× | 84 → 22 MiB |
  | excalidraw | 6.63 s / 6.69 s | 358 ms / 505 ms | 437 ms / 426 ms | 18.5× / 15.6× | 132 → 38 MiB |
  | ripgrep | 1.61 s / 1.62 s | 203 ms / 295 ms | 270 ms / 257 ms | 7.91× / 5.60× | 53 → 30 MiB |
  | fzf | 553 ms / 587 ms | 80 ms / 121 ms | 102 ms / 100 ms | 6.90× / 5.88× | 59 → 36 MiB |
  | gson | 349 ms / 346 ms | 59 ms / 71 ms | 48 ms / 47 ms | 5.89× / 5.19× | 21 → 14 MiB |
  | flask | 126 ms / 121 ms | 49 ms / 49 ms | 28 ms / 27 ms | 2.57× / 3.14× | 16 → 13 MiB |
  | webpack, axios | 4–7 ms | 4–7 ms | 4 ms | 1.0× (no dry files) | 6–7 MiB both |

  Sum of medians over the 12 repos: 143.1 s → 3.45 s at p10, and 139.1 s →
  4.74 s at p1. The algorithm alone (p1 vs p1) accounts for 28–46× on the
  large repos. Parallel rows add 1.3–1.7× wall on top, at the cost of some
  extra CPU (scheduling and GC threads). Max RSS is now 2.8–5.8× lower than
  before on the large repos, and below upstream (django 31 vs 100 MiB). Dry
  is 120× faster than upstream on django and 16.5× on fzf.
* **Parity:** byte-identical to the before binary on every repo for stdout,
  `.metrics/dry.edn`, stderr and exit code, with default flags. Also
  identical with `--threshold 0.5 --edn` on all 12 repos (up to 1.43 M lines
  on guava), and with `--threshold nan --min-lines 1 --min-nodes 1` (every
  pair) on fzf, gson and flask. `harness/run.sh check`: 36/36, raw-equal
  55/55.
* **Notes / next:** the pair loop is no longer the hotspot. What is left is
  sequential parsing plus GC (TypeScript peak RSS is the largest file's
  tree). Parsing files in parallel is the next lever for both crap and dry.
