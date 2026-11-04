package java

import "github.com/matheusAle/coderefinery/internal/syntax"

var modifierWords = map[string]bool{
	"public": true, "protected": true, "private": true, "abstract": true, "static": true, "final": true,
	"strictfp": true, "default": true, "synchronized": true, "native": true, "transient": true,
	"volatile": true, "sealed": true, "non-sealed": true,
}

// isModifier reports whether the current token is a modifier keyword in a
// declaration position.
func (p *parser) isModifier() bool {
	t := p.cur()
	if t.kind != tkIdent || !modifierWords[t.text] {
		return false
	}
	switch t.text {
	case "synchronized":
		return !p.isOpAt(1, "(")
	case "static":
		return !p.isOpAt(1, "{")
	case "default":
		return !p.isOpAt(1, ":") && !p.isOpAt(1, "->")
	case "sealed":
		n := p.peek(1)
		return n.kind == tkIdent && (modifierWords[n.text] || n.text == "class" || n.text == "interface") || n.kind == tkOp && n.text == "@"
	}
	return true
}

func (p *parser) isAnnotation() bool { return p.isOp("@") }

// annotations parses zero or more annotations.
func (p *parser) annotations() []*syntax.Node {
	var kids []*syntax.Node
	for p.isAnnotation() {
		kids = append(kids, p.annotation())
	}
	return kids
}

// modifiers parses the `modifiers` node, or nil when there is none.
func (p *parser) modifiers() *syntax.Node {
	var kids []*syntax.Node
	for {
		if p.isAnnotation() {
			kids = append(kids, p.annotation())
		} else if p.isModifier() {
			kids = append(kids, p.next())
		} else {
			break
		}
	}
	if len(kids) == 0 {
		return nil
	}
	return inner("modifiers", kids...)
}

// name is `_name`: identifier or left-nested scoped_identifier.
func (p *parser) name() *syntax.Node {
	n := p.identifier()
	for p.isOp(".") && p.isIdentAt(1) {
		dot := p.next()
		n = inner("scoped_identifier", field("scope", n), dot, field("name", p.identifier()))
	}
	return n
}

func (p *parser) annotation() *syntax.Node {
	at := p.expect("@")
	name := field("name", p.name())
	if !p.isOp("(") {
		return inner("marker_annotation", at, name)
	}
	kids := []*syntax.Node{p.next()}
	if !p.isOp(")") {
		if p.isIdent() && p.isOpAt(1, "=") {
			kids = p.commaList(kids, func() *syntax.Node {
				key := field("key", p.identifier())
				eq := p.expect("=")
				return inner("element_value_pair", key, eq, field("value", p.elementValue()))
			})
		} else {
			kids = append(kids, p.elementValue())
		}
	}
	kids = append(kids, p.expect(")"))
	return inner("annotation", at, name, field("arguments", inner("annotation_argument_list", kids...)))
}

func (p *parser) elementValue() *syntax.Node {
	switch {
	case p.isOp("{"):
		kids := []*syntax.Node{p.next()}
		for !p.isOp("}") {
			kids = append(kids, p.elementValue())
			if !p.isOp(",") {
				break
			}
			kids = append(kids, p.next())
		}
		return inner("element_value_array_initializer", append(kids, p.expect("}"))...)
	case p.isAnnotation():
		return p.annotation()
	}
	return p.expression()
}

// isTypeDeclStart: class / interface / enum / @interface / record NAME.
func (p *parser) isTypeDeclStart() bool {
	t := p.cur()
	if t.kind == tkOp {
		return t.text == "@interface"
	}
	switch t.text {
	case "class", "interface", "enum":
		return true
	case "record":
		return p.isIdentAt(1) && (p.isOpAt(2, "(") || p.isOpAt(2, "<"))
	}
	return false
}

// typeDecl parses a class/interface/enum/record/annotation declaration
// after its (possibly nil) modifiers.
func (p *parser) typeDecl(mods *syntax.Node) *syntax.Node {
	kw := p.cur().text
	kids := []*syntax.Node{mods, p.next(), field("name", p.identifier())}
	switch kw {
	case "class":
		if p.isOp("<") {
			kids = append(kids, field("type_parameters", p.typeParameters()))
		}
		if p.isKw("extends") {
			kids = append(kids, field("superclass", inner("superclass", p.next(), p.typ())))
		}
		kids = append(kids, p.superInterfaces(), p.permits(), field("body", p.classBody()))
		return inner("class_declaration", kids...)
	case "interface":
		if p.isOp("<") {
			kids = append(kids, field("type_parameters", p.typeParameters()))
		}
		if p.isKw("extends") {
			kids = append(kids, inner("extends_interfaces", p.next(), p.typeList()))
		}
		kids = append(kids, p.permits(), field("body", p.interfaceBody()))
		return inner("interface_declaration", kids...)
	case "enum":
		kids = append(kids, p.superInterfaces(), field("body", p.enumBody()))
		return inner("enum_declaration", kids...)
	case "record":
		if p.isOp("<") {
			kids = append(kids, field("type_parameters", p.typeParameters()))
		}
		kids = append(kids, field("parameters", p.formalParameters()), p.superInterfaces(), field("body", p.classBody()))
		return inner("record_declaration", kids...)
	}
	kids = append(kids, field("body", p.annotationTypeBody()))
	return inner("annotation_type_declaration", kids...)
}

