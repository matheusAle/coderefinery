package typescript

import "github.com/matheusAle/ub-mode/ubgate/internal/syntax"

// Statements and declarations (grammar.js `statement`, `declaration`).

func (p *parser) statement() *syntax.Node {
	saved := p.noASI
	savedIn := p.noIn
	p.noASI, p.noIn = false, false
	defer func() { p.noASI, p.noIn = saved, savedIn }()
	inCase := p.caseBody
	p.caseBody = false

	if p.atPunct("@") {
		return p.decorated()
	}
	if p.atPunct("{") {
		return p.block(!inCase)
	}
	if p.atPunct(";") {
		return mk("empty_statement", p.anon())
	}
	if p.tok.kind == tIdent {
		if n := p.keywordStatement(); n != nil {
			return n
		}
		if nt := p.peek(); nt.kind == tPunct && nt.text == ":" && !isReservedWord(p.tok.text) {
			label := fld("label", p.leaf("statement_identifier"))
			colon := p.anon()
			return mk("labeled_statement", label, colon, fld("body", p.statement()))
		}
	}
	return p.expressionStatement()
}

func (p *parser) expressionStatement() *syntax.Node {
	e := p.expressions()
	return mk("expression_statement", e, p.semicolon())
}

// keywordStatement handles statements introduced by a word; nil when the
// word starts an expression instead.
func (p *parser) keywordStatement() *syntax.Node {
	switch p.tok.text {
	case "if":
		return p.ifStatement()
	case "for":
		return p.forStatement()
	case "while":
		kw := p.anon()
		cond := fld("condition", p.parenthesized())
		return mk("while_statement", kw, cond, fld("body", p.statement()))
	case "do":
		return p.doStatement()
	case "try":
		return p.tryStatement()
	case "switch":
		return p.switchStatement()
	case "return":
		kw := p.anon()
		var e *syntax.Node
		if !p.atPunct(";") && !p.canASI() {
			e = p.expressions()
		}
		return mk("return_statement", kw, e, p.semicolon())
	case "throw":
		kw := p.anon()
		e := p.expressions()
		return mk("throw_statement", kw, e, p.semicolon())
	case "break", "continue":
		typ := p.tok.text + "_statement"
		kw := p.anon()
		var label *syntax.Node
		if p.tok.kind == tIdent && !p.tok.nl {
			label = fld("label", p.leaf("statement_identifier"))
		}
		return mk(typ, kw, label, p.semicolon())
	case "debugger":
		kw := p.anon()
		return mk("debugger_statement", kw, p.semicolon())
	case "with":
		kw := p.anon()
		obj := fld("object", p.parenthesized())
		return mk("with_statement", kw, obj, fld("body", p.statement()))
	case "import":
		if nt := p.peek(); nt.kind == tPunct && (nt.text == "(" || nt.text == ".") {
			return nil
		}
		return p.importStatement()
	case "export":
		return p.exportStatement(nil)
	}
	return p.declaration(nil)
}

