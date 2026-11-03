package dry

import (
	"strings"

	"github.com/matheusAle/ub-mode/ubgate/internal/lang"
	"github.com/matheusAle/ub-mode/ubgate/internal/syntax"
)

// Candidate extraction: upstream extract.py (spec-dryer §3).

// Candidate is one form to compare, before the --min-lines/--min-nodes filter.
type Candidate struct {
	StartLine, EndLine int
	Norm               *Value
}

var tsFunctionTypes = set("function_declaration", "method_definition", "arrow_function", "function_expression")
var tsDefinitionParents = set("variable_declarator", "public_field_definition", "property_definition")
var javaNestingTypes = set("method_declaration", "constructor_declaration", "lambda_expression")

// grammarOf is extract._PICKERS' grammar choice: .tsx -> tsx, every other
// typescript extension -> typescript.
func grammarOf(language, path string) string {
	if language == "typescript" {
		if strings.HasSuffix(path, ".tsx") {
			return "tsx"
		}
		return "typescript"
	}
	return language
}

func inside(n *syntax.Node, types map[string]bool) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if types[p.Type] {
			return true
		}
	}
	return false
}

// pythonNested is _python_nested: a def inside another def; a class in
// between keeps the method.
func pythonNested(n *syntax.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		switch p.Type {
		case "class_definition":
			return false
		case "function_definition":
			return true
		}
	}
	return false
}

func rustInTests(src []byte, n *syntax.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == "mod_item" {
			if ident := p.ChildOfType("identifier"); ident != nil && ident.Text(src) == "tests" {
				return true
			}
		}
	}
	return false
}

func descendantsWhere(root *syntax.Node, keep func(*syntax.Node) *syntax.Node) []*syntax.Node {
	var found []*syntax.Node
	root.Descendants(func(n *syntax.Node) bool {
		if c := keep(n); c != nil {
			found = append(found, c)
		}
		return true
	})
	return found
}

func pythonNodes(_ []byte, root *syntax.Node) []*syntax.Node {
	return descendantsWhere(root, func(n *syntax.Node) *syntax.Node {
		if n.Type == "function_definition" && !pythonNested(n) {
			return n
		}
		return nil
	})
}

func goNodes(_ []byte, root *syntax.Node) []*syntax.Node {
	return descendantsWhere(root, func(n *syntax.Node) *syntax.Node {
		if (n.Type == "function_declaration" || n.Type == "method_declaration") && n.HasChildOfType("block") {
			return n
		}
		return nil
	})
}

// typescriptNodes is _typescript_nodes. For arrows and function
// expressions the candidate is the declarator or field that holds them.
func typescriptNodes(_ []byte, root *syntax.Node) []*syntax.Node {
	return descendantsWhere(root, func(n *syntax.Node) *syntax.Node {
		switch n.Type {
		case "function_declaration", "method_definition":
			if n.HasChildOfType("statement_block") && !inside(n, tsFunctionTypes) {
				return n
			}
		case "arrow_function", "function_expression":
			if p := n.Parent; p != nil && tsDefinitionParents[p.Type] && !inside(p, tsFunctionTypes) {
				return p
			}
		}
		return nil
	})
}

func rustNodes(src []byte, root *syntax.Node) []*syntax.Node {
	return descendantsWhere(root, func(n *syntax.Node) *syntax.Node {
		if n.Type != "function_item" || !n.HasChildOfType("block") {
			return nil
		}
		if n.Parent != nil && n.Parent.Type == "block" {
			return nil
		}
		if rustInTests(src, n) {
			return nil
		}
		return n
	})
}

// javaNodes is _java_nodes: an explicit pre-order stack that never descends
// into a method_declaration (so it does descend into constructors,
// initializers and field initializers).
func javaNodes(_ []byte, root *syntax.Node) []*syntax.Node {
	var found []*syntax.Node
	stack := []*syntax.Node{root}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n.Type == "method_declaration" {
			if n.HasChildOfType("block") && !inside(n, javaNestingTypes) {
				found = append(found, n)
			}
			continue
		}
		for i := len(n.Children) - 1; i >= 0; i-- {
			stack = append(stack, n.Children[i])
		}
	}
	return found
}

var pickers = map[string]func([]byte, *syntax.Node) []*syntax.Node{
	"java":       javaNodes,
	"go":         goNodes,
	"python":     pythonNodes,
	"typescript": typescriptNodes,
	"rust":       rustNodes,
}

// Candidates is extract.entries_in_source before the Entry wrapping: the
// candidate forms of one source text in document order, and for Clojure a
// reader warning ("<line>: <msg>", "" when none). err is set when the
// language's parser is missing or fails on the text; callers then treat the
// file as having no candidates (DEVIATIONS A2).
func Candidates(language, text, path string) (cands []Candidate, warning string, err error) {
	if language == "clojure" {
		cands, warning = clojureCandidates(text)
		return cands, warning, nil
	}
	pick, ok := pickers[language]
	if !ok {
		return nil, "", nil
	}
	tree, err := lang.ParseText(grammarOf(language, path), text)
	if err != nil {
		return nil, "", err
	}
	for _, n := range pick(tree.Source, tree.Root) {
		if v := Normalize(n, tree.Source); v != nil {
			cands = append(cands, Candidate{StartLine: n.StartLine(), EndLine: n.EndLine(), Norm: v})
		}
	}
	return cands, "", nil
}

func clojureCandidates(text string) ([]Candidate, string) {
	forms, warning := ReadClojure(text)
	var out []Candidate
	for _, f := range forms {
		if !IsCandidateForm(f) {
			continue
		}
		out = append(out, Candidate{StartLine: f.Line, EndLine: MaxLine(f), Norm: NormalizeClojure(f)})
	}
	return out, warning
}
