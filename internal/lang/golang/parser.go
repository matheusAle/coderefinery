// Package golang is a hand-written Go parser that builds tree-sitter-go
// shaped trees (the grammar shipped in tree-sitter-language-pack 1.23.1).
// Tokens come from go/scanner; the recursive descent follows tree-sitter-go's
// grammar.js and reproduces its syntax-only choices (see docs/PARSERS.md).
// Parity with upstream tree-sitter is checked with tools/treediff.
//
// Layout: lexer.go (go/scanner tokens), parser.go (files, declarations,
// statements), expr.go (expressions, literals), types.go (types, parameter
// and field lists, type conversions of expressions), extras.go (comments).
package golang

import (
	"fmt"
	"go/token"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

// Parse parses src (already decoded and newline-normalized, see
// internal/source) and returns the root `source_file` node. On input the
// parser cannot handle it returns an error; it does not emulate tree-sitter's
// ERROR recovery.
func Parse(src []byte) (root *syntax.Node, err error) {
	toks, comments, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{src: src, toks: toks, hidden: map[*syntax.Node]bool{}, insts: map[*syntax.Node]*instantiation{}}
	defer func() {
		if r := recover(); r != nil {
			pe, ok := r.(*parseError)
			if !ok {
				panic(r)
			}
			root, err = nil, pe
		}
	}()
	file := p.sourceFile()
	attachExtras(file, comments)
	syntax.RemoveHidden(file, p.hidden)
	if len(file.Children) > 0 {
		file.StartByte = file.Children[0].StartByte
	} else {
		file.StartByte = len(src)
	}
	file.EndByte = len(src)
	return syntax.Finish(file, src), nil
}

type parseError struct {
	offset int
	msg    string
}

func (e *parseError) Error() string {
	return fmt.Sprintf("go parse error at byte %d: %s", e.offset, e.msg)
}

type parser struct {
	src     []byte
	toks    []tok
	pos     int
	prevEnd int // end of the last consumed real token
	exprLev int // < 0 inside control clause headers (no T{} literals)
	hidden  map[*syntax.Node]bool
	insts   map[*syntax.Node]*instantiation
}

func (p *parser) cur() tok { return p.toks[p.pos] }

func (p *parser) peek(n int) tok {
	if p.pos+n < len(p.toks) {
		return p.toks[p.pos+n]
	}
	return p.toks[len(p.toks)-1]
}

func (p *parser) is(t token.Token) bool { return p.toks[p.pos].t == t }

func (p *parser) fail(format string, args ...any) {
	panic(&parseError{offset: p.cur().start, msg: fmt.Sprintf(format, args...)})
}

// next consumes the current token.
func (p *parser) next() tok {
	t := p.toks[p.pos]
	if t.t == token.EOF {
		p.fail("unexpected end of file")
	}
	p.pos++
	if !t.auto() {
		p.prevEnd = t.end
	}
	return t
}

// anon consumes the current token as an anonymous leaf named by its text.
func (p *parser) anon() *syntax.Node {
	t := p.next()
	return syntax.Leaf(t.lit, false, t.start, t.end)
}

// named consumes the current token as a named leaf of type typ.
func (p *parser) named(typ string) *syntax.Node {
	t := p.next()
	return syntax.Leaf(typ, true, t.start, t.end)
}

func (p *parser) expect(t token.Token) *syntax.Node {
	if !p.is(t) {
		p.fail("expected %s, found %q", t, p.cur().lit)
	}
	return p.anon()
}

func (p *parser) ident(typ string) *syntax.Node {
	if !p.is(token.IDENT) {
		p.fail("expected identifier, found %q", p.cur().lit)
	}
	return p.named(typ)
}

// node builds an inner node over the non-nil children.
func node(typ string, children ...*syntax.Node) *syntax.Node {
	return syntax.Build(typ, children...)
}

// field sets n's field name (nil-safe).
func field(name string, n *syntax.Node) *syntax.Node { return syntax.SetField(name, n) }

// newlineAfter returns the offset of the first newline after off that is not
// inside a comment, or -1 when a token (or EOF) comes first.
func (p *parser) newlineAfter(off int) int {
	src := p.src
	for i := off; i < len(src); i++ {
		switch c := src[i]; {
		case c == '\n':
			return i
		case c == ' ' || c == '\t' || c == '\r' || c == '\f':
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
			if i < len(src) {
				return i
			}
			return -1
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			j := i + 2
			for j+1 < len(src) && !(src[j] == '*' && src[j+1] == '/') {
				j++
			}
			i = j + 1
		default:
			return -1
		}
	}
	return -1
}

// terminator consumes a statement terminator if there is one: an explicit
// ";" (visible), an automatic semicolon (tree-sitter's hidden "\n" token on
// the newline character) or, at EOF, tree-sitter's "\0" terminator, which
// shows as a zero-width "" node (only after top-level statements: eofNode).
// afterSemi handles an empty statement, after which go/scanner inserts no
// semicolon although tree-sitter still lexes the newline as a terminator.
func (p *parser) terminator(eofNode, afterSemi bool) *syntax.Node {
	t := p.cur()
	if t.t == token.SEMICOLON && !afterSemi {
		if !t.auto() {
			return p.anon()
		}
		p.next()
		return p.newlineNode(eofNode)
	}
	if afterSemi {
		if nl := p.newlineAfter(p.prevEnd); nl >= 0 && nl < t.start {
			if t.auto() {
				p.next()
			}
			return p.newlineRun(nl)
		}
		if t.t == token.SEMICOLON && !t.auto() {
			return p.anon()
		}
		if t.t == token.EOF && p.newlineAfter(p.prevEnd) < 0 && eofNode {
			return syntax.Leaf("", false, len(p.src), len(p.src))
		}
	}
	return nil
}

func (p *parser) newlineNode(eofNode bool) *syntax.Node {
	nl := p.newlineAfter(p.prevEnd)
	if nl >= 0 {
		return p.newlineRun(nl)
	}
	if p.is(token.EOF) {
		if eofNode {
			return syntax.Leaf("", false, len(p.src), len(p.src))
		}
		return nil
	}
	p.fail("unsupported newline inside a block comment")
	return nil
}

// newlineRun is the hidden terminator token: a run of consecutive newlines.
func (p *parser) newlineRun(nl int) *syntax.Node {
	end := nl
	for end < len(p.src) && p.src[end] == '\n' {
		end++
	}
	return p.hide(syntax.Leaf("\n", false, nl, end))
}

func (p *parser) hide(n *syntax.Node) *syntax.Node {
	p.hidden[n] = true
	return n
}

// ---- source file and top-level declarations ----

func (p *parser) sourceFile() *syntax.Node {
	var kids []*syntax.Node
	for !p.is(token.EOF) {
		var n *syntax.Node
		topDecl := false
		switch p.cur().t {
		case token.PACKAGE:
			n = node("package_clause", p.anon(), p.ident("package_identifier"))
			topDecl = true
		case token.IMPORT:
			n = p.importDecl()
			topDecl = true
		case token.FUNC:
			if d := p.funcDecl(); d != nil {
				n, topDecl = d, true
			} else {
				n = p.statement()
			}
		case token.SEMICOLON:
			n = node("empty_statement", p.anon())
		default:
			n = p.statement()
		}
		kids = append(kids, n)
		t := p.terminator(!topDecl && endsWithValue(n), endsWithSemi(n))
		if t == nil {
			if p.is(token.EOF) {
				break
			}
			p.fail("expected terminator, found %q", p.cur().lit)
		}
		kids = append(kids, t)
	}
	return syntax.Inner("source_file", 0, kids...)
}

// endsWithSemi reports whether n ends with an explicit ";" (an empty
// statement), after which go/scanner inserts no automatic semicolon.
func endsWithSemi(n *syntax.Node) bool {
	for len(n.Children) > 0 {
		n = n.Children[len(n.Children)-1]
	}
	return n.Type == ";"
}

// endsWithValue reports whether top-level statement n is an ungrouped
// var/const spec with a value: only then does tree-sitter-go lex its "\0"
// terminator at EOF (a zero-width "" node).
func endsWithValue(n *syntax.Node) bool {
	if n.Type != "var_declaration" && n.Type != "const_declaration" {
		return false
	}
	spec := n.Children[len(n.Children)-1]
	if spec.Type != "var_spec" && spec.Type != "const_spec" {
		return false
	}
	return spec.Children[len(spec.Children)-1].Field == "value"
}

func (p *parser) importDecl() *syntax.Node {
	kw := p.anon()
	if !p.is(token.LPAREN) {
		return node("import_declaration", kw, p.importSpec())
	}
	kids := []*syntax.Node{p.anon()}
	for !p.is(token.RPAREN) {
		kids = append(kids, p.importSpec())
		if p.is(token.RPAREN) {
			break
		}
		t := p.terminator(false, false)
		if t == nil {
			p.fail("expected terminator in import list")
		}
		kids = append(kids, t)
	}
	kids = append(kids, p.anon())
	return node("import_declaration", kw, node("import_spec_list", kids...))
}

func (p *parser) importSpec() *syntax.Node {
	var name *syntax.Node
	switch {
	case p.is(token.PERIOD):
		name = node("dot", p.anon())
	case p.is(token.IDENT) && p.cur().lit == "_":
		name = p.named("blank_identifier")
	case p.is(token.IDENT):
		name = p.named("package_identifier")
	}
	if !p.is(token.STRING) {
		p.fail("expected import path")
	}
	return node("import_spec", field("name", name), field("path", p.stringLit()))
}

// funcDecl parses a function or method declaration; it returns nil (and
// consumes nothing) for a func literal or type at statement level.
func (p *parser) funcDecl() *syntax.Node {
	nt := p.peek(1).t
	if nt != token.IDENT && nt != token.LPAREN {
		return nil
	}
	if nt == token.LPAREN {
		// func (recv) Name(...) vs func(...) {...}() at top level
		j := p.matching(p.pos + 1)
		if j < 0 || p.toks[j+1].t != token.IDENT || p.toks[j+2].t != token.LPAREN {
			return nil
		}
	}
	kw := p.anon()
	if p.is(token.LPAREN) {
		recv := field("receiver", p.parameterList())
		name := field("name", p.ident("field_identifier"))
		params := field("parameters", p.parameterList())
		result := field("result", p.result())
		var body *syntax.Node
		if p.is(token.LBRACE) {
			body = field("body", p.block())
		}
		return node("method_declaration", kw, recv, name, params, result, body)
	}
	name := field("name", p.ident("identifier"))
	var tparams *syntax.Node
	if p.is(token.LBRACK) {
		tparams = field("type_parameters", p.typeParameterList())
	}
	params := field("parameters", p.parameterList())
	result := field("result", p.result())
	var body *syntax.Node
	if p.is(token.LBRACE) {
		body = field("body", p.block())
	}
	return node("function_declaration", kw, name, tparams, params, result, body)
}

// matching returns the index of the token closing the bracket at i, or -1.
func (p *parser) matching(i int) int {
	depth := 0
	for j := i; j < len(p.toks); j++ {
		switch p.toks[j].t {
		case token.LPAREN, token.LBRACK, token.LBRACE:
			depth++
		case token.RPAREN, token.RBRACK, token.RBRACE:
			depth--
			if depth == 0 {
				return j
			}
		case token.EOF:
			return -1
		}
	}
	return -1
}

// ---- declarations usable as statements ----

func (p *parser) varDecl() *syntax.Node {
	kw := p.anon()
	if !p.is(token.LPAREN) {
		return node("var_declaration", kw, p.varSpec())
	}
	kids := []*syntax.Node{p.anon()}
	for !p.is(token.RPAREN) {
		kids = append(kids, p.varSpec())
		if t := p.terminator(false, false); t != nil {
			kids = append(kids, t)
		} else if !p.is(token.RPAREN) {
			p.fail("expected terminator in var list")
		}
	}
	kids = append(kids, p.anon())
	return node("var_declaration", kw, node("var_spec_list", kids...))
}

func (p *parser) varSpec() *syntax.Node {
	kids := []*syntax.Node{field("name", p.ident("identifier"))}
	for p.is(token.COMMA) {
		kids = append(kids, p.anon(), field("name", p.ident("identifier")))
	}
	if !p.is(token.ASSIGN) {
		kids = append(kids, field("type", p.parseType()))
	}
	if p.is(token.ASSIGN) {
		kids = append(kids, p.anon(), field("value", p.expressionList()))
	}
	return node("var_spec", kids...)
}

func (p *parser) constDecl() *syntax.Node {
	kw := p.anon()
	if !p.is(token.LPAREN) {
		return node("const_declaration", kw, p.constSpec())
	}
	kids := []*syntax.Node{kw, p.anon()}
	for !p.is(token.RPAREN) {
		kids = append(kids, p.constSpec())
		if t := p.terminator(false, false); t != nil {
			kids = append(kids, t)
		} else if !p.is(token.RPAREN) {
			p.fail("expected terminator in const list")
		}
	}
	kids = append(kids, p.anon())
	return node("const_declaration", kids...)
}

func (p *parser) constSpec() *syntax.Node {
	kids := []*syntax.Node{field("name", p.ident("identifier"))}
	for p.is(token.COMMA) {
		kids = append(kids, field("name", p.anon()), field("name", p.ident("identifier")))
	}
	if !p.is(token.ASSIGN) && !p.is(token.SEMICOLON) && !p.is(token.RPAREN) {
		kids = append(kids, field("type", p.parseType()))
	}
	if p.is(token.ASSIGN) {
		kids = append(kids, p.anon(), field("value", p.expressionList()))
	}
	return node("const_spec", kids...)
}

func (p *parser) typeDecl() *syntax.Node {
	kw := p.anon()
	if !p.is(token.LPAREN) {
		return node("type_declaration", kw, p.typeSpec())
	}
	kids := []*syntax.Node{kw, p.anon()}
	for !p.is(token.RPAREN) {
		kids = append(kids, p.typeSpec())
		if t := p.terminator(false, false); t != nil {
			kids = append(kids, t)
		} else if !p.is(token.RPAREN) {
			p.fail("expected terminator in type list")
		}
	}
	kids = append(kids, p.anon())
	return node("type_declaration", kids...)
}

func (p *parser) typeSpec() *syntax.Node {
	name := field("name", p.ident("type_identifier"))
	var tparams *syntax.Node
	if p.is(token.LBRACK) && p.looksLikeTypeParams() {
		tparams = field("type_parameters", p.typeParameterList())
	}
	if p.is(token.ASSIGN) {
		eq := p.anon()
		return node("type_alias", name, tparams, eq, field("type", p.parseType()))
	}
	return node("type_spec", name, tparams, field("type", p.parseType()))
}

// looksLikeTypeParams decides `type T[P C] ...` (type parameters) against
// `type T [N]E` (array type) at the "[".
func (p *parser) looksLikeTypeParams() bool {
	if p.peek(1).t != token.IDENT {
		return false
	}
	// an array length never contains a top-level comma
	if j := p.matching(p.pos); j > 0 {
		depth := 0
		for k := p.pos + 1; k < j; k++ {
			switch p.toks[k].t {
			case token.LPAREN, token.LBRACK, token.LBRACE:
				depth++
			case token.RPAREN, token.RBRACK, token.RBRACE:
				depth--
			case token.COMMA:
				if depth == 0 {
					return true
				}
			}
		}
	}
	switch p.peek(2).t {
	case token.IDENT, token.COMMA, token.TILDE, token.INTERFACE, token.FUNC, token.CHAN, token.MAP, token.STRUCT:
		return true
	case token.LBRACK:
		return p.peek(3).t == token.RBRACK
	case token.MUL:
		// [P *C] is a type parameter when the constraint is followed by "]" or ",".
		return p.peek(3).t == token.IDENT && (p.peek(4).t == token.COMMA || (p.peek(4).t == token.RBRACK && isTypeStart(p.peek(5).t)))
	}
	return false
}

// ---- statements ----

func (p *parser) block() *syntax.Node {
	lb := p.expect(token.LBRACE)
	lev := p.exprLev
	p.exprLev = 0
	list := p.statementList()
	p.exprLev = lev
	rb := p.expect(token.RBRACE)
	return node("block", lb, list, rb)
}

func isListEnd(t token.Token) bool {
	return t == token.RBRACE || t == token.CASE || t == token.DEFAULT || t == token.EOF
}

func (p *parser) statementList() *syntax.Node {
	var kids []*syntax.Node
	for !isListEnd(p.cur().t) {
		var st *syntax.Node
		if p.is(token.SEMICOLON) {
			st = node("empty_statement", p.anon())
		} else {
			st = p.statement()
		}
		kids = append(kids, st)
		if st.Type == "labeled_statement" && len(st.Children) == 2 {
			break // empty labeled statement: only at the end of a list
		}
		t := p.terminator(false, endsWithSemi(st))
		if t == nil {
			if !isListEnd(p.cur().t) {
				p.fail("expected terminator, found %q", p.cur().lit)
			}
			break
		}
		kids = append(kids, t)
	}
	if len(kids) == 0 {
		return nil
	}
	return syntax.Inner("statement_list", 0, kids...)
}

func (p *parser) statement() *syntax.Node {
	switch p.cur().t {
	case token.VAR:
		return p.varDecl()
	case token.CONST:
		return p.constDecl()
	case token.TYPE:
		return p.typeDecl()
	case token.RETURN:
		kw := p.anon()
		if p.is(token.SEMICOLON) || isListEnd(p.cur().t) || p.is(token.RPAREN) {
			return node("return_statement", kw)
		}
		return node("return_statement", kw, p.expressionList())
	case token.GO:
		return node("go_statement", p.anon(), p.expr())
	case token.DEFER:
		return node("defer_statement", p.anon(), p.expr())
	case token.IF:
		return p.ifStatement()
	case token.FOR:
		return p.forStatement()
	case token.SWITCH:
		return p.switchStatement()
	case token.SELECT:
		return p.selectStatement()
	case token.FALLTHROUGH:
		return node("fallthrough_statement", p.anon())
	case token.BREAK, token.CONTINUE:
		typ := "break_statement"
		if p.is(token.CONTINUE) {
			typ = "continue_statement"
		}
		kw := p.anon()
		if p.is(token.IDENT) {
			return node(typ, kw, p.named("label_name"))
		}
		return node(typ, kw)
	case token.GOTO:
		return node("goto_statement", p.anon(), p.ident("label_name"))
	case token.LBRACE:
		return p.block()
	case token.SEMICOLON:
		return node("empty_statement", p.anon())
	case token.IDENT:
		if p.peek(1).t == token.COLON {
			label := field("label", p.named("label_name"))
			colon := p.anon()
			if isListEnd(p.cur().t) {
				return node("labeled_statement", label, colon)
			}
			return node("labeled_statement", label, colon, p.statement())
		}
	}
	return p.simpleStatement(0)
}

const (
	allowRange = 1 << iota
	allowGuard
)

// simpleStatement parses tree-sitter's _simple_statement. With allowRange
// it may return a range_clause; with allowGuard the expression may be a
// type switch guard `x.(type)` (a typeGuard node, flattened by the caller).
func (p *parser) simpleStatement(mode int) *syntax.Node {
	if mode&allowRange != 0 && p.is(token.RANGE) {
		kw := p.anon()
		return node("range_clause", kw, field("right", p.expr()))
	}
	lhs := p.exprListItems(mode&allowGuard != 0)
	switch t := p.cur().t; t {
	case token.DEFINE, token.ASSIGN:
		op := p.anon()
		left := field("left", node("expression_list", lhs...))
		if mode&allowRange != 0 && p.is(token.RANGE) {
			kw := p.anon()
			return node("range_clause", left, op, kw, field("right", p.expr()))
		}
		rhs := p.exprListItems(mode&allowGuard != 0)
		if t == token.DEFINE {
			return node("short_var_declaration", left, op, field("right", node("expression_list", rhs...)))
		}
		return node("assignment_statement", left, field("operator", op), field("right", node("expression_list", rhs...)))
	case token.ADD_ASSIGN, token.SUB_ASSIGN, token.MUL_ASSIGN, token.QUO_ASSIGN, token.REM_ASSIGN,
		token.AND_ASSIGN, token.OR_ASSIGN, token.XOR_ASSIGN, token.SHL_ASSIGN, token.SHR_ASSIGN, token.AND_NOT_ASSIGN:
		op := p.anon()
		left := field("left", node("expression_list", lhs...))
		rhs := p.exprListItems(false)
		return node("assignment_statement", left, field("operator", op), field("right", node("expression_list", rhs...)))
	}
	if len(lhs) != 1 {
		p.fail("expected assignment after expression list")
	}
	x := lhs[0]
	switch p.cur().t {
	case token.ARROW:
		arrow := p.anon()
		return node("send_statement", field("channel", x), arrow, field("value", p.expr()))
	case token.INC:
		return node("inc_statement", x, p.anon())
	case token.DEC:
		return node("dec_statement", x, p.anon())
	}
	if x.Type == typeGuard {
		return x
	}
	return node("expression_statement", x)
}

func (p *parser) ifStatement() *syntax.Node {
	kw := p.anon()
	lev := p.exprLev
	p.exprLev = -1
	var init, semi *syntax.Node
	if p.is(token.SEMICOLON) {
		semi = p.anon()
	} else {
		s := p.simpleStatement(0)
		if p.is(token.SEMICOLON) {
			init, semi = s, p.anon()
		} else {
			init = s
		}
	}
	var cond *syntax.Node
	if semi == nil {
		cond = unwrapExprStmt(init)
		init = nil
	} else {
		cond = p.expr()
	}
	p.exprLev = lev
	kids := []*syntax.Node{kw, field("initializer", init), semi, field("condition", cond), field("consequence", p.block())}
	if p.is(token.ELSE) {
		kids = append(kids, p.anon())
		if p.is(token.IF) {
			kids = append(kids, field("alternative", p.ifStatement()))
		} else {
			kids = append(kids, field("alternative", p.block()))
		}
	}
	return node("if_statement", kids...)
}

func unwrapExprStmt(s *syntax.Node) *syntax.Node {
	if s.Type != "expression_statement" {
		panic(&parseError{offset: s.StartByte, msg: "expected expression"})
	}
	x := s.Children[0]
	x.Field = ""
	return x
}

func (p *parser) forStatement() *syntax.Node {
	kw := p.anon()
	if p.is(token.LBRACE) {
		return node("for_statement", kw, field("body", p.block()))
	}
	lev := p.exprLev
	p.exprLev = -1
	var clause *syntax.Node
	var init *syntax.Node
	if !p.is(token.SEMICOLON) {
		init = p.simpleStatement(allowRange)
	}
	switch {
	case init != nil && init.Type == "range_clause":
		clause = init
	case p.is(token.SEMICOLON):
		semi1 := p.anon()
		var cond *syntax.Node
		if !p.is(token.SEMICOLON) {
			cond = p.expr()
		}
		semi2 := p.expect(token.SEMICOLON)
		var post *syntax.Node
		if !p.is(token.LBRACE) {
			post = p.simpleStatement(0)
		}
		clause = node("for_clause", field("initializer", init), semi1, field("condition", cond), semi2, field("update", post))
	default:
		clause = unwrapExprStmt(init)
	}
	p.exprLev = lev
	return node("for_statement", kw, clause, field("body", p.block()))
}

func (p *parser) switchStatement() *syntax.Node {
	kw := p.anon()
	lev := p.exprLev
	p.exprLev = -1
	var init, semi, value *syntax.Node
	if !p.is(token.LBRACE) {
		var s1 *syntax.Node
		if !p.is(token.SEMICOLON) {
			s1 = p.simpleStatement(allowGuard)
		}
		if p.is(token.SEMICOLON) {
			init, semi = s1, p.anon()
			if !p.is(token.LBRACE) {
				value = p.simpleStatement(allowGuard)
			}
		} else {
			value = s1
		}
	}
	p.exprLev = lev
	kids := []*syntax.Node{kw, field("initializer", init), semi}
	typeSwitch := false
	if value != nil {
		switch {
		case value.Type == typeGuard:
			typeSwitch = true
			kids = append(kids, guardChildren(value)...)
		case value.Type == "short_var_declaration" && len(value.Children[2].Children) == 1 && value.Children[2].Children[0].Type == typeGuard:
			typeSwitch = true
			left := value.Children[0]
			left.Field = "alias"
			kids = append(kids, left, value.Children[1])
			kids = append(kids, guardChildren(value.Children[2].Children[0])...)
		default:
			kids = append(kids, field("value", unwrapExprStmt(value)))
		}
	}
	kids = append(kids, p.expect(token.LBRACE))
	for !p.is(token.RBRACE) {
		kids = append(kids, p.caseClause(typeSwitch))
	}
	kids = append(kids, p.anon())
	if typeSwitch {
		return node("type_switch_statement", kids...)
	}
	return node("expression_switch_statement", kids...)
}

// guardChildren flattens `x.(type)` into value: x "." "(" "type" ")".
func guardChildren(g *syntax.Node) []*syntax.Node {
	g.Children[0].Field = "value"
	return g.Children
}

func (p *parser) caseClause(typeSwitch bool) *syntax.Node {
	if p.is(token.DEFAULT) {
		kw := p.anon()
		colon := p.expect(token.COLON)
		return node("default_case", kw, colon, p.statementList())
	}
	kw := p.expect(token.CASE)
	if typeSwitch {
		kids := []*syntax.Node{kw, field("type", p.parseType())}
		for p.is(token.COMMA) {
			kids = append(kids, field("type", p.anon()), field("type", p.parseType()))
		}
		kids = append(kids, p.expect(token.COLON), p.statementList())
		return node("type_case", kids...)
	}
	value := field("value", p.expressionList())
	colon := p.expect(token.COLON)
	return node("expression_case", kw, value, colon, p.statementList())
}

func (p *parser) selectStatement() *syntax.Node {
	kids := []*syntax.Node{p.anon(), p.expect(token.LBRACE)}
	for !p.is(token.RBRACE) {
		if p.is(token.DEFAULT) {
			kw := p.anon()
			colon := p.expect(token.COLON)
			kids = append(kids, node("default_case", kw, colon, p.statementList()))
			continue
		}
		kw := p.expect(token.CASE)
		comm := p.simpleStatement(0)
		switch comm.Type {
		case "send_statement":
		case "short_var_declaration", "assignment_statement":
			rhs := comm.Children[2]
			if len(rhs.Children) != 1 {
				p.fail("bad receive statement")
			}
			comm.Children[1].Field = ""
			comm = node("receive_statement", comm.Children[0], comm.Children[1], field("right", rhs.Children[0]))
		case "expression_statement":
			comm = node("receive_statement", field("right", comm.Children[0]))
		default:
			p.fail("bad communication clause")
		}
		colon := p.expect(token.COLON)
		kids = append(kids, node("communication_case", kw, field("communication", comm), colon, p.statementList()))
	}
	kids = append(kids, p.anon())
	return node("select_statement", kids...)
}
