package python

import "github.com/matheusAle/coderefinery/internal/syntax"

// match/case patterns (tree-sitter-python case_pattern and friends).

func (p *parser) casePattern() *syntax.Node {
	var children []*syntax.Node
	if p.peek().kind == tkName && !hardKeywords[p.peek().text] && p.peekN(1).isOp("=") {
		name := p.identifier()
		eq := p.anon(p.advance())
		children = one(node("keyword_pattern", append(one(name, eq), p.simpleOrUnion()...)...))
	} else {
		children = p.simpleOrUnion()
	}
	pattern := node("case_pattern", children...)
	for p.atName("as") {
		as := p.anon(p.advance())
		pattern = node("case_pattern", node("as_pattern", pattern, as, p.identifier()))
	}
	return pattern
}

// simpleOrUnion returns the children a simple pattern contributes to its
// parent, or a single union_pattern.
func (p *parser) simpleOrUnion() []*syntax.Node {
	first := p.simplePattern()
	if !p.atOp("|") {
		return first
	}
	children := first
	for p.atOp("|") {
		children = append(children, p.anon(p.advance()))
		children = append(children, p.simplePattern()...)
	}
	return one(node("union_pattern", children...))
}

func (p *parser) patternSequence(typ, closer string) *syntax.Node {
	children := one(p.anon(p.advance()))
	for !p.atOp(closer) {
		children = append(children, p.casePattern())
		if !p.atOp(",") {
			break
		}
		children = append(children, p.anon(p.advance()))
	}
	return node(typ, append(children, p.expectOp(closer))...)
}

func (p *parser) number() *syntax.Node {
	t := p.peek()
	if t.kind != tkNumber {
		fail(t.start, "expected number")
	}
	p.advance()
	return syntax.Leaf(t.typ, true, t.start, t.end)
}

func (p *parser) simplePattern() []*syntax.Node {
	t := p.peek()
	switch {
	case t.isOp("["):
		return one(p.patternSequence("list_pattern", "]"))
	case t.isOp("("):
		return one(p.patternSequence("tuple_pattern", ")"))
	case t.isOp("{"):
		return one(p.dictPattern())
	case t.isOp("*") || t.isOp("**"):
		star := p.anon(p.advance())
		if p.atName("_") {
			return one(node("splat_pattern", star, p.anon(p.advance())))
		}
		return one(node("splat_pattern", star, p.identifier()))
	case t.kind == tkStringStart:
		return one(p.stringOrConcat())
	case t.isOp("-") || t.kind == tkNumber:
		var lead []*syntax.Node
		if t.isOp("-") {
			lead = one(p.anon(p.advance()))
		}
		lead = append(lead, p.number())
		if p.atOp("+") || p.atOp("-") {
			lead = append(lead, p.anon(p.advance()), p.number())
			return one(node("complex_pattern", lead...))
		}
		return lead
	case t.kind == tkName:
		switch t.text {
		case "True", "False", "None":
			return one(p.atom())
		case "_":
			next := p.peekN(1)
			if !next.isOp(".") && !next.isOp("(") {
				return one(p.anon(p.advance()))
			}
		}
		name := p.dottedName()
		if p.atOp("(") {
			children := one(name, p.anon(p.advance()))
			for !p.atOp(")") {
				children = append(children, p.casePattern())
				if !p.atOp(",") {
					break
				}
				children = append(children, p.anon(p.advance()))
			}
			return one(node("class_pattern", append(children, p.expectOp(")"))...))
		}
		return one(name)
	}
	fail(t.start, "unexpected %q in pattern", t.text)
	return nil
}

func (p *parser) dictPattern() *syntax.Node {
	children := one(p.anon(p.advance()))
	for !p.atOp("}") {
		if p.atOp("**") {
			star := p.anon(p.advance())
			children = append(children, node("splat_pattern", star, p.identifier()))
		} else {
			for _, k := range p.simplePattern() {
				children = append(children, field("key", k))
			}
			children = append(children, p.expectOp(":"), field("value", p.casePattern()))
		}
		if !p.atOp(",") {
			break
		}
		children = append(children, p.anon(p.advance()))
	}
	return node("dict_pattern", append(children, p.expectOp("}"))...)
}
