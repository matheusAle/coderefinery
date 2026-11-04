package crap

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// summary renders a decision as "line:col kind «header» reach=l:c [label@l:c-l:c ...]".
func summary(d Decision) string {
	var arms []string
	for _, a := range d.Arms {
		s := a.Label
		if a.StartLine > 0 {
			s += fmt.Sprintf("@%d:%d-%d:%d", a.StartLine, a.StartColumn, a.EndLine, a.EndColumn)
		}
		if a.Text != "" {
			s += "(" + a.Text + ")"
		}
		arms = append(arms, s)
	}
	return fmt.Sprintf("%d:%d %s «%s» reach=%d:%d [%s]", d.Line, d.Column, d.Kind, d.Header, d.ReachLine, d.ReachColumn,
		strings.Join(arms, " "))
}

func decisionsOf(t *testing.T, path, rel string) []string {
	t.Helper()
	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	root, _ := filepath.Abs(filepath.Dir(path))
	decs, err := Decisions(string(text), rel, root)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(decs))
	for i, d := range decs {
		out[i] = summary(d)
	}
	return out
}

func TestPythonDecisionsMatchComplexity(t *testing.T) {
	got := decisionsOf(t, "testdata/branches_python/billing/pricing.py", "billing/pricing.py")
	want := []string{
		`5:5 if_statement «if tier == "gold"» reach=5:5 [true@6:9-6:19 false@7:5-8:19]`,
		`7:5 elif_clause «elif tier == "silver"» reach=7:5 [true@8:9-8:19 false]`,
		`14:5 for_statement «for unit, qty in items» reach=14:5 [body@15:9-15:28 exit]`,
		`16:5 if_statement «if total <= 0» reach=16:5 [true@17:9-17:17 false]`,
		`21:5 for_statement «for item in items» reach=21:5 [body@22:9-22:20 exit]`,
		`28:9 case_clause «case "US"» reach=28:9 [case 1@29:13-29:21 no match]`,
		`30:9 case_clause «case "CA" | "MX"» reach=30:9 [case 2@31:13-31:21 no match]`,
		`32:9 case_clause «case _» reach=32:9 [case 3@33:13-33:22 no match]`,
		`37:5 if_statement «if flag» reach=37:5 [true@38:9-38:17 false]`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n%s", strings.Join(got, "\n"))
	}
	// one decision per complexity point: CC - 1 per function
	text, _ := os.ReadFile("testdata/branches_python/billing/pricing.py")
	fns, _ := PythonFunctions(string(text), "billing/pricing.py", "")
	points := 0
	for _, fn := range fns {
		points += fn.Complexity - 1
	}
	if points != len(want) {
		t.Fatalf("complexity counts %d decisions, Decisions found %d", points, len(want))
	}
}

func TestGoDecisionsCaseArmsStartAfterTheColon(t *testing.T) {
	got := decisionsOf(t, "testdata/branches_go/calc.go", "calc.go")
	want := []string{
		`6:2 if_statement «if v < lo» reach=6:2 [true@6:12-8:3 false]`,
		`9:2 if_statement «if v > hi» reach=9:2 [true@9:12-11:3 false]`,
		`18:2 expression_case «case n < 0» reach=17:2 [case 1@18:13-19:1073741824]`,
		`20:2 expression_case «case n == 0» reach=17:2 [case 2@20:14-21:1073741824]`,
		`22:2 default_case «default» reach=17:2 [default@22:10-23:1073741824]`,
		`30:2 for_statement «for _, v := range values» reach=30:2 [body@30:27-32:3 exit]`,
		`37:2 if_statement «if n > 0» reach=37:2 [true@37:11-39:3 false]`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n%s", strings.Join(got, "\n"))
	}
}

