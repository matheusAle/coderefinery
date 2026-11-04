package rust

import "github.com/matheusAle/coderefinery/internal/syntax"

// Expressions: precedence climbing over tree-sitter-rust's PREC table.

const (
	precRange  = 1
	precOr     = 2
	precAnd    = 3
	precComp   = 4
	precBitOr  = 5
	precBitXor = 6
	precBitAnd = 7
	precShift  = 8
	precAdd    = 9
	precMul    = 10
	precCast   = 11
	precUnary  = 12
	precTry    = 13
)

var binaryPrec = map[string]int{
	"||": precOr, "&&": precAnd,
	"==": precComp, "!=": precComp, "<": precComp, "<=": precComp, ">": precComp, ">=": precComp,
	"|": precBitOr, "^": precBitXor, "&": precBitAnd, "<<": precShift, ">>": precShift,
	"+": precAdd, "-": precAdd, "*": precMul, "/": precMul, "%": precMul,
}

var compoundOps = map[string]bool{
	"+=": true, "-=": true, "*=": true, "/=": true, "%=": true, "^=": true, "&=": true, "|=": true, "<<=": true, ">>=": true,
}

type exprCtx struct {
	noStruct bool // condition position: `x {` opens the body, not a struct literal
	stopLet  bool // condition position: `&& let` belongs to a let_chain
	stopAnd  bool // let-chain element: a top-level `&&` ends it (not inherited)
	closure  bool // closure body: the closure itself may be called, so `x..(y)` is (|| x..)(y)
}

func (c exprCtx) inner() exprCtx {
	return exprCtx{noStruct: c.noStruct, stopLet: c.stopLet, closure: c.closure}
}

func (p *parser) expression(ctx exprCtx) *syntax.Node { return p.exprPrec(0, ctx) }

func (p *parser) exprPrec(minPrec int, ctx exprCtx) *syntax.Node {
	left := p.prefixExpr(ctx)
	return p.continueExpression(left, minPrec, ctx)
}

// continueExpression applies postfix and binary operators whose precedence
// is at least minPrec to left.
func (p *parser) continueExpression(left *syntax.Node, minPrec int, ctx exprCtx) *syntax.Node {
	for {
		t := p.peek()
		if t.kind == tkIdent {
			if t.text == "as" && precCast >= minPrec {
				as := p.expect("as")
				left = node("type_cast_expression", field("value", left), as, field("type", p.typeNoBounds()))
				continue
			}
			return left
		}
		if t.kind != tkPunct {
			return left
		}
		switch t.text {
		case "?":
			if precTry < minPrec {
				return left
			}
			left = node("try_expression", left, p.expect("?"))
			continue
		case ".":
			left = p.dotExpression(left)
			continue
		case "(":
			if left.Type == "range_expression" {
				return left // the callee is an _expression_except_range
			}
			left = node("call_expression", field("function", left), field("arguments", p.arguments()))
			continue
		case "[":
			open := p.expect("[")
			idx := p.expression(exprCtx{})
			left = node("index_expression", left, open, idx, p.expect("]"))
			continue
		case "=":
			if minPrec > 0 {
				return left
			}
			eq := p.expect("=")
			left = node("assignment_expression", field("left", left), eq, field("right", p.exprPrec(1, ctx.inner())))
			continue
		case "..", "...", "..=":
			if precRange < minPrec {
				return left
			}
			op := p.expect(t.text)
			if p.canStartRangeEnd(ctx, t.text) {
				left = node("range_expression", left, op, p.exprPrec(precRange+1, ctx.inner()))
			} else {
				left = node("range_expression", left, op)
			}
			continue
		}
		if compoundOps[t.text] {
			if minPrec > 0 {
				return left
			}
			op := p.expect(t.text)
			left = node("compound_assignment_expr", field("left", left), field("operator", op), field("right", p.exprPrec(1, ctx.inner())))
			continue
		}
		prec, ok := binaryPrec[t.text]
		if !ok || prec < minPrec {
			return left
		}
		if t.text == "&&" && (ctx.stopAnd || ctx.stopLet && p.peek2().is("let")) {
			return left
		}
		op := p.expect(t.text)
		right := p.exprPrec(prec+1, ctx.inner())
		left = node("binary_expression", field("left", left), field("operator", op), field("right", right))
	}
}

