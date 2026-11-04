package rust

import "github.com/matheusAle/coderefinery/internal/syntax"

// Items: the _declaration_statement choice of tree-sitter-rust.

// declaration parses a declaration statement, or returns nil (position
// unchanged) when the next tokens start an expression.
func (p *parser) declaration() *syntax.Node {
	t := p.peek()
	switch {
	case t.is(";"):
		return node("empty_statement", p.expect(";"))
	case t.is("#"):
		return p.attributeItem()
	case t.is("let"):
		return p.letDeclaration()
	case t.is("macro_rules") && p.peek2().is("!"):
		return p.macroDefinition()
	}
	if !p.atItemStart() {
		return nil
	}
	return p.item()
}

// atItemStart looks past a visibility and qualifiers for an item keyword.
func (p *parser) atItemStart() bool {
	i := 0
	t := p.peekN(i)
	if t.is("pub") {
		i++
		if p.peekN(i).is("(") && p.visibilityParen(i) {
			for !p.peekN(i).is(")") {
				i++
			}
			i++
		}
		t = p.peekN(i)
		return t.kind == tkIdent // after `pub` anything is an item
	}
	if t.kind != tkIdent {
		return false
	}
	switch t.text {
	case "fn", "struct", "enum", "trait", "impl", "mod", "use", "type":
		return true
	case "union":
		n := p.peekN(i + 1)
		return n.kind == tkIdent
	case "extern":
		return true
	case "static":
		n := p.peekN(i + 1)
		return n.kind == tkIdent && !n.is("move") && !n.is("async")
	case "const":
		n := p.peekN(i + 1)
		if n.is("{") {
			return false
		}
		return n.kind == tkIdent
	case "unsafe":
		n := p.peekN(i + 1)
		return n.kind == tkIdent
	case "async":
		n := p.peekN(i + 1)
		return n.is("fn") || n.is("unsafe") || (n.is("const") && p.peekN(i+2).kind == tkIdent) || n.is("extern")
	case "default":
		n := p.peekN(i + 1)
		return n.is("fn") || n.is("unsafe") || n.is("async") || n.is("const") || n.is("extern")
	}
	return false
}

// visibilityParen: `pub(` crate|self|super|in ... `)`.
func (p *parser) visibilityParen(i int) bool {
	a := p.peekN(i + 1)
	if a.is("in") {
		return true
	}
	return (a.is("crate") || a.is("self") || a.is("super")) && p.peekN(i+2).is(")")
}

func (p *parser) visibility() *syntax.Node {
	if p.at("crate") && !p.peek2().is("::") {
		return node("visibility_modifier", p.kwNode("crate"))
	}
	if !p.at("pub") {
		return nil
	}
	parts := []*syntax.Node{p.expect("pub")}
	if p.at("(") && p.visibilityParen(0) {
		parts = append(parts, p.expect("("))
		if p.at("in") {
			parts = append(parts, p.expect("in"), p.path())
		} else {
			parts = append(parts, p.kwNode(p.peek().text))
		}
		parts = append(parts, p.expect(")"))
	}
	return node("visibility_modifier", parts...)
}

// kwNode consumes self/super/crate as their named nodes.
func (p *parser) kwNode(kw string) *syntax.Node {
	t := p.next()
	if t.text != kw {
		fail(t.start, "expected %s", kw)
	}
	return syntax.Leaf(kw, true, t.start, t.end)
}

func (p *parser) item() *syntax.Node {
	vis := p.visibility()
	t := p.peek()
	switch t.text {
	case "struct":
		return p.structItem(vis)
	case "union":
		return p.unionItem(vis)
	case "enum":
		return p.enumItem(vis)
	case "trait":
		return p.traitItem(vis, nil)
	case "impl":
		return p.implItem(nil)
	case "mod":
		return p.modItem(vis)
	case "use":
		return p.useDeclaration(vis)
	case "type":
		return p.typeItem(vis)
	case "static":
		return p.staticItem(vis)
	case "const":
		if n := p.peek2(); !n.is("fn") && !n.is("unsafe") && !n.is("async") && !n.is("extern") {
			return p.constItem(vis)
		}
	case "extern":
		n := p.peek2()
		if n.is("crate") {
			return p.externCrate(vis)
		}
		if n.is("{") || (n.kind == tkString || n.kind == tkRawString) && p.peekN(2).is("{") {
			return p.foreignMod(vis, nil)
		}
	case "unsafe":
		n := p.peek2()
		if n.is("impl") {
			return p.implItem(p.expect("unsafe"))
		}
		if n.is("trait") {
			return p.traitItem(vis, p.expect("unsafe"))
		}
		if n.is("extern") && (p.peekN(2).is("{") || p.peekN(2).kind == tkString && p.peekN(3).is("{")) {
			unsafe := p.expect("unsafe")
			return p.foreignMod(vis, unsafe)
		}
	}
	return p.functionItem(vis)
}