func writeTemp(t *testing.T, name, text string) string {
	t.Helper()
	dir := scratchDir(t)
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o666); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTypeScriptDecisionsFlattenLogicalChains(t *testing.T) {
	p := writeTemp(t, "src/a.ts", "export function f(a: number, b: boolean, c: boolean) {\n"+
		"  if (a > 0 && (b || c)) {\n    return 1;\n  } else {\n    return 2;\n  }\n}\n"+
		"export function g(x: string) {\n  switch (x) {\n    case \"a\":\n      return x ? 1 : 2;\n    default:\n      return 0;\n  }\n}\n")
	got := decisionsOf(t, p, "src/a.ts")
	want := []string{
		`2:3 if_statement «if (a > 0 && (b || c))» reach=2:3 [true@2:26-4:4 false@4:5-6:4]`,
		`2:7 binary_expression «a > 0 && (b || c)» reach=2:7 [operand 1@2:7-2:12(a > 0) operand 2@2:17-2:18(b) operand 3@2:22-2:23(c)]`,
		`2:17 binary_expression «b || c» reach=2:17 [operand 1@2:17-2:18(b) operand 2@2:22-2:23(c)]`,
		`10:5 switch_case «case "a"» reach=9:3 [case 1@10:14-11:24]`,
		`11:14 ternary_expression «x ? 1 : 2» reach=11:14 [true@11:18-11:19 false@11:22-11:23]`,
		`12:5 switch_default «default» reach=9:3 [default@12:13-13:16]`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got\n%s", strings.Join(got, "\n"))
	}
}

func TestJavaRustClojureDecisions(t *testing.T) {
	java := decisionsOf(t, "testdata/e2e_java/src/main/java/demo/Board.java", "src/main/java/demo/Board.java")
	wantJava := []string{
		`11:13 if_statement «if (cells.isEmpty())» reach=11:13 [true@11:34-13:14 false]`,
		`26:9 if_statement «if (x > 0 && x < cells.size())» reach=26:9 [true@26:40-28:10 false]`,
		`26:13 binary_expression «x > 0 && x < cells.size()» reach=26:13 [operand 1@26:13-26:18(x > 0) operand 2@26:22-26:38(x < cells.size())]`,
		`33:16 ternary_expression «name.isEmpty() ? 0 : place(name.length())» reach=33:16 [true@33:33-33:34 false@33:37-33:57]`,
		`38:9 enhanced_for_statement «for (int cell : cells)» reach=38:9 [body@38:32-44:10 exit]`,
		`40:17 switch_label «case 0» reach=39:22 [case 1@40:23-40:29]`,
		`41:17 switch_label «case 1, 2» reach=39:22 [case 2@41:26-41:32]`,
		`42:17 switch_label «default» reach=39:22 [default@42:24-42:46]`,
		`42:28 ternary_expression «cell > 10 ? 3 : 2» reach=42:28 [true@42:40-42:41 false@42:44-42:45]`,
		`52:20 binary_expression «value == 0 || value < 0» reach=52:20 [operand 1@52:20-52:30(value == 0) operand 2@52:34-52:43(value < 0)]`,
	}
	if !reflect.DeepEqual(java, wantJava) {
		t.Fatalf("java got\n%s", strings.Join(java, "\n"))
	}
	rust := decisionsOf(t, "testdata/e2e_rust/src/lib.rs", "src/lib.rs")
	wantRust := []string{
		`4:5 if_expression «if x > 0 && x < 10» reach=4:5 [true@4:24-6:6 false]`,
		`4:8 binary_expression «x > 0 && x < 10» reach=4:8 [operand 1@4:8-4:13(x > 0) operand 2@4:17-4:23(x < 10)]`,
		`8:9 match_arm «1 =>» reach=7:5 [case 1@8:14-8:15]`,
		`9:9 match_arm «_ =>» reach=7:5 [case 2@9:14-9:15]`,
		`19:9 if_expression «if self.ok» reach=19:9 [true@19:20-19:25 false@19:26-19:36]`,
	}
	if !reflect.DeepEqual(rust, wantRust) {
		t.Fatalf("rust got\n%s", strings.Join(rust, "\n"))
	}
	clj := decisionsOf(t, "../coverage/testdata/branches/clojure/core.clj", "src/demo/core.clj")
	wantClj := []string{
		`4:3 if «(if (pos? x)» reach=4:3 []`,
		`6:5 when «(when (neg? x) :negative)))» reach=6:5 []`,
		`9:3 cond «(cond» reach=9:3 []`,
		`15:3 if «(if x 1 0))» reach=15:3 []`,
	}
	if !reflect.DeepEqual(clj, wantClj) {
		t.Fatalf("clojure got\n%s", strings.Join(clj, "\n"))
	}
}