func (p *parser) dotExpression(left *syntax.Node) *syntax.Node {
	dot := p.expect(".")
	t := p.peek()
	switch {
	case t.is("await"):
		return node("await_expression", left, dot, p.expect("await"))
	case t.kind == tkInt || t.kind == tkFloat:
		it := p.lx.lex(t.start, false)
		p.pos = it.end
		return node("field_expression", field("value", left), dot, field("field", syntax.Leaf("integer_literal", true, it.start, it.end)))
	}
	fe := node("field_expression", field("value", left), dot, field("field", p.identAs("field_identifier")))
	if p.atTurbofish() {
		sep := p.expect("::")
		return node("generic_function", field("function", fe), sep, field("type_arguments", p.typeArguments()))
	}
	return fe
}

// canStartRangeEnd: whether the range operator op is followed by its end
// operand. After `..` (which may also end a range) a token that is both a
// prefix and an infix operator continues the expression instead:
// `a..-b` is (a..) - b, `a..[i]` indexes the range.
func (p *parser) canStartRangeEnd(ctx exprCtx, op string) bool {
	t := p.peek()
	if ctx.noStruct && t.is("{") {
		return false
	}
	if op == ".." && (!t.is("(") || ctx.closure) && p.atInfixAmbiguous() {
		return false
	}
	return p.canStartExpression()
}

// atInfixAmbiguous: the next token both starts an operand and continues an
// expression (a binary, index or call operator).
func (p *parser) atInfixAmbiguous() bool {
	t := p.peek()
	if t.kind != tkPunct {
		return false
	}
	switch t.text {
	case "-", "*", "&", "&&", "|", "||", "[", "(", "<", "<<", "..":
		return true
	}
	return false
}

func (p *parser) canStartExpression() bool {
	t := p.peek()
	switch t.kind {
	case tkEOF:
		return false
	case tkInt, tkFloat, tkChar, tkString, tkRawString:
		return true
	case tkIdent:
		switch t.text {
		case "as", "else", "in", "where":
			return false
		}
		return true
	}
	switch t.text {
	case "(", "[", "{", "-", "*", "!", "&", "&&", "|", "||", "..", "..=", "<", "<<", "::", "'", "#", "$":
		return true
	}
	return false
}

func (p *parser) arguments() *syntax.Node {
	parts := []*syntax.Node{p.expect("(")}
	for !p.at(")") {
		parts = append(parts, p.attributes()...)
		parts = append(parts, p.expression(exprCtx{}))
		if !p.at(")") {
			parts = append(parts, p.expect(","))
		}
	}
	parts = append(parts, p.expect(")"))
	return node("arguments", parts...)
}

// startsBlockLike reports whether the next tokens start an
// _expression_ending_with_block.
func (p *parser) startsBlockLike() bool {
	t := p.peek()
	switch {
	case t.is("{"), t.is("if"), t.is("match"), t.is("while"), t.is("loop"), t.is("for"):
		return true
	case t.is("unsafe"), t.is("try"), t.is("const"):
		return p.peek2().is("{")
	case t.is("async"), t.is("gen"):
		n := p.peek2()
		return n.is("{") || n.is("move") && p.peekN(2).is("{")
	case t.is("'"):
		return p.peekN(2).is(":")
	}
	return false
}

