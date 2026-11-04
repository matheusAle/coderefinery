// Package rust is a hand-written parser that builds the tree
// tree-sitter-rust (tree-sitter-language-pack 1.23.1) builds for valid Rust:
// same node types, fields, spans and extras placement. See docs/PARSERS.md.
package rust

import "github.com/matheusAle/coderefinery/internal/syntax"

// Parse parses decoded Rust source and returns the source_file root.
func Parse(src []byte) (root *syntax.Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			var pe *parseError
			if e, ok := r.(*parseError); ok {
				pe = e
			}
			if pe == nil {
				panic(r)
			}
			root, err = nil, error(pe)
		}
	}()
	p := &parser{lx: newLexer(src), src: src}
	root = p.sourceFile()
	attachExtras(root, p.lx.comments)
	if len(root.Children) > 0 {
		root.StartByte = root.Children[0].StartByte
	}
	return syntax.Finish(root, src), nil
}

type parser struct {
	lx  *lexer
	src []byte
	pos int
}

func (p *parser) peek() token { return p.lx.at(p.pos) }

// peek2 returns the token after the next one.
func (p *parser) peek2() token { return p.lx.at(p.peek().end) }

func (p *parser) peekN(n int) token {
	t := p.peek()
	for ; n > 0; n-- {
		t = p.lx.at(t.end)
	}
	return t
}

func (p *parser) next() token {
	t := p.peek()
	p.pos = t.end
	return t
}

func (p *parser) at(s string) bool { return p.peek().is(s) }

func (p *parser) atEOF() bool { return p.peek().kind == tkEOF }

// splittable compound tokens: tree-sitter lexes only the tokens valid in the
// current state, so `>>` closes two generic lists, `||` opens and closes a
// closure, `&&` is two references, `<<` opens two qualified paths, and a
// prefix `..=` is `..` followed by `=`.
var splitFirst = map[string]bool{">": true, "|": true, "&": true, "<": true, "..": true}

func (p *parser) atSplit(s string) bool {
	t := p.peek()
	if t.kind != tkPunct {
		return false
	}
	if t.text == s {
		return true
	}
	return splitFirst[s] && len(t.text) > len(s) && t.text[:len(s)] == s
}

// anon turns a consumed token into an anonymous leaf.
func (p *parser) anon(t token) *syntax.Node { return syntax.Leaf(t.text, false, t.start, t.end) }

func (p *parser) expect(s string) *syntax.Node {
	t := p.peek()
	if t.text == s && (t.kind == tkPunct || t.kind == tkIdent) {
		p.pos = t.end
		return p.anon(t)
	}
	if p.atSplit(s) {
		p.pos = t.start + len(s)
		return syntax.Leaf(s, false, t.start, t.start+len(s))
	}
	fail(t.start, "expected %q, got %q", s, t.text)
	return nil
}

func (p *parser) accept(s string) *syntax.Node {
	if p.at(s) || p.atSplit(s) {
		return p.expect(s)
	}
	return nil
}

func (p *parser) ident() *syntax.Node {
	t := p.peek()
	if t.kind != tkIdent {
		fail(t.start, "expected identifier, got %q", t.text)
	}
	p.pos = t.end
	return syntax.Leaf("identifier", true, t.start, t.end)
}

func (p *parser) identAs(typ string) *syntax.Node {
	n := p.ident()
	n.Type = typ
	return n
}

func node(typ string, children ...*syntax.Node) *syntax.Node {
	kept := children[:0:0]
	for _, c := range children {
		if c != nil {
			kept = append(kept, c)
		}
	}
	if len(kept) == 0 {
		panic("node without children: " + typ)
	}
	return syntax.Inner(typ, kept[0].StartByte, kept...)
}

func field(name string, n *syntax.Node) *syntax.Node { return syntax.SetField(name, n) }

// attempt runs f and restores the position when it fails.
func (p *parser) attempt(f func()) (ok bool) {
	save := p.pos
	defer func() {
		if r := recover(); r != nil {
			if _, isParse := r.(*parseError); !isParse {
				panic(r)
			}
			p.pos = save
			ok = false
		}
	}()
	f()
	return true
}

// ---------------------------------------------------------------- statements

