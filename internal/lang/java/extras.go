package java

import "github.com/matheusAle/coderefinery/internal/syntax"

// Comments are extras. tree-sitter keeps an extra inside the deepest node
// that has (non-extra) children on both sides of it; leading and trailing
// extras bubble up to the nearest enclosing ancestor, the program root
// taking whatever is left.
func attachExtras(root *syntax.Node, extras []token) {
	for _, t := range extras {
		ex := syntax.Leaf(t.typ, true, t.start, t.end)
		ex.Extra = true
		syntax.InsertExtra(syntax.ExtraParent(root, ex, false), ex)
	}
}