// expressionWithBlock parses an _expression_ending_with_block.
func (p *parser) expressionWithBlock() *syntax.Node {
	t := p.peek()
	switch {
	case t.is("{"):
		return p.block()
	case t.is("if"):
		return p.ifExpression()
	case t.is("match"):
		return p.matchExpression()
	case t.is("unsafe"):
		return node("unsafe_block", p.expect("unsafe"), p.block())
	case t.is("try"):
		return node("try_block", p.expect("try"), p.block())
	case t.is("const"):
		return node("const_block", p.expect("const"), field("body", p.block()))
	case t.is("async"), t.is("gen"):
		kw := p.expect(t.text)
		mv := p.accept("move")
		typ := "async_block"
		if t.text == "gen" {
			typ = "gen_block"
		}
		return node(typ, kw, mv, p.block())
	}
	var label, colon *syntax.Node
	if t.is("'") {
		if p.peekN(3).is("{") {
			return p.block()
		}
		label = p.label()
		colon = p.expect(":")
	}
	switch {
	case p.at("while"):
		kw := p.expect("while")
		cond := p.condition()
		return node("while_expression", label, colon, kw, field("condition", cond), field("body", p.block()))
	case p.at("loop"):
		kw := p.expect("loop")
		return node("loop_expression", label, colon, kw, field("body", p.block()))
	case p.at("for"):
		kw := p.expect("for")
		pat := p.pattern()
		in := p.expect("in")
		val := p.expression(exprCtx{noStruct: true})
		return node("for_expression", label, colon, kw, field("pattern", pat), in, field("value", val), field("body", p.block()))
	}
	fail(p.peek().start, "expected block expression")
	return nil
}

func (p *parser) ifExpression() *syntax.Node {
	kw := p.expect("if")
	cond := p.condition()
	cons := p.block()
	parts := []*syntax.Node{kw, field("condition", cond), field("consequence", cons)}
	if p.at("else") {
		els := p.expect("else")
		var alt *syntax.Node
		if p.at("if") {
			alt = p.ifExpression()
		} else {
			alt = p.block()
		}
		parts = append(parts, field("alternative", node("else_clause", els, alt)))
	}
	return node("if_expression", parts...)
}

// condition parses a _condition: an expression, a let_condition, or a
// let_chain.
func (p *parser) condition() *syntax.Node {
	ctx := exprCtx{noStruct: true, stopLet: true}
	var first *syntax.Node
	if p.at("let") {
		first = p.letCondition()
	} else {
		first = p.expression(ctx)
	}
	if !p.at("&&") {
		return first
	}
	parts := []*syntax.Node{first}
	for p.at("&&") {
		parts = append(parts, p.expect("&&"))
		if p.at("let") {
			parts = append(parts, p.letCondition())
		} else {
			parts = append(parts, p.expression(exprCtx{noStruct: true, stopLet: true, stopAnd: true}))
		}
	}
	return node("let_chain", parts...)
}

func (p *parser) letCondition() *syntax.Node {
	kw := p.expect("let")
	pat := p.pattern()
	eq := p.expect("=")
	val := p.expression(exprCtx{noStruct: true, stopLet: true, stopAnd: true})
	return node("let_condition", kw, field("pattern", pat), eq, field("value", val))
}

func (p *parser) matchExpression() *syntax.Node {
	kw := p.expect("match")
	val := p.expression(exprCtx{noStruct: true})
	parts := []*syntax.Node{p.expect("{")}
	for !p.at("}") {
		parts = append(parts, p.matchArm())
	}
	parts = append(parts, p.expect("}"))
	return node("match_expression", kw, field("value", val), field("body", node("match_block", parts...)))
}

func (p *parser) matchArm() *syntax.Node {
	var parts []*syntax.Node
	for p.at("#") {
		parts = append(parts, p.attributeItem())
	}
	mp := []*syntax.Node{p.pattern()}
	if p.at("if") {
		mp = append(mp, p.expect("if"), field("condition", p.condition()))
	}
	parts = append(parts, field("pattern", node("match_pattern", mp...)), p.expect("=>"))
	var value *syntax.Node
	if p.startsBlockLike() {
		value = p.expressionWithBlock()
		if p.continuesAfterBlockLike() {
			value = p.continueExpression(value, 0, exprCtx{})
		}
	} else {
		value = p.expression(exprCtx{})
	}
	parts = append(parts, field("value", value))
	if p.at(",") {
		parts = append(parts, p.expect(","))
	} else if !p.at("}") && !isBlockLike(value) {
		fail(p.peek().start, "expected , after match arm")
	}
	return node("match_arm", parts...)
}

