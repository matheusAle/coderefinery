package python

import "github.com/matheusAle/ub-mode/ubgate/internal/syntax"

// tree-sitter-python precedences (grammar.js PREC). Note xor (16) binds
// tighter than bitwise and (15): `a & b ^ c` is `a & (b ^ c)` in the tree.
const (
	precConditional = -1
	precOr          = 10
	precAnd         = 11
	precNot         = 12
	precCompare     = 13
	precBitOr       = 14
	precBitAnd      = 15
	precXor         = 16
	precShift       = 17
	precPlus        = 18
	precTimes       = 19
	precUnary       = 20
	precPower       = 21
)

var binaryPrec = map[string]int{
	"|": precBitOr, "&": precBitAnd, "^": precXor, "<<": precShift, ">>": precShift,
	"+": precPlus, "-": precPlus, "*": precTimes, "@": precTimes, "/": precTimes, "%": precTimes, "//": precTimes,
	"**": precPower,
}

var compareOps = map[string]bool{"<": true, "<=": true, "==": true, "!=": true, ">=": true, ">": true, "<>": true}

var hardKeywords = map[string]bool{
	"and": true, "as": true, "assert": true, "break": true, "class": true, "continue": true, "def": true,
	"del": true, "elif": true, "else": true, "except": true, "finally": true, "for": true, "from": true,
	"global": true, "if": true, "import": true, "in": true, "is": true, "lambda": true, "nonlocal": true,
	"not": true, "or": true, "pass": true, "raise": true, "return": true, "try": true, "while": true,
	"with": true, "yield": true, "True": true, "False": true, "None": true,
}

func isHardKeyword(s string) bool {
	return hardKeywords[s] && s != "True" && s != "False" && s != "None"
}

func isOperatorWord(s string) bool {
	return s == "and" || s == "or" || s == "in" || s == "is" || s == "if" || s == "else" || s == "for" || s == "not"
}

// startsExpression reports whether the next token can begin an expression.
func (p *parser) startsExpression() bool {
	t := p.peek()
	switch t.kind {
	case tkName:
		return !hardKeywords[t.text] || t.text == "not" || t.text == "lambda" || t.text == "True" ||
			t.text == "False" || t.text == "None" || t.text == "yield"
	case tkNumber, tkStringStart:
		return true
	case tkOp:
		switch t.text {
		case "(", "[", "{", "-", "+", "~", "*", "**", "...":
			return true
		}
	}
	return false
}

func (p *parser) identifier() *syntax.Node {
	t := p.peek()
	if t.kind != tkName || hardKeywords[t.text] {
		fail(t.start, "expected identifier, found %q", t.text)
	}
	p.advance()
	return syntax.Leaf("identifier", true, t.start, t.end)
}

// expression is tree-sitter's `expression` (no as_pattern).
func (p *parser) expression() *syntax.Node { return p.expr(precConditional-1, false) }

// expressionAs also accepts `expr as target` (with items, except clauses).
func (p *parser) expressionAs() *syntax.Node { return p.expr(precConditional-1, true) }

// expressions is `_expressions`: an expression or an expression_list.
func (p *parser) expressions() *syntax.Node {
	first, _ := p.starExpressionList("expression_list")
	return first
}

// starExpressionList parses comma-separated expressions (with *splat);
// more than one (or a trailing comma) becomes listType.
func (p *parser) starExpressionList(listType string) (*syntax.Node, bool) {
	first := p.starExpression()
	if !p.atOp(",") {
		return first, false
	}
	children := one(first)
	for p.atOp(",") {
		children = append(children, p.anon(p.advance()))
		if !p.startsExpression() || p.atName("yield") {
			break
		}
		children = append(children, p.starExpression())
	}
	return node(listType, children...), true
}

func (p *parser) starExpression() *syntax.Node {
	if p.atOp("*") {
		return p.tightSplat()
	}
	return p.expression()
}

func (p *parser) expr(minPrec int, allowAs bool) *syntax.Node {
	return p.infix(p.prefix(allowAs), minPrec, allowAs)
}

// tightSplat parses "*atom…" where tree-sitter binds the splat to the atom
// alone (list, set and expression_list elements): `[*a.b]` is
// attribute(list_splat(a), b) and postfix/binary operators continue on the
// list_splat node.
func (p *parser) tightSplat() *syntax.Node {
	star := p.anon(p.advance())
	left := p.postfix(node("list_splat", star, p.atom()))
	return p.infix(left, precConditional-1, false)
}

