package crap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/lang"
	"github.com/matheusAle/coderefinery/internal/model"
	"github.com/matheusAle/coderefinery/internal/syntax"
)

// Go entries (upstream crapper/languages/golang.py), following crap4go:
// decisions are if, for (incl. range), every switch / type-switch / select
// case and default, and && / ||. The namespace is the package import path;
// methods are namespaced by their receiver type.

var goDecisions = map[string]bool{
	"if_statement": true, "for_statement": true, "expression_case": true,
	"type_case": true, "communication_case": true, "default_case": true,
}

func goIsDecision(n *syntax.Node) bool { return goDecisions[n.Type] || BinaryLogic(n) }

func goSkip(*syntax.Node) bool { return false }

// goPackageClause is golang._package_clause: the first package_identifier
// anywhere in the file (preorder), or "".
func goPackageClause(src []byte, root *syntax.Node) string {
	name := ""
	root.Descendants(func(n *syntax.Node) bool {
		if n.Type == "package_identifier" {
			name = n.Text(src)
			return false
		}
		return true
	})
	return name
}

// goReceiverType is golang._receiver_type: the first type_identifier in the
// first parameter_list child (the receiver), or "".
func goReceiverType(src []byte, method *syntax.Node) string {
	params := method.ChildOfType("parameter_list")
	if params == nil {
		return ""
	}
	found := ""
	params.Descendants(func(n *syntax.Node) bool {
		if n.Type == "type_identifier" {
			found = n.Text(src)
			return false
		}
		return true
	})
	return found
}

func childText(src []byte, n *syntax.Node, typ string) string {
	if c := n.ChildOfType(typ); c != nil {
		return c.Text(src)
	}
	return ""
}

// absolutePath is treesitter.absolute_path.
func absolutePath(path, projectRoot string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if projectRoot != "" {
		return discover.Resolve(filepath.Join(projectRoot, path))
	}
	return discover.Resolve(path)
}

// GoPackageNamespace is golang.package_namespace: the import path of the
// package (nearest go.mod's module + directory), or packageName when no
// go.mod (or no module line in the nearest one) exists.
func GoPackageNamespace(path, packageName string) (string, error) {
	dir := filepath.Dir(discover.Resolve(path))
	module, moduleDir := "", ""
	for current := dir; ; current = filepath.Dir(current) {
		candidate := filepath.Join(current, "go.mod")
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			data, err := os.ReadFile(candidate)
			if err != nil {
				return "", err
			}
			if !utf8.Valid(data) {
				return "", fmt.Errorf("'utf-8' codec can't decode %s", candidate)
			}
			for _, line := range coverage.SplitLines(string(data)) {
				stripped := coverage.PyStrip(line)
				if strings.HasPrefix(stripped, "module ") {
					module = coverage.PyStrip(pySplitRest(stripped))
					moduleDir = current
					break
				}
			}
			break
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	if module == "" || moduleDir == "" {
		return packageName, nil
	}
	rel, err := filepath.Rel(moduleDir, dir)
	if err != nil {
		return "", err
	}
	if rel = filepath.ToSlash(rel); rel == "." {
		return module, nil
	}
	return module + "/" + rel, nil
}

// pySplitRest is s.split(None, 1)[1] for s starting with a non-space word
// followed by whitespace.
func pySplitRest(s string) string {
	i := strings.IndexFunc(s, isPyWhitespace)
	if i < 0 {
		return ""
	}
	return strings.TrimLeftFunc(s[i:], isPyWhitespace)
}

func isPyWhitespace(r rune) bool { return coverage.PyStrip(string(r)) == "" }

// GoFunctions is golang.functions_in_source.
func GoFunctions(source, path, projectRoot string) ([]model.Function, error) {
	tree, err := lang.ParseText("go", source)
	if err != nil {
		return nil, err
	}
	src := tree.Source
	packageNS, err := GoPackageNamespace(absolutePath(path, projectRoot), goPackageClause(src, tree.Root))
	if err != nil {
		return nil, err
	}
	var found []model.Function
	tree.Root.Descendants(func(n *syntax.Node) bool {
		if n.Type != "function_declaration" && n.Type != "method_declaration" {
			return true
		}
		if n.ChildOfType("block") == nil {
			return true
		}
		var name, namespace string
		if n.Type == "method_declaration" {
			name = childText(src, n, "field_identifier")
			receiver := goReceiverType(src, n)
			if name == "" || receiver == "" {
				return true
			}
			namespace = packageNS + "." + receiver
		} else {
			name = childText(src, n, "identifier")
			if name == "" {
				return true
			}
			namespace = packageNS
		}
		found = append(found, model.Function{
			Name:       name,
			Namespace:  namespace,
			Complexity: Complexity(n, goIsDecision, goSkip),
			StartLine:  n.StartLine(),
			EndLine:    n.EndLine(),
			Path:       path,
			Language:   "go",
			StartByte:  -1,
			EndByte:    -1,
		})
		return true
	})
	return found, nil
}