func (p *parser) functionModifiers() *syntax.Node {
	var mods []*syntax.Node
	for {
		t := p.peek()
		switch {
		case t.is("async"), t.is("default"), t.is("const"), t.is("unsafe"):
			mods = append(mods, p.expect(t.text))
		case t.is("extern"):
			mods = append(mods, p.externModifier())
		default:
			if len(mods) == 0 {
				return nil
			}
			return node("function_modifiers", mods...)
		}
	}
}

func (p *parser) externModifier() *syntax.Node {
	kw := p.expect("extern")
	if t := p.peek(); t.kind == tkString {
		p.next()
		return node("extern_modifier", kw, t.node)
	}
	return node("extern_modifier", kw)
}

func (p *parser) functionItem(vis *syntax.Node) *syntax.Node {
	mods := p.functionModifiers()
	kw := p.expect("fn")
	var name *syntax.Node
	if p.at("$") {
		name = p.metavariable()
	} else {
		name = p.ident()
	}
	parts := []*syntax.Node{vis, mods, kw, field("name", name)}
	if p.at("<") {
		parts = append(parts, field("type_parameters", p.typeParameters()))
	}
	parts = append(parts, field("parameters", p.parameters()))
	if p.at("->") {
		parts = append(parts, p.expect("->"), field("return_type", p.typ()))
	}
	if p.at("where") {
		parts = append(parts, p.whereClause())
	}
	if p.at(";") {
		parts = append(parts, p.expect(";"))
		return node("function_signature_item", parts...)
	}
	parts = append(parts, field("body", p.block()))
	return node("function_item", parts...)
}

func (p *parser) metavariable() *syntax.Node {
	d := p.expect("$")
	id := p.ident()
	return syntax.Leaf("metavariable", true, d.StartByte, id.EndByte)
}

func (p *parser) parameters() *syntax.Node {
	parts := []*syntax.Node{p.expect("(")}
	for !p.at(")") {
		if p.at("#") {
			parts = append(parts, p.attributeItem())
		}
		parts = append(parts, p.parameter())
		if !p.at(")") {
			parts = append(parts, p.expect(","))
		}
	}
	parts = append(parts, p.expect(")"))
	return node("parameters", parts...)
}

func (p *parser) parameter() *syntax.Node {
	// self_parameter: [&] [lifetime] [mut] self
	save := p.pos
	var sp []*syntax.Node
	if p.atSplit("&") {
		sp = append(sp, p.expect("&"))
		if p.at("'") {
			sp = append(sp, p.lifetime())
		}
	}
	if p.at("mut") {
		sp = append(sp, p.mutableSpecifier())
	}
	if p.at("self") && !p.peek2().is(":") && !p.peek2().is("::") {
		sp = append(sp, p.kwNode("self"))
		return node("self_parameter", sp...)
	}
	p.pos = save
	if p.at("...") {
		return node("variadic_parameter", p.expect("..."))
	}
	var result *syntax.Node
	if p.attempt(func() {
		var mut *syntax.Node
		if p.at("mut") {
			mut = p.mutableSpecifier()
		}
		var pat *syntax.Node
		if p.at("self") {
			pat = p.kwNode("self")
		} else {
			pat = p.pattern()
		}
		colon := p.expect(":")
		if p.at("...") {
			result = node("variadic_parameter", mut, field("pattern", pat), colon, p.expect("..."))
			return
		}
		result = node("parameter", mut, field("pattern", pat), colon, field("type", p.typ()))
	}) {
		return result
	}
	if p.at("_") && (p.peek2().is(",") || p.peek2().is(")")) {
		return p.expect("_")
	}
	return p.typ()
}

func (p *parser) mutableSpecifier() *syntax.Node {
	t := p.next()
	return syntax.Leaf("mutable_specifier", true, t.start, t.end)
}

func (p *parser) whereClause() *syntax.Node {
	parts := []*syntax.Node{p.expect("where")}
	for p.atWherePredicateStart() {
		parts = append(parts, p.wherePredicate())
		if !p.at(",") {
			break
		}
		parts = append(parts, p.expect(","))
	}
	return node("where_clause", parts...)
}

