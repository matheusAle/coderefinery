package python

import "github.com/matheusAle/ub-mode/ubgate/internal/syntax"

// Extras (comments, line continuations) follow tree-sitter's reduce rule: a
// node keeps the extras that sit strictly between two of its (non-extra)
// children, counting the hidden layout tokens (NEWLINE, INDENT, DEDENT) as
// children. Leading and trailing extras bubble up to the nearest ancestor
// that encloses them; the module root takes whatever is left.

func attachExtras(root *syntax.Node, extras []token) {
	for _, t := range extras {
		typ := "comment"
		if t.kind == tkLineCont {
			typ = "line_continuation"
		}
		ex := syntax.Leaf(typ, true, t.start, t.end)
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

func enclosingChild(n *syntax.Node, ex *syntax.Node) *syntax.Node {
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

func insertSorted(n *syntax.Node, ex *syntax.Node) {
	i := len(n.Children)
	for i > 0 && n.Children[i-1].StartByte >= ex.EndByte && !(n.Children[i-1].StartByte == n.Children[i-1].EndByte && n.Children[i-1].StartByte <= ex.StartByte) {
		i--
	}
	n.Children = append(n.Children, nil)
	copy(n.Children[i+1:], n.Children[i:])
	n.Children[i] = ex
}

func removeHidden(root *syntax.Node, hidden map[*syntax.Node]bool) {
	syntax.Walk(root, func(n *syntax.Node) bool {
		kept := n.Children[:0]
		for _, c := range n.Children {
			if !hidden[c] {
				kept = append(kept, c)
			}
		}
		n.Children = kept
		return true
	})
}
