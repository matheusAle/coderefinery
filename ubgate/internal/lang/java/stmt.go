package java

import "github.com/matheusAle/ub-mode/ubgate/internal/syntax"

func (p *parser) block() *syntax.Node {
	kids := []*syntax.Node{p.expect("{")}
	for !p.isOp("}") {
		if p.eof() {
			p.fail("unterminated block")
		}
		kids = append(kids, p.statement(false))
	}
	return inner("block", append(kids, p.next())...)
}

// statement parses one statement; at the top level a method declaration
// (implicitly declared class) is accepted too.
func (p *parser) statement(top bool) *syntax.Node {
	t := p.cur()
	if t.kind == tkOp {
		switch t.text {
		case "{":
			return p.block()
		case ";":
			return p.next()
		case "@interface":
			return p.typeDecl(nil)
		case "@":
			return p.declarationStatement(top)
		}
	}
	if t.kind == tkIdent {
		switch t.text {
		case "if":
			kids := []*syntax.Node{p.next(), field("condition", p.parenthesized()), field("consequence", p.statement(false))}
			if p.isKw("else") {
				kids = append(kids, p.next(), field("alternative", p.statement(false)))
			}
			return inner("if_statement", kids...)
		case "while":
			return inner("while_statement", p.next(), field("condition", p.parenthesized()), field("body", p.statement(false)))
		case "do":
			kw := p.next()
			body := field("body", p.statement(false))
			return inner("do_statement", kw, body, p.expect("while"), field("condition", p.parenthesized()), p.expect(";"))
		case "for":
			return p.forStatement()
		case "try":
			return p.tryStatement()
		case "switch":
			return p.switchExpression()
		case "return":
			kids := []*syntax.Node{p.next()}
			if !p.isOp(";") {
				kids = append(kids, p.expression())
			}
			return inner("return_statement", append(kids, p.expect(";"))...)
		case "break", "continue":
			kids := []*syntax.Node{p.next()}
			if p.isIdent() {
				kids = append(kids, p.identifier())
			}
			return inner(t.text+"_statement", append(kids, p.expect(";"))...)
		case "throw":
			return inner("throw_statement", p.next(), p.expression(), p.expect(";"))
		case "assert":
			kids := []*syntax.Node{p.next(), p.expression()}
			if p.isOp(":") {
				kids = append(kids, p.next(), p.expression())
			}
			return inner("assert_statement", append(kids, p.expect(";"))...)
		case "synchronized":
			if p.isOpAt(1, "(") {
				return inner("synchronized_statement", p.next(), p.parenthesized(), field("body", p.block()))
			}
		case "yield":
			if p.isYieldStatement() {
				return inner("yield_statement", p.next(), p.expression(), p.expect(";"))
			}
		case "package":
			return inner("package_declaration", p.next(), p.name(), p.expect(";"))
		case "import":
			kids := []*syntax.Node{p.next()}
			if p.isKw("static") {
				kids = append(kids, p.next())
			}
			kids = append(kids, p.name())
			if p.isOp(".") && p.isOpAt(1, "*") {
				kids = append(kids, p.next(), inner("asterisk", p.next()))
			}
			return inner("import_declaration", append(kids, p.expect(";"))...)
		case "open", "module":
			if m := p.try(func() *syntax.Node { return p.moduleDeclaration(nil) }); m != nil {
				return m
			}
		}
		if p.isIdent() && p.isOpAt(1, ":") {
			return inner("labeled_statement", p.identifier(), p.next(), p.statement(false))
		}
		if p.isModifier() || p.isTypeDeclStart() {
			return p.declarationStatement(top)
		}
	}
	if d := p.try(func() *syntax.Node { return p.localDeclaration(nil, top) }); d != nil {
		return d
	}
	return inner("expression_statement", p.expression(), p.expect(";"))
}

// isYieldStatement: `yield` starts a yield statement unless it is used as
// a plain identifier (assignment, update, member access...).
func (p *parser) isYieldStatement() bool {
	n := p.peek(1)
	if n.kind == tkOp {
		switch n.text {
		case "=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ".", "[", ";", "->", "::", ">", "<", "==", "!=", "&&", "||", "?", ":", "*", "/", "%", "&", "|", "^", ",", ")":
			return false
		case "++", "--":
			return !(p.peek(2).kind == tkOp && p.peek(2).text == ";")
		}
	}
	return true
}

// declarationStatement: annotations/modifiers followed by a type
// declaration, a local variable declaration, package or module.
func (p *parser) declarationStatement(top bool) *syntax.Node {
	save := p.pos
	if p.isOp("@") {
		anns := p.annotations()
		if p.isKw("package") {
			kids := append(anns, p.next(), p.name(), p.expect(";"))
			return inner("package_declaration", kids...)
		}
		if p.isKw("open") || p.isKw("module") {
			if m := p.try(func() *syntax.Node { return p.moduleDeclaration(anns) }); m != nil {
				return m
			}
		}
		p.pos = save
	}
	mods := p.modifiers()
	if p.isTypeDeclStart() {
		return p.typeDecl(mods)
	}
	return p.localDeclaration(mods, top)
}

