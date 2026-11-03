package typescript

import "github.com/matheusAle/ub-mode/ubgate/internal/syntax"

// Expressions. Precedence follows tree-sitter-javascript's grammar.js
// (`??` sits at the ternary level, below `||`), plus tree-sitter-typescript's
// `as` / `satisfies` / postfix `!`, which take everything to their left up
// to the nearest assignment/ternary.

// expressions is `_expressions`: an expression or a flat sequence_expression.
func (p *parser) expressions() *syntax.Node {
	e := p.assignment()
	if !p.atPunct(",") {
		return e
	}
	kids := []*syntax.Node{e}
	for p.atPunct(",") {
		kids = append(kids, p.anon(), p.assignment())
	}
	return mk("sequence_expression", kids...)
}

var assignOps = map[string]bool{
	"+=": true, "-=": true, "*=": true, "/=": true, "%=": true, "**=": true, "<<=": true, ">>=": true,
	">>>=": true, "&=": true, "|=": true, "^=": true, "&&=": true, "||=": true, "??=": true,
}

// assignment parses an AssignmentExpression (incl. arrows and yield).
func (p *parser) assignment() *syntax.Node {
	if a := p.arrowFunction(); a != nil {
		return a
	}
	if p.atWord("yield") && p.yieldStarts() {
		return p.yieldExpression()
	}
	left := p.conditional()
	if p.atPunct("=") {
		if left.Type == "arrow_function" {
			return left
		}
		eq := p.anon()
		right := p.assignment()
		return mk("assignment_expression", fld("left", toPattern(left)), eq, fld("right", right))
	}
	if p.tok.kind == tPunct && assignOps[p.tok.text] {
		op := fld("operator", p.anon())
		right := p.assignment()
		return mk("augmented_assignment_expression", fld("left", left), op, fld("right", right))
	}
	return left
}

func (p *parser) yieldStarts() bool {
	nt := p.peek()
	switch nt.kind {
	case tIdent, tPunct:
		return !(nt.kind == tIdent && (nt.text == "in" || nt.text == "of" || nt.text == "instanceof")) &&
			!(nt.kind == tPunct && (nt.text == "=" || assignOps[nt.text] || nt.text == "." || nt.text == "?." || nt.text == "=>"))
	}
	return true
}

func (p *parser) yieldExpression() *syntax.Node {
	kw := p.anon()
	if p.atPunct("*") {
		star := p.anon()
		return mk("yield_expression", kw, star, p.assignment())
	}
	if p.tok.nl || !p.startsExpression() {
		return mk("yield_expression", kw)
	}
	return mk("yield_expression", kw, p.assignment())
}

// startsExpression reports whether the current token can begin an
// expression.
func (p *parser) startsExpression() bool {
	switch p.tok.kind {
	case tEOF:
		return false
	case tPunct:
		switch p.tok.text {
		case ")", "]", "}", ",", ";", ":", "=>", "?", "?.", ".", "=", "==", "===", "!=", "!==", "&&", "||", "??",
			"*", "**", "%", "|", "&", "^", ">", ">=", "<=", "<<", ">>", ">>>", "/=", "*=", "+=", "-=":
			return false
		}
	case tIdent:
		switch p.tok.text {
		case "in", "of", "instanceof", "as", "satisfies":
			return false
		}
	}
	return true
}

func (p *parser) conditional() *syntax.Node {
	cond := p.binary(0)
	if !p.atPunct("?") {
		return cond
	}
	q := p.anon()
	savedASI, savedIn := p.noASI, p.noIn
	p.noASI, p.noIn = true, false
	cons := p.assignment()
	p.noASI, p.noIn = savedASI, savedIn
	colon := p.expect(":")
	alt := p.assignment()
	return mk("ternary_expression", fld("condition", cond), q, fld("consequence", cons), colon, fld("alternative", alt))
}

