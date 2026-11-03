package crap

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/matheusAle/ub-mode/ubgate/internal/discover"
	"github.com/matheusAle/ub-mode/ubgate/internal/lang"
	"github.com/matheusAle/ub-mode/ubgate/internal/model"
	"github.com/matheusAle/ub-mode/ubgate/internal/source"
	"github.com/matheusAle/ub-mode/ubgate/internal/syntax"
)

// Rust entries (upstream crapper/languages/rust.py). Decision points are
// if / if let, for, while / while let, loop, each match arm, `?`, and
// && / ||. Free functions are namespaced by the module path
// (crate::foo::bar), methods by the self type (crate::foo::Widget). Bodies
// inside `mod tests` are test code and are not scored.

func init() { Readers["rust"] = RustFunctions }

var rustDecisions = map[string]bool{
	"if_expression": true, "for_expression": true, "while_expression": true,
	"loop_expression": true, "match_arm": true, "try_expression": true,
}

var (
	rustPackageBlock = regexp.MustCompile(`(?ms)^\[package\](.*?)(?:^\[|\z)`)
	rustName         = regexp.MustCompile(`(?m)^name` + pyS + `*=` + pyS + `*"([^"]+)"`)
)

func rustIsDecision(n *syntax.Node) bool { return rustDecisions[n.Type] || BinaryLogic(n) }

func rustSkip(*syntax.Node) bool { return false }

func rustHasBlock(n *syntax.Node) bool { return n.ChildOfType("block") != nil }

// rustAncestorMods is rust._ancestor_mods: the names of the enclosing
// mod_items, outermost first.
func rustAncestorMods(src []byte, n *syntax.Node) []string {
	var names []string
	for cur := n.Parent; cur != nil; cur = cur.Parent {
		if cur.Type == "mod_item" {
			if ident := cur.ChildOfType("identifier"); ident != nil {
				names = append(names, ident.Text(src))
			}
		}
	}
	for i, j := 0, len(names)-1; i < j; i, j = i+1, j-1 {
		names[i], names[j] = names[j], names[i]
	}
	return names
}

func rustInModNamed(src []byte, n *syntax.Node, name string) bool {
	for _, m := range rustAncestorMods(src, n) {
		if m == name {
			return true
		}
	}
	return false
}

// rustFirstTypeName is rust._first_type_name: the first type_identifier in
// preorder (the node itself included), or "".
func rustFirstTypeName(src []byte, n *syntax.Node) string {
	name := ""
	n.Descendants(func(item *syntax.Node) bool {
		if item.Type == "type_identifier" {
			name = item.Text(src)
			return false
		}
		return true
	})
	return name
}

// rustImplType is rust._impl_type: the self type of an impl_item, the type
// after `for` when there is one.
func rustImplType(src []byte, impl *syntax.Node) string {
	sawFor := false
	chosen := ""
	for _, child := range impl.Children {
		if child.Type == "for" {
			sawFor = true
			chosen = ""
			continue
		}
		switch child.Type {
		case "type_identifier", "generic_type", "scoped_type_identifier":
			chosen = rustFirstTypeName(src, child)
			if sawFor {
				return chosen
			}
		}
	}
	return chosen
}