func (p *parser) infix(left *syntax.Node, minPrec int, allowAs bool) *syntax.Node {
	for {
		t := p.peek()
		switch {
		case t.isName("if") && minPrec < precConditional:
			var cond *syntax.Node
			if !p.attempt(func() {
				ifTok := p.anon(p.advance())
				middle := p.expression()
				elseTok := p.expectKw("else")
				right := p.expr(precConditional-1, false)
				cond = node("conditional_expression", left, ifTok, middle, elseTok, right)
			}) {
				return left
			}
			left = cond
		case t.isName("as") && allowAs && minPrec < 0:
			as := p.anon(p.advance())
			target := node("as_pattern_target", p.expr(precOr, false))
			left = node("as_pattern", left, as, field("alias", target))
		case (t.isName("or") && minPrec < precOr) || (t.isName("and") && minPrec < precAnd):
			prec := precOr
			if t.text == "and" {
				prec = precAnd
			}
			op := field("operator", p.anon(p.advance()))
			right := p.expr(prec, false)
			left = node("boolean_operator", field("left", left), op, field("right", right))
		case p.atCompare() && minPrec < precCompare:
			children := one(left)
			for p.atCompare() {
				children = append(children, field("operators", p.compareOp()), p.expr(precCompare, false))
			}
			left = node("comparison_operator", children...)
		case t.kind == tkOp && binaryPrec[t.text] > 0 && minPrec < binaryPrec[t.text]:
			prec := binaryPrec[t.text]
			op := field("operator", p.anon(p.advance()))
			rightMin := prec
			if t.text == "**" {
				rightMin = prec - 1
			}
			right := p.expr(rightMin, false)
			left = node("binary_operator", field("left", left), op, field("right", right))
		default:
			return left
		}
	}
}

func (p *parser) atCompare() bool {
	t := p.peek()
	if t.kind == tkOp {
		return compareOps[t.text]
	}
	if t.isName("in") || t.isName("is") {
		return true
	}
	return t.isName("not") && p.peekN(1).isName("in")
}

func (p *parser) compareOp() *syntax.Node {
	t := p.advance()
	if t.isName("not") {
		in := p.anon(p.advance())
		return anonInner("not in", p.anon(t), in)
	}
	if t.isName("is") && p.atName("not") {
		not := p.anon(p.advance())
		return anonInner("is not", p.anon(t), not)
	}
	return p.anon(t)
}

func (p *parser) prefix(allowAs bool) *syntax.Node {
	t := p.peek()
	switch {
	case t.isName("not"):
		not := p.anon(p.advance())
		return node("not_operator", not, field("argument", p.expr(precNot, false)))
	case t.isOp("-") || t.isOp("+") || t.isOp("~"):
		op := field("operator", p.anon(p.advance()))
		return node("unary_operator", op, field("argument", p.expr(precUnary, false)))
	case t.isName("lambda"):
		return p.lambda()
	case t.isName("await") && p.awaitFollows():
		await := p.anon(p.advance())
		return node("await", await, p.expr(precUnary, false))
	case t.kind == tkName && !hardKeywords[t.text] && p.peekN(1).isOp(":="):
		name := field("name", p.identifier())
		op := p.anon(p.advance())
		return node("named_expression", name, op, field("value", p.expression()))
	}
	return p.primary()
}

func (p *parser) awaitFollows() bool {
	next := p.peekN(1)
	switch next.kind {
	case tkName:
		return !hardKeywords[next.text] || next.text == "True" || next.text == "False" || next.text == "None" ||
			next.text == "lambda" || next.text == "not"
	case tkNumber, tkStringStart:
		return true
	case tkOp:
		switch next.text {
		case "(", "[", "{", "-", "+", "~", "...":
			return true
		}
	}
	return false
}

func (p *parser) lambda() *syntax.Node {
	children := one(p.anon(p.advance()))
	if !p.atOp(":") {
		children = append(children, field("parameters", node("lambda_parameters", p.parameterList(":", false)...)))
	}
	children = append(children, p.expectOp(":"), field("body", p.expression()))
	return node("lambda", children...)
}

func (p *parser) yield() *syntax.Node {
	children := one(p.expectKw("yield"))
	if p.atName("from") {
		children = append(children, p.anon(p.advance()), p.expression())
	} else if p.startsExpression() {
		children = append(children, p.expressions())
	}
	return node("yield", children...)
}

