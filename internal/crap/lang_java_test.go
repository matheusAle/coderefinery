package crap

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/model"
	"github.com/matheusAle/coderefinery/internal/source"
	"github.com/matheusAle/coderefinery/internal/treediff"
)

func javaRows(t *testing.T, source, path string) []fnRow {
	t.Helper()
	fns, err := JavaFunctions(source, path, "/proj")
	if err != nil {
		t.Fatalf("JavaFunctions: %v", err)
	}
	var rows []fnRow
	for _, f := range fns {
		if f.Language != "java" || f.Path != path || f.StartByte != -1 || f.EndByte != -1 {
			t.Fatalf("unexpected metadata %+v", f)
		}
		rows = append(rows, fnRow{f.Namespace, f.Name, f.Complexity})
	}
	return rows
}

// Ported from upstream tests/test_java.py.
const javaScoreSource = `
class Sample {
    int score(boolean a, boolean b, int[] values) {
        for (int i = 0; i < values.length; i++) {
        }
        for (int value : values) {
        }
        while (a) {
            a = false;
        }
        do {
            b = false;
        } while (b);
        if (a && b || values.length > 0) {
        }
        try {
            return a ? 1 : 0;
        } catch (RuntimeException ex) {
            return 2;
        }
    }
}
`

const javaNestedSource = `
class Sample {
    int nested(boolean a, boolean b, int[] values) {
        for (int i = 0; i < values.length; i++) {
            if (a) {
            }
        }
        for (int value : values) {
            if (b) {
            }
        }
        while (a) {
            if (b) {
            }
            a = false;
        }
        do {
            if (a) {
            }
            b = false;
        } while (b);
        try {
            return a ? (b ? 1 : 0) : 2;
        } catch (RuntimeException ex) {
            if (values.length > 0) {
                return values[0];
            }
            return 3;
        }
    }

    int switched(int value) {
        switch (value) {
            case 1:
                if (value > 0) {
                    return 1;
                }
                return 0;
            default:
                return 2;
        }
    }
}
`

func TestJavaCountsTheSameDecisionsAsCrap4java(t *testing.T) {
	expectRows(t, javaRows(t, javaScoreSource, "Sample.java"), fnRow{"Sample", "score", 10})
	expectRows(t, javaRows(t, javaNestedSource, "Sample.java"),
		fnRow{"Sample", "nested", 13}, fnRow{"Sample", "switched", 4})
}

func TestJavaAnonymousClassMethodsAreNotEntriesAndDoNotRaiseComplexity(t *testing.T) {
	source := `
class Sample {
    int outer() {
        Runnable runnable = new Runnable() {
            @Override
            public void run() {
                if (true) {
                }
            }
        };
        return 1;
    }
}
`
	expectRows(t, javaRows(t, source, "Sample.java"), fnRow{"Sample", "outer", 1})
}

func TestJavaIgnoresConstructorsAndNamesTheClassForUmlViewer(t *testing.T) {
	source := `
    package demo.pkg;
    public class Board {
        Board() { if (true) {} }
        public void place(int x) { if (x > 0 && ready) return; }
        class Inner { void tick() { return; } }
    }
    `
	fns, err := JavaFunctions(source, "src/demo/pkg/Board.java", "/proj")
	if err != nil {
		t.Fatal(err)
	}
	expectRows(t, javaRows(t, source, "src/demo/pkg/Board.java"),
		fnRow{"demo.pkg.Board", "place", 3}, fnRow{"demo.pkg.Board.Inner", "tick", 1})
	if fns[0].JacocoClass != "demo.pkg.Board" || fns[1].JacocoClass != "demo.pkg.Board$Inner" {
		t.Fatalf("jacoco classes = %q, %q", fns[0].JacocoClass, fns[1].JacocoClass)
	}
}

func TestJavaKeywordsInCommentsAndStringsAreNotDecisions(t *testing.T) {
	source := `
    class Sample {
        int stable() {
            String text = "if && || ? case default catch";
            // if && || ? case default catch
            /* if && || ? case default catch */
            return 1;
        }
    }
    `
	expectRows(t, javaRows(t, source, "Sample.java"), fnRow{"Sample", "stable", 1})
}