// declaration parses a `declaration` when the current word starts one;
// nil otherwise. decorators (already parsed) go to a class.
func (p *parser) declaration(decorators []*syntax.Node) *syntax.Node {
	nt := p.peek()
	switch p.tok.text {
	case "function":
		return p.functionDeclaration(nil)
	case "async":
		if nt.kind == tIdent && nt.text == "function" && !nt.nl {
			return p.functionDeclaration(p.anon())
		}
	case "class":
		return p.classDeclaration(decorators, nil)
	case "var":
		return p.variableDeclaration()
	case "const":
		if p.ts && nt.kind == tIdent && nt.text == "enum" {
			return p.enumDeclaration()
		}
		return p.lexicalDeclaration()
	case "let":
		if nt.kind == tIdent || nt.kind == tPunct && (nt.text == "[" || nt.text == "{") {
			return p.lexicalDeclaration()
		}
	case "using":
		if !p.ts && nt.kind == tIdent && !nt.nl && nt.text != "in" && nt.text != "of" && nt.text != "instanceof" {
			return p.usingDeclaration(nil)
		}
	case "await":
		if !p.ts && nt.kind == tIdent && nt.text == "using" && !nt.nl {
			if t2 := p.peek2(); t2.kind == tIdent && !t2.nl {
				return p.usingDeclaration(p.anon())
			}
		}
	}
	if !p.ts {
		return nil
	}
	switch p.tok.text {
	case "abstract":
		if nt.kind == tIdent && nt.text == "class" {
			return p.classDeclaration(decorators, p.anon())
		}
	case "type":
		if nt.kind == tIdent && !nt.nl {
			return p.typeAlias()
		}
	case "interface":
		if nt.kind == tIdent && !nt.nl {
			return p.interfaceDeclaration()
		}
	case "enum":
		if nt.kind == tIdent {
			return p.enumDeclaration()
		}
	case "module":
		if (nt.kind == tIdent || nt.kind == tString) && !nt.nl {
			kw := p.anon()
			return p.moduleRest("module", kw)
		}
	case "namespace":
		if nt.kind == tIdent && !nt.nl {
			// a namespace at statement level is an expression statement
			kw := p.anon()
			m := p.moduleRest("internal_module", kw)
			return mk("expression_statement", m, p.optSemicolon())
		}
	case "declare":
		if (nt.kind == tIdent) && !nt.nl {
			return p.ambientDeclaration()
		}
	case "global":
		if nt.kind == tPunct && nt.text == "{" {
			kw := p.anon()
			return p.moduleRestBody("module", kw, nil)
		}
	}
	return nil
}

// optSemicolon consumes a `;` if present (no ASI requirement).
func (p *parser) optSemicolon() *syntax.Node {
	if p.atPunct(";") {
		return p.anon()
	}
	return nil
}

func (p *parser) decorated() *syntax.Node {
	var decs []*syntax.Node
	for p.atPunct("@") {
		decs = append(decs, fld("decorator", p.decorator()))
	}
	switch {
	case p.atWord("export"):
		return p.exportStatement(decs)
	case p.atWord("class"):
		return p.classDeclaration(decs, nil)
	case p.ts && p.atWord("abstract"):
		return p.classDeclaration(decs, p.anon())
	}
	fail(p.tok.start, "decorator without class")
	return nil
}

func (p *parser) decorator() *syntax.Node {
	at := p.anon()
	var e *syntax.Node
	if p.atPunct("(") {
		e = p.parenthesized()
	} else {
		if p.tok.kind != tIdent {
			fail(p.tok.start, "bad decorator")
		}
		e = p.leaf("identifier")
		for p.atPunct(".") {
			dot := p.anon()
			prop := p.propertyIdentifierOrPrivate()
			e = mk("member_expression", fld("object", e), dot, fld("property", prop))
		}
		if p.ts && p.atPunct("<") {
			if ta := p.try(func() *syntax.Node {
				ta := p.typeArguments()
				if !p.atPunct("(") {
					fail(p.tok.start, "no call")
				}
				return ta
			}); ta != nil {
				args := p.arguments()
				e = mk("call_expression", fld("function", e), fld("type_arguments", ta), fld("arguments", args))
			}
		}
		if p.atPunct("(") {
			e = mk("call_expression", fld("function", e), fld("arguments", p.arguments()))
		}
	}
	return mk("decorator", at, e)
}

// block parses a statement_block; absorbJS says whether the javascript
// grammar allows its optional automatic semicolon here.
func (p *parser) block(absorbJS bool) *syntax.Node {
	open := p.expect("{")
	kids := []*syntax.Node{open}
	for !p.atPunct("}") {
		if p.tok.kind == tEOF {
			fail(p.tok.start, "unterminated block")
		}
		kids = append(kids, p.statement())
	}
	kids = append(kids, p.anon())
	b := mk("statement_block", kids...)
	if m := p.blockASI(b, absorbJS); m != nil {
		b.Children = append(b.Children, m)
		b.FitChildren()
	}
	return b
}

func (p *parser) parenthesized() *syntax.Node {
	open := p.expect("(")
	saved := p.noASI
	savedIn := p.noIn
	p.noASI, p.noIn = true, false
	e := p.expressions()
	p.noASI, p.noIn = saved, savedIn
	return mk("parenthesized_expression", open, e, p.expect(")"))
}

