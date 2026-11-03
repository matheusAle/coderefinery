package java

import "github.com/matheusAle/ub-mode/ubgate/internal/syntax"

// Binary operator precedences (tree-sitter-java PREC).
var binaryPrec = map[string]int{
	"||": 4, "&&": 5, "|": 6, "^": 7, "&": 8, "==": 9, "!=": 9,
	"<": 10, ">": 10, "<=": 10, ">=": 10, "instanceof": 10,
	"<<": 11, ">>": 11, ">>>": 11, "+": 12, "-": 12, "*": 13, "/": 13, "%": 13,
}

var assignOps = map[string]bool{
	"=": true, "+=": true, "-=": true, "*=": true, "/=": true, "%=": true, "&=": true, "|=": true,
	"^=": true, "<<=": true, ">>=": true, ">>>=": true,
}

// opAt returns the operator at the current position, joining the
// adjacent `>`, `>`, `>`, `=` tokens the lexer keeps apart, and the
// number of tokens it spans.
func (p *parser) opAt() (string, int) {
	t := p.cur()
	if t.kind == tkIdent && t.text == "instanceof" {
		return t.text, 1
	}
	if t.kind != tkOp {
		return "", 0
	}
	if t.text != ">" {
		return t.text, 1
	}
	text, n, end := ">", 1, t.end
	for n < 3 {
		nt := p.peek(n)
		if nt.kind != tkOp || nt.start != end || nt.text != ">" {
			break
		}
		text, n, end = text+">", n+1, nt.end
	}
	if nt := p.peek(n); nt.kind == tkOp && nt.start == end && (nt.text == "=" || nt.text == "==") {
		if nt.text == "=" {
			return text + "=", n + 1
		}
	}
	return text, n
}

func (p *parser) takeOp(text string, n int) *syntax.Node {
	start := p.cur().start
	end := p.peek(n - 1).end
	p.pos += n
	return syntax.Leaf(text, false, start, end)
}

func (p *parser) expression() *syntax.Node {
	if p.isLambdaStart() {
		return p.lambda()
	}
	left := p.ternary()
	if op, n := p.opAt(); assignOps[op] {
		switch left.Type {
		case "identifier", "field_access", "array_access":
			opNode := field("operator", p.takeOp(op, n))
			return inner("assignment_expression", field("left", left), opNode, field("right", p.expression()))
		}
	}
	return left
}

func (p *parser) ternary() *syntax.Node {
	cond := p.binary(4, 0)
	if !p.isOp("?") {
		return cond
	}
	q := p.next()
	cons := p.expression()
	colon := p.expect(":")
	var alt *syntax.Node
	if p.isLambdaStart() {
		alt = p.lambda()
	} else {
		alt = p.ternary()
	}
	return inner("ternary_expression", field("condition", cond), q, field("consequence", cons), colon, field("alternative", alt))
}

// binary is precedence climbing; leftPrec is the precedence of the
// operator whose right operand this chain is (0 at the start of an
// expression), which the cast ambiguity rule needs (see cast).
func (p *parser) binary(minPrec, leftPrec int) *syntax.Node {
	p.leftPrec = leftPrec
	left := p.unary()
	for {
		op, n := p.opAt()
		prec, ok := binaryPrec[op]
		if !ok || prec < minPrec {
			return left
		}
		if op == "instanceof" {
			left = p.instanceofRest(left)
			continue
		}
		opNode := field("operator", p.takeOp(op, n))
		right := p.binary(prec+1, prec)
		left = inner("binary_expression", field("left", left), opNode, field("right", right))
	}
}

func (p *parser) instanceofRest(left *syntax.Node) *syntax.Node {
	kids := []*syntax.Node{field("left", left), p.next()}
	if p.isKw("final") {
		kids = append(kids, p.next())
	}
	if p.isIdent() && (p.isOpAt(1, "(") || p.isOpAt(1, "<")) {
		if rp := p.try(p.recordPattern); rp != nil {
			return inner("instanceof_expression", append(kids, field("pattern", rp))...)
		}
	}
	kids = append(kids, field("right", p.typ()))
	if p.isIdent() {
		kids = append(kids, field("name", p.identifier()))
	}
	return inner("instanceof_expression", kids...)
}

