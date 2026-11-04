package dry

import (
	"cmp"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/matheusAle/coderefinery/internal/cliutil"
	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/edn"
	"github.com/matheusAle/coderefinery/internal/lang"
	"github.com/matheusAle/coderefinery/internal/pyfmt"
	"github.com/matheusAle/coderefinery/internal/source"
	"github.com/matheusAle/coderefinery/internal/syntax"
)

// dry --jsx (docs/design-react.md §12): duplicated JSX element subtrees
// inside and across components, the "extract a component" signal. Every
// element (and fragment) of a .tsx/.jsx/.js file is a candidate; it is
// normalized with dryer's normalizer plus the JSX rules below, fingerprinted
// by the same interner and paired by the same size-pruned parallel Jaccard
// loop (groupHits). Pairs where one side holds the other are dropped, and a
// pair is only reported when no reported pair encloses it. The function
// report and .metrics/dry.edn are untouched; the JSX pairs get their own
// section and .metrics/dry-jsx.edn.

// JSXOptions are the --jsx settings. Any --jsx-* flag turns --jsx on.
type JSXOptions struct {
	Enabled     bool
	Threshold   float64
	MinElements int
	MinNodes    int
}

// DefaultJSXOptions are the --jsx defaults (calibration: design-react §12).
func DefaultJSXOptions() JSXOptions {
	return JSXOptions{Enabled: true, Threshold: 0.82, MinElements: 4, MinNodes: 40}
}

// jsxArg parses a --jsx flag at args[i]: the index after it, or 0 when
// args[i] is not one; msg is a usage error.
func (o *Options) jsxArg(args []string, i int) (next int, msg string) {
	arg := args[i]
	if arg != "--jsx" && arg != "--jsx-threshold" && arg != "--jsx-min-elements" && arg != "--jsx-min-nodes" {
		return 0, ""
	}
	if !o.JSX.Enabled {
		o.JSX = DefaultJSXOptions()
	}
	if arg == "--jsx" {
		return i + 1, ""
	}
	v, err := cliutil.Value(args, i, arg)
	if err != nil {
		return 0, err.Error()
	}
	if arg == "--jsx-threshold" {
		t, err := pyfmt.ParseFloat(v)
		if err != nil {
			return 0, arg + " requires a number"
		}
		o.JSX.Threshold = t
		return i + 2, ""
	}
	n, err := pyfmt.ParseInt(v)
	if err != nil || n < 0 {
		return 0, arg + " requires a non-negative integer"
	}
	if arg == "--jsx-min-elements" {
		o.JSX.MinElements = n
	} else {
		o.JSX.MinNodes = n
	}
	return i + 2, ""
}

// isJSXFile: the extensions that may hold JSX (.ts cannot).
func isJSXFile(path string) bool {
	switch filepath.Ext(path) {
	case ".tsx", ".jsx", ".js":
		return true
	}
	return false
}

// withJSXFiles widens the selection walk to JSX files: dry's own languages
// as before, plus .jsx/.js (test files only under --tests). ScanFiles skips
// the files dryer has no language for, so the function report is the same.
func withJSXFiles(tests bool, keep func(string) bool) (func([]string) []string, func(string) bool) {
	wider := func(p string) bool { return keep(p) || isJSXFile(p) && (tests || !discover.IsTestFile(p)) }
	return func(roots []string) []string { return discover.IterFilesWith(roots, wider, tests) }, wider
}

var jsxElementTypes = set("jsx_element", "jsx_self_closing_element")

// svgTags are SVG drawing elements (plus the fe* filter primitives). A
// subtree made only of them is an icon or a drawing: its content is in the
// attribute values normalization drops, so any two icons with as many
// paths look alike. Such subtrees are not candidates (excalidraw's
// icons.tsx alone gave 742 of 771 pairs; design-react §12).
var svgTags = set("svg", "g", "path", "circle", "ellipse", "line", "polyline", "polygon", "rect", "defs",
	"clipPath", "mask", "use", "symbol", "linearGradient", "radialGradient", "stop", "pattern", "marker",
	"text", "tspan", "textPath", "title", "desc", "filter", "image", "foreignObject")

