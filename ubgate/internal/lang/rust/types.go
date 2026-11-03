package rust

import "github.com/matheusAle/ub-mode/ubgate/internal/syntax"

var primitiveTypes = map[string]bool{
	"u8": true, "i8": true, "u16": true, "i16": true, "u32": true, "i32": true, "u64": true, "i64": true,
	"u128": true, "i128": true, "isize": true, "usize": true, "f32": true, "f64": true,
	"bool": true, "str": true, "char": true,
}

// typ parses a _type, including `A + B` (bounded_type, left-associative,
// binding loosest).
func (p *parser) typ() *syntax.Node {
	t := p.boundOrType()
	for p.at("+") {
		plus := p.expect("+")
		t = node("bounded_type", t, plus, p.boundOrType())
	}
	return t
}

func (p *parser) boundOrType() *syntax.Node {
	if p.at("'") {
		return p.lifetime()
	}
	if p.at("use") && p.peek2().is("<") {
		return p.useBounds()
	}
	return p.typeNoBounds()
}

func (p *parser) useBounds() *syntax.Node {
	parts := []*syntax.Node{p.expect("use"), p.expect("<")}
	for !p.atSplit(">") {
		if p.at("'") {
			parts = append(parts, p.lifetime())
		} else {
			parts = append(parts, p.identAs("type_identifier"))
		}
		if !p.atSplit(">") {
			parts = append(parts, p.expect(","))
		}
	}
	parts = append(parts, p.expect(">"))
	return node("use_bounds", parts...)
}

// typeNoBounds parses a _type that does not extend over `+`.
func (p *parser) typeNoBounds() *syntax.Node {
	t := p.peek()
	switch {
	case p.atSplit("&"):
		parts := []*syntax.Node{p.expect("&")}
		if p.at("'") {
			parts = append(parts, p.lifetime())
		}
		if p.at("mut") {
			parts = append(parts, p.mutableSpecifier())
		}
		parts = append(parts, field("type", p.typeNoBounds()))
		return node("reference_type", parts...)
	case t.is("*"):
		parts := []*syntax.Node{p.expect("*")}
		if p.at("const") {
			parts = append(parts, p.expect("const"))
		} else {
			parts = append(parts, p.mutableSpecifier())
		}
		parts = append(parts, field("type", p.typeNoBounds()))
		return node("pointer_type", parts...)
	case t.is("("):
		open := p.expect("(")
		if p.at(")") {
			return node("unit_type", open, p.expect(")"))
		}
		parts := []*syntax.Node{open}
		for !p.at(")") {
			parts = append(parts, p.typ())
			if !p.at(")") {
				parts = append(parts, p.expect(","))
			}
		}
		parts = append(parts, p.expect(")"))
		return node("tuple_type", parts...)
	case t.is("["):
		parts := []*syntax.Node{p.expect("["), field("element", p.typ())}
		if p.at(";") {
			parts = append(parts, p.expect(";"), field("length", p.expression(exprCtx{})))
		}
		parts = append(parts, p.expect("]"))
		return node("array_type", parts...)
	case t.is("!"):
		return node("never_type", p.expect("!"))
	case t.is("impl"):
		parts := []*syntax.Node{p.expect("impl")}
		if p.at("for") {
			parts = append(parts, p.expect("for"), p.typeParameters())
		}
		parts = append(parts, field("trait", p.traitType()))
		return node("abstract_type", parts...)
	case t.is("dyn"):
		kw := p.expect("dyn")
		var tr *syntax.Node
		if p.at("for") {
			tr = p.higherRankedTraitBound()
		} else {
			tr = p.typeNoBounds()
		}
		return node("dynamic_type", kw, field("trait", tr))
	case t.is("?"):
		return node("removed_trait_bound", p.expect("?"), p.typeNoBounds())
	case t.is("fn"), t.is("unsafe"), t.is("extern"), t.is("for"), t.is("const") && p.peek2().is("fn"), t.is("async") && p.peek2().is("fn"):
		return p.functionType(nil, nil)
	case t.is("<"), t.is("<<"):
		return p.typePath()
	case t.is("$"):
		return p.metavariable()
	case t.kind == tkIdent:
		if p.isMacroStart() {
			return p.macroInvocation(p.simplePath())
		}
		if primitiveTypes[t.text] && !p.peek2().is("::") {
			p.next()
			return syntax.Leaf("primitive_type", true, t.start, t.end)
		}
		return p.typePath()
	case t.is("::"):
		return p.typePath()
	}
	fail(t.start, "expected type, got %q", t.text)
	return nil
}

// traitType is the trait of an abstract_type.
func (p *parser) traitType() *syntax.Node {
	if p.at("'") {
		// `impl 'a + T`: only a bounded_type can start with a lifetime.
		lt := p.lifetime()
		plus := p.expect("+")
		return node("bounded_type", lt, plus, p.boundOrType())
	}
	if p.at("?") {
		return node("removed_trait_bound", p.expect("?"), p.typeNoBounds())
	}
	return p.typeNoBounds()
}

