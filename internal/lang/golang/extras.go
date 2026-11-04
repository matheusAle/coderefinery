package golang

import "github.com/matheusAle/coderefinery/internal/syntax"

// Comments follow tree-sitter's reduce rule: a node keeps the extras that
// sit strictly between two of its (non-extra) children, counting the hidden
// "\n" terminators as children. Leading and trailing comments bubble up to
// the nearest enclosing node; source_file takes whatever is left.

func attachExtras(root *syntax.Node, comments []tok) {
	for _, t := range comments {
		ex := syntax.Leaf("comment", true, t.start, t.end)
		ex.Extra = true
		syntax.InsertExtra(syntax.ExtraParent(root, ex, false), ex)
	}
}
