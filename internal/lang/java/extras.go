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
		n := root
		for {
			next := enclosingChild(n, ex)
			if next == nil {
				break
			}
			n = next
		}
		insertSorted(n, ex)
	}
}

func enclosingChild(n, ex *syntax.Node) *syntax.Node {
	for _, c := range n.Children {
		if len(c.Children) == 0 || c.Extra {
			continue
		}
		if c.StartByte < ex.StartByte && ex.EndByte <= c.EndByte {
			return c
		}
	}
	return nil
}

func insertSorted(n, ex *syntax.Node) {
	i := len(n.Children)
	for i > 0 && n.Children[i-1].StartByte >= ex.EndByte {
		i--
	}
	n.Children = append(n.Children, nil)
	copy(n.Children[i+1:], n.Children[i:])
	n.Children[i] = ex
}