// localDeclaration is local_variable_declaration (or a top-level method)
// after the modifiers; nil when the tokens are not a declaration.
func (p *parser) localDeclaration(mods *syntax.Node, top bool) *syntax.Node {
	if top {
		if m := p.try(func() *syntax.Node { return p.topMethod(mods) }); m != nil {
			return m
		}
	}
	typ := p.unannotatedType()
	if !(p.isIdent() || p.isKw("_")) {
		return nil
	}
	n := p.peek(1)
	if n.kind != tkOp || n.text != "=" && n.text != ";" && n.text != "," && n.text != "[" && n.text != "@" {
		return nil
	}
	kids := []*syntax.Node{mods, field("type", typ)}
	kids = append(kids, p.declarators()...)
	return inner("local_variable_declaration", append(kids, p.expect(";"))...)
}

func (p *parser) topMethod(mods *syntax.Node) *syntax.Node {
	var tparams []*syntax.Node
	if p.isOp("<") {
		tparams = append(tparams, field("type_parameters", p.typeParameters()))
		tparams = append(tparams, p.annotations()...)
	}
	typ := field("type", p.unannotatedType())
	if !(p.isIdent() && p.isOpAt(1, "(")) {
		return nil
	}
	return p.methodRest(mods, tparams, typ)
}

func (p *parser) moduleDeclaration(anns []*syntax.Node) *syntax.Node {
	kids := anns
	if p.isKw("open") {
		kids = append(kids, p.next())
	}
	kids = append(kids, p.expect("module"), field("name", p.name()))
	body := []*syntax.Node{p.expect("{")}
	for !p.isOp("}") {
		body = append(body, p.moduleDirective())
	}
	body = append(body, p.next())
	return inner("module_declaration", append(kids, field("body", inner("module_body", body...)))...)
}

func (p *parser) moduleDirective() *syntax.Node {
	kw := p.cur().text
	kids := []*syntax.Node{p.next()}
	switch kw {
	case "requires":
		for (p.isKw("transitive") || p.isKw("static")) && !p.isOpAt(1, ";") && !p.isOpAt(1, ".") {
			kids = append(kids, field("modifiers", inner("requires_modifier", p.next())))
		}
		kids = append(kids, field("module", p.name()))
	case "exports", "opens":
		kids = append(kids, field("package", p.name()))
		if p.isKw("to") {
			kids = append(kids, p.next(), field("modules", p.name()))
			for p.isOp(",") {
				kids = append(kids, p.next(), field("modules", p.name()))
			}
		}
	case "uses":
		kids = append(kids, field("type", p.name()))
	case "provides":
		// tree-sitter-java: the first provider carries no field name.
		kids = append(kids, field("provided", p.name()), p.expect("with"), p.name())
		for p.isOp(",") {
			kids = append(kids, p.next(), field("provider", p.name()))
		}
	default:
		p.fail("unknown module directive %q", kw)
	}
	return inner(kw+"_module_directive", append(kids, p.expect(";"))...)
}

func (p *parser) forStatement() *syntax.Node {
	kw := p.next()
	open := p.expect("(")
	if each := p.try(func() *syntax.Node { return p.enhancedForRest(kw, open) }); each != nil {
		return each
	}
	kids := []*syntax.Node{kw, open}
	if decl := p.try(func() *syntax.Node { return p.localDeclaration(p.modifiers(), false) }); decl != nil {
		kids = append(kids, field("init", decl))
	} else {
		if !p.isOp(";") {
			kids = p.commaList(kids, func() *syntax.Node { return field("init", p.expression()) })
		}
		kids = append(kids, p.expect(";"))
	}
	if !p.isOp(";") {
		kids = append(kids, field("condition", p.expression()))
	}
	kids = append(kids, p.expect(";"))
	if !p.isOp(")") {
		kids = p.commaList(kids, func() *syntax.Node { return field("update", p.expression()) })
	}
	kids = append(kids, p.expect(")"), field("body", p.statement(false)))
	return inner("for_statement", kids...)
}

func (p *parser) enhancedForRest(kw, open *syntax.Node) *syntax.Node {
	kids := []*syntax.Node{kw, open, p.modifiers(), field("type", p.unannotatedType())}
	kids = append(kids, p.declaratorID()...)
	if !p.isOp(":") {
		return nil
	}
	kids = append(kids, p.next(), field("value", p.expression()), p.expect(")"), field("body", p.statement(false)))
	return inner("enhanced_for_statement", kids...)
}