// unary: prefix operators and casts bind before postfix `++`/`--`
// (tree-sitter: `-a++` is update(unary), `(byte) i++` is update(cast)).
func (p *parser) unary() *syntax.Node {
	n := p.unaryCore()
	for p.isOp("++") || p.isOp("--") {
		n = inner("update_expression", n, p.next())
	}
	return n
}

func (p *parser) unaryCore() *syntax.Node {
	t := p.cur()
	if t.kind == tkOp {
		switch t.text {
		case "+", "-", "!", "~":
			op := field("operator", p.next())
			return inner("unary_expression", op, field("operand", p.unaryCore()))
		case "++", "--":
			op := p.next()
			return inner("update_expression", op, p.unaryCore())
		case "(":
			if c := p.try(p.cast); c != nil {
				return c
			}
		}
	}
	return p.primary()
}

// castValueStart: tokens that can begin the operand of a reference-type
// cast (tree-sitter reads `(Foo) -x` and `(Foo) +x` as binary).
func (p *parser) castValueStart() bool {
	t := p.cur()
	switch t.kind {
	case tkLit, tkStrStart:
		return true
	case tkIdent:
		switch t.text {
		case "instanceof":
			return false
		}
		return true
	case tkOp:
		switch t.text {
		case "(", "!", "~", "++", "--", "@":
			return true
		}
	}
	return false
}

func (p *parser) cast() *syntax.Node {
	kids := []*syntax.Node{p.expect("(")}
	first := p.typ()
	kids = append(kids, field("type", first))
	inter := false
	for p.isOp("&") {
		inter = true
		kids = append(kids, p.next(), field("type", p.typ()))
	}
	kids = append(kids, p.expect(")"))
	if !inter && isPrimitiveType(first) {
		if p.isKw("instanceof") || p.cur().kind == tkOp && !p.isPrefixOp() {
			return nil
		}
	} else if p.isOp("+") || p.isOp("-") {
		return p.ambiguousSignCast(kids)
	} else if !p.castValueStart() {
		return nil
	}
	var value *syntax.Node
	if p.isLambdaStart() {
		value = p.lambda()
	} else {
		value = p.unaryCore()
	}
	return inner("cast_expression", append(kids, field("value", value))...)
}

// ambiguousSignCast decides `(X) + y` / `(X) - y` with a reference type X,
// which tree-sitter parses both ways (GLR) and then picks one: the binary
// reading when both readings complete at the same time (y is not followed
// by `*`, `/` or `%`), otherwise the tree that compares smaller symbol by
// symbol, which is the cast unless `(X)` is itself the right operand of an
// additive operator.
func (p *parser) ambiguousSignCast(kids []*syntax.Node) *syntax.Node {
	if p.leftPrec == binaryPrec["+"] {
		return nil
	}
	value := p.unaryCore()
	switch op, _ := p.opAt(); op {
	case "*", "/", "%":
		return inner("cast_expression", append(kids, field("value", value))...)
	}
	return nil
}

func (p *parser) isPrefixOp() bool {
	switch p.cur().text {
	case "+", "-", "!", "~", "++", "--", "(":
		return true
	}
	return false
}

// isLambdaStart: `x ->`, `_ ->` or a parenthesized head followed by `->`.
func (p *parser) isLambdaStart() bool {
	if p.noLambda {
		return false
	}
	if (p.isIdent() || p.isKw("_")) && p.isOpAt(1, "->") {
		return true
	}
	if !p.isOp("(") {
		return false
	}
	depth := 0
	for k := 0; ; k++ {
		t := p.peek(k)
		if t.kind == tkEOF {
			return false
		}
		if t.kind != tkOp {
			continue
		}
		switch t.text {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return p.isOpAt(k+1, "->")
			}
		case "{", "}", ";":
			return false
		}
	}
}

