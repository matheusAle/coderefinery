package mutate

// Ported from upstream tests/test_coverage.py and the coverage part of
// tests/test_boundaries.py, plus the --fix-go-coverage mapping.

import (
	"path/filepath"
	"reflect"
	"testing"
)

func lines(ns ...int) lineSet {
	out := lineSet{}
	for _, n := range ns {
		out[n] = true
	}
	return out
}

func mustCovered(t *testing.T, root, path, language string, fixGo bool) lineSet {
	t.Helper()
	got, err := coveredLines(root, path, language, fixGo)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestJacocoMarksLinesWithCoveredInstructionsOrBranches(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "target", "site", "jacoco", "jacoco.xml"), `<?xml version="1.0"?>
<!DOCTYPE report PUBLIC "-//JACOCO//DTD Report 1.1//EN" "report.dtd">
<report name="demo">
  <package name="demo">
    <sourcefile name="Board.java">
      <line nr="3" mi="0" ci="2" mb="0" cb="0"/>
      <line nr="4" mi="4" ci="0" mb="1" cb="0"/>
      <line nr="5" mi="0" ci="0" mb="0" cb="1"/>
      <line nr="6" mi="0" ci="x" mb="0" cb="1"/>
    </sourcefile>
  </package>
</report>
`)
	src := writeFile(t, filepath.Join(root, "src", "demo", "Board.java"), "package demo;\nclass Board {}\n")
	if got := mustCovered(t, root, src, "java", false); !reflect.DeepEqual(got, lines(3, 5)) {
		t.Fatalf("got %v", got)
	}
	keys, _ := jacocoIndex(root)
	if !reflect.DeepEqual(keys, []string{"demo/Board.java"}) {
		t.Fatalf("index keys %v", keys)
	}
	writeFile(t, filepath.Join(root, "mod", "target", "site", "jacoco", "jacoco.xml"), "<report><package")
	if got := mustCovered(t, root, src, "java", false); !reflect.DeepEqual(got, lines(3, 5)) {
		t.Fatalf("a broken report must be skipped, got %v", got)
	}
}

func TestGoProfileMarksHitLines(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "target", "coverage", "go", "coverage.out"),
		"mode: set\ndemo/widget.go:2.1,4.2 1 1\ndemo/widget.go:5.1,6.2 1 0\n")
	src := writeFile(t, filepath.Join(root, "demo", "widget.go"), "package demo\nfunc Run() {}\n")
	if got := mustCovered(t, root, src, "go", false); !reflect.DeepEqual(got, lines(2, 3, 4)) {
		t.Fatalf("got %v", got)
	}
	if got := mustCovered(t, root, filepath.Join(root, "other", "missing.go"), "go", false); got != nil {
		t.Fatalf("a file absent from the profile has no data, got %v", got)
	}
	empty := filepath.Join(root, "empty")
	writeFile(t, filepath.Join(empty, "keep"), "")
	if got := mustCovered(t, empty, filepath.Join(empty, "demo", "widget.go"), "go", false); got != nil {
		t.Fatalf("no profile, got %v", got)
	}
}

func TestLcovMarksExecutedLines(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "target", "coverage", "python", "lcov.info"), "SF:src/demo/app.py\nDA:1,3\nDA:2,0\nend_of_record\n")
	src := writeFile(t, filepath.Join(root, "src", "demo", "app.py"), "def place():\n    return 1\n")
	for _, language := range []string{"python", "clojure"} {
		if got := mustCovered(t, root, src, language, false); !reflect.DeepEqual(got, lines(1)) {
			t.Errorf("%s: got %v", language, got)
		}
	}
}

