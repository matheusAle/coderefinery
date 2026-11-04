package rust

import "github.com/matheusAle/coderefinery/internal/syntax"

// pattern parses a _pattern including or_pattern (left-associative; a
// leading `|` wraps the first alternative).
func (p *parser) pattern() *syntax.Node {
	var left *syntax.Node
	if p.at("|") {
		bar := p.expect("|")
		left = node("or_pattern", bar, p.patternNoOr())
	} else {
		left = p.patternNoOr()
	}
	for p.at("|") {
		bar := p.expect("|")
		left = node("or_pattern", left, bar, p.patternNoOr())
	}
	return left
}

func (p *parser) patternNoOr() *syntax.Node {
	t := p.peek()
	switch {
	case t.is("_"):
		return p.expect("_")
	case t.is("ref"):
		kw := p.expect("ref")
		return node("ref_pattern", kw, p.patternNoOr())
	case t.is("mut"):
		m := p.mutableSpecifier()
		return node("mut_pattern", m, p.patternNoOr())
	case p.atSplit("&"):
		amp := p.expect("&")
		if p.at("mut") {
			m := p.mutableSpecifier()
			return node("reference_pattern", amp, m, p.patternNoOr())
		}
		return node("reference_pattern", amp, p.patternNoOr())
	case t.is("("):
		return p.patternList("tuple_pattern", "(", ")", true)
	case t.is("["):
		return p.patternList("slice_pattern", "[", "]", false)
	case t.is(".."):
		if p.isPatternListEnd(p.peek2()) {
			return node("remaining_field_pattern", p.expect(".."))
		}
		op := p.expect("..")
		return node("range_pattern", op, field("right", p.rangeEnd()))
	case t.is("..="):
		op := p.expect("..=")
		return node("range_pattern", op, field("right", p.rangeEnd()))
	case t.is("const") && p.peek2().is("{"):
		return node("const_block", p.expect("const"), field("body", p.block()))
	}
	var left *syntax.Node
	switch {
	case t.kind == tkInt, t.kind == tkFloat, t.kind == tkChar, t.kind == tkString, t.kind == tkRawString, t.is("true"), t.is("false"):
		left = p.literal()
	case t.is("-"):
		minus := p.expect("-")
		left = node("negative_literal", minus, p.literal())
	default:
		left = p.pathPattern()
		switch left.Type {
		case "identifier", "scoped_identifier", "generic_pattern":
		default:
			return left
		}
	}
	if p.at("..=") || p.at("...") || p.at("..") {
		op := p.expect(p.peek().text)
		if op.Type == ".." && !p.atRangeEndStart() {
			return node("range_pattern", field("left", left), op)
		}
		return node("range_pattern", field("left", left), op, field("right", p.rangeEnd()))
	}
	return left
}

func (p *parser) isPatternListEnd(t token) bool {
	return t.is(",") || t.is(")") || t.is("]") || t.is("}") || t.is("|") || t.is("=>") || t.is("if") || t.is("=") || t.is(":") || t.kind == tkEOF
}

func (p *parser) atRangeEndStart() bool {
	t := p.peek()
	switch t.kind {
	case tkInt, tkFloat, tkChar, tkString, tkRawString:
		return true
	case tkIdent:
		return !t.is("if")
	}
	return t.is("-") || t.is("::") || t.is("<")
}

func (p *parser) rangeEnd() *syntax.Node {
	t := p.peek()
	switch {
	case t.kind == tkInt, t.kind == tkFloat, t.kind == tkChar, t.kind == tkString, t.kind == tkRawString, t.is("true"), t.is("false"):
		return p.literal()
	case t.is("-"):
		minus := p.expect("-")
		return node("negative_literal", minus, p.literal())
	}
	return p.simplePath()
}

// simplePath parses a _path for patterns (identifiers joined by ::).
func (p *parser) simplePath() *syntax.Node {
	n := p.exprPathStart()
	for p.at("::") && !p.atTurbofish() {
		sep := p.expect("::")
		if n == nil {
			n = node("scoped_identifier", sep, field("name", p.scopedName()))
		} else {
			n = node("scoped_identifier", field("path", n), sep, field("name", p.scopedName()))
		}
	}
	return n
}

func (p *parser) pathPattern() *syntax.Node {
	n := p.simplePath()
	if n.Type == "self" || n.Type == "super" || n.Type == "crate" {
		return n
	}
	var turbofish *syntax.Node
	if p.atTurbofish() {
		sep := p.expect("::")
		args := p.typeArguments()
		turbofish = node("generic_type", field("type", turbofishType(n)), sep, field("type_arguments", args))
		if p.at("{") {
			// tree-sitter recovers with an ERROR here; keep the turbofish type.
			return p.structPattern(turbofish)
		}
		if !p.at("(") {
			return node("generic_pattern", n, sep, field("type_arguments", args))
		}
	}
	switch {
	case p.at("@") && n.Type == "identifier" && turbofish == nil:
		at := p.expect("@")
		return node("captured_pattern", n, at, p.patternNoOr())
	case p.at("("):
		typ := n
		if turbofish != nil {
			typ = turbofish
		}
		list := p.patternList("tuple_struct_pattern", "(", ")", false)
		list.Children = append([]*syntax.Node{field("type", typ)}, list.Children...)
		list.FitChildren()
		return list
	case p.at("{"):
		return p.structPattern(toTypeName(n))
	case p.at("!") && p.isDelimAt(p.peek2()):
		return p.macroInvocation(n)
	}
	return n
}

func (p *parser) patternList(typ, open, close string, tupleElems bool) *syntax.Node {
	parts := []*syntax.Node{p.expect(open)}
	for !p.at(close) {
		if tupleElems && (p.at("|") || p.at("||") || p.at("move")) {
			parts = append(parts, p.closure())
		} else {
			parts = append(parts, p.pattern())
		}
		if !p.at(close) {
			parts = append(parts, p.expect(","))
		}
	}
	parts = append(parts, p.expect(close))
	return node(typ, parts...)
}

func (p *parser) structPattern(typ *syntax.Node) *syntax.Node {
	parts := []*syntax.Node{field("type", typ), p.expect("{")}
	for !p.at("}") {
		parts = append(parts, p.attributes()...)
		if p.at("..") {
			parts = append(parts, node("remaining_field_pattern", p.expect("..")))
		} else {
			var fp []*syntax.Node
			if p.at("ref") {
				fp = append(fp, p.expect("ref"))
			}
			if p.at("mut") {
				fp = append(fp, p.mutableSpecifier())
			}
			if p.peek2().is(":") {
				var name *syntax.Node
				if t := p.peek(); t.kind == tkInt {
					p.next()
					name = syntax.Leaf("integer_literal", true, t.start, t.end)
				} else {
					name = p.identAs("field_identifier")
				}
				fp = append(fp, field("name", name), p.expect(":"), field("pattern", p.pattern()))
			} else {
				fp = append(fp, field("name", p.identAs("shorthand_field_identifier")))
			}
			parts = append(parts, node("field_pattern", fp...))
		}
		if !p.at("}") {
			parts = append(parts, p.expect(","))
		}
	}
	parts = append(parts, p.expect("}"))
	return node("struct_pattern", parts...)
}