func isBlockLike(n *syntax.Node) bool {
	switch n.Type {
	case "block", "if_expression", "match_expression", "while_expression", "loop_expression", "for_expression",
		"unsafe_block", "async_block", "gen_block", "try_block", "const_block":
		return true
	}
	return n.Type == "macro_invocation"
}

// ---------------------------------------------------------------- prefix

func (p *parser) prefixExpr(ctx exprCtx) *syntax.Node {
	t := p.peek()
	switch t.kind {
	case tkInt, tkFloat, tkChar, tkString, tkRawString:
		return p.literal()
	case tkEOF:
		fail(t.start, "unexpected end of file")
	}
	if p.startsBlockLike() {
		return p.expressionWithBlock()
	}
	if t.kind == tkIdent {
		return p.identExpr(ctx)
	}
	switch t.text {
	case "-", "*", "!":
		op := p.expect(t.text)
		return node("unary_expression", op, p.exprPrec(precTry, ctx.inner()))
	case "&", "&&":
		amp := p.expect("&")
		parts := []*syntax.Node{amp}
		if p.at("raw") && (p.peek2().is("const") || p.peek2().is("mut")) {
			parts = append(parts, p.expect("raw"))
			if p.at("const") {
				parts = append(parts, p.expect("const"))
			} else {
				parts = append(parts, p.mutableSpecifier())
			}
		} else if p.at("mut") {
			parts = append(parts, p.mutableSpecifier())
		}
		parts = append(parts, field("value", p.exprPrec(precTry, ctx.inner())))
		return node("reference_expression", parts...)
	case "(":
		return p.parenExpr()
	case "[":
		return p.arrayExpr()
	case "|", "||":
		return p.closure()
	case "..", "..=", "...":
		op := p.expect("..")
		if p.canStartRangeEnd(ctx, "..") {
			return node("range_expression", op, p.exprPrec(precRange+1, ctx.inner()))
		}
		return node("range_expression", op)
	case "<", "<<", "::":
		return p.pathExpr(ctx)
	case "#":
		fail(t.start, "attribute in expression position")
	}
	fail(t.start, "unexpected %q", t.text)
	return nil
}

func (p *parser) literal() *syntax.Node {
	t := p.next()
	switch t.kind {
	case tkInt:
		return syntax.Leaf("integer_literal", true, t.start, t.end)
	case tkFloat:
		return syntax.Leaf("float_literal", true, t.start, t.end)
	case tkChar:
		return syntax.Leaf("char_literal", true, t.start, t.end)
	case tkString, tkRawString:
		return t.node
	}
	if t.is("true") || t.is("false") {
		return node("boolean_literal", p.anon(t))
	}
	fail(t.start, "expected literal, got %q", t.text)
	return nil
}

func (p *parser) identExpr(ctx exprCtx) *syntax.Node {
	t := p.peek()
	switch t.text {
	case "true", "false":
		return p.literal()
	case "return", "yield":
		kw := p.expect(t.text)
		typ := t.text + "_expression"
		if p.canStartExpression() && !(ctx.noStruct && p.at("{")) {
			return node(typ, kw, p.exprPrec(1, ctx.inner()))
		}
		return node(typ, kw)
	case "break":
		parts := []*syntax.Node{p.expect("break")}
		if p.at("'") {
			parts = append(parts, p.label())
		}
		// A bare break is as tight as `break x` (prec.left), so a token that
		// could also continue the expression ends it: `break -1` is
		// (break) - 1, `break (x)` calls the break.
		if p.canStartExpression() && !(ctx.noStruct && p.at("{")) && !p.atInfixAmbiguous() {
			parts = append(parts, p.exprPrec(1, ctx.inner()))
		}
		return node("break_expression", parts...)
	case "continue":
		parts := []*syntax.Node{p.expect("continue")}
		if p.at("'") {
			parts = append(parts, p.label())
		}
		return node("continue_expression", parts...)
	case "move", "static":
		return p.closure()
	case "async":
		if n := p.peek2(); n.is("move") || n.is("|") || n.is("||") {
			return p.closure()
		}
	case "_":
		p.next()
		return syntax.Leaf("identifier", true, t.start, t.end)
	}
	return p.pathExpr(ctx)
}