// primary parses an atom and its postfix chain (attribute, call, subscript).
func (p *parser) primary() *syntax.Node { return p.postfix(p.atom()) }

func (p *parser) postfix(left *syntax.Node) *syntax.Node {
	for {
		t := p.peek()
		switch {
		case t.isOp("."):
			dot := p.anon(p.advance())
			left = node("attribute", field("object", left), dot, field("attribute", p.identifier()))
		case t.isOp("("):
			var args *syntax.Node
			if p.isGeneratorCall() {
				args = p.parenthesized()
			} else {
				args = p.argumentList()
			}
			left = node("call", field("function", left), field("arguments", args))
		case t.isOp("["):
			children := one(field("value", left), p.anon(p.advance()))
			for {
				children = append(children, field("subscript", p.subscriptItem()))
				if !p.atOp(",") {
					break
				}
				children = append(children, p.anon(p.advance()))
				if p.atOp("]") {
					break
				}
			}
			children = append(children, p.expectOp("]"))
			left = node("subscript", children...)
		default:
			return left
		}
	}
}

// isGeneratorCall reports f(x for x in y): a single unparenthesized
// generator argument.
func (p *parser) isGeneratorCall() bool {
	saved := p.pos
	defer func() { p.pos = saved }()
	isGen := false
	p.attempt(func() {
		p.advance()
		if p.atOp(")") || p.atOp("*") || p.atOp("**") {
			return
		}
		p.expression()
		isGen = p.atName("for") || p.atName("async") && p.peekN(1).isName("for")
	})
	return isGen
}

func (p *parser) subscriptItem() *syntax.Node {
	var children []*syntax.Node
	if !p.atOp(":") {
		e := p.expression()
		if !p.atOp(":") {
			return e
		}
		children = append(children, e)
	}
	children = append(children, p.anon(p.advance()))
	if !p.atOp(":") && !p.atOp(",") && !p.atOp("]") {
		children = append(children, p.expression())
	}
	if p.atOp(":") {
		children = append(children, p.anon(p.advance()))
		if !p.atOp(",") && !p.atOp("]") {
			children = append(children, p.expression())
		}
	}
	n := node("slice", children...)
	return n
}

func (p *parser) atom() *syntax.Node {
	t := p.peek()
	switch t.kind {
	case tkName:
		switch t.text {
		case "True":
			p.advance()
			return syntax.Leaf("true", true, t.start, t.end)
		case "False":
			p.advance()
			return syntax.Leaf("false", true, t.start, t.end)
		case "None":
			p.advance()
			return syntax.Leaf("none", true, t.start, t.end)
		}
		return p.identifier()
	case tkNumber:
		p.advance()
		return syntax.Leaf(t.typ, true, t.start, t.end)
	case tkStringStart:
		return p.stringOrConcat()
	case tkOp:
		switch t.text {
		case "...":
			p.advance()
			return syntax.Leaf("ellipsis", true, t.start, t.end)
		case "(":
			return p.parenthesized()
		case "[":
			return p.listLike()
		case "{":
			return p.dictLike()
		}
	}
	fail(t.start, "unexpected %q", t.text)
	return nil
}

// collectionElement is one element of a list/tuple/set/argument.
// collectionElement parses one element. With closer set (list/set first
// element), a lone "*x…" directly followed by the closer binds tightly
// (tree-sitter: [*a.b] is attribute(list_splat(a), b), but [*a.b, c] is
// list_splat(attribute)).
func (p *parser) collectionElement(closer string) *syntax.Node {
	switch {
	case p.atOp("*") && closer != "":
		var n *syntax.Node
		if p.attempt(func() {
			n = p.tightSplat()
			if !p.atOp(closer) {
				fail(p.peek().start, "not a lone splat")
			}
		}) {
			return n
		}
		star := p.anon(p.advance())
		return node("list_splat", star, p.expression())
	case p.atOp("*"):
		star := p.anon(p.advance())
		return node("list_splat", star, p.expression())
	case p.atName("yield"):
		return p.yield()
	}
	return p.expression()
}

func (p *parser) atComprehension() bool {
	return p.atName("for") || p.atName("async") && p.peekN(1).isName("for")
}

