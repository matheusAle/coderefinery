package typescript

import "github.com/matheusAle/coderefinery/internal/syntax"

// Functions, parameters, arrows, classes and object members.

func (p *parser) functionDeclaration(async *syntax.Node) *syntax.Node {
	kw := p.expect("function")
	kids := []*syntax.Node{async, kw}
	typ := "function_declaration"
	if p.atPunct("*") {
		kids = append(kids, p.anon())
		typ = "generator_function_declaration"
	}
	if p.tok.kind != tIdent {
		fail(p.tok.start, "expected function name")
	}
	kids = append(kids, fld("name", p.leaf("identifier")))
	exporting := p.exporting
	p.exporting = false
	kids = append(kids, p.callSignature()...)
	if p.atPunct("{") {
		kids = append(kids, fld("body", p.block(!exporting)))
		return mk(typ, kids...)
	}
	if !p.ts {
		fail(p.tok.start, "expected function body")
	}
	kids = append(kids, p.signatureEnd())
	return mk("function_signature", kids...)
}

// signatureEnd is `_semicolon` or the TS function-signature automatic
// semicolon (accepted everywhere a body is absent).
func (p *parser) signatureEnd() *syntax.Node {
	if p.atPunct(";") {
		return p.anon()
	}
	return p.asiMarker(p.prevEnd)
}

func (p *parser) functionExpression(async *syntax.Node) *syntax.Node {
	kw := p.expect("function")
	kids := []*syntax.Node{async, kw}
	typ := "function_expression"
	if p.atPunct("*") {
		kids = append(kids, p.anon())
		typ = "generator_function"
	}
	if p.tok.kind == tIdent {
		kids = append(kids, fld("name", p.leaf("identifier")))
	}
	kids = append(kids, p.callSignature()...)
	kids = append(kids, fld("body", p.block(true)))
	return mk(typ, kids...)
}

// callSignature is `_call_signature`: [type_parameters] formal_parameters
// [return type].
func (p *parser) callSignature() []*syntax.Node {
	var kids []*syntax.Node
	if p.ts && p.atPunct("<") {
		kids = append(kids, fld("type_parameters", p.typeParameters()))
	}
	kids = append(kids, fld("parameters", p.formalParameters()))
	if p.ts && p.atPunct(":") {
		kids = append(kids, fld("return_type", p.returnType()))
	}
	return kids
}

// returnType parses `: T`, `: x is T` or `: asserts x [is T]`.
func (p *parser) returnType() *syntax.Node {
	colon := p.expect(":")
	if p.atWord("asserts") {
		nt := p.peek()
		if nt.kind == tIdent && !nt.nl {
			kw := p.anon()
			var target *syntax.Node
			if p.peekIs("is") {
				target = p.typePredicate()
			} else if p.atWord("this") {
				target = p.leaf("this")
			} else {
				target = p.leaf("identifier")
			}
			return mk("asserts_annotation", colon, mk("asserts", kw, target))
		}
	}
	if p.tok.kind == tIdent && p.peekIs("is") && !p.peek().nl {
		return mk("type_predicate_annotation", colon, p.typePredicate())
	}
	return mk("type_annotation", colon, p.typ())
}

func (p *parser) typePredicate() *syntax.Node {
	var name *syntax.Node
	if p.atWord("this") {
		name = p.leaf("this")
	} else {
		name = p.leaf("identifier")
	}
	is := p.expect("is")
	return mk("type_predicate", fld("name", name), is, fld("type", p.typ()))
}

func (p *parser) formalParameters() *syntax.Node {
	savedAs := p.asType
	p.asType = false
	defer func() { p.asType = savedAs }()
	kids := []*syntax.Node{p.expect("(")}
	savedASI, savedIn := p.noASI, p.noIn
	p.noASI, p.noIn = true, false
	for !p.atPunct(")") {
		kids = append(kids, p.parameter())
		if !p.atPunct(",") {
			break
		}
		kids = append(kids, p.anon())
	}
	p.noASI, p.noIn = savedASI, savedIn
	kids = append(kids, p.expect(")"))
	return mk("formal_parameters", kids...)
}

