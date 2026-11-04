// Package typescript is a hand-written parser for three tree-sitter grammars
// that share one code base: tree-sitter-typescript's `typescript` and `tsx`
// grammars and tree-sitter-javascript (which includes JSX). It produces trees
// with the exact shape of the pinned grammars (tree-sitter-language-pack
// 1.23.1): node types, named flags, fields, byte spans and extras.
//
// Dialects: javascript = JS + JSX; typescript = TS without JSX (`<T>x` is a
// type assertion); tsx = TS + JSX.
package typescript

import (
	"errors"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

type parser struct {
	src      []byte
	ts       bool // TypeScript syntax (types, modifiers, declarations)
	jsx      bool // JSX elements
	tok      token
	prevEnd  int
	comments []*syntax.Node

	// noASI is true inside brackets, where tree-sitter's scanner never
	// offers an automatic semicolon, so a line break cannot end the
	// expression early (`(a\nas T)`).
	noASI bool
	// noIn forbids the `in` operator (for-statement heads).
	noIn bool
	// caseBody / exporting tell the javascript block rule where a block
	// sits (see blockASI).
	caseBody  bool
	exporting bool
	// asType: outside brackets in the type after `as`/`satisfies` (see
	// asTarget).
	asType bool
	// exprTypeArgs: inside the type arguments of a call/instantiation,
	// where an import type makes the `<` a comparison unless
	// importTypeArgs (the fallback parse) is set.
	exprTypeArgs           bool
	importTypeArgs         bool
	rejectedImportTypeArgs bool

	arrowFail map[int]bool // offsets where an arrow-function attempt failed
}

// ParseTypeScript parses with the tree-sitter-typescript `typescript` grammar.
func ParseTypeScript(src []byte) (*syntax.Node, error) { return parse(src, true, false) }

// ParseTSX parses with the tree-sitter-typescript `tsx` grammar.
func ParseTSX(src []byte) (*syntax.Node, error) { return parse(src, true, true) }

// ParseJavaScript parses with the tree-sitter-javascript grammar.
func ParseJavaScript(src []byte) (*syntax.Node, error) { return parse(src, false, true) }

func parse(src []byte, ts, jsx bool) (*syntax.Node, error) {
	root, err, rejected := parseOnce(src, ts, jsx, false)
	if err != nil && rejected {
		// tree-sitter reads `f<typeof import("m")>()` as comparisons and
		// recovers with ERROR nodes when they do not parse; we fall back
		// to the call reading instead.
		root, err, _ = parseOnce(src, ts, jsx, true)
	}
	return root, err
}

func parseOnce(src []byte, ts, jsx, importTypeArgs bool) (root *syntax.Node, err error, rejected bool) {
	p := &parser{src: src, ts: ts, jsx: jsx, arrowFail: map[int]bool{}, importTypeArgs: importTypeArgs}
	defer func() {
		rejected = p.rejectedImportTypeArgs
		if r := recover(); r != nil {
			pe, ok := r.(*parseError)
			if !ok {
				panic(r)
			}
			root, err = nil, pe
		}
	}()
	root = p.program()
	attachExtras(root, p.comments)
	removeASI(root)
	return syntax.Finish(root, src), nil, false
}

// ---- token helpers ----

func (p *parser) next() {
	p.prevEnd = p.tok.end
	p.tok = p.scan(p.tok.end)
}

// at reports whether the current token is the punctuator or word s.
func (p *parser) at(s string) bool {
	return (p.tok.kind == tPunct || p.tok.kind == tIdent) && p.tok.text == s
}

func (p *parser) atPunct(s string) bool { return p.tok.kind == tPunct && p.tok.text == s }

func (p *parser) atWord(s string) bool { return p.tok.kind == tIdent && p.tok.text == s }

// peek returns the token after the current one without consuming it.
func (p *parser) peek() token {
	n := len(p.comments)
	t := p.scan(p.tok.end)
	p.comments = p.comments[:n]
	return t
}

// peek2 returns the second token after the current one.
func (p *parser) peek2() token {
	n := len(p.comments)
	t := p.scan(p.tok.end)
	t = p.scan(t.end)
	p.comments = p.comments[:n]
	return t
}

// anon makes the current token an anonymous leaf and advances.
func (p *parser) anon() *syntax.Node {
	n := syntax.Leaf(p.tok.text, false, p.tok.start, p.tok.end)
	p.next()
	return n
}

// leaf makes the current token a named leaf of type typ and advances.
func (p *parser) leaf(typ string) *syntax.Node {
	n := syntax.Leaf(typ, true, p.tok.start, p.tok.end)
	p.next()
	return n
}

func (p *parser) expect(s string) *syntax.Node {
	if !p.at(s) {
		fail(p.tok.start, "expected %q, got %q", s, p.tok.text)
	}
	return p.anon()
}

// splitGT turns a leading `>` of `>>`, `>=`, `>>=`, `>>>`... into its own
// token (type argument lists).
func (p *parser) splitGT() {
	if p.tok.kind == tPunct && len(p.tok.text) > 1 && p.tok.text[0] == '>' {
		p.tok.end = p.tok.start + 1
		p.tok.text = ">"
	}
}

func (p *parser) expectGT() *syntax.Node {
	p.splitGT()
	return p.expect(">")
}

type state struct {
	tok       token
	prevEnd   int
	ncomments int
	noASI     bool
	noIn      bool
	asType    bool
	typeArgs  bool
}

func (p *parser) save() state {
	return state{p.tok, p.prevEnd, len(p.comments), p.noASI, p.noIn, p.asType, p.exprTypeArgs}
}

func (p *parser) restore(s state) {
	p.tok, p.prevEnd, p.noASI, p.noIn = s.tok, s.prevEnd, s.noASI, s.noIn
	p.asType, p.exprTypeArgs = s.asType, s.typeArgs
	p.comments = p.comments[:s.ncomments]
}

// try runs f speculatively: on a parse error the state is restored and nil
// returned.
func (p *parser) try(f func() *syntax.Node) (n *syntax.Node) {
	s := p.save()
	defer func() {
		if r := recover(); r != nil {
			var pe *parseError
			if err, ok := r.(error); !ok || !errors.As(err, &pe) {
				panic(r)
			}
			p.restore(s)
			n = nil
		}
	}()
	return f()
}

// ---- node helpers ----

// mk builds an inner node over the non-nil children.
func mk(typ string, children ...*syntax.Node) *syntax.Node {
	return syntax.Build(typ, children...)
}

// fld sets n's field name (nil-safe).
func fld(name string, n *syntax.Node) *syntax.Node { return syntax.SetField(name, n) }

// semicolon consumes `;` or accepts an automatic semicolon.
func (p *parser) semicolon() *syntax.Node {
	if p.atPunct(";") {
		return p.anon()
	}
	if p.canASI() {
		return p.asiMarker(p.prevEnd)
	}
	fail(p.tok.start, "expected ';', got %q", p.tok.text)
	return nil
}

func (p *parser) canASI() bool {
	return p.tok.kind == tEOF || p.atPunct("}") || p.tok.nl
}

// ---- program ----

func (p *parser) program() *syntax.Node {
	p.tok = p.scan(0)
	var kids []*syntax.Node
	if p.tok.kind == tHashbang {
		kids = append(kids, p.leaf("hash_bang_line"))
	}
	for p.tok.kind != tEOF {
		kids = append(kids, p.statement())
	}
	root := &syntax.Node{Type: "program", Named: true, Children: kids}
	start := len(p.src)
	if len(kids) > 0 {
		start = kids[0].StartByte
	}
	if len(p.comments) > 0 && p.comments[0].StartByte < start {
		start = p.comments[0].StartByte
	}
	if len(kids) == 0 && len(p.comments) == 0 {
		start = len(p.src)
	}
	root.StartByte, root.EndByte = start, len(p.src)
	return root
}
