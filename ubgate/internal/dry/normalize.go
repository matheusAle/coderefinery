package dry

import "github.com/matheusAle/ub-mode/ubgate/internal/syntax"

// Normalization of tree-sitter trees: upstream astnorm.py (spec-dryer §4).
// Upstream walks an explicit post-order stack keyed by (node, mode, head);
// this is the equivalent recursive form from spec §4.2. Go stacks grow on
// demand, so 1,200-deep expressions do not overflow.

func set(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, n := range names {
		m[n] = true
	}
	return m
}

var (
	skipTypes = set("comment", "line_comment", "block_comment", "documentation_comment", "doc_comment")

	identifierTypes = set("identifier", "type_identifier", "field_identifier", "property_identifier",
		"shorthand_property_identifier", "shorthand_property_identifier_pattern", "package_identifier")

	literalTypes = set("string", "string_literal", "interpreted_string_literal", "raw_string_literal",
		"char_literal", "rune_literal", "character_literal", "number", "integer", "float",
		"decimal_integer_literal", "hex_integer_literal", "octal_integer_literal", "binary_integer_literal",
		"decimal_floating_point_literal", "hex_floating_point_literal", "int_literal", "float_literal",
		"imaginary_literal", "integer_literal", "true", "false", "null", "nil", "none", "undefined",
		"null_literal", "boolean_literal", "template_string", "regex", "regex_literal")

	operatorTypes = set("+", "-", "*", "/", "%", "**", "==", "!=", "<", ">", "<=", ">=", "===", "!==",
		"&&", "||", "&", "|", "^", "<<", ">>", ">>>", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
		"<<=", ">>=", ">>>=", "!", "~", "++", "--", "and", "or", "not", "??", "?.", "=", ":=", "=>", "?")

	callTypes      = set("call", "call_expression", "method_invocation", "object_creation_expression", "new_expression")
	attributeTypes = set("attribute", "member_expression", "selector_expression", "field_access", "field_expression")
	// scopedTypes is _SCOPED plus "generic_type", which upstream tests
	// alongside it everywhere.
	scopedTypes   = set("scoped_identifier", "scoped_type_identifier", "generic_type")
	argListTypes  = set("argument_list", "arguments")
	typeArgsTypes = set("type_arguments", "type_parameters")
)

type normalizer struct{ src []byte }

// Normalize is astnorm.normalize(node, data): the normalized tree of a
// candidate node. It returns nil only when the node itself is punctuation,
// a comment or another dropped token.
func Normalize(n *syntax.Node, src []byte) *Value {
	return normalizer{src}.norm(n, false)
}

func (z normalizer) text(n *syntax.Node) string { return n.Text(z.src) }

// isLiteral is _is_literal: a literal type, except a string holding an
// interpolation (Python f-strings).
func isLiteral(n *syntax.Node) bool {
	if !literalTypes[n.Type] {
		return false
	}
	return !(n.Type == "string" && n.HasChildOfType("interpolation"))
}

func (z normalizer) norm(n *syntax.Node, head bool) *Value {
	if skipTypes[n.Type] {
		return nil
	}
	if operatorTypes[n.Type] {
		return symbolOf(n.Type)
	}
	if !n.Named {
		return nil
	}
	if identifierTypes[n.Type] {
		if head {
			return symbolOf(z.text(n))
		}
		return K("symbol")
	}
	if isLiteral(n) {
		return K("literal")
	}
	if n.Type == "parenthesized_expression" {
		if named := n.NamedChildren(); len(named) == 1 {
			return z.norm(named[0], head)
		}
	}
	if callTypes[n.Type] {
		return z.call(n)
	}
	if attributeTypes[n.Type] {
		return z.attr(n, head)
	}
	out := List(K(n.Type))
	for _, c := range n.Children {
		out.append(z.norm(c, false))
	}
	return out
}

// append adds v to a list unless it is nil (upstream's `if norm is not None`).
func (v *Value) append(item *Value) {
	if item != nil {
		v.Items = append(v.Items, item)
	}
}

// call is _assemble_call: receivers, then the callee with its name kept,
// then type arguments, then the argument list. Named children after the
// argument list (a Java anonymous class body) are dropped.
func (z normalizer) call(n *syntax.Node) *Value {
	var rest, typeArgs []*syntax.Node
	var args *syntax.Node
	for _, c := range n.NamedChildren() {
		if argListTypes[c.Type] {
			args = c
			break
		}
		if typeArgsTypes[c.Type] {
			typeArgs = append(typeArgs, c)
		} else {
			rest = append(rest, c)
		}
	}
	out := List(K(n.Type))
	if len(rest) > 0 {
		for _, r := range rest[:len(rest)-1] {
			out.append(z.norm(r, false))
		}
		// Upstream appends the callee without a None check and crashes in
		// pr() when it is a comment (spec §4.6); ubgate skips it (DEVIATIONS D1).
		out.append(z.callee(rest[len(rest)-1]))
	}
	for _, t := range typeArgs {
		out.append(z.norm(t, false))
	}
	if args != nil {
		out.append(z.norm(args, false))
	}
	return out
}

// callee is _callee_leaf / _callee_value.
func (z normalizer) callee(n *syntax.Node) *Value {
	switch {
	case identifierTypes[n.Type]:
		return symbolOf(z.text(n))
	case attributeTypes[n.Type]:
		return z.attr(n, true)
	case scopedTypes[n.Type]:
		return z.path(n)
	}
	return z.norm(n, true)
}

// attr is _assemble_attr: the objects, then the member name, which keeps
// its spelling only in callee position.
func (z normalizer) attr(n *syntax.Node, head bool) *Value {
	named := n.NamedChildren()
	out := List(K(n.Type))
	if len(named) == 0 {
		return out
	}
	for _, o := range named[:len(named)-1] {
		out.append(z.norm(o, false))
	}
	name := named[len(named)-1]
	switch {
	case identifierTypes[name.Type] && head:
		out.append(symbolOf(z.text(name)))
	case identifierTypes[name.Type]:
		out.append(K("symbol"))
	case head:
		out.append(z.callee(name))
	default:
		out.append(z.norm(name, false))
	}
	return out
}

// path is _assemble_path: a callee path keeps every name in it; type
// arguments stay structural.
func (z normalizer) path(n *syntax.Node) *Value {
	out := List(K(n.Type))
	for _, c := range n.NamedChildren() {
		switch {
		case identifierTypes[c.Type]:
			out.append(symbolOf(z.text(c)))
		case scopedTypes[c.Type]:
			out.append(z.path(c))
		default:
			out.append(z.norm(c, false))
		}
	}
	return out
}