// jsxCandidate is one normalized element or fragment.
type jsxCandidate struct {
	Span
	Start, End int    // byte range, for containment
	Tag        string // "" for a fragment
	Elements   int    // elements in the subtree, itself included (fragments do not count)
	Markup     int    // those that are not SVG drawing elements (svgTags)
	Nodes      int
	Norm       *Value
}

// jsxScan is the normalizer's JSX state for one file: it records every
// element it normalizes, nested ones included.
type jsxScan struct {
	src      []byte
	file     string
	elements int
	markup   int
	found    []jsxCandidate
}

// element normalizes a jsx_element or jsx_self_closing_element:
//
//	[:element [:tag "Name"] attribute... child...]   or   [:fragment child...]
//
// Tag names stay (which component is used matters). An attribute is
// [:attr "name"]: its value is dropped (string, className, {expression}:
// values become props when the markup is extracted), unless the value
// holds JSX (icon={<Icon/>}, a render prop), which is normalized like a
// child. Children: elements, and {expression}s that hold more than
// comments, normalized by dryer's rules (locals become :symbol, literals
// :literal, callees keep their names; JSX inside them is normalized here);
// text and character references are dropped. <A/> and <A></A> normalize
// alike. A spread {...props} attribute stays as an expression; key is
// dropped.
func (s *jsxScan) element(z normalizer, n *syntax.Node) *Value {
	count, markup := s.elements, s.markup
	open := n
	if n.Type == "jsx_element" {
		open = n.ChildByFieldName("open_tag")
	}
	tag := ""
	out := List(K("fragment"))
	if name := open.ChildByFieldName("name"); name != nil {
		tag = strings.Join(strings.Fields(name.Text(s.src)), "")
		out = List(K("element"), List(K("tag"), Str(tag)))
		s.elements++
		if !svgTags[tag] && !strings.HasPrefix(tag, "fe") {
			s.markup++
		}
	}
	for _, a := range open.ChildrenByFieldName("attribute") {
		out.append(s.attribute(z, a))
	}
	if n.Type == "jsx_element" {
		for _, c := range n.Children {
			switch {
			case jsxElementTypes[c.Type]:
				out.append(s.element(z, c))
			case c.Type == "jsx_expression":
				out.append(s.expression(z, c))
			}
		}
	}
	s.found = append(s.found, jsxCandidate{
		Span:  Span{File: s.file, StartLine: n.StartLine(), EndLine: n.EndLine()},
		Start: n.StartByte, End: n.EndByte, Tag: tag,
		Elements: s.elements - count, Markup: s.markup - markup, Nodes: NodeCount(out), Norm: out,
	})
	return out
}

func (s *jsxScan) attribute(z normalizer, a *syntax.Node) *Value {
	if a.Type == "jsx_expression" { // {...props}
		return s.expression(z, a)
	}
	name := a.Children[0].Text(s.src)
	if name == "key" { // reconciliation, not markup: list items have it, the same markup outside a .map does not
		return nil
	}
	out := List(K("attr"), Str(name))
	if value := a.Children[len(a.Children)-1]; len(a.Children) == 3 && holdsJSX(value) {
		out.append(z.norm(value, false))
	}
	return out
}

// holdsJSX: n is or contains a JSX element.
func holdsJSX(n *syntax.Node) bool {
	found := false
	syntax.Walk(n, func(c *syntax.Node) bool {
		found = found || jsxElementTypes[c.Type]
		return !found
	})
	return found
}

// expression is dryer's normalization of a {...}; nil when it holds only
// comments (or nothing).
func (s *jsxScan) expression(z normalizer, n *syntax.Node) *Value {
	v := z.norm(n, false)
	if v == nil || len(v.Items) < 2 {
		return nil
	}
	return v
}

// jsxCandidatesIn normalizes every element of one source text, outermost
// first in document order. A file the parser rejects has none (A2).
func jsxCandidatesIn(text, path, file string) []jsxCandidate {
	tree, err := lang.ParseText(lang.GrammarFor(path), text)
	if err != nil {
		return nil
	}
	s := &jsxScan{src: tree.Source, file: file}
	z := normalizer{src: tree.Source, jsx: s}
	syntax.Walk(tree.Root, func(n *syntax.Node) bool {
		if jsxElementTypes[n.Type] {
			z.norm(n, false)
			return false
		}
		return true
	})
	slices.SortFunc(s.found, func(a, b jsxCandidate) int { return cmp.Compare(a.Start, b.Start) })
	return s.found
}

