# Installing and troubleshooting coderefinery

Open this when the launcher exits 127, when `coderefinery --version` is not the
one you expect, or when a gate finds no files.

## Launcher resolution order

`scripts/coderefinery` (POSIX `sh`) runs the first binary it finds, with every
argument unchanged:

1. `$CODEREFINERY_BIN`, when set. It must be an executable file, else exit 127
   (`CODEREFINERY_BIN=<path> is not an executable file`).
2. `coderefinery` on `PATH`. The launcher sets `CODEREFINERY_LAUNCHER=1` before
   it execs it, so a launcher that is itself on `PATH` as `coderefinery` does not
   call itself forever.
3. The cache: `${XDG_CACHE_HOME:-$HOME/.cache}/coderefinery/<version>/coderefinery`.
4. The GitHub release asset `coderefinery_<version>_<os>_<arch>.tar.gz`, checked
   against the release's `SHA256SUMS`, unpacked into the cache. Needs `curl` or
   `wget`, `sha256sum` or `shasum`, and `tar`. Release builds: macOS and Linux,
   amd64 and arm64. There is no Windows build.
5. `go install github.com/matheusAle/coderefinery/cmd/coderefinery@<version>`
   into the cache directory (Go 1.26+).

Otherwise it exits 127 with a message.

## Variables

| variable | effect |
|---|---|
| `CODEREFINERY_BIN` | Path of the binary to run. The way to use a local build (`make build` writes `bin/coderefinery`) or another version. |
| `CODEREFINERY_VERSION` | Not an environment override. It is pinned once, at the top of the launcher; a release tag must match it. |
| `CODEREFINERY_RELEASE_URL` | Base URL of the release downloads (a mirror). Default `https://github.com/matheusAle/coderefinery/releases/download`. |
| `XDG_CACHE_HOME`, `HOME` | Cache location. One of them must be set. |
| `CODEREFINERY_LAUNCHER` | Set by the launcher (step 2). Do not set it yourself. |

## Exit 127 messages

| message | do |
|---|---|
| `CODEREFINERY_BIN=... is not an executable file` | Fix the path or `chmod +x` it. |
| `set HOME or XDG_CACHE_HOME (cache location)` | Export one of them. |
| `no release build for <os>/<arch>` | Install Go 1.26+ (step 5) or build from source and set `CODEREFINERY_BIN`. |
| `download failed: <url>` / `neither curl nor wget is installed` | Network or tool problem; install curl, or set `CODEREFINERY_RELEASE_URL` to a mirror. |
| `checksum mismatch for <asset> ...; not installed` | Do not bypass it. Retry; if it persists, install by hand and check the archive against `SHA256SUMS`. |
| `go is not installed` | Install Go, or a release binary on `PATH`. |
| `could not install coderefinery <version>. ...` | Every step failed. Install it yourself and put it on `PATH`, or set `CODEREFINERY_BIN`. |

Install by hand: download `coderefinery_<version>_<os>_<arch>.tar.gz` from
https://github.com/matheusAle/coderefinery/releases, check it against
`SHA256SUMS`, put `coderefinery` on `PATH`. Or
`go install github.com/matheusAle/coderefinery/cmd/coderefinery@latest`.
Check with `coderefinery --version`.

A stale or broken cached binary: delete the cache directory of that version
(`~/.cache/coderefinery/<version>/`, written out as a literal path) and run again.

## A gate finds nothing or ignores paths

These are upstream behaviours, kept for parity:

- Test directories are detected on the absolute path. A project that sits
  under a directory named `test`, `tests`, `spec`, `testdata`, ... finds no
  source files. Run it from a copy outside such a path.
- Naming a test directory explicitly yields nothing. Naming a test file works.
- `-s <path>` and `--changed` ignore positional paths.
- A path argument that does not exist is used as a substring filter on the
  absolute path, not reported as an error.
- `--changed` needs git. It exits with git's own code (128 outside a
  repository).
- A file the parser cannot read (syntax errors, some rare constructs) contributes
  no functions, candidates or sites, silently. The language reference lists the
  known cases.