func (p *parser) ifStatement() *syntax.Node {
	kw := p.anon()
	cond := fld("condition", p.parenthesized())
	cons := fld("consequence", p.statement())
	var alt *syntax.Node
	if p.atWord("else") {
		e := p.anon()
		alt = fld("alternative", mk("else_clause", e, p.statement()))
	}
	return mk("if_statement", kw, cond, cons, alt)
}

func (p *parser) doStatement() *syntax.Node {
	kw := p.anon()
	body := fld("body", p.statement())
	w := p.expect("while")
	cond := fld("condition", p.parenthesized())
	var semi *syntax.Node
	if p.atPunct(";") {
		semi = p.anon()
	}
	return mk("do_statement", kw, body, w, cond, semi)
}

func (p *parser) tryStatement() *syntax.Node {
	kw := p.anon()
	body := fld("body", p.block(false))
	var handler, finalizer *syntax.Node
	if p.atWord("catch") {
		c := p.anon()
		kids := []*syntax.Node{c}
		if p.atPunct("(") {
			kids = append(kids, p.anon())
			kids = append(kids, fld("parameter", p.bindingTarget()))
			if p.ts && p.atPunct(":") {
				kids = append(kids, fld("type", p.typeAnnotation()))
			}
			kids = append(kids, p.expect(")"))
		}
		kids = append(kids, fld("body", p.block(false)))
		handler = fld("handler", mk("catch_clause", kids...))
	}
	if p.atWord("finally") {
		f := p.anon()
		finalizer = fld("finalizer", mk("finally_clause", f, fld("body", p.block(false))))
	}
	return mk("try_statement", kw, body, handler, finalizer)
}

func (p *parser) switchStatement() *syntax.Node {
	kw := p.anon()
	value := fld("value", p.parenthesized())
	open := p.expect("{")
	kids := []*syntax.Node{open}
	for !p.atPunct("}") {
		var ck []*syntax.Node
		typ := "switch_case"
		switch {
		case p.atWord("case"):
			ck = append(ck, p.anon())
			saved := p.noASI
			p.noASI = true
			ck = append(ck, fld("value", p.expressions()))
			p.noASI = saved
		case p.atWord("default"):
			typ = "switch_default"
			ck = append(ck, p.anon())
		default:
			fail(p.tok.start, "expected case")
		}
		ck = append(ck, p.expect(":"))
		for !p.atPunct("}") && !p.atWord("case") && !p.atWord("default") {
			if p.tok.kind == tEOF {
				fail(p.tok.start, "unterminated switch")
			}
			p.caseBody = true
			ck = append(ck, fld("body", p.statement()))
		}
		kids = append(kids, mk(typ, ck...))
	}
	kids = append(kids, p.anon())
	return mk("switch_statement", kw, value, fld("body", mk("switch_body", kids...)))
}

func (p *parser) forStatement() *syntax.Node {
	kw := p.anon()
	kids := []*syntax.Node{kw}
	if p.atWord("await") {
		kids = append(kids, p.anon())
	}
	open := p.expect("(")
	kids = append(kids, open)
	savedASI := p.noASI
	p.noASI = true
	defer func() { p.noASI = savedASI }()

	// declaration heads
	if p.atWord("var") || p.atWord("const") || p.atWord("let") && p.letStartsDeclaration() {
		s := p.save()
		kind := p.anon()
		target := p.bindingTarget()
		if p.atWord("in") || p.atWord("of") {
			kind.Field = "kind"
			kids = append(kids, kind, fld("left", target))
			return p.forInRest(kids)
		}
		if kind.Type == "var" && p.atPunct("=") {
			// `for (var k = 1 in o)`
			if n := p.try(func() *syntax.Node {
				eq := p.anon()
				p.noIn = true
				v := p.assignment()
				p.noIn = false
				if !p.atWord("in") {
					fail(p.tok.start, "no in")
				}
				kind.Field = "kind"
				k := append(kids, kind, fld("left", target), eq, fld("value", v))
				return p.forInRest(k)
			}); n != nil {
				return n
			}
		}
		p.restore(s)
		p.noIn = true
		var decl *syntax.Node
		if p.atWord("var") {
			decl = p.variableDeclaration()
		} else {
			decl = p.lexicalDeclaration()
		}
		p.noIn = false
		kids = append(kids, fld("initializer", decl))
		return p.forRest(kids)
	}
	if p.atPunct(";") {
		kids = append(kids, fld("initializer", mk("empty_statement", p.anon())))
		return p.forRest(kids)
	}
	p.noIn = true
	e := p.expressions()
	p.noIn = false
	if p.atWord("in") || p.atWord("of") {
		kids = append(kids, fld("left", toPattern(e)))
		return p.forInRest(kids)
	}
	kids = append(kids, fld("initializer", e), p.expect(";"))
	return p.forRest(kids)
}