func (p *parser) parameter() *syntax.Node {
	var decs []*syntax.Node
	for p.atPunct("@") {
		decs = append(decs, fld("decorator", p.decorator()))
	}
	if !p.ts {
		e := p.patternElement()
		if len(decs) > 0 {
			fail(p.tok.start, "decorated parameter")
		}
		return e
	}
	kids := decs
	for p.tok.kind == tIdent && p.isParamModifier() {
		switch p.tok.text {
		case "public", "private", "protected":
			kids = append(kids, mk("accessibility_modifier", p.anon()))
		case "override":
			kids = append(kids, mk("override_modifier", p.anon()))
		default:
			kids = append(kids, p.anon())
		}
	}
	var pat *syntax.Node
	switch {
	case p.atPunct("..."):
		dots := p.anon()
		pat = mk("rest_pattern", dots, p.bindingTarget())
	case p.atWord("this"):
		pat = p.leaf("this")
	default:
		pat = p.bindingTarget()
	}
	kids = append(kids, fld("pattern", pat))
	typ := "required_parameter"
	if p.atPunct("?") {
		kids = append(kids, p.anon())
		typ = "optional_parameter"
	}
	if p.atPunct(":") {
		kids = append(kids, fld("type", p.typeAnnotation()))
	}
	if p.atPunct("=") {
		kids = append(kids, p.anon(), fld("value", p.assignment()))
	}
	return mk(typ, kids...)
}

func (p *parser) isParamModifier() bool {
	switch p.tok.text {
	case "public", "private", "protected", "override", "readonly":
	default:
		return false
	}
	nt := p.peek()
	return nt.kind == tIdent || nt.kind == tPunct && (nt.text == "{" || nt.text == "[" || nt.text == "...")
}

// ---- arrows ----

func (p *parser) arrowFunction() *syntax.Node {
	t := p.tok
	if p.arrowFail[t.start] {
		return nil
	}
	switch {
	case t.kind == tIdent && !isReservedWord(t.text):
		nt := p.peek()
		if nt.kind == tPunct && nt.text == "=>" {
			param := fld("parameter", p.leaf("identifier"))
			return p.arrowRest([]*syntax.Node{param})
		}
		if t.text != "async" || nt.nl {
			return nil
		}
		if nt.kind == tIdent && !isReservedWord(nt.text) {
			if t2 := p.peek2(); t2.kind == tPunct && t2.text == "=>" {
				async := p.anon()
				param := fld("parameter", p.leaf("identifier"))
				return p.arrowRest([]*syntax.Node{async, param})
			}
			return nil
		}
		if nt.kind == tPunct && (nt.text == "(" || p.ts && nt.text == "<") {
			n := p.try(func() *syntax.Node {
				async := p.anon()
				return p.arrowWithSignature(async)
			})
			if n == nil {
				p.arrowFail[t.start] = true
			}
			return n
		}
		return nil
	case t.kind == tPunct && (t.text == "(" || p.ts && t.text == "<"):
		n := p.try(func() *syntax.Node { return p.arrowWithSignature(nil) })
		if n == nil {
			p.arrowFail[t.start] = true
		}
		return n
	}
	return nil
}

func (p *parser) arrowWithSignature(async *syntax.Node) *syntax.Node {
	kids := []*syntax.Node{async}
	kids = append(kids, p.callSignature()...)
	if !p.atPunct("=>") {
		fail(p.tok.start, "not an arrow")
	}
	return p.arrowRest(kids)
}

func (p *parser) arrowRest(kids []*syntax.Node) *syntax.Node {
	arrow := p.expect("=>")
	kids = append(kids, arrow)
	if p.atPunct("{") {
		savedASI, savedIn := p.noASI, p.noIn
		kids = append(kids, fld("body", p.block(true)))
		p.noASI, p.noIn = savedASI, savedIn
	} else {
		kids = append(kids, fld("body", p.assignment()))
	}
	return mk("arrow_function", kids...)
}

// ---- classes ----

func (p *parser) classDeclaration(decs []*syntax.Node, abstract *syntax.Node) *syntax.Node {
	typ := "class_declaration"
	kids := append([]*syntax.Node{}, decs...)
	if abstract != nil {
		typ = "abstract_class_declaration"
		kids = append(kids, abstract)
	}
	kids = append(kids, p.expect("class"))
	nameType := "identifier"
	if p.ts {
		nameType = "type_identifier"
	}
	if p.tok.kind != tIdent {
		fail(p.tok.start, "expected class name")
	}
	kids = append(kids, fld("name", p.leaf(nameType)))
	kids = append(kids, p.classTail()...)
	if body := kids[len(kids)-1]; p.asiAfterBlock(body.EndByte) {
		kids = append(kids, p.asiMarker(body.EndByte))
	}
	return mk(typ, kids...)
}

