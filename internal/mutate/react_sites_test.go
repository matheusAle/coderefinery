package mutate

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/matheusAle/coderefinery/internal/lang"
)

// reactSite is the part of a RawSite each operator decides.
type reactSite struct {
	Line, Start, End        int
	Original, Mutant, Label string
}

func reactOnly(t *testing.T, source, path string) []reactSite {
	t.Helper()
	var out []reactSite
	for _, s := range discoverRaw(source, "typescript", path, true) {
		if s.Category == "react" {
			out = append(out, reactSite{s.Line, s.Start, s.End, s.Original, s.Mutant, s.Label})
		}
	}
	return out
}

// checkSpans is the smoke check: every site's bytes are its original, and
// the mutated file still parses (a mutant must not be a syntax error).
func checkSpans(t *testing.T, source, path string, sites []reactSite) {
	t.Helper()
	for _, s := range sites {
		if got := source[s.Start:s.End]; got != s.Original {
			t.Errorf("%s: bytes %d..%d are %q, not %q", s.Label, s.Start, s.End, got, s.Original)
		}
		mutated := applySite([]byte(source), s.Start, s.End, s.Mutant)
		if _, err := lang.Parse(grammarFor("typescript", path), mutated); err != nil {
			t.Errorf("%s: mutant does not parse: %v\n%s", s.Label, err, mutated)
		}
	}
}

const panelTSX = `export function Panel({ open, items, onPick }: Props) {
  useEffect(() => sync(open), [open, items.length]);
  const total = React.useMemo(() => sum(items), [items]);
  return (
    <section key="s" className="p" style={{ gap: 1 }} {...rest} hidden aria-label="panel">
      {open && (<Body total={total} />)}
      {open || <Empty />}
      {items.length ? <List items={items} onPick={onPick} /> : <>none</>}
      {open ? "yes" : "no"}
      <button onClick={() => {}} onBlur={done}>Go</button>
    </section>
  );
}
`