func (p *parser) letStartsDeclaration() bool {
	nt := p.peek()
	return nt.kind == tIdent && nt.text != "in" && nt.text != "of" || nt.kind == tPunct && (nt.text == "[" || nt.text == "{")
}

func (p *parser) forInRest(kids []*syntax.Node) *syntax.Node {
	kids = append(kids, fld("operator", p.anon()))
	kids = append(kids, fld("right", p.expressions()))
	kids = append(kids, p.expect(")"))
	p.noASI = false
	kids = append(kids, fld("body", p.statement()))
	return mk("for_in_statement", kids...)
}

func (p *parser) forRest(kids []*syntax.Node) *syntax.Node {
	if p.atPunct(";") {
		kids = append(kids, fld("condition", mk("empty_statement", p.anon())))
	} else {
		kids = append(kids, fld("condition", p.expressions()), fld("condition", p.expect(";")))
	}
	if !p.atPunct(")") {
		kids = append(kids, fld("increment", p.expressions()))
	}
	kids = append(kids, p.expect(")"))
	p.noASI = false
	kids = append(kids, fld("body", p.statement()))
	return mk("for_statement", kids...)
}

func (p *parser) variableDeclaration() *syntax.Node {
	kw := p.anon()
	kids := append([]*syntax.Node{kw}, p.declarators()...)
	return mk("variable_declaration", append(kids, p.semicolon())...)
}

func (p *parser) lexicalDeclaration() *syntax.Node {
	kw := fld("kind", p.anon())
	kids := append([]*syntax.Node{kw}, p.declarators()...)
	return mk("lexical_declaration", append(kids, p.semicolon())...)
}

func (p *parser) usingDeclaration(await *syntax.Node) *syntax.Node {
	kw := p.anon()
	kids := []*syntax.Node{await, kw}
	kids = append(kids, p.declarators()...)
	return mk("using_declaration", append(kids, p.semicolon())...)
}

func (p *parser) declarators() []*syntax.Node {
	var out []*syntax.Node
	for {
		out = append(out, p.variableDeclarator())
		if !p.atPunct(",") {
			return out
		}
		out = append(out, p.anon())
	}
}

func (p *parser) variableDeclarator() *syntax.Node {
	name := fld("name", p.bindingTarget())
	kids := []*syntax.Node{name}
	if p.ts && p.atPunct("!") && name.Type == "identifier" {
		kids = append(kids, p.anon())
	}
	if p.ts && p.atPunct(":") {
		kids = append(kids, fld("type", p.typeAnnotation()))
	}
	if p.atPunct("=") {
		kids = append(kids, p.anon(), fld("value", p.assignment()))
	}
	return mk("variable_declarator", kids...)
}

// ---- modules ----

func (p *parser) fromClause() []*syntax.Node {
	from := p.expect("from")
	return []*syntax.Node{from, fld("source", p.stringLit())}
}