func (p *parser) lambda() *syntax.Node {
	var params *syntax.Node
	if !p.isOp("(") {
		params = p.named("identifier")
	} else if inf := p.try(p.inferredParameters); inf != nil {
		params = inf
	} else {
		params = p.formalParameters()
	}
	arrow := p.expect("->")
	var body *syntax.Node
	if p.isOp("{") {
		body = p.block()
	} else {
		body = p.expression()
	}
	return inner("lambda_expression", field("parameters", params), arrow, field("body", body))
}

func (p *parser) inferredParameters() *syntax.Node {
	kids := []*syntax.Node{p.expect("(")}
	if p.isOp(")") {
		return nil
	}
	kids = p.commaList(kids, p.identifier)
	kids = append(kids, p.expect(")"))
	return inner("inferred_parameters", kids...)
}

func (p *parser) parenthesized() *syntax.Node {
	open := p.expect("(")
	e := p.expression()
	return inner("parenthesized_expression", open, e, p.expect(")"))
}

func (p *parser) argumentList() *syntax.Node {
	kids := []*syntax.Node{p.expect("(")}
	if !p.isOp(")") {
		kids = p.commaList(kids, p.expression)
	}
	return inner("argument_list", append(kids, p.expect(")"))...)
}

var literalWords = map[string]string{"true": "true", "false": "false", "null": "null_literal", "this": "this"}

// primary parses a primary expression with its selectors.
func (p *parser) primary() *syntax.Node {
	return p.selectors(p.primaryBase())
}

func (p *parser) primaryBase() *syntax.Node {
	t := p.cur()
	switch t.kind {
	case tkLit:
		return p.named(t.typ)
	case tkStrStart:
		return p.stringLiteral()
	case tkOp:
		switch t.text {
		case "(":
			return p.parenthesized()
		case "@":
			if r := p.try(p.typeOnPrimary); r != nil {
				return r
			}
		}
		p.fail("unexpected %q", t.text)
	case tkIdent:
		if typ, ok := literalWords[t.text]; ok {
			return p.named(typ)
		}
		switch t.text {
		case "super":
			return p.named("super")
		case "new":
			return p.creation(nil)
		case "switch":
			return p.switchExpression()
		}
		if p.isPrimitive() {
			if r := p.typeOnPrimary(); r != nil {
				return r
			}
			p.fail("unexpected type %q", t.text)
		}
		if !p.isIdent() {
			p.fail("unexpected %q", t.text)
		}
		if p.isOpAt(1, "(") {
			name := field("name", p.identifier())
			return inner("method_invocation", name, field("arguments", p.argumentList()))
		}
		if p.isOpAt(1, "<") || p.isOpAt(1, ".") || p.isOpAt(1, "[") {
			if r := p.try(p.typeOnPrimary); r != nil {
				return r
			}
		}
		return p.identifier()
	}
	p.fail("unexpected end of input")
	return nil
}

// typeOnPrimary: a class literal (`T.class`) or a method reference on a
// generic or array type (`List<T>::new`, `int[]::new`); nil otherwise.
func (p *parser) typeOnPrimary() *syntax.Node {
	typ := p.typ()
	if p.isOp(".") && p.isKwAt(1, "class") {
		return inner("class_literal", typ, p.next(), p.next())
	}
	if p.isOp("::") && (typ.Type != "type_identifier" && typ.Type != "scoped_type_identifier") {
		return p.methodReferenceRest(typ)
	}
	return nil
}

func (p *parser) methodReferenceRest(target *syntax.Node) *syntax.Node {
	kids := []*syntax.Node{target, p.expect("::")}
	if p.isOp("<") {
		kids = append(kids, p.typeArguments())
	}
	if p.isKw("new") {
		kids = append(kids, p.next())
	} else {
		kids = append(kids, p.identifier())
	}
	return inner("method_reference", kids...)
}