func (p *parser) atWherePredicateStart() bool {
	t := p.peek()
	if t.is("{") || t.is(";") || t.is("=") || t.kind == tkEOF {
		return false
	}
	return true
}

func (p *parser) wherePredicate() *syntax.Node {
	var left *syntax.Node
	switch {
	case p.at("'"):
		left = p.lifetime()
	case p.at("for"):
		left = p.higherRankedTraitBound()
	default:
		left = p.typeNoBounds()
	}
	return node("where_predicate", field("left", left), field("bounds", p.traitBounds()))
}

func (p *parser) structItem(vis *syntax.Node) *syntax.Node {
	parts := []*syntax.Node{vis, p.expect("struct"), field("name", p.identAs("type_identifier"))}
	if p.at("<") {
		parts = append(parts, field("type_parameters", p.typeParameters()))
	}
	switch {
	case p.at(";"):
		parts = append(parts, p.expect(";"))
	case p.at("("):
		parts = append(parts, field("body", p.orderedFieldDeclarationList()))
		if p.at("where") {
			parts = append(parts, p.whereClause())
		}
		parts = append(parts, p.expect(";"))
	default:
		if p.at("where") {
			parts = append(parts, p.whereClause())
		}
		if p.at(";") {
			parts = append(parts, p.expect(";"))
		} else {
			parts = append(parts, field("body", p.fieldDeclarationList()))
		}
	}
	return node("struct_item", parts...)
}

func (p *parser) unionItem(vis *syntax.Node) *syntax.Node {
	parts := []*syntax.Node{vis, p.expect("union"), field("name", p.identAs("type_identifier"))}
	if p.at("<") {
		parts = append(parts, field("type_parameters", p.typeParameters()))
	}
	if p.at("where") {
		parts = append(parts, p.whereClause())
	}
	parts = append(parts, field("body", p.fieldDeclarationList()))
	return node("union_item", parts...)
}

func (p *parser) attributes() []*syntax.Node {
	var out []*syntax.Node
	for p.at("#") {
		out = append(out, p.attributeItem())
	}
	return out
}

func (p *parser) fieldDeclarationList() *syntax.Node {
	parts := []*syntax.Node{p.expect("{")}
	for !p.at("}") {
		parts = append(parts, p.attributes()...)
		var vis *syntax.Node
		vis = p.visibility()
		name := p.identAs("field_identifier")
		colon := p.expect(":")
		parts = append(parts, node("field_declaration", vis, field("name", name), colon, field("type", p.typ())))
		if !p.at("}") {
			parts = append(parts, p.expect(","))
		}
	}
	parts = append(parts, p.expect("}"))
	return node("field_declaration_list", parts...)
}

func (p *parser) orderedFieldDeclarationList() *syntax.Node {
	parts := []*syntax.Node{p.expect("(")}
	for !p.at(")") {
		parts = append(parts, p.attributes()...)
		if v := p.visibility(); v != nil {
			parts = append(parts, v)
		}
		parts = append(parts, field("type", p.typ()))
		if !p.at(")") {
			parts = append(parts, p.expect(","))
		}
	}
	parts = append(parts, p.expect(")"))
	return node("ordered_field_declaration_list", parts...)
}

func (p *parser) enumItem(vis *syntax.Node) *syntax.Node {
	parts := []*syntax.Node{vis, p.expect("enum"), field("name", p.identAs("type_identifier"))}
	if p.at("<") {
		parts = append(parts, field("type_parameters", p.typeParameters()))
	}
	if p.at("where") {
		parts = append(parts, p.whereClause())
	}
	list := []*syntax.Node{p.expect("{")}
	for !p.at("}") {
		list = append(list, p.attributes()...)
		v := []*syntax.Node{p.visibility(), field("name", p.ident())}
		switch {
		case p.at("{"):
			v = append(v, field("body", p.fieldDeclarationList()))
		case p.at("("):
			v = append(v, field("body", p.orderedFieldDeclarationList()))
		}
		if p.at("=") {
			v = append(v, p.expect("="), field("value", p.expression(exprCtx{})))
		}
		list = append(list, node("enum_variant", v...))
		if !p.at("}") {
			list = append(list, p.expect(","))
		}
	}
	list = append(list, p.expect("}"))
	parts = append(parts, field("body", node("enum_variant_list", list...)))
	return node("enum_item", parts...)
}

