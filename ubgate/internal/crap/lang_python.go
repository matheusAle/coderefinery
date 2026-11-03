package crap

import (
	"strings"

	"github.com/matheusAle/ub-mode/ubgate/internal/lang"
	"github.com/matheusAle/ub-mode/ubgate/internal/model"
	"github.com/matheusAle/ub-mode/ubgate/internal/syntax"
)

// Python entries (upstream crapper/languages/python.py).

var pythonDecisions = map[string]bool{
	"if_statement": true, "elif_clause": true, "for_statement": true, "while_statement": true,
	"except_clause": true, "conditional_expression": true, "case_clause": true, "if_clause": true,
}

func pythonIsDecision(n *syntax.Node) bool {
	if pythonDecisions[n.Type] {
		return true
	}
	return n.Type == "boolean_operator" && n.HasChildOfType("and", "or")
}

func pythonSkip(n *syntax.Node) bool { return n.Type == "class_definition" }

func stripRoot(relative, sourceRoot string) string {
	root := strings.TrimRight(strings.ReplaceAll(sourceRoot, `\`, "/"), "/")
	if root != "" && root != "." && strings.HasPrefix(relative, root+"/") {
		return relative[len(root)+1:]
	}
	if strings.HasPrefix(relative, "./") {
		return relative[2:]
	}
	return relative
}

func stripSrc(relative string) string {
	if strings.HasPrefix(relative, "src/") {
		return relative[4:]
	}
	if _, after, ok := strings.Cut(relative, "/src/"); ok {
		return after
	}
	return relative
}

func stripPythonSuffix(relative string) string {
	relative = strings.TrimSuffix(relative, ".py")
	if strings.HasSuffix(relative, "/__init__") {
		return strings.TrimSuffix(relative, "/__init__")
	}
	if relative == "__init__" {
		return ""
	}
	return relative
}

// PythonModuleNamespace is python._module_namespace.
func PythonModuleNamespace(path, sourceRoot string) string {
	relative := stripPythonSuffix(stripSrc(stripRoot(strings.ReplaceAll(path, `\`, "/"), sourceRoot)))
	if relative == "" {
		return "__init__"
	}
	return strings.ReplaceAll(relative, "/", ".")
}

func isNestedPythonFunction(n *syntax.Node) bool {
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

func pythonClassNames(src []byte, n *syntax.Node) []string {
	var names []string
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == "class_definition" {
			if ident := p.ChildOfType("identifier"); ident != nil {
				names = append(names, ident.Text(src))
			}
		}
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	return names
}

// PythonFunctions is python.functions_in_source.
func PythonFunctions(source, path, sourceRoot string) ([]model.Function, error) {
	tree, err := lang.ParseText("python", source)
	if err != nil {
		return nil, err
	}
	src := tree.Source
	module := PythonModuleNamespace(path, sourceRoot)
	var found []model.Function
	tree.Root.Descendants(func(n *syntax.Node) bool {
		if n.Type != "function_definition" || isNestedPythonFunction(n) {
			return true
		}
		ident := n.ChildOfType("identifier")
		if ident == nil {
			return true
		}
		namespace := module
		if classes := pythonClassNames(src, n); len(classes) > 0 {
			if module != "" {
				namespace = strings.Join(append([]string{module}, classes...), ".")
			} else {
				namespace = strings.Join(classes, ".")
			}
		}
		found = append(found, model.Function{
			Name:       ident.Text(src),
			Namespace:  namespace,
			Complexity: Complexity(n, pythonIsDecision, pythonSkip),
			StartLine:  n.StartLine(),
			EndLine:    n.EndLine(),
			Path:       path,
			Language:   "python",
			StartByte:  -1,
			EndByte:    -1,
		})
		return true
	})
	return found, nil
}
