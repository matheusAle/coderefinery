package react

import (
	"reflect"
	"strings"
	"testing"
)

func analyze(t *testing.T, path, src string) File {
	t.Helper()
	f, err := newResolver("/project").analyzeSource(src, path, "/project")
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return f
}

func unitNamed(t *testing.T, f File, name string) Unit {
	t.Helper()
	for _, u := range f.Units {
		if u.Name == name {
			return u
		}
	}
	t.Fatalf("no unit %q in %+v", name, f.Units)
	return Unit{}
}

// entryCC maps entry name -> "kind CC" for compact, exact comparisons.
func entryCC(f File) map[string]string {
	out := map[string]string{}
	for _, e := range f.Entries {
		out[e.Name] = e.Kind + " " + itoa(e.Complexity)
	}
	return out
}

func unitNames(f File) []string {
	var out []string
	for _, u := range f.Units {
		out = append(out, u.Kind+" "+u.Name)
	}
	return out
}

func TestDetectsFunctionComponentsWrappersAndHooks(t *testing.T) {
	f := analyze(t, "src/ui.tsx", `
import React, { memo, forwardRef } from "react";
export function Plain() { return <div />; }
export const Arrow = () => <span />;
const Memo = memo(function Inner() { return <i />; });
const Ref = React.forwardRef<HTMLButtonElement, {}>((props, ref) => <button ref={ref} />);
const Both = React.memo(forwardRef((p, r) => (<b />)));
const lower = () => <div />;
function NotJSX() { return 1; }
function useThing() { return 1; }
const useOther = () => { };
function factory() { return function Hidden() { return <p />; }; }
class Klass extends React.Component { render() { return <div />; } }
export default function () { return <i />; }
`)
	want := []string{"component Plain", "component Arrow", "component Memo", "component Ref",
		"component Both", "hook useThing", "hook useOther"}
	if got := unitNames(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("units = %v, want %v", got, want)
	}
	// smoke: every unit has its body entry, in the file's namespace
	for _, e := range f.Entries {
		if e.Namespace != "ui" || e.Path != "src/ui.tsx" || e.Kind == "" {
			t.Errorf("entry %+v", e)
		}
	}
}

func TestHookCallbacksAndHandlersAreSeparateEntries(t *testing.T) {
	f := analyze(t, "Cart.tsx", `
export function Cart({ items, onPay }) {
  const [n, setN] = useState(() => (items ? 1 : 0));
  useEffect(() => { if (n > 1 && items) { setN(0); } }, [n]);
  useEffect(() => () => {}, []);
  const total = useMemo(() => items.reduce((a, b) => a + (b.price ?? 0), 0), [items]);
  const onSubmit = (e) => { if (e) onPay(total); };
  function onReset() { for (const i of items) {} }
  return (
    <form onSubmit={onSubmit}>
      <button onClick={() => (n ? setN(1) : setN(2))} />
      <button onClick={() => setN(3)} />
    </form>
  );
}
`)
	want := map[string]string{
		"Cart":             "component 1",
		"Cart#useState#1":  "callback 2",
		"Cart#useEffect#1": "callback 3",
		"Cart#useEffect#2": "callback 1",
		"Cart#useMemo#1":   "callback 2",
		"Cart.onSubmit":    "handler 2",
		"Cart.onReset":     "handler 2",
		"Cart.onClick":     "handler 2",
		"Cart.onClick#2":   "handler 1",
	}
	if got := entryCC(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("entries = %v\nwant      %v", got, want)
	}
	u := unitNamed(t, f, "Cart")
	if u.Render != 1 || u.Logic != 1 || u.Cost.InlineFunctions != 2 {
		t.Fatalf("decisions of callbacks leaked into Cart: %+v", u)
	}
	// smoke: entries are in source order with their own line spans
	if f.Entries[0].Name != "Cart" || f.Entries[1].StartLine != 3 || f.Entries[len(f.Entries)-1].Name != "Cart.onClick#2" {
		t.Fatalf("order: %+v", f.Entries)
	}
}