// pathExpr parses identifiers, paths, generic functions, macro invocations
// and struct expressions.
func (p *parser) pathExpr(ctx exprCtx) *syntax.Node {
	n := p.exprPathStart()
	for p.at("::") {
		if p.atTurbofish() {
			sep := p.expect("::")
			args := p.typeArguments()
			if p.at("::") && p.peek2().kind == tkIdent {
				g := node("generic_type", field("type", turbofishType(n)), sep, field("type_arguments", args))
				sep2 := p.expect("::")
				n = node("scoped_identifier", field("path", g), sep2, field("name", p.scopedName()))
				continue
			}
			if !ctx.noStruct && p.at("{") {
				name := node("generic_type_with_turbofish", field("type", turbofishType(n)), sep, field("type_arguments", args))
				return node("struct_expression", field("name", name), field("body", p.fieldInitializerList()))
			}
			return node("generic_function", field("function", n), sep, field("type_arguments", args))
		}
		sep := p.expect("::")
		if n == nil {
			n = node("scoped_identifier", sep, field("name", p.scopedName()))
		} else {
			n = node("scoped_identifier", field("path", n), sep, field("name", p.scopedName()))
		}
	}
	if n == nil {
		fail(p.peek().start, "bad path")
	}
	if p.at("!") && (n.Type == "identifier" || n.Type == "scoped_identifier") && p.isDelimAt(p.peek2()) {
		return p.macroInvocation(n)
	}
	if !ctx.noStruct && p.at("{") && isStructName(n) {
		return node("struct_expression", field("name", toTypeName(n)), field("body", p.fieldInitializerList()))
	}
	return n
}

func (p *parser) isDelimAt(t token) bool { return t.is("(") || t.is("[") || t.is("{") }

// exprPathStart parses the first segment of an expression path; nil means
// a leading `::` follows.
func (p *parser) exprPathStart() *syntax.Node {
	t := p.peek()
	switch {
	case t.is("::"):
		return nil
	case p.atSplit("<"):
		b := p.bracketedType()
		if !p.at("::") {
			fail(p.peek().start, "expected :: after qualified path")
		}
		sep := p.expect("::")
		return node("scoped_identifier", field("path", b), sep, field("name", p.scopedName()))
	case t.is("self"), t.is("super"), t.is("crate"):
		return p.kwNode(t.text)
	case t.is("$"):
		return p.metavariable()
	case t.kind == tkIdent:
		return p.ident()
	}
	fail(t.start, "expected expression, got %q", t.text)
	return nil
}

func (p *parser) scopedName() *syntax.Node {
	if p.at("super") {
		return p.kwNode("super")
	}
	return p.ident()
}

func isStructName(n *syntax.Node) bool {
	switch n.Type {
	case "identifier":
		return true
	case "scoped_identifier":
		return n.ChildByFieldName("name").Type == "identifier"
	}
	return false
}

// toTypeName converts an expression path to its type spelling:
// identifier -> type_identifier, scoped_identifier -> scoped_type_identifier.
func toTypeName(n *syntax.Node) *syntax.Node {
	switch n.Type {
	case "identifier":
		c := *n
		c.Type = "type_identifier"
		return &c
	case "scoped_identifier":
		c := *n
		c.Type = "scoped_type_identifier"
		kids := make([]*syntax.Node, len(n.Children))
		copy(kids, n.Children)
		last := *kids[len(kids)-1]
		last.Type = "type_identifier"
		kids[len(kids)-1] = &last
		c.Children = kids
		return &c
	}
	return n
}

// turbofishType is the type of generic_type_with_turbofish: an identifier
// becomes a type_identifier, a scoped_identifier stays one.
func turbofishType(n *syntax.Node) *syntax.Node {
	if n.Type == "identifier" {
		return toTypeName(n)
	}
	return n
}

