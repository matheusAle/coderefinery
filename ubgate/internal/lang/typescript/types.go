package typescript

import "github.com/matheusAle/ub-mode/ubgate/internal/syntax"

// TypeScript types (tree-sitter-typescript define-grammar.js `type`).

var predefinedTypes = map[string]bool{
	"any": true, "number": true, "boolean": true, "string": true, "symbol": true, "void": true,
	"unknown": true, "never": true, "object": true,
}

func (p *parser) typeAnnotation() *syntax.Node {
	colon := p.expect(":")
	return mk("type_annotation", colon, p.typ())
}

// typ parses a `type`.
func (p *parser) typ() *syntax.Node {
	saved := p.noASI
	p.noASI = true
	defer func() { p.noASI = saved }()
	return p.typeNoASI(true)
}

func (p *parser) typeNoASI(allowConditional bool) *syntax.Node {
	if f := p.functionType(); f != nil {
		return f
	}
	t := p.unionType()
	if allowConditional && p.atWord("extends") {
		ext := p.anon()
		right := p.typeNoASI(false)
		q := p.expect("?")
		cons := p.typeNoASI(true)
		colon := p.expect(":")
		alt := p.typeNoASI(true)
		t = mk("conditional_type", fld("left", t), ext, fld("right", right), q, fld("consequence", cons), colon, fld("alternative", alt))
	}
	return t
}

// functionType parses function and constructor types; nil when the input
// is not one.
func (p *parser) functionType() *syntax.Node {
	switch {
	case p.atPunct("<"):
		tp := fld("type_parameters", p.typeParameters())
		params := fld("parameters", p.formalParameters())
		arrow := p.expect("=>")
		return mk("function_type", tp, params, arrow, fld("return_type", p.returnTypeOfFunctionType()))
	case p.atPunct("("):
		return p.try(func() *syntax.Node {
			params := fld("parameters", p.formalParameters())
			arrow := p.expect("=>")
			return mk("function_type", params, arrow, fld("return_type", p.returnTypeOfFunctionType()))
		})
	case p.atWord("new"), p.atWord("abstract") && p.peekIs("new"):
		var kids []*syntax.Node
		if p.atWord("abstract") {
			kids = append(kids, p.anon())
		}
		kids = append(kids, p.anon())
		if p.atPunct("<") {
			kids = append(kids, fld("type_parameters", p.typeParameters()))
		}
		kids = append(kids, fld("parameters", p.formalParameters()), p.expect("=>"))
		kids = append(kids, fld("type", p.prefixType()))
		return mk("constructor_type", kids...)
	}
	return nil
}

func (p *parser) returnTypeOfFunctionType() *syntax.Node {
	if p.atWord("asserts") && p.peek().kind == tIdent {
		kw := p.anon()
		var target *syntax.Node
		if p.peekIs("is") {
			target = p.typePredicate()
		} else if p.atWord("this") {
			target = p.leaf("this")
		} else {
			target = p.leaf("identifier")
		}
		return mk("asserts", kw, target)
	}
	if p.tok.kind == tIdent && p.peekIs("is") {
		return p.typePredicate()
	}
	return p.typeNoASI(true)
}

func (p *parser) unionType() *syntax.Node {
	var left *syntax.Node
	if p.atPunct("|") {
		bar := p.anon()
		left = mk("union_type", bar, p.intersectionType())
	} else {
		left = p.intersectionType()
	}
	for p.atPunct("|") {
		bar := p.anon()
		var right *syntax.Node
		if f := p.functionType(); f != nil {
			right = f
		} else {
			right = p.intersectionType()
		}
		left = mk("union_type", left, bar, right)
	}
	return left
}

func (p *parser) intersectionType() *syntax.Node {
	var left *syntax.Node
	if p.atPunct("&") {
		amp := p.anon()
		left = mk("intersection_type", amp, p.prefixType())
	} else {
		left = p.prefixType()
	}
	for p.atPunct("&") {
		amp := p.anon()
		var right *syntax.Node
		if f := p.functionType(); f != nil {
			right = f
		} else {
			right = p.prefixType()
		}
		left = mk("intersection_type", left, amp, right)
	}
	return left
}

