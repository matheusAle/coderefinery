package syntax

// Extras (comments, line continuations) follow tree-sitter's reduce rule: a
// node keeps the extras that sit strictly between two of its non-extra
// children. Leading and trailing extras bubble up to the nearest ancestor
// that encloses them; the root takes whatever is left.

// ExtraParent returns the node that keeps ex: the deepest node, from root
// down, reached through inner (non-leaf, non-extra) children whose span
// encloses ex. A child encloses ex when it starts before ex (at or before
// it with startInclusive) and ends at or after ex's end.
func ExtraParent(root, ex *Node, startInclusive bool) *Node {
	n := root
	for {
		next := enclosingChild(n, ex, startInclusive)
		if next == nil {
			return n
		}
		n = next
	}
}

func enclosingChild(n, ex *Node, startInclusive bool) *Node {
	for _, c := range n.Children {
		if len(c.Children) == 0 || c.Extra {
			continue
		}
		if (c.StartByte < ex.StartByte || startInclusive && c.StartByte == ex.StartByte) && ex.EndByte <= c.EndByte {
			return c
		}
	}
	return nil
}

// InsertExtra inserts ex among n's children before the trailing children
// that start at or after ex's end.
func InsertExtra(n, ex *Node) {
	i := len(n.Children)
	for i > 0 && n.Children[i-1].StartByte >= ex.EndByte {
		i--
	}
	n.Children = append(n.Children, nil)
	copy(n.Children[i+1:], n.Children[i:])
	n.Children[i] = ex
}

// RemoveHidden drops every hidden node (a grammar's hidden layout tokens,
// kept only while extras are placed) from the tree.
func RemoveHidden(root *Node, hidden map[*Node]bool) {
	Walk(root, func(n *Node) bool {
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