func (p *parser) classExpression(decs []*syntax.Node) *syntax.Node {
	kids := append([]*syntax.Node{}, decs...)
	kids = append(kids, p.expect("class"))
	if p.tok.kind == tIdent && !p.atWord("extends") && !(p.ts && p.atWord("implements")) {
		nameType := "identifier"
		if p.ts {
			nameType = "type_identifier"
		}
		kids = append(kids, fld("name", p.leaf(nameType)))
	}
	kids = append(kids, p.classTail()...)
	return mk("class", kids...)
}

func (p *parser) classTail() []*syntax.Node {
	var kids []*syntax.Node
	if p.ts && p.atPunct("<") {
		kids = append(kids, fld("type_parameters", p.typeParameters()))
	}
	if p.atWord("extends") || p.ts && p.atWord("implements") {
		kids = append(kids, p.classHeritage())
	}
	kids = append(kids, fld("body", p.classBody()))
	return kids
}

func (p *parser) classHeritage() *syntax.Node {
	if !p.ts {
		kw := p.anon()
		return mk("class_heritage", kw, p.heritageExpression())
	}
	var kids []*syntax.Node
	if p.atWord("extends") {
		ek := []*syntax.Node{p.anon()}
		for {
			ek = append(ek, fld("value", p.heritageExpression()))
			if p.atPunct("<") {
				ek = append(ek, fld("type_arguments", p.typeArguments()))
			}
			if !p.atPunct(",") {
				break
			}
			ek = append(ek, p.anon())
		}
		kids = append(kids, mk("extends_clause", ek...))
	}
	if p.atWord("implements") {
		ik := []*syntax.Node{p.anon()}
		for {
			ik = append(ik, p.typ())
			if !p.atPunct(",") {
				break
			}
			ik = append(ik, p.anon())
		}
		kids = append(kids, mk("implements_clause", ik...))
	}
	return mk("class_heritage", kids...)
}

// heritageExpression is the `extends` operand: a left-hand-side expression
// (no type arguments; those belong to extends_clause in TypeScript).
func (p *parser) heritageExpression() *syntax.Node {
	saved := p.ts
	e := p.primary()
	p.ts = false
	e = p.chain(e, true)
	p.ts = saved
	return e
}

func (p *parser) classBody() *syntax.Node {
	kids := []*syntax.Node{p.expect("{")}
	savedASI, savedIn := p.noASI, p.noIn
	p.noASI, p.noIn = false, false
	for !p.atPunct("}") {
		if p.tok.kind == tEOF {
			fail(p.tok.start, "unterminated class body")
		}
		if p.atPunct(";") {
			kids = append(kids, p.anon())
			continue
		}
		if p.ts {
			for p.atPunct("@") {
				if !p.decoratorsBeforeMethod() {
					break
				}
				kids = append(kids, fld("decorator", p.decorator()))
			}
		}
		m := p.classMember(false)
		if !p.ts {
			m.Field = "member"
		}
		kids = append(kids, m)
		switch m.Type {
		case "method_definition", "class_static_block":
			if p.atPunct(";") && m.Type == "method_definition" && !p.ts {
				kids = append(kids, p.anon())
			}
		default:
			if p.ts && p.atPunct(",") {
				kids = append(kids, p.anon())
			} else if s := p.semicolon(); s != nil {
				kids = append(kids, s)
			}
		}
	}
	p.noASI, p.noIn = savedASI, savedIn
	kids = append(kids, p.anon())
	return mk("class_body", kids...)
}

// decoratorsBeforeMethod reports whether the decorators starting here
// decorate a method (TS keeps them in class_body) rather than a field.
func (p *parser) decoratorsBeforeMethod() bool {
	s := p.save()
	defer p.restore(s)
	r := p.try(func() *syntax.Node {
		for p.atPunct("@") {
			p.decorator()
		}
		m := p.classMember(false)
		if m.Type != "method_definition" {
			fail(0, "field")
		}
		return m
	})
	return r != nil
}