// Spec-crapper §8.4 observations; rows verified against upstream by
// TestJavaReaderMatchesUpstream.
func TestJavaFieldInitialiserEnumInterfaceRecordAndSwitchRules(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "java", "cases", "field_initialisers.java"))
	if err != nil {
		t.Fatal(err)
	}
	fns, err := JavaFunctions(string(data), "S.java", "/proj")
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		Namespace, Name, Jacoco string
		CC, Start, End          int
	}
	var got []row
	for _, f := range fns {
		got = append(got, row{f.Namespace, f.Name, f.JacocoClass, f.Complexity, f.StartLine, f.EndLine})
	}
	want := []row{
		{"a.b.S", "run", "a.b.S", 2, 5, 5},     // anonymous class in a field initialiser (quirk)
		{"a.b.S.E", "f", "a.b.S$E", 2, 10, 10}, // enum constant body
		{"a.b.S.E", "g", "a.b.S$E", 1, 14, 14}, //
		{"a.b.S", "h", "a.b.S", 4, 17, 25},     // starts at the @Override line; 3 labels + default
		{"a.b.S.I", "d", "a.b.S$I", 2, 29, 29}, // interface default method; abstract one skipped
		{"a.b.S.R", "y", "a.b.S$R", 2, 36, 36}, // record method; compact constructor skipped
		{"a.b.S", "l", "a.b.S", 2, 39, 46},     // lambda `if` counts, local class does not
		{"a.b.S", "p", "a.b.S", 7, 48, 58},     // instanceof && + pattern switch with guard
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions:\n got  %v\n want %v", got, want)
	}
}

func TestJavaSpecOneLiners(t *testing.T) {
	expectRows(t, javaRows(t, "class Sample { Runnable r = () -> { class Local { int score() { if (true) return 1; return 0; } } return null; }; int outer() { return 1; } }", "S.java"),
		fnRow{"Sample", "outer", 1})
	expectRows(t, javaRows(t, "class A { static { if (a) {} } void m() { new Thread() { public void run() {} }; try (var x = r()) {} catch (A | B e) {} finally {} } }\n@interface Ann { String v() default \"\"; }\n", "A.java"),
		fnRow{"A", "m", 2})
	expectRows(t, javaRows(t, "class G<T extends Comparable<T>> { <U> U m(java.util.List<? extends U> l) { return l.isEmpty() ? null : l.get(0); } }", "G.java"),
		fnRow{"G", "m", 2})
	huge := "class Huge {\n  int huge() { return " + strings.Repeat("1 + ", 1199) + "1; }\n}\n"
	expectRows(t, javaRows(t, huge, "Huge.java"), fnRow{"Huge", "huge", 1})
}

func TestJavaPackageRegexQuirks(t *testing.T) {
	cases := map[string]string{
		"/* package wrong.pkg; */\npackage right.pkg;\nclass A { void m() {} }\n":   "right.pkg.A",
		"/*\npackage wrong.pkg;\n*/\npackage right.pkg;\nclass A { void m() {} }\n": "wrong.pkg.A", // first match, even in a comment
		"package é.ü;\nclass A { void m() {} }\n":                                   "A",           // must start with [a-zA-Z_]
		"package a.é_1;\nclass A { void m() {} }\n":                                 "a.é_1.A",     // Python \w is Unicode
		"\x0b package  x ;\nclass A { void m() {} }\n":                              "x.A",         // Python \s includes \v
		"class A { void m() {} }\n":                                                 "A",
	}
	for src, want := range cases {
		rows := javaRows(t, src, "A.java")
		if len(rows) != 1 || rows[0].Namespace != want {
			t.Errorf("%q: namespace rows %v, want %q", src, rows, want)
		}
	}
}

func TestJavaTopLevelMethodsHaveNoTypeChainAndAreSkipped(t *testing.T) {
	expectRows(t, javaRows(t, "void main() { if (a) {} }\nclass A { int f() { return 1; } }\n", "Main.java"), fnRow{"A", "f", 1})
}

func TestJavaUnparsableSourceIsAnError(t *testing.T) {
	if fns, err := JavaFunctions("class A {", "A.java", "/proj"); err == nil || fns != nil {
		t.Fatalf("JavaFunctions on broken input = %v, %v; want an error", fns, err)
	}
}