// JSXSide is one side of a JSX duplicate.
type JSXSide struct {
	Span
	Tag             string
	Elements, Nodes int
}

// JSXDuplicate is a reported pair of element subtrees.
type JSXDuplicate struct {
	Score       float64
	Left, Right JSXSide
}

// findJSXDuplicates pairs the candidates (all files, in file then
// document order) that pass the size filters.
func findJSXDuplicates(all []jsxCandidate, o JSXOptions) []JSXDuplicate {
	var kept []jsxCandidate
	for _, c := range all {
		if c.Elements >= o.MinElements && c.Nodes >= o.MinNodes && c.Markup > 0 {
			kept = append(kept, c)
		}
	}
	fp := NewFingerprinter(false)
	entries := make([]Entry, len(kept))
	for i, c := range kept {
		entries[i] = Entry{Language: "jsx", Span: c.Span, Nodes: c.Nodes, Fingerprints: fp.Set(c.Norm)}
	}
	parent := enclosing(kept)
	hits := groupHits(entries, 0, o.Threshold)
	reported := dedupe(kept, parent, hits)
	slices.SortFunc(reported, func(a, b pairHit) int {
		l, r, m, n := kept[a.left], kept[a.right], kept[b.left], kept[b.right]
		return cmp.Or(cmp.Compare(b.dup.Score, a.dup.Score),
			cmp.Compare(l.File, m.File), cmp.Compare(l.StartLine, m.StartLine),
			cmp.Compare(r.File, n.File), cmp.Compare(r.StartLine, n.StartLine),
			cmp.Compare(a.left, b.left), cmp.Compare(a.right, b.right))
	})
	out := make([]JSXDuplicate, len(reported))
	for i, h := range reported {
		out[i] = JSXDuplicate{Score: h.dup.Score, Left: side(kept[h.left]), Right: side(kept[h.right])}
	}
	return out
}

func side(c jsxCandidate) JSXSide {
	return JSXSide{Span: c.Span, Tag: c.Tag, Elements: c.Elements, Nodes: c.Nodes}
}

// holds: outer's subtree contains inner (same file, byte range inside).
func holds(outer, inner jsxCandidate) bool {
	return outer.File == inner.File && outer.Start <= inner.Start && inner.End <= outer.End
}

// enclosing gives each candidate the index of the nearest candidate that
// holds it, -1 at the top. Candidates are in file then start order.
func enclosing(cands []jsxCandidate) []int {
	parent := make([]int, len(cands))
	var stack []int
	for i, c := range cands {
		for len(stack) > 0 && !holds(cands[stack[len(stack)-1]], c) {
			stack = stack[:len(stack)-1]
		}
		parent[i] = -1
		if len(stack) > 0 {
			parent[i] = stack[len(stack)-1]
		}
		stack = append(stack, i)
	}
	return parent
}

// dedupe drops the pairs where one side holds the other, then every pair
// another kept-so-far valid pair dominates: one whose sides are each the
// same candidate as, or an ancestor/descendant of, this pair's sides.
func dedupe(kept []jsxCandidate, parent []int, hits []pairHit) []pairHit {
	var valid []pairHit
	for _, h := range hits {
		if !holds(kept[h.left], kept[h.right]) && !holds(kept[h.right], kept[h.left]) {
			valid = append(valid, h)
		}
	}
	byCandidate := map[int][]int{}
	for i, h := range valid {
		byCandidate[h.left] = append(byCandidate[h.left], i)
		byCandidate[h.right] = append(byCandidate[h.right], i)
	}
	related := func(a, b int) bool { return a == b || holds(kept[a], kept[b]) || holds(kept[b], kept[a]) }
	// dominated: a pair q at an ancestor-or-self of one side and related to
	// the other. That covers every q not strictly inside p on both sides
	// (those never dominate).
	dominated := func(i int) bool {
		p := valid[i]
		for _, sides := range [2][2]int{{p.left, p.right}, {p.right, p.left}} {
			this, other := sides[0], sides[1]
			for a := this; a >= 0; a = parent[a] {
				for _, qi := range byCandidate[a] {
					q := valid[qi]
					b := q.left + q.right - a
					if qi != i && related(b, other) && dominates(kept, q, p, a, this, b, other) {
						return true
					}
				}
			}
		}
		return false
	}
	var out []pairHit
	for i := range valid {
		if !dominated(i) {
			out = append(out, valid[i])
		}
	}
	return out
}

