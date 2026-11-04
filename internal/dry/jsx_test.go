package dry

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// dry --jsx: normalization, candidate sizes, pairing and the nested-pair
// rules, with exact values.

func expectCandidate(t *testing.T, got jsxCandidate, tag string, elements, nodes int, norm string) {
	t.Helper()
	if got.Tag != tag || got.Elements != elements || got.Nodes != nodes || Pr(got.Norm) != norm {
		t.Errorf("candidate:\n got <%s> elements=%d nodes=%d %s\nwant <%s> elements=%d nodes=%d %s",
			got.Tag, got.Elements, got.Nodes, Pr(got.Norm), tag, elements, nodes, norm)
	}
}

func expectPairs(t *testing.T, got []JSXDuplicate, want ...JSXDuplicate) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pairs:\n got %+v\nwant %+v", got, want)
	}
}

func jsxSide(file string, start, end int, tag string, elements, nodes int) JSXSide {
	return JSXSide{Span: Span{File: file, StartLine: start, EndLine: end}, Tag: tag, Elements: elements, Nodes: nodes}
}

func TestJSXNormalizationKeepsTagsAndAttributeNamesOnly(t *testing.T) {
	src := `const A = () => <Card title="x" className={cx("a")} onClick={() => go(1)} icon={<Icon size={2} />}>Hello &amp; {user.name}{/* c */}<b/></Card>;`
	got := jsxCandidatesIn(src, "a.tsx", "a.tsx")
	if len(got) != 3 {
		t.Fatalf("%d candidates", len(got))
	}
	// text, the comment-only {} and every non-JSX attribute value are gone;
	// the JSX inside icon={...} stays, and counts as an element
	expectCandidate(t, got[0], "Card", 3, 38, `[:element [:tag "Card"] [:attr "title"] [:attr "className"] [:attr "onClick"] `+
		`[:attr "icon" [:jsx_expression [:element [:tag "Icon"] [:attr "size"]]]] `+
		`[:jsx_expression [:member_expression :symbol :symbol]] [:element [:tag "b"]]]`)
	expectCandidate(t, got[1], "Icon", 1, 8, `[:element [:tag "Icon"] [:attr "size"]]`)
	expectCandidate(t, got[2], "b", 1, 5, `[:element [:tag "b"]]`)
	if got[0].Span != (Span{File: "a.tsx", StartLine: 1, EndLine: 1}) || got[0].Start != 16 || got[0].End != 142 {
		t.Errorf("span %+v %d-%d", got[0].Span, got[0].Start, got[0].End)
	}
}

func TestJSXNormalizationOfFragmentsListsAndKeys(t *testing.T) {
	// <A x="1"/> and <A x={y}></A> normalize alike; a fragment is not an element
	got := jsxCandidatesIn(`const B = () => <><A x="1"/><A x={y}></A></>;`, "b.tsx", "b.tsx")
	if len(got) != 3 {
		t.Fatalf("%d candidates", len(got))
	}
	expectCandidate(t, got[0], "", 2, 18, `[:fragment [:element [:tag "A"] [:attr "x"]] [:element [:tag "A"] [:attr "x"]]]`)
	expectCandidate(t, got[1], "A", 1, 8, `[:element [:tag "A"] [:attr "x"]]`)
	expectCandidate(t, got[2], "A", 1, 8, Pr(got[1].Norm))

	// child expressions follow dryer (map keeps its name, locals are :symbol);
	// JSX inside them is normalized and counted; key is dropped
	src := `function L({items}) { return <ul>{items.map((i) => <li key={i.id}><a {...i}>{i.label}</a></li>)}{ok && <hr/>}</ul>; }`
	got = jsxCandidatesIn(src, "l.jsx", "l.jsx")
	if len(got) != 4 {
		t.Fatalf("%d candidates", len(got))
	}
	li := `[:element [:tag "li"] [:element [:tag "a"] [:jsx_expression [:spread_element :symbol]] [:jsx_expression [:member_expression :symbol :symbol]]]]`
	// (the javascript grammar has no required_parameter around i)
	expectCandidate(t, got[0], "ul", 4, 59, `[:element [:tag "ul"] [:jsx_expression [:call_expression [:member_expression :symbol [:symbol "map"]] `+
		`[:arguments [:arrow_function [:formal_parameters :symbol] [:symbol "=>"] `+li+`]]]] `+
		`[:jsx_expression [:binary_expression :symbol [:symbol "&&"] [:element [:tag "hr"]]]]]`)
	expectCandidate(t, got[1], "li", 2, 21, li)
	expectCandidate(t, got[2], "a", 1, 16, `[:element [:tag "a"] [:jsx_expression [:spread_element :symbol]] [:jsx_expression [:member_expression :symbol :symbol]]]`)
	expectCandidate(t, got[3], "hr", 1, 5, `[:element [:tag "hr"]]`)

	// dryer's own normalization of the same function is unchanged by the hook
	cands, _, err := Candidates("typescript", src, "l.tsx")
	if err != nil || len(cands) != 1 {
		t.Fatalf("dryer candidates: %d %v", len(cands), err)
	}
	if dry := Pr(cands[0].Norm); strings.Contains(dry, ":element") || !strings.Contains(dry, `[:jsx_closing_element :symbol [:symbol ">"]]`) {
		t.Fatalf("dryer normalization changed: %s", dry)
	}
}