func (p *parser) sourceFile() *syntax.Node {
	var children []*syntax.Node
	if len(p.src) >= 2 && p.src[0] == '#' && p.src[1] == '!' && !p.isInnerAttrStart() {
		end := 0
		for end < len(p.src) && p.src[end] != '\n' {
			end++
		}
		if end < len(p.src) {
			end++
		}
		children = append(children, syntax.Leaf("shebang", true, 0, end))
		p.pos = end
	}
	for !p.atEOF() {
		children = append(children, p.statement(false))
	}
	root := &syntax.Node{Type: "source_file", Named: true, Children: children}
	root.StartByte, root.EndByte = len(p.src), len(p.src)
	return root
}

func (p *parser) isInnerAttrStart() bool {
	i := 2
	for i < len(p.src) && (p.src[i] == ' ' || p.src[i] == '\t' || p.src[i] == '\n' || p.src[i] == '\r') {
		i++
	}
	return i < len(p.src) && p.src[i] == '['
}

// statement parses one _statement of source_file, or (declOnly) one
// _declaration_statement of a declaration_list.
func (p *parser) statement(declOnly bool) *syntax.Node {
	if n := p.declaration(); n != nil {
		return n
	}
	if declOnly {
		if p.isMacroStart() {
			return p.macroInvocation(p.simplePath())
		}
		fail(p.peek().start, "expected declaration, got %q", p.peek().text)
	}
	n, _ := p.expressionStatement()
	return n
}

// expressionStatement returns the statement node and whether it is a bare
// tail expression (no `;`, not block-like) that must end the block.
func (p *parser) expressionStatement() (*syntax.Node, bool) {
	if p.startsBlockLike() {
		e := p.expressionWithBlock()
		if !p.continuesAfterBlockLike() {
			// a following `;` is its own empty_statement
			return node("expression_statement", e), false
		}
		e = p.continueExpression(e, 0, exprCtx{})
		return p.finishStatement(e)
	}
	e := p.expression(exprCtx{})
	return p.finishStatement(e)
}

func (p *parser) finishStatement(e *syntax.Node) (*syntax.Node, bool) {
	if p.at(";") {
		return node("expression_statement", e, p.expect(";")), false
	}
	if e.Type == "macro_invocation" {
		return e, false
	}
	return e, true
}

// continuesAfterBlockLike: a block-like expression at statement start ends
// the statement (expression_statement has prec 1) unless the next token
// cannot start a statement, in which case the expression goes on
// (`match x {}.f()`, `if a {} else {}?`).
func (p *parser) continuesAfterBlockLike() bool {
	t := p.peek()
	if t.kind != tkPunct {
		return t.kind == tkIdent && t.text == "as"
	}
	switch t.text {
	case ".", "?", "+", "/", "%", "^", "=", "==", "!=", "<=", ">=", ">", ">>", "<<", "&&", "||",
		"+=", "-=", "*=", "/=", "%=", "^=", "&=", "|=", "<<=", ">>=", "...":
		return true
	}
	return false
}

func (p *parser) blockBody() []*syntax.Node {
	var out []*syntax.Node
	for !p.at("}") {
		if n := p.declaration(); n != nil {
			out = append(out, n)
			continue
		}
		n, tail := p.expressionStatement()
		out = append(out, n)
		if tail && !p.at("}") {
			fail(p.peek().start, "expected ; or }, got %q", p.peek().text)
		}
	}
	return out
}

func (p *parser) block() *syntax.Node {
	var label, colon *syntax.Node
	if p.at("'") {
		label = p.label()
		colon = p.expect(":")
	}
	open := p.expect("{")
	body := p.blockBody()
	close := p.expect("}")
	return node("block", append(append([]*syntax.Node{label, colon, open}, body...), close)...)
}

func (p *parser) label() *syntax.Node {
	q := p.expect("'")
	return node("label", q, p.ident())
}

func (p *parser) lifetime() *syntax.Node {
	q := p.expect("'")
	return node("lifetime", q, p.ident())
}

// isLess: `<` or a compound token that starts with it (`<<`).
func isLess(t token) bool { return t.is("<") || t.is("<<") }

// atTurbofish: `::` followed by type arguments.
func (p *parser) atTurbofish() bool { return p.at("::") && isLess(p.peek2()) }
