// Package lang is the parser registry: grammar name -> hand-written parser
// producing a tree-sitter-shaped *syntax.Tree.
//
// Grammar names are tree-sitter-language-pack names: python, typescript,
// tsx, javascript, go, java, rust. To add a language, write
// internal/lang/<name> exposing `func Parse(src []byte) (*syntax.Node, error)`
// and add it to the registry below (see docs/ARCHITECTURE.md).
package lang

import (
	"fmt"
	"path/filepath"

	"github.com/matheusAle/ub-mode/ubgate/internal/lang/golang"
	"github.com/matheusAle/ub-mode/ubgate/internal/lang/java"
	"github.com/matheusAle/ub-mode/ubgate/internal/lang/python"
	"github.com/matheusAle/ub-mode/ubgate/internal/lang/rust"
	"github.com/matheusAle/ub-mode/ubgate/internal/lang/typescript"
	"github.com/matheusAle/ub-mode/ubgate/internal/syntax"
)

// ParseFunc parses decoded, newline-normalized source (internal/source) and
// returns the root node with Parent pointers and Points filled in
// (syntax.Finish). It returns an error for input it cannot handle.
type ParseFunc func(src []byte) (*syntax.Node, error)

var registry = map[string]ParseFunc{
	"typescript": typescript.ParseTypeScript,
	"tsx":        typescript.ParseTSX,
	"javascript": typescript.ParseJavaScript,
	"java":       java.Parse,
	"python":     python.Parse,
	"rust":       rust.Parse,
	"go":         golang.Parse,
}

// Supported reports whether a parser is registered for the grammar.
func Supported(grammar string) bool { _, ok := registry[grammar]; return ok }

// Parse parses src with the named grammar.
func Parse(grammar string, src []byte) (*syntax.Tree, error) {
	parse, ok := registry[grammar]
	if !ok {
		return nil, fmt.Errorf("no parser for grammar %q", grammar)
	}
	root, err := parse(src)
	if err != nil {
		return nil, err
	}
	return &syntax.Tree{Root: root, Source: src, Language: grammar}, nil
}

// ParseText parses already-decoded text (the result of source.Read).
func ParseText(grammar, text string) (*syntax.Tree, error) {
	return Parse(grammar, []byte(text))
}

// GrammarFor maps a file name to its grammar with the rule all three
// upstream tools share: .tsx -> tsx; .js .jsx .mjs .cjs -> javascript;
// .ts .mts .cts -> typescript; .py .go .java .rs -> python go java rust.
// It returns "" for anything else (including .d.ts handling, which is a
// discovery concern, not a grammar one).
func GrammarFor(path string) string {
	switch filepath.Ext(path) {
	case ".py":
		return "python"
	case ".go":
		return "go"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".tsx":
		return "tsx"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".ts", ".mts", ".cts":
		return "typescript"
	}
	return ""
}