const jsxCardA = `export const A = ({u}) => (
  <section className="a">
    <header>
      <img src={u.src} />
      <h2>{u.name}</h2>
    </header>
    <p>{u.bio}</p>
  </section>
);
`

const jsxCardB = `export function B({t}) {
  return (
    <section className="b">
      <header>
        <img src={t.logo} />
        <h2>{t.title}</h2>
      </header>
      <p>{t.text}</p>
    </section>
  );
}
`

const jsxCardC = `export const C = ({t}) => (
  <section>
    <header>
      <img src={t.logo} />
      <h2>{t.title}</h2>
      <small />
    </header>
    <p>{t.text}</p>
  </section>
);
`

const jsxRows = `export const R = () => (
  <form>
    <label><input /><span>A</span></label>
    <label><input /><span>B</span></label>
  </form>
);
`

const jsxLists = `export const N = ({items, home}) => (
  <div>
    <ul>{items.map((i) => <li key={i.id}><a href={i.url}><b>{i.label}</b></a></li>)}</ul>
    <aside>{home && <li><a href="/"><b>{home.label}</b></a></li>}</aside>
  </div>
);
`

func jsxCorpus() []jsxCandidate {
	var all []jsxCandidate
	for _, f := range []struct{ name, src string }{
		{"a.tsx", jsxCardA}, {"b.jsx", jsxCardB}, {"c.tsx", jsxCardC}, {"r.tsx", jsxRows}, {"n.tsx", jsxLists},
	} {
		all = append(all, jsxCandidatesIn(f.src, f.name, f.name)...)
	}
	return all
}

func TestJSXPairsReportTheLargestEnclosingDuplicate(t *testing.T) {
	got := findJSXDuplicates(jsxCorpus(), JSXOptions{Threshold: 0.82, MinElements: 1, MinNodes: 1})
	expectPairs(t, got,
		// A and B: only the sections, not header/img/h2/p under them
		JSXDuplicate{1, jsxSide("a.tsx", 2, 8, "section", 5, 43), jsxSide("b.jsx", 3, 9, "section", 5, 43)},
		// C's extra <small /> keeps its section and header under 0.82: the
		// identical children are reported instead
		JSXDuplicate{1, jsxSide("a.tsx", 4, 4, "img", 1, 8), jsxSide("c.tsx", 4, 4, "img", 1, 8)},
		JSXDuplicate{1, jsxSide("a.tsx", 5, 5, "h2", 1, 11), jsxSide("c.tsx", 5, 5, "h2", 1, 11)},
		JSXDuplicate{1, jsxSide("a.tsx", 7, 7, "p", 1, 11), jsxSide("c.tsx", 8, 8, "p", 1, 11)},
		JSXDuplicate{1, jsxSide("b.jsx", 5, 5, "img", 1, 8), jsxSide("c.tsx", 4, 4, "img", 1, 8)},
		JSXDuplicate{1, jsxSide("b.jsx", 6, 6, "h2", 1, 11), jsxSide("c.tsx", 5, 5, "h2", 1, 11)},
		JSXDuplicate{1, jsxSide("b.jsx", 8, 8, "p", 1, 11), jsxSide("c.tsx", 8, 8, "p", 1, 11)},
		// the list item in a .map and the one behind &&: the li pair, not
		// (li, aside) or the a/b pairs inside
		JSXDuplicate{1, jsxSide("n.tsx", 3, 3, "li", 3, 24), jsxSide("n.tsx", 4, 4, "li", 3, 24)},
		// sibling rows: the labels, not (form, label) though form holds both
		JSXDuplicate{1, jsxSide("r.tsx", 3, 3, "label", 3, 15), jsxSide("r.tsx", 4, 4, "label", 3, 15)},
	)

	// the defaults (>= 4 elements, >= 40 nodes) keep the card pair only
	expectPairs(t, findJSXDuplicates(jsxCorpus(), DefaultJSXOptions()),
		JSXDuplicate{1, jsxSide("a.tsx", 2, 8, "section", 5, 43), jsxSide("b.jsx", 3, 9, "section", 5, 43)})
	expectPairs(t, findJSXDuplicates(jsxCorpus(), JSXOptions{Threshold: 0.82, MinElements: 6, MinNodes: 1}))
}

