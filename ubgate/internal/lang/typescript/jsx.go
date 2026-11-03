package typescript

import "github.com/matheusAle/ub-mode/ubgate/internal/syntax"

// JSX (javascript and tsx grammars). JSX is scanned from byte offsets:
// element names may contain `-`, and text between tags is the scanner's
// jsx_text token, which the ordinary lexer cannot produce.

// jsxElement parses the element starting at the current `<` token and
// resumes ordinary scanning after it.
func (p *parser) jsxElement() *syntax.Node {
	n := p.jsxElementAt(p.tok.start)
	p.prevEnd = n.EndByte
	p.tok = p.scan(n.EndByte)
	return n
}

func (p *parser) jsxElementAt(start int) *syntax.Node {
	lt := syntax.Leaf("<", false, start, start+1)
	i, _ := p.skipTrivia(start + 1)
	if p.byteAt(i) == '>' {
		open := mk("jsx_opening_element", lt, syntax.Leaf(">", false, i, i+1))
		return p.jsxChildren(open, i+1, false)
	}
	name, i := p.jsxElementName(i, true)
	kids := []*syntax.Node{lt, fld("name", name)}
	for {
		i, _ = p.skipTrivia(i)
		c := p.byteAt(i)
		switch {
		case c == '/' && p.byteAt(i+1) == '>':
			kids = append(kids, syntax.Leaf("/>", false, i, i+2))
			return mk("jsx_self_closing_element", kids...)
		case c == '>':
			kids = append(kids, syntax.Leaf(">", false, i, i+1))
			return p.jsxChildren(mk("jsx_opening_element", kids...), i+1, true)
		case c == '{':
			e := p.jsxExpressionAt(i)
			kids = append(kids, fld("attribute", e))
			i = e.EndByte
		case c == '<' && p.ts:
			// type arguments on the element name
			p.tok = p.scan(i)
			ta := p.typeArguments()
			kids = append(kids, fld("type_arguments", ta))
			i = ta.EndByte
		default:
			var attr *syntax.Node
			attr, i = p.jsxAttribute(i)
			kids = append(kids, fld("attribute", attr))
		}
	}
}

// jsxChildren parses children after an opening tag ending at i, then the
// closing tag, and builds the jsx_element.
func (p *parser) jsxChildren(open *syntax.Node, i int, named bool) *syntax.Node {
	kids := []*syntax.Node{fld("open_tag", open)}
	for {
		j, sawText := p.jsxTextEnd(i)
		if sawText {
			kids = append(kids, syntax.Leaf("jsx_text", true, i, j))
		}
		i = j
		if i >= len(p.src) {
			fail(i, "unterminated JSX element")
		}
		switch p.src[i] {
		case '<':
			k, _ := p.skipTrivia(i + 1)
			if p.byteAt(k) == '/' {
				closing := p.jsxClosing(i, k+1, named)
				kids = append(kids, fld("close_tag", closing))
				return mk("jsx_element", kids...)
			}
			child := p.jsxElementAt(i)
			kids = append(kids, child)
			i = child.EndByte
		case '{':
			e := p.jsxExpressionAt(i)
			kids = append(kids, e)
			i = e.EndByte
		case '&':
			e := p.charRefEnd(i)
			if e < 0 {
				fail(i, "bad character reference")
			}
			kids = append(kids, syntax.Leaf("html_character_reference", true, i, e))
			i = e
		default:
			fail(i, "unexpected %q in JSX text", p.src[i])
		}
	}
}

// jsxTextEnd emulates the scanner's jsx_text: it runs to the next
// `< > { } &`; it counts as text unless it is only whitespace in which every
// run starts at a line break.
func (p *parser) jsxTextEnd(i int) (int, bool) {
	sawText, atNewline := false, false
	for i < len(p.src) {
		r, w := p.runeAt(i)
		if r == '<' || r == '>' || r == '{' || r == '}' || r == '&' {
			break
		}
		if r == '\n' {
			atNewline = true
		} else {
			atNewline = atNewline && isSpace(r)
			if !atNewline {
				sawText = true
			}
		}
		i += w
	}
	return i, sawText
}

func (p *parser) jsxClosing(lt, i int, named bool) *syntax.Node {
	kids := []*syntax.Node{syntax.Leaf("</", false, lt, i)}
	i, _ = p.skipTrivia(i)
	if named {
		var name *syntax.Node
		name, i = p.jsxElementName(i, true)
		kids = append(kids, fld("name", name))
		i, _ = p.skipTrivia(i)
	}
	if p.byteAt(i) != '>' {
		fail(i, "expected > in closing tag")
	}
	kids = append(kids, syntax.Leaf(">", false, i, i+1))
	return mk("jsx_closing_element", kids...)
}

func (p *parser) jsxIdentEnd(i int) int {
	j := i
	for j < len(p.src) {
		r, w := p.runeAt(j)
		if !(isIDPart(r) && r != '\\' || r == '-') {
			break
		}
		j += w
	}
	if j == i {
		fail(i, "expected JSX identifier")
	}
	return j
}

