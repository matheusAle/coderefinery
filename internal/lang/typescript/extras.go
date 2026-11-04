package typescript

import (
	"sort"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

// attachExtras places comments where tree-sitter's reduce rule puts them: a
// node keeps the extras that sit strictly between two of its non-extra
// children; leading and trailing extras bubble up to the nearest ancestor
// that encloses them, and the program root takes whatever is left.
func attachExtras(root *syntax.Node, comments []*syntax.Node) {
	sort.SliceStable(comments, func(i, j int) bool { return comments[i].StartByte < comments[j].StartByte })
	for _, c := range comments {
		syntax.InsertExtra(syntax.ExtraParent(root, c, true), c)
	}
}