func (p *parser) importStatement() *syntax.Node {
	kw := p.anon()
	kids := []*syntax.Node{kw}
	if p.ts && (p.atWord("type") || p.atWord("typeof")) {
		nt := p.peek()
		if nt.kind == tIdent && nt.text != "from" || nt.kind == tPunct && (nt.text == "{" || nt.text == "*") ||
			nt.kind == tIdent && nt.text == "from" && p.peek2().kind == tIdent {
			kids = append(kids, p.anon())
		}
	}
	switch {
	case p.tok.kind == tString:
		kids = append(kids, fld("source", p.stringLit()))
	case p.ts && p.tok.kind == tIdent && p.peekIs("="):
		// import x = require("m") / import x = A.B
		name := p.leaf("identifier")
		eq := p.anon()
		if p.atWord("require") && p.peekIs("(") {
			req := p.anon()
			open := p.anon()
			src := fld("source", p.stringLit())
			cl := mk("import_require_clause", name, eq, req, open, src, p.expect(")"))
			kids = append(kids, cl)
		} else {
			// import_alias
			target := p.entityName()
			kids[0] = kw
			all := append(kids, name, eq, target, p.semicolon())
			return mk("import_alias", all...)
		}
	default:
		kids = append(kids, p.importClause())
		kids = append(kids, p.fromClause()...)
	}
	if p.atWord("with") || p.atWord("assert") && !p.tok.nl {
		kw := p.anon()
		kids = append(kids, mk("import_attribute", kw, p.object()))
	}
	kids = append(kids, p.semicolon())
	return mk("import_statement", kids...)
}

func (p *parser) peekIs(s string) bool {
	nt := p.peek()
	return (nt.kind == tPunct || nt.kind == tIdent) && nt.text == s
}

// entityName parses identifier or nested_identifier (a.b.c).
func (p *parser) entityName() *syntax.Node {
	e := p.leaf("identifier")
	for p.atPunct(".") {
		dot := p.anon()
		prop := p.leaf("property_identifier")
		obj := e
		if obj.Type == "nested_identifier" {
			obj.Type = "member_expression"
		}
		e = mk("nested_identifier", fld("object", obj), dot, fld("property", prop))
	}
	return e
}

func (p *parser) importClause() *syntax.Node {
	var kids []*syntax.Node
	if p.tok.kind == tIdent && !p.atWord("from") || p.atWord("from") && p.peekIs("from") {
		kids = append(kids, p.leaf("identifier"))
		if !p.atPunct(",") {
			return mk("import_clause", kids...)
		}
		kids = append(kids, p.anon())
	}
	if p.atPunct("*") {
		star := p.anon()
		as := p.expect("as")
		kids = append(kids, mk("namespace_import", star, as, p.leaf("identifier")))
	} else {
		kids = append(kids, p.namedImports())
	}
	return mk("import_clause", kids...)
}

func (p *parser) namedImports() *syntax.Node {
	kids := []*syntax.Node{p.expect("{")}
	for !p.atPunct("}") {
		var sk []*syntax.Node
		if p.ts && (p.atWord("type") || p.atWord("typeof")) {
			nt := p.peek()
			if nt.kind == tIdent && (nt.text != "as" || p.peek2().kind == tIdent) || nt.kind == tString {
				sk = append(sk, p.anon())
			}
		}
		name := p.moduleExportName()
		if p.atWord("as") {
			as := p.anon()
			sk = append(sk, fld("name", name), as, fld("alias", p.leaf("identifier")))
		} else {
			sk = append(sk, fld("name", name))
		}
		kids = append(kids, mk("import_specifier", sk...))
		if !p.atPunct(",") {
			break
		}
		kids = append(kids, p.anon())
	}
	kids = append(kids, p.expect("}"))
	return mk("named_imports", kids...)
}

func (p *parser) moduleExportName() *syntax.Node {
	if p.tok.kind == tString {
		return p.stringLit()
	}
	if !p.ts && p.atWord("default") {
		return p.anon() // typescript reads it as an identifier
	}
	if p.tok.kind != tIdent {
		fail(p.tok.start, "expected name")
	}
	return p.leaf("identifier")
}

