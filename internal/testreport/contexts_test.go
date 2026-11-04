package testreport

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/testutil"
)

const contextsReport = `{"files": {
  "src/shop/cart.py": {
    "contexts": {
      "3": ["tests/test_cart.py::test_total|run", "tests/test_cart.py::test_total|setup"],
      "7": ["tests/test_cart.py::test_discount|run", ""],
      "9": ["tests/test_cart.py::test_total|run"],
      "x": ["tests/test_cart.py::test_bad|run"]
    },
    "functions": {
      "Cart.total": {"executed_lines": [3]},
      "Cart.discount": {"executed_lines": [7]},
      "": {"executed_lines": [9]}
    }
  }
}}`

func TestReadContextsMapsEachTestToTheFunctionsItRan(t *testing.T) {
	report := testutil.WriteFile(t, filepath.Join(t.TempDir(), "coverage.json"), contextsReport)
	m := &Matrix{}
	readContexts(report, "svc", m)
	want := map[string]map[string]bool{
		"svc/tests/test_cart.py::test_total":    {"svc/src/shop/cart.py::Cart.total": true},
		"svc/tests/test_cart.py::test_discount": {"svc/src/shop/cart.py::Cart.discount": true},
	}
	if !reflect.DeepEqual(m.Runs, want) {
		t.Fatalf("Runs = %v\nwant   %v", m.Runs, want)
	}
	root := &Matrix{}
	readContexts(report, ".", root)
	if _, ok := root.Runs["tests/test_cart.py::test_total"]; !ok || len(root.Runs) != 2 {
		t.Fatalf("module \".\" adds no prefix: %v", root.Runs)
	}
}

func TestReadContextsIgnoresMissingAndMalformedReports(t *testing.T) {
	dir := t.TempDir()
	m := &Matrix{}
	readContexts(filepath.Join(dir, "missing.json"), "", m)
	readContexts(testutil.WriteFile(t, filepath.Join(dir, "bad.json"), "{"), "", m)
	if m.Runs != nil {
		t.Fatalf("Runs = %v, want nil (unknown)", m.Runs)
	}
}

func TestChangedRowsKeepsChangedTestsAndKillersOfChangedSources(t *testing.T) {
	root := discover.Resolve(t.TempDir())
	testutil.InitRepo(t, root)
	for _, rel := range []string{"src/a.py", "src/b.py", "tests/test_a.py", "tests/test_b.py"} {
		testutil.WriteFile(t, filepath.Join(root, rel), "x = 1\n")
	}
	testutil.Git(t, root, "add", "-A")
	testutil.Git(t, root, "commit", "-q", "-m", "base")
	testutil.WriteFile(t, filepath.Join(root, "src/a.py"), "x = 2\n")
	testutil.WriteFile(t, filepath.Join(root, "tests/test_b.py"), "x = 2\n")
	m := &Matrix{Mutants: []Mutant{{File: "src/a.py"}, {File: "src/b.py"}}}
	rows := []Row{
		{ID: "kills-a", File: "tests/test_a.py", kills: []int{0}},
		{ID: "kills-b", File: "tests/test_other.py", kills: []int{1}},
		{ID: "changed-test", File: "tests/test_b.py", kills: []int{1}},
		{ID: "idle", File: "tests/test_c.py"},
	}
	got, err := changedRows(root, m, rows)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, r := range got {
		ids = append(ids, r.ID)
	}
	if want := []string{"kills-a", "changed-test"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("changedRows = %v, want %v", ids, want)
	}
}