// binary precedence levels (higher binds tighter).
const (
	precAs = iota // TS as / satisfies / postfix `!`
	precNullish
	precOr
	precAnd
	precBitOr
	precBitXor
	precBitAnd
	precEquality
	precRelation
	precShift
	precPlus
	precTimes
	precExp
)

func (p *parser) binaryPrec() int {
	t := p.tok
	if t.kind == tPunct {
		switch t.text {
		case "??":
			return precNullish
		case "||":
			return precOr
		case "&&":
			return precAnd
		case "|":
			return precBitOr
		case "^":
			return precBitXor
		case "&":
			return precBitAnd
		case "==", "!=", "===", "!==":
			return precEquality
		case "<", ">", "<=", ">=":
			return precRelation
		case "<<", ">>", ">>>":
			return precShift
		case "+", "-":
			return precPlus
		case "*", "/", "%":
			return precTimes
		case "**":
			return precExp
		case "!":
			if p.ts && !(t.nl && !p.noASI) {
				return precAs
			}
		}
		return -1
	}
	if t.kind == tIdent {
		switch t.text {
		case "instanceof":
			return precRelation
		case "in":
			if !p.noIn {
				return precRelation
			}
		case "as", "satisfies":
			if p.ts && !(t.nl && !p.noASI) {
				return precAs
			}
		}
	}
	return -1
}

func (p *parser) binary(minPrec int) *syntax.Node {
	left := p.unary()
	return p.binaryRest(left, minPrec)
}

func (p *parser) binaryRest(left *syntax.Node, minPrec int) *syntax.Node {
	for {
		if p.ts && (p.atPunct("<") || p.atPunct("<<")) && p.typeArgumentsAhead() {
			// a type argument list binds like a relational operator (shifting
			// over a pending one: `a < b<T>(c)` keeps the call on b)
			if precRelation+1 < minPrec {
				return left
			}
			left = p.postfix(p.typeArgumentsCall(left))
			continue
		}
		prec := p.binaryPrec()
		if prec < minPrec || prec < 0 {
			return left
		}
		if prec == precAs {
			left = p.asLevel(left)
			continue
		}
		op := fld("operator", p.anon())
		var right *syntax.Node
		if prec == precExp {
			right = p.binary(prec)
		} else {
			right = p.binary(prec + 1)
		}
		left = mk("binary_expression", fld("left", left), op, fld("right", right))
	}
}

// asLevel applies one TS `as T`, `satisfies T` or postfix `!` (which take
// the whole expression to their left in tree-sitter-typescript), or a type
// argument list (`<T>(args)` call, `<T>` instantiation).
func (p *parser) asLevel(left *syntax.Node) *syntax.Node {
	switch p.tok.text {
	case "<", "<<":
		return p.postfix(p.typeArgumentsCall(left))
	case "!":
		bang := p.anon()
		return p.postfix(mk("non_null_expression", left, bang))
	case "as":
		kw := p.anon()
		if p.atWord("const") {
			return mk("as_expression", left, kw, p.anon())
		}
		return p.postfix(mk("as_expression", left, kw, p.asTarget()))
	default:
		kw := p.anon()
		return p.postfix(mk("satisfies_expression", left, kw, p.asTarget()))
	}
}

func (p *parser) unary() *syntax.Node {
	t := p.tok
	if t.kind == tPunct {
		switch t.text {
		case "!", "~", "-", "+":
			op := fld("operator", p.anon())
			return mk("unary_expression", op, fld("argument", p.unary()))
		case "++", "--":
			op := fld("operator", p.anon())
			return mk("update_expression", op, fld("argument", p.unary()))
		case "<":
			if p.ts && !p.jsx {
				ta := p.typeArguments()
				arg := p.unary()
				for (p.atPunct("<") || p.atPunct("<<")) && p.typeArgumentsAhead() {
					arg = p.asLevel(arg)
				}
				return mk("type_assertion", ta, arg)
			}
		}
	}
	if t.kind == tIdent {
		switch t.text {
		case "typeof", "void", "delete":
			op := fld("operator", p.anon())
			return mk("unary_expression", op, fld("argument", p.unary()))
		case "await":
			if !p.ts && p.peekIs("(") {
				if call := p.awaitCall(); call != nil {
					return call
				}
			}
			if p.awaitStarts() {
				kw := p.anon()
				arg := p.unary()
				for p.ts && p.binaryPrec() == precAs {
					arg = p.asLevel(arg)
				}
				return mk("await_expression", kw, arg)
			}
		}
	}
	return p.postfix(p.primary())
}

