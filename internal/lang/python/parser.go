// Package python is a hand-written parser that builds tree-sitter-python
// shaped trees (the grammar shipped in tree-sitter-language-pack 1.23.1).
// Parity with upstream tree-sitter is checked with tools/treediff.
//
// Layout: lexer.go (tokens + the external scanner's NEWLINE/INDENT/DEDENT
// rules), parser.go (statements), expr.go (expressions, strings, types),
// pattern.go (match/case patterns), extras.go (comment attachment).
package python

import (
	"github.com/matheusAle/coderefinery/internal/syntax"
)

// Parse parses src (already decoded and newline-normalized, see
// internal/source) and returns the root `module` node. On input the parser
// cannot handle it returns an error; it does not emulate tree-sitter's ERROR
// recovery.
func Parse(src []byte) (root *syntax.Node, err error) {
	text := string(src)
	toks, err := lex(text)
	if err != nil {
		return nil, err
	}
	p := &parser{src: text, hidden: map[*syntax.Node]bool{}}
	for _, t := range toks {
		if t.kind == tkComment || t.kind == tkLineCont {
			p.extras = append(p.extras, t)
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
	module := p.module()
	attachExtras(module, p.extras)
	removeHidden(module, p.hidden)
	if len(module.Children) > 0 {
		module.StartByte = module.Children[0].StartByte
	} else {
		module.StartByte = len(src)
	}
	module.EndByte = len(src)
	return syntax.Finish(module, src), nil
}

type parser struct {
	src    string
	toks   []token
	extras []token
	pos    int
	hidden map[*syntax.Node]bool
}

// ---- token helpers ----

func (p *parser) peek() token { return p.peekN(0) }

func (p *parser) peekN(n int) token {
	if p.pos+n < len(p.toks) {
		return p.toks[p.pos+n]
	}
	return p.toks[len(p.toks)-1] // EOF
}

func (p *parser) advance() token {
	t := p.peek()
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func (p *parser) atOp(s string) bool   { return p.peek().isOp(s) }
func (p *parser) atName(s string) bool { return p.peek().isName(s) }

func (p *parser) anon(t token) *syntax.Node {
	return syntax.Leaf(t.text, false, t.start, t.end)
}

func (p *parser) expectOp(s string) *syntax.Node {
	t := p.peek()
	if !t.isOp(s) {
		fail(t.start, "expected %q, found %q", s, t.text)
	}
	return p.anon(p.advance())
}

func (p *parser) expectKw(s string) *syntax.Node {
	t := p.peek()
	if !t.isName(s) {
		fail(t.start, "expected %q, found %q", s, t.text)
	}
	return p.anon(p.advance())
}

func (p *parser) hiddenTok(kind tokKind) *syntax.Node {
	t := p.peek()
	if t.kind != kind {
		fail(t.start, "expected layout token %d, found %q", kind, t.text)
	}
	p.advance()
	n := syntax.Leaf("_layout", false, t.start, t.end)
	p.hidden[n] = true
	return n
}

func node(typ string, children ...*syntax.Node) *syntax.Node {
	var kept []*syntax.Node
	for _, c := range children {
		if c != nil {
			kept = append(kept, c)
		}
	}
	return syntax.Inner(typ, 0, kept...)
}

func field(name string, n *syntax.Node) *syntax.Node {
	if n == nil {
		return nil
	}
	n.Field = name
	return n
}

// attempt runs f; on a parse error it restores the position and reports false.
func (p *parser) attempt(f func()) (ok bool) {
	saved := p.pos
	defer func() {
		if r := recover(); r != nil {
			if _, isParse := r.(*parseError); !isParse {
				panic(r)
			}
			p.pos = saved
			ok = false
		}
	}()
	f()
	return true
}

// ---- statements ----

func (p *parser) module() *syntax.Node {
	var children []*syntax.Node
	for p.peek().kind != tkEOF {
		children = append(children, p.statement()...)
	}
	m := &syntax.Node{Type: "module", Named: true, Children: children}
	m.FitChildren()
	return m
}

func (p *parser) statement() []*syntax.Node {
	t := p.peek()
	if t.kind == tkName {
		switch t.text {
		case "if":
			return []*syntax.Node{p.ifStatement()}
		case "for":
			return []*syntax.Node{p.forStatement(nil)}
		case "while":
			return []*syntax.Node{p.whileStatement()}
		case "try":
			return []*syntax.Node{p.tryStatement()}
		case "with":
			return []*syntax.Node{p.withStatement(nil)}
		case "def":
			return []*syntax.Node{p.functionDefinition(nil)}
		case "class":
			return []*syntax.Node{p.classDefinition()}
		case "async":
			next := p.peekN(1)
			if next.isName("def") || next.isName("for") || next.isName("with") {
				async := p.anon(p.advance())
				switch next.text {
				case "def":
					return []*syntax.Node{p.functionDefinition(async)}
				case "for":
					return []*syntax.Node{p.forStatement(async)}
				default:
					return []*syntax.Node{p.withStatement(async)}
				}
			}
		case "match":
			var n *syntax.Node
			if p.attempt(func() { n = p.matchStatement() }) {
				return []*syntax.Node{n}
			}
		}
	}
	if t.isOp("@") {
		return []*syntax.Node{p.decoratedDefinition()}
	}
	return p.simpleStatements()
}

func (p *parser) simpleStatements() []*syntax.Node {
	var out []*syntax.Node
	for {
		out = append(out, p.simpleStatement())
		if p.atOp(";") {
			out = append(out, p.anon(p.advance()))
			if p.peek().kind == tkNewline {
				break
			}
			continue
		}
		break
	}
	out = append(out, p.hiddenTok(tkNewline))
	return out
}

// suite returns the children a compound statement gets for its body: the
// block (tagged with fieldName) and, for an indented body, the hidden INDENT
// before it.
func (p *parser) suite(fieldName string) []*syntax.Node {
	switch p.peek().kind {
	case tkIndent:
		indent := p.hiddenTok(tkIndent)
		var children []*syntax.Node
		for p.peek().kind != tkDedent && p.peek().kind != tkEOF {
			children = append(children, p.statement()...)
		}
		children = append(children, p.hiddenTok(tkDedent))
		return []*syntax.Node{indent, field(fieldName, node("block", children...))}
	case tkNewline:
		return []*syntax.Node{field(fieldName, node("block", p.hiddenTok(tkNewline)))}
	}
	return []*syntax.Node{field(fieldName, node("block", p.simpleStatements()...))}
}

func compound(typ string, parts ...[]*syntax.Node) *syntax.Node {
	var all []*syntax.Node
	for _, part := range parts {
		all = append(all, part...)
	}
	return node(typ, all...)
}

func one(nodes ...*syntax.Node) []*syntax.Node { return nodes }

func (p *parser) ifStatement() *syntax.Node {
	head := one(p.expectKw("if"), field("condition", p.expression()), p.expectOp(":"))
	body := p.suite("consequence")
	var alts []*syntax.Node
	for p.atName("elif") {
		h := one(p.anon(p.advance()), field("condition", p.expression()), p.expectOp(":"))
		alts = append(alts, field("alternative", compound("elif_clause", h, p.suite("consequence"))))
	}
	if p.atName("else") {
		alts = append(alts, field("alternative", p.elseClause()))
	}
	return compound("if_statement", head, body, alts)
}

func (p *parser) elseClause() *syntax.Node {
	h := one(p.expectKw("else"), p.expectOp(":"))
	return compound("else_clause", h, p.suite("body"))
}

func (p *parser) forStatement(async *syntax.Node) *syntax.Node {
	head := one(async, p.expectKw("for"), field("left", p.targets()), p.expectKw("in"),
		field("right", p.expressions()), p.expectOp(":"))
	body := p.suite("body")
	var alt []*syntax.Node
	if p.atName("else") {
		alt = one(field("alternative", p.elseClause()))
	}
	return compound("for_statement", head, body, alt)
}

func (p *parser) whileStatement() *syntax.Node {
	head := one(p.expectKw("while"), field("condition", p.expression()), p.expectOp(":"))
	body := p.suite("body")
	var alt []*syntax.Node
	if p.atName("else") {
		alt = one(field("alternative", p.elseClause()))
	}
	return compound("while_statement", head, body, alt)
}

func (p *parser) tryStatement() *syntax.Node {
	head := one(p.expectKw("try"), p.expectOp(":"))
	body := p.suite("body")
	var rest []*syntax.Node
	for p.atName("except") {
		h := one(p.anon(p.advance()))
		if p.atOp("*") {
			h = append(h, p.anon(p.advance()))
		}
		if !p.atOp(":") {
			h = append(h, field("value", p.expressionAs()))
			if p.atOp(",") || p.atName("as") {
				h = append(h, p.anon(p.advance()), field("alias", p.expression()))
			}
		}
		h = append(h, p.expectOp(":"))
		rest = append(rest, compound("except_clause", h, p.suite("")))
	}
	if p.atName("else") {
		rest = append(rest, p.elseClause())
	}
	if p.atName("finally") {
		h := one(p.anon(p.advance()), p.expectOp(":"))
		rest = append(rest, compound("finally_clause", h, p.suite("")))
	}
	return compound("try_statement", head, body, rest)
}

func (p *parser) withStatement(async *syntax.Node) *syntax.Node {
	head := one(async, p.expectKw("with"), p.withClause(), p.expectOp(":"))
	return compound("with_statement", head, p.suite("body"))
}

func (p *parser) withClause() *syntax.Node {
	if p.atOp("(") {
		var clause *syntax.Node
		if p.attempt(func() {
			children := one(p.anon(p.advance()))
			multi := false
			for {
				item := node("with_item", field("value", p.expressionAs()))
				if item.Children[0].Type == "as_pattern" {
					multi = true
				}
				children = append(children, item)
				if !p.atOp(",") {
					break
				}
				multi = true
				children = append(children, p.anon(p.advance()))
				if p.atOp(")") {
					break
				}
			}
			children = append(children, p.expectOp(")"))
			if !multi || !p.atOp(":") {
				fail(p.peek().start, "not a parenthesized with clause")
			}
			clause = node("with_clause", children...)
		}) {
			return clause
		}
	}
	var children []*syntax.Node
	for {
		children = append(children, node("with_item", field("value", p.expressionAs())))
		if !p.atOp(",") {
			break
		}
		children = append(children, p.anon(p.advance()))
	}
	return node("with_clause", children...)
}

func (p *parser) functionDefinition(async *syntax.Node) *syntax.Node {
	head := one(async, p.expectKw("def"), field("name", p.identifier()))
	if p.atOp("[") {
		head = append(head, field("type_parameters", p.typeParameter()))
	}
	head = append(head, field("parameters", p.parameters()))
	if p.atOp("->") {
		head = append(head, p.anon(p.advance()), field("return_type", p.typeNode()))
	}
	head = append(head, p.expectOp(":"))
	return compound("function_definition", head, p.suite("body"))
}

func (p *parser) classDefinition() *syntax.Node {
	head := one(p.expectKw("class"), field("name", p.identifier()))
	if p.atOp("[") {
		head = append(head, field("type_parameters", p.typeParameter()))
	}
	if p.atOp("(") {
		head = append(head, field("superclasses", p.argumentList()))
	}
	head = append(head, p.expectOp(":"))
	return compound("class_definition", head, p.suite("body"))
}

func (p *parser) decoratedDefinition() *syntax.Node {
	var decorators []*syntax.Node
	for p.atOp("@") {
		at := p.anon(p.advance())
		decorators = append(decorators, node("decorator", at, p.expression(), p.hiddenTok(tkNewline)))
	}
	var def *syntax.Node
	switch {
	case p.atName("class"):
		def = p.classDefinition()
	case p.atName("async") && p.peekN(1).isName("def"):
		async := p.anon(p.advance())
		def = p.functionDefinition(async)
	default:
		def = p.functionDefinition(nil)
	}
	return node("decorated_definition", append(decorators, field("definition", def))...)
}

func (p *parser) matchStatement() *syntax.Node {
	head := one(p.expectKw("match"))
	for {
		head = append(head, field("subject", p.expression()))
		if !p.atOp(",") {
			break
		}
		head = append(head, p.anon(p.advance()))
		if p.atOp(":") {
			break
		}
	}
	head = append(head, p.expectOp(":"))
	children := one(p.hiddenTok(tkIndent))
	if !p.atName("case") {
		fail(p.peek().start, "expected case")
	}
	for p.atName("case") {
		children = append(children, field("alternative", p.caseClause()))
	}
	children = append(children, p.hiddenTok(tkDedent))
	return compound("match_statement", head, one(field("body", node("block", children...))))
}

func (p *parser) caseClause() *syntax.Node {
	head := one(p.expectKw("case"))
	for {
		head = append(head, p.casePattern())
		if !p.atOp(",") {
			break
		}
		head = append(head, p.anon(p.advance()))
		if p.atOp(":") || p.atName("if") {
			break
		}
	}
	if p.atName("if") {
		head = append(head, field("guard", node("if_clause", p.anon(p.advance()), p.expression())))
	}
	head = append(head, p.expectOp(":"))
	return compound("case_clause", head, p.suite("consequence"))
}

var augmentedOps = map[string]bool{
	"+=": true, "-=": true, "*=": true, "/=": true, "@=": true, "//=": true, "%=": true,
	"**=": true, ">>=": true, "<<=": true, "&=": true, "^=": true, "|=": true,
}

func (p *parser) simpleStatement() *syntax.Node {
	t := p.peek()
	if t.kind == tkName {
		switch t.text {
		case "import":
			return p.importStatement()
		case "from":
			return p.importFrom()
		case "assert":
			children := one(p.anon(p.advance()), p.expression())
			for p.atOp(",") {
				children = append(children, p.anon(p.advance()), p.expression())
			}
			return node("assert_statement", children...)
		case "return":
			children := one(p.anon(p.advance()))
			if p.startsExpression() {
				children = append(children, p.expressions())
			}
			return node("return_statement", children...)
		case "del":
			return node("delete_statement", p.anon(p.advance()), p.expressions())
		case "raise":
			children := one(p.anon(p.advance()))
			if p.startsExpression() {
				children = append(children, p.expressions())
			}
			if p.atName("from") {
				children = append(children, p.anon(p.advance()), field("cause", p.expression()))
			}
			return node("raise_statement", children...)
		case "pass", "break", "continue":
			return node(t.text+"_statement", p.anon(p.advance()))
		case "global", "nonlocal":
			children := one(p.anon(p.advance()), p.identifier())
			for p.atOp(",") {
				children = append(children, p.anon(p.advance()), p.identifier())
			}
			return node(t.text+"_statement", children...)
		case "print":
			if n := p.printStatement(); n != nil {
				return n
			}
		case "exec":
			next := p.peekN(1)
			if next.kind == tkStringStart || next.kind == tkName && !isHardKeyword(next.text) {
				return p.execStatement()
			}
		case "type":
			var n *syntax.Node
			if !p.peekN(1).isOp("=") && p.attempt(func() { n = p.typeAlias() }) {
				return n
			}
		}
	}
	return p.expressionStatement()
}

func (p *parser) printStatement() *syntax.Node {
	next := p.peekN(1)
	chevron := next.isOp(">>")
	plain := next.kind == tkStringStart || next.kind == tkNumber ||
		next.kind == tkName && !isHardKeyword(next.text) && !isOperatorWord(next.text)
	if !chevron && !plain {
		return nil
	}
	children := one(p.anon(p.advance()))
	if chevron {
		children = append(children, node("chevron", p.anon(p.advance()), p.expression()))
		for p.atOp(",") {
			children = append(children, p.anon(p.advance()))
			if !p.startsExpression() {
				break
			}
			children = append(children, field("argument", p.expression()))
		}
		return node("print_statement", children...)
	}
	for {
		children = append(children, field("argument", p.expression()))
		if !p.atOp(",") {
			break
		}
		children = append(children, p.anon(p.advance()))
		if !p.startsExpression() {
			break
		}
	}
	return node("print_statement", children...)
}

func (p *parser) execStatement() *syntax.Node {
	children := one(p.anon(p.advance()))
	if p.peek().kind == tkStringStart {
		children = append(children, field("code", p.stringOrConcat()))
	} else {
		children = append(children, field("code", p.identifier()))
	}
	if p.atName("in") {
		children = append(children, p.anon(p.advance()), p.expression())
		for p.atOp(",") {
			children = append(children, p.anon(p.advance()), p.expression())
		}
	}
	return node("exec_statement", children...)
}

func (p *parser) typeAlias() *syntax.Node {
	kw := p.expectKw("type")
	left := field("left", p.typeNode())
	eq := p.expectOp("=")
	return node("type_alias_statement", kw, left, eq, field("right", p.typeNode()))
}

func (p *parser) importStatement() *syntax.Node {
	children := one(p.anon(p.advance()))
	children = append(children, p.importList(false)...)
	return node("import_statement", children...)
}

func (p *parser) importList(parens bool) []*syntax.Node {
	var out []*syntax.Node
	for {
		name := p.dottedName()
		if p.atName("as") {
			name = node("aliased_import", field("name", name), p.anon(p.advance()), field("alias", p.identifier()))
		}
		out = append(out, field("name", name))
		if !p.atOp(",") {
			break
		}
		out = append(out, p.anon(p.advance()))
		if parens && p.atOp(")") || p.peek().kind == tkNewline {
			break
		}
	}
	return out
}

func (p *parser) importFrom() *syntax.Node {
	from := p.anon(p.advance())
	if p.atName("__future__") {
		children := one(from, p.anon(p.advance()), p.expectKw("import"))
		children = append(children, p.importTail()...)
		return node("future_import_statement", children...)
	}
	var module *syntax.Node
	if p.atOp(".") || p.atOp("...") {
		var dots []*syntax.Node
		for p.atOp(".") || p.atOp("...") {
			t := p.advance()
			for i := t.start; i < t.end; i++ {
				dots = append(dots, syntax.Leaf(".", false, i, i+1))
			}
		}
		rel := one(node("import_prefix", dots...))
		if !p.atName("import") {
			rel = append(rel, p.dottedName())
		}
		module = node("relative_import", rel...)
	} else {
		module = p.dottedName()
	}
	children := one(from, field("module_name", module), p.expectKw("import"))
	if p.atOp("*") {
		children = append(children, node("wildcard_import", p.anon(p.advance())))
	} else {
		children = append(children, p.importTail()...)
	}
	return node("import_from_statement", children...)
}

func (p *parser) importTail() []*syntax.Node {
	if p.atOp("(") {
		out := one(p.anon(p.advance()))
		out = append(out, p.importList(true)...)
		return append(out, p.expectOp(")"))
	}
	return p.importList(false)
}

func (p *parser) dottedName() *syntax.Node {
	children := one(p.identifier())
	for p.atOp(".") {
		children = append(children, p.anon(p.advance()), p.identifier())
	}
	return node("dotted_name", children...)
}

func (p *parser) expressionStatement() *syntax.Node {
	if p.atName("yield") {
		return p.yield()
	}
	first, list := p.starExpressionList("tuple_expression")
	return p.assignmentTail(first, list)
}

// assignmentTail continues an expression statement or right-hand side
// after its first expression (list) has been parsed.
func (p *parser) assignmentTail(first *syntax.Node, isList bool) *syntax.Node {
	t := p.peek()
	switch {
	case t.isOp("="):
		left := field("left", toPattern(first, true))
		eq := p.anon(p.advance())
		return node("assignment", left, eq, field("right", p.rightHandSide()))
	case t.isOp(":") && !isList:
		left := field("left", toPattern(first, true))
		colon := p.anon(p.advance())
		children := one(left, colon, field("type", p.typeNode()))
		if p.atOp("=") {
			children = append(children, p.anon(p.advance()), field("right", p.rightHandSide()))
		}
		return node("assignment", children...)
	case t.kind == tkOp && augmentedOps[t.text]:
		left := field("left", toPattern(first, true))
		op := field("operator", p.anon(p.advance()))
		return node("augmented_assignment", left, op, field("right", p.rightHandSide()))
	}
	return first
}

func (p *parser) rightHandSide() *syntax.Node {
	if p.atName("yield") {
		return p.yield()
	}
	first, list := p.starExpressionList("expression_list")
	return p.assignmentTail(first, list)
}
