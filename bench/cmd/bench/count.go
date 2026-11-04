package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The discovery rules shared by crapper and mutator (docs/spec-crapper.md §2).
// dryer differs only in its extension table (no .js/.jsx/.mjs/.cjs), reported
// separately as dry_files.
var (
	skipDirs = set(".git", ".hg", ".svn", ".idea", ".venv", "venv", "node_modules", "vendor", "target",
		"dist", "build", "out", "coverage", ".metrics", "testdata", "__pycache__", ".clj-kondo",
		"test", "tests", "spec", "specs", "__tests__")
	extLang = map[string]string{
		".clj": "clojure", ".cljc": "clojure", ".cljs": "clojure", ".bb": "clojure",
		".java": "java", ".go": "go", ".rs": "rust", ".py": "python",
		".ts": "typescript", ".tsx": "typescript", ".mts": "typescript", ".cts": "typescript",
		".js": "javascript", ".jsx": "javascript", ".mjs": "javascript", ".cjs": "javascript",
	}
	jsExts = []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"}
)

func set(xs ...string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

func isTestName(name string) bool {
	if strings.HasSuffix(name, "_test.go") || name == "conftest.py" || strings.HasSuffix(name, "_test.py") ||
		(strings.HasPrefix(name, "test_") && strings.HasSuffix(name, ".py")) {
		return true
	}
	for _, s := range []string{"_test.clj", "_test.cljc", "_test.cljs", "_test.bb"} {
		if strings.HasSuffix(name, s) {
			return true
		}
	}
	for _, k := range []string{".test", ".spec"} {
		for _, e := range jsExts {
			if strings.HasSuffix(name, k+e) {
				return true
			}
		}
	}
	return false
}

// LangCount is one language's share of the analysed set.
type LangCount struct {
	Language string `json:"language"`
	Files    int    `json:"files"`
	Lines    int    `json:"lines"`
	Code     int    `json:"nonblank_lines"`
	Bytes    int64  `json:"bytes"`
}

func countMain(args []string) error {
	fset := flag.NewFlagSet("count", flag.ExitOnError)
	root := fset.String("root", ".", "project root")
	name := fset.String("name", "", "label in the JSON output")
	asJSON := fset.Bool("json", false, "print one JSON object instead of a table")
	fset.Parse(args)
	counts := map[string]*LangCount{}
	dryFiles := 0
	err := filepath.WalkDir(*root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if p != *root && skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		n := d.Name()
		ext := filepath.Ext(n)
		if strings.HasPrefix(n, ".") && strings.Count(n, ".") == 1 {
			ext = ""
		}
		lang, ok := extLang[ext]
		if !ok || strings.HasSuffix(n, ".d.ts") || isTestName(n) {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		c := counts[lang]
		if c == nil {
			c = &LangCount{Language: lang}
			counts[lang] = c
		}
		c.Files++
		c.Bytes += int64(len(b))
		for _, line := range bytes.Split(b, []byte("\n")) {
			c.Lines++
			if len(bytes.TrimSpace(line)) > 0 {
				c.Code++
			}
		}
		if len(b) > 0 && b[len(b)-1] == '\n' {
			c.Lines--
		}
		if lang != "javascript" {
			dryFiles++
		}
		return nil
	})
	if err != nil {
		return err
	}
	var list []LangCount
	tot := LangCount{Language: "total"}
	for _, c := range counts {
		list = append(list, *c)
		tot.Files += c.Files
		tot.Lines += c.Lines
		tot.Code += c.Code
		tot.Bytes += c.Bytes
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Lines > list[j].Lines })
	if *asJSON {
		return json.NewEncoder(os.Stdout).Encode(map[string]any{
			"name": *name, "root": *root, "languages": list, "total": tot, "dry_files": dryFiles,
		})
	}
	for _, c := range append(list, tot) {
		fmt.Printf("%-11s %6d files %9d lines %9d non-blank\n", c.Language, c.Files, c.Lines, c.Code)
	}
	fmt.Printf("dryer walks %d of these files (no .js/.jsx/.mjs/.cjs)\n", dryFiles)
	return nil
}
