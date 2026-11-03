package crap

import (
	"regexp"
	"strings"

	"github.com/matheusAle/ub-mode/ubgate/internal/lang"
	"github.com/matheusAle/ub-mode/ubgate/internal/model"
	"github.com/matheusAle/ub-mode/ubgate/internal/syntax"
)

// Java entries (upstream crapper/languages/java.py, following crap4java).
// Constructors, abstract methods, and methods inside anonymous or local
// classes are ignored; nested member types become their own namespace
// (pkg.Outer.Inner) while JacocoClass keeps JaCoCo's binary name
// (pkg.Outer$Inner).

func init() { Readers["java"] = JavaFunctions }

// pyS and pyW are Python's Unicode `\s` and `\w` for str patterns (Go's
// RE2 classes are ASCII-only).
const (
	pyS = `[\t\n\x0b\f\r \x1c-\x1f\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`
	pyW = `[\p{L}\p{N}_]`
)

// javaPackage is `(?m)^\s*package\s+([a-zA-Z_][\w.]*)\s*;`; the first match
// wins, even inside a multi-line block comment (upstream quirk).
var javaPackage = regexp.MustCompile(`(?m)^` + pyS + `*package` + pyS + `+([a-zA-Z_](?:` + pyW + `|\.)*)` + pyS + `*;`)

var javaNestedTypes = map[string]bool{
	"class_declaration": true, "interface_declaration": true, "enum_declaration": true,
	"record_declaration": true, "annotation_type_declaration": true,
}

var javaDecisions = map[string]bool{
	"if_statement": true, "for_statement": true, "enhanced_for_statement": true, "while_statement": true,
	"do_statement": true, "catch_clause": true, "ternary_expression": true, "switch_label": true,
}

func javaIsDecision(n *syntax.Node) bool { return javaDecisions[n.Type] || BinaryLogic(n) }

func javaSkip(n *syntax.Node) bool { return javaNestedTypes[n.Type] || n.Type == "class_body" }

// javaInsideExecutable is java._inside_executable.
func javaInsideExecutable(n *syntax.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		switch p.Type {
		case "method_declaration", "constructor_declaration", "lambda_expression":
			return true
		}
	}
	return false
}

// javaPackageName is java._package_name ("" = None).
func javaPackageName(source string) string {
	if m := javaPackage.FindStringSubmatch(source); m != nil {
		return m[1]
	}
	return ""
}

// javaTypeChain is java._type_chain: enclosing type names, outermost first.
func javaTypeChain(src []byte, n *syntax.Node) []string {
	var names []string
	for p := n.Parent; p != nil; p = p.Parent {
		if javaNestedTypes[p.Type] {
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

// javaQualify is java._qualify.
func javaQualify(pkg string, names []string, nested string) string {
	body := strings.Join(names, nested)
	if pkg != "" {
		if body != "" {
			return pkg + "." + body
		}
		return pkg
	}
	return body
}

// JavaFunctions is java.functions_in_source.
func JavaFunctions(source, path, projectRoot string) ([]model.Function, error) {
	tree, err := lang.ParseText("java", source)
	if err != nil {
		return nil, err
	}
	src := tree.Source
	pkg := javaPackageName(source)
	var found []model.Function
	syntax.Walk(tree.Root, func(n *syntax.Node) bool {
		if n.Type != "method_declaration" || javaInsideExecutable(n) {
			return true
		}
		if n.ChildOfType("block") != nil {
			ident := n.ChildOfType("identifier")
			names := javaTypeChain(src, n)
			if ident != nil && len(names) > 0 {
				found = append(found, model.Function{
					Name:        ident.Text(src),
					Namespace:   javaQualify(pkg, names, "."),
					Complexity:  Complexity(n, javaIsDecision, javaSkip),
					StartLine:   n.StartLine(),
					EndLine:     n.EndLine(),
					Path:        path,
					Language:    "java",
					JacocoClass: javaQualify(pkg, names, "$"),
					StartByte:   -1,
					EndByte:     -1,
				})
			}
		}
		return false
	})
	return found, nil
}