func (p *parser) traitItem(vis, unsafe *syntax.Node) *syntax.Node {
	if unsafe == nil && p.at("unsafe") {
		unsafe = p.expect("unsafe")
	}
	parts := []*syntax.Node{vis, unsafe, p.expect("trait"), field("name", p.identAs("type_identifier"))}
	if p.at("<") {
		parts = append(parts, field("type_parameters", p.typeParameters()))
	}
	if p.at(":") {
		parts = append(parts, field("bounds", p.traitBounds()))
	}
	if p.at("where") {
		parts = append(parts, p.whereClause())
	}
	parts = append(parts, field("body", p.declarationList()))
	return node("trait_item", parts...)
}

func (p *parser) implItem(unsafe *syntax.Node) *syntax.Node {
	parts := []*syntax.Node{unsafe, p.expect("impl")}
	if p.at("<") {
		parts = append(parts, field("type_parameters", p.typeParameters()))
	}
	var neg *syntax.Node
	if p.at("!") && !p.peek2().is("{") && !p.peek2().is("where") && !p.peek2().is(";") {
		neg = p.expect("!")
	}
	first := p.typ()
	if p.at("for") {
		parts = append(parts, neg, field("trait", first), p.expect("for"), field("type", p.typ()))
	} else {
		if neg != nil {
			fail(neg.StartByte, "negative impl without for")
		}
		parts = append(parts, field("type", first))
	}
	if p.at("where") {
		parts = append(parts, p.whereClause())
	}
	if p.at(";") {
		parts = append(parts, p.expect(";"))
	} else {
		parts = append(parts, field("body", p.declarationList()))
	}
	return node("impl_item", parts...)
}

func (p *parser) declarationList() *syntax.Node {
	parts := []*syntax.Node{p.expect("{")}
	for !p.at("}") {
		parts = append(parts, p.statement(true))
	}
	parts = append(parts, p.expect("}"))
	return node("declaration_list", parts...)
}

func (p *parser) modItem(vis *syntax.Node) *syntax.Node {
	parts := []*syntax.Node{vis, p.expect("mod"), field("name", p.ident())}
	if p.at(";") {
		parts = append(parts, p.expect(";"))
	} else {
		parts = append(parts, field("body", p.declarationList()))
	}
	return node("mod_item", parts...)
}

func (p *parser) foreignMod(vis, unsafe *syntax.Node) *syntax.Node {
	parts := []*syntax.Node{vis, unsafe, p.externModifier()}
	if p.at(";") {
		parts = append(parts, p.expect(";"))
	} else {
		parts = append(parts, field("body", p.declarationList()))
	}
	return node("foreign_mod_item", parts...)
}

func (p *parser) externCrate(vis *syntax.Node) *syntax.Node {
	parts := []*syntax.Node{vis, p.expect("extern"), p.kwNode("crate"), field("name", p.ident())}
	if p.at("as") {
		parts = append(parts, p.expect("as"), field("alias", p.ident()))
	}
	parts = append(parts, p.expect(";"))
	return node("extern_crate_declaration", parts...)
}

func (p *parser) constItem(vis *syntax.Node) *syntax.Node {
	parts := []*syntax.Node{vis, p.expect("const"), field("name", p.ident()), p.expect(":"), field("type", p.typ())}
	if p.at("=") {
		parts = append(parts, p.expect("="), field("value", p.expression(exprCtx{})))
	}
	parts = append(parts, p.expect(";"))
	return node("const_item", parts...)
}

func (p *parser) staticItem(vis *syntax.Node) *syntax.Node {
	parts := []*syntax.Node{vis, p.expect("static")}
	if p.at("ref") {
		parts = append(parts, p.expect("ref"))
	}
	if p.at("mut") {
		parts = append(parts, p.mutableSpecifier())
	}
	parts = append(parts, field("name", p.ident()), p.expect(":"), field("type", p.typ()))
	if p.at("=") {
		parts = append(parts, p.expect("="), field("value", p.expression(exprCtx{})))
	}
	parts = append(parts, p.expect(";"))
	return node("static_item", parts...)
}

func (p *parser) typeItem(vis *syntax.Node) *syntax.Node {
	kw := p.expect("type")
	name := field("name", p.identAs("type_identifier"))
	var tparams *syntax.Node
	if p.at("<") {
		tparams = field("type_parameters", p.typeParameters())
	}
	var bounds, where *syntax.Node
	if p.at(":") {
		bounds = field("bounds", p.traitBounds())
	}
	if p.at("where") {
		where = p.whereClause()
	}
	if p.at(";") {
		return node("associated_type", vis, kw, name, tparams, bounds, where, p.expect(";"))
	}
	parts := []*syntax.Node{vis, kw, name, tparams, where, p.expect("="), field("type", p.typ())}
	if p.at("where") {
		parts = append(parts, p.whereClause())
	}
	parts = append(parts, p.expect(";"))
	return node("type_item", parts...)
}

