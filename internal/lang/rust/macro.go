package rust

import (
	"sort"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

// Macros: invocations carry opaque token trees; macro_rules! bodies use the
// pattern/repetition forms with metavariables.

// tokenTreeKeywords are the keywords tree-sitter-rust keeps as anonymous
// tokens inside token trees; every other word is an identifier.
var tokenTreeKeywords = map[string]bool{
	"as": true, "async": true, "await": true, "break": true, "const": true, "continue": true, "default": true,
	"enum": true, "fn": true, "for": true, "gen": true, "if": true, "impl": true, "let": true, "loop": true,
	"match": true, "mod": true, "pub": true, "return": true, "static": true, "struct": true, "trait": true,
	"type": true, "union": true, "unsafe": true, "use": true, "where": true, "while": true,
}

var fragmentSpecifiers = map[string]bool{
	"block": true, "expr": true, "expr_2021": true, "ident": true, "item": true, "lifetime": true, "literal": true,
	"meta": true, "pat": true, "pat_param": true, "path": true, "stmt": true, "tt": true, "ty": true, "vis": true,
}

var closing = map[string]string{"(": ")", "[": "]", "{": "}"}

// isMacroStart: a path followed by `!` and a delimiter.
func (p *parser) isMacroStart() bool {
	i := 0
	if p.peekN(i).is("::") {
		i++
	}
	for {
		t := p.peekN(i)
		if t.kind != tkIdent {
			return false
		}
		i++
		if !p.peekN(i).is("::") {
			break
		}
		i++
	}
	return p.peekN(i).is("!") && p.isDelimAt(p.peekN(i+1))
}

func (p *parser) macroInvocation(path *syntax.Node) *syntax.Node {
	bang := p.expect("!")
	return node("macro_invocation", field("macro", path), bang, p.tokenTree(false))
}

// tokenTree parses a delimited token tree. In macro_rules! right-hand sides
// (rules=true) `$name` is a metavariable and `$( … ) sep op` a
// token_repetition.
func (p *parser) tokenTree(rules bool) *syntax.Node {
	open := p.peek()
	closeTok, ok := closing[open.text]
	if !ok || open.kind != tkPunct {
		fail(open.start, "expected token tree")
	}
	parts := []*syntax.Node{p.expect(open.text)}
	for !p.at(closeTok) {
		parts = append(parts, p.treeToken(rules, false))
	}
	parts = append(parts, p.expect(closeTok))
	return node("token_tree", parts...)
}

func (p *parser) tokenTreePattern() *syntax.Node {
	open := p.peek()
	closeTok, ok := closing[open.text]
	if !ok || open.kind != tkPunct {
		fail(open.start, "expected token tree pattern")
	}
	parts := []*syntax.Node{p.expect(open.text)}
	for !p.at(closeTok) {
		parts = append(parts, p.treeToken(true, true))
	}
	parts = append(parts, p.expect(closeTok))
	return node("token_tree_pattern", parts...)
}

// treeToken parses one element of a token tree.
func (p *parser) treeToken(rules, pattern bool) *syntax.Node {
	t := p.peek()
	switch t.kind {
	case tkEOF:
		fail(t.start, "unterminated token tree")
	case tkInt, tkFloat, tkChar, tkString, tkRawString:
		return p.literal()
	case tkPunct:
		if _, isOpen := closing[t.text]; isOpen {
			if pattern {
				return p.tokenTreePattern()
			}
			return p.tokenTree(rules)
		}
		if t.text == "$" && rules {
			if n := p.dollar(pattern); n != nil {
				return n
			}
		}
		if t.text == ")" || t.text == "]" || t.text == "}" {
			fail(t.start, "unbalanced %q", t.text)
		}
		p.next()
		return p.anon(t)
	}
	p.next()
	switch {
	case t.text == "true" || t.text == "false":
		return node("boolean_literal", p.anon(t))
	case t.text == "mut":
		return syntax.Leaf("mutable_specifier", true, t.start, t.end)
	case t.text == "self" || t.text == "super" || t.text == "crate":
		return syntax.Leaf(t.text, true, t.start, t.end)
	case primitiveTypes[t.text]:
		return syntax.Leaf("primitive_type", true, t.start, t.end)
	case tokenTreeKeywords[t.text] || t.text == "_":
		return p.anon(t)
	}
	return syntax.Leaf("identifier", true, t.start, t.end)
}

// dollar handles `$name`, `$name:frag` and `$( … )` inside macro_rules!.
func (p *parser) dollar(pattern bool) *syntax.Node {
	d := p.peek()
	n := p.lx.at(d.end)
	if n.start == d.end && n.kind == tkIdent {
		p.pos = n.end
		mv := syntax.Leaf("metavariable", true, d.start, n.end)
		if pattern && p.at(":") {
			if f := p.peek2(); f.kind == tkIdent && fragmentSpecifiers[f.text] {
				colon := p.expect(":")
				p.next()
				spec := node("fragment_specifier", p.anon(f))
				return node("token_binding_pattern", field("name", mv), colon, field("type", spec))
			}
		}
		return mv
	}
	if !n.is("(") {
		return nil
	}
	parts := []*syntax.Node{p.expect("$"), p.expect("(")}
	for !p.at(")") {
		parts = append(parts, p.treeToken(true, pattern))
	}
	parts = append(parts, p.expect(")"))
	// optional hidden separator /[^+*?]+/, then the repetition operator
	sep := p.lx.skip(p.pos)
	end := sep
	for end < len(p.src) && p.src[end] != '+' && p.src[end] != '*' && p.src[end] != '?' {
		end++
	}
	p.pos = end
	op := p.peek()
	if !op.is("+") && !op.is("*") && !op.is("?") && !(op.kind == tkPunct && len(op.text) > 1 && (op.text[0] == '+' || op.text[0] == '*')) {
		fail(op.start, "expected repetition operator")
	}
	opNode := syntax.Leaf(op.text[:1], false, op.start, op.start+1)
	p.pos = op.start + 1
	parts = append(parts, opNode)
	if pattern {
		return node("token_repetition_pattern", parts...)
	}
	return node("token_repetition", parts...)
}

func (p *parser) macroDefinition() *syntax.Node {
	mr := p.next()
	bang := p.expect("!")
	kw := syntax.Leaf("macro_rules!", false, mr.start, bang.EndByte)
	parts := []*syntax.Node{kw, field("name", p.ident())}
	open := p.peek()
	closeTok, ok := closing[open.text]
	if !ok {
		fail(open.start, "expected macro_rules! body")
	}
	parts = append(parts, p.expect(open.text))
	for !p.at(closeTok) {
		left := p.tokenTreePattern()
		arrow := p.expect("=>")
		right := p.tokenTree(true)
		parts = append(parts, node("macro_rule", field("left", left), arrow, field("right", right)))
		if !p.at(closeTok) {
			parts = append(parts, p.expect(";"))
		}
	}
	parts = append(parts, p.expect(closeTok))
	if open.text != "{" {
		parts = append(parts, p.expect(";"))
	}
	return node("macro_definition", parts...)
}

// ---------------------------------------------------------------- extras

// attachExtras places each comment in the deepest node that has non-extra
// children on both sides of it (tree-sitter's reduce rule).
func attachExtras(root *syntax.Node, comments map[int]*syntax.Node) {
	starts := make([]int, 0, len(comments))
	for s := range comments {
		starts = append(starts, s)
	}
	sort.Ints(starts)
	for _, s := range starts {
		ex := comments[s]
		syntax.InsertExtra(syntax.ExtraParent(root, ex, false), ex)
	}
}