func TestReactOperatorsFindExactSites(t *testing.T) {
	got := reactOnly(t, panelTSX, "src/Panel.tsx")
	want := []reactSite{
		{2, 87, 93, "open, ", "", "drop dep open from useEffect"},
		{2, 91, 105, ", items.length", "", "drop dep items.length from useEffect"},
		{3, 158, 163, "items", "", "drop dep items from useMemo"},
		{5, 242, 248, "hidden", "", "drop prop hidden from <section>"},
		{5, 249, 267, `aria-label="panel"`, "", "drop prop aria-label from <section>"},
		{6, 276, 308, "open && (<Body total={total} />)", "false", "drop render <Body/>"},
		{6, 276, 308, "open && (<Body total={total} />)", "(<Body total={total} />)", "always render <Body/>"},
		{6, 291, 304, "total={total}", "", "drop prop total from <Body/>"},
		{8, 358, 408, "<List items={items} onPick={onPick} /> : <>none</>",
			"<>none</> : <List items={items} onPick={onPick} />", "swap ternary <List/> : <>"},
		{8, 364, 377, "items={items}", "", "drop prop items from <List/>"},
		{8, 386, 392, "onPick", "() => {}", "noop onPick"},
		{10, 479, 483, "done", "() => {}", "noop onBlur"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sites:\n%#v\nwant:\n%#v", got, want)
	}
	checkSpans(t, panelTSX, "src/Panel.tsx", got)
}

func TestReactOperatorsSkipWhatTheyLeaveAlone(t *testing.T) {
	// key / className / style / spreads are not dropped; `||` is not a
	// conditional render; a ternary with no JSX branch is not swapped; a
	// handler that already is `() => {}` has no no-op mutant; a string
	// handler value is not an expression.
	got := reactOnly(t, `function A({ open }) {
  return <div key="k" className="c" style={{ a: 1 }} {...open} onClick={() => {}} onKeyDown="x">
    {open || <B />}{open ? 1 : 2}{open ?? <C />}
  </div>;
}
`, "a.jsx")
	if len(got) != 0 {
		t.Fatalf("want no React sites, got %#v", got)
	}
}

func TestReactDependencyRemovalKeepsTheArrayWellFormed(t *testing.T) {
	src := `function useX(a, b, c) {
  useCallback(() => a, [a]);
  useLayoutEffect(() => {}, [a, b, c]);
  useState(() => [a, b]);
  useEffect(() => {});
  other(() => {}, [a]);
}
`
	got := reactOnly(t, src, "src/useX.ts")
	want := []reactSite{
		{2, 49, 50, "a", "", "drop dep a from useCallback"},
		{3, 83, 86, "a, ", "", "drop dep a from useLayoutEffect"},
		{3, 86, 89, "b, ", "", "drop dep b from useLayoutEffect"},
		{3, 87, 90, ", c", "", "drop dep c from useLayoutEffect"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sites:\n%#v\nwant:\n%#v", got, want)
	}
	checkSpans(t, src, "src/useX.ts", got)
	mutated := make([]string, len(got))
	for i, s := range got {
		mutated[i] = strings.Split(string(applySite([]byte(src), s.Start, s.End, s.Mutant)), "\n")[got[i].Line-1]
	}
	if !reflect.DeepEqual(mutated, []string{
		"  useCallback(() => a, []);",
		"  useLayoutEffect(() => {}, [b, c]);",
		"  useLayoutEffect(() => {}, [a, c]);",
		"  useLayoutEffect(() => {}, [a, b]);",
	}) {
		t.Fatalf("mutated lines %q", mutated)
	}
}

func TestReactLabelsShortenLongBranchesAndDeps(t *testing.T) {
	src := `function A({ v }) {
  useMemo(() => v, [v.aVeryLongPropertyName.anotherVeryLongPropertyName]);
  return v ? <B /> : renderFallback(v,
    "with a long argument");
}
`
	var labels []string
	for _, s := range reactOnly(t, src, "a.tsx") {
		labels = append(labels, s.Label)
	}
	if !reflect.DeepEqual(labels, []string{
		"drop dep v.aVeryLongPropertyName.anotherVeryLong… from useMemo",
		`swap ternary <B/> : renderFallback(v, "with a long argument…`,
	}) {
		t.Fatalf("labels %q", labels)
	}
}

func TestReactSitesBelongToCrapperFormsAndKeepTheIDShape(t *testing.T) {
	root := tempRoot(t)
	src := "export function Cart({ ok, onGo }) {\n  return <div>{ok && <p>ok</p>}<b onClick={onGo} /></div>;\n}\n"
	plain := sitesOf(t, root, "src/Cart.tsx", src)
	path := writeFile(t, root+"/src/Cart.tsx", src)
	u := newFileUnit(src, path, root, "src/Cart.tsx")
	u.react = true
	withReact := u.sites()

	var descriptions, ids []string
	for _, s := range withReact {
		descriptions = append(descriptions, s.Description())
		if s.Category == "react" {
			ids = append(ids, s.ID)
		}
		if s.FormID != "defn/Cart" || s.Namespace != "Cart" {
			t.Errorf("%s owned by %s %s", s.Description(), s.Namespace, s.FormID)
		}
	}
	if !reflect.DeepEqual(descriptions, []string{"drop render <p>", "always render <p>", "&& -> ||", "noop onClick"}) {
		t.Fatalf("descriptions %q", descriptions)
	}
	if !reflect.DeepEqual(ids, []string{
		`["src/Cart.tsx","Cart","defn/Cart",52,67,"ok && <p>ok</p>","false"]`,
		`["src/Cart.tsx","Cart","defn/Cart",52,67,"ok && <p>ok</p>","<p>ok</p>"]`,
		`["src/Cart.tsx","Cart","defn/Cart",80,84,"onGo","() => {}"]`,
	}) {
		t.Fatalf("ids %q", ids)
	}
	// Without --react the sites are the upstream ones, nothing more.
	if len(plain) != 1 || plain[0].Description() != "&& -> ||" || plain[0].Label != "" {
		t.Fatalf("default sites %+v", plain)
	}
}

func TestReactOperatorsIgnoreOtherLanguages(t *testing.T) {
	if got := discoverRaw("def f(a):\n    return a and b\n", "python", "a.py", true); len(got) != 1 || got[0].Category != "logical" {
		t.Fatalf("python sites %+v", got)
	}
}

func TestReactFlagIsParsedAndHidden(t *testing.T) {
	if o := ParseArgs([]string{"--react", "--scan", "src"}); !o.React || !o.Scan || o.Help || len(o.Positionals) != 1 {
		t.Fatalf("parsed %+v", o)
	}
	if ParseArgs([]string{"src"}).React || strings.Contains(Help, "--react") {
		t.Fatal("--react is opt-in and not in the upstream --help")
	}
}

const cartTSX = "export function Cart({ ok, onGo }) {\n  return <div>{ok && <p>ok</p>}<b onClick={onGo} /></div>;\n}\n"

func TestReactScanAddsLabelledSitesAndTheDefaultScanIsUnchanged(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "src", "Cart.tsx"), cartTSX)
	plain := cli("--scan", "--no-coverage", "--root", root)
	if want := "Scan: 1 mutation sites in src/Cart.tsx\n  src/Cart.tsx:2 && -> ||  [defn/Cart]\n"; plain.code != 0 || plain.stdout != want {
		t.Fatalf("default scan %+v", plain)
	}
	r := cli("--scan", "--no-coverage", "--react", "--root", root)
	want := "Scan: 4 mutation sites in src/Cart.tsx\n" +
		"  src/Cart.tsx:2 drop render <p>  [defn/Cart]\n" +
		"  src/Cart.tsx:2 always render <p>  [defn/Cart]\n" +
		"  src/Cart.tsx:2 && -> ||  [defn/Cart]\n" +
		"  src/Cart.tsx:2 noop onClick  [defn/Cart]\n"
	if r.code != 0 || r.stdout != want || r.stderr != "" {
		t.Fatalf("react scan %+v", r)
	}
}

