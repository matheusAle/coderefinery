package syntax

import "sort"

// Builders: parsers only have to decide types, flags, fields and byte spans.
// Finish then sets Parent pointers and derives every Point from the byte
// offsets, so row/column bookkeeping lives in one place.

// Leaf makes a childless node spanning [start, end).
func Leaf(typ string, named bool, start, end int) *Node {
	return &Node{Type: typ, Named: named, StartByte: start, EndByte: end}
}

// Inner makes a node whose span is [first child start, last child end).
// With no children it is zero-width at pos.
func Inner(typ string, pos int, children ...*Node) *Node {
	n := &Node{Type: typ, Named: true, Children: children, StartByte: pos, EndByte: pos}
	n.FitChildren()
	return n
}

// WithField sets the field name and returns the node (for chaining).
func (n *Node) WithField(field string) *Node {
	n.Field = field
	return n
}

// FitChildren resets the span to cover the children (no-op when childless).
func (n *Node) FitChildren() {
	if len(n.Children) == 0 {
		return
	}
	n.StartByte = n.Children[0].StartByte
	n.EndByte = n.Children[len(n.Children)-1].EndByte
}

// LineIndex converts byte offsets to tree-sitter points.
type LineIndex struct {
	starts []int // byte offset of the first byte of each row
}

// NewLineIndex indexes src. Only '\n' starts a new row (tree-sitter rule;
// sources are newline-normalized before parsing).
func NewLineIndex(src []byte) *LineIndex {
	starts := []int{0}
	for i, b := range src {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &LineIndex{starts: starts}
}

// Point returns the row and byte column of offset.
func (li *LineIndex) Point(offset int) Point {
	row := sort.Search(len(li.starts), func(i int) bool { return li.starts[i] > offset }) - 1
	if row < 0 {
		row = 0
	}
	return Point{Row: row, Column: offset - li.starts[row]}
}

// Finish sets Parent pointers and Start/EndPoint for the whole tree rooted
// at root, from the byte offsets. Iterative.
func Finish(root *Node, src []byte) *Node {
	li := NewLineIndex(src)
	stack := []*Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		n.StartPoint = li.Point(n.StartByte)
		n.EndPoint = li.Point(n.EndByte)
		for _, c := range n.Children {
			c.Parent = n
			stack = append(stack, c)
		}
	}
	return root
}