func (p *parser) prefixType() *syntax.Node {
	switch {
	case p.atWord("keyof") && p.peek().kind != tPunct || p.atWord("keyof") && p.startsTypeAfter():
		kw := p.anon()
		return mk("index_type_query", kw, p.prefixType())
	case p.atWord("readonly") && (p.peek().kind == tIdent || p.peekIs("[") || p.peekIs("(") || p.peekIs("{")):
		kw := p.anon()
		return mk("readonly_type", kw, p.typeNoASI(true))
	case p.atWord("infer") && p.peek().kind == tIdent:
		kw := p.anon()
		name := p.leaf("type_identifier")
		kids := []*syntax.Node{kw, name}
		if p.atWord("extends") {
			if c := p.try(func() *syntax.Node {
				ext := p.anon()
				t := p.typeNoASI(false)
				if p.atPunct("?") {
					fail(p.tok.start, "conditional")
				}
				return mk("constraint", ext, t)
			}); c != nil {
				kids = append(kids, c.Children...)
			}
		}
		return mk("infer_type", kids...)
	}
	return p.postfixType()
}

func (p *parser) startsTypeAfter() bool {
	nt := p.peek()
	return nt.kind == tPunct && (nt.text == "(" || nt.text == "[" || nt.text == "{" || nt.text == "`")
}

func (p *parser) postfixType() *syntax.Node {
	t := p.primaryType()
	for p.atPunct("[") && !p.tok.nl && !(p.asType && (t.Type == "member_expression" || t.Type == "call_expression")) {
		open := p.anon()
		if p.atPunct("]") {
			t = mk("array_type", t, open, p.anon())
			continue
		}
		idx := p.typ()
		t = mk("lookup_type", t, open, idx, p.expect("]"))
	}
	return t
}

func (p *parser) primaryType() *syntax.Node {
	t := p.tok
	if t.kind == tPunct || t.kind == tString {
		// bracketed types are not at the edge of an `as` expression
		saved := p.asType
		p.asType = false
		defer func() { p.asType = saved }()
	}
	switch t.kind {
	case tString:
		return mk("literal_type", p.stringLit())
	case tNumber:
		return mk("literal_type", p.leaf("number"))
	case tPunct:
		switch t.text {
		case "(":
			open := p.anon()
			inner := p.typ()
			return mk("parenthesized_type", open, inner, p.expect(")"))
		case "{":
			return p.objectType("object_type")
		case "[":
			return p.tupleType()
		case "`":
			return p.templateLiteralType()
		case "-", "+":
			if nt := p.peek(); nt.kind == tNumber {
				op := fld("operator", p.anon())
				return mk("literal_type", mk("unary_expression", op, fld("argument", p.leaf("number"))))
			}
		case "*":
			return mk("existential_type", p.anon())
		case "?":
			q := p.anon()
			return mk("flow_maybe_type", q, p.postfixType())
		}
		fail(t.start, "unexpected %q in type", t.text)
	case tIdent:
		switch t.text {
		case "true", "false", "null", "undefined":
			return mk("literal_type", p.leaf(t.text))
		case "this":
			return p.leaf("this_type")
		case "typeof":
			return p.typeQuery()
		case "unique":
			if p.peekIs("symbol") {
				a := syntax.Leaf("unique symbol", false, p.tok.start, p.tok.end)
				p.next()
				b := syntax.Leaf("unique symbol", false, p.tok.start, p.tok.end)
				p.next()
				return mk("predefined_type", a, b)
			}
		case "const":
			return p.anon()
		case "import":
			if p.peekIs("(") {
				return p.importType()
			}
		}
		if predefinedTypes[t.text] && !p.peekIs(".") {
			return mk("predefined_type", p.anon())
		}
		name := p.typeName()
		if p.atPunct("<") && !p.tok.nl {
			return mk("generic_type", fld("name", name), fld("type_arguments", p.typeArguments()))
		}
		return name
	}
	fail(t.start, "unexpected token in type")
	return nil
}

// typeName parses type_identifier or nested_type_identifier.
func (p *parser) typeName() *syntax.Node {
	limit := p.asType
	if p.tok.kind != tIdent {
		fail(p.tok.start, "expected type name")
	}
	if !p.peekIs(".") {
		return p.leaf("type_identifier")
	}
	module := p.leaf("identifier")
	for {
		dot := p.expect(".")
		if p.peekIs(".") && !limit {
			prop := p.leaf("property_identifier")
			obj := module
			if obj.Type == "nested_identifier" {
				obj.Type = "member_expression"
			}
			module = mk("nested_identifier", fld("object", obj), dot, fld("property", prop))
			continue
		}
		name := p.leaf("type_identifier")
		return mk("nested_type_identifier", fld("module", module), dot, fld("name", name))
	}
}