func (p *parser) functionType(forLifetimes, trait *syntax.Node) *syntax.Node {
	parts := []*syntax.Node{forLifetimes}
	if forLifetimes == nil && p.at("for") {
		parts[0] = p.forLifetimes()
		if !p.at("fn") && !p.at("unsafe") && !p.at("extern") && !p.at("const") && !p.at("async") {
			path := p.typePathBase()
			return p.functionType(parts[0], path)
		}
	}
	if trait != nil {
		parts = append(parts, field("trait", trait))
	} else {
		parts = append(parts, p.functionModifiers(), p.expect("fn"))
	}
	parts = append(parts, field("parameters", p.parameters()))
	if p.at("->") {
		parts = append(parts, p.expect("->"), field("return_type", p.typeNoBounds()))
	}
	return node("function_type", parts...)
}

func (p *parser) forLifetimes() *syntax.Node {
	parts := []*syntax.Node{p.expect("for"), p.expect("<")}
	for !p.atSplit(">") {
		parts = append(parts, p.lifetime())
		if !p.atSplit(">") {
			parts = append(parts, p.expect(","))
		}
	}
	parts = append(parts, p.expect(">"))
	return node("for_lifetimes", parts...)
}

func (p *parser) higherRankedTraitBound() *syntax.Node {
	kw := p.expect("for")
	tp := p.typeParameters()
	return node("higher_ranked_trait_bound", kw, field("type_parameters", tp), field("type", p.typeNoBounds()))
}

// typePath parses a path type: type_identifier, scoped_type_identifier,
// generic_type, or function_type for `Fn(A) -> B` sugar.
func (p *parser) typePath() *syntax.Node {
	n := p.typePathBase()
	if p.atSplit("<") && !p.at("<=") && !p.at("<<=") {
		n = node("generic_type", field("type", n), field("type_arguments", p.typeArguments()))
	} else if p.atTurbofish() {
		// Turbofish in type position: generic_type with `::`.
		sep := p.expect("::")
		n = node("generic_type", field("type", n), sep, field("type_arguments", p.typeArguments()))
	}
	if p.at("(") && (n.Type == "type_identifier" || n.Type == "scoped_type_identifier") {
		return p.functionType(nil, n)
	}
	return n
}

// typePathBase parses the path up to (excluding) trailing type arguments.
func (p *parser) typePathBase() *syntax.Node {
	var prefix *syntax.Node // a _path / bracketed_type / generic_type before `::`
	if p.atSplit("<") {
		prefix = p.bracketedType()
		if !p.at("::") {
			fail(p.peek().start, "expected :: after qualified type")
		}
	}
	var lead *syntax.Node
	if prefix == nil && p.at("::") {
		lead = p.expect("::")
	}
	if prefix == nil && lead == nil {
		seg := p.peek()
		if !p.peek2().is("::") || isLess(p.peekN(2)) {
			if seg.is("self") || seg.is("super") || seg.is("crate") {
				fail(seg.start, "path keyword as type")
			}
			return p.identAs("type_identifier")
		}
		prefix = p.pathSegment()
	}
	// Here: either lead (`::` consumed, no prefix) or prefix followed by `::`.
	for {
		var sep *syntax.Node
		if lead != nil {
			sep, lead = lead, nil
		} else {
			sep = p.expect("::")
		}
		seg := p.peek()
		lastSeg := !p.peek2().is("::") || isLess(p.peekN(2))
		if seg.is("<") {
			fail(seg.start, "turbofish in type path")
		}
		if lastSeg {
			name := p.identAs("type_identifier")
			n := node("scoped_type_identifier", field("path", prefix), sep, field("name", name))
			if prefix == nil {
				n = node("scoped_type_identifier", sep, field("name", name))
			}
			if p.atSplit("<") && !p.at("<=") && p.peekPastTypeArgsIs("::") {
				g := node("generic_type", field("type", n), field("type_arguments", p.typeArguments()))
				sep2 := p.expect("::")
				name2 := p.identAs("type_identifier")
				return node("scoped_type_identifier", field("path", g), sep2, field("name", name2))
			}
			return n
		}
		segNode := p.pathSegment()
		if prefix == nil {
			prefix = node("scoped_identifier", sep, field("name", segNode))
		} else {
			prefix = node("scoped_identifier", field("path", prefix), sep, field("name", segNode))
		}
	}
}

// peekPastTypeArgsIs reports whether the balanced <...> at the current
// position is followed by tok (a linear token scan, no parsing).
func (p *parser) peekPastTypeArgsIs(tok string) bool {
	depth := 0
	t := p.peek()
	for {
		switch {
		case t.kind == tkEOF, t.is(";"), t.is("{"), t.is("}"):
			return false
		case t.is("<"):
			depth++
		case t.is("<<"):
			depth += 2
		case t.is(">"), t.is(">="):
			depth--
		case t.is(">>"), t.is(">>="):
			depth -= 2
		}
		if depth < 0 {
			return tok == ">" // the closing `>` is split off `>>`
		}
		if depth == 0 {
			if t.is(">=") || t.is(">>=") {
				return tok == "=" // ... or off `>=`
			}
			return p.lx.at(t.end).is(tok)
		}
		t = p.lx.at(t.end)
	}
}

