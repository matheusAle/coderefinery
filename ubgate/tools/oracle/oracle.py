#!/usr/bin/env python3
"""Upstream tree-sitter oracle for ubgate parser parity.

Prints the tree-sitter CST of each file in exactly the format of ubgate's
Go dumper (internal/syntax.Dump), or tree-sitter's own str(node) with --sexp.

Run it with an upstream venv interpreter (tree-sitter 0.26.0,
tree-sitter-language-pack 1.23.1), never the system Python:

    upstream/crapper/.venv/bin/python -I ubgate/tools/oracle/oracle.py [--lang L] [--sexp] FILE...

Each file is read the way every upstream tool reads it:
read_text(encoding="utf-8", errors="replace") (universal newlines), then
re-encoded as UTF-8. Byte offsets refer to that re-encoding.

With more than one FILE (or --headers) each tree is preceded by a line
"=== FILE <path>". Language defaults to the extension rule shared by
crapper/dryer/mutator: .py python, .go go, .java java, .rs rust,
.tsx tsx, .js .jsx .mjs .cjs javascript, .ts .mts .cts typescript.
"""

import sys
from pathlib import Path

EXT = {
    ".py": "python",
    ".go": "go",
    ".java": "java",
    ".rs": "rust",
    ".tsx": "tsx",
    ".js": "javascript",
    ".jsx": "javascript",
    ".mjs": "javascript",
    ".cjs": "javascript",
    ".ts": "typescript",
    ".mts": "typescript",
    ".cts": "typescript",
}


def quote(text: str) -> str:
    """Must stay identical to syntax.Quote in Go."""
    out = ['"']
    for ch in text:
        o = ord(ch)
        if ch == "\\":
            out.append("\\\\")
        elif ch == '"':
            out.append('\\"')
        elif ch == "\n":
            out.append("\\n")
        elif ch == "\r":
            out.append("\\r")
        elif ch == "\t":
            out.append("\\t")
        elif o < 0x20 or o == 0x7F:
            out.append("\\x%02x" % o)
        else:
            out.append(ch)
    out.append('"')
    return "".join(out)


def dump(data: bytes, tree, write) -> None:
    """One line per node, preorder, two spaces of indent per depth.

    <indent>[<field>: ]<kind> [<start_byte>,<end_byte>) <sr>:<sc>-<er>:<ec>[ MISSING][ EXTRA][ <text>]
    kind = bare type when named, quoted type when anonymous. text = quoted
    source text, printed only for leaves (child_count == 0).
    """
    cursor = tree.walk()
    depth = 0
    while True:
        node = cursor.node
        field = cursor.field_name
        kind = node.type if node.is_named else quote(node.type)
        sp, ep = node.start_point, node.end_point
        line = "%s%s%s [%d,%d) %d:%d-%d:%d" % (
            "  " * depth,
            field + ": " if field else "",
            kind,
            node.start_byte,
            node.end_byte,
            sp[0],
            sp[1],
            ep[0],
            ep[1],
        )
        if node.is_missing:
            line += " MISSING"
        if node.is_extra:
            line += " EXTRA"
        if node.child_count == 0:
            line += " " + quote(data[node.start_byte : node.end_byte].decode("utf-8", "replace"))
        write(line + "\n")
        if cursor.goto_first_child():
            depth += 1
            continue
        while not cursor.goto_next_sibling():
            if not cursor.goto_parent():
                return
            depth -= 1


def main(argv: list[str]) -> int:
    lang = None
    sexp = False
    headers = False
    files: list[str] = []
    i = 0
    while i < len(argv):
        arg = argv[i]
        if arg == "--lang":
            lang = argv[i + 1]
            i += 2
            continue
        if arg == "--sexp":
            sexp = True
        elif arg == "--headers":
            headers = True
        elif arg in ("-h", "--help"):
            print(__doc__)
            return 0
        else:
            files.append(arg)
        i += 1
    if not files:
        print(__doc__, file=sys.stderr)
        return 2
    from tree_sitter_language_pack import get_parser

    headers = headers or len(files) > 1
    out = sys.stdout
    status = 0
    for name in files:
        language = lang or EXT.get(Path(name).suffix)
        if language is None:
            print(f"oracle: no language for {name}", file=sys.stderr)
            status = 1
            continue
        text = Path(name).read_text(encoding="utf-8", errors="replace")
        data = text.encode("utf-8")
        tree = get_parser(language).parse(data)
        if headers:
            out.write(f"=== FILE {name}\n")
        if sexp:
            out.write(str(tree.root_node) + "\n")
        else:
            dump(data, tree, out.write)
    out.flush()
    return status


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