func (p *parser) tryStatement() *syntax.Node {
	kids := []*syntax.Node{p.next()}
	typ := "try_statement"
	if p.isOp("(") {
		typ = "try_with_resources_statement"
		kids = append(kids, field("resources", p.resourceSpecification()))
	}
	kids = append(kids, field("body", p.block()))
	for p.isKw("catch") {
		kw, open := p.next(), p.expect("(")
		param := []*syntax.Node{p.modifiers()}
		types := []*syntax.Node{p.unannotatedType()}
		for p.isOp("|") {
			types = append(types, p.next(), p.unannotatedType())
		}
		param = append(param, inner("catch_type", types...))
		param = append(param, p.declaratorID()...)
		kids = append(kids, inner("catch_clause", kw, open, inner("catch_formal_parameter", param...), p.expect(")"), field("body", p.block())))
	}
	if p.isKw("finally") {
		kids = append(kids, inner("finally_clause", p.next(), p.block()))
	}
	return inner(typ, kids...)
}

func (p *parser) resourceSpecification() *syntax.Node {
	kids := []*syntax.Node{p.expect("(")}
	for {
		kids = append(kids, p.resource())
		if !p.isOp(";") {
			break
		}
		kids = append(kids, p.next())
		if p.isOp(")") {
			break
		}
	}
	return inner("resource_specification", append(kids, p.expect(")"))...)
}

func (p *parser) resource() *syntax.Node {
	if r := p.try(func() *syntax.Node {
		kids := []*syntax.Node{p.modifiers(), field("type", p.unannotatedType())}
		kids = append(kids, p.declaratorID()...)
		if !p.isOp("=") {
			return nil
		}
		kids = append(kids, p.next(), field("value", p.expression()))
		return inner("resource", kids...)
	}); r != nil {
		return r
	}
	return inner("resource", p.primary())
}

func (p *parser) switchExpression() *syntax.Node {
	kw := p.expect("switch")
	cond := field("condition", p.parenthesized())
	kids := []*syntax.Node{p.expect("{")}
	for !p.isOp("}") {
		label := p.switchLabel()
		if p.isOp("->") {
			arrow := p.next()
			var body *syntax.Node
			switch {
			case p.isOp("{"):
				body = p.block()
			case p.isKw("throw"):
				body = p.statement(false)
			default:
				body = inner("expression_statement", p.expression(), p.expect(";"))
			}
			kids = append(kids, inner("switch_rule", label, arrow, body))
			continue
		}
		group := []*syntax.Node{label, p.expect(":")}
		for !p.isOp("}") && !p.isKw("case") && !(p.isKw("default") && (p.isOpAt(1, ":") || p.isOpAt(1, "->"))) {
			group = append(group, p.statement(false))
		}
		kids = append(kids, inner("switch_block_statement_group", group...))
	}
	kids = append(kids, p.next())
	return inner("switch_expression", kw, cond, field("body", inner("switch_block", kids...)))
}

func (p *parser) switchLabel() *syntax.Node {
	if p.isKw("default") {
		return inner("switch_label", p.next())
	}
	kids := []*syntax.Node{p.expect("case")}
	if pat := p.try(p.casePattern); pat != nil {
		kids = append(kids, pat)
		if p.isKw("when") {
			kids = append(kids, inner("guard", p.next(), p.expression()))
		}
		return inner("switch_label", kids...)
	}
	saved := p.noLambda
	p.noLambda = true
	defer func() { p.noLambda = saved }()
	kids = p.commaList(kids, func() *syntax.Node {
		if p.isKw("default") {
			return p.named("identifier")
		}
		return p.expression()
	})
	return inner("switch_label", kids...)
}

// casePattern: a type or record pattern followed by `->`, `:`, or `when`.
func (p *parser) casePattern() *syntax.Node {
	pat := p.pattern()
	if pat == nil || !(p.isOp("->") || p.isOp(":") || p.isKw("when")) {
		return nil
	}
	return inner("pattern", pat)
}

// pattern parses a type_pattern or record_pattern (nil when neither).
func (p *parser) pattern() *syntax.Node {
	if p.isIdent() && (p.isOpAt(1, "(") || p.isOpAt(1, "<")) {
		if rp := p.try(p.recordPattern); rp != nil {
			return rp
		}
	}
	typ := p.unannotatedType()
	if !p.isIdent() {
		return nil
	}
	return inner("type_pattern", typ, p.identifier())
}

func (p *parser) recordPattern() *syntax.Node {
	var head *syntax.Node
	if p.isOpAt(1, "<") {
		head = p.simpleType()
		if head.Type != "generic_type" {
			return nil
		}
	} else {
		head = p.identifier()
	}
	kids := []*syntax.Node{p.expect("(")}
	if !p.isOp(")") {
		kids = p.commaList(kids, func() *syntax.Node {
			if p.isKw("_") {
				return inner("record_pattern_component", p.named("underscore_pattern"))
			}
			if p.isIdent() && (p.isOpAt(1, "(") || p.isOpAt(1, "<")) {
				if rp := p.try(p.recordPattern); rp != nil {
					return rp
				}
			}
			typ := p.unannotatedType()
			return inner("record_pattern_component", typ, p.identifier())
		})
	}
	kids = append(kids, p.expect(")"))
	return inner("record_pattern", head, inner("record_pattern_body", kids...))
}