// RustCrateName is rust._crate_name: the package name of the nearest
// Cargo.toml with a [package] table above path (dashes become
// underscores), and that directory; ("crate", "") when there is none.
func RustCrateName(path string) (string, string, error) {
	current := filepath.Dir(discover.Resolve(path))
	for {
		cargo := filepath.Join(current, "Cargo.toml")
		if discover.IsFile(cargo) {
			raw, err := os.ReadFile(cargo)
			if err != nil {
				return "", "", err
			}
			if !utf8.Valid(raw) {
				return "", "", fmt.Errorf("%s: invalid utf-8", cargo)
			}
			text := source.UniversalNewlines(string(raw))
			if block := rustPackageBlock.FindStringSubmatch(text); block != nil {
				name := filepath.Base(current)
				if m := rustName.FindStringSubmatch(block[1]); m != nil {
					name = m[1]
				}
				return strings.ReplaceAll(name, "-", "_"), current, nil
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "crate", "", nil
		}
		current = parent
	}
}

// pathStem is pathlib's PurePath.stem.
func pathStem(path string) string {
	name := filepath.Base(path)
	if i := strings.LastIndex(name, "."); i > 0 && i < len(name)-1 {
		return name[:i]
	}
	return name
}

func rustParts(relative string) []string {
	parts := strings.Split(relative, "/")
	last := parts[len(parts)-1]
	if last == "mod.rs" {
		return parts[:len(parts)-1]
	}
	if strings.HasSuffix(last, ".rs") {
		parts[len(parts)-1] = last[:len(last)-3]
	}
	return parts
}

func rustWithCrate(crateName string, parts []string) []string {
	if len(parts) >= 2 && parts[0] == "bin" {
		return parts[1:]
	}
	return append([]string{crateName}, parts...)
}

// RustFileModules is rust._file_modules: the module path of a file from its
// place in the crate (crateRoot "" = no Cargo.toml found).
func RustFileModules(path, crateName, crateRoot string) []string {
	filePath := discover.Resolve(path)
	if crateRoot == "" {
		if stem := pathStem(filePath); stem == "lib" || stem == "main" {
			return []string{crateName}
		}
		return []string{crateName, pathStem(filePath)}
	}
	var relative string
	switch {
	case filePath == crateRoot:
		relative = "."
	case strings.HasPrefix(filePath, strings.TrimSuffix(crateRoot, "/")+"/"):
		relative = filepath.ToSlash(filePath[len(strings.TrimSuffix(crateRoot, "/"))+1:])
	default:
		return []string{crateName}
	}
	relative = strings.TrimPrefix(relative, "src/")
	if relative == "lib.rs" || relative == "main.rs" {
		return []string{crateName}
	}
	return rustWithCrate(crateName, rustParts(relative))
}

func rustNamespace(modules, extra []string, typeName string) string {
	parts := append(append([]string{}, modules...), extra...)
	if typeName != "" {
		parts = append(parts, typeName)
	}
	kept := parts[:0]
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, "::")
}

// rustImplOwner: the self type when n sits directly in an impl's body.
func rustImplOwner(src []byte, n *syntax.Node) string {
	parent := n.Parent
	if parent == nil || parent.Type != "declaration_list" {
		return ""
	}
	owner := parent.Parent
	if owner == nil || owner.Type != "impl_item" {
		return ""
	}
	return rustImplType(src, owner)
}

func rustRecordFunction(src []byte, n *syntax.Node, modules []string, path string) (model.Function, bool) {
	if n.Type != "function_item" || !rustHasBlock(n) {
		return model.Function{}, false
	}
	if n.Parent != nil && n.Parent.Type == "block" {
		return model.Function{}, false
	}
	if rustInModNamed(src, n, "tests") {
		return model.Function{}, false
	}
	ident := n.ChildOfType("identifier")
	if ident == nil {
		return model.Function{}, false
	}
	return model.Function{
		Name:       ident.Text(src),
		Namespace:  rustNamespace(modules, rustAncestorMods(src, n), rustImplOwner(src, n)),
		Complexity: Complexity(n, rustIsDecision, rustSkip),
		StartLine:  n.StartLine(),
		EndLine:    n.EndLine(),
		Path:       path,
		Language:   "rust",
		StartByte:  -1,
		EndByte:    -1,
	}, true
}

// RustFunctions is rust.functions_in_source.
func RustFunctions(source, path, projectRoot string) ([]model.Function, error) {
	tree, err := lang.ParseText("rust", source)
	if err != nil {
		return nil, err
	}
	located := absolutePath(path, projectRoot)
	crateName, crateRoot, err := RustCrateName(located)
	if err != nil {
		return nil, err
	}
	modules := RustFileModules(located, crateName, crateRoot)
	var found []model.Function
	tree.Root.Descendants(func(n *syntax.Node) bool {
		if f, ok := rustRecordFunction(tree.Source, n, modules, path); ok {
			found = append(found, f)
		}
		return true
	})
	return found, nil
}
