package branch

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/matheusAle/coderefinery/internal/coverage"
)

func armAt(label string, l, c, el, ec int) Arm {
	return Arm{Label: label, StartLine: l, StartColumn: c, EndLine: el, EndColumn: ec}
}

// pyIf is `if total <= 0:` on line 16 with its body on 17.
var pyIf = Decision{Path: "p.py", Line: 16, Column: 5, EndLine: 17, Kind: "if_statement", Header: "if total <= 0",
	Arms: []Arm{armAt("true", 17, 9, 17, 17), {Label: "false"}}, ReachLine: 16, ReachColumn: 5}

func texts(fs []Finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Path + ":" + strconv.Itoa(f.Line) + " " + f.Decision + " · " + f.Text + " [" + f.Arm + "]"
	}
	return out
}

func TestArcsPickTheArmHoldingTheTarget(t *testing.T) {
	f := &coverage.BranchFile{Granularity: coverage.GranBranch, Arms: []coverage.ArmHit{
		{Kind: coverage.KindArc, Line: 16, FromLine: 16, ToLine: 17, Hits: 0, Reached: 1},
		{Kind: coverage.KindArc, Line: 16, FromLine: 16, ToLine: -12, Hits: 0, Reached: 1},
		{Kind: coverage.KindArc, Line: 16, FromLine: 16, ToLine: 18, Hits: 1, Reached: 1},
	}}
	got := Map("p.py", []Decision{pyIf}, f, "")
	want := []Finding{
		{Path: "p.py", Line: 16, Column: 5, Decision: "if total <= 0", Arm: "true", Granularity: "branch",
			Text: "true arm never taken", SpanStart: 17, SpanEnd: 17},
		{Path: "p.py", Line: 16, Column: 5, Decision: "if total <= 0", Arm: "false", Granularity: "branch",
			Text: "false arm never taken"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestUnreachedDecisionsAreNotReported(t *testing.T) {
	f := &coverage.BranchFile{Granularity: coverage.GranBranch, Arms: []coverage.ArmHit{
		{Kind: coverage.KindArc, Line: 16, FromLine: 16, ToLine: 17, Reached: 0},
	}}
	if got := Map("p.py", []Decision{pyIf}, f, ""); len(got) != 0 {
		t.Fatalf("an unexecuted decision was reported: %+v", got)
	}
}

func TestIstanbulIndexNamesTheArmAndKindBreaksColumnTies(t *testing.T) {
	ternary := Decision{Path: "a.ts", Line: 3, Column: 9, Kind: "ternary_expression", Header: "a && b ? 1 : 2",
		Arms: []Arm{armAt("true", 3, 18, 3, 19), armAt("false", 3, 22, 3, 23)}, ReachLine: 3, ReachColumn: 9}
	and := Decision{Path: "a.ts", Line: 3, Column: 9, Kind: "binary_expression", Header: "a && b",
		Arms: []Arm{{Label: "operand 1", Text: "a", StartLine: 3, StartColumn: 9, EndLine: 3, EndColumn: 10},
			{Label: "operand 2", Text: "b", StartLine: 3, StartColumn: 14, EndLine: 3, EndColumn: 15}},
		ReachLine: 3, ReachColumn: 9}
	f := &coverage.BranchFile{Granularity: coverage.GranBranch, Arms: []coverage.ArmHit{
		{Kind: "cond-expr", Line: 3, Column: 9, Index: 1, Of: 2, Hits: 0, Reached: 4},
		{Kind: "binary-expr", Line: 3, Column: 9, Index: 1, Of: 2, Hits: 0, Reached: 4},
		{Kind: "default-arg", Line: 3, Column: 9, Of: 1, Reached: 4},
	}}
	got := texts(Map("a.ts", []Decision{ternary, and}, f, ""))
	want := []string{"a.ts:3 a && b ? 1 : 2 · false arm never taken [false]",
		"a.ts:3 a && b · operand 2 (b) never evaluated [operand 2]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
}

// goSrc mirrors fixtures/go calc.go lines 21-23 (`if count == 0 {`).
const goSrc = "\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n\tif count == 0 {\n\t\treturn 0, nil\n\t}\n"

func TestGoBlocksBelongToTheInnermostArmOfAnExecutedDecision(t *testing.T) {
	d := Decision{Path: "calc.go", Line: 21, Column: 2, Kind: "if_statement", Header: "if count == 0",
		Arms: []Arm{armAt("true", 21, 16, 23, 3), {Label: "false"}}, ReachLine: 21, ReachColumn: 2}
	f := &coverage.BranchFile{Granularity: coverage.GranBlock, Arms: []coverage.ArmHit{
		{Kind: coverage.KindBlock, Line: 14, Column: 2, EndLine: 21, EndColumn: 16, Hits: 1},
		{Kind: coverage.KindBlock, Line: 21, Column: 16, EndLine: 23, EndColumn: 3, Hits: 0},
		{Kind: coverage.KindBlock, Line: 24, Column: 2, EndLine: 24, EndColumn: 43, Hits: 0},
	}}
	got := Map("calc.go", []Decision{d}, f, goSrc)
	want := []Finding{{Path: "calc.go", Line: 21, Column: 2, Decision: "if count == 0", Arm: "true", Granularity: "block",
		Text: "block 22-23 never executed", SpanStart: 22, SpanEnd: 23}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
	f.Arms[0].Hits = 0
	if got := Map("calc.go", []Decision{d}, f, goSrc); len(got) != 0 {
		t.Fatalf("block of an unexecuted decision reported: %+v", got)
	}
}

func TestNestedZeroRegionsReportOnlyTheOutermost(t *testing.T) {
	d := Decision{Path: "lib.rs", Line: 4, Column: 5, Kind: "if_expression", Header: "if x",
		Arms: []Arm{armAt("true", 4, 10, 8, 6), {Label: "false"}}, ReachLine: 4, ReachColumn: 5}
	f := &coverage.BranchFile{Granularity: coverage.GranRegion, Arms: []coverage.ArmHit{
		{Kind: coverage.KindRegion, Line: 3, Column: 1, EndLine: 9, EndColumn: 2, Hits: 1},
		{Kind: coverage.KindRegion, Line: 4, Column: 10, EndLine: 8, EndColumn: 6, Hits: 0},
		{Kind: coverage.KindRegion, Line: 5, Column: 9, EndLine: 5, EndColumn: 20, Hits: 0},
	}}
	got := texts(Map("lib.rs", []Decision{d}, f, ""))
	if want := []string{"lib.rs:4 if x · region 4:10-8:6 never executed [true]"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestLineCountsGoToTheLinesDecisionCasesLast(t *testing.T) {
	cs := Decision{Path: "B.java", Line: 42, Column: 17, Kind: "switch_label", Header: "default",
		Arms: []Arm{armAt("default", 42, 24, 42, 40)}, ReachLine: 39, ReachColumn: 22}
	tern := Decision{Path: "B.java", Line: 42, Column: 28, Kind: "ternary_expression", Header: "cell > 10 ? 3 : 2",
		ReachLine: 42, ReachColumn: 28}
	f := &coverage.BranchFile{Granularity: coverage.GranBranchCount, Arms: []coverage.ArmHit{
		{Kind: coverage.KindLineCount, Line: 39, Of: 3, Hits: 2, Reached: 6},
		{Kind: coverage.KindLineCount, Line: 42, Of: 2, Hits: 1, Reached: 4},
		{Kind: coverage.KindLineCount, Line: 50, Of: 2, Hits: 0, Reached: 0},
	}}
	got := texts(Map("B.java", []Decision{cs, tern}, f, ""))
	want := []string{"B.java:42 default · 1 of 3 branches missed []", "B.java:42 cell > 10 ? 3 : 2 · 1 of 2 branches missed []"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestSectionAndEDN(t *testing.T) {
	GeneratedAt = func() string { return "2026-11-03T12:00:00-03:00" }
	fs := []Finding{{Path: "billing/pricing.py", Line: 16, Decision: "if total <= 0", Arm: "true", Granularity: "branch",
		Text: "true arm never taken"}}
	got := Section(map[string]string{"python": "branch", "go": "block"}, map[string]string{"rust": coverage.MissingTarpaulin}, fs)
	want := "\nUntested branches (go block, python branch)\n" +
		"billing/pricing.py:16 if total <= 0 · true arm never taken\n" +
		"No branch data: rust (tarpaulin reports lines only)\n"
	if got != want {
		t.Fatalf("section %q", got)
	}
	if got := Section(map[string]string{"go": "block"}, nil, nil); got != "\nUntested branches (go block)\nNone in executed code.\n" {
		t.Fatalf("empty section %q", got)
	}
	edn := RenderEDN(fs)
	wantEDN := "{:version 1 :generated-at \"2026-11-03T12:00:00-03:00\" :branches [\n" +
		"  {:file \"billing/pricing.py\" :line 16 :decision \"if total <= 0\" :arm \"true\" :granularity \"branch\" :text \"true arm never taken\"}\n]}\n"
	if edn != wantEDN {
		t.Fatalf("edn %q", edn)
	}
	if got := RenderEDN(nil); !strings.HasSuffix(got, ":branches []}\n") {
		t.Fatalf("empty edn %q", got)
	}
}

func TestArmTexts(t *testing.T) {
	for label, want := range map[string]string{
		"true": "true arm never taken", "body": "body never entered", "exit": "exit never taken",
		"case 2": "case 2 never matched", "default": "default never matched", "handler": "handler never entered",
		"no match": "case always matched", "error": "error arm never taken", "operand 3": "operand 3 never evaluated",
	} {
		if got := ArmText(Arm{Label: label}); got != want {
			t.Errorf("%s: %q", label, got)
		}
	}
}