func (p *parser) selectors(n *syntax.Node) *syntax.Node {
	for {
		switch {
		case p.isOp("."):
			nt := p.peek(1)
			switch {
			case nt.kind == tkIdent && nt.text == "new":
				n = p.creation(n)
			case nt.kind == tkOp && nt.text == "<":
				dot := p.next()
				targs := field("type_arguments", p.typeArguments())
				name := field("name", p.identifier())
				n = inner("method_invocation", field("object", n), dot, targs, name, field("arguments", p.argumentList()))
			case nt.kind == tkIdent && nt.text == "this":
				dot := p.next()
				n = inner("field_access", field("object", n), dot, field("field", p.named("this")))
			case nt.kind == tkIdent && nt.text == "super":
				if !p.isOpAt(2, ".") {
					return n
				}
				dot, sup, dot2 := p.next(), p.named("super"), p.next()
				kids := []*syntax.Node{field("object", n), dot, sup, dot2}
				if p.isOp("<") {
					kids = append(kids, field("type_arguments", p.typeArguments()))
				}
				if p.isOpAt(1, "(") || p.isOp("<") {
					kids = append(kids, field("name", p.identifier()), field("arguments", p.argumentList()))
					n = inner("method_invocation", kids...)
				} else {
					n = inner("field_access", append(kids, field("field", p.identifier()))...)
				}
			case nt.kind == tkStrStart:
				dot := p.next()
				n = inner("template_expression", field("template_processor", n), dot, field("template_argument", p.stringLiteral()))
			case p.isIdentAt(1):
				dot := p.next()
				if p.isOpAt(1, "(") {
					name := field("name", p.identifier())
					n = inner("method_invocation", field("object", n), dot, name, field("arguments", p.argumentList()))
				} else {
					n = inner("field_access", field("object", n), dot, field("field", p.identifier()))
				}
			default:
				return n
			}
		case p.isOp("[") && !p.isOpAt(1, "]"):
			open := p.next()
			idx := field("index", p.expression())
			n = inner("array_access", field("array", n), open, idx, p.expect("]"))
		case p.isOp("::"):
			n = p.methodReferenceRest(n)
		default:
			return n
		}
	}
}

// creation parses `new ...` (object or array creation); qualifier is the
// primary before `.new`, or nil.
func (p *parser) creation(qualifier *syntax.Node) *syntax.Node {
	var kids []*syntax.Node
	if qualifier != nil {
		kids = append(kids, qualifier, p.expect("."))
	}
	kids = append(kids, p.expect("new"))
	if qualifier == nil && !p.isOp("<") {
		save := p.pos
		anns := p.annotations()
		typ := p.simpleType()
		if p.isOp("[") || p.isOp("@") {
			kids = append(kids, anns...)
			kids = append(kids, field("type", typ))
			return p.arrayCreationRest(kids)
		}
		p.pos = save
	}
	if p.isOp("<") {
		kids = append(kids, field("type_arguments", p.typeArguments()))
	}
	kids = append(kids, p.annotations()...)
	kids = append(kids, field("type", p.simpleType()), field("arguments", p.argumentList()))
	if p.isOp("{") {
		kids = append(kids, p.classBody())
	}
	return inner("object_creation_expression", kids...)
}

func (p *parser) arrayCreationRest(kids []*syntax.Node) *syntax.Node {
	if p.atDimensions() {
		kids = append(kids, field("dimensions", p.dimensions()), field("value", p.arrayInitializer()))
		return inner("array_creation_expression", kids...)
	}
	for p.isOp("[") && !p.isOpAt(1, "]") || p.isOp("@") && !p.atDimensions() {
		d := p.annotations()
		d = append(d, p.expect("["), p.expression(), p.expect("]"))
		kids = append(kids, field("dimensions", inner("dimensions_expr", d...)))
	}
	if p.atDimensions() {
		kids = append(kids, field("dimensions", p.dimensions()))
	}
	return inner("array_creation_expression", kids...)
}

// stringLiteral builds string_literal from the lexer's content tokens.
func (p *parser) stringLiteral() *syntax.Node {
	kids := []*syntax.Node{p.next()}
	for {
		t := p.cur()
		switch t.kind {
		case tkStrEnd:
			kids = append(kids, p.next())
			return inner("string_literal", kids...)
		case tkFrag, tkEscape:
			kids = append(kids, p.named(t.typ))
		case tkHiddenEsc:
			p.pos++
		case tkInterp:
			open := p.next()
			e := p.expression()
			kids = append(kids, inner("string_interpolation", open, e, p.expect("}")))
		default:
			p.fail("bad string content")
		}
	}
}