func (p *parser) awaitStarts() bool {
	nt := p.peek()
	if nt.kind == tEOF {
		return false
	}
	if nt.kind == tPunct {
		switch nt.text {
		case ")", "]", "}", ",", ";", ":", "=", "=>", ".", "?.", "?", "==", "===", "!=", "!==":
			return false
		}
	}
	return true
}

// postfix applies the member/call chain and a postfix ++/--.
func (p *parser) postfix(e *syntax.Node) *syntax.Node {
	e = p.chain(e, true)
	if (p.atPunct("++") || p.atPunct("--")) && !(p.tok.nl && !p.noASI) {
		op := fld("operator", p.anon())
		e = mk("update_expression", fld("argument", e), op)
	}
	return e
}

// chain parses member accesses, subscripts, calls and tagged templates.
// calls=false is the `new` callee (no call arguments).
func (p *parser) chain(e *syntax.Node, calls bool) *syntax.Node {
	for {
		t := p.tok
		if t.kind != tPunct {
			return e
		}
		switch t.text {
		case ".":
			dot := p.anon()
			e = mk("member_expression", fld("object", e), dot, fld("property", p.propertyIdentifierOrPrivate()))
		case "?.":
			if !calls {
				return e
			}
			oc := p.optionalChain()
			switch {
			case p.atPunct("("):
				if p.ts {
					e = mk("call_expression", fld("function", e), oc, fld("arguments", p.arguments()))
				} else {
					e = mk("call_expression", fld("function", e), fld("optional_chain", oc), fld("arguments", p.arguments()))
				}
			case p.atPunct("["):
				open := p.anon()
				idx := p.bracketed(p.expressions)
				e = mk("subscript_expression", fld("object", e), fld("optional_chain", oc), open, fld("index", idx), p.expect("]"))
			case p.ts && p.atPunct("<"):
				ta := p.typeArguments()
				e = mk("call_expression", fld("function", e), oc, fld("type_arguments", ta), fld("arguments", p.arguments()))
			default:
				e = mk("member_expression", fld("object", e), fld("optional_chain", oc), fld("property", p.propertyIdentifierOrPrivate()))
			}
		case "[":
			open := p.anon()
			idx := p.bracketed(p.expressions)
			e = mk("subscript_expression", fld("object", e), open, fld("index", idx), p.expect("]"))
		case "(":
			if !calls {
				return e
			}
			e = mk("call_expression", fld("function", e), fld("arguments", p.arguments()))
		case "`":
			e = mk("call_expression", fld("function", e), fld("arguments", p.template()))
		default:
			return e
		}
	}
}

// optionalChain is the `?.` token: a named optional_chain node in member
// and subscript position, anonymous for TS calls.
func (p *parser) optionalChain() *syntax.Node {
	n := p.anon()
	if p.atPunct("(") && p.ts {
		return n
	}
	if p.ts && p.atPunct("<") {
		return n
	}
	if p.ts {
		// tree-sitter-typescript wraps the token; javascript renames it
		return mk("optional_chain", n)
	}
	n.Type, n.Named = "optional_chain", true
	return n
}