func TestRenderAndLogicComplexitySplit(t *testing.T) {
	f := analyze(t, "List.tsx", `
export function List({ items, user, loading }) {
  const label = user?.name ?? "anon";
  const visible = items.filter((i) => i.ok || i.pinned);
  if (loading) return <Spinner />;
  if (!user) {
    return null;
  }
  if (label) { console.log(label); }
  return (
    <ul>
      {visible.length > 0 && visible.map((i) => <li key={i.id}>{i.done ? "x" : "-"}</li>)}
    </ul>
  );
}
`)
	u := unitNamed(t, f, "List")
	// render: 2 early returns, &&, .map, ternary; logic: ?., ??, ||, if (no return)
	if u.Render != 6 || u.Logic != 5 {
		t.Fatalf("render/logic = %d/%d, want 6/5", u.Render, u.Logic)
	}
	// CC is crap's: every decision but .map (1 + 4 render + 4 logic)
	if got := entryCC(f)["List"]; got != "component 9" {
		t.Fatalf("List = %s", got)
	}
}

func TestNestedComponentsAreUnitsOfTheirOwn(t *testing.T) {
	f := analyze(t, "Outer.tsx", `
export function Outer({ rows }) {
  const Row = memo(({ r }) => <li onClick={() => r.go()}>{r.a ? 1 : 2}</li>);
  function Header() { return <h1 />; }
  const helper = (x) => (x ? <b /> : null);
  return <ul><Header />{rows.map((r) => <Row r={r} />)}</ul>;
}
`)
	if got := unitNames(f); !reflect.DeepEqual(got, []string{"component Outer", "component Outer.Row", "component Outer.Header"}) {
		t.Fatalf("units = %v", got)
	}
	want := map[string]string{
		"Outer": "component 1", "Outer.Row": "component 2", "Outer.Row.onClick": "handler 1",
		"Outer.Header": "component 1", "Outer.helper": "handler 2",
	}
	if got := entryCC(f); !reflect.DeepEqual(got, want) {
		t.Fatalf("entries = %v", got)
	}
	outer := unitNamed(t, f, "Outer")
	if outer.Cost != (Cost{Depth: 2, Elements: 3, Nested: 2}) {
		t.Fatalf("outer cost = %+v", outer.Cost)
	}
}

func TestPropsFromDestructuringAndInFileTypes(t *testing.T) {
	f := analyze(t, "Panel.tsx", `
interface Base { id: string; hidden?: boolean }
interface PanelProps extends Base {
  title: string;
  dark: boolean;
  mode?: true | false;
  onOpen: () => void;
  render(): void;
  "data-x"?: string;
}
type Alias = PropsWithChildren<{ a: number; b?: boolean }> & { c: string };
export function Panel({ title, dark = false, extra = true, onClose, "quoted": q, ...rest }: PanelProps) { return <div />; }
export const Aliased = ({ a }: Alias) => <div />;
export const Inline = ({ x, y }: { x: boolean; y?: () => void }) => <div />;
export const Omitted = ({ x }: Omit<PanelProps, "dark">) => <div />;
export const Typed: React.FC<PanelProps> = (props) => <div />;
`)
	cases := map[string]Props{
		// type: id hidden title dark mode onOpen render data-x; destructured extras: extra onClose quoted
		"Panel":   {Count: 11, Boolean: 4, Optional: 5, Callback: 3, Rest: true},
		"Aliased": {Count: 4, Boolean: 1, Optional: 2}, // a b c + PropsWithChildren's children?
		"Inline":  {Count: 2, Boolean: 1, Optional: 1, Callback: 1},
		"Omitted": {Count: 8, Boolean: 2, Optional: 3, Callback: 2}, // PanelProps' 8 members - dark + x
		"Typed":   {Count: 8, Boolean: 3, Optional: 3, Callback: 2},
	}
	for name, want := range cases {
		if got := unitNamed(t, f, name).Props; !reflect.DeepEqual(got, want) {
			t.Errorf("%s props = %+v, want %+v", name, got, want)
		}
	}
}

func TestPropsReadThroughAPropsParameter(t *testing.T) {
	f := analyze(t, "P.tsx", `
export function P(props) {
  const { a, b } = props;
  return <div title={props.c} {...props}>{a}{b}</div>;
}
`)
	if got := unitNamed(t, f, "P").Props; !reflect.DeepEqual(got, Props{Count: 3, Rest: true}) {
		t.Fatalf("props = %+v", got)
	}
}