var memberModifiers = map[string]bool{
	"static": true, "async": true, "get": true, "set": true,
	"public": true, "private": true, "protected": true, "readonly": true, "declare": true,
	"abstract": true, "override": true, "accessor": true,
}

// isMemberModifier reports whether the current word modifies the member
// that follows rather than naming it.
func (p *parser) isMemberModifier(inObject bool) bool {
	if p.tok.kind != tIdent {
		return false
	}
	w := p.tok.text
	if inObject {
		if w != "async" && w != "get" && w != "set" {
			return false
		}
	} else if !memberModifiers[w] || !p.ts && w != "static" && w != "async" && w != "get" && w != "set" {
		return false
	}
	nt := p.peek()
	if w == "async" && nt.nl {
		return false
	}
	switch nt.kind {
	case tIdent, tString, tNumber, tPrivate:
		return true
	case tPunct:
		return nt.text == "[" || nt.text == "*" && w != "get" && w != "set" || nt.text == "{" && w == "static" && !inObject
	}
	return false
}

// classMember parses a class member or (inObject) an object-literal method
// or pair.
func (p *parser) classMember(inObject bool) *syntax.Node {
	var kids []*syntax.Node
	for p.atPunct("@") {
		kids = append(kids, fld("decorator", p.decorator()))
	}
	if !inObject && p.atWord("static") && p.peekIs("{") {
		kw := p.anon()
		return mk("class_static_block", append(kids, kw, fld("body", p.block(false)))...)
	}
	isAbstract := false
	nmods := 0
	for p.isMemberModifier(inObject) {
		nmods++
		switch p.tok.text {
		case "public", "private", "protected":
			kids = append(kids, mk("accessibility_modifier", p.anon()))
		case "override":
			kids = append(kids, mk("override_modifier", p.anon()))
		case "abstract":
			isAbstract = true
			kids = append(kids, p.anon())
		default:
			kids = append(kids, p.anon())
		}
	}
	if p.atPunct("*") {
		nmods++
		kids = append(kids, p.anon())
	}
	if !inObject && p.ts && p.atPunct("[") && p.isIndexSignature() {
		return p.indexSignature(kids)
	}
	name := p.propertyName()
	if inObject && nmods == 0 && p.atPunct(":") {
		colon := p.anon()
		return mk("pair", fld("key", name), colon, fld("value", p.assignment()))
	}
	if inObject && nmods == 0 && name.Type == "property_identifier" && !p.atPunct("(") && !p.atPunct("<") {
		name.Type = "shorthand_property_identifier"
		return name
	}
	kids = append(kids, fld("name", name))
	if p.ts && (p.atPunct("?") || p.atPunct("!") && !inObject) {
		kids = append(kids, p.anon())
	}
	if p.atPunct("(") || p.ts && p.atPunct("<") {
		kids = append(kids, p.callSignature()...)
		if p.atPunct("{") {
			kids = append(kids, fld("body", p.block(false)))
			return mk("method_definition", kids...)
		}
		if !p.ts || inObject {
			fail(p.tok.start, "expected method body")
		}
		if isAbstract {
			return mk("abstract_method_signature", kids...)
		}
		return mk("method_signature", kids...)
	}
	if inObject {
		fail(p.tok.start, "bad object member")
	}
	if p.ts {
		if p.atPunct(":") {
			kids = append(kids, fld("type", p.typeAnnotation()))
		}
	}
	if p.atPunct("=") {
		kids = append(kids, p.anon(), fld("value", p.assignment()))
	}
	if p.ts {
		return mk("public_field_definition", kids...)
	}
	// JS field_definition names its key `property`
	name.Field = "property"
	return mk("field_definition", kids...)
}

// propertyName is `_property_name`.
func (p *parser) propertyName() *syntax.Node {
	switch p.tok.kind {
	case tIdent:
		return p.leaf("property_identifier")
	case tPrivate:
		return p.leaf("private_property_identifier")
	case tString:
		return p.stringLit()
	case tNumber:
		return p.leaf("number")
	case tPunct:
		if p.atPunct("[") {
			open := p.anon()
			e := p.bracketed(p.assignment)
			return mk("computed_property_name", open, e, p.expect("]"))
		}
	}
	fail(p.tok.start, "expected property name, got %q", p.tok.text)
	return nil
}