// typeArgumentsCall parses `<T>(...)` (call) or an instantiation
// expression `f<T>`; it fails when the `<` is a comparison (a template after
// `>` is one too: tree-sitter has no typed tagged templates).
func (p *parser) typeArgumentsCall(e *syntax.Node) *syntax.Node {
	saved := p.exprTypeArgs
	p.exprTypeArgs = true
	ta := p.typeArguments()
	p.exprTypeArgs = saved
	if p.atPunct("(") {
		return mk("call_expression", fld("function", e), fld("type_arguments", ta), fld("arguments", p.arguments()))
	}
	if p.startsExpression() && !p.tok.nl || p.atPunct("<") || p.atPunct(">") {
		fail(p.tok.start, "comparison, not type arguments")
	}
	return mk("instantiation_expression", e, fld("type_arguments", ta))
}

// bracketed runs f with ASI disabled and `in` allowed.
func (p *parser) bracketed(f func() *syntax.Node) *syntax.Node {
	savedASI, savedIn := p.noASI, p.noIn
	p.noASI, p.noIn = true, false
	n := f()
	p.noASI, p.noIn = savedASI, savedIn
	return n
}

func (p *parser) propertyIdentifierOrPrivate() *syntax.Node {
	switch p.tok.kind {
	case tPrivate:
		return p.leaf("private_property_identifier")
	case tIdent:
		return p.leaf("property_identifier")
	}
	fail(p.tok.start, "expected property name")
	return nil
}

func (p *parser) arguments() *syntax.Node {
	open := p.expect("(")
	kids := []*syntax.Node{open}
	savedASI, savedIn := p.noASI, p.noIn
	p.noASI, p.noIn = true, false
	for !p.atPunct(")") {
		if p.atPunct(",") {
			kids = append(kids, p.anon())
			continue
		}
		kids = append(kids, p.spreadOrAssignment())
		if !p.atPunct(",") {
			break
		}
		kids = append(kids, p.anon())
	}
	p.noASI, p.noIn = savedASI, savedIn
	kids = append(kids, p.expect(")"))
	return mk("arguments", kids...)
}

func (p *parser) spreadOrAssignment() *syntax.Node {
	if p.atPunct("...") {
		dots := p.anon()
		return mk("spread_element", dots, p.assignment())
	}
	return p.assignment()
}

// ---- primary expressions ----

func (p *parser) primary() *syntax.Node {
	t := p.tok
	switch t.kind {
	case tNumber:
		return p.leaf("number")
	case tString:
		return p.stringLit()
	case tPrivate:
		return p.leaf("private_property_identifier")
	case tPunct:
		switch t.text {
		case "(":
			return p.parenthesized()
		case "[":
			return p.array()
		case "{":
			return p.object()
		case "`":
			return p.template()
		case "/", "/=":
			return p.regex()
		case "<":
			if p.jsx {
				return p.jsxElement()
			}
		case "@":
			// decorated class expression
			var decs []*syntax.Node
			for p.atPunct("@") {
				decs = append(decs, fld("decorator", p.decorator()))
			}
			if p.atWord("class") {
				return p.classExpression(decs)
			}
		}
		fail(t.start, "unexpected %q", t.text)
	case tIdent:
		return p.wordPrimary()
	}
	fail(t.start, "unexpected token %q", t.text)
	return nil
}

func (p *parser) wordPrimary() *syntax.Node {
	t := p.tok
	switch t.text {
	case "this", "super", "null", "true", "false", "undefined":
		return p.leaf(t.text)
	case "function":
		return p.functionExpression(nil)
	case "async":
		if nt := p.peek(); nt.kind == tIdent && nt.text == "function" && !nt.nl {
			return p.functionExpression(p.anon())
		}
	case "class":
		return p.classExpression(nil)
	case "new":
		return p.newExpression()
	case "import":
		if nt := p.peek(); nt.kind == tPunct && nt.text == "." {
			kw := p.anon()
			dot := p.anon()
			return mk("meta_property", kw, dot, p.expect("meta"))
		}
		return mk("import", p.anon())
	}
	if isReservedWord(t.text) {
		fail(t.start, "unexpected keyword %q", t.text)
	}
	return p.leaf("identifier")
}