func TestPassThroughPropsAreOnlyForwardedToChildComponents(t *testing.T) {
	f := analyze(t, "D.tsx", `
export function D({ user, theme, onLogout, value, size }) {
  const big = size > 2;
  return (
    <Layout user={user} theme={theme} onLogout={onLogout} size={size}>
      <input value={value} className={theme} />
      <ui.Menu onLogout={onLogout} />
    </Layout>
  );
}
export function L(props) {
  return <Side user={props.user} theme={props.theme}>{props.theme}</Side>;
}
`)
	// theme is also read on a DOM element, size in logic; value goes to a DOM element
	if got := unitNamed(t, f, "D").Props.PassThrough; !reflect.DeepEqual(got, []string{"onLogout", "user"}) {
		t.Fatalf("D pass-through = %v", got)
	}
	if got := unitNamed(t, f, "L").Props.PassThrough; !reflect.DeepEqual(got, []string{"user"}) {
		t.Fatalf("L pass-through = %v", got)
	}
}

func TestHookMetrics(t *testing.T) {
	f := analyze(t, "useForm.ts", `
export function useForm(source) {
  const [a, setA] = useState(0);
  const [b, setB] = React.useState(1);
  const [c] = useState();
  const ref = useRef();
  useEffect(() => setA(source.a), [source.a]);
  useEffect(() => { if (source.b) { setB(1); } else setA(2); }, [source.b, source]);
  useLayoutEffect(() => { setA(1); ref.current = 2; }, []);
  React.useEffect(() => { setA(1); return () => setB(0); });
  useMemo(() => a + b, [a, b, c, source, ref, 1, 2]);
  useCallback(() => notASetter(1), [a]);
}
`)
	want := Hooks{State: 3, Effects: 4, MaxDeps: 7, SetterEffects: 2}
	if got := unitNamed(t, f, "useForm").Hooks; got != want {
		t.Fatalf("hooks = %+v, want %+v", got, want)
	}
}

func TestRenderCostCounts(t *testing.T) {
	f := analyze(t, "C.tsx", `
export const C = ({ rows }) => (
  <>
    <div style={{ a: 1 }} data-list={[1, 2]} className={cx({ a: true })} onClick={() => go()}>
      <section>
        {rows.map((r) => (
          <p key={r} onMouseEnter={function () {}}><b><i /></b></p>
        ))}
      </section>
    </div>
  </>
);
`)
	want := Cost{Depth: 5, Elements: 5, InlineFunctions: 2, InlineObjects: 2}
	if got := unitNamed(t, f, "C").Cost; got != want {
		t.Fatalf("cost = %+v, want %+v", got, want)
	}
}

func f64(v float64) *float64 { return &v }