func (p *parser) comprehensionClauses() []*syntax.Node {
	var out []*syntax.Node
	for {
		switch {
		case p.atComprehension():
			var children []*syntax.Node
			if p.atName("async") {
				children = append(children, p.anon(p.advance()))
			}
			children = append(children, p.expectKw("for"), field("left", p.targets()), p.expectKw("in"))
			for {
				children = append(children, field("right", p.expr(precConditional-1, false)))
				if !p.atOp(",") {
					break
				}
				children = append(children, p.anon(p.advance()))
				if !p.startsExpression() {
					break
				}
			}
			out = append(out, node("for_in_clause", children...))
		case p.atName("if"):
			ifTok := p.anon(p.advance())
			out = append(out, node("if_clause", ifTok, p.expr(precOr-1, false)))
		default:
			return out
		}
	}
}

func (p *parser) parenthesized() *syntax.Node {
	open := p.anon(p.advance())
	if p.atOp(")") {
		return node("tuple", open, p.anon(p.advance()))
	}
	first := p.collectionElement("")
	if p.atComprehension() {
		children := one(open, field("body", first))
		children = append(children, p.comprehensionClauses()...)
		return node("generator_expression", append(children, p.expectOp(")"))...)
	}
	if p.atOp(")") && first.Type != "list_splat" {
		return node("parenthesized_expression", open, first, p.anon(p.advance()))
	}
	children := one(open, first)
	for p.atOp(",") {
		children = append(children, p.anon(p.advance()))
		if p.atOp(")") {
			break
		}
		children = append(children, p.collectionElement(""))
	}
	return node("tuple", append(children, p.expectOp(")"))...)
}

func (p *parser) listLike() *syntax.Node {
	open := p.anon(p.advance())
	if p.atOp("]") {
		return node("list", open, p.anon(p.advance()))
	}
	first := p.collectionElement("]")
	if p.atComprehension() {
		children := one(open, field("body", first))
		children = append(children, p.comprehensionClauses()...)
		return node("list_comprehension", append(children, p.expectOp("]"))...)
	}
	children := one(open, first)
	for p.atOp(",") {
		children = append(children, p.anon(p.advance()))
		if p.atOp("]") {
			break
		}
		children = append(children, p.collectionElement(""))
	}
	return node("list", append(children, p.expectOp("]"))...)
}

func (p *parser) dictLike() *syntax.Node {
	open := p.anon(p.advance())
	if p.atOp("}") {
		return node("dictionary", open, p.anon(p.advance()))
	}
	entry := func() *syntax.Node {
		if p.atOp("**") {
			star := p.anon(p.advance())
			return node("dictionary_splat", star, p.expression())
		}
		key := p.expression()
		colon := p.expectOp(":")
		return node("pair", field("key", key), colon, field("value", p.expression()))
	}
	if p.atOp("**") || p.isDictEntry() {
		first := entry()
		if first.Type == "pair" && p.atComprehension() {
			children := one(open, field("body", first))
			children = append(children, p.comprehensionClauses()...)
			return node("dictionary_comprehension", append(children, p.expectOp("}"))...)
		}
		children := one(open, first)
		for p.atOp(",") {
			children = append(children, p.anon(p.advance()))
			if p.atOp("}") {
				break
			}
			children = append(children, entry())
		}
		return node("dictionary", append(children, p.expectOp("}"))...)
	}
	first := p.collectionElement("}")
	if p.atComprehension() {
		children := one(open, field("body", first))
		children = append(children, p.comprehensionClauses()...)
		return node("set_comprehension", append(children, p.expectOp("}"))...)
	}
	children := one(open, first)
	for p.atOp(",") {
		children = append(children, p.anon(p.advance()))
		if p.atOp("}") {
			break
		}
		children = append(children, p.collectionElement(""))
	}
	return node("set", append(children, p.expectOp("}"))...)
}

func (p *parser) isDictEntry() bool {
	saved := p.pos
	defer func() { p.pos = saved }()
	isPair := false
	p.attempt(func() {
		p.expression()
		isPair = p.atOp(":")
	})
	return isPair
}

func (p *parser) argumentList() *syntax.Node {
	children := one(p.expectOp("("))
	for !p.atOp(")") {
		var arg *syntax.Node
		switch {
		case p.atOp("*"):
			star := p.anon(p.advance())
			arg = node("list_splat", star, p.expression())
		case p.atOp("**"):
			star := p.anon(p.advance())
			arg = node("dictionary_splat", star, p.expression())
		case p.peek().kind == tkName && !hardKeywords[p.peek().text] && p.peekN(1).isOp("="):
			name := field("name", p.identifier())
			eq := p.anon(p.advance())
			arg = node("keyword_argument", name, eq, field("value", p.expression()))
		default:
			arg = p.expression()
			if p.atComprehension() {
				// f(x for x in y, z) is invalid Python; keep the shape simple.
				fail(p.peek().start, "generator argument needs parentheses")
			}
		}
		children = append(children, arg)
		if !p.atOp(",") {
			break
		}
		children = append(children, p.anon(p.advance()))
	}
	return node("argument_list", append(children, p.expectOp(")"))...)
}