func TestJavaIsARegisteredReader(t *testing.T) {
	fns, err := FunctionsInFile("java", javaScoreSource, "Sample.java", "/proj")
	if err != nil || len(fns) != 1 || fns[0].Name != "score" {
		t.Fatalf("FunctionsInFile(java) = %v, %v", fns, err)
	}
}

// JaCoCo INSTRUCTION coverage through the reader's names: nested classes
// look up the `$` binary name, the start line includes annotations and the
// nearest JaCoCo method line wins, anonymous classes in field initialisers
// miss (their JaCoCo class is Outer$1).
func TestJavaJacocoCoverageMapping(t *testing.T) {
	xml := `<?xml version="1.0"?><!DOCTYPE report PUBLIC "-//JACOCO//DTD Report 1.1//EN" "report.dtd">
<report name="r"><package name="demo/pkg">
<class name="demo/pkg/Board"><method name="place" line="5"><counter type="INSTRUCTION" missed="1" covered="3"/></method>
<method name="place" line="20"><counter type="INSTRUCTION" missed="4" covered="0"/></method>
<method name="run" line="9"><counter type="LINE" missed="0" covered="1"/></method></class>
<class name="demo/pkg/Board$Inner"><method name="tick" line="12"><counter type="INSTRUCTION" missed="0" covered="2"/></method></class>
<class name="demo/pkg/Board$1"><method name="run" line="9"><counter type="INSTRUCTION" missed="0" covered="5"/></method></class>
</package></report>`
	index, err := coverage.ParseJacocoIndex(xml)
	if err != nil {
		t.Fatal(err)
	}
	src := `package demo.pkg;
public class Board {
    Runnable r = new Runnable() { public void run() {} };
    @Deprecated
    public void place(int x) {
        if (x > 0 && ready) return;
    }

    class Inner {
        void tick() { return; }
    }

    void missing() {}

    @Override
    @Deprecated
    public void place(String s) {
        place(s.length());
    }
}
`
	fns, err := JavaFunctions(src, "src/demo/pkg/Board.java", "/proj")
	if err != nil {
		t.Fatal(err)
	}
	bundle := coverage.NewBundle()
	bundle.Jacoco = index
	got := map[string]string{}
	for _, fn := range fns {
		key := fmt.Sprintf("%s#%s@%d", fn.JacocoClass, fn.Name, fn.StartLine)
		if pct, ok := bundle.PercentFor(fn); ok {
			got[key] = fmt.Sprint(pct)
		} else {
			got[key] = "nil"
		}
	}
	want := map[string]string{
		"demo.pkg.Board#run@3":         "nil", // only a LINE counter on Board#run; Board$1 is never consulted
		"demo.pkg.Board#place@4":       "75",  // @Deprecated line 4, nearest JaCoCo line 5
		"demo.pkg.Board$Inner#tick@10": "100",
		"demo.pkg.Board#missing@13":    "nil",
		"demo.pkg.Board#place@15":      "0", // nearest is line 20 (0%), not line 5
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("coverage:\n got  %v\n want %v", got, want)
	}
}

const e2eJavaReport = `CRAP Report
===========
Languages: java

Function                       Namespace                             CC    Cov%     CRAP
----------------------------------------------------------------------------------------
score                          demo.Board                             6  87.5%      6.1
place                          demo.Board                             2   0.0%      6.0
run                            demo.Board                             2   0.0%      6.0
place                          demo.Board                             3  75.0%      3.1
empty                          demo.Board.Cell                        2  50.0%      2.5
speed                          demo.Board.Mode                        1   0.0%      2.0
speed                          demo.Board.Mode                        1   0.0%      2.0
`

const e2eJavaEDN = `{:entries [
  {:name "score", :namespace "demo.Board", :complexity 6, :coverage 87.5, :crap 6.0703}
  {:name "place", :namespace "demo.Board", :complexity 2, :coverage 0.0, :crap 6.0}
  {:name "run", :namespace "demo.Board", :complexity 2, :coverage 0.0, :crap 6.0}
  {:name "place", :namespace "demo.Board", :complexity 3, :coverage 75.0, :crap 3.1406}
  {:name "empty", :namespace "demo.Board.Cell", :complexity 2, :coverage 50.0, :crap 2.5}
  {:name "speed", :namespace "demo.Board.Mode", :complexity 1, :coverage 0.0, :crap 2.0}
  {:name "speed", :namespace "demo.Board.Mode", :complexity 1, :coverage 0.0, :crap 2.0}
]}
`

