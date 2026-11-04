package golang

import (
	"go/token"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

// typeGuard is the transient node for `x.(type)` in a switch header.
const typeGuard = "\x00type_guard"

func (p *parser) expressionList() *syntax.Node {
	return node("expression_list", p.exprListItems(false)...)
}

func (p *parser) exprListItems(guard bool) []*syntax.Node {
	items := []*syntax.Node{p.exprG(guard)}
	for p.is(token.COMMA) {
		items = append(items, p.anon(), p.exprG(guard))
	}
	return items
}

// expr parses an expression; type literals ([]T, map, chan, func, struct,
// interface) are operands too, so call arguments and index operands may be
// bare types.
func (p *parser) expr() *syntax.Node { return p.binary(token.LowestPrec+1, false) }

func (p *parser) exprG(guard bool) *syntax.Node { return p.binary(token.LowestPrec+1, guard) }

func (p *parser) binary(prec1 int, guard bool) *syntax.Node {
	x := p.unary(guard)
	for {
		oprec := p.cur().t.Precedence()
		if oprec < prec1 {
			return x
		}
		op := field("operator", p.anon())
		y := p.binary(oprec+1, false)
		x = node("binary_expression", field("left", x), op, field("right", y))
	}
}

func (p *parser) unary(guard bool) *syntax.Node {
	switch p.cur().t {
	case token.ARROW:
		if p.peek(1).t == token.CHAN {
			return p.primary(p.parseType(), guard)
		}
		fallthrough
	case token.ADD, token.SUB, token.NOT, token.XOR, token.MUL, token.AND:
		op := p.anon()
		if op.Type == "*" && !isUnaryStart(p.cur().t) {
			// `*` before a type-only operand builds a pointer_type before
			// any postfix: *(*T)(x) is a conversion to *(*T).
			x := p.operand()
			if isPureType(x) {
				return p.primary(node("pointer_type", op, x), guard)
			}
			x = p.primary(x, false)
			return node("unary_expression", field("operator", op), field("operand", x))
		}
		x := p.unary(false)
		if op.Type == "*" && isPureType(x) {
			return node("pointer_type", op, x)
		}
		return node("unary_expression", field("operator", op), field("operand", x))
	}
	return p.primary(nil, guard)
}

func isUnaryStart(t token.Token) bool {
	switch t {
	case token.ADD, token.SUB, token.NOT, token.XOR, token.MUL, token.AND, token.ARROW:
		return true
	}
	return false
}

func (p *parser) operand() *syntax.Node {
	t := p.cur()
	switch t.t {
	case token.IDENT:
		switch t.lit {
		case "nil", "true", "false", "iota":
			return p.named(t.lit)
		}
		return p.named("identifier")
	case token.INT:
		return p.named("int_literal")
	case token.FLOAT:
		return p.named("float_literal")
	case token.IMAG:
		return p.named("imaginary_literal")
	case token.CHAR:
		return p.named("rune_literal")
	case token.STRING:
		return p.stringLit()
	case token.LPAREN:
		lp := p.anon()
		p.exprLev++
		x := p.expr()
		p.exprLev--
		rp := p.expect(token.RPAREN)
		if isPureType(x) {
			return node("parenthesized_type", lp, x, rp)
		}
		return node("parenthesized_expression", lp, x, rp)
	case token.FUNC:
		kw := p.anon()
		params := field("parameters", p.parameterList())
		result := field("result", p.result())
		if p.is(token.LBRACE) {
			p.exprLev++
			body := field("body", p.block())
			p.exprLev--
			return node("func_literal", kw, params, result, body)
		}
		return node("function_type", kw, params, result)
	case token.LBRACK, token.MAP, token.CHAN, token.STRUCT, token.INTERFACE:
		return p.parseType()
	}
	p.fail("unexpected %q in expression", t.lit)
	return nil
}

func (p *parser) primary(x *syntax.Node, guard bool) *syntax.Node {
	if x == nil {
		x = p.operand()
	}
	for {
		switch p.cur().t {
		case token.PERIOD:
			if p.peek(1).t == token.IDENT {
				dot := p.anon()
				x = node("selector_expression", field("operand", x), dot, field("field", p.named("field_identifier")))
				continue
			}
			if p.peek(1).t == token.LPAREN {
				if p.peek(2).t == token.TYPE {
					if !guard {
						p.fail("use of .(type) outside type switch")
					}
					dot, lp, kw := p.anon(), p.anon(), p.anon()
					rp := p.expect(token.RPAREN)
					return &syntax.Node{Type: typeGuard, Named: true, Children: []*syntax.Node{x, dot, lp, kw, rp}}
				}
				dot, lp := p.anon(), p.anon()
				typ := field("type", p.parseType())
				rp := p.expect(token.RPAREN)
				x = node("type_assertion_expression", field("operand", x), dot, lp, typ, rp)
				continue
			}
			p.fail("unexpected . in expression")
		case token.LBRACK:
			x = p.indexOrSlice(x)
		case token.LPAREN:
			x = p.call(x)
		case token.LBRACE:
			lit := p.compositeType(x)
			if lit == nil {
				return x
			}
			x = node("composite_literal", field("type", lit), field("body", p.literalValue()))
		default:
			return x
		}
	}
}

// indexOrSlice parses x[...]: index_expression, slice_expression or
// type_instantiation_expression (several or type-only arguments).
func (p *parser) indexOrSlice(x *syntax.Node) *syntax.Node {
	lb := p.anon()
	p.exprLev++
	if p.is(token.COLON) {
		defer func() { p.exprLev-- }()
		return p.sliceRest(x, lb, nil)
	}
	first := p.expr()
	if p.is(token.COLON) {
		defer func() { p.exprLev-- }()
		return p.sliceRest(x, lb, first)
	}
	elems := []*syntax.Node{first}
	for p.is(token.COMMA) {
		comma := p.anon()
		elems = append(elems, comma)
		if p.is(token.RBRACK) {
			break
		}
		elems = append(elems, p.expr())
	}
	p.exprLev--
	rb := p.expect(token.RBRACK)
	if len(elems) == 1 && !isPureType(first) {
		if !p.preferInstantiation(x, first) {
			return node("index_expression", field("operand", x), lb, field("index", first), rb)
		}
	}
	inst := &instantiation{base: x, lb: lb, elems: elems, rb: rb}
	n := p.instNode(inst)
	p.insts[n] = inst
	return n
}

// preferInstantiation decides x[e] (not followed by a call or a literal)
// between index_expression (score 0) and type_instantiation_expression.
func (p *parser) preferInstantiation(x, e *syntax.Node) bool {
	if p.is(token.LPAREN) || p.is(token.LBRACE) && p.exprLev >= 0 {
		return false // decided by call / compositeType
	}
	if !p.typeable(e) || !p.typeable(x) || x.Type == "unary_expression" || x.Type == "parenthesized_expression" {
		return false
	}
	if x.Type == "index_expression" && !genericBase(x.Children[0]) {
		return false
	}
	return dynInst+p.tscore(x)+p.tscore(e) > p.escore(x)+p.escore(e)
}

func (p *parser) sliceRest(x, lb, start *syntax.Node) *syntax.Node {
	kids := []*syntax.Node{field("operand", x), lb, field("start", start), p.anon()}
	if !p.is(token.RBRACK) && !p.is(token.COLON) {
		kids = append(kids, field("end", p.expr()))
	}
	if p.is(token.COLON) {
		kids = append(kids, p.anon(), field("capacity", p.expr()))
	}
	kids = append(kids, p.expect(token.RBRACK))
	return node("slice_expression", kids...)
}

// call parses the argument list after x and applies tree-sitter-go's
// syntactic choice between call_expression and type_conversion_expression.
func (p *parser) call(x *syntax.Node) *syntax.Node {
	lp := p.anon()
	p.exprLev++
	var items []*syntax.Node // args and commas
	nargs, variadic := 0, false
	for !p.is(token.RPAREN) {
		a := p.expr()
		if p.is(token.ELLIPSIS) {
			a = node("variadic_argument", a, p.anon())
			variadic = true
		}
		items = append(items, a)
		nargs++
		if !p.is(token.COMMA) {
			break
		}
		items = append(items, p.anon())
	}
	p.exprLev--
	rp := p.expect(token.RPAREN)
	single := nargs == 1 && !variadic
	args := func() *syntax.Node {
		return field("arguments", node("argument_list", append(append([]*syntax.Node{lp}, items...), rp)...))
	}
	if inst, ok := p.insts[x]; ok {
		if single && genericBase(inst.base) {
			return conversion(p.instGeneric(inst), lp, items, rp)
		}
		inst.base.Field = ""
		return node("call_expression", field("function", inst.base), field("type_arguments", p.instTypeArguments(inst)), args())
	}
	if x.Type == "index_expression" && p.typeable(x.Children[2]) {
		// GLR alternatives: conversion, call with type arguments, call of
		// an index expression (score 0, wins ties).
		elem := p.tscore(x.Children[2])
		expr := p.escore(x)
		if single && genericBase(x.Children[0]) && dynGeneric+dynTypeArgs+elem > expr {
			return conversion(p.toType(x), lp, items, rp)
		}
		if dynTypeArgs+elem > expr {
			base := x.Children[0]
			base.Field = ""
			targs := node("type_arguments", p.clone(x.Children[1]), node("type_elem", p.toType(x.Children[2])), p.clone(x.Children[3]))
			return node("call_expression", field("function", base), field("type_arguments", targs), args())
		}
	}
	if single && isPureType(x) {
		x.Field = ""
		return conversion(x, lp, items, rp)
	}
	if single && x.Type == "parenthesized_expression" && p.typeable(x) && p.tscore(x) > p.escore(x) {
		return conversion(p.toType(x), lp, items, rp)
	}
	if x.Type == "identifier" && (p.text(x) == "new" || p.text(x) == "make") && nargs > 0 && !variadic {
		switch a := items[0]; {
		case a.Type == "nil" || a.Type == "true" || a.Type == "false" || a.Type == "iota":
			a.Type = "type_identifier"
		case p.typeable(a):
			items[0] = p.toType(a)
		}
	}
	x.Field = ""
	return node("call_expression", field("function", x), args())
}

func conversion(typ, lp *syntax.Node, items []*syntax.Node, rp *syntax.Node) *syntax.Node {
	kids := []*syntax.Node{field("type", typ), lp, field("operand", items[0])}
	kids = append(kids, items[1:]...)
	kids = append(kids, rp)
	return node("type_conversion_expression", kids...)
}

func (p *parser) text(n *syntax.Node) string { return string(p.src[n.StartByte:n.EndByte]) }

// compositeType returns the composite literal type for x before "{", or nil
// when the brace opens a block (go/parser's rule, which tree-sitter's
// precedences agree with on valid code).
func (p *parser) compositeType(x *syntax.Node) *syntax.Node {
	switch x.Type {
	case "identifier", "selector_expression", "index_expression":
		if p.exprLev < 0 || !p.typeable(x) {
			return nil
		}
		x.Field = ""
		return p.toType(x)
	case "slice_type", "array_type", "implicit_length_array_type", "map_type", "struct_type":
		x.Field = ""
		return x
	case "generic_type", "qualified_type", "type_identifier":
		if p.exprLev < 0 {
			return nil
		}
		x.Field = ""
		return x
	}
	if inst, ok := p.insts[x]; ok && p.exprLev >= 0 && p.typeable(inst.base) {
		return p.instGeneric(inst)
	}
	return nil
}

func (p *parser) literalValue() *syntax.Node {
	kids := []*syntax.Node{p.expect(token.LBRACE)}
	lev := p.exprLev
	p.exprLev = 0
	for !p.is(token.RBRACE) {
		e := p.literalElement()
		if p.is(token.COLON) {
			colon := p.anon()
			e = node("keyed_element", field("key", e), colon, field("value", p.literalElement()))
		}
		kids = append(kids, e)
		if !p.is(token.COMMA) {
			break
		}
		kids = append(kids, p.anon())
	}
	p.exprLev = lev
	kids = append(kids, p.expect(token.RBRACE))
	return node("literal_value", kids...)
}

func (p *parser) literalElement() *syntax.Node {
	if p.is(token.LBRACE) {
		return node("literal_element", p.literalValue())
	}
	return node("literal_element", p.expr())
}

// stringLit splits a string token into tree-sitter-go's children.
func (p *parser) stringLit() *syntax.Node {
	t := p.next()
	s := p.src
	if s[t.start] == '`' {
		open := syntax.Leaf("`", false, t.start, t.start+1)
		content := syntax.Leaf("raw_string_literal_content", true, t.start+1, t.end-1)
		closing := syntax.Leaf("`", false, t.end-1, t.end)
		return node("raw_string_literal", open, content, closing)
	}
	kids := []*syntax.Node{syntax.Leaf(`"`, false, t.start, t.start+1)}
	i, run := t.start+1, t.start+1
	end := t.end - 1
	flush := func(to int) {
		if to > run {
			kids = append(kids, syntax.Leaf("interpreted_string_literal_content", true, run, to))
		}
	}
	for i < end {
		if s[i] != '\\' {
			i++
			continue
		}
		flush(i)
		j := escapeEnd(s, i, end)
		kids = append(kids, syntax.Leaf("escape_sequence", true, i, j))
		i, run = j, j
	}
	flush(end)
	kids = append(kids, syntax.Leaf(`"`, false, end, t.end))
	return node("interpreted_string_literal", kids...)
}

// escapeEnd is the end of the escape sequence starting at s[i] == '\\'.
func escapeEnd(s []byte, i, end int) int {
	c := s[i+1]
	count := func(from, n int, ok func(byte) bool) int {
		j := from
		for j < end && j-from < n && ok(s[j]) {
			j++
		}
		return j
	}
	switch {
	case c == 'x':
		return count(i+2, 2, isHex)
	case c == 'u':
		return count(i+2, 4, isHex)
	case c == 'U':
		return count(i+2, 8, isHex)
	case c >= '0' && c <= '9':
		return count(i+1, 3, func(b byte) bool { return b >= '0' && b <= '9' })
	}
	// one (possibly multi-byte) character
	j := i + 2
	for j < end && s[j]&0xC0 == 0x80 {
		j++
	}
	return j
}

func isHex(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}