// parameters is a def's "(…)".
func (p *parser) parameters() *syntax.Node {
	children := one(p.expectOp("("))
	children = append(children, p.parameterList(")", true)...)
	return node("parameters", append(children, p.expectOp(")"))...)
}

func (p *parser) parameterList(closer string, typed bool) []*syntax.Node {
	var out []*syntax.Node
	for !p.atOp(closer) {
		out = append(out, p.parameter(typed))
		if !p.atOp(",") {
			break
		}
		out = append(out, p.anon(p.advance()))
	}
	return out
}

func (p *parser) parameter(typed bool) *syntax.Node {
	typedOf := func(target *syntax.Node) *syntax.Node {
		if typed && p.atOp(":") {
			colon := p.anon(p.advance())
			return node("typed_parameter", target, colon, field("type", p.typeNode()))
		}
		return target
	}
	switch {
	case p.atOp("*"):
		star := p.anon(p.advance())
		if p.peek().kind == tkName && !hardKeywords[p.peek().text] {
			return typedOf(node("list_splat_pattern", star, p.identifier()))
		}
		return node("keyword_separator", star)
	case p.atOp("**"):
		star := p.anon(p.advance())
		return typedOf(node("dictionary_splat_pattern", star, p.identifier()))
	case p.atOp("/"):
		return node("positional_separator", p.anon(p.advance()))
	case p.atOp("("):
		target := toPattern(p.parenthesized(), false)
		if p.atOp("=") {
			eq := p.anon(p.advance())
			return node("default_parameter", field("name", target), eq, field("value", p.expression()))
		}
		return target
	}
	name := p.identifier()
	if typed && p.atOp(":") {
		colon := p.anon(p.advance())
		typ := field("type", p.typeNode())
		if p.atOp("=") {
			eq := p.anon(p.advance())
			return node("typed_default_parameter", field("name", name), colon, typ, eq, field("value", p.expression()))
		}
		return node("typed_parameter", name, colon, typ)
	}
	if p.atOp("=") {
		eq := p.anon(p.advance())
		return node("default_parameter", field("name", name), eq, field("value", p.expression()))
	}
	return name
}

// typeNode parses tree-sitter-python's `type` rule.
func (p *parser) typeNode() *syntax.Node {
	var inner *syntax.Node
	switch {
	case (p.atOp("*") || p.atOp("**")) && p.peekN(1).kind == tkName:
		star := p.anon(p.advance())
		inner = node("splat_type", star, p.identifier())
	case p.isGenericType():
		name := p.identifier()
		inner = node("generic_type", name, p.typeParameter())
	default:
		return node("type", p.expression())
	}
	// union_type is left-associative over generic operands; an expression
	// operand (no generic_type at its start) absorbs the rest as binary `|`.
	t := node("type", inner)
	for p.atOp("|") {
		bar := p.anon(p.advance())
		var right *syntax.Node
		if p.isGenericType() {
			name := p.identifier()
			right = node("type", node("generic_type", name, p.typeParameter()))
		} else {
			right = node("type", p.expression())
		}
		t = node("type", node("union_type", t, bar, right))
	}
	return t
}

// isGenericType: NAME [ … ] not followed by a postfix (. ( [).
func (p *parser) isGenericType() bool {
	if p.peek().kind != tkName || hardKeywords[p.peek().text] || !p.peekN(1).isOp("[") {
		return false
	}
	depth := 0
	for i := p.pos + 1; i < len(p.toks); i++ {
		t := p.toks[i]
		if t.kind != tkOp {
			continue
		}
		switch t.text {
		case "(", "[", "{":
			depth++
		case ")", "]", "}":
			depth--
			if depth == 0 {
				next := p.toks[min(i+1, len(p.toks)-1)]
				return !(next.isOp(".") || next.isOp("(") || next.isOp("["))
			}
		}
	}
	return false
}

