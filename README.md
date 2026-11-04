# coderefinery

Formerly ubgate / ub-mode-port, renamed 2026-11-03.

This repo has two parts:

- **The `coderefinery` agent skill** (`skills/coderefinery/`). It is Robert C.
  Martin's [ub-mode](https://github.com/unclebob/ub-mode) principles, with
  the Crapper, Dryer and Mutator steps rewritten to run `coderefinery`, plus a
  launcher that finds or installs the binary.
- **`coderefinery`** (the Go module at the repo root). One static, stdlib-only
  Go binary that ports Uncle Bob's three quality-gate tools:
  [crapper](https://github.com/unclebob/crapper) (`coderefinery crap`),
  [dryer](https://github.com/unclebob/dryer) (`coderefinery dry`) and
  [mutator](https://github.com/unclebob/mutator) (`coderefinery mutate`). The flags,
  stdout, exit codes and `.metrics/` files match the Python originals byte
  for byte. It needs no Python and no tree-sitter.
  On top of that it adds two opt-in commands, `test` and `tests`, and a few
  hidden flags that report which tests earn their keep and which branches no
  test takes ([Test relevance](#test-relevance-and-untested-branches)).

## Install the skill

```bash
npx skills add matheusAle/coderefinery                    # pick agents interactively
npx skills add matheusAle/coderefinery -a claude-code -y  # this project, Claude Code
npx skills add matheusAle/coderefinery -g -a claude-code  # every project
```

This uses the [`skills`](https://github.com/vercel-labs/skills) CLI
(Node.js 22.20+). It copies `skills/coderefinery/` (`SKILL.md`, `LICENSE`,
`scripts/coderefinery`) into each agent's skills directory, for example
`.claude/skills/coderefinery/`. Then run `/coderefinery`. To install by hand, copy
`skills/coderefinery/` to `~/.claude/skills/coderefinery/` (or your agent's skills
directory). How discovery and installation work: [docs/skills-manager.md](docs/skills-manager.md).

The skill calls `scripts/coderefinery`, a POSIX `sh` launcher. Claude Code reaches
it as `${CLAUDE_SKILL_DIR}/scripts/coderefinery`. The launcher uses the first of:

1. `$CODEREFINERY_BIN`
2. `coderefinery` on `PATH`
3. `${XDG_CACHE_HOME:-$HOME/.cache}/coderefinery/<version>/coderefinery`
4. the GitHub release asset for this OS and arch, checked against the
   release's `SHA256SUMS` and cached (needs curl or wget, and sha256sum or shasum)
5. `go install github.com/matheusAle/coderefinery/cmd/coderefinery@<version>` into
   that cache directory

The version is pinned once, in `CODEREFINERY_VERSION` at the top of the launcher.
When nothing works, the launcher exits 127 with a message.

## Install coderefinery

Release builds exist for macOS and Linux on amd64 and arm64. There is no
Windows build: mutate kills test processes through Unix process groups.

- **Release binary**: download `coderefinery_<version>_<os>_<arch>.tar.gz` from
  [Releases](https://github.com/matheusAle/coderefinery/releases), check it against
  `SHA256SUMS`, and put `coderefinery` on your `PATH`.
- **go install** (Go 1.26+):
  `go install github.com/matheusAle/coderefinery/cmd/coderefinery@latest`
- **From source**: `make build`, which writes `bin/coderefinery`.
  The Makefile sources `harness/env.sh` (see [Development](#development)).

`coderefinery --version` prints the version.

## Usage

Run from the project root. Each command takes `--help`.

```bash
coderefinery crap --changed --threshold 9.9999   # CRAP < 10 on changed files; exit 2 when one is at 10 or more
coderefinery dry --threshold 0.8                 # duplicates scoring 0.8 or more (the default is 0.82)
coderefinery mutate --changed --use-existing-coverage   # exit 3 when a mutant survives
```

| command | writes | exit codes |
|---|---|---|
| `crap` | `.metrics/crap.edn` | 0 ok · 1 usage/error · 2 `--threshold` exceeded |
| `dry` | `.metrics/dry.edn` | 0 report written (read it) · 1 usage · 2 unknown format |
| `mutate` | `.metrics/mutate/<namespace>.edn` | 0 all killed · 1 usage/error · 2 baseline tests failed · 3 survivors |
| `test` | `.metrics/tests/` (`runs.edn`, results, coverage) | 0 all passed or skipped · 1 usage / missing tool or plugin · 2 a test failed, build failed or no results |
| `tests` | `.metrics/tests.edn` | 0 no offenders · 1 usage / no attribution data · 2 latest results have failures · 3 a test kills nothing or is shadowed |

With `--changed`, a git failure exits with git's own code. Supported
languages: Python, TypeScript/TSX/JavaScript, Go, Java, Rust, Clojure (dry
skips plain JavaScript, as upstream does). Coverage comes from each
language's usual tool (coverage.py, Vitest/c8, `go test`, JaCoCo, cargo
llvm-cov or tarpaulin, Cloverage). How the skill acts on each result:
[skills/coderefinery/SKILL.md](skills/coderefinery/SKILL.md).

### Test relevance and untested branches

These are coderefinery additions, not upstream behaviour. `test` and `tests`
appear in `coderefinery --help`; the flags on crap, dry and mutate are hidden
from their `--help`, which stays byte-identical to upstream. Design and
contract: [docs/design-tests.md](docs/design-tests.md); the list of additions:
[docs/DEVIATIONS.md](docs/DEVIATIONS.md#additions-opt-in).

```bash
coderefinery test                     # run the tests once: JUnit results + branch coverage in .metrics/tests/
coderefinery test --repeat 3          # tests whose status changes between runs are marked flaky
coderefinery test --changed           # only the projects of changed files (test files included)
coderefinery test --test-command 'pytest --junitxml="$CODEREFINERY_RESULTS"'   # your own runner
coderefinery test --no-coverage       # results only
coderefinery mutate --changed --use-existing-coverage --attribute   # record which tests kill each mutant (:killed-by)
coderefinery tests                    # Kills / Unique / Shadowed by per test case; exit 3 on offenders
coderefinery tests --group            # one row per parametrized test instead of per case
coderefinery tests --changed          # rows for changed tests and tests that kill mutants in changed files
coderefinery crap --branches          # adds "Untested branches" after the report, writes .metrics/branches.edn
coderefinery mutate --branches        # UNCOVERED lines say which arm was never taken
coderefinery dry --tests --threshold 0.8   # compare test files too (copied tests)
```

Order: `crap` (optionally `--branches`), `dry` (then `dry --tests`), `test`,
`mutate --attribute`, `tests`. A test with no kills, or one shadowed by tests
that kill the same mutants, is an offender (exit 3).

| language | runner `test` drives | status | branch granularity |
|---|---|---|---|
| Python | pytest + coverage.py (pytest-cov adds per-test contexts, which narrow `mutate --attribute`) | live-tested | branch (coverage.py arcs) |
| Go | `go test -json` | live-tested | block (coverprofile blocks; never called a branch) |
| TypeScript/JavaScript | Vitest, Jest (jest-junit) | live-tested | branch via Istanbul JSON; branch-count from V8 block ranges and LCOV `BRDA` |
| TypeScript/JavaScript | Mocha (mocha-junit-reporter, c8) | parser-only | as Vitest/Jest |
| Java | Maven Surefire + JaCoCo; Gradle `test` | sample-only | branch-count (JaCoCo `mb`/`cb`) |
| Rust | cargo nextest + cargo llvm-cov | sample-only | region (branch only with llvm-cov `--branch`, nightly) |
| Clojure | Kaocha (kaocha-junit-xml, kaocha-cloverage); lein test2junit | sample-only | form (Cloverage) |

Live-tested: run end to end on the fixture projects under `fixtures/*-tests/`.
Sample-only: the result parser and branch loader are tested on checked-in
sample reports; the runner command is not run in CI. Parser-only: only the
JUnit results parser is tested. `coderefinery` never installs a runner or
plugin; when one is missing it prints one line naming what to install and
exits 1.

## Parity status

- **Harness**: 36 of 36 cases byte-identical to upstream (crap, dry and
  mutate × the Python, TypeScript, JavaScript and Go fixtures × 3 variants
  each). That covers exit code, stdout and all 55 `.metrics` files raw-equal;
  stderr matches in 36 of 36. CI runs this in golden mode.
- **Parsers**: coderefinery parses source with hand-written parsers that emit
  tree-sitter-shaped trees. `treediff` compares them with the pinned
  tree-sitter grammars ([docs/PARSERS.md](docs/PARSERS.md)):
  - Python: 23 213 / 23 253 files identical (CPython 3.12 stdlib and tests,
    pip, pygments, pytest, coverage).
  - Java: 15 759 / 15 759 (JDK 21 `src.zip`).
  - Go: 7 647 / 7 702 (`$GOROOT/src`). The rest are deliberately invalid
    test inputs, or files where tree-sitter itself builds ERROR nodes.
  - Rust: 10 162 / 10 216 (410 crates). The 54 others all have tree-sitter
    ERROR nodes.
  - TypeScript/TSX/JavaScript: 100% of the files tree-sitter parses cleanly
    (about 14 000).
- **Deviations**: every known difference from upstream is listed in
  [docs/DEVIATIONS.md](docs/DEVIATIONS.md), mostly input that
  tree-sitter would recover with ERROR nodes. Each opt-in fix sits behind a
  flag, such as `mutate --fix-go-coverage`.
- **Additions**: `test`, `tests`, `crap --branches`, `dry --tests`,
  `mutate --attribute` and `mutate --branches` are additive and opt-in. The
  flags are hidden from the upstream `--help` text (only the top-level
  `coderefinery --help` lists `test` and `tests`), and the default `crap`,
  `dry` and `mutate` output, exit codes and `.metrics` files stay
  byte-identical (the harness still passes 36 of 36).

## Layout

```
skills/coderefinery/ the skill: SKILL.md, LICENSE (upstream MIT), scripts/coderefinery (launcher)
go.mod, Makefile    Go module github.com/matheusAle/coderefinery
cmd/, internal/     the binary (cmd/coderefinery) and its packages
tools/, testdata/   parser-parity tools (treediff, oracles) and their corpus
harness/            parity harness: run.sh, matrix.txt, golden/, cmd/compare, fetch-upstream.sh
fixtures/           the four fixture projects the harness runs on, plus setup.sh
                    and the test-ingestion fixtures (*-tests/, setup-tests.sh)
bench/              coderefinery vs upstream timings (bench/RESULTS.md)
docs/               specs of the three upstream tools; ARCHITECTURE, DEVIATIONS, PARSERS,
                    PARSER-BUGS; benchmarks.md; skills-manager.md; design-tests.md
tests/              launcher_test.sh (tests for the skill's launcher)
.github/workflows/  ci.yml (tests + parity), release.yml (tag vX.Y.Z → release assets)
upstream/           gitignored: upstream checkouts made by harness/fetch-upstream.sh
```

## Development

Every script sources `harness/env.sh`, which keeps Go, npm and pip caches and
venvs under `.local/`. Its default `REPO_ROOT` is the author's checkout path, so
elsewhere export `REPO_ROOT=$PWD`, and `REPO_GOROOT_BIN=$(go env GOROOT)/bin`
when Go doesn't come from asdf, before running anything.

```bash
make ci                    # gofmt, go vet, go test ./..., build
tests/launcher_test.sh     # the skill launcher's resolution order, download and checksum paths

fixtures/setup.sh          # once: fixture venv (pytest, coverage), node_modules
harness/run.sh check       # coderefinery vs golden: expect SUMMARY ... cases=36 PASS=36
```

Golden mode needs only the fixtures. Pytest prints its platform and Python
version in the recorded stdout, so a check matches on macOS with Python
3.12.3 (the CI parity job pins it). The `live`, `record` and `selftest` modes
run the Python originals, so fetch them first:

```bash
harness/fetch-upstream.sh  # clones crapper/dryer/mutator at the pinned commits into upstream/, with venvs
harness/run.sh live        # coderefinery and upstream side by side
```

See [harness/README.md](harness/README.md) for the matrix, the flags and the
normalisation rules.

Releasing: set `CODEREFINERY_VERSION` in `skills/coderefinery/scripts/coderefinery`, then push
the matching tag `vX.Y.Z`. `release.yml` checks that the two agree, runs the
tests, builds with `make dist VERSION=vX.Y.Z`, publishes the archives and
`SHA256SUMS` (the module sits at the repo root, so `go install` uses the same
tag), then runs the launcher against the new release on Linux and macOS.

## Credits

- [ub-mode](https://github.com/unclebob/ub-mode) by Robert C. Martin, MIT.
  The skill's principles are his text; see [skills/coderefinery/LICENSE](skills/coderefinery/LICENSE).
- [crapper](https://github.com/unclebob/crapper), [dryer](https://github.com/unclebob/dryer)
  and [mutator](https://github.com/unclebob/mutator) by Robert C. Martin, 0BSD.
  `coderefinery` is a port of their behaviour; the specs are in `docs/spec-*.md`.

## License

TODO: the owner has not chosen a license for this repository yet (coderefinery,
the launcher, the harness). Until one is added, only `skills/coderefinery/LICENSE`
(the upstream MIT notice for the adapted skill text) grants any rights.