// jsxElementName parses identifier, member_expression (A.B.C) or
// jsx_namespace_name (a:b); element=false gives an attribute name.
func (p *parser) jsxElementName(i int, element bool) (*syntax.Node, int) {
	j := p.jsxIdentEnd(i)
	first := syntax.Leaf("identifier", true, i, j)
	k, _ := p.skipTrivia(j)
	switch p.byteAt(k) {
	case ':':
		l, _ := p.skipTrivia(k + 1)
		m := p.jsxIdentEnd(l)
		return mk("jsx_namespace_name", first, syntax.Leaf(":", false, k, k+1), syntax.Leaf("identifier", true, l, m)), m
	case '.':
		if !element {
			break
		}
		e := first
		for p.byteAt(k) == '.' {
			l, _ := p.skipTrivia(k + 1)
			m := p.jsxIdentEnd(l)
			prop := fld("property", syntax.Leaf("property_identifier", true, l, m))
			e = mk("member_expression", fld("object", e), syntax.Leaf(".", false, k, k+1), prop)
			j = m
			k, _ = p.skipTrivia(m)
		}
		return e, j
	}
	if !element {
		first.Type = "property_identifier"
	}
	return first, j
}

func (p *parser) jsxAttribute(i int) (*syntax.Node, int) {
	name, j := p.jsxElementName(i, false)
	k, _ := p.skipTrivia(j)
	if p.byteAt(k) != '=' {
		return mk("jsx_attribute", name), j
	}
	eq := syntax.Leaf("=", false, k, k+1)
	l, _ := p.skipTrivia(k + 1)
	var value *syntax.Node
	switch p.byteAt(l) {
	case '"', '\'':
		value = p.jsxString(l)
	case '{':
		value = p.jsxExpressionAt(l)
	case '<':
		value = p.jsxElementAt(l)
	default:
		fail(l, "bad JSX attribute value")
	}
	return mk("jsx_attribute", name, eq, value), value.EndByte
}

func (p *parser) jsxString(i int) *syntax.Node {
	q := p.src[i]
	kids := []*syntax.Node{syntax.Leaf(string(q), false, i, i+1)}
	j := i + 1
	for {
		if j >= len(p.src) {
			fail(i, "unterminated JSX string")
		}
		if p.src[j] == q {
			kids = append(kids, syntax.Leaf(string(q), false, j, j+1))
			return mk("string", kids...)
		}
		if p.src[j] == '&' {
			if e := p.charRefEnd(j); e > 0 {
				kids = append(kids, syntax.Leaf("html_character_reference", true, j, e))
				j = e
				continue
			}
		}
		k := j
		for k < len(p.src) && p.src[k] != q {
			if p.src[k] == '&' && p.charRefEnd(k) > 0 {
				break
			}
			k++
		}
		kids = append(kids, syntax.Leaf("string_fragment", true, j, k))
		j = k
	}
}

// charRefEnd matches html_character_reference at i (`&name;`, `&#10;`,
// `&#x1F;`); -1 when there is none.
func (p *parser) charRefEnd(i int) int {
	j := i + 1
	isHex := func(b byte) bool { return isDigitFor(b, 16) }
	isDec := func(b byte) bool { return b >= '0' && b <= '9' }
	isAlpha := func(b byte) bool { return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' }
	switch {
	case p.byteAt(j) == '#' && (p.byteAt(j+1) == 'x' || p.byteAt(j+1) == 'X'):
		k := j + 2
		for k < j+8 && isHex(p.byteAt(k)) {
			k++
		}
		if k > j+2 && p.byteAt(k) == ';' {
			return k + 1
		}
	case p.byteAt(j) == '#':
		k := j + 1
		for k < j+6 && isDec(p.byteAt(k)) {
			k++
		}
		if k > j+1 && p.byteAt(k) == ';' {
			return k + 1
		}
	case isAlpha(p.byteAt(j)):
		k := j
		for k < j+30 && (isAlpha(p.byteAt(k)) || isDec(p.byteAt(k))) {
			k++
		}
		if p.byteAt(k) == ';' {
			return k + 1
		}
	}
	return -1
}

// jsxExpressionAt parses `{ expression }`, `{ ...spread }` or `{}` at i.
// It leaves p.tok on the closing brace; the caller resumes at EndByte.
func (p *parser) jsxExpressionAt(i int) *syntax.Node {
	p.tok = p.scan(i)
	open := p.expect("{")
	kids := []*syntax.Node{open}
	savedASI, savedIn := p.noASI, p.noIn
	p.noASI, p.noIn = true, false
	switch {
	case p.atPunct("}"):
	case p.atPunct("..."):
		dots := p.anon()
		kids = append(kids, mk("spread_element", dots, p.assignment()))
	default:
		kids = append(kids, p.expressions())
	}
	p.noASI, p.noIn = savedASI, savedIn
	if !p.atPunct("}") {
		fail(p.tok.start, "expected } in JSX")
	}
	kids = append(kids, syntax.Leaf("}", false, p.tok.start, p.tok.end))
	return mk("jsx_expression", kids...)
}