func (p *parser) superInterfaces() *syntax.Node {
	if !p.isKw("implements") {
		return nil
	}
	return field("interfaces", inner("super_interfaces", p.next(), p.typeList()))
}

func (p *parser) permits() *syntax.Node {
	if !p.isKw("permits") {
		return nil
	}
	return field("permits", inner("permits", p.next(), p.typeList()))
}

func (p *parser) classBody() *syntax.Node {
	kids := []*syntax.Node{p.expect("{")}
	for !p.isOp("}") {
		kids = append(kids, p.member(false))
	}
	return inner("class_body", append(kids, p.next())...)
}

func (p *parser) interfaceBody() *syntax.Node {
	kids := []*syntax.Node{p.expect("{")}
	for !p.isOp("}") {
		kids = append(kids, p.member(true))
	}
	return inner("interface_body", append(kids, p.next())...)
}

// member is `_class_body_declaration` (interface bodies: constants
// instead of fields, no initializers or constructors).
func (p *parser) member(iface bool) *syntax.Node {
	switch {
	case p.isOp(";"):
		return p.next()
	case p.isOp("{") && !iface:
		return p.block()
	case p.isKw("static") && p.isOpAt(1, "{") && !iface:
		return inner("static_initializer", p.next(), p.block())
	}
	mods := p.modifiers()
	if p.isTypeDeclStart() {
		return p.typeDecl(mods)
	}
	var tparams []*syntax.Node
	if p.isOp("<") {
		tparams = append(tparams, field("type_parameters", p.typeParameters()))
		if !iface && p.isIdent() && p.isOpAt(1, "(") {
			return p.constructor(mods, tparams)
		}
		tparams = append(tparams, p.annotations()...)
	}
	if !iface && tparams == nil {
		if p.isIdent() && p.isOpAt(1, "(") {
			return p.constructor(mods, nil)
		}
		if p.isIdent() && p.isOpAt(1, "{") {
			return inner("compact_constructor_declaration", mods, field("name", p.identifier()), field("body", p.block()))
		}
	}
	typ := field("type", p.unannotatedType())
	if tparams != nil || p.isIdent() && p.isOpAt(1, "(") {
		return p.methodRest(mods, tparams, typ)
	}
	kids := []*syntax.Node{mods, typ}
	kids = append(kids, p.declarators()...)
	kids = append(kids, p.expect(";"))
	if iface {
		return inner("constant_declaration", kids...)
	}
	return inner("field_declaration", kids...)
}

func (p *parser) methodRest(mods *syntax.Node, tparams []*syntax.Node, typ *syntax.Node) *syntax.Node {
	kids := append([]*syntax.Node{mods}, tparams...)
	kids = append(kids, typ, field("name", p.identifier()), field("parameters", p.formalParameters()))
	if p.atDimensions() {
		kids = append(kids, field("dimensions", p.dimensions()))
	}
	kids = append(kids, p.throws())
	if p.isOp(";") {
		kids = append(kids, p.next())
	} else {
		kids = append(kids, field("body", p.block()))
	}
	return inner("method_declaration", kids...)
}

func (p *parser) throws() *syntax.Node {
	if !p.isKw("throws") {
		return nil
	}
	return inner("throws", p.commaList([]*syntax.Node{p.next()}, p.typ)...)
}

func (p *parser) constructor(mods *syntax.Node, tparams []*syntax.Node) *syntax.Node {
	kids := append([]*syntax.Node{mods}, tparams...)
	kids = append(kids, field("name", p.identifier()), field("parameters", p.formalParameters()), p.throws())
	body := []*syntax.Node{p.expect("{")}
	if eci := p.try(p.explicitConstructorInvocation); eci != nil {
		body = append(body, eci)
	}
	for !p.isOp("}") {
		body = append(body, p.statement(false))
	}
	body = append(body, p.next())
	kids = append(kids, field("body", inner("constructor_body", body...)))
	return inner("constructor_declaration", kids...)
}

func (p *parser) explicitConstructorInvocation() *syntax.Node {
	var kids []*syntax.Node
	if p.isOp("<") || p.isKw("this") || p.isKw("super") {
		if p.isOp("<") {
			kids = append(kids, field("type_arguments", p.typeArguments()))
		}
		if !p.isKw("this") && !p.isKw("super") {
			return nil
		}
		kids = append(kids, field("constructor", p.named(p.cur().text)))
	} else {
		obj := p.primary()
		kids = append(kids, field("object", obj), p.expect("."))
		if p.isOp("<") {
			kids = append(kids, field("type_arguments", p.typeArguments()))
		}
		if !p.isKw("super") {
			return nil
		}
		kids = append(kids, field("constructor", p.named("super")))
	}
	if !p.isOp("(") {
		return nil
	}
	kids = append(kids, field("arguments", p.argumentList()), p.expect(";"))
	return inner("explicit_constructor_invocation", kids...)
}

