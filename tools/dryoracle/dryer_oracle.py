#!/usr/bin/env python3
"""Upstream dryer oracle: the normalized form of every candidate, per file.

Run it with the upstream dryer venv interpreter, isolated and without
writing bytecode next to the upstream sources:

    upstream/dryer/.venv/bin/python -I -B tools/dryoracle/dryer_oracle.py FILE...

For each FILE it prints "=== FILE <path>" and then one JSON line per
candidate, in document order:

    {"start": 1, "end": 4, "nodes": 35, "fps": 21, "norm": "[:function_definition ...]"}

and, for Clojure files with an unreadable tail, a line
{"warning": "<line>: <msg>"}. When upstream itself crashes on a file (the
comment-callee bug, spec-dryer §4.6) the file's only line is
{"error": "<exception>"}. The language comes from dryer's own
extension table; files it does not support print only the header. Files are
read the way dryer reads them (utf-8, errors=replace, universal newlines).
The upstream sources are imported from <repo root>/upstream/dryer/src (found
from this script's location); nothing is written there.
"""

import json
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(REPO_ROOT / "upstream" / "dryer" / "src"))

from tree_sitter_language_pack import get_parser  # noqa: E402

from dryer import clojure_read  # noqa: E402
from dryer.astnorm import normalize  # noqa: E402
from dryer.discover import language_of  # noqa: E402
from dryer.extract import _PICKERS  # noqa: E402
from dryer.shape import fingerprints, node_count, pr  # noqa: E402
from dryer.treesitter import end_line, start_line  # noqa: E402


LINES = []


def emit(start, end, value):
    LINES.append(json.dumps({"start": start, "end": end, "nodes": node_count(value),
                             "fps": len(fingerprints(value)), "norm": pr(value)}))


def tree_file(language, path, text):
    grammar, pick = _PICKERS[language]
    grammar = grammar(path) if callable(grammar) else grammar
    data = text.encode("utf-8")
    tree = get_parser(grammar).parse(data)
    for node in pick(data, tree.root_node):
        value = normalize(node, data)
        if value is not None:
            emit(start_line(node), end_line(node), value)


def clojure_file(text):
    forms, warning = clojure_read.read_source(text)
    for form in forms:
        if clojure_read.is_candidate_form(form):
            emit(form.line, clojure_read.max_line(form), clojure_read.normalize(form))
    if warning:
        LINES.append(json.dumps({"warning": warning}))


def main(paths):
    for path in paths:
        print(f"=== FILE {path}")
        language = language_of(path)
        text = Path(path).read_text(encoding="utf-8", errors="replace")
        LINES.clear()
        try:
            if language == "clojure":
                clojure_file(text)
            elif language in _PICKERS:
                tree_file(language, path, text)
        except Exception as exc:  # upstream crash, e.g. the comment-callee bug
            LINES[:] = [json.dumps({"error": f"{type(exc).__name__}: {exc}"})]
        for line in LINES:
            print(line)
    sys.stdout.flush()


if __name__ == "__main__":
    main(sys.argv[1:])
