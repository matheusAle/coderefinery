package react

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/matheusAle/coderefinery/internal/lang"
	"github.com/matheusAle/coderefinery/internal/source"
	"github.com/matheusAle/coderefinery/internal/syntax"
)

// resolver follows props types across the project's modules. One resolver
// lives for a whole run, so every file is read and parsed at most once
// (the analysed files included) and every tsconfig is read at most once.
type resolver struct {
	root    string                // absolute project root; modules outside it are not followed
	files   map[string]*tsModule  // absolute path -> module; nil: missing or unparsable
	configs map[string]*pathsConf // directory -> the tsconfig paths in force there (nil: none)
}

func newResolver(root string) *resolver {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return &resolver{root: root, files: map[string]*tsModule{}, configs: map[string]*pathsConf{}}
}

// tsModule indexes what a module declares, imports and exports by name.
type tsModule struct {
	path     string
	tree     *syntax.Tree
	src      []byte
	decls    map[string][]*syntax.Node // top-level interface / type alias declarations (merged interfaces: several)
	defName  string                    // the local name behind `export default X` / `export default interface X`
	imports  map[string]importRef      // local name -> where it comes from
	exports  map[string]string         // `export { X as Y }` (no source): Y -> X
	reexport map[string]importRef      // `export { X as Y } from "m"`: Y -> {m, X}
	stars    []string                  // `export * from "m"`: m
}

// importRef is a binding taken from another module: name is the exported
// name, "default", or "*" for a namespace import.
type importRef struct{ spec, name string }

// module returns the module at an absolute path, reading and parsing it on
// first use.
func (r *resolver) module(path string) *tsModule {
	if m, ok := r.files[path]; ok {
		return m
	}
	r.files[path] = nil // a cycle back here while indexing finds nothing
	text, err := source.Read(path)
	if err != nil {
		return nil
	}
	tree, err := lang.ParseText(lang.GrammarFor(path), text)
	if err != nil {
		return nil
	}
	return r.add(path, tree)
}

// add indexes an already parsed module (an analysed file) and caches it.
func (r *resolver) add(path string, tree *syntax.Tree) *tsModule {
	m := &tsModule{path: path, tree: tree, src: tree.Source, decls: map[string][]*syntax.Node{},
		imports: map[string]importRef{}, exports: map[string]string{}, reexport: map[string]importRef{}}
	for _, c := range tree.Root.Children {
		switch c.Type {
		case "import_statement":
			m.indexImport(c)
		case "export_statement":
			m.indexExport(c)
		case "interface_declaration", "type_alias_declaration":
			m.declare(c)
		}
	}
	r.files[path] = m
	return m
}

func (m *tsModule) declare(d *syntax.Node) {
	if name := d.ChildByFieldName("name"); name != nil {
		m.decls[name.Text(m.src)] = append(m.decls[name.Text(m.src)], d)
	}
}

func (m *tsModule) specOf(stmt *syntax.Node) string {
	s := stmt.ChildByFieldName("source")
	if s == nil {
		return ""
	}
	return unquote(m.src, s)
}

func (m *tsModule) indexImport(stmt *syntax.Node) {
	spec, clause := m.specOf(stmt), stmt.ChildOfType("import_clause")
	if spec == "" || clause == nil {
		return
	}
	for _, c := range clause.NamedChildren() {
		switch c.Type {
		case "identifier": // import Def from "m"
			m.imports[c.Text(m.src)] = importRef{spec, "default"}
		case "namespace_import": // import * as NS from "m"
			if id := c.ChildOfType("identifier"); id != nil {
				m.imports[id.Text(m.src)] = importRef{spec, "*"}
			}
		case "named_imports": // import { A, B as C, type D } from "m"
			for _, s := range c.NamedChildren() {
				if name, local := m.specifier(s); name != "" {
					m.imports[local] = importRef{spec, name}
				}
			}
		}
	}
}

// specifier reads `name` or `name as alias` (import or export specifier).
func (m *tsModule) specifier(s *syntax.Node) (name, alias string) {
	if s.Type != "import_specifier" && s.Type != "export_specifier" {
		return "", ""
	}
	n := s.ChildByFieldName("name")
	if n == nil {
		return "", ""
	}
	name, alias = unquote(m.src, n), unquote(m.src, n)
	if a := s.ChildByFieldName("alias"); a != nil {
		alias = unquote(m.src, a)
	}
	return name, alias
}

