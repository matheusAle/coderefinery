# Rust

Files: `.rs`. Skipped: `target/`, test directories, and `mod tests` blocks.
Namespace: `crate::module`; methods are `crate::module::Type`.

## crap and mutate: coverage

- Per nearest `Cargo.toml` (workspaces included): `cargo llvm-cov --lcov` when
  `cargo-llvm-cov` is installed, else `cargo tarpaulin --out Lcov`.
- With neither installed, crap installs cargo-llvm-cov
  (`rustup component add llvm-tools-preview`, `cargo install cargo-llvm-cov --locked`)
  and says so on stderr (upstream behaviour). If that fails, Rust scores 0%.
- mutate runs `cargo test` in the nearest `Cargo.toml` directory.
- stderr `Rust coverage exited <n> in <module>` means a failed build or tests.

## coderefinery test

- `cargo llvm-cov nextest --no-report --no-fail-fast` with a generated nextest
  profile, then `cargo llvm-cov report --json` and `--lcov`.
- Needs cargo-nextest and cargo-llvm-cov:
  `install cargo-nextest: cargo install cargo-nextest --locked`,
  `install cargo-llvm-cov: cargo install cargo-llvm-cov --locked`.
  Plain `cargo test` has no stable JUnit output, so it is not supported.
- The generated `--config-file` replaces the project's `.config/nextest.toml`
  for these runs.
- nextest runs no doctests: under `mutate --attribute`, a mutant only a doctest
  kills survives.
- Test ids start with the nextest binary id (`crate`, `crate::bin/x`), plus the
  file for a `tests/<x>.rs` binary.
- Status: sample-only.

## Branches

- Granularity `region`, from llvm-cov JSON: `region 13:5-15:6 never executed`.
  Real `branch` data only with llvm-cov `--branch` (nightly), and then it says
  branch.
- tarpaulin reports lines only: `No branch data: rust (tarpaulin reports lines only)`.
  Use `coderefinery test` (llvm-cov) for branch lines.

## Approximate or different

- Only `mod tests` is skipped. Functions in `mod test` or other `#[cfg(test)]`
  modules are scored like production code.
- A `fn` directly inside a block is not an entry (charged to the enclosing
  function); a `fn` in an `impl` inside a block is.
- Files tree-sitter-rust itself cannot parse cleanly (`try!(...)`, `~` in token
  trees, `str![...]`, negative const arguments, ...) are parsed where the meaning
  is unambiguous, otherwise they contribute nothing.
