package mutate

// Find mutation sites (upstream mutator/sites.py). Every non-Clojure
// language walks the tree-sitter-shaped tree from internal/lang; Clojure
// uses the reader in clojure.go.

import (
	"strings"

	"github.com/matheusAle/coderefinery/internal/lang"
	"github.com/matheusAle/coderefinery/internal/syntax"
)

var binaryMutants = map[string]string{
	"+": "-", "-": "+", "*": "/", "/": "*",
	">": ">=", ">=": ">", "<": "<=", "<=": "<",
	"==": "!=", "!=": "==", "===": "!==", "!==": "===",
	"&&": "||", "||": "&&", "??": "||",
	"and": "or", "or": "and",
}

var binaryParents = map[string]bool{
	"binary_expression": true, "binary_operator": true, "comparison_operator": true, "boolean_operator": true,
}

var unaryParents = map[string]bool{"unary_expression": true, "unary_operator": true, "not_operator": true}

var booleans = map[string]string{"true": "false", "false": "true", "True": "False", "False": "True"}

var numberTypes = map[string]bool{
	"decimal_integer_literal": true, "integer_literal": true, "int_literal": true,
	"integer": true, "number": true, "num_lit": true,
}

// typeNodes: a literal in a TypeScript type never runs.
var typeNodes = map[string]bool{
	"type_annotation": true, "type_alias_declaration": true, "interface_declaration": true,
	"type_arguments": true, "type_parameters": true, "literal_type": true,
}

// RawSite is sites.RawSite.
type RawSite struct {
	Line, AnchorLine int
	Start, End       int
	Original, Mutant string
	Category         string
	Label            string // react operators (react_sites.go): the site-log description
}

// grammarFor is sites._grammar.
func grammarFor(language, path string) string {
	if language == "typescript" {
		switch {
		case strings.HasSuffix(path, ".tsx"):
			return "tsx"
		case strings.HasSuffix(path, ".js"), strings.HasSuffix(path, ".jsx"),
			strings.HasSuffix(path, ".mjs"), strings.HasSuffix(path, ".cjs"):
			return "javascript"
		}
		return "typescript"
	}
	return language
}

func category(token string) string {
	switch token {
	case ">", ">=", "<", "<=":
		return "comparison"
	case "==", "!=", "===", "!==":
		return "equality"
	case "&&", "||", "??", "and", "or":
		return "logical"
	}
	return "arithmetic"
}

func binaryMutant(language, token string) (string, bool) {
	if language == "go" && token == "/" {
		return "", false
	}
	if token == "??" && language != "typescript" {
		return "", false
	}
	m, ok := binaryMutants[token]
	return m, ok
}

// optionalMutant: `a?.b` becomes `a.b`; a call or index drops `?.`.
func optionalMutant(n *syntax.Node) (string, bool) {
	parent := n.Parent
	if parent == nil {
		return "", false
	}
	host := parent
	if parent.Type == "optional_chain" {
		host = parent.Parent
	}
	if host == nil {
		return "", false
	}
	switch host.Type {
	case "member_expression":
		return ".", true
	case "call_expression", "subscript_expression":
		return "", true
	}
	return "", false
}

func isStatement(t string) bool {
	return strings.HasSuffix(t, "_statement") || strings.HasSuffix(t, "_declaration")
}

// statementLine is sites._statement_line: the start line of the nearest
// strict ancestor statement or declaration, else the node's own line.
func statementLine(n *syntax.Node) int {
	for cur := n.Parent; cur != nil; cur = cur.Parent {
		if isStatement(cur.Type) {
			return cur.StartPoint.Row + 1
		}
	}
	return n.StartPoint.Row + 1
}

func inType(n *syntax.Node) bool {
	for cur := n.Parent; cur != nil; cur = cur.Parent {
		if typeNodes[cur.Type] {
			return true
		}
	}
	return false
}

type siteFinder struct {
	data     []byte
	language string
	found    []RawSite
}

func (f *siteFinder) add(n *syntax.Node, original, mutant, cat string) {
	if mutant == original {
		return
	}
	f.found = append(f.found, RawSite{
		Line:       n.StartPoint.Row + 1,
		AnchorLine: statementLine(n),
		Start:      n.StartByte,
		End:        n.EndByte,
		Original:   original,
		Mutant:     mutant,
		Category:   cat,
	})
}

func flip01(text string) string {
	if text == "0" {
		return "1"
	}
	return "0"
}

// leaf is sites._visit_leaf: the checks are independent.
func (f *siteFinder) leaf(n *syntax.Node) {
	token := n.Text(f.data)
	parent := ""
	if n.Parent != nil {
		parent = n.Parent.Type
	}
	if binaryParents[parent] {
		if m, ok := binaryMutant(f.language, token); ok {
			f.add(n, token, m, category(token))
		}
	}
	if f.language == "typescript" && token == "?." {
		if m, ok := optionalMutant(n); ok {
			f.add(n, "?.", m, "optional")
		}
	}
	if (token == "!" || token == "not") && (unaryParents[parent] || parent == "comparison_operator") {
		f.add(n, token, "", "unary")
	}
	if token == "-" && unaryParents[parent] {
		f.add(n, token, "", "unary")
	}
	if (n.Type == "true" || n.Type == "false") && booleans[token] != "" {
		f.add(n, token, booleans[token], "boolean")
	}
	if numberTypes[n.Type] && (token == "0" || token == "1") {
		f.add(n, token, flip01(token), "constant")
	}
}

func (f *siteFinder) node(n *syntax.Node) {
	if f.language == "typescript" && inType(n) {
		return
	}
	if len(n.Children) == 0 {
		f.leaf(n)
		return
	}
	if n.Type == "bool_lit" {
		text := n.Text(f.data)
		if m := booleans[text]; m != "" {
			f.add(n, text, m, "boolean")
		}
	}
}

// visit walks every node in pre-order with an explicit stack.
func visit(root *syntax.Node, each func(*syntax.Node)) {
	stack := []*syntax.Node{root}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		each(cur)
		for i := len(cur.Children) - 1; i >= 0; i-- {
			stack = append(stack, cur.Children[i])
		}
	}
}

// discoverRaw is sites.discover_raw: sites in (line, start, end) order.
// A file the hand parser rejects has no sites (DEVIATIONS.md A2). react adds
// the React operators' sites (mutate --react, react_sites.go).
func discoverRaw(source, language, path string, react bool) []RawSite {
	data := []byte(source)
	var root *syntax.Node
	if language == "clojure" {
		root = parseClojure(data)
	} else {
		tree, err := lang.Parse(grammarFor(language, path), data)
		if err != nil {
			return nil
		}
		root = tree.Root
	}
	f := &siteFinder{data: data, language: language}
	if language == "clojure" {
		visit(root, f.clojureNode)
	} else {
		visit(root, f.node)
		if react && language == "typescript" {
			f.found = append(f.found, reactSites(root, data)...)
		}
	}
	sortStable(f.found, func(a, b RawSite) bool {
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.Start != b.Start {
			return a.Start < b.Start
		}
		return a.End < b.End
	})
	return f.found
}

// applySite is sites.apply_site on bytes.
func applySite(data []byte, start, end int, mutant string) []byte {
	out := make([]byte, 0, len(data)-(end-start)+len(mutant))
	out = append(out, data[:start]...)
	out = append(out, mutant...)
	return append(out, data[end:]...)
}
