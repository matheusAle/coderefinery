package react

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const profileA = `{"renderers":[{"package":"react-dom","version":"19.3.1","bundleType":1}],
 "components":{"Clicker":{"renders":4,"mounts":1,"totalMs":2.5,"maxMs":1.25},
               "TestComponent":{"renders":2,"mounts":1,"totalMs":0.5,"maxMs":0.5}},
 "untimed":false,"error":""}`

const profileB = `{"renderers":[],"components":{"Clicker":{"renders":2,"mounts":1,"totalMs":1,"maxMs":2},
 "Row":{"renders":3,"mounts":3,"totalMs":0.3,"maxMs":0.1}},"untimed":false,"error":""}`

func TestProfileMergeSumsCountsAndKeepsTheMax(t *testing.T) {
	var p Profile
	for _, doc := range []string{profileA, profileB} {
		if err := p.Merge([]byte(doc)); err != nil {
			t.Fatal(err)
		}
	}
	want := map[string]RenderStats{
		"Clicker":       {Renders: 6, Mounts: 2, TotalMs: 3.5, MaxMs: 2},
		"TestComponent": {Renders: 2, Mounts: 1, TotalMs: 0.5, MaxMs: 0.5},
		"Row":           {Renders: 3, Mounts: 3, TotalMs: 0.3, MaxMs: 0.1},
	}
	if !reflect.DeepEqual(p.Components, want) {
		t.Fatalf("components = %+v, want %+v", p.Components, want)
	}
	if p.Renderers != 1 || p.Untimed || p.Error != "" || p.Problem() != "" {
		t.Fatalf("renderers=%d untimed=%v error=%q problem=%q", p.Renderers, p.Untimed, p.Error, p.Problem())
	}
	if got := p.Components["Clicker"].Updates(); got != 4 {
		t.Fatalf("Clicker updates = %d, want 4", got)
	}
}