func TestCheckAppliesLimitsAndTheCrapGate(t *testing.T) {
	units := []Unit{
		{Name: "Big", Namespace: "n", Kind: KindComponent, Props: Props{Count: 8, Boolean: 3}, Cost: Cost{Depth: 7, Nested: 1}},
		{Name: "Ok", Namespace: "n", Kind: KindComponent, Props: Props{Count: 7, Boolean: 2}, Cost: Cost{Depth: 6}},
	}
	entries := []Entry{
		{Name: "Big", Namespace: "n", Complexity: 3, Crap: f64(10)},
		{Name: "Ok", Namespace: "n", Complexity: 3, Crap: f64(9.99)},
		{Name: "Raw", Namespace: "n", Complexity: 10},
		{Name: "Raw2", Namespace: "n", Complexity: 9},
	}
	l := DefaultLimits()
	l.Max["jsx-depth"] = -1
	var got []string
	for _, v := range Check(units, entries, l) {
		got = append(got, v.Name+" "+violationText(v))
	}
	want := []string{
		"Big crap 10.0 >= 10.0",
		"Raw complexity 10 >= 10.0 (no coverage)",
		"Big props 8 > 7",
		"Big boolean-props 3 > 2",
		"Big nested-components 1 > 0",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("violations =\n%v\nwant\n%v", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	sorted := SortUnits([]Unit{units[1], units[0]}, Check(units, entries, l))
	if sorted[0].Name != "Big" {
		t.Fatalf("worst unit first: %v", sorted)
	}
}

func TestParseArgs(t *testing.T) {
	o := ParseArgs([]string{"--no-coverage", "--max-props", "9", "--max-elements", "40", "--max-nested", "off", "--crap", "8", "src"})
	if o.Help || !o.NoCoverage || o.Limits.Crap != 8 || o.Limits.Max["props"] != 9 ||
		o.Limits.Max["elements"] != 40 || o.Limits.Max["nested"] != -1 || !reflect.DeepEqual(o.Positionals, []string{"src"}) {
		t.Fatalf("opts = %+v", o)
	}
	for args, msg := range map[string]string{
		"--bogus":                               "Unknown option: --bogus",
		"--max-props":                           "--max-props requires a value",
		"--max-props x":                         "--max-props requires a non-negative integer or off",
		"--crap x":                              "--crap requires a number",
		"--no-coverage --coverage-command":      "--coverage-command requires a value",
		"--no-coverage --use-existing-coverage": "--no-coverage cannot be combined with --coverage-command or --use-existing-coverage",
	} {
		o := ParseArgs(strings.Fields(args))
		if !o.Help || o.ExitCode != 1 || !strings.HasPrefix(o.Message, msg+"\n\nUsage: coderefinery react") {
			t.Errorf("%s: %+v", args, o.Message[:min(80, len(o.Message))])
		}
	}
	if h := ParseArgs([]string{"-h"}); !h.Help || h.ExitCode != 0 || !strings.Contains(h.Message, " pass-through 3,") {
		t.Fatalf("help = %q", h.Message)
	}
}

func TestRenderEDNAndReport(t *testing.T) {
	entries := []Entry{{Name: "A#useEffect#1", Namespace: "src.A", Kind: KindCallback, Unit: "A", Path: "src/A.tsx",
		StartLine: 3, EndLine: 4, Complexity: 2, Coverage: f64(50), Crap: f64(2.5)}}
	units := []Unit{
		{Name: "A", Namespace: "src.A", Kind: KindComponent, Path: "src/A.tsx", Line: 1, Render: 2, Logic: 1,
			Props: Props{Count: 2, Callback: 1, PassThrough: []string{"x"}}, Hooks: Hooks{Effects: 1, MaxDeps: 1},
			Cost: Cost{Depth: 1, Elements: 1}},
		{Name: "useB", Namespace: "src.A", Kind: KindHook, Path: "src/A.tsx", Line: 9, Render: 1, Logic: 1},
	}
	violations := []Violation{{"A", "src.A", "src/A.tsx", 1, "pass-through", 4, 3}}
	want := `{:entries [
  {:name "A#useEffect#1", :namespace "src.A", :kind :callback, :unit "A", :path "src/A.tsx", :line 3, :complexity 2, :coverage 50.0, :crap 2.5}
]
 :components [
  {:name "A", :namespace "src.A", :kind :component, :path "src/A.tsx", :line 1, :render 2, :logic 1,
   :props {:count 2, :boolean 0, :optional 0, :callback 1, :rest false, :pass-through ["x"], :open false},
   :hooks {:state 0, :effects 1, :max-deps 1, :setter-effects 0},
   :render-cost {:jsx-depth 1, :elements 1, :inline-functions 0, :inline-objects 0, :nested-components 0}}
  {:name "useB", :namespace "src.A", :kind :hook, :path "src/A.tsx", :line 9, :render 1, :logic 1,
   :props nil,
   :hooks {:state 0, :effects 0, :max-deps 0, :setter-effects 0},
   :render-cost {:jsx-depth 0, :elements 0, :inline-functions 0, :inline-objects 0, :nested-components 0}}
]
 :violations [
  {:name "A", :namespace "src.A", :path "src/A.tsx", :line 1, :metric :pass-through, :value 4.0, :limit 3.0}
]}
`
	if got := RenderEDN(entries, units, violations); got != want {
		t.Fatalf("edn:\n%s\nwant:\n%s", got, want)
	}
	if got := RenderEDN(nil, nil, nil); got != "{:entries []\n :components []\n :violations []}\n" {
		t.Fatalf("empty edn = %q", got)
	}
	report := FormatReport(entries, units, violations)
	for _, line := range []string{
		"A#useEffect#1                  callback  src.A                                   3    2   50.0%      2.5",
		"useB                           src.A                                    1     1     -    -   -  -    -    -     0   0    0     0     0     0    0     0    0",
		"A                              src.A                               pass-through 4 > 3",
	} {
		if !strings.Contains(report, "\n"+line+"\n") {
			t.Errorf("report lacks %q:\n%s", line, report)
		}
	}
}
