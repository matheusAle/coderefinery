package golang

import (
	"go/token"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

// pureTypes are the type nodes that are not also valid expressions.
var pureTypes = map[string]bool{
	"type_identifier": true, "qualified_type": true, "generic_type": true, "pointer_type": true,
	"slice_type": true, "array_type": true, "implicit_length_array_type": true, "map_type": true,
	"channel_type": true, "function_type": true, "struct_type": true, "interface_type": true,
	"parenthesized_type": true, "negated_type": true,
}

func isPureType(n *syntax.Node) bool { return pureTypes[n.Type] }

func isTypeStart(t token.Token) bool {
	switch t {
	case token.IDENT, token.MUL, token.LBRACK, token.MAP, token.CHAN, token.ARROW, token.FUNC,
		token.STRUCT, token.INTERFACE, token.LPAREN:
		return true
	}
	return false
}

// parseType parses tree-sitter-go's _type (negated_type included, for
// constraints).
func (p *parser) parseType() *syntax.Node {
	switch p.cur().t {
	case token.IDENT:
		var base *syntax.Node
		if p.peek(1).t == token.PERIOD && p.peek(2).t == token.IDENT {
			pkg := field("package", p.named("package_identifier"))
			dot := p.anon()
			base = node("qualified_type", pkg, dot, field("name", p.named("type_identifier")))
		} else {
			base = p.named("type_identifier")
		}
		if p.is(token.LBRACK) && p.peek(1).t != token.RBRACK {
			return node("generic_type", field("type", base), field("type_arguments", p.typeArguments()))
		}
		return base
	case token.MUL:
		star := p.anon()
		return node("pointer_type", star, p.parseType())
	case token.TILDE:
		tilde := p.anon()
		return node("negated_type", tilde, p.parseType())
	case token.LPAREN:
		lp := p.anon()
		t := p.parseType()
		return node("parenthesized_type", lp, t, p.expect(token.RPAREN))
	case token.LBRACK:
		lb := p.anon()
		if p.is(token.RBRACK) {
			rb := p.anon()
			return node("slice_type", lb, rb, field("element", p.parseType()))
		}
		if p.is(token.ELLIPSIS) {
			dots := p.anon()
			rb := p.expect(token.RBRACK)
			return node("implicit_length_array_type", lb, dots, rb, field("element", p.parseType()))
		}
		p.exprLev++
		length := field("length", p.expr())
		p.exprLev--
		rb := p.expect(token.RBRACK)
		return node("array_type", lb, length, rb, field("element", p.parseType()))
	case token.MAP:
		kw := p.anon()
		lb := p.expect(token.LBRACK)
		key := field("key", p.parseType())
		rb := p.expect(token.RBRACK)
		return node("map_type", kw, lb, key, rb, field("value", p.parseType()))
	case token.CHAN:
		kw := p.anon()
		if p.is(token.ARROW) && p.peek(1).t != token.CHAN {
			arrow := p.anon()
			return node("channel_type", kw, arrow, field("value", p.parseType()))
		}
		return node("channel_type", kw, field("value", p.parseType()))
	case token.ARROW:
		arrow := p.anon()
		kw := p.expect(token.CHAN)
		return node("channel_type", arrow, kw, field("value", p.parseType()))
	case token.FUNC:
		kw := p.anon()
		params := field("parameters", p.parameterList())
		return node("function_type", kw, params, field("result", p.result()))
	case token.STRUCT:
		kw := p.anon()
		return node("struct_type", kw, p.fieldDeclarationList())
	case token.INTERFACE:
		return p.interfaceType()
	}
	p.fail("expected type, found %q", p.cur().lit)
	return nil
}

// typeArguments parses "[" type_elem {"," type_elem} [","] "]".
func (p *parser) typeArguments() *syntax.Node {
	kids := []*syntax.Node{p.anon()}
	p.exprLev++
	for !p.is(token.RBRACK) {
		kids = append(kids, p.typeElem("type_elem"))
		if !p.is(token.COMMA) {
			break
		}
		kids = append(kids, p.anon())
	}
	p.exprLev--
	kids = append(kids, p.expect(token.RBRACK))
	return node("type_arguments", kids...)
}

// typeElem parses sep1(_type, "|") as typ (type_elem or type_constraint).
func (p *parser) typeElem(typ string) *syntax.Node {
	kids := []*syntax.Node{p.parseType()}
	for p.is(token.OR) {
		kids = append(kids, p.anon(), p.parseType())
	}
	return node(typ, kids...)
}

// result parses an optional function result: parameter_list or _simple_type.
func (p *parser) result() *syntax.Node {
	t := p.cur().t
	if t == token.LPAREN {
		return p.parameterList()
	}
	if t == token.ARROW && p.peek(1).t != token.CHAN {
		return nil
	}
	if isTypeStart(t) {
		return p.parseType()
	}
	return nil
}

// maxGroupedNames is how many names tree-sitter-go puts into one
// parameter_declaration (`a1, ..., a9 T`) before its GLR stack-version
// limit drops the name readings of the leading identifiers.
const maxGroupedNames = 9

type paramEntry struct {
	name     *syntax.Node // identifier, or nil
	dots     *syntax.Node // "..." for variadic
	typ      *syntax.Node // nil for a bare identifier
	bare     bool         // a lone identifier: a name or a type
	comma    *syntax.Node // the comma after the entry
	variadic bool
}

// arrayAfterName reports whether "[" at index i starts an array or slice
// type of a named field/parameter (`a [N]T`, `a []T`) rather than type
// arguments (`G[T]`): after the matching "]" a type follows.
func (p *parser) arrayAfterName(i int) bool {
	j := p.matching(i)
	return j >= 0 && isTypeStart(p.toks[j+1].t) && p.toks[j+1].t != token.LPAREN
}

func (p *parser) paramEntry() *paramEntry {
	e := &paramEntry{}
	if p.is(token.ELLIPSIS) {
		e.dots = p.anon()
		e.typ = p.parseType()
		e.variadic = true
		return e
	}
	if p.is(token.IDENT) {
		switch p.peek(1).t {
		case token.COMMA, token.RPAREN:
			e.name = p.named("identifier")
			e.bare = true
			return e
		case token.PERIOD:
			e.typ = p.parseType()
			return e
		case token.LBRACK:
			if !p.arrayAfterName(p.pos + 1) {
				e.typ = p.parseType()
				return e
			}
		case token.ELLIPSIS:
			e.name = p.named("identifier")
			e.dots = p.anon()
			e.typ = p.parseType()
			e.variadic = true
			return e
		}
		e.name = p.named("identifier")
		e.typ = p.parseType()
		return e
	}
	e.typ = p.parseType()
	return e
}

// funcAsName rewrites the entry `func(T)` (one unnamed parameter, no
// result) into the name "func" and the type parenthesized_type (T): what
// tree-sitter-go builds after a bare identifier, e.g. `(int, func(int))`.
func funcAsName(e *paramEntry) bool {
	ft := e.typ
	if ft == nil || ft.Type != "function_type" || len(ft.Children) != 2 {
		return false
	}
	pl := ft.Children[1]
	if len(pl.Children) != 3 || pl.Children[1].Type != "parameter_declaration" || len(pl.Children[1].Children) != 1 {
		return false
	}
	kw := ft.Children[0]
	inner := pl.Children[1].Children[0]
	inner.Field = ""
	e.name = syntax.Leaf("identifier", true, kw.StartByte, kw.EndByte)
	e.typ = node("parenthesized_type", pl.Children[0], inner, pl.Children[2])
	return true
}

func (p *parser) parameterList() *syntax.Node {
	lp := p.expect(token.LPAREN)
	p.exprLev++
	var entries []*paramEntry
	for !p.is(token.RPAREN) {
		e := p.paramEntry()
		if n := len(entries); n > 0 && entries[n-1].bare && e.name == nil && !e.variadic {
			funcAsName(e)
		}
		entries = append(entries, e)
		if !p.is(token.COMMA) {
			break
		}
		e.comma = p.anon()
	}
	p.exprLev--
	rp := p.expect(token.RPAREN)
	named := false
	for _, e := range entries {
		if e.name != nil && !e.bare {
			named = true
		}
	}
	kids := []*syntax.Node{lp}
	var pending []*paramEntry // bare names waiting for a type
	typeOnly := func(e *paramEntry) {
		e.name.Type = "type_identifier"
		kids = append(kids, node("parameter_declaration", field("type", e.name)), e.comma)
	}
	for _, e := range entries {
		switch {
		case named && e.bare:
			pending = append(pending, e)
			continue
		case e.bare:
			typeOnly(e)
			continue
		}
		// tree-sitter's GLR keeps at most maxGroupedNames names in one
		// declaration; older bare names fall back to type-only entries.
		// A variadic declaration takes a single name.
		keep := len(pending)
		if e.variadic {
			keep = 0
		} else if keep > maxGroupedNames-1 {
			keep = maxGroupedNames - 1
		}
		for _, b := range pending[:len(pending)-keep] {
			typeOnly(b)
		}
		var decl []*syntax.Node
		for _, b := range pending[len(pending)-keep:] {
			decl = append(decl, field("name", b.name), b.comma)
		}
		pending = nil
		decl = append(decl, field("name", e.name))
		if e.variadic {
			decl = append(decl, e.dots, field("type", e.typ))
			kids = append(kids, node("variadic_parameter_declaration", decl...), e.comma)
			continue
		}
		decl = append(decl, field("type", e.typ))
		kids = append(kids, node("parameter_declaration", decl...), e.comma)
	}
	if len(pending) > 0 {
		p.fail("parameter names without a type")
	}
	kids = append(kids, rp)
	return node("parameter_list", kids...)
}

func (p *parser) typeParameterList() *syntax.Node {
	kids := []*syntax.Node{p.expect(token.LBRACK)}
	p.exprLev++
	var pending []*syntax.Node
	for !p.is(token.RBRACK) {
		name := field("name", p.ident("identifier"))
		if p.is(token.COMMA) {
			pending = append(pending, name, p.anon())
			continue
		}
		decl := append(pending, name, field("type", p.typeElem("type_constraint")))
		pending = nil
		kids = append(kids, node("type_parameter_declaration", decl...))
		if !p.is(token.COMMA) {
			break
		}
		kids = append(kids, p.anon())
	}
	p.exprLev--
	if len(pending) > 0 {
		p.fail("type parameter names without a constraint")
	}
	kids = append(kids, p.expect(token.RBRACK))
	return node("type_parameter_list", kids...)
}

func (p *parser) fieldDeclarationList() *syntax.Node {
	kids := []*syntax.Node{p.expect(token.LBRACE)}
	for !p.is(token.RBRACE) {
		kids = append(kids, p.fieldDeclaration())
		t := p.terminator(false, false)
		if t == nil {
			if !p.is(token.RBRACE) {
				p.fail("expected terminator in struct")
			}
			break
		}
		kids = append(kids, t)
	}
	kids = append(kids, p.anon())
	return node("field_declaration_list", kids...)
}

func (p *parser) fieldDeclaration() *syntax.Node {
	var kids []*syntax.Node
	switch {
	case p.is(token.MUL):
		kids = append(kids, p.anon(), field("type", p.parseType()))
	case p.is(token.IDENT):
		switch p.peek(1).t {
		case token.PERIOD, token.SEMICOLON, token.RBRACE, token.STRING:
			kids = append(kids, field("type", p.parseType()))
		case token.LBRACK:
			if p.arrayAfterName(p.pos + 1) {
				kids = append(kids, field("name", p.named("field_identifier")), field("type", p.parseType()))
			} else {
				kids = append(kids, field("type", p.parseType()))
			}
		default:
			kids = append(kids, field("name", p.named("field_identifier")))
			for p.is(token.COMMA) {
				kids = append(kids, p.anon(), field("name", p.ident("field_identifier")))
			}
			kids = append(kids, field("type", p.parseType()))
		}
	default:
		kids = append(kids, field("type", p.parseType()))
	}
	if p.is(token.STRING) {
		kids = append(kids, field("tag", p.stringLit()))
	}
	return node("field_declaration", kids...)
}

func (p *parser) interfaceType() *syntax.Node {
	kids := []*syntax.Node{p.anon(), p.expect(token.LBRACE)}
	for !p.is(token.RBRACE) {
		if p.is(token.IDENT) && p.peek(1).t == token.LPAREN {
			name := field("name", p.named("field_identifier"))
			params := field("parameters", p.parameterList())
			kids = append(kids, node("method_elem", name, params, field("result", p.result())))
		} else {
			kids = append(kids, p.typeElem("type_elem"))
		}
		t := p.terminator(false, false)
		if t == nil {
			if !p.is(token.RBRACE) {
				p.fail("expected terminator in interface")
			}
			break
		}
		kids = append(kids, t)
	}
	kids = append(kids, p.anon())
	return node("interface_type", kids...)
}

// ---- expressions reinterpreted as types ----

// typeable reports whether expression n can be read as a type, the way
// tree-sitter-go's GLR alternatives allow (identifiers, pkg.Name, generic
// instantiations, pointers, parenthesized types, literal types).
func (p *parser) typeable(n *syntax.Node) bool {
	switch n.Type {
	case "identifier":
		return true
	case "selector_expression":
		return n.Children[0].Type == "identifier"
	case "index_expression":
		return genericBase(n.Children[0]) && p.typeable(n.Children[2])
	case "unary_expression":
		return n.Children[0].Type == "*" && p.typeable(n.Children[1])
	case "parenthesized_expression":
		return p.typeable(n.Children[1])
	case "type_instantiation_expression":
		inst := p.insts[n]
		return inst != nil && genericBase(inst.base)
	}
	return isPureType(n)
}

// Dynamic precedences of tree-sitter-go, as fitted against the oracle: a
// type_identifier in a _type position -1, generic_type +1, type_arguments
// +1, type_instantiation_expression -1, everything else 0. GLR keeps the
// reading with the highest sum; ties keep the expression reading.
const (
	dynTypeIdent = -1
	dynGeneric   = 1
	dynTypeArgs  = 1
	dynInst      = -1
)

// escore is the dynamic precedence of n read as an expression: the sum of
// its type_instantiation_expression readings.
func (p *parser) escore(n *syntax.Node) int {
	switch n.Type {
	case "type_instantiation_expression":
		score := dynInst + p.tscore(p.insts[n].base)
		for _, e := range p.insts[n].elems {
			if e.Type != "," {
				score += p.tscore(e)
			}
		}
		return score
	case "unary_expression", "parenthesized_expression", "index_expression", "selector_expression":
		score := 0
		for _, c := range n.Children {
			score += p.escore(c)
		}
		return score
	}
	return 0
}

// tscore is the dynamic precedence of reading typeable n as a _type.
func (p *parser) tscore(n *syntax.Node) int {
	switch n.Type {
	case "identifier", "type_identifier":
		return dynTypeIdent
	case "selector_expression", "qualified_type":
		return 0
	case "index_expression":
		return dynGeneric + dynTypeArgs + p.tscore(n.Children[2])
	case "type_instantiation_expression":
		score := dynGeneric + dynTypeArgs
		for _, e := range p.insts[n].elems {
			if e.Type != "," {
				score += p.tscore(e)
			}
		}
		return score
	case "unary_expression", "pointer_type", "negated_type":
		return p.tscore(n.Children[1])
	case "parenthesized_expression", "parenthesized_type":
		return p.tscore(n.Children[1])
	case "generic_type":
		score := dynGeneric + dynTypeArgs
		for _, c := range n.Children[1].Children {
			if c.Type == "type_elem" {
				for _, t := range c.Children {
					if t.Type != "|" {
						score += p.tscore(t)
					}
				}
			}
		}
		return score
	case "slice_type", "array_type", "implicit_length_array_type", "channel_type":
		return p.tscore(n.Children[len(n.Children)-1])
	case "map_type":
		return p.tscore(n.Children[2]) + p.tscore(n.Children[4])
	}
	return 0
}

func genericBase(n *syntax.Node) bool {
	return n.Type == "identifier" || n.Type == "selector_expression" && n.Children[0].Type == "identifier"
}

// toType converts a typeable expression into fresh type nodes (the input is
// left untouched, so the caller may still use it as an expression).
func (p *parser) toType(n *syntax.Node) *syntax.Node {
	switch n.Type {
	case "identifier":
		return syntax.Leaf("type_identifier", true, n.StartByte, n.EndByte)
	case "selector_expression":
		pkg := n.Children[0]
		name := n.Children[2]
		return node("qualified_type",
			field("package", syntax.Leaf("package_identifier", true, pkg.StartByte, pkg.EndByte)),
			p.clone(n.Children[1]),
			field("name", syntax.Leaf("type_identifier", true, name.StartByte, name.EndByte)))
	case "index_expression":
		args := node("type_arguments", p.clone(n.Children[1]), node("type_elem", p.toType(n.Children[2])), p.clone(n.Children[3]))
		return node("generic_type", field("type", p.toType(n.Children[0])), field("type_arguments", args))
	case "unary_expression":
		return node("pointer_type", p.bare(n.Children[0]), p.toType(n.Children[1]))
	case "parenthesized_expression":
		return node("parenthesized_type", p.clone(n.Children[0]), p.toType(n.Children[1]), p.clone(n.Children[2]))
	case "type_instantiation_expression":
		return p.instGeneric(p.insts[n])
	}
	c := p.clone(n)
	c.Field = ""
	return c
}

// bare is a clone without the field name.
func (p *parser) bare(n *syntax.Node) *syntax.Node {
	c := p.clone(n)
	c.Field = ""
	return c
}

// clone deep-copies n, keeping hidden tokens hidden.
func (p *parser) clone(n *syntax.Node) *syntax.Node {
	c := *n
	c.Children = make([]*syntax.Node, len(n.Children))
	for i, ch := range n.Children {
		c.Children[i] = p.clone(ch)
	}
	if p.hidden[n] {
		p.hidden[&c] = true
	}
	return &c
}

// instantiation is x[A, B] (or x[[]T]): resolved by what follows it.
type instantiation struct {
	base   *syntax.Node
	lb, rb *syntax.Node
	elems  []*syntax.Node // types and commas, as parsed (expressions)
}

func (p *parser) instNode(in *instantiation) *syntax.Node {
	var base *syntax.Node
	if p.typeable(in.base) {
		base = p.toType(in.base)
	} else {
		base = p.clone(in.base)
	}
	kids := []*syntax.Node{field("type", base), p.clone(in.lb)}
	for _, e := range in.elems {
		if e.Type == "," {
			kids = append(kids, p.clone(e))
			continue
		}
		kids = append(kids, p.elemType(e))
	}
	kids = append(kids, p.clone(in.rb))
	return node("type_instantiation_expression", kids...)
}

func (p *parser) elemType(e *syntax.Node) *syntax.Node {
	if p.typeable(e) {
		return p.toType(e)
	}
	c := p.clone(e)
	c.Field = ""
	return c
}

func (p *parser) instTypeArguments(in *instantiation) *syntax.Node {
	kids := []*syntax.Node{p.clone(in.lb)}
	for _, e := range in.elems {
		if e.Type == "," {
			kids = append(kids, p.clone(e))
			continue
		}
		kids = append(kids, node("type_elem", p.elemType(e)))
	}
	kids = append(kids, p.clone(in.rb))
	return node("type_arguments", kids...)
}

func (p *parser) instGeneric(in *instantiation) *syntax.Node {
	return node("generic_type", field("type", p.toType(in.base)), field("type_arguments", p.instTypeArguments(in)))
}
