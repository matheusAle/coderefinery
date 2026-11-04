# ubgate vs upstream: benchmark results

Run of 2026-11-03 (`.local/bench/results/20261103`, rerun with
`bench/run.sh`, see [README.md](README.md)). ubgate is the stdlib-only Go
port; upstream is Uncle Bob's crapper / dryer / mutator (Python 3.12 +
py-tree-sitter).

## Headline

Totals over the 12 repos (sum of per-case medians), plus startup and the fixture runs:

| workload | upstream wall | ubgate wall | speedup (total) | geomean speedup (range) | upstream CPU | ubgate CPU | max RSS upstream → ubgate (typical) | parity |
|---|---:|---:|---:|---:|---:|---:|---|---|
| startup (`--help`) | 46–83 ms | 3–5 ms | 10–30× | – | 36–56 ms | 2–4 ms | 17–23 MiB → 5 MiB | – |
| crap `--no-coverage` | 13.6 s | 3.9 s | 3.5× | 3.8× (2.1–8.1×) | 12.9 s | 4.7 s | 23–168 → 12–125 MiB (ubgate 1.2–2.1× less, guava 0.93×) | 11/12 identical |
| crap `--use-existing-coverage` (fzf) | 463 ms | 134 ms | 3.5× | – | 452 ms | 162 ms | 43 → 33 MiB | identical |
| dry | 313.5 s | 162.8 s | 1.9× | 2.9× (1.3–10×) | 296 s | 157 s | django/excalidraw/hugo/guava/tokio: ubgate 1.2–1.3× **more** (e.g. hugo 128 → 155 MiB); typescript 364 → 346; small repos less | 11/12 identical |
| mutate `--scan` | 136.2 s | 30.5 s | 4.5× | 5.0× (1.4–13×) | 131 s | 32 s | 29–172 → 14–161 MiB (1.07–2.3× less) | 11/12 identical |
| mutate full, `--no-coverage` (fixtures) | 17.3 s | 17.6 s | 0.98× | 0.95× | 29.8 s | 29.5 s | same (the peak is the test process) | 2/2 identical |

* ubgate is faster on every case except the full mutation runs. Those are
  bounded by the fixtures' own pytest / `go test` runs (one per mutant), so
  the two tools tie within noise. ubgate's own overhead there is negligible.
* The largest wins are in startup (no interpreter or import cost, 10–30×) and
  `mutate --scan` on TypeScript/JavaScript (13× on TypeScript, 9× on
  webpack). The smallest are dry on TypeScript (1.29×) and `mutate --scan`
  on django (1.42×). Both are algorithmic hotspots the port inherited 1:1
  from upstream (see Profiling).
* Memory: ubgate's peak is lower on crap and `mutate --scan` (Go has no
  interpreter baseline: a 5 MiB floor against Python's 17–23 MiB), but
  **higher on dry for most large repos** (ratio 0.77–0.84×; TypeScript is about even at 1.05×). The fingerprint sets are
  Go `map[string]struct{}` holding long strings (see Profiling).

## Environment

| | |
|---|---|
| CPU | Apple M4, 10 cores (4 performance + 6 efficiency) |
| RAM | 32 GiB |
| OS | macOS 26.6.1 (Darwin 25.6.0, arm64) |
| Go | 1.26.1 (ubgate built with `-trimpath` from `git archive HEAD`, commit `12becbc`) |
| Python | 3.12.3 (upstream venvs), tree-sitter 0.26.0, tree-sitter-language-pack 1.23.1 |
| upstream | crapper `6fd92c4`, dryer `66ff6d2`, mutator `a5a0cb8` |
| raw data | `.local/bench/results/20261103/` (`results.jsonl`, `summary.csv`, `parity.jsonl`, `env.txt`, `counts.jsonl`, `out/`) |