func (m *tsModule) indexExport(stmt *syntax.Node) {
	spec := m.specOf(stmt)
	if d := stmt.ChildByFieldName("declaration"); d != nil {
		if d.Type == "interface_declaration" || d.Type == "type_alias_declaration" {
			m.declare(d)
			if stmt.HasChildOfType("default") {
				if name := d.ChildByFieldName("name"); name != nil {
					m.defName = name.Text(m.src)
				}
			}
		}
		return
	}
	if v := stmt.ChildByFieldName("value"); v != nil { // export default X
		if v.Type == "identifier" {
			m.defName = v.Text(m.src)
		}
		return
	}
	clause := stmt.ChildOfType("export_clause")
	switch {
	case clause != nil:
		for _, s := range clause.NamedChildren() {
			name, alias := m.specifier(s)
			switch {
			case name == "":
			case spec != "":
				m.reexport[alias] = importRef{spec, name}
			case alias == "default":
				m.defName = name
			default:
				m.exports[alias] = name
			}
		}
	case spec == "":
	case stmt.HasChildOfType("namespace_export"): // export * as NS from "m"
		if id := stmt.ChildOfType("namespace_export").ChildOfType("identifier"); id != nil {
			m.reexport[id.Text(m.src)] = importRef{spec, "*"}
		}
	case stmt.HasChildOfType("*"):
		m.stars = append(m.stars, spec)
	}
}

// moduleExts are tried, in order, after a specifier without a known
// extension, then after `<specifier>/index`.
var moduleExts = []string{".tsx", ".ts", ".d.ts", ".jsx", ".js", ".mts", ".cts"}

// resolveSpec maps an import specifier to a module file inside the project:
// relative paths, then the nearest tsconfig `paths`, then its `baseUrl`.
// Packages (node_modules) are not followed.
func (r *resolver) resolveSpec(from *tsModule, spec string) *tsModule {
	dir := filepath.Dir(from.path)
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "../") || spec == "." || spec == ".." {
		return r.tryModule(filepath.Join(dir, spec))
	}
	conf := r.pathsFor(dir)
	if conf == nil {
		return nil
	}
	for _, p := range conf.patterns {
		star, ok := p.match(spec)
		if !ok {
			continue
		}
		for _, target := range p.targets {
			if m := r.tryModule(filepath.Join(conf.pathsBase, strings.Replace(target, "*", star, 1))); m != nil {
				return m
			}
		}
	}
	if conf.baseURL != "" {
		return r.tryModule(filepath.Join(conf.baseURL, spec))
	}
	return nil
}

func (r *resolver) tryModule(base string) *tsModule {
	if !r.inProject(base) {
		return nil
	}
	var candidates []string
	switch ext := filepath.Ext(base); ext {
	case ".ts", ".tsx", ".mts", ".cts":
		candidates = append(candidates, base)
	case ".js", ".jsx", ".mjs", ".cjs": // ESM style "./x.js" naming "./x.ts"
		stem := strings.TrimSuffix(base, ext)
		candidates = append(candidates, stem+".ts", stem+".tsx", stem+".d.ts", base)
	}
	for _, ext := range moduleExts {
		candidates = append(candidates, base+ext)
	}
	for _, ext := range moduleExts {
		candidates = append(candidates, filepath.Join(base, "index"+ext))
	}
	for _, c := range candidates {
		if m, ok := r.files[c]; ok && m != nil {
			return m
		}
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return r.module(c)
		}
	}
	return nil
}

func (r *resolver) inProject(path string) bool {
	rel, err := filepath.Rel(r.root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) &&
		!strings.Contains(filepath.ToSlash(path), "/node_modules/")
}

// pathsConf is the module resolution a tsconfig sets up.
type pathsConf struct {
	pathsBase string // where `paths` targets are relative to
	patterns  []pathPattern
	baseURL   string // absolute; "" when unset
}

type pathPattern struct {
	prefix, suffix string
	wildcard       bool
	targets        []string
}

func (p pathPattern) match(spec string) (string, bool) {
	if !p.wildcard {
		return "", spec == p.prefix
	}
	if len(spec) < len(p.prefix)+len(p.suffix) || !strings.HasPrefix(spec, p.prefix) || !strings.HasSuffix(spec, p.suffix) {
		return "", false
	}
	return spec[len(p.prefix) : len(spec)-len(p.suffix)], true
}