func (p *parser) exportStatement(decs []*syntax.Node) *syntax.Node {
	kids := append([]*syntax.Node{}, decs...)
	kids = append(kids, p.expect("export"))
	switch {
	case p.atPunct("*"):
		star := p.anon()
		if p.atWord("as") {
			as := p.anon()
			kids = append(kids, mk("namespace_export", star, as, p.moduleExportName()))
		} else {
			kids = append(kids, star)
		}
		kids = append(kids, p.fromClause()...)
		kids = append(kids, p.semicolon())
	case p.atPunct("{"):
		kids = append(kids, p.exportClause())
		if p.atWord("from") {
			kids = append(kids, p.fromClause()...)
		}
		kids = append(kids, p.semicolon())
	case p.ts && p.atWord("type") && p.peekIs("{"):
		kids = append(kids, p.anon(), p.exportClause())
		if p.atWord("from") {
			kids = append(kids, p.fromClause()...)
		}
		kids = append(kids, p.semicolon())
	case p.ts && p.atPunct("="):
		kids = append(kids, p.anon(), p.expressions(), p.semicolon())
	case p.ts && p.atWord("as") && p.peekIs("namespace"):
		kids = append(kids, p.anon(), p.anon(), p.leaf("identifier"), p.semicolon())
	case p.ts && p.atWord("import"):
		kw := p.anon()
		name := p.leaf("identifier")
		eq := p.expect("=")
		target := p.entityName()
		kids = append(kids, fld("declaration", mk("import_alias", kw, name, eq, target, p.semicolon())))
	case p.atWord("default"):
		kids = append(kids, p.anon())
		if d := p.defaultDeclaration(); d != nil {
			kids = append(kids, fld("declaration", d))
		} else {
			kids = append(kids, fld("value", p.expressions()), p.semicolon())
		}
	default:
		var d *syntax.Node
		if p.atPunct("@") {
			var ds []*syntax.Node
			for p.atPunct("@") {
				ds = append(ds, fld("decorator", p.decorator()))
			}
			if p.ts && p.atWord("abstract") {
				d = p.classDeclaration(ds, p.anon())
			} else {
				d = p.classDeclaration(ds, nil)
			}
		} else if p.tok.kind == tIdent {
			if p.ts && p.atWord("namespace") {
				kw := p.anon()
				d = p.moduleRest("internal_module", kw)
			} else {
				p.exporting = true
				d = p.declaration(nil)
				p.exporting = false
			}
		}
		if d == nil {
			fail(p.tok.start, "bad export")
		}
		kids = append(kids, fld("declaration", d))
	}
	return mk("export_statement", kids...)
}

// defaultDeclaration parses what follows `export default` when it is a
// declaration (function/class with a name or not); nil for an expression.
func (p *parser) defaultDeclaration() *syntax.Node {
	nt := p.peek()
	switch {
	case p.atWord("function"):
		if nt.kind == tIdent || nt.kind == tPunct && nt.text == "*" && p.peek2().kind == tIdent {
			return p.functionDeclaration(nil)
		}
		return nil
	case p.atWord("async") && nt.kind == tIdent && nt.text == "function" && !nt.nl:
		if t2 := p.peek2(); t2.kind == tIdent || t2.kind == tPunct && t2.text == "*" {
			return p.functionDeclaration(p.anon())
		}
		return nil
	case p.atWord("class"):
		if nt.kind == tIdent && nt.text != "extends" && nt.text != "implements" {
			return p.classDeclaration(nil, nil)
		}
		return nil
	case p.ts && p.atWord("abstract") && nt.kind == tIdent && nt.text == "class":
		return p.classDeclaration(nil, p.anon())
	case p.ts && p.atWord("interface") && nt.kind == tIdent:
		return p.interfaceDeclaration()
	}
	if p.tok.kind == tIdent && (p.atWord("const") || p.atWord("let") || p.atWord("var")) {
		return p.declaration(nil)
	}
	return nil
}

func (p *parser) exportClause() *syntax.Node {
	kids := []*syntax.Node{p.expect("{")}
	for !p.atPunct("}") {
		var sk []*syntax.Node
		if p.ts && (p.atWord("type") || p.atWord("typeof")) {
			nt := p.peek()
			if nt.kind == tIdent && (nt.text != "as" || p.peek2().kind == tIdent) || nt.kind == tString {
				sk = append(sk, p.anon())
			}
		}
		sk = append(sk, fld("name", p.moduleExportName()))
		if p.atWord("as") {
			sk = append(sk, p.anon(), fld("alias", p.moduleExportName()))
		}
		kids = append(kids, mk("export_specifier", sk...))
		if !p.atPunct(",") {
			break
		}
		kids = append(kids, p.anon())
	}
	kids = append(kids, p.expect("}"))
	return mk("export_clause", kids...)
}

// ---- TypeScript declarations ----