func (p *parser) newExpression() *syntax.Node {
	kw := p.anon()
	if p.atPunct(".") {
		dot := p.anon()
		return mk("meta_property", kw, dot, p.expect("target"))
	}
	var ctor *syntax.Node
	if p.atWord("new") {
		// `new new A()()`: tree-sitter calls the inner new_expression
		ctor = p.newExpression()
		if p.atPunct("(") {
			ctor = mk("call_expression", fld("function", ctor), fld("arguments", p.arguments()))
		}
	} else {
		ctor = p.chain(p.primary(), false)
	}
	kids := []*syntax.Node{kw, fld("constructor", ctor)}
	if p.ts && p.atPunct("<") {
		if ta := p.try(func() *syntax.Node {
			ta := p.typeArguments()
			return ta
		}); ta != nil {
			kids = append(kids, fld("type_arguments", ta))
		}
	}
	if p.atPunct("(") {
		kids = append(kids, fld("arguments", p.arguments()))
	}
	return mk("new_expression", kids...)
}

func (p *parser) array() *syntax.Node {
	kids := []*syntax.Node{p.expect("[")}
	savedASI, savedIn := p.noASI, p.noIn
	p.noASI, p.noIn = true, false
	for !p.atPunct("]") {
		if p.atPunct(",") {
			kids = append(kids, p.anon())
			continue
		}
		kids = append(kids, p.spreadOrAssignment())
		if !p.atPunct(",") {
			break
		}
		kids = append(kids, p.anon())
	}
	p.noASI, p.noIn = savedASI, savedIn
	kids = append(kids, p.expect("]"))
	return mk("array", kids...)
}

// object parses an object literal. `{ a = 1 }` members (only valid as
// patterns) come out as object_assignment_pattern.
func (p *parser) object() *syntax.Node {
	kids := []*syntax.Node{p.expect("{")}
	savedASI, savedIn := p.noASI, p.noIn
	p.noASI, p.noIn = true, false
	for !p.atPunct("}") {
		if p.atPunct(",") {
			kids = append(kids, p.anon())
			continue
		}
		kids = append(kids, p.objectMember())
		if !p.atPunct(",") {
			break
		}
		kids = append(kids, p.anon())
	}
	p.noASI, p.noIn = savedASI, savedIn
	kids = append(kids, p.expect("}"))
	return mk("object", kids...)
}

func (p *parser) objectMember() *syntax.Node {
	if p.atPunct("...") {
		dots := p.anon()
		return mk("spread_element", dots, p.assignment())
	}
	if p.atPunct("@") {
		return p.classMember(true)
	}
	// shorthand / pair / method
	if p.tok.kind == tIdent {
		nt := p.peek()
		if nt.kind == tPunct {
			switch nt.text {
			case ",", "}":
				return p.leaf("shorthand_property_identifier")
			case "=":
				left := p.leaf("shorthand_property_identifier_pattern")
				eq := p.anon()
				return mk("object_assignment_pattern", fld("left", left), eq, fld("right", p.assignment()))
			}
		}
	}
	return p.classMember(true)
}

func (p *parser) regex() *syntax.Node {
	start := p.tok.start
	i := start + 1
	inClass := false
	for {
		if i >= len(p.src) || p.src[i] == '\n' {
			fail(start, "unterminated regex")
		}
		c := p.src[i]
		if c == '\\' {
			_, w := p.runeAt(i + 1)
			i += 1 + w
			continue
		}
		if inClass {
			if c == ']' {
				inClass = false
			}
		} else if c == '[' {
			inClass = true
		} else if c == '/' {
			break
		}
		i++
	}
	open := syntax.Leaf("/", false, start, start+1)
	pat := fld("pattern", syntax.Leaf("regex_pattern", true, start+1, i))
	closing := syntax.Leaf("/", false, i, i+1)
	j := i + 1
	for j < len(p.src) && (p.src[j] >= 'a' && p.src[j] <= 'z') {
		j++
	}
	var flags *syntax.Node
	if j > i+1 {
		flags = fld("flags", syntax.Leaf("regex_flags", true, i+1, j))
	}
	p.prevEnd = j
	p.tok = p.scan(j)
	return mk("regex", open, pat, closing, flags)
}

