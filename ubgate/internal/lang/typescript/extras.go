package typescript

import (
	"sort"

	"github.com/matheusAle/ub-mode/ubgate/internal/syntax"
)

// attachExtras places comments where tree-sitter's reduce rule puts them: a
// node keeps the extras that sit strictly between two of its non-extra
// children; leading and trailing extras bubble up to the nearest ancestor
// that encloses them, and the program root takes whatever is left.
func attachExtras(root *syntax.Node, comments []*syntax.Node) {
	sort.SliceStable(comments, func(i, j int) bool { return comments[i].StartByte < comments[j].StartByte })
	for _, c := range comments {
		n := root
		for {
			next := enclosingChild(n, c)
			if next == nil {
				break
			}
			n = next
		}
		insertSorted(n, c)
	}
}

// enclosingChild returns the inner child of n whose span encloses ex
// (spans are tight around children, so ex then sits between or inside them).
func enclosingChild(n, ex *syntax.Node) *syntax.Node {
	for _, c := range n.Children {
		if len(c.Children) == 0 || c.Extra {
			continue
		}
		if c.StartByte <= ex.StartByte && ex.EndByte <= c.EndByte {
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