func (p *parser) bracketedType() *syntax.Node {
	open := p.expect("<")
	t := p.typ()
	if p.at("as") {
		as := p.expect("as")
		t = node("qualified_type", field("type", t), as, field("alias", p.typ()))
	}
	return node("bracketed_type", open, t, p.expect(">"))
}

func (p *parser) typeArguments() *syntax.Node {
	parts := []*syntax.Node{p.expect("<")}
	for !p.atSplit(">") {
		parts = append(parts, p.typeArgument())
		if p.at(":") {
			parts = append(parts, p.traitBounds())
		}
		if !p.atSplit(">") {
			parts = append(parts, p.expect(","))
		}
	}
	parts = append(parts, p.expect(">"))
	return node("type_arguments", parts...)
}

func (p *parser) typeArgument() *syntax.Node {
	t := p.peek()
	switch {
	case t.is("'"):
		return p.lifetime()
	case t.is("{"):
		return p.block()
	case t.kind == tkInt, t.kind == tkFloat, t.kind == tkChar, t.kind == tkString, t.kind == tkRawString, t.is("true"), t.is("false"):
		return p.literal()
	case t.is("-") && (p.peek2().kind == tkInt || p.peek2().kind == tkFloat):
		minus := p.expect("-")
		return node("negative_literal", minus, p.literal())
	case t.kind == tkIdent && !primitiveTypes[t.text]:
		n2 := p.peek2()
		if n2.is("=") {
			name := p.identAs("type_identifier")
			eq := p.expect("=")
			return node("type_binding", field("name", name), eq, field("type", p.typ()))
		}
		if n2.is("<") {
			save := p.pos
			name := p.identAs("type_identifier")
			if p.peekPastTypeArgsIs("=") {
				args := p.typeArguments()
				eq := p.expect("=")
				return node("type_binding", field("name", name), field("type_arguments", args), eq, field("type", p.typ()))
			}
			p.pos = save
		}
	}
	return p.typ()
}

func (p *parser) traitBounds() *syntax.Node {
	parts := []*syntax.Node{p.expect(":")}
	for {
		parts = append(parts, p.bound())
		if !p.at("+") {
			break
		}
		parts = append(parts, p.expect("+"))
		if !p.atBoundStart() {
			break
		}
	}
	return node("trait_bounds", parts...)
}

func (p *parser) atBoundStart() bool {
	t := p.peek()
	return t.kind == tkIdent || t.is("'") || t.is("?") || t.is("(") || t.is("::") || t.is("<") || t.is("&") || t.is("[") || t.is("*")
}

func (p *parser) bound() *syntax.Node {
	switch {
	case p.at("'"):
		return p.lifetime()
	case p.at("?"):
		return node("removed_trait_bound", p.expect("?"), p.typeNoBounds())
	case p.at("for"):
		return p.higherRankedTraitBound()
	case p.at("use") && p.peek2().is("<"):
		return p.useBounds()
	}
	return p.typeNoBounds()
}

func (p *parser) typeParameters() *syntax.Node {
	parts := []*syntax.Node{p.expect("<")}
	for !p.atSplit(">") {
		parts = append(parts, p.attributes()...)
		parts = append(parts, p.typeParameter())
		if !p.atSplit(">") {
			parts = append(parts, p.expect(","))
		}
	}
	parts = append(parts, p.expect(">"))
	return node("type_parameters", parts...)
}

func (p *parser) typeParameter() *syntax.Node {
	switch {
	case p.at("$"):
		return p.metavariable()
	case p.at("'"):
		parts := []*syntax.Node{field("name", p.lifetime())}
		if p.at(":") {
			parts = append(parts, field("bounds", p.traitBounds()))
		}
		return node("lifetime_parameter", parts...)
	case p.at("const"):
		parts := []*syntax.Node{p.expect("const"), field("name", p.ident()), p.expect(":"), field("type", p.typ())}
		if p.at("=") {
			parts = append(parts, p.expect("="), field("value", p.constParamValue()))
		}
		return node("const_parameter", parts...)
	}
	parts := []*syntax.Node{field("name", p.identAs("type_identifier"))}
	if p.at(":") {
		parts = append(parts, field("bounds", p.traitBounds()))
	}
	if p.at("=") {
		parts = append(parts, p.expect("="), field("default_type", p.typ()))
	}
	return node("type_parameter", parts...)
}

func (p *parser) constParamValue() *syntax.Node {
	t := p.peek()
	switch {
	case t.is("{"):
		return p.block()
	case t.is("-"):
		minus := p.expect("-")
		return node("negative_literal", minus, p.literal())
	case t.kind == tkIdent && !t.is("true") && !t.is("false"):
		return p.ident()
	}
	return p.literal()
}