// stringLit builds a string node from the current string token.
func (p *parser) stringLit() *syntax.Node {
	if p.tok.kind != tString {
		fail(p.tok.start, "expected string")
	}
	start, end := p.tok.start, p.tok.end
	q := string(p.src[start])
	kids := []*syntax.Node{syntax.Leaf(q, false, start, start+1)}
	i := start + 1
	for i < end-1 {
		if p.src[i] == '\\' {
			e := p.escapeEnd(i)
			kids = append(kids, syntax.Leaf("escape_sequence", true, i, e))
			i = e
			continue
		}
		j := i
		for j < end-1 && p.src[j] != '\\' && p.src[j] != '\n' && p.src[j] != '\r' {
			j++
		}
		kids = append(kids, syntax.Leaf("string_fragment", true, i, j))
		i = j
	}
	kids = append(kids, syntax.Leaf(q, false, end-1, end))
	p.next()
	return mk("string", kids...)
}

// template parses a template_string starting at the current '`' token.
func (p *parser) template() *syntax.Node {
	start := p.tok.start
	kids := []*syntax.Node{syntax.Leaf("`", false, start, start+1)}
	i := start + 1
	for {
		j := i
		for j < len(p.src) && p.src[j] != '`' && p.src[j] != '\\' && !(p.src[j] == '$' && p.byteAt(j+1) == '{') {
			j++
		}
		if j > i {
			kids = append(kids, syntax.Leaf("string_fragment", true, i, j))
		}
		if j >= len(p.src) {
			fail(start, "unterminated template")
		}
		switch p.src[j] {
		case '`':
			kids = append(kids, syntax.Leaf("`", false, j, j+1))
			p.prevEnd = j + 1
			p.tok = p.scan(j + 1)
			return mk("template_string", kids...)
		case '\\':
			e := p.escapeEnd(j)
			kids = append(kids, syntax.Leaf("escape_sequence", true, j, e))
			i = e
		default: // ${
			open := syntax.Leaf("${", false, j, j+2)
			p.prevEnd = j + 2
			p.tok = p.scan(j + 2)
			e := p.bracketed(p.expressions)
			if !p.atPunct("}") {
				fail(p.tok.start, "expected } in template")
			}
			closing := syntax.Leaf("}", false, p.tok.start, p.tok.end)
			kids = append(kids, mk("template_substitution", open, e, closing))
			i = p.tok.end
		}
	}
}

// ---- patterns ----

// toPattern converts an expression parsed as an assignment target into
// its pattern form (array_pattern, object_pattern, ...).
func toPattern(n *syntax.Node) *syntax.Node {
	switch n.Type {
	case "array":
		n.Type = "array_pattern"
		for _, c := range n.Children {
			if c.Named {
				convertElement(c)
			}
		}
	case "object":
		n.Type = "object_pattern"
		for _, c := range n.Children {
			switch c.Type {
			case "pair":
				c.Type = "pair_pattern"
				if v := c.ChildByFieldName("value"); v != nil {
					convertElement(v)
				}
			case "shorthand_property_identifier":
				c.Type = "shorthand_property_identifier_pattern"
			case "spread_element":
				c.Type = "rest_pattern"
				toPattern(c.Children[len(c.Children)-1])
			case "object_assignment_pattern":
				toPattern(c.Children[0])
			}
		}
	}
	return n
}

func convertElement(c *syntax.Node) {
	switch c.Type {
	case "assignment_expression":
		c.Type = "assignment_pattern"
		toPattern(c.Children[0])
	case "spread_element":
		c.Type = "rest_pattern"
		toPattern(c.Children[len(c.Children)-1])
	default:
		toPattern(c)
	}
}