**Other load.** The machine was shared: other agents were running builds
and tests, and OrbStack (Docker VM), Chrome, Spotify and `mediaanalysisd`
were active. The 1-minute load average stayed between 4.1 and 13.5 (one burst to about 130, see Caveats) during
the timed runs, never below the target of 4. The runner waited up to 20 s
before each case and then ran anyway, recording the load per run (the
`load1` column). Every tool except mutate-full is single-threaded, so with 10
cores a free core was usually available. But the M4 has only 4 performance
cores, and under this load the scheduler can place a run on an efficiency
core. That noise is in both columns. Upstream and ubgate ran back to back for
each case (upstream first), so they saw similar conditions.

## Corpus

Analysed set = the files the tools walk after their own `SKIP_DIRS` /
`TEST_DIRS` pruning and test-file rules (`bench count`): tests,
`node_modules`, `vendor`, `testdata` etc. are excluded, as the tools exclude
them. "dry files" is smaller because dryer has no `.js/.jsx/.mjs/.cjs`
support, so dry on webpack/axios only walks the tree.

| repo | upstream | pinned SHA | language | size | checkout | analysed files | lines | non-blank | dry files |
|---|---|---|---|---|---|---:|---:|---:|---:|
| django | [django/django](https://github.com/django/django) | `2382f40aed50` | python | large | whole repo | 943 (python 921, javascript 22) | 167,564 | 144,922 | 921 |
| flask | [pallets/flask](https://github.com/pallets/flask) | `49943b5a9441` | python | medium | whole repo | 35 (python 35) | 10,139 | 8,051 | 35 |
| typescript | [microsoft/TypeScript](https://github.com/microsoft/TypeScript) | `050880ce59e3` | typescript | large | `src/` | 604 (typescript 601, javascript 3) | 380,278 | 347,187 | 601 |
| excalidraw | [excalidraw/excalidraw](https://github.com/excalidraw/excalidraw) | `dd2f8fc32b92` | typescript | medium | whole repo | 549 (typescript 521, javascript 28) | 154,718 | 139,926 | 521 |
| webpack | [webpack/webpack](https://github.com/webpack/webpack) | `0b35ae904f20` | javascript | large | `lib/` | 818 (javascript 818) | 339,946 | 318,686 | 0 |
| axios | [axios/axios](https://github.com/axios/axios) | `fb88a1b133c1` | javascript | medium | `lib/` | 79 (javascript 78, typescript 1) | 10,755 | 9,218 | 1 |
| hugo | [gohugoio/hugo](https://github.com/gohugoio/hugo) | `fa5a3d48b8dc` | go | large | whole repo | 542 (go 519, javascript 23) | 133,747 | 114,612 | 519 |
| fzf | [junegunn/fzf](https://github.com/junegunn/fzf) | `99bfd5aa0f66` | go | medium | whole repo | 60 (go 60) | 27,262 | 25,190 | 60 |
| guava | [google/guava](https://github.com/google/guava) | `2bcd693991a6` | java | large | `guava/src/` | 611 (java 611) | 182,525 | 164,002 | 611 |
| gson | [google/gson](https://github.com/google/gson) | `57731049da3e` | java | medium | `gson/src/main/` | 87 (java 87) | 19,538 | 17,834 | 87 |
| tokio | [tokio-rs/tokio](https://github.com/tokio-rs/tokio) | `f67444986f67` | rust | large | whole repo | 509 (rust 509) | 131,535 | 118,613 | 509 |
| ripgrep | [BurntSushi/ripgrep](https://github.com/BurntSushi/ripgrep) | `91f5d22ef48b` | rust | medium | whole repo | 87 (rust 87) | 49,037 | 44,625 | 87 |

Giants are sparse checkouts: TypeScript `src/` (pinned to tag v6.0.3, the
last release written in TypeScript; `main` is now the Go port), webpack
`lib/`, guava `guava/src/`, gson `gson/src/main/`, axios `lib/`. Root files
(`package.json`, `go.mod`, …) are always present.

## Results

Median of 3 measured runs after 1 warm-up (startup: 10 runs after 2 warm-ups).
Ranges are min–max. Speedup = upstream median wall / ubgate median wall. CPU
ratio = upstream CPU / ubgate CPU (user+sys). Memory ratio = upstream max
RSS / ubgate max RSS (> 1 means ubgate uses less). Runs `3/3` = measured runs
upstream/ubgate. Parity = byte-identical stdout and `.metrics/*` files after
root-path masking.

| workload | repo | upstream wall (min–max) | ubgate wall (min–max) | speedup | upstream CPU | ubgate CPU | CPU ratio | upstream max RSS | ubgate max RSS | mem ratio | runs | load1 | parity |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---|
| help-crap | crap | 83 ms (44–152 ms) | 3 ms (3–6 ms) | 29.7× | 56 ms | 2 ms | 24.8× | 19 MiB | 5 MiB | 3.46× | 10/10 | 8.9–10.2 | – |
| help-dry | dry | 46 ms (35–72 ms) | 5 ms (3–8 ms) | 10.1× | 36 ms | 4 ms | 9.51× | 17 MiB | 5 MiB | 3.15× | 10/10 | 9.2–11.4 | – |
| help-mutate | mutate | 52 ms (49–73 ms) | 3 ms (3–5 ms) | 17.3× | 46 ms | 2 ms | 19.6× | 23 MiB | 5 MiB | 4.21× | 10/10 | 9.1–10.0 | – |
| crap | django | 1.25 s (1.25–1.30 s) | 420 ms (389–485 ms) | 2.99× | 1.23 s | 538 ms | 2.28× | 36 MiB | 21 MiB | 1.73× | 3/3 | 8.1–9.6 | identical (2/2) |
| crap | flask | 150 ms (139–160 ms) | 21 ms (19–38 ms) | 7.17× | 128 ms | 23 ms | 5.58× | 24 MiB | 13 MiB | 1.89× | 3/3 | 9.0–10.1 | identical (2/2) |
| crap | typescript | 3.90 s (3.75–4.26 s) | 907 ms (883–1.43 s) | 4.30× | 3.70 s | 1.12 s | 3.30× | 168 MiB | 125 MiB | 1.34× | 3/3 | 8.9–11.2 | 0/2 identical; stdout 99.3%, crap.edn 99.3% |
| crap | excalidraw | 1.30 s (1.25–1.33 s) | 340 ms (337–534 ms) | 3.82× | 1.25 s | 391 ms | 3.19× | 45 MiB | 29 MiB | 1.54× | 3/3 | 10.0–11.2 | identical (2/2) |
| crap | webpack | 2.64 s (2.55–3.25 s) | 603 ms (570–693 ms) | 4.37× | 2.58 s | 743 ms | 3.47× | 75 MiB | 52 MiB | 1.44× | 3/3 | 9.2–10.8 | identical (2/2) |
| crap | axios | 145 ms (140–154 ms) | 26 ms (25–30 ms) | 5.61× | 139 ms | 27 ms | 5.17× | 26 MiB | 12 MiB | 2.12× | 3/3 | 10.9–13.5 | identical (2/2) |
| crap | hugo | 1.29 s (1.21–1.30 s) | 405 ms (360–506 ms) | 3.19× | 1.22 s | 458 ms | 2.67× | 67 MiB | 43 MiB | 1.56× | 3/3 | 11.2–11.6 | identical (2/2) |
| crap | fzf | 442 ms (423–493 ms) | 54 ms (52–58 ms) | 8.13× | 357 ms | 74 ms | 4.84× | 42 MiB | 32 MiB | 1.31× | 3/3 | 11.2–11.6 | identical (2/2) |
| crap | guava | 863 ms (838–917 ms) | 418 ms (383–448 ms) | 2.07× | 849 ms | 542 ms | 1.57× | 34 MiB | 36 MiB | 0.93× | 3/3 | 10.3–12.1 | identical (2/2) |
| crap | gson | 138 ms (134–169 ms) | 37 ms (36–40 ms) | 3.71× | 132 ms | 40 ms | 3.31× | 23 MiB | 13 MiB | 1.76× | 3/3 | 10.7–10.9 | identical (2/2) |
| crap | tokio | 1.02 s (862–1.07 s) | 499 ms (374–764 ms) | 2.05× | 964 ms | 535 ms | 1.80× | 30 MiB | 17 MiB | 1.79× | 3/3 | 9.4–9.7 | identical (2/2) |
| crap | ripgrep | 438 ms (386–487 ms) | 179 ms (174–266 ms) | 2.45× | 387 ms | 227 ms | 1.71× | 38 MiB | 30 MiB | 1.23× | 3/3 | 9.3–10.2 | identical (2/2) |
| crap-cover | fzf | 463 ms (457–614 ms) | 134 ms (102–175 ms) | 3.47× | 452 ms | 162 ms | 2.79× | 43 MiB | 33 MiB | 1.29× | 3/3 | 9.0–9.4 | identical (2/2) |
| dry | django | 64.30 s (63.65–65.54 s) | 28.11 s (26.57–30.65 s) | 2.29× | 61.40 s | 26.30 s | 2.33× | 99 MiB | 127 MiB | 0.78× | 3/3 | 4.9–13.2 | identical (2/2) |
| dry | flask | 306 ms (290–316 ms) | 85 ms (84–108 ms) | 3.58× | 288 ms | 91 ms | 3.15× | 25 MiB | 15 MiB | 1.61× | 3/3 | 6.2–6.9 | identical (2/2) |
| dry | typescript | 78.34 s (74.73–85.53 s) | 60.92 s (60.75–61.45 s) | 1.29× | 73.83 s | 58.86 s | 1.25× | 364 MiB | 346 MiB | 1.05× | 3/3 | 5.8–9.3 | identical (2/2) |
| dry | excalidraw | 19.90 s (19.44–20.31 s) | 8.43 s (8.09–9.18 s) | 2.36× | 18.20 s | 8.12 s | 2.24× | 112 MiB | 139 MiB | 0.80× | 3/3 | 5.7–7.3 | identical (2/2) |
| dry | webpack | 45 ms (42–50 ms) | 7 ms (7–8 ms) | 6.09× | 38 ms | 6 ms | 6.94× | 17 MiB | 6 MiB | 2.89× | 3/3 | 8.4–9.3 | identical (1/1) |
| dry | axios | 66 ms (53–84 ms) | 6 ms (5–7 ms) | 10.2× | 58 ms | 6 ms | 10.0× | 23 MiB | 8 MiB | 3.05× | 3/3 | 6.7–7.9 | identical (2/2) |
| dry | hugo | 35.67 s (35.50–38.44 s) | 16.02 s (15.97–16.40 s) | 2.23× | 34.00 s | 15.77 s | 2.16× | 128 MiB | 155 MiB | 0.82× | 3/3 | 6.7–9.2 | identical (2/2) |
| dry | fzf | 1.44 s (1.32–1.54 s) | 548 ms (538–759 ms) | 2.63× | 1.39 s | 578 ms | 2.41× | 70 MiB | 57 MiB | 1.24× | 3/3 | 6.2–6.7 | identical (2/2) |
| dry | guava | 89.95 s (88.44–104 s) | 38.83 s (37.16–39.09 s) | 2.32× | 84.45 s | 37.19 s | 2.27× | 104 MiB | 134 MiB | 0.77× | 3/3 | 5.4–6.8 | 0/2 identical; stdout 100.0%, dry.edn 100.0% |
| dry | gson | 755 ms (739–1.01 s) | 297 ms (245–299 ms) | 2.54× | 726 ms | 285 ms | 2.55× | 30 MiB | 22 MiB | 1.33× | 3/3 | 4.9–5.2 | identical (2/2) |
| dry | tokio | 18.52 s (18.05–18.68 s) | 8.03 s (7.87–8.63 s) | 2.31× | 17.65 s | 7.84 s | 2.25× | 72 MiB | 86 MiB | 0.84× | 3/3 | 5.3–6.1 | identical (2/2) |
| dry | ripgrep | 4.14 s (3.74–4.22 s) | 1.51 s (1.42–1.76 s) | 2.74× | 3.94 s | 1.56 s | 2.52× | 55 MiB | 52 MiB | 1.07× | 3/3 | 4.9–5.4 | identical (2/2) |
| mutate-scan | django | 16.62 s (16.19–16.72 s) | 11.71 s (11.20–14.93 s) | 1.42× | 15.90 s | 11.67 s | 1.36× | 34 MiB | 21 MiB | 1.64× | 3/3 | 4.2–6.1 | identical (1/1) |
| mutate-scan | flask | 359 ms (318–377 ms) | 86 ms (85–104 ms) | 4.19× | 325 ms | 92 ms | 3.54× | 28 MiB | 14 MiB | 2.10× | 3/3 | 6.4–6.6 | identical (1/1) |
| mutate-scan | typescript | 38.62 s (38.51–38.80 s) | 2.94 s (2.92–3.74 s) | 13.1× | 37.59 s | 3.41 s | 11.0× | 172 MiB | 161 MiB | 1.07× | 3/3 | 4.1–6.1 | 0/1 identical; stdout 99.6% |
| mutate-scan | excalidraw | 12.59 s (12.47–12.74 s) | 2.23 s (2.13–2.27 s) | 5.66× | 12.16 s | 2.27 s | 5.35× | 49 MiB | 27 MiB | 1.79× | 3/3 | 4.5–6.5 | identical (1/1) |
| mutate-scan | webpack | 38.24 s (37.93–38.67 s) | 4.19 s (4.09–4.29 s) | 9.12× | 36.76 s | 4.47 s | 8.23× | 78 MiB | 62 MiB | 1.27× | 3/3 | 5.2–10.2 | identical (1/1) |
| mutate-scan | axios | 827 ms (806–927 ms) | 120 ms (118–147 ms) | 6.92× | 819 ms | 125 ms | 6.57× | 31 MiB | 14 MiB | 2.24× | 3/3 | 8.0–8.0 | identical (1/1) |
| mutate-scan | hugo | 13.24 s (13.08–14.43 s) | 5.58 s (5.51–5.81 s) | 2.37× | 12.60 s | 5.71 s | 2.21× | 71 MiB | 52 MiB | 1.36× | 3/3 | 6.7–8.1 | identical (1/1) |
| mutate-scan | fzf | 1.38 s (1.33–1.38 s) | 211 ms (201–253 ms) | 6.57× | 1.37 s | 256 ms | 5.34× | 49 MiB | 34 MiB | 1.43× | 3/3 | 6.3–6.6 | identical (1/1) |
| mutate-scan | guava | 5.65 s (5.57–5.80 s) | 747 ms (731–938 ms) | 7.56× | 5.33 s | 905 ms | 5.89× | 38 MiB | 34 MiB | 1.14× | 3/3 | 6.3–6.8 | identical (1/1) |
| mutate-scan | gson | 484 ms (480–499 ms) | 86 ms (82–94 ms) | 5.64× | 477 ms | 96 ms | 4.99× | 29 MiB | 14 MiB | 2.02× | 3/3 | 6.7–6.8 | identical (1/1) |
| mutate-scan | tokio | 5.69 s (5.30–5.79 s) | 2.12 s (2.00–2.30 s) | 2.69× | 5.35 s | 2.13 s | 2.51× | 32 MiB | 18 MiB | 1.83× | 3/3 | 6.2–7.4 | identical (1/1) |
| mutate-scan | ripgrep | 2.49 s (2.44–2.90 s) | 477 ms (476–549 ms) | 5.22× | 2.40 s | 558 ms | 4.30× | 43 MiB | 33 MiB | 1.29× | 3/3 | 7.2–7.8 | identical (1/1) |
| mutate-full | python | 1.89 s (1.87–2.17 s) exit 3 | 2.06 s (1.85–2.09 s) exit 3 | 0.91× | 11.07 s | 10.66 s | 1.04× | 36 MiB | 36 MiB | 1.00× | 3/3 | 7.5–9.4 | identical (5/5) |
| mutate-full | go | 15.46 s (15.44–15.97 s) exit 3 | 15.59 s (15.01–16.00 s) exit 3 | 0.99× | 18.69 s | 18.84 s | 0.99× | 90 MiB | 89 MiB | 1.00× | 3/3 | 6.5–11.7 | identical (4/4) |

Per-workload geometric mean of the wall-time speedups:

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

### Reading the numbers

* **CPU vs wall.** ubgate's CPU is often a little above its wall time (for
  example crap typescript 1.12 s CPU in 0.91 s wall) because the Go garbage
  collector runs on other cores. Upstream is a single Python thread, so its
  CPU stays just under wall. The CPU ratio is therefore a bit lower than the
  speedup. It is the fairer measure of total work done.
* **Where the speedup comes from.** crap and `mutate --scan` are dominated by
  parsing. py-tree-sitter parses in C, so Python is not as far behind as
  usual: the 2–8× crap gains come mostly from Python's per-node attribute
  access and object overhead in the readers, plus startup. Small repos show
  the biggest ratios (flask 7×, fzf 8×) because Python's ~50–80 ms startup
  dominates there.
* **dry scales with pairs, not files.** Both implementations compare every
  same-language candidate pair (O(n²)). On TypeScript (601 files, 380 k
  lines) ubgate spends 95% of its time in that loop and is only 1.29× ahead.
  Upstream's `len(a & b)` on frozensets is C code and CPython caches each
  string's hash. ubgate's Go map lookups rehash every long fingerprint string
  on every probe.
* **`mutate --scan` on django (1.42×) and hugo (2.4×)** are slow for a
  different reason: the coverage bundle is reloaded for every source file
  (upstream does the same), and each reload globs the whole tree for
  `*/*/target/...` reports. That costs most on trees with many directories at
  depth 2. TypeScript/webpack have shallow trees, so the parser speedup shows
  through (9–13×).
* **dry on webpack/axios** only walks the tree: dryer has no `.js` support,
  so there is nothing to compare (6–10× is pure startup/walk).
* **Warm vs first run.** The median ratio of first (warm-up) run to measured
  median is 1.06 for ubgate and 1.09 for upstream. The largest outliers are
  ubgate crap tokio (2.3×) and django (1.6×) on their first touch of the
  work copy, which is page-cache warm-up of a sub-second run.

## Parity on the big repos

36 of 39 timed cases (excluding `--help`) produce byte-identical stdout and
`.metrics/*` on both sides. That is 65 of 70 compared artifacts, after
masking each side's root path and mutator's `:tested-at` timestamp. The three
mismatches, all minimised and appended to `ubgate/docs/PARSER-BUGS.md` under
"Found by benchmarks":

| case | match | cause | repro |
|---|---|---|---|
| crap typescript | 99.3% of lines (7 443 vs 7 495) | two files fail to parse in ubgate and are dropped whole (52 functions) | (1) `const d = x as a.b.C[];`: a real parser bug, since tree-sitter parses it without ERROR. (2) `interface L<in out T> {}`: TS variance annotations. tree-sitter recovers with an ERROR node, ubgate rejects the file (deviation A2, but it costs 48 functions) |
| mutate-scan typescript | 99.65% (210 site lines) | the same two files: 166 + 44 sites missing | as above |
| dry guava | 99.99% (one extra pair) | `@Nullable Object @Nullable ... args`: tree-sitter-java builds ERROR nodes, ubgate parses it cleanly, so `checkNotNull`'s fingerprints differ and ubgate reports `Preconditions.java:995-1005 ~ 1062-1071` (0.84) | `class C { void f(Object @A ... a) {} }`. DEVIATIONS J1 says ubgate rejects this input, but it now parses it |

Every Python, Go, Rust and JavaScript repo matched exactly on all workloads,
as did the fixture mutation runs (stdout plus all 7 `.metrics/mutate/*.edn` snapshots).

## Profiling hotspots (ubgate)

Profiles were taken with a throwaway build of `ubgate/cmd/ubgate` plus a pprof
wrapper (`bench/run.sh profile`, built under `.local/bench/profile`; ubgate/
is not modified). The heap "inuse" view is a snapshot at the peak of live heap
objects, sampled every 100 ms. Raw profiles and `go tool pprof -top` output
are in `.local/bench/profile/<tool>-<repo>/`.

Hotspots in order of impact. These are suggestions only; nothing was changed.

1. **dry pair loop, slowest workload (dry typescript, 61 s).**
   `dry.FindDuplicates` → `dry.Jaccard` takes **94.6%** of CPU. Inside it,
   `runtime.mapaccess2_faststr` is 83% cumulative, with `aeshashbody` at
   32% flat and `memequal` at 11%. `Jaccard(left, right map[string]struct{})`
   iterates `left` and probes `right`, hashing every (long) fingerprint string
   again for every pair. Suggestions, all of which keep the output exact:
   * **Intern fingerprints** into dense `uint32` ids per language once, after
     scanning. Store each entry as a sorted `[]uint32` and intersect by merge.
     This removes the hashing (about 75% of the loop) and most of the memory.
   * **Size bound**: `J(A,B) ≤ min(|A|,|B|) / max(|A|,|B|)`. Skip a pair when
     that bound is below the threshold. Sorting entries by set size also lets
     the inner loop `break` early. The bound never drops a qualifying pair.
   * Iterate the smaller set (today it is always `left`).
   * Parallelise the outer loop over `GOMAXPROCS`, with per-worker result
     slices concatenated in `i` order before the existing stable sort, so the
     output stays deterministic. The machine has 10 cores and dry uses one.
   Same profile on django: Jaccard is 93% of a 26 s run.
2. **dry memory.** Peak live heap is 348 MiB on TypeScript. The heap snapshot
   at the peak is 80% `strings.Builder` output (fingerprint strings built by
   `dry.Fingerprints`, 129 MiB) plus 11% the per-entry sets. Interning (above)
   keeps each distinct string once and replaces the maps with `[]uint32`.
   That should make dry's RSS lower than upstream's instead of 1.2–1.3× higher.
3. **`mutate --scan`: coverage bundle reloaded per file** (django: 85% of a
   12.8 s run). `mutate.coveredLines` → `coverage.LoadBundle` →
   `coverage.globSorted` runs once **per source file**. `filepath.Glob` for
   `root/*/*/target/coverage/go/coverage.out` (and the JaCoCo twin) reads
   every directory at depth 2, `.git/objects/*` included. 816 MB of the run's
   2.5 GB allocations are `os.(*File).readdir`. Upstream has the same shape
   (`covered_lines` calls `load_bundle` for every file). Load the bundle (and
   `jacocoIndex`) **once per run** and pass it down. Nothing writes reports
   during a scan, and in a full run coverage is generated before the per-file
   loop. Expected: django scan from about 12 s to about 2 s.
4. **crap: allocation and GC** (crap typescript, 0.9 s). Only about 37% of CPU
   is ubgate code. The rest is the Go runtime: `runtime.madvise` 28% (the
   scavenger returning freed tree memory to the OS), GC marking and thread
   wake-ups. 65% of the 620 MB allocated are tree nodes
   (`syntax.Leaf` 274 MB, `syntax.Inner` 127 MB) that die right after each
   file. Suggestions: allocate nodes from a per-file slab (`[]syntax.Node`
   chunks) instead of one heap object each, and/or raise `GOGC` (for example
   `debug.SetGCPercent(400)`) for these short-lived batch runs. Files are
   independent, so parsing them in parallel across cores (results re-sorted as
   today) would cut wall time further. The same pattern shows on crap ripgrep
   (`madvise` 30% of samples).

## Methodology

* **Runner** (`bench/cmd/bench`, stdlib only). Each run starts the command in
  its own process group and reads its rusage from `wait4`
  (`ProcessState.SysUsage()`). **wall** = monotonic clock around start/wait.
  **CPU** = `ru_utime + ru_stime`. That includes every descendant the child
  waited for (pytest / `go test` in mutate-full), and it matched the runner's
  own `RUSAGE_CHILDREN` delta exactly in all runs. **max RSS** =
  `ru_maxrss` (bytes on macOS, KiB on Linux, normalised). It is the peak of
  the **single largest process** in the tree, not a sum: rusage merges
  children with `max`. For every workload except mutate-full that is the
  tool's own peak. For mutate-full it is the largest of the tool and its test
  processes.
* **Fresh inputs.** Each implementation has its own APFS clone of every repo.
  `.metrics/` is deleted before every run (untimed `-pre` step), so crap/dry
  snapshots and mutate history never carry over. `mutate --scan` writes
  nothing (checked: no `.metrics/` after a scan). mutate-full re-clones the
  fixture before every run.
* **Warm-up.** The first run of every case is recorded but excluded. It warms
  the page cache (and `__pycache__` for the upstream venvs). ubgate keeps no
  persistent cache of its own, so "cold vs warm cache" only means the OS page
  cache. In this run the first (warm-up) run was a median 1.06× (ubgate) / 1.09× (upstream) of the measured median. A truly cold page cache (`sudo purge`) was not tested.
* **Slow cases.** A warm-up longer than 120 s would have become the single
  measured run (marked `1*`). The timeout was 30 min. No case needed it: the slowest warm-up, upstream dry guava, took 85 s. Nothing timed out.
* **Workloads.** `crap --no-coverage` (discovery + parse + complexity +
  report), `crap --use-existing-coverage` on fzf with a Go profile made once
  with `go test ./... -coverprofile` (untimed), `dry` with default flags,
  `mutate --scan` (site discovery and naming, no tests), and a full
  `mutate --no-coverage` on `fixtures/python` and `fixtures/go`. Test-suite
  execution of the big repos was deliberately left out, because it would time
  the project's tests, not the tool.
* **ubgate build.** `git archive HEAD ubgate` (commit `12becbc`) built with
  `go build -trimpath`, so concurrent edits in the working tree do not leak in.

## Caveats

* **Shared, loaded machine.** The 1-minute load average was 4.1–13.5 during
  the kept runs (the target was < 4; other agents' builds, OrbStack, Chrome,
  `mediaanalysisd`). One burst (load up to 130, an Android emulator plus media
  analysis) hit the first mutate-scan pass for axios, hugo, fzf, guava, gson,
  tokio and ripgrep. Those seven cases were rerun once the load was back to
  6–8, and the report keeps only the rerun. Expect roughly ±10% noise per
  case, more on sub-100 ms cases. The ratios hold better than the absolute
  times, because both sides ran back to back.
* **E-cores.** On an M4 with 4 P-cores under this load, the macOS scheduler
  may move a single-threaded run onto an efficiency core. That cannot be
  pinned from user space. The min–max ranges show where it happened (e.g.
  ubgate crap tokio 374–764 ms).
* **max RSS is per process, not per tree.** It is the largest single process
  (rusage merges with `max`). For mutate-full that is the test process
  (pytest about 36 MiB, `go test` about 90 MiB), not mutator/ubgate.
* **Subtrees.** Giant repos were measured on subtrees (TypeScript `src/`,
  webpack `lib/`, guava `guava/src/`, gson `gson/src/main/`, axios `lib/`),
  and tests are excluded by the tools' own rules. TypeScript is pinned to
  v6.0.3 because `main` is now the Go port.
* **Not measured:** full mutation runs on the big repos (dominated by their
  test suites), coverage generation (`crap` without `--no-coverage`), and
  `--changed`. Parity on fixtures with structural EDN comparison stays the
  job of `harness/run.sh`. `bench parity` is a stricter byte comparison.
* **A2 inputs.** Two of the three mismatches are inputs where tree-sitter
  itself reports ERROR nodes. ubgate's parser does not emulate error recovery
  (DEVIATIONS A2), so such a file is dropped whole (crap/mutate) or gets
  different fingerprints (dry).