// pathsFor finds the nearest tsconfig.json (or jsconfig.json) from dir up to
// the project root.
func (r *resolver) pathsFor(dir string) *pathsConf {
	if c, ok := r.configs[dir]; ok {
		return c
	}
	var conf *pathsConf
	found := false
	for _, name := range []string{"tsconfig.json", "jsconfig.json"} {
		if path := filepath.Join(dir, name); fileExists(path) {
			conf, found = loadPathsConf(path, 0), true
			break
		}
	}
	if !found && dir != r.root && r.inProject(filepath.Dir(dir)) {
		conf = r.pathsFor(filepath.Dir(dir))
	}
	r.configs[dir] = conf
	return conf
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// loadPathsConf reads compilerOptions.paths and baseUrl, following a
// relative `extends` chain (a package `extends` is not followed): the
// nearest file that sets each option wins; paths are relative to baseUrl
// when set, else to the file that declares them.
func loadPathsConf(path string, depth int) *pathsConf {
	raw, err := os.ReadFile(path)
	if err != nil || depth > 8 {
		return nil
	}
	var cfg struct {
		Extends         string `json:"extends"`
		CompilerOptions struct {
			BaseURL *string             `json:"baseUrl"`
			Paths   map[string][]string `json:"paths"`
		} `json:"compilerOptions"`
	}
	if json.Unmarshal(stripJSONC(raw), &cfg) != nil {
		return nil
	}
	conf := &pathsConf{}
	if ext := cfg.Extends; strings.HasPrefix(ext, "./") || strings.HasPrefix(ext, "../") {
		if !strings.HasSuffix(ext, ".json") {
			ext += ".json"
		}
		if parent := loadPathsConf(filepath.Join(filepath.Dir(path), ext), depth+1); parent != nil {
			conf = parent
		}
	}
	dir := filepath.Dir(path)
	if cfg.CompilerOptions.BaseURL != nil {
		conf.baseURL = filepath.Join(dir, *cfg.CompilerOptions.BaseURL)
	}
	if cfg.CompilerOptions.Paths != nil {
		conf.pathsBase, conf.patterns = dir, nil
		for key, targets := range cfg.CompilerOptions.Paths {
			p := pathPattern{prefix: key, targets: targets}
			if i := strings.IndexByte(key, '*'); i >= 0 {
				p = pathPattern{prefix: key[:i], suffix: key[i+1:], wildcard: true, targets: targets}
			}
			conf.patterns = append(conf.patterns, p)
		}
		sortPatterns(conf.patterns)
	}
	if conf.baseURL != "" {
		conf.pathsBase = conf.baseURL
	}
	return conf
}

// sortPatterns orders exact keys first, then wildcards by longest prefix,
// as TypeScript picks the most specific match.
func sortPatterns(ps []pathPattern) {
	less := func(a, b pathPattern) bool {
		if a.wildcard != b.wildcard {
			return !a.wildcard
		}
		if len(a.prefix) != len(b.prefix) {
			return len(a.prefix) > len(b.prefix)
		}
		return a.prefix+"*"+a.suffix < b.prefix+"*"+b.suffix
	}
	for i := 1; i < len(ps); i++ {
		for j := i; j > 0 && less(ps[j], ps[j-1]); j-- {
			ps[j], ps[j-1] = ps[j-1], ps[j]
		}
	}
}

// stripJSONC removes // and /* */ comments and trailing commas (tsconfig
// is JSON with comments) outside strings.
func stripJSONC(raw []byte) []byte {
	var out bytes.Buffer
	inString := false
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case inString:
			out.WriteByte(c)
			if c == '\\' && i+1 < len(raw) {
				i++
				out.WriteByte(raw[i])
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
			out.WriteByte(c)
		case c == '/' && i+1 < len(raw) && raw[i+1] == '/':
			for i < len(raw) && raw[i] != '\n' {
				i++
			}
			out.WriteByte('\n')
		case c == '/' && i+1 < len(raw) && raw[i+1] == '*':
			end := bytes.Index(raw[i+2:], []byte("*/"))
			if end < 0 {
				i = len(raw)
			} else {
				i += end + 3
			}
			out.WriteByte(' ')
		case c == '}' || c == ']':
			trimTrailingComma(&out)
			out.WriteByte(c)
		default:
			out.WriteByte(c)
		}
	}
	return out.Bytes()
}

func trimTrailingComma(out *bytes.Buffer) {
	b := out.Bytes()
	i := len(b) - 1
	for i >= 0 && (b[i] == ' ' || b[i] == '\t' || b[i] == '\n' || b[i] == '\r') {
		i--
	}
	if i >= 0 && b[i] == ',' {
		out.Truncate(i)
	}
}
