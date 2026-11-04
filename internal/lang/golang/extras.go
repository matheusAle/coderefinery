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