func (p *parser) typeQuery() *syntax.Node {
	kw := p.anon()
	var e *syntax.Node
	if p.atWord("this") {
		e = p.leaf("this")
	} else if p.atWord("import") {
		p.rejectImportTypeArgs()
		e = mk("import", p.anon())
	} else {
		e = p.leaf("identifier")
	}
	for {
		switch {
		case p.atPunct("."):
			dot := p.anon()
			prop := p.propertyIdentifierOrPrivate()
			e = mk("member_expression", fld("object", e), dot, fld("property", prop))
			continue
		case p.atPunct("("):
			e = mk("call_expression", fld("function", e), fld("arguments", p.arguments()))
			continue
		case p.atPunct("<") && !p.tok.nl:
			ta := p.typeArguments()
			e = mk("instantiation_expression", fld("function", e), fld("type_arguments", ta))
		}
		break
	}
	return mk("type_query", kw, e)
}

func (p *parser) tupleType() *syntax.Node {
	kids := []*syntax.Node{p.expect("[")}
	for !p.atPunct("]") {
		kids = append(kids, p.tupleMember())
		if !p.atPunct(",") {
			break
		}
		kids = append(kids, p.anon())
	}
	kids = append(kids, p.expect("]"))
	return mk("tuple_type", kids...)
}

func (p *parser) tupleMember() *syntax.Node {
	// labelled members: `name: T`, `name?: T`, `...name: T`
	if p.tok.kind == tIdent {
		nt := p.peek()
		if nt.kind == tPunct && nt.text == ":" {
			name := fld("name", p.leaf("identifier"))
			return mk("required_parameter", name, fld("type", p.typeAnnotation()))
		}
		if nt.kind == tPunct && nt.text == "?" && p.peek2().kind == tPunct && p.peek2().text == ":" {
			name := fld("name", p.leaf("identifier"))
			q := p.anon()
			return mk("optional_parameter", name, q, fld("type", p.typeAnnotation()))
		}
	}
	if p.atPunct("...") {
		if nt := p.peek(); nt.kind == tIdent && p.peek2().kind == tPunct && p.peek2().text == ":" {
			dots := p.anon()
			name := fld("name", mk("rest_pattern", dots, p.leaf("identifier")))
			return mk("required_parameter", name, fld("type", p.typeAnnotation()))
		}
		dots := p.anon()
		return mk("rest_type", dots, p.typ())
	}
	t := p.typ()
	if p.atPunct("?") {
		return mk("optional_type", t, p.anon())
	}
	return t
}

func (p *parser) templateLiteralType() *syntax.Node {
	start := p.tok.start
	kids := []*syntax.Node{syntax.Leaf("`", false, start, start+1)}
	i := start + 1
	for {
		j := i
		for j < len(p.src) && p.src[j] != '`' && !(p.src[j] == '$' && p.byteAt(j+1) == '{') {
			if p.src[j] == '\\' {
				j++
			}
			j++
		}
		if j > i {
			kids = append(kids, syntax.Leaf("string_fragment", true, i, j))
		}
		if j >= len(p.src) {
			fail(start, "unterminated template type")
		}
		if p.src[j] == '`' {
			kids = append(kids, syntax.Leaf("`", false, j, j+1))
			p.prevEnd = j + 1
			p.tok = p.scan(j + 1)
			return mk("template_literal_type", kids...)
		}
		open := syntax.Leaf("${", false, j, j+2)
		p.prevEnd = j + 2
		p.tok = p.scan(j + 2)
		var inner *syntax.Node
		if p.atWord("infer") {
			inner = p.prefixType()
		} else {
			inner = p.typ()
		}
		if !p.atPunct("}") {
			fail(p.tok.start, "expected } in template type")
		}
		closing := syntax.Leaf("}", false, p.tok.start, p.tok.end)
		kids = append(kids, mk("template_type", open, inner, closing))
		i = p.tok.end
	}
}

