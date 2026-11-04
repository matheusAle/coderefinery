package syntax

import (
	"fmt"
	"strings"
)

// Quote renders text the way tools/oracle/oracle.py quote() does. Keep the
// two in lockstep: treediff compares their output byte for byte.
func Quote(text string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range text {
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// Dump renders the tree one node per line, preorder, two spaces per depth:
//
//	<indent>[<field>: ]<kind> [<start>,<end>) <sr>:<sc>-<er>:<ec>[ MISSING][ EXTRA][ <text>]
//
// kind is the bare type for named nodes and Quote(type) for anonymous ones;
// text (Quote of the source slice) is printed for leaves only. This is the
// parity format: tools/oracle/oracle.py prints the same thing for upstream
// tree-sitter.
func Dump(root *Node, src []byte) string {
	var b strings.Builder
	type item struct {
		n     *Node
		depth int
	}
	stack := []item{{root, 0}}
	for len(stack) > 0 {
		it := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		n := it.n
		b.WriteString(strings.Repeat("  ", it.depth))
		if n.Field != "" {
			b.WriteString(n.Field)
			b.WriteString(": ")
		}
		if n.Named {
			b.WriteString(n.Type)
		} else {
			b.WriteString(Quote(n.Type))
		}
		fmt.Fprintf(&b, " [%d,%d) %d:%d-%d:%d", n.StartByte, n.EndByte,
			n.StartPoint.Row, n.StartPoint.Column, n.EndPoint.Row, n.EndPoint.Column)
		if n.Missing {
			b.WriteString(" MISSING")
		}
		if n.Extra {
			b.WriteString(" EXTRA")
		}
		if len(n.Children) == 0 {
			b.WriteByte(' ')
			b.WriteString(Quote(decodeReplace(src[n.StartByte:n.EndByte])))
		}
		b.WriteByte('\n')
		for i := len(n.Children) - 1; i >= 0; i-- {
			stack = append(stack, item{n.Children[i], it.depth + 1})
		}
	}
	return b.String()
}

// SExp renders tree-sitter's own str(node) format: named (and MISSING) nodes
// only, with field names, e.g.
// (module (expression_statement (identifier))).
func SExp(root *Node) string {
	var b strings.Builder
	writeSExp(&b, root, true)
	return b.String()
}

func writeSExp(b *strings.Builder, n *Node, isRoot bool) {
	visible := n.Named || n.Missing
	if visible {
		if !isRoot {
			b.WriteByte(' ')
			if n.Field != "" {
				b.WriteString(n.Field)
				b.WriteString(": ")
			}
		}
		switch {
		case n.Missing && n.Named:
			b.WriteString("(MISSING " + n.Type)
		case n.Missing:
			b.WriteString("(MISSING \"" + n.Type + "\"")
		default:
			b.WriteString("(" + n.Type)
		}
	} else if isRoot {
		if len(n.Children) > 0 {
			b.WriteString("(" + n.Type)
		} else {
			b.WriteString("(\"" + n.Type + "\")")
		}
	}
	for _, c := range n.Children {
		writeSExp(b, c, false)
	}
	if visible {
		b.WriteByte(')')
	}
}

// decodeReplace mirrors bytes.decode("utf-8", "replace") closely enough for
// leaf text (sources are valid UTF-8 after internal/source decoding).
func decodeReplace(b []byte) string {
	return strings.ToValidUTF8(string(b), "�")
}