func (p *parser) fieldInitializerList() *syntax.Node {
	parts := []*syntax.Node{p.expect("{")}
	for !p.at("}") {
		attrs := p.attributes()
		switch {
		case p.at(".."):
			parts = append(parts, attrs...)
			dots := p.expect("..")
			parts = append(parts, node("base_field_initializer", dots, p.expression(exprCtx{})))
		case p.peek2().is(":") && !p.peek2().is("::"):
			var name *syntax.Node
			if t := p.peek(); t.kind == tkInt {
				p.next()
				name = syntax.Leaf("integer_literal", true, t.start, t.end)
			} else {
				name = p.identAs("field_identifier")
			}
			colon := p.expect(":")
			fi := append(attrs, field("field", name), colon, field("value", p.expression(exprCtx{})))
			parts = append(parts, node("field_initializer", fi...))
		default:
			parts = append(parts, node("shorthand_field_initializer", append(attrs, p.ident())...))
		}
		if !p.at("}") {
			parts = append(parts, p.expect(","))
		}
	}
	parts = append(parts, p.expect("}"))
	return node("field_initializer_list", parts...)
}

func (p *parser) parenExpr() *syntax.Node {
	open := p.expect("(")
	if p.at(")") {
		return node("unit_expression", open, p.expect(")"))
	}
	parts := []*syntax.Node{open}
	parts = append(parts, p.attributes()...)
	first := p.expression(exprCtx{})
	parts = append(parts, first)
	if p.at(")") {
		parts = append(parts, p.expect(")"))
		return node("parenthesized_expression", parts...)
	}
	for p.at(",") {
		parts = append(parts, p.expect(","))
		if p.at(")") {
			break
		}
		parts = append(parts, p.attributes()...)
		parts = append(parts, p.expression(exprCtx{}))
	}
	parts = append(parts, p.expect(")"))
	return node("tuple_expression", parts...)
}

func (p *parser) arrayExpr() *syntax.Node {
	parts := []*syntax.Node{p.expect("[")}
	parts = append(parts, p.attributes()...)
	if p.at("]") {
		return node("array_expression", append(parts, p.expect("]"))...)
	}
	first := p.expression(exprCtx{})
	parts = append(parts, first)
	if p.at(";") {
		parts = append(parts, p.expect(";"), field("length", p.expression(exprCtx{})))
		return node("array_expression", append(parts, p.expect("]"))...)
	}
	for p.at(",") {
		parts = append(parts, p.expect(","))
		if p.at("]") {
			break
		}
		parts = append(parts, p.attributes()...)
		parts = append(parts, p.expression(exprCtx{}))
	}
	return node("array_expression", append(parts, p.expect("]"))...)
}

func (p *parser) closure() *syntax.Node {
	var parts []*syntax.Node
	parts = append(parts, p.accept("static"), p.accept("async"), p.accept("move"))
	params := []*syntax.Node{p.expect("|")}
	for !p.atSplit("|") {
		params = append(params, p.closureParam())
		if !p.atSplit("|") {
			params = append(params, p.expect(","))
		}
	}
	params = append(params, p.expect("|"))
	parts = append(parts, field("parameters", node("closure_parameters", params...)))
	if p.at("->") {
		parts = append(parts, p.expect("->"), field("return_type", p.typ()), field("body", p.block()))
		return node("closure_expression", parts...)
	}
	if p.at("_") && !p.peek2().is("=") {
		parts = append(parts, field("body", p.expect("_")))
		return node("closure_expression", parts...)
	}
	parts = append(parts, field("body", p.expression(exprCtx{closure: true})))
	return node("closure_expression", parts...)
}

func (p *parser) closureParam() *syntax.Node {
	if p.at("mut") {
		save := p.pos
		var n *syntax.Node
		if p.attempt(func() {
			mut := p.mutableSpecifier()
			pat := p.patternNoOr()
			colon := p.expect(":")
			n = node("parameter", mut, field("pattern", pat), colon, field("type", p.typ()))
		}) {
			return n
		}
		p.pos = save
	}
	pat := p.patternNoOr()
	if p.at(":") {
		colon := p.expect(":")
		return node("parameter", field("pattern", pat), colon, field("type", p.typ()))
	}
	return pat
}