func (p *parser) typeArguments() *syntax.Node {
	savedAs := p.asType
	p.asType = false
	defer func() { p.asType = savedAs }()
	if p.atPunct("<<") {
		// `f<<T>(x: T) => void>()`: split the token
		p.tok.end = p.tok.start + 1
		p.tok.text = "<"
	}
	open := p.expect("<")
	kids := []*syntax.Node{open}
	for {
		kids = append(kids, p.typ())
		if !p.atPunct(",") {
			break
		}
		kids = append(kids, p.anon())
		p.splitGT()
		if p.atPunct(">") {
			break
		}
	}
	kids = append(kids, p.expectGT())
	return mk("type_arguments", kids...)
}

func (p *parser) typeParameters() *syntax.Node {
	savedAs := p.asType
	p.asType = false
	defer func() { p.asType = savedAs }()
	open := p.expect("<")
	kids := []*syntax.Node{open}
	for {
		var tk []*syntax.Node
		if p.atWord("const") {
			tk = append(tk, p.anon())
		}
		if p.atWord("in") || p.atWord("out") {
			if nt := p.peek(); nt.kind == tIdent {
				tk = append(tk, p.anon())
			}
		}
		tk = append(tk, fld("name", p.leaf("type_identifier")))
		if p.atWord("extends") {
			ext := p.anon()
			tk = append(tk, fld("constraint", mk("constraint", ext, p.typ())))
		}
		if p.atPunct("=") {
			eq := p.anon()
			tk = append(tk, fld("value", mk("default_type", eq, p.typ())))
		}
		kids = append(kids, mk("type_parameter", tk...))
		if !p.atPunct(",") {
			break
		}
		kids = append(kids, p.anon())
		p.splitGT()
		if p.atPunct(">") {
			break
		}
	}
	kids = append(kids, p.expectGT())
	return mk("type_parameters", kids...)
}

// objectType parses object_type / interface_body.
func (p *parser) objectType(typ string) *syntax.Node {
	kids := []*syntax.Node{p.expect("{")}
	savedASI := p.noASI
	p.noASI = false
	for !p.atPunct("}") {
		if p.tok.kind == tEOF {
			fail(p.tok.start, "unterminated object type")
		}
		if p.atPunct(",") || p.atPunct(";") {
			kids = append(kids, p.anon())
			continue
		}
		kids = append(kids, p.typeMember())
		if p.atPunct(",") || p.atPunct(";") {
			kids = append(kids, p.anon())
			continue
		}
		if !p.canASI() {
			fail(p.tok.start, "expected member separator")
		}
	}
	p.noASI = savedASI
	kids = append(kids, p.anon())
	return mk(typ, kids...)
}

func (p *parser) typeMember() *syntax.Node {
	switch {
	case p.atPunct("(") || p.atPunct("<"):
		return mk("call_signature", p.callSignature()...)
	case p.atWord("new") && (p.peekIs("(") || p.peekIs("<")):
		kw := p.anon()
		kids := []*syntax.Node{kw}
		if p.atPunct("<") {
			kids = append(kids, fld("type_parameters", p.typeParameters()))
		}
		kids = append(kids, fld("parameters", p.formalParameters()))
		if p.atPunct(":") {
			kids = append(kids, fld("type", p.typeAnnotation()))
		}
		return mk("construct_signature", kids...)
	case p.atWord("export") && p.peek().kind == tIdent:
		return p.exportStatement(nil)
	}
	var kids []*syntax.Node
	if (p.atPunct("-") || p.atPunct("+")) && p.peekIs("readonly") {
		kids = append(kids, fld("sign", p.anon()), p.anon())
		return p.indexSignature(kids)
	}
	for p.isTypeMemberModifier() {
		switch p.tok.text {
		case "public", "private", "protected":
			kids = append(kids, mk("accessibility_modifier", p.anon()))
		case "override":
			kids = append(kids, mk("override_modifier", p.anon()))
		default:
			kids = append(kids, p.anon())
		}
	}
	if p.atPunct("*") {
		kids = append(kids, p.anon())
	}
	if p.atPunct("[") && p.isIndexSignature() {
		return p.indexSignature(kids)
	}
	kids = append(kids, fld("name", p.propertyName()))
	if p.atPunct("?") {
		kids = append(kids, p.anon())
	}
	if p.atPunct("(") || p.atPunct("<") {
		kids = append(kids, p.callSignature()...)
		return mk("method_signature", kids...)
	}
	if p.atPunct(":") {
		kids = append(kids, fld("type", p.typeAnnotation()))
	}
	return mk("property_signature", kids...)
}