func TestJSXScoreIsJaccardOfSubtreeFingerprints(t *testing.T) {
	// {:element :tag "ul" "li" [:tag "ul"] [:tag "li"] [:element [:tag "li"]] root}:
	// 8 fingerprints each, 7 shared (the roots differ), 7/9; nodes 5 + 5 per li
	four := jsxCandidatesIn("const X = () => <ul><li/><li/><li/><li/></ul>;", "x.tsx", "x.tsx")
	five := jsxCandidatesIn("const Y = () => <ul><li/><li/><li/><li/><li/></ul>;", "y.tsx", "y.tsx")
	all := append(four, five...)
	o := JSXOptions{Threshold: 0.75, MinElements: 5, MinNodes: 1}
	expectPairs(t, findJSXDuplicates(all, o),
		JSXDuplicate{7.0 / 9, jsxSide("x.tsx", 1, 1, "ul", 5, 25), jsxSide("y.tsx", 1, 1, "ul", 6, 30)})
	o.Threshold = 0.78 // just above 7/9
	expectPairs(t, findJSXDuplicates(all, o))
}

func TestJSXFlagsAreHiddenAndImplyJSX(t *testing.T) {
	if strings.Contains(Help, "--jsx") {
		t.Fatal("--jsx is listed in --help")
	}
	if opts := ParseArgs(nil); opts.JSX.Enabled {
		t.Fatalf("jsx on by default: %+v", opts.JSX)
	}
	if opts := ParseArgs([]string{"--jsx"}); opts.JSX != DefaultJSXOptions() || opts.Help {
		t.Fatalf("--jsx: %+v", opts.JSX)
	}
	opts := ParseArgs([]string{"--jsx-min-nodes", "10", "src", "--jsx-threshold", "0.9", "--jsx-min-elements", "2"})
	if want := (JSXOptions{Enabled: true, Threshold: 0.9, MinElements: 2, MinNodes: 10}); opts.JSX != want || opts.Help {
		t.Fatalf("got %+v", opts.JSX)
	}
	if !reflect.DeepEqual(opts.Positionals, []string{"src"}) || opts.Threshold != 0.82 {
		t.Fatalf("dry options changed: %+v", opts)
	}
	for args, msg := range map[string]string{
		"--jsx-threshold x":     "--jsx-threshold requires a number",
		"--jsx-min-nodes -1":    "--jsx-min-nodes requires a value",
		"--jsx-min-elements no": "--jsx-min-elements requires a non-negative integer",
		"--jsx-min-elements":    "--jsx-min-elements requires a value",
	} {
		opts := ParseArgs(strings.Fields(args))
		if !opts.Help || opts.ExitCode != 1 || !strings.HasPrefix(opts.Message, msg+"\n\n") {
			t.Errorf("%s: %+v", args, opts)
		}
	}
}