// dominates: q, whose candidate a lines up with p's side this and b with
// p's side other, wins over p. A q that strictly encloses p on both sides
// always wins (the largest enclosing duplicate is the one to extract);
// otherwise (one side shared, or one side larger and one smaller) the
// higher score wins, then the larger pair.
func dominates(kept []jsxCandidate, q, p pairHit, a, this, b, other int) bool {
	if a != this && b != other && holds(kept[a], kept[this]) && holds(kept[b], kept[other]) {
		return true
	}
	if q.dup.Score != p.dup.Score {
		return q.dup.Score > p.dup.Score
	}
	return q.dup.LeftNodes+q.dup.RightNodes > p.dup.LeftNodes+p.dup.RightNodes
}

// JSXInFiles reads the JSX files among files and returns their duplicates.
func JSXInFiles(files []string, root string, o JSXOptions) ([]JSXDuplicate, error) {
	var all []jsxCandidate
	for _, path := range files {
		if !isJSXFile(path) {
			continue
		}
		text, err := source.Read(path)
		if err != nil {
			return nil, err
		}
		all = append(all, jsxCandidatesIn(text, path, displayPath(path, root))...)
	}
	return findJSXDuplicates(all, o), nil
}

func tagLabel(tag string) string { return "<" + tag + ">" }

// FormatJSXText is the "Duplicate JSX" section of the text report.
func FormatJSXText(dups []JSXDuplicate) string {
	head := "\nDuplicate JSX\n=============\n\n"
	if len(dups) == 0 {
		return head + "No duplicate JSX candidates found.\n"
	}
	sideText := func(s JSXSide) string {
		return fmt.Sprintf("  %s:%d-%d %s %d elements", s.File, s.StartLine, s.EndLine, tagLabel(s.Tag), s.Elements)
	}
	blocks := make([]string, len(dups))
	for i, d := range dups {
		blocks[i] = fmt.Sprintf("DUPLICATE score=%.2f\n%s\n%s", d.Score, sideText(d.Left), sideText(d.Right))
	}
	return head + strings.Join(blocks, "\n\n") + "\n"
}

// RenderJSXEDN is .metrics/dry-jsx.edn (and the --edn stdout section).
func RenderJSXEDN(dups []JSXDuplicate) string {
	if len(dups) == 0 {
		return "{:jsx-candidates []}\n"
	}
	sideEDN := func(s JSXSide) string {
		return fmt.Sprintf("{:file %s, :start-line %d, :end-line %d, :tag %s, :elements %d, :nodes %d}",
			edn.String(s.File), s.StartLine, s.EndLine, edn.String(s.Tag), s.Elements, s.Nodes)
	}
	rows := make([]string, len(dups))
	for i, d := range dups {
		rows[i] = " {:score " + ednFloat(d.Score) + "\n  :left " + sideEDN(d.Left) + "\n  :right " + sideEDN(d.Right) + "}"
	}
	return "{:jsx-candidates [\n" + strings.Join(rows, "\n") + "\n]}\n"
}

// runJSX prints the JSX section and writes .metrics/dry-jsx.edn; it
// returns the metrics path.
func runJSX(opts Options, files []string, root string, stdout io.Writer) (string, error) {
	dups, err := JSXInFiles(files, root, opts.JSX)
	if err != nil {
		return "", err
	}
	rendered := RenderJSXEDN(dups)
	path := filepath.Join(root, ".metrics", "dry-jsx.edn")
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(rendered), 0o666); err != nil {
		return "", err
	}
	if opts.Format == "edn" {
		fmt.Fprint(stdout, rendered)
	} else {
		fmt.Fprint(stdout, FormatJSXText(dups))
	}
	return path, nil
}
