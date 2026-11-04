package java

import "github.com/matheusAle/coderefinery/internal/syntax"

var integralTypes = map[string]bool{"byte": true, "short": true, "int": true, "long": true, "char": true}

func (p *parser) isPrimitive() bool {
	t := p.cur()
	if t.kind != tkIdent {
		return false
	}
	switch t.text {
	case "byte", "short", "int", "long", "char", "float", "double", "boolean", "void":
		return true
	}
	return false
}

// typ is `_type`: an optionally annotated `_unannotated_type`.
func (p *parser) typ() *syntax.Node {
	if p.isOp("@") {
		kids := p.annotations()
		return inner("annotated_type", append(kids, p.unannotatedType())...)
	}
	return p.unannotatedType()
}

// unannotatedType is `_unannotated_type`: a simple type, or array_type.
func (p *parser) unannotatedType() *syntax.Node {
	t := p.simpleType()
	if p.atDimensions() {
		return inner("array_type", field("element", t), field("dimensions", p.dimensions()))
	}
	return t
}

// simpleType is `_simple_type`.
func (p *parser) simpleType() *syntax.Node {
	t := p.cur()
	if t.kind == tkIdent {
		switch {
		case integralTypes[t.text]:
			return inner("integral_type", p.next())
		case t.text == "float" || t.text == "double":
			return inner("floating_point_type", p.next())
		case t.text == "boolean":
			return p.named("boolean_type")
		case t.text == "void":
			return p.named("void_type")
		}
	}
	if !p.isIdent() {
		p.fail("expected a type, found %q", t.text)
	}
	n := p.named("type_identifier")
	for {
		if p.isOp("<") {
			n = inner("generic_type", n, p.typeArguments())
			continue
		}
		if p.isOp(".") && (p.isIdentAt(1) || p.isOpAt(1, "@")) {
			save := p.pos
			dot := p.next()
			anns := p.annotations()
			if !p.isIdent() {
				p.pos = save
				return n
			}
			kids := append([]*syntax.Node{n, dot}, anns...)
			n = inner("scoped_type_identifier", append(kids, p.named("type_identifier"))...)
			continue
		}
		return n
	}
}

func (p *parser) typeArguments() *syntax.Node {
	kids := []*syntax.Node{p.expect("<")}
	if !p.isOp(">") {
		kids = p.commaList(kids, func() *syntax.Node {
			if p.isOp("?") || p.isOp("@") && p.wildcardAfterAnnotations() {
				return p.wildcard()
			}
			return p.typ()
		})
	}
	return inner("type_arguments", append(kids, p.expect(">"))...)
}

func (p *parser) wildcardAfterAnnotations() bool {
	save := p.pos
	defer func() { p.pos = save }()
	return p.try(func() *syntax.Node { p.annotations(); return p.expect("?") }) != nil
}

func (p *parser) wildcard() *syntax.Node {
	kids := p.annotations()
	kids = append(kids, p.expect("?"))
	switch {
	case p.isKw("extends"):
		kids = append(kids, p.next(), p.typ())
	case p.isKw("super"):
		kids = append(kids, p.named("super"), p.typ())
	}
	return inner("wildcard", kids...)
}

// atDimensions: `[ ]` possibly preceded by annotations.
func (p *parser) atDimensions() bool {
	if p.isOp("[") && p.isOpAt(1, "]") {
		return true
	}
	if !p.isOp("@") {
		return false
	}
	save := p.pos
	defer func() { p.pos = save }()
	return p.try(func() *syntax.Node {
		p.annotations()
		if p.isOp("[") && p.isOpAt(1, "]") {
			return p.next()
		}
		return nil
	}) != nil
}

func (p *parser) dimensions() *syntax.Node {
	var kids []*syntax.Node
	for p.atDimensions() {
		kids = append(kids, p.annotations()...)
		kids = append(kids, p.next(), p.next())
	}
	return inner("dimensions", kids...)
}

func (p *parser) typeParameters() *syntax.Node {
	kids := []*syntax.Node{p.expect("<")}
	kids = p.commaList(kids, func() *syntax.Node {
		kids := p.annotations()
		if !p.isIdent() {
			p.fail("expected type parameter")
		}
		kids = append(kids, p.named("type_identifier"))
		if p.isKw("extends") {
			bound := []*syntax.Node{p.next(), p.typ()}
			for p.isOp("&") {
				bound = append(bound, p.next(), p.typ())
			}
			kids = append(kids, inner("type_bound", bound...))
		}
		return inner("type_parameter", kids...)
	})
	return inner("type_parameters", append(kids, p.expect(">"))...)
}

func (p *parser) typeList() *syntax.Node {
	return inner("type_list", p.commaList(nil, p.typ)...)
}

func isPrimitiveType(n *syntax.Node) bool {
	switch n.Type {
	case "integral_type", "floating_point_type", "boolean_type", "void_type":
		return true
	}
	return false
}