// cartTest stands in for a test that asserts the <p> renders when ok and
// never clicks the button: it fails when `{ok && <p>ok</p>}` changes.
func cartTest(cwd string, _ time.Duration) int {
	text, err := os.ReadFile(filepath.Join(cwd, "src", "Cart.tsx"))
	if err != nil || !strings.Contains(string(text), "{ok && <p>ok</p>}") {
		return 1
	}
	return 0
}

func outcomesOfRoot(root string) map[string]bool {
	out := map[string]bool{}
	for _, o := range SnapshotOutcomes(root) {
		out[o.ID] = o.Killed
	}
	return out
}

func TestReactMutantsShareTheSnapshotAndRerunDifferentially(t *testing.T) {
	root := tempRoot(t)
	path := writeFile(t, filepath.Join(root, "src", "Cart.tsx"), cartTSX)
	runner := &fakeRunner{decide: cartTest}
	o := baseOptions(runner)
	o.react = true
	first := mutateOK(t, path, root, o)
	if len(first.Forms) != 1 || first.Forms[0].ID != "defn/Cart" || first.Forms[0].Killed != 3 || first.Forms[0].Survived != 1 || first.Forms[0].Sites != 4 {
		t.Fatalf("forms %+v", first.Forms)
	}
	if runner.calls != 5 || readFile(t, path) != cartTSX {
		t.Fatalf("baseline + 4 mutants, source restored: %d calls", runner.calls)
	}
	log := formatSiteLog(first.Sites, first.Statuses, nil)
	if log != "KILLED    src/Cart.tsx:2 drop render <p>\nKILLED    src/Cart.tsx:2 always render <p>\n"+
		"KILLED    src/Cart.tsx:2 && -> ||\nSURVIVED  src/Cart.tsx:2 noop onClick\n" {
		t.Fatalf("site log:\n%s", log)
	}
	noop := `["src/Cart.tsx","Cart","defn/Cart",80,84,"onGo","() => {}"]`
	if got := outcomesOfRoot(root); len(got) != 4 || got[noop] || !got[`["src/Cart.tsx","Cart","defn/Cart",52,67,"ok && <p>ok</p>","false"]`] {
		t.Fatalf("snapshot outcomes %v", got)
	}

	// Unchanged form: only the survivor runs again (the baseline is cached
	// in o.baselines).
	o.mutateAll = false
	second := mutateOK(t, path, root, o)
	if runner.calls != 6 || second.Forms[0].Killed != 3 || second.Forms[0].Survived != 1 {
		t.Fatalf("differential rerun: %d calls, %+v", runner.calls, second.Forms[0])
	}

	// Without --react the same form id carries the upstream kill and
	// nothing runs; the React outcomes leave the snapshot.
	plain := baseOptions(runner)
	plain.mutateAll = false
	third := mutateOK(t, path, root, plain)
	if runner.calls != 6 || third.Forms[0].ID != "defn/Cart" || third.Forms[0].Killed != 1 || third.Forms[0].Sites != 1 {
		t.Fatalf("plain run: %d calls, %+v", runner.calls, third.Forms[0])
	}
	if got := outcomesOfRoot(root); len(got) != 1 || !got[`["src/Cart.tsx","Cart","defn/Cart",55,57,"&&","||"]`] {
		t.Fatalf("plain snapshot outcomes %v", got)
	}
}