func TestJSXSelectionAddsJSXFilesWithoutChangingDry(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	write := func(rel, text string) {
		_ = os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755)
		if err := os.WriteFile(filepath.Join(root, rel), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"src/a.tsx", "src/b.jsx", "src/c.js", "src/d.ts", "src/e.test.jsx", "tests/f.jsx"} {
		write(f, "export const x = 1;\n")
	}
	rel := func(opts Options) []string {
		files, err := SelectFiles(opts)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, f := range files {
			out = append(out, displayPath(f, root))
		}
		return out
	}
	if got := rel(Options{ProjectRoot: root}); !reflect.DeepEqual(got, []string{"src/a.tsx", "src/d.ts"}) {
		t.Fatalf("dry: %q", got)
	}
	jsx := Options{ProjectRoot: root, JSX: DefaultJSXOptions()}
	if got := rel(jsx); !reflect.DeepEqual(got, []string{"src/a.tsx", "src/b.jsx", "src/c.js", "src/d.ts"}) {
		t.Fatalf("--jsx: %q", got)
	}
	jsx.Tests = true
	if got := rel(jsx); !reflect.DeepEqual(got, []string{"src/a.tsx", "src/b.jsx", "src/c.js", "src/d.ts", "src/e.test.jsx", "tests/f.jsx"}) {
		t.Fatalf("--jsx --tests: %q", got)
	}
	jsx.Tests = false
	jsx.Positionals = []string{"src/b.jsx"}
	if got := rel(jsx); !reflect.DeepEqual(got, []string{"src/b.jsx"}) {
		t.Fatalf("named .jsx: %q", got)
	}
}

func TestDryJSXWritesItsOwnSectionAndMetrics(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	for name, src := range map[string]string{"src/A.tsx": jsxCardA, "src/B.jsx": jsxCardB} {
		_ = os.MkdirAll(filepath.Join(root, "src"), 0o755)
		if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dryEDN := filepath.Join(root, ".metrics", "dry.edn")
	jsxEDN := filepath.Join(root, ".metrics", "dry-jsx.edn")

	code, out, errs := runDry("--root", root)
	plain, _ := os.ReadFile(dryEDN)
	if _, err := os.Stat(jsxEDN); code != 0 || out != "No duplicate candidates found.\n" || err == nil {
		t.Fatalf("default: exit %d %q %q (dry-jsx.edn: %v)", code, out, errs, err)
	}

	code, out, errs = runDry("--root", root, "--jsx")
	wantOut := "No duplicate candidates found.\n\nDuplicate JSX\n=============\n\n" +
		"DUPLICATE score=1.00\n  src/A.tsx:2-8 <section> 5 elements\n  src/B.jsx:3-9 <section> 5 elements\n"
	if code != 0 || out != wantOut || errs != "Wrote "+dryEDN+"\nWrote "+jsxEDN+"\n" {
		t.Fatalf("--jsx: exit %d\n%s\n%q", code, out, errs)
	}
	withJSX, _ := os.ReadFile(dryEDN)
	if string(withJSX) != string(plain) {
		t.Fatalf("dry.edn changed under --jsx:\n%s", withJSX)
	}
	wantEDN := "{:jsx-candidates [\n {:score 1.0\n" +
		`  :left {:file "src/A.tsx", :start-line 2, :end-line 8, :tag "section", :elements 5, :nodes 43}` + "\n" +
		`  :right {:file "src/B.jsx", :start-line 3, :end-line 9, :tag "section", :elements 5, :nodes 43}}` + "\n]}\n"
	if got, _ := os.ReadFile(jsxEDN); string(got) != wantEDN {
		t.Fatalf("dry-jsx.edn:\n%s", got)
	}

	// --edn prints dry's map, then the JSX map
	if _, out, _ := runDry("--root", root, "--jsx", "--edn"); out != "{:candidates []}\n"+wantEDN {
		t.Fatalf("--edn:\n%s", out)
	}
	if _, out, _ := runDry("--root", root, "--jsx-min-elements", "6"); !strings.HasSuffix(out, "\nNo duplicate JSX candidates found.\n") {
		t.Fatalf("min elements 6:\n%s", out)
	}
}
