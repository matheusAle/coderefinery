package mutate

// Ported from upstream tests/test_metrics.py and the snapshot parts of
// tests/test_boundaries.py.

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func fresh(namespace, name, file, digest string, killed int, private bool) FormResult {
	id := "defn/" + name
	if private {
		id = "defn-/" + name
	}
	return FormResult{ID: id, Namespace: namespace, Name: name, Private: private, File: file, Line: 2, EndLine: 4,
		Digest: digest, Killed: killed, Uncovered: 1, Sites: 2}
}

func loadSnapshot(t *testing.T, root, namespace string) *ordMap {
	t.Helper()
	p, err := snapshotPath(root, namespace)
	if err != nil {
		t.Fatal(err)
	}
	v, err := ednLoads(readFile(t, p))
	if err != nil {
		t.Fatal(err)
	}
	return v.(*ordMap)
}

func formsByID(m *ordMap) map[string]*ordMap {
	out := map[string]*ordMap{}
	for _, f := range formsOf(m) {
		out[f.Get("id").(string)] = f
	}
	return out
}

func mustWrite(t *testing.T, root, key string, forms []FormResult, outcomes map[string]string) []string {
	t.Helper()
	written, err := writeResults(root, key, forms, outcomes)
	if err != nil {
		t.Fatal(err)
	}
	return written
}

func ednFilesRel(t *testing.T, root string) []string {
	var rel []string
	for _, p := range ednFiles(root) {
		r, _ := filepath.Rel(root, p)
		rel = append(rel, r)
	}
	return rel
}

func TestOneSnapshotPerNamespacePreservesTheOtherFile(t *testing.T) {
	root := tempRoot(t)
	mustWrite(t, root, "a.go", []FormResult{fresh("demo", "Run", "a.go", "hash-a", 1, false)}, nil)
	written := mustWrite(t, root, "b.go", []FormResult{fresh("demo", "Stop", "b.go", "hash-b", 4, false)}, nil)
	if want := []string{filepath.Join(root, ".metrics", "mutate", "demo.edn")}; !reflect.DeepEqual(written, want) {
		t.Fatalf("written %v", written)
	}
	data := loadSnapshot(t, root, "demo")
	if data.Get("namespace") != "demo" || data.Get("source") != "b.go" {
		t.Fatalf("snapshot %s", pyRepr(data))
	}
	by := formsByID(data)
	if by["defn/Run"].Get("killed") != int64(1) || by["defn/Stop"].Get("killed") != int64(4) || by["defn/Stop"].Get("sites") != int64(2) {
		t.Fatalf("forms %s", pyRepr(data.Get("forms")))
	}
	if files := ednFilesRel(t, root); !reflect.DeepEqual(files, []string{".metrics/mutate/demo.edn"}) {
		t.Fatalf("files %v", files)
	}
	mustWrite(t, root, "a.go", []FormResult{fresh("demo", "Run", "a.go", "hash-a", 9, false)}, nil)
	by = formsByID(loadSnapshot(t, root, "demo"))
	if by["defn/Run"].Get("killed") != int64(9) || by["defn/Stop"].Get("killed") != int64(4) {
		t.Fatal("rewriting a.go must keep b.go's form")
	}
}