func TestReadProfilesMergesEveryFileAndReportsBadJSON(t *testing.T) {
	dir := t.TempDir()
	for name, doc := range map[string]string{"profile-1.json": profileA, "profile-2.json": profileB, "other.json": "nope"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(doc), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	var p Profile
	if err := p.ReadProfiles(dir); err != nil {
		t.Fatal(err)
	}
	if got := p.Components["Clicker"]; got != (RenderStats{Renders: 6, Mounts: 2, TotalMs: 3.5, MaxMs: 2}) {
		t.Fatalf("Clicker = %+v", got)
	}
	if err := os.WriteFile(filepath.Join(dir, "profile-3.json"), []byte("{"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := (&Profile{}).ReadProfiles(dir); err == nil || !strings.Contains(err.Error(), "profile-3.json") {
		t.Fatalf("bad JSON error = %v", err)
	}
}

func TestProfileProblemNamesWhyThereAreNoNumbers(t *testing.T) {
	cases := map[string]Profile{
		"React never connected":       {},
		"production React build":      {Renderers: 1, Untimed: true},
		"profiler hook failed: boom!": {Renderers: 1, Error: "boom!\n    at commitRoot"},
	}
	for want, p := range cases {
		got := p.Problem()
		if !strings.HasPrefix(got, "--profile: ") || !strings.Contains(got, want) || strings.Contains(got, "\n") {
			t.Errorf("Problem() = %q, want one line containing %q", got, want)
		}
	}
}

func profiledUnits() []Unit {
	return []Unit{
		{Name: "Clicker", Namespace: "Clicker", Kind: KindComponent, Path: "src/Clicker.tsx", Cost: Cost{InlineFunctions: 2, InlineObjects: 1}},
		{Name: "Label", Namespace: "Clicker", Kind: KindComponent, Path: "src/Clicker.tsx"},
		{Name: "TodoList", Namespace: "TodoList", Kind: KindComponent, Path: "src/TodoList.tsx", Cost: Cost{InlineFunctions: 2}},
		{Name: "TodoList.Row", Namespace: "TodoList", Kind: KindComponent, Path: "src/TodoList.tsx", Cost: Cost{InlineObjects: 1}},
		{Name: "Row", Namespace: "Grid", Kind: KindComponent, Path: "src/Grid.tsx"},
		{Name: "IconButton", Namespace: "IconButton", Kind: KindComponent, Path: "src/IconButton.tsx"},
		{Name: "useClicks", Namespace: "useClicks", Kind: KindHook, Path: "src/useClicks.ts"},
	}
}

func TestJoinRendersJoinsByNameAndFlagsHotInlineProps(t *testing.T) {
	p := Profile{Renderers: 1, Components: map[string]RenderStats{
		"Clicker":       {Renders: 4, Mounts: 1, TotalMs: 2, MaxMs: 1},
		"Label":         {Renders: 4, Mounts: 1, TotalMs: 1, MaxMs: 0.5},
		"TodoList":      {Renders: 2, Mounts: 1},
		"Row":           {Renders: 3, Mounts: 3},
		"TestComponent": {Renders: 9, Mounts: 1},
		"useClicks":     {Renders: 9, Mounts: 1},
	}}
	rows, notRendered := JoinRenders(profiledUnits(), p)
	want := []RenderRow{
		{Name: "Clicker", Namespace: "Clicker", Path: "src/Clicker.tsx", Stats: p.Components["Clicker"], Inline: 3, Flagged: true},
		{Name: "Label", Namespace: "Clicker", Path: "src/Clicker.tsx", Stats: p.Components["Label"]},
		{Name: "TodoList", Namespace: "TodoList", Path: "src/TodoList.tsx", Stats: p.Components["TodoList"], Inline: 2},
		{Name: "Row", Ambiguous: 2, Stats: p.Components["Row"]},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Fatalf("rows:\n%+v\nwant:\n%+v", rows, want)
	}
	if !reflect.DeepEqual(notRendered, []string{"IconButton"}) {
		t.Fatalf("notRendered = %v", notRendered)
	}
}

func TestUpdatesPerMountThresholdIsInclusive(t *testing.T) {
	u := []Unit{{Name: "A", Namespace: "A", Kind: KindComponent, Cost: Cost{InlineObjects: 1}}}
	for _, c := range []struct {
		stats RenderStats
		ratio float64
		flag  bool
	}{
		{RenderStats{Renders: 7, Mounts: 2}, 2.5, false},
		{RenderStats{Renders: 8, Mounts: 2}, 3, true},
		{RenderStats{Renders: 3, Mounts: 0}, 3, true},
	} {
		rows, _ := JoinRenders(u, Profile{Components: map[string]RenderStats{"A": c.stats}})
		if len(rows) != 1 || rows[0].UpdatesPerMount() != c.ratio || rows[0].Flagged != c.flag {
			t.Errorf("%+v: rows = %+v, want ratio %v flagged %v", c.stats, rows, c.ratio, c.flag)
		}
	}
}

func TestFormatRendersAndEDN(t *testing.T) {
	rows := []RenderRow{
		{Name: "Clicker", Namespace: "Clicker", Path: "src/Clicker.tsx", Stats: RenderStats{4, 1, 2.25, 1.5}, Inline: 3, Flagged: true},
		{Name: "Row", Ambiguous: 2, Stats: RenderStats{3, 3, 0.3, 0.1}},
	}
	got := FormatRenders(rows, []string{"IconButton", "TodoList.Row"}, []string{"vitest"})
	want := `
Runtime renders (vitest test run; durations are React actualDuration in ms, noisy)

Component                      Namespace                           Renders Mounts Updates Upd/Mount TotalMs MaxMs Inline Flag
-----------------------------------------------------------------------------------------------------------------------------
Clicker                        Clicker                                   4      1       3       3.0     2.2   1.5      3 re-renders with inline props
Row                            (2 components)                            3      3       0       0.0     0.3   0.1      -

Not rendered by the tests: IconButton, TodoList.Row
`
	if got != want {
		t.Fatalf("FormatRenders:\n%s\nwant:\n%s", got, want)
	}
	if empty := FormatRenders(nil, nil, []string{"jest"}); !strings.HasSuffix(empty, "\n\nNo analyzed component rendered in the tests.\n") {
		t.Fatalf("empty = %q", empty)
	}
	doc := WithRenders("{:entries []\n :components []\n :violations []}\n", RendersEDN(rows, []string{"IconButton"}, []string{"vitest"}))
	wantDoc := `{:entries []
 :components []
 :violations []
 :renders {:runners ["vitest"], :not-rendered ["IconButton"],
  :components [
  {:name "Clicker", :namespace "Clicker", :path "src/Clicker.tsx", :renders 4, :mounts 1, :updates 3, :total-ms 2.25, :max-ms 1.5, :inline-props 3, :ambiguous 0, :flagged true}
  {:name "Row", :namespace nil, :path nil, :renders 3, :mounts 3, :updates 0, :total-ms 0.3, :max-ms 0.1, :inline-props 0, :ambiguous 2, :flagged false}
]}}
`
	if doc != wantDoc {
		t.Fatalf("EDN:\n%s\nwant:\n%s", doc, wantDoc)
	}
}