// bindingTarget parses an identifier or destructuring pattern.
func (p *parser) bindingTarget() *syntax.Node {
	switch {
	case p.atPunct("["):
		return p.arrayPattern()
	case p.atPunct("{"):
		return p.objectPattern()
	case p.tok.kind == tIdent:
		return p.leaf("identifier")
	}
	fail(p.tok.start, "expected binding")
	return nil
}

// patternElement parses a pattern (binding or member target) with an
// optional default.
func (p *parser) patternElement() *syntax.Node {
	if p.atPunct("...") {
		dots := p.anon()
		return mk("rest_pattern", dots, p.patternTarget())
	}
	t := p.patternTarget()
	if p.atPunct("=") {
		eq := p.anon()
		return mk("assignment_pattern", fld("left", t), eq, fld("right", p.assignment()))
	}
	return t
}

func (p *parser) patternTarget() *syntax.Node {
	switch {
	case p.atPunct("["):
		return p.arrayPattern()
	case p.atPunct("{"):
		return p.objectPattern()
	}
	e := p.chain(p.primary(), true)
	return e
}

func (p *parser) arrayPattern() *syntax.Node {
	kids := []*syntax.Node{p.expect("[")}
	savedASI, savedIn := p.noASI, p.noIn
	p.noASI, p.noIn = true, false
	for !p.atPunct("]") {
		if p.atPunct(",") {
			kids = append(kids, p.anon())
			continue
		}
		kids = append(kids, p.patternElement())
		if !p.atPunct(",") {
			break
		}
		kids = append(kids, p.anon())
	}
	p.noASI, p.noIn = savedASI, savedIn
	kids = append(kids, p.expect("]"))
	return mk("array_pattern", kids...)
}

func (p *parser) objectPattern() *syntax.Node {
	kids := []*syntax.Node{p.expect("{")}
	savedASI, savedIn := p.noASI, p.noIn
	p.noASI, p.noIn = true, false
	for !p.atPunct("}") {
		if p.atPunct(",") {
			kids = append(kids, p.anon())
			continue
		}
		kids = append(kids, p.objectPatternMember())
		if !p.atPunct(",") {
			break
		}
		kids = append(kids, p.anon())
	}
	p.noASI, p.noIn = savedASI, savedIn
	kids = append(kids, p.expect("}"))
	return mk("object_pattern", kids...)
}

func (p *parser) objectPatternMember() *syntax.Node {
	if p.atPunct("...") {
		dots := p.anon()
		return mk("rest_pattern", dots, p.patternTarget())
	}
	if p.tok.kind == tIdent {
		nt := p.peek()
		if nt.kind == tPunct && (nt.text == "," || nt.text == "}" || nt.text == "=") {
			id := p.leaf("shorthand_property_identifier_pattern")
			if p.atPunct("=") {
				eq := p.anon()
				return mk("object_assignment_pattern", fld("left", id), eq, fld("right", p.assignment()))
			}
			return id
		}
	}
	key := fld("key", p.propertyName())
	colon := p.expect(":")
	return mk("pair_pattern", key, colon, fld("value", p.patternElement()))
}

// awaitCall handles a javascript-grammar quirk: `await (x)` directly
// followed by a call, member or subscript reads as a call of an identifier
// named await. nil when the parenthesis is not followed by one.
func (p *parser) awaitCall() *syntax.Node {
	s := p.save()
	id := p.leaf("identifier")
	args := p.arguments()
	if !(p.atPunct("(") || p.atPunct(".") || p.atPunct("[") || p.atPunct("?.") || p.atPunct("`")) {
		p.restore(s)
		return nil
	}
	return p.postfix(mk("call_expression", fld("function", id), fld("arguments", args)))
}

// typeArgumentsAhead reports whether the `<` here opens type arguments of a
// call or instantiation rather than a comparison.
func (p *parser) typeArgumentsAhead() bool {
	s := p.save()
	ok := p.try(func() *syntax.Node { return p.typeArgumentsCall(mk("identifier")) }) != nil
	p.restore(s)
	return ok
}
