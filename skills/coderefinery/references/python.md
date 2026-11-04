# Python

Files: `.py`. Skipped: `test_*.py`, `*_test.py`, `conftest.py` and test
directories (unless `dry --tests`). Namespace: the dotted module path with a
leading `src/` removed; methods are `module.Class`.

## crap and mutate: coverage

- Projects: for each file, the nearest directory holding `pyproject.toml`, then
  `setup.cfg`, `setup.py`, `pytest.ini`, `tox.ini` (each marker walked all the
  way up before the next is tried), else the root.
- Interpreter: `<project>/.venv/bin/python`, else `<project>/venv/bin/python`,
  else `python3` from `PATH`.
- Runner: pytest when `pytest.ini` or `conftest.py` exists or `pytest` appears in
  `pyproject.toml`/`setup.cfg`/`tox.ini`; otherwise `unittest discover`.
- Command: `<py> -m coverage run --source=<packages> -m pytest`, then
  `coverage lcov` into `target/coverage/python[/<module>]/lcov.info`.
- crap and mutate install `coverage` (and `pytest`) into that interpreter with
  pip when they are missing, and say so on stderr (upstream behaviour). Prefer a
  project venv that already has them.
- The LCOV has no branch data, so Python CRAP uses line coverage.
- mutate runs `<py> -m pytest` or `<py> -m unittest discover` in the nearest
  `pyproject.toml`/`pytest.ini`/`setup.cfg` directory, once per mutant.
- A package directory named `build`, `coverage`, `out`, `dist` or `target` is
  left out of the worker copy (upstream), so mutants in modules that import it
  are false KILLED (`· by <build>` with `--attribute`). Add `--fix-overlay`
  (on with `--attribute`): it links such a directory when it holds `.py` files.
- stderr `Python tests exited <n> in <pkg>. Coverage was still recorded.` means
  tests failed: fix them before trusting CRAP. `coverage is missing in <pkg>`
  means every function scores 0%.

## coderefinery test

- `<py> -m coverage run --branch ... -m pytest --junitxml=<results> -o junit_family=xunit1 --maxfail=0`,
  then coverage JSON (with contexts) and LCOV. With pytest-cov installed it runs
  `pytest --cov --cov-branch --cov-context=test` instead.
- coderefinery never installs anything here. Hints (exit 1):
  `coverage is not installed for <py>: run <py> -m pip install coverage`,
  `pytest is not installed for <py>: run <py> -m pip install pytest`,
  `unittest has no JUnit reporter: install pytest (it runs unittest suites) or pass --test-command`.
- Test ids are pytest node ids (`tests/test_x.py::TestA::test_b[case]`).
- pytest-cov's per-test contexts narrow `mutate --attribute`: each mutant runs
  only the tests that cover its line (at most 200), and a survivor is re-run
  with the full suite. Without pytest-cov every mutant runs the full suite.
- Status: live-tested.

## Branches

- Granularity `branch`, from coverage.py arcs: statement-level `if`, `elif`,
  `for`, `while`, `match`, `try`. Short-circuit `and`/`or` and ternaries are not
  branches here.
- `crap --branches` always needs `coderefinery test` first, because the crap
  LCOV has no branch data: `No branch data: python (run coderefinery test)`.

## Approximate or different

- Complexity counts nested defs, lambdas and comprehension `if`s in the
  enclosing function. A `case p if g:` guard counts twice (upstream quirk).
- Files the parser rejects contribute nothing: a BOM at offset 0, Python 2
  backticks, comments inside f-string interpolations, invalid code.
- A file outside the root without `/src/` gets a namespace with a leading `.`.