func (p *parser) letDeclaration() *syntax.Node {
	parts := []*syntax.Node{p.expect("let")}
	if p.at("mut") {
		parts = append(parts, p.mutableSpecifier())
	}
	parts = append(parts, field("pattern", p.pattern()))
	if p.at(":") {
		parts = append(parts, p.expect(":"), field("type", p.typ()))
	}
	if p.at("=") {
		parts = append(parts, p.expect("="), field("value", p.expression(exprCtx{})))
	}
	if p.at("else") {
		parts = append(parts, p.expect("else"), field("alternative", p.block()))
	}
	parts = append(parts, p.expect(";"))
	return node("let_declaration", parts...)
}

// ---------------------------------------------------------------- use

func (p *parser) useDeclaration(vis *syntax.Node) *syntax.Node {
	kw := p.expect("use")
	arg := p.useClause()
	return node("use_declaration", vis, kw, field("argument", arg), p.expect(";"))
}

func (p *parser) useClause() *syntax.Node {
	switch {
	case p.at("{"):
		return p.useList()
	case p.at("*"):
		return node("use_wildcard", p.expect("*"))
	case p.at("::"):
		sep := p.expect("::")
		switch {
		case p.at("{"):
			return node("scoped_use_list", sep, field("list", p.useList()))
		case p.at("*"):
			return node("use_wildcard", sep, p.expect("*"))
		}
		path := node("scoped_identifier", sep, field("name", p.pathSegment()))
		return p.useRest(path)
	}
	return p.useRest(p.pathSegment())
}

// useRest continues a use path after its first segment.
func (p *parser) useRest(path *syntax.Node) *syntax.Node {
	for p.at("::") {
		sep := p.expect("::")
		switch {
		case p.at("{"):
			return node("scoped_use_list", field("path", path), sep, field("list", p.useList()))
		case p.at("*"):
			return node("use_wildcard", path, sep, p.expect("*"))
		}
		path = node("scoped_identifier", field("path", path), sep, field("name", p.pathSegment()))
	}
	if p.at("as") {
		return node("use_as_clause", field("path", path), p.expect("as"), field("alias", p.ident()))
	}
	return path
}

func (p *parser) useList() *syntax.Node {
	parts := []*syntax.Node{p.expect("{")}
	for !p.at("}") {
		parts = append(parts, p.useClause())
		if !p.at("}") {
			parts = append(parts, p.expect(","))
		}
	}
	parts = append(parts, p.expect("}"))
	return node("use_list", parts...)
}

// pathSegment is one segment of a _path: identifier, self, super, crate,
// a metavariable, or a primitive type name (an identifier here).
func (p *parser) pathSegment() *syntax.Node {
	t := p.peek()
	switch {
	case t.is("self"), t.is("super"), t.is("crate"):
		return p.kwNode(t.text)
	case t.is("$"):
		return p.metavariable()
	}
	return p.ident()
}

// path parses a _path (used by attributes and visibility `in`).
func (p *parser) path() *syntax.Node {
	var n *syntax.Node
	if p.at("::") {
		sep := p.expect("::")
		n = node("scoped_identifier", sep, field("name", p.pathSegment()))
	} else {
		n = p.pathSegment()
	}
	for p.at("::") {
		sep := p.expect("::")
		n = node("scoped_identifier", field("path", n), sep, field("name", p.pathSegment()))
	}
	return n
}

// ---------------------------------------------------------------- attributes

func (p *parser) attributeItem() *syntax.Node {
	hash := p.expect("#")
	if p.at("!") {
		bang := p.expect("!")
		open := p.expect("[")
		attr := p.attribute()
		return node("inner_attribute_item", hash, bang, open, attr, p.expect("]"))
	}
	open := p.expect("[")
	attr := p.attribute()
	return node("attribute_item", hash, open, attr, p.expect("]"))
}

func (p *parser) attribute() *syntax.Node {
	parts := []*syntax.Node{p.path()}
	parts = append(parts, p.attributeTail()...)
	return node("attribute", parts...)
}

func (p *parser) attributeTail() []*syntax.Node {
	switch {
	case p.at("="):
		return []*syntax.Node{p.expect("="), field("value", p.expression(exprCtx{}))}
	case p.at("("), p.at("["), p.at("{"):
		return []*syntax.Node{field("arguments", p.tokenTree(false))}
	}
	return nil
}