func (p *parser) typeParameter() *syntax.Node {
	children := one(p.expectOp("["))
	for {
		elem := p.typeNode()
		if p.atOp(":") {
			colon := p.anon(p.advance())
			elem = node("type", node("constrained_type", elem, colon, p.typeNode()))
		}
		children = append(children, elem)
		if !p.atOp(",") {
			break
		}
		children = append(children, p.anon(p.advance()))
		if p.atOp("]") {
			break
		}
	}
	return node("type_parameter", append(children, p.expectOp("]"))...)
}

// targets parses a for/comprehension target list (`_left_hand_side`).
func (p *parser) targets() *syntax.Node {
	target := func() *syntax.Node {
		if p.atOp("*") {
			star := p.anon(p.advance())
			return node("list_splat_pattern", star, p.primary())
		}
		return toPattern(p.primary(), false)
	}
	first := target()
	if !p.atOp(",") {
		return first
	}
	children := one(first)
	for p.atOp(",") {
		children = append(children, p.anon(p.advance()))
		if p.atName("in") || p.atOp("=") {
			break
		}
		children = append(children, target())
	}
	return node("pattern_list", children...)
}

// toPattern rewrites an expression parsed in target position into
// tree-sitter's pattern node types.
func toPattern(n *syntax.Node, top bool) *syntax.Node {
	switch n.Type {
	case "tuple", "parenthesized_expression":
		n.Type = "tuple_pattern"
	case "list":
		n.Type = "list_pattern"
	case "list_splat":
		n.Type = "list_splat_pattern"
	case "expression_list", "tuple_expression":
		n.Type = "pattern_list"
	default:
		return n
	}
	for i, c := range n.Children {
		if c.Named {
			n.Children[i] = toPattern(c, false)
		}
	}
	return n
}

func (p *parser) stringOrConcat() *syntax.Node {
	first := p.stringLit()
	if p.peek().kind != tkStringStart {
		return first
	}
	children := one(first)
	for p.peek().kind == tkStringStart {
		children = append(children, p.stringLit())
	}
	return node("concatenated_string", children...)
}

func (p *parser) stringLit() *syntax.Node {
	st := p.advance()
	children := one(syntax.Leaf("string_start", true, st.start, st.end))
	var content *syntax.Node
	for {
		t := p.peek()
		switch t.kind {
		case tkStrPiece:
			p.advance()
			if content == nil {
				content = syntax.Leaf("string_content", true, t.start, t.end)
				children = append(children, content)
			}
			content.EndByte = t.end
			if t.typ != "" {
				content.Children = append(content.Children, syntax.Leaf(t.typ, t.typ != "\\", t.start, t.end))
			}
			continue
		case tkStringEnd:
			p.advance()
			children = append(children, syntax.Leaf("string_end", true, t.start, t.end))
			return node("string", children...)
		}
		if t.isOp("{") {
			content = nil
			children = append(children, p.interpolation("interpolation"))
			continue
		}
		fail(t.start, "unexpected token in string")
	}
}

func (p *parser) interpolation(typ string) *syntax.Node {
	children := one(p.expectOp("{"))
	var expr *syntax.Node
	if p.atName("yield") {
		expr = p.yield()
	} else {
		first, list := p.starExpressionList("expression_list")
		expr = first
		if list && p.atOp("=") {
			expr = toPattern(first, true)
		}
	}
	children = append(children, field("expression", expr))
	if p.atOp("=") {
		children = append(children, p.anon(p.advance()))
	}
	if t := p.peek(); t.kind == tkTypeConv {
		p.advance()
		children = append(children, field("type_conversion", syntax.Leaf("type_conversion", true, t.start, t.end)))
	}
	if p.atOp(":") {
		colon := p.anon(p.advance())
		spec := syntax.Inner("format_specifier", colon.StartByte, colon)
		for {
			t := p.peek()
			if t.kind == tkFormatText {
				p.advance()
				spec.EndByte = t.end
				continue
			}
			if t.isOp("{") {
				fe := p.interpolation("format_expression")
				spec.Children = append(spec.Children, fe)
				spec.EndByte = fe.EndByte
				continue
			}
			break
		}
		children = append(children, field("format_specifier", spec))
	}
	return node(typ, append(children, p.expectOp("}"))...)
}

// anonInner is an anonymous node with children (tree-sitter's
// alias(seq('not', 'in'), 'not in')).
func anonInner(typ string, children ...*syntax.Node) *syntax.Node {
	n := node(typ, children...)
	n.Named = false
	return n
}
