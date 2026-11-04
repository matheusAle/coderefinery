// Package java is a hand-written parser that builds tree-sitter-java shaped
// trees (the grammar shipped in tree-sitter-language-pack 1.23.1). Parity
// with upstream tree-sitter is checked with tools/treediff.
//
// Layout: lexer.go (tokens, string/text-block content), parser.go (driver,
// token helpers, extras), decl.go (declarations and members), stmt.go
// (statements), types.go (types), expr.go (expressions).
//
// Ambiguities tree-sitter resolves with GLR (declaration vs expression
// statement, cast vs parenthesized expression, lambda heads, class literals
// and method references on types) are resolved here by speculative parsing:
// try the construct, restore the token position on failure.
package java

import (
	"fmt"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

// Parse parses src (decoded and newline-normalized, see internal/source)
// and returns the root `program` node. Input the parser cannot handle
// yields an error; tree-sitter's ERROR recovery is not emulated.
func Parse(src []byte) (root *syntax.Node, err error) {
	toks, err := lex(string(src))
	if err != nil {
		return nil, err
	}
	p := &parser{src: string(src)}
	var extras []token
	for _, t := range toks {
		if t.kind == tkComment {
			extras = append(extras, t)
		} else {
			p.toks = append(p.toks, t)
		}
	}
	defer func() {
		if r := recover(); r != nil {
			pe, ok := r.(*parseError)
			if !ok {
				panic(r)
			}
			root, err = nil, pe
		}
	}()
	program := p.program()
	attachExtras(program, extras)
	if len(program.Children) > 0 {
		program.StartByte = program.Children[0].StartByte
	} else {
		program.StartByte = len(src)
	}
	program.EndByte = len(src)
	return syntax.Finish(program, src), nil
}

type parser struct {
	src  string
	toks []token
	pos  int
	// noLambda is set while parsing `case` label expressions, where
	// `X ->` is the rule arrow, not a lambda.
	noLambda bool
	// leftPrec: precedence of the binary operator left of the operand
	// being parsed (0 = none), for ambiguousSignCast.
	leftPrec int
}

type parseError struct {
	pos int
	msg string
}

func (e *parseError) Error() string { return fmt.Sprintf("java: %s at byte %d", e.msg, e.pos) }

func (p *parser) fail(format string, args ...any) {
	panic(&parseError{pos: p.cur().start, msg: fmt.Sprintf(format, args...)})
}

// try runs f speculatively: on a parse error (or a nil result) it restores
// the token position and returns nil.
func (p *parser) try(f func() *syntax.Node) (n *syntax.Node) {
	save := p.pos
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(*parseError); !ok {
				panic(r)
			}
			p.pos, n = save, nil
		}
	}()
	n = f()
	if n == nil {
		p.pos = save
	}
	return n
}

func (p *parser) cur() token { return p.peek(0) }

func (p *parser) peek(k int) token {
	if p.pos+k < len(p.toks) {
		return p.toks[p.pos+k]
	}
	return p.toks[len(p.toks)-1]
}

// isOp reports whether the token k ahead is the operator text.
func (p *parser) isOpAt(k int, text string) bool {
	t := p.peek(k)
	return t.kind == tkOp && t.text == text
}

func (p *parser) isOp(text string) bool { return p.isOpAt(0, text) }

func (p *parser) isKwAt(k int, text string) bool {
	t := p.peek(k)
	return t.kind == tkIdent && t.text == text
}

func (p *parser) isKw(text string) bool { return p.isKwAt(0, text) }

func (p *parser) eof() bool { return p.cur().kind == tkEOF }

// next consumes the current token as an anonymous leaf typed by its text.
func (p *parser) next() *syntax.Node {
	t := p.cur()
	p.pos++
	return syntax.Leaf(t.text, false, t.start, t.end)
}

// named consumes the current token as a named leaf.
func (p *parser) named(typ string) *syntax.Node {
	t := p.cur()
	p.pos++
	return syntax.Leaf(typ, true, t.start, t.end)
}

// expect consumes an operator or keyword with exactly this text.
func (p *parser) expect(text string) *syntax.Node {
	t := p.cur()
	if (t.kind == tkOp || t.kind == tkIdent) && t.text == text {
		return p.next()
	}
	p.fail("expected %q, found %q", text, t.text)
	return nil
}

// inner builds a named node over the non-nil children.
func inner(typ string, kids ...*syntax.Node) *syntax.Node {
	out := kids[:0:0]
	for _, k := range kids {
		if k != nil {
			out = append(out, k)
		}
	}
	return syntax.Inner(typ, 0, out...)
}

// field sets n's field name (nil-safe).
func field(name string, n *syntax.Node) *syntax.Node {
	if n != nil {
		n.Field = name
	}
	return n
}

// reserved words: never identifiers (tree-sitter keyword extraction).
var reserved = map[string]bool{
	"abstract": true, "assert": true, "boolean": true, "break": true, "byte": true, "case": true,
	"catch": true, "char": true, "class": true, "continue": true, "default": true, "do": true,
	"double": true, "else": true, "enum": true, "extends": true, "final": true, "finally": true,
	"float": true, "for": true, "if": true, "implements": true, "import": true, "instanceof": true,
	"int": true, "interface": true, "long": true, "native": true, "new": true, "package": true,
	"private": true, "protected": true, "public": true, "return": true, "short": true, "static": true,
	"strictfp": true, "super": true, "switch": true, "synchronized": true, "this": true, "throw": true,
	"throws": true, "transient": true, "try": true, "void": true, "volatile": true, "while": true,
	"true": true, "false": true, "null": true, "non-sealed": true, "_": true,
}

// isIdentAt reports whether the token k ahead can be an identifier.
func (p *parser) isIdentAt(k int) bool {
	t := p.peek(k)
	return t.kind == tkIdent && !reserved[t.text]
}

func (p *parser) isIdent() bool { return p.isIdentAt(0) }

func (p *parser) identifier() *syntax.Node {
	if !p.isIdent() {
		p.fail("expected identifier, found %q", p.cur().text)
	}
	return p.named("identifier")
}

func (p *parser) program() *syntax.Node {
	var kids []*syntax.Node
	for !p.eof() {
		kids = append(kids, p.statement(true))
	}
	return inner("program", kids...)
}

// commaList parses `item (, item)*` appending the separators.
func (p *parser) commaList(kids []*syntax.Node, item func() *syntax.Node) []*syntax.Node {
	kids = append(kids, item())
	for p.isOp(",") {
		kids = append(kids, p.next(), item())
	}
	return kids
}