func (p *parser) enumBody() *syntax.Node {
	kids := []*syntax.Node{p.expect("{")}
	for !p.isOp("}") && !p.isOp(";") {
		if p.isOp(",") {
			kids = append(kids, p.next())
			continue
		}
		c := []*syntax.Node{p.modifiers(), field("name", p.identifier())}
		if p.isOp("(") {
			c = append(c, field("arguments", p.argumentList()))
		}
		if p.isOp("{") {
			c = append(c, field("body", p.classBody()))
		}
		kids = append(kids, inner("enum_constant", c...))
	}
	if p.isOp(";") {
		decls := []*syntax.Node{p.next()}
		for !p.isOp("}") {
			decls = append(decls, p.member(false))
		}
		kids = append(kids, inner("enum_body_declarations", decls...))
	}
	return inner("enum_body", append(kids, p.expect("}"))...)
}

func (p *parser) annotationTypeBody() *syntax.Node {
	kids := []*syntax.Node{p.expect("{")}
	for !p.isOp("}") {
		if p.isOp(";") {
			kids = append(kids, p.next())
			continue
		}
		mods := p.modifiers()
		if p.isTypeDeclStart() {
			kids = append(kids, p.typeDecl(mods))
			continue
		}
		typ := field("type", p.unannotatedType())
		if p.isIdent() && p.isOpAt(1, "(") {
			e := []*syntax.Node{mods, typ, field("name", p.identifier()), p.expect("("), p.expect(")")}
			if p.atDimensions() {
				e = append(e, field("dimensions", p.dimensions()))
			}
			if p.isKw("default") {
				e = append(e, p.next(), field("value", p.elementValue()))
			}
			kids = append(kids, inner("annotation_type_element_declaration", append(e, p.expect(";"))...))
			continue
		}
		c := append([]*syntax.Node{mods, typ}, p.declarators()...)
		kids = append(kids, inner("constant_declaration", append(c, p.expect(";"))...))
	}
	return inner("annotation_type_body", append(kids, p.next())...)
}

// declarators is `_variable_declarator_list`.
func (p *parser) declarators() []*syntax.Node {
	return p.commaList(nil, func() *syntax.Node {
		kids := p.declaratorID()
		if p.isOp("=") {
			kids = append(kids, p.next(), field("value", p.variableInitializer()))
		}
		return field("declarator", inner("variable_declarator", kids...))
	})
}

// declaratorID is `_variable_declarator_id`: name and optional dimensions.
func (p *parser) declaratorID() []*syntax.Node {
	var name *syntax.Node
	if p.isKw("_") {
		name = p.named("underscore_pattern")
	} else {
		name = p.identifier()
	}
	kids := []*syntax.Node{field("name", name)}
	if p.atDimensions() {
		kids = append(kids, field("dimensions", p.dimensions()))
	}
	return kids
}

func (p *parser) variableInitializer() *syntax.Node {
	if p.isOp("{") {
		return p.arrayInitializer()
	}
	return p.expression()
}

func (p *parser) arrayInitializer() *syntax.Node {
	kids := []*syntax.Node{p.expect("{")}
	for !p.isOp("}") {
		kids = append(kids, p.variableInitializer())
		if !p.isOp(",") {
			break
		}
		kids = append(kids, p.next())
	}
	return inner("array_initializer", append(kids, p.expect("}"))...)
}

func (p *parser) formalParameters() *syntax.Node {
	kids := []*syntax.Node{p.expect("(")}
	if !p.isOp(")") {
		kids = p.commaList(kids, p.formalParameter)
	}
	return inner("formal_parameters", append(kids, p.expect(")"))...)
}

func (p *parser) formalParameter() *syntax.Node {
	if rp := p.try(p.receiverParameter); rp != nil {
		return rp
	}
	mods := p.modifiers()
	typ := p.unannotatedType()
	if p.isOp("...") || p.isOp("@") {
		kids := []*syntax.Node{mods, typ}
		kids = append(kids, p.annotations()...)
		kids = append(kids, p.expect("..."))
		kids = append(kids, inner("variable_declarator", p.declaratorID()...))
		return inner("spread_parameter", kids...)
	}
	return inner("formal_parameter", append([]*syntax.Node{mods, field("type", typ)}, p.declaratorID()...)...)
}

func (p *parser) receiverParameter() *syntax.Node {
	kids := p.annotations()
	kids = append(kids, p.unannotatedType())
	for p.isIdent() && p.isOpAt(1, ".") {
		kids = append(kids, p.identifier(), p.next())
	}
	if !p.isKw("this") {
		return nil
	}
	kids = append(kids, p.named("this"))
	return inner("receiver_parameter", kids...)
}