func TestSnapshotTextIsByteExact(t *testing.T) {
	root := tempRoot(t)
	defer func(old func() string) { testedAt = old }(testedAt)
	testedAt = func() string { return "2026-11-03T12:59:57-03:00" }
	id := mutationID("src/demo.py", "demo", "defn/add", 24, 25, ">", ">=")
	f := fresh("demo", "add", "src/demo.py", "1a13", 1, false)
	mustWrite(t, root, "src/demo.py", []FormResult{f}, map[string]string{id: "survived", "foreign": "killed"})
	want := `{:version 1 :tested-at "2026-11-03T12:59:57-03:00" :source "src/demo.py" :namespace "demo" ` +
		`:outcomes {"[\"src/demo.py\",\"demo\",\"defn/add\",24,25,\">\",\">=\"]" :survived} ` +
		`:forms [{:id "defn/add" :kind "defn" :file "src/demo.py" :line 2 :end-line 4 :hash "1a13" :killed 1 :survived 0 :uncovered 1 :sites 2}]}` + "\n"
	if got := readFile(t, filepath.Join(root, ".metrics", "mutate", "demo.edn")); got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestPrivateFormIDAndHistoryRoundTrip(t *testing.T) {
	root := tempRoot(t)
	outcome := `["hide.clj","demo.core","defn-/hide",8,12,"false","true"]`
	mustWrite(t, root, "hide.clj", []FormResult{fresh("demo.core", "hide", "hide.clj", "digest", 1, true)}, map[string]string{outcome: "killed"})
	data := loadSnapshot(t, root, "demo.core")
	form := formsOf(data)[0]
	if form.Get("id") != "defn-/hide" || form.Get("kind") != "defn-" {
		t.Fatalf("form %s", pyRepr(form))
	}
	h := loadHistory(root, "hide.clj")
	if h.Forms[formKey{"demo.core", "defn-/hide"}] != "digest" || h.Outcomes[outcome] != "killed" {
		t.Fatalf("history %+v", h)
	}
}

func TestSnapshotPathsFollowTheNamespace(t *testing.T) {
	root := "/r"
	for ns, want := range map[string]string{
		"example.com/demo.Widget": "/r/.metrics/mutate/example.com/demo.Widget.edn",
		"crate::foo::Bar":         "/r/.metrics/mutate/crate/foo/Bar.edn",
		`a\b`:                     "/r/.metrics/mutate/a/b.edn",
		"demo.board.Board":        "/r/.metrics/mutate/demo.board.Board.edn",
	} {
		if got, err := snapshotPath(root, ns); err != nil || got != want {
			t.Errorf("%s: %s %v", ns, got, err)
		}
	}
	for _, bad := range []string{"", "a//b", "../x", "a/."} {
		if _, err := snapshotPath(root, bad); err == nil || !strings.Contains(err.Error(), "unsafe namespace") {
			t.Errorf("%q must be unsafe, got %v", bad, err)
		}
	}
}

func TestANonCanonicalSnapshotIsFoldedIntoTheNamespaceFile(t *testing.T) {
	root := tempRoot(t)
	stale := writeFile(t, filepath.Join(root, ".metrics", "mutate", "legacy.edn"),
		`{:version 1 :namespace "demo" :source "b.go" :forms [{:id "defn/Stop" :file "b.go" :line 3 :hash "hash-b" :killed 4 :survived 0 :uncovered 1 :sites 2}] :outcomes {}}`+"\n")
	mustWrite(t, root, "a.go", []FormResult{fresh("demo", "Run", "a.go", "hash-a", 1, false)}, nil)
	if _, err := os.Stat(stale); err == nil {
		t.Fatal("legacy.edn must be folded and removed")
	}
	by := formsByID(loadSnapshot(t, root, "demo"))
	if by["defn/Stop"].Get("killed") != int64(4) || by["defn/Run"].Get("file") != "a.go" {
		t.Fatalf("forms %v", by)
	}
}

func TestDroppingAFileRemovesItsNamespaceWhenNothingElseRemains(t *testing.T) {
	root := tempRoot(t)
	mustWrite(t, root, "a.go", []FormResult{fresh("demo", "Run", "a.go", "hash-a", 1, false)}, nil)
	mustWrite(t, root, "a.go", nil, nil)
	if _, err := os.Stat(filepath.Join(root, ".metrics", "mutate", "demo.edn")); err == nil {
		t.Fatal("demo.edn must be deleted")
	}
}

func TestAnUnrelatedSnapshotIsLeftInPlace(t *testing.T) {
	root := tempRoot(t)
	legacy := writeFile(t, filepath.Join(root, ".metrics", "mutate", "legacy.edn"), `{:namespace "untouched" :source "c.go" :forms [] :outcomes {}}`+"\n")
	mustWrite(t, root, "a.go", []FormResult{fresh("demo", "Run", "a.go", "h", 1, false)}, nil)
	if _, err := os.Stat(legacy); err != nil {
		t.Fatal("an unrelated empty snapshot must stay")
	}
}

func TestSnapshotsKeepOtherFilesAndOddRecords(t *testing.T) {
	root := tempRoot(t)
	mustWrite(t, root, "b.go", []FormResult{fresh("demo", "Stop", "b.go", "hash-b", 1, false)}, nil)
	mustWrite(t, root, "a.go", nil, nil)
	kept := loadSnapshot(t, root, "demo")
	if kept.Get("source") != "b.go" || formsOf(kept)[0].Get("id") != "defn/Stop" {
		t.Fatalf("kept %s", pyRepr(kept))
	}

	writeFile(t, filepath.Join(root, ".metrics", "mutate", "other.edn"),
		`{:namespace "other" :source "c.go" :forms [{:id "defn/Stay" :file "c.go" :line 1 :hash "h"}] :outcomes {}}`+"\n")
	foreign := mutationID("b.go", "demo", "defn/Stop", 1, 2, "+", "-")
	demo, _ := snapshotPath(root, "demo")
	writeFile(t, demo, `{:namespace "demo" :source "a.go" :forms [{:id "defn/m" :file "b.go" :hash "m"} {:id "defn/a" :file "b.go" :line 1 :hash "a"}] :outcomes {`+
		ednText(foreign)+` :killed}}`+"\n")
	mustWrite(t, root, "a.go", nil, nil)
	rewritten := loadSnapshot(t, root, "demo")
	var ids []string
	for _, f := range formsOf(rewritten) {
		ids = append(ids, f.Get("id").(string))
	}
	if !reflect.DeepEqual(ids, []string{"defn/m", "defn/a"}) {
		t.Fatalf("a form without a line sorts first: %v", ids)
	}
	if rewritten.Get("outcomes").(*ordMap).Get(foreign) != "killed" {
		t.Fatal("foreign outcome dropped")
	}
	if _, err := os.Stat(filepath.Join(root, ".metrics", "mutate", "other.edn")); err != nil {
		t.Fatal("other.edn must survive")
	}

	writeFile(t, filepath.Join(root, ".metrics", "mutate", "odd.edn"), `{:namespace "odd" :forms [{:id 1 :hash "digest" :file "a.go"}] :outcomes {}}`+"\n")
	if h := loadHistory(root, "a.go"); len(h.Forms) != 0 {
		t.Fatalf("a non-string id is ignored, got %v", h.Forms)
	}
	odd := mapOf("outcomes", mapOf("id", ":killed", "x", "maybe"))
	if got := outcomesOf(odd); !reflect.DeepEqual(got, []outcome{{"id", "killed"}}) {
		t.Fatalf("outcomes %v", got)
	}
	if f, ok := formFile(mapOf("file", ""), mapOf("source", "a.go")); !ok || f != "a.go" {
		t.Fatal("an empty :file falls back to :source")
	}
	for _, c := range []struct {
		data *ordMap
		want bool
	}{
		{mapOf("source", "a.go", "forms", []any{}), true},
		{mapOf("source", "b.go", "forms", []any{}), false},
		{mapOf("forms", []any{mapOf("file", "a.go")}), true},
		{mapOf("forms", []any{mapOf("file", "b.go")}), false},
	} {
		if formFileOnly(c.data, "a.go") != c.want {
			t.Errorf("formFileOnly(%s) != %v", pyRepr(c.data), c.want)
		}
	}
}

func ednText(s string) string { out, _ := ednDumps(s); return out }

func TestRemoveEmptyDirsKeepsFilledOnes(t *testing.T) {
	base := filepath.Join(tempRoot(t), "m")
	if err := os.MkdirAll(filepath.Join(base, "empty", "nested"), 0o777); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(base, "full", "keep.edn"), "x")
	removeEmptyDirs(base)
	if _, err := os.Stat(filepath.Join(base, "empty")); err == nil {
		t.Fatal("empty dirs must go")
	}
	if readFile(t, filepath.Join(base, "full", "keep.edn")) != "x" {
		t.Fatal("full dir must stay")
	}
	removeEmptyDirs(filepath.Join(base, "missing"))
}

func TestHistoryFilesSortLikePathlib(t *testing.T) {
	paths := []string{"/m/a.b.edn", "/m/a/b.edn", "/m/a-c.edn"}
	sortPaths(paths)
	if want := []string{"/m/a/b.edn", "/m/a-c.edn", "/m/a.b.edn"}; !reflect.DeepEqual(paths, want) {
		t.Fatalf("got %v", paths)
	}
}