func (p *parser) typeAlias() *syntax.Node {
	kw := p.anon()
	name := fld("name", p.leaf("type_identifier"))
	var tp *syntax.Node
	if p.atPunct("<") {
		tp = fld("type_parameters", p.typeParameters())
	}
	eq := p.expect("=")
	value := fld("value", p.typ())
	return mk("type_alias_declaration", kw, name, tp, eq, value, p.semicolon())
}

func (p *parser) interfaceDeclaration() *syntax.Node {
	kw := p.anon()
	name := fld("name", p.leaf("type_identifier"))
	var tp, ext *syntax.Node
	if p.atPunct("<") {
		tp = fld("type_parameters", p.typeParameters())
	}
	if p.atWord("extends") {
		ek := []*syntax.Node{p.anon()}
		for {
			ek = append(ek, fld("type", p.heritageType()))
			if !p.atPunct(",") {
				break
			}
			ek = append(ek, p.anon())
		}
		ext = mk("extends_type_clause", ek...)
	}
	body := p.objectType("interface_body")
	return mk("interface_declaration", kw, name, tp, ext, fld("body", body))
}

// heritageType: type_identifier, nested_type_identifier or generic_type.
func (p *parser) heritageType() *syntax.Node {
	t := p.typeName()
	if p.atPunct("<") {
		t = mk("generic_type", fld("name", t), fld("type_arguments", p.typeArguments()))
	}
	return t
}

func (p *parser) enumDeclaration() *syntax.Node {
	var kids []*syntax.Node
	if p.atWord("const") {
		kids = append(kids, p.anon())
	}
	kids = append(kids, p.expect("enum"), fld("name", p.leaf("identifier")))
	bk := []*syntax.Node{p.expect("{")}
	for !p.atPunct("}") {
		name := p.propertyName()
		if p.atPunct("=") {
			eq := p.anon()
			bk = append(bk, mk("enum_assignment", fld("name", name), eq, fld("value", p.assignment())))
		} else {
			bk = append(bk, fld("name", name))
		}
		if !p.atPunct(",") {
			break
		}
		bk = append(bk, p.anon())
	}
	bk = append(bk, p.expect("}"))
	kids = append(kids, fld("body", mk("enum_body", bk...)))
	return mk("enum_declaration", kids...)
}

func (p *parser) moduleRest(typ string, kw *syntax.Node) *syntax.Node {
	var name *syntax.Node
	if p.tok.kind == tString {
		name = p.stringLit()
	} else {
		name = p.entityName()
	}
	return p.moduleRestBody(typ, kw, fld("name", name))
}

func (p *parser) moduleRestBody(typ string, kw, name *syntax.Node) *syntax.Node {
	var body *syntax.Node
	if p.atPunct("{") {
		body = fld("body", p.block(true))
	}
	if body == nil && typ == "module" {
		return mk(typ, kw, name, p.optSemicolon())
	}
	return mk(typ, kw, name, body)
}

func (p *parser) ambientDeclaration() *syntax.Node {
	kw := p.anon()
	switch {
	case p.atWord("global") && p.peekIs("{"):
		g := p.anon()
		return mk("ambient_declaration", kw, g, p.block(true))
	case p.atWord("module") && p.peekIs("."):
		m := p.anon()
		dot := p.anon()
		prop := p.leaf("property_identifier")
		colon := p.expect(":")
		t := p.typ()
		return mk("ambient_declaration", kw, m, dot, prop, colon, t, p.semicolon())
	case p.atWord("namespace"):
		nkw := p.anon()
		return mk("ambient_declaration", kw, p.moduleRest("internal_module", nkw))
	}
	d := p.declaration(nil)
	if d == nil {
		fail(p.tok.start, "bad declare")
	}
	return mk("ambient_declaration", kw, d)
}

// isReservedWord lists words that cannot be labels or plain identifiers.
func isReservedWord(s string) bool {
	switch s {
	case "break", "case", "catch", "class", "const", "continue", "debugger", "default", "delete", "do",
		"else", "export", "extends", "finally", "for", "function", "if", "import", "in", "instanceof",
		"new", "return", "super", "switch", "this", "throw", "try", "typeof", "var", "void", "while", "with",
		"null", "true", "false", "enum":
		return true
	}
	return false
}