func TestAShorterReportPathNeverStealsHits(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "target", "coverage", "python", "lcov.info"),
		"SF:b/app.py\nDA:2,1\nend_of_record\nSF:a/b/app.py\nDA:1,1\nend_of_record\n")
	short := writeFile(t, filepath.Join(root, "b", "app.py"), "def place():\n    return 1\n")
	long := writeFile(t, filepath.Join(root, "a", "b", "app.py"), "def place():\n    return 1\n")
	if got := mustCovered(t, root, long, "python", false); !reflect.DeepEqual(got, lines(1)) {
		t.Errorf("long: %v", got)
	}
	if got := mustCovered(t, root, short, "python", false); !reflect.DeepEqual(got, lines(2)) {
		t.Errorf("short: %v", got)
	}
}

func TestGoProfileDoesNotCrossFiles(t *testing.T) {
	root := tempRoot(t)
	profile := filepath.Join(root, "target", "coverage", "go", "coverage.out")
	writeFile(t, profile, "mode: set\nb/widget.go:2.1,2.2 1 1\na/b/widget.go:3.1,3.2 1 0\n")
	short := writeFile(t, filepath.Join(root, "b", "widget.go"), "package b\nfunc Run() {}\n")
	long := writeFile(t, filepath.Join(root, "a", "b", "widget.go"), "package b\nfunc Run() {}\n")
	if got := mustCovered(t, root, short, "go", false); !reflect.DeepEqual(got, lines(2)) {
		t.Errorf("short: %v", got)
	}
	if got := mustCovered(t, root, long, "go", false); got == nil || len(got) != 0 {
		t.Errorf("long: want an empty set, got %#v", got)
	}
	writeFile(t, profile, "mode: set\na/b/widget.go:2.1,2.2 1 1\n")
	if got := mustCovered(t, root, short, "go", false); got != nil {
		t.Errorf("short must not borrow a/b: %v", got)
	}
	if got := mustCovered(t, root, long, "go", false); !reflect.DeepEqual(got, lines(2)) {
		t.Errorf("long: %v", got)
	}
}

func TestCloverageHTMLBeatsLcovForClojure(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "target", "coverage", "src", "demo", "core.clj.html"),
		`<span title="1 out of 1 forms covered">4&nbsp;</span>`+"\n")
	writeFile(t, filepath.Join(root, "target", "coverage", "clojure", "lcov.info"), "SF:src/demo/core.clj\nDA:9,1\nend_of_record\n")
	src := writeFile(t, filepath.Join(root, "src", "demo", "core.clj"), "(ns demo.core)\n(defn place [] 1)\n")
	if got := mustCovered(t, root, src, "clojure", false); !reflect.DeepEqual(got, lines(4)) {
		t.Fatalf("got %v", got)
	}
}

// Upstream matches import-path keys by suffix only, so a module path that
// does not mirror the directories is never found (DEVIATIONS.md M1).
func TestFixGoCoverageMapsImportPathsThroughGoMod(t *testing.T) {
	root := tempRoot(t)
	writeFile(t, filepath.Join(root, "go.mod"), "module example.com/ubfix\n\ngo 1.22\n")
	writeFile(t, filepath.Join(root, "target", "coverage", "go", "coverage.out"),
		"mode: set\nexample.com/ubfix/calc.go:3.1,4.2 1 1\nexample.com/ubfix/geometry/shapes.go:7.1,7.9 1 1\n")
	calc := writeFile(t, filepath.Join(root, "calc.go"), "package ubfix\n")
	shapes := writeFile(t, filepath.Join(root, "geometry", "shapes.go"), "package geometry\n")
	if got := mustCovered(t, root, calc, "go", false); got != nil {
		t.Fatalf("upstream behaviour: no data, got %v", got)
	}
	if got := mustCovered(t, root, calc, "go", true); !reflect.DeepEqual(got, lines(3, 4)) {
		t.Fatalf("calc.go: %v", got)
	}
	if got := mustCovered(t, root, shapes, "go", true); !reflect.DeepEqual(got, lines(7)) {
		t.Fatalf("geometry/shapes.go: %v", got)
	}
	if got := goImportPath(shapes, root); got != "example.com/ubfix/geometry/shapes.go" {
		t.Fatalf("import path %q", got)
	}
}
