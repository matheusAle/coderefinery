// Package syntax holds the one concrete-syntax-tree type every coderefinery tool
// walks. Hand-written parsers (internal/lang/<language>) build trees with the
// same shape tree-sitter would build for the pinned grammar
// (tree-sitter 0.26.0, tree-sitter-language-pack 1.23.1): same node type
// names, same named/anonymous flag, same field names, same byte offsets and
// row/column points, same nesting. The tools are 1:1 ports of upstream
// Python code that walks py-tree-sitter nodes, so every accessor here mirrors
// a py-tree-sitter attribute.
package syntax

// Point is a 0-based row and a 0-based column counted in BYTES (tree-sitter
// semantics), not runes.
type Point struct {
	Row    int
	Column int
}

// Node is one CST node. Children holds every child, named and anonymous,
// including extras (comments, line continuations) in source order, exactly
// like py-tree-sitter's node.children.
type Node struct {
	Type       string // tree-sitter node.type ("identifier", "if", "+", "ERROR")
	Named      bool   // node.is_named
	Missing    bool   // node.is_missing (zero-width node inserted by error recovery)
	Extra      bool   // node.is_extra (comments and other extras)
	Field      string // field name of this node within its parent, "" when none
	StartByte  int    // UTF-8 byte offset, inclusive
	EndByte    int    // UTF-8 byte offset, exclusive
	StartPoint Point
	EndPoint   Point
	Parent     *Node
	Children   []*Node
}

// Tree is a parsed file. Source is the exact byte slice the offsets refer to
// (the UTF-8 re-encoding of the decoded text, see internal/source).
type Tree struct {
	Root     *Node
	Source   []byte
	Language string // grammar name: python, typescript, tsx, javascript, go, java, rust
}

// ChildCount is node.child_count.
func (n *Node) ChildCount() int { return len(n.Children) }

// Child is node.child(i), nil when out of range.
func (n *Node) Child(i int) *Node {
	if i < 0 || i >= len(n.Children) {
		return nil
	}
	return n.Children[i]
}

// NamedChildren is node.named_children.
func (n *Node) NamedChildren() []*Node {
	var out []*Node
	for _, c := range n.Children {
		if c.Named {
			out = append(out, c)
		}
	}
	return out
}

// NamedChildCount is node.named_child_count.
func (n *Node) NamedChildCount() int {
	count := 0
	for _, c := range n.Children {
		if c.Named {
			count++
		}
	}
	return count
}

// ChildByFieldName is node.child_by_field_name(name): the first child with
// that field, or nil.
func (n *Node) ChildByFieldName(name string) *Node {
	for _, c := range n.Children {
		if c.Field == name {
			return c
		}
	}
	return nil
}

// ChildrenByFieldName is node.children_by_field_name(name).
func (n *Node) ChildrenByFieldName(name string) []*Node {
	var out []*Node
	for _, c := range n.Children {
		if c.Field == name {
			out = append(out, c)
		}
	}
	return out
}

// ChildOfType is upstream's child_of_type(node, *types): the first direct
// child whose type is one of types, or nil.
func (n *Node) ChildOfType(types ...string) *Node {
	for _, c := range n.Children {
		for _, t := range types {
			if c.Type == t {
				return c
			}
		}
	}
	return nil
}

// HasChildOfType reports whether any direct child has one of the types.
func (n *Node) HasChildOfType(types ...string) bool { return n.ChildOfType(types...) != nil }

// Text is data[start_byte:end_byte] as a string.
func (n *Node) Text(src []byte) string { return string(src[n.StartByte:n.EndByte]) }

// IsError reports node.type == "ERROR".
func (n *Node) IsError() bool { return n.Type == "ERROR" }

// HasError is node.has_error: the node or a descendant is ERROR or MISSING.
func (n *Node) HasError() bool {
	found := false
	Walk(n, func(c *Node) bool {
		if c.IsError() || c.Missing {
			found = true
		}
		return !found
	})
	return found
}

// NextSibling is node.next_sibling.
func (n *Node) NextSibling() *Node {
	if n.Parent == nil {
		return nil
	}
	for i, c := range n.Parent.Children {
		if c == n {
			return n.Parent.Child(i + 1)
		}
	}
	return nil
}

// PrevSibling is node.prev_sibling.
func (n *Node) PrevSibling() *Node {
	if n.Parent == nil {
		return nil
	}
	for i, c := range n.Parent.Children {
		if c == n {
			return n.Parent.Child(i - 1)
		}
	}
	return nil
}

// StartLine is upstream treesitter.start_line: start row + 1.
func (n *Node) StartLine() int { return n.StartPoint.Row + 1 }

// EndLine is upstream treesitter.end_line: end row + 1, except a node that
// ends at column 0 ends on the previous row (but never before StartLine).
func (n *Node) EndLine() int {
	if n.EndPoint.Column == 0 {
		return max(n.StartLine(), n.EndPoint.Row)
	}
	return n.EndPoint.Row + 1
}

// Ancestors yields parent, grandparent, ... up to the root.
func (n *Node) Ancestors(yield func(*Node) bool) {
	for p := n.Parent; p != nil; p = p.Parent {
		if !yield(p) {
			return
		}
	}
}

// Descendants is upstream treesitter.descendants: the node and its whole
// subtree in preorder, iteratively (no recursion, so 1200-deep trees are fine).
func (n *Node) Descendants(yield func(*Node) bool) {
	stack := []*Node{n}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !yield(cur) {
			return
		}
		for i := len(cur.Children) - 1; i >= 0; i-- {
			stack = append(stack, cur.Children[i])
		}
	}
}

// Walk visits n and its subtree in preorder. When visit returns false the
// node's subtree is skipped (not the rest of the walk). Iterative.
func Walk(n *Node, visit func(*Node) bool) {
	stack := []*Node{n}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if !visit(cur) {
			continue
		}
		for i := len(cur.Children) - 1; i >= 0; i-- {
			stack = append(stack, cur.Children[i])
		}
	}
}