func (p *parser) isTypeMemberModifier() bool {
	if p.tok.kind != tIdent {
		return false
	}
	switch p.tok.text {
	case "public", "private", "protected", "static", "override", "readonly", "async", "get", "set":
	default:
		return false
	}
	nt := p.peek()
	switch nt.kind {
	case tIdent, tString, tNumber, tPrivate:
		return true
	case tPunct:
		return nt.text == "[" || nt.text == "*"
	}
	return false
}

// isIndexSignature: at `[`, an index signature `[k: T]` or a mapped type
// `[K in T]` rather than a computed property name.
func (p *parser) isIndexSignature() bool {
	nt := p.peek()
	if nt.kind != tIdent {
		return false
	}
	t2 := p.peek2()
	return t2.kind == tPunct && t2.text == ":" || t2.kind == tIdent && t2.text == "in"
}

func (p *parser) indexSignature(kids []*syntax.Node) *syntax.Node {
	kids = append(kids, p.expect("["))
	if p.peekIs("in") {
		name := fld("name", p.leaf("type_identifier"))
		in := p.anon()
		ck := []*syntax.Node{name, in, fld("type", p.typ())}
		if p.atWord("as") {
			ck = append(ck, p.anon(), fld("alias", p.typ()))
		}
		kids = append(kids, mk("mapped_type_clause", ck...))
	} else {
		name := fld("name", p.leaf("identifier"))
		colon := p.expect(":")
		kids = append(kids, name, colon, fld("index_type", p.typ()))
	}
	kids = append(kids, p.expect("]"))
	switch {
	case p.atPunct(":"):
		kids = append(kids, fld("type", p.typeAnnotation()))
	case p.atPunct("?") && p.byteAt(p.tok.end) == ':':
		op := syntax.Leaf("?:", false, p.tok.start, p.tok.end+1)
		p.tok = p.scan(p.tok.end + 1)
		kids = append(kids, fld("type", mk("opting_type_annotation", op, p.typ())))
	case (p.atPunct("-") || p.atPunct("+")) && p.byteAt(p.tok.end) == '?' && p.byteAt(p.tok.end+1) == ':':
		typ := "omitting_type_annotation"
		if p.tok.text == "+" {
			typ = "adding_type_annotation"
		}
		op := syntax.Leaf(p.tok.text+"?:", false, p.tok.start, p.tok.end+2)
		p.tok = p.scan(p.tok.end + 2)
		kids = append(kids, fld("type", mk(typ, op, p.typ())))
	default:
		if p.atPunct("?") {
			// `[K in T]?` without a type is not valid; fall through
			fail(p.tok.start, "bad index signature")
		}
	}
	return mk("index_signature", kids...)
}

// importType is `import("m").A.B` in a type position: tree-sitter-typescript
// reads it as call_expression / member_expression.
// rejectImportTypeArgs fails an import type inside expression type
// arguments: tree-sitter cannot read `f<typeof import("m")>()` as a call.
func (p *parser) rejectImportTypeArgs() {
	if p.exprTypeArgs && !p.importTypeArgs {
		p.rejectedImportTypeArgs = true
		fail(p.tok.start, "import type in expression type arguments")
	}
}

// asTarget parses the type after `as` / `satisfies`. tree-sitter ends the
// expression early there: a dotted name keeps two parts and an import type
// takes no `[...]` lookup (`x as a.b.C` is member C of `x as a.b`).
func (p *parser) asTarget() *syntax.Node {
	saved := p.asType
	p.asType = true
	defer func() { p.asType = saved }()
	return p.typ()
}

func (p *parser) importType() *syntax.Node {
	p.rejectImportTypeArgs()
	imp := mk("import", p.anon())
	e := mk("call_expression", fld("function", imp), fld("arguments", p.arguments()))
	for p.atPunct(".") {
		dot := p.anon()
		e = mk("member_expression", fld("object", e), dot, fld("property", p.leaf("property_identifier")))
	}
	return e
}