// Smoke: the whole pipeline on a Java tree with a synthetic jacoco.xml,
// against output recorded from upstream crapper (--use-existing-coverage,
// so no Maven run).
func TestSmokeJavaExistingCoverageMatchesUpstream(t *testing.T) {
	root := copyTree(t, "e2e_java")
	code, stdout, stderr := runCrap("--root", root, "--use-existing-coverage", "--threshold", "5")
	if code != 2 {
		t.Fatalf("exit %d, want 2; stderr %s", code, stderr)
	}
	if stdout != e2eJavaReport {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, e2eJavaReport)
	}
	metrics := filepath.Join(root, ".metrics", "crap.edn")
	if want := "Wrote " + metrics + "\nCRAP threshold exceeded: 6.1 > 5.0\n"; stderr != want {
		t.Fatalf("stderr %q, want %q", stderr, want)
	}
	edn, err := os.ReadFile(metrics)
	if err != nil {
		t.Fatal(err)
	}
	if string(edn) != e2eJavaEDN {
		t.Fatalf("crap.edn:\n%s\nwant:\n%s", edn, e2eJavaEDN)
	}
}

// upstreamJavaScript prints upstream java.functions_in_source for each
// path on stdin as one JSON array per line.
const upstreamJavaScript = `
import json, sys
from pathlib import Path
from crapper.languages.java import functions_in_source
for line in sys.stdin:
    path = line.rstrip("\n")
    src = Path(path).read_text(encoding="utf-8", errors="replace")
    fns = functions_in_source(src, path)
    print(json.dumps([[f.namespace, f.name, f.complexity, f.start_line, f.end_line, f.jacoco_class] for f in fns]))
`

// TestJavaReaderMatchesUpstream runs upstream crapper's Java reader (its
// venv) over testdata/java, plus any directories listed in
// CODEREFINERY_JAVA_CORPUS (colon-separated), and compares every entry: name,
// namespace, complexity, start/end line and JaCoCo class.
func TestJavaReaderMatchesUpstream(t *testing.T) {
	cwd, _ := os.Getwd()
	root, err := treediff.FindRoot(cwd)
	if err != nil {
		t.Skip("upstream venv not found:", err)
	}
	python := treediff.DefaultConfig(root).Python
	if _, err := os.Stat(python); err != nil {
		t.Skip("upstream venv not found")
	}
	dirs := []string{filepath.Join(root, "testdata", "java")}
	if extra := os.Getenv("CODEREFINERY_JAVA_CORPUS"); extra != "" {
		dirs = append(dirs, strings.Split(extra, ":")...)
	}
	files, err := treediff.Collect(dirs, "java")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python, "-I", "-c", upstreamJavaScript)
	cmd.Stdin = strings.NewReader(strings.Join(files, "\n") + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("upstream reader: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) != len(files) {
		t.Fatalf("upstream printed %d lines for %d files", len(lines), len(files))
	}
	entries := 0
	for i, path := range files {
		text, err := source.Read(path)
		if err != nil {
			t.Fatal(err)
		}
		fns, err := JavaFunctions(text, path, root)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		got, err := json.Marshal(javaTuples(fns))
		if err != nil {
			t.Fatal(err)
		}
		var want []any
		if err := json.Unmarshal([]byte(lines[i]), &want); err != nil {
			t.Fatal(err)
		}
		wantJSON, _ := json.Marshal(want)
		if string(got) != string(wantJSON) {
			t.Errorf("%s:\n got  %s\n want %s", path, got, wantJSON)
		}
		entries += len(fns)
	}
	if entries < 50 {
		t.Fatalf("only %d entries compared; corpus too small", entries)
	}
}

func javaTuples(fns []model.Function) [][]any {
	out := [][]any{}
	for _, f := range fns {
		out = append(out, []any{f.Namespace, f.Name, f.Complexity, f.StartLine, f.EndLine, f.JacocoClass})
	}
	return out
}
