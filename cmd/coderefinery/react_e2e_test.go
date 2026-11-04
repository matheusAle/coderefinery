package main

// End to end: `coderefinery react` on fixtures/react (scratch copy), through
// the dispatcher: complexity-only (always) and with Vitest coverage (needs
// fixtures/react/node_modules: run fixtures/setup-tests.sh).

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const reactNoCoverageReport = `React Report
============

Entry                          Kind      Namespace                            Line   CC    Cov%     CRAP
--------------------------------------------------------------------------------------------------------
Cart#useEffect#1               callback  Cart                                   22   10     N/A      N/A
SettingsPanel                  component SettingsPanel                          13    6     N/A      N/A
Cart                           component Cart                                   16    4     N/A      N/A
Badge                          component Badge                                   6    2     N/A      N/A
Cart.handleCheckout            handler   Cart                                   46    2     N/A      N/A
Toggle                         component Clicker                                20    2     N/A      N/A
Sidebar                        component Dashboard                              28    2     N/A      N/A
TodoList                       component TodoList                                9    2     N/A      N/A
TodoList.TodoRow               component TodoList                               12    2     N/A      N/A
useCounter#useCallback#1       callback  useCounter                              7    2     N/A      N/A
useCounter#useEffect#1         callback  useCounter                             15    2     N/A      N/A
useCounter#useMemo#1           callback  useCounter                             13    2     N/A      N/A
Cart#useEffect#2               callback  Cart                                   42    1     N/A      N/A
Cart.handleClear               handler   Cart                                   50    1     N/A      N/A
Clicker                        component Clicker                                28    1     N/A      N/A
Clicker.onClick                handler   Clicker                                32    1     N/A      N/A
Clicker.onSelect               handler   Clicker                                33    1     N/A      N/A
Label                          component Clicker                                11    1     N/A      N/A
Toggle.flip                    handler   Clicker                                22    1     N/A      N/A
Dashboard                      component Dashboard                              14    1     N/A      N/A
Layout                         component Dashboard                              20    1     N/A      N/A
IconButton                     component IconButton                              9    1     N/A      N/A
TodoList.TodoRow.onClick       handler   TodoList                               14    1     N/A      N/A
TodoList.onChange              handler   TodoList                               23    1     N/A      N/A
TodoList.onClick               handler   TodoList                               32    1     N/A      N/A
Toolbar                        component Toolbar                                12    1     N/A      N/A
Toolbar.onClick                handler   Toolbar                                16    1     N/A      N/A
ToolbarGroup                   component Toolbar                                25    1     N/A      N/A
useCounter                     hook      useCounter                              3    1     N/A      N/A
useCounter.reset               handler   useCounter                             11    1     N/A      N/A

Components and hooks

Component                      Namespace                           Render Logic Props Bool Opt On Rest Pass State Eff Deps SetFx Depth Elems InFn InObj Nest
------------------------------------------------------------------------------------------------------------------------------------------------------------
SettingsPanel                  SettingsPanel                            6     1     9    5   5  2    -    0     0   0    0     0     2     5    0     0    0
Cart                           Cart                                     5     1     4    0   1  1    -    0     4   2    2     1     3     9    0     0    0
TodoList                       TodoList                                 2     2     2    0   0  1    -    0     1   0    0     0     3     7    2     2    1
Toolbar                        Toolbar                                  2     1     8    2   3  2  yes    0     0   0    0     0     2     2    1     0    0
Dashboard                      Dashboard                                1     1     5    0   0  1    -    5     0   0    0     0     1     1    0     0    0
Layout                         Dashboard                                1     1     5    0   0  1    -    4     0   0    0     0     2     2    0     0    0
Badge                          Badge                                    2     1     2    0   0  0    -    0     0   0    0     0     1     1    0     0    0
Toggle                         Clicker                                  2     1     0    0   0  0    -    0     1   0    0     0     1     1    0     0    0
Sidebar                        Dashboard                                2     1     4    0   0  1    -    0     0   0    0     0     2     4    0     0    0
TodoList.TodoRow               TodoList                                 2     1     1    0   0  0    -    0     0   0    0     0     1     1    1     1    0
Clicker                        Clicker                                  1     1     0    0   0  0    -    0     1   0    0     0     2     5    2     1    0
Label                          Clicker                                  1     1     3    0   0  1    -    0     0   0    0     0     1     1    0     0    0
IconButton                     IconButton                               1     1     3    1   1  0  yes    0     0   0    0     0     1     1    0     0    0
ToolbarGroup                   Toolbar                                  1     1    2+    0   0  0  yes    0     0   0    0     0     2     2    0     0    0
useCounter                     useCounter                               1     1     -    -   -  -    -    -     2   1    2     0     0     0    0     0    0

Threshold violations

Name                           Namespace                           Exceeded
---------------------------------------------------------------------------
Cart#useEffect#1               Cart                                complexity 10 >= 10.0 (no coverage)
SettingsPanel                  SettingsPanel                       props 9 > 7
SettingsPanel                  SettingsPanel                       boolean-props 5 > 2
Cart                           Cart                                setter-effects 1 > 0
TodoList                       TodoList                            nested-components 1 > 0
Toolbar                        Toolbar                             props 8 > 7
Dashboard                      Dashboard                           pass-through 5 > 3
Layout                         Dashboard                           pass-through 4 > 3
`

func TestReactOnTheFixtureWithoutCoverage(t *testing.T) {
	root := copyFixture(t, "react", "node_modules")
	code, stdout, stderr := runCapture([]string{"react", "--no-coverage", "--root", root}, "v")
	if code != 3 {
		t.Fatalf("exit %d, want 3:\n%s\n%s", code, stdout, stderr)
	}
	if stdout != reactNoCoverageReport {
		t.Fatalf("stdout:\n%s\nwant:\n%s", stdout, reactNoCoverageReport)
	}
	if !strings.HasSuffix(stderr, "React thresholds exceeded: 8 violation(s)\n") {
		t.Fatalf("stderr = %q", stderr)
	}
	edn, err := os.ReadFile(filepath.Join(root, ".metrics", "react.edn"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`  {:name "Cart#useEffect#1", :namespace "Cart", :kind :callback, :unit "Cart", :path "src/Cart.tsx", :line 22, :complexity 10, :coverage nil, :crap nil}`,
		`   :props {:count 5, :boolean 0, :optional 0, :callback 1, :rest false, :pass-through ["locale" "notifications" "onLogout" "user"], :open false},`,
		`  {:name "TodoList", :namespace "TodoList", :path "src/TodoList.tsx", :line 9, :metric :nested-components, :value 1.0, :limit 0.0}`,
		// Toolbar: Omit<ButtonBase, "variant"> & {...}, ButtonBase imported through src/types/index.ts
		`   :props {:count 8, :boolean 2, :optional 3, :callback 2, :rest true, :pass-through [], :open false},`,
		// ToolbarGroup: & HTMLAttributes<HTMLDivElement> from react stays open
		`   :props {:count 2, :boolean 0, :optional 0, :callback 0, :rest true, :pass-through [], :open true},`,
	} {
		if !strings.Contains(string(edn), "\n"+want+"\n") {
			t.Errorf("react.edn lacks %s", want)
		}
	}
	// Raising the limits it trips (and the CRAP gate) makes the same run clean.
	code, _, stderr = runCapture([]string{"react", "--no-coverage", "--root", root, "--crap", "11",
		"--max-props", "9", "--max-bool-props", "off", "--max-setter-effects", "1", "--max-nested", "1",
		"--max-pass-through", "5"}, "v")
	if code != 0 || strings.Contains(stderr, "exceeded") {
		t.Fatalf("relaxed limits: exit %d\n%s", code, stderr)
	}
}

func TestReactExits2WhenTheCoverageRunFails(t *testing.T) {
	root := copyFixture(t, "react", "node_modules")
	code, stdout, stderr := runCapture([]string{"react", "--root", root, "--coverage-command", "exit 1"}, "v")
	if code != 2 || stdout != "" || !strings.HasSuffix(stderr, "react: the coverage run failed; fix the failing tests or use --no-coverage\n") {
		t.Fatalf("exit %d\nstdout=%q\nstderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".metrics", "react.edn")); !os.IsNotExist(err) {
		t.Fatalf("no metrics on a failed baseline: %v", err)
	}
}

func TestReactOnTheFixtureWithCoverage(t *testing.T) {
	if testing.Short() {
		t.Skip("runs vitest")
	}
	if _, err := os.Stat(filepath.Join("..", "..", "fixtures", "react", "node_modules")); err != nil {
		t.Skip("fixtures/react/node_modules missing: run fixtures/setup-tests.sh")
	}
	pinAsdfNode(t)
	root := copyFixture(t, "react", "node_modules")
	code, stdout, stderr := runCapture([]string{"react", "--root", root}, "v")
	if code != 3 {
		t.Fatalf("exit %d, want 3:\n%s\n%s", code, stdout, stderr)
	}
	for _, want := range []string{
		"Cart#useEffect#1               callback  Cart                                   22   10   44.4%     27.1",
		"SettingsPanel                  component SettingsPanel                          13    6   20.0%     24.4",
		"Badge                          component Badge                                   6    2  100.0%      2.0",
		"IconButton                     component IconButton                              9    1    0.0%      2.0",
		"Cart#useEffect#1               Cart                                crap 27.1 >= 10.0",
		"SettingsPanel                  SettingsPanel                       crap 24.4 >= 10.0",
	} {
		if !strings.Contains(stdout, "\n"+want+"\n") {
			t.Errorf("report lacks %q:\n%s", want, stdout)
		}
	}
	if !strings.HasSuffix(stderr, "React thresholds exceeded: 9 violation(s)\n") {
		t.Fatalf("stderr tail = %q", stderr[max(0, len(stderr)-200):])
	}
}

// reactProfileRows are the exact runtime counts of the fixture's Vitest run
// (Component Namespace Renders Mounts Updates Upd/Mount | Inline Flag); the
// TotalMs and MaxMs columns are timings and only checked to be numbers.
var reactProfileRows = [][2]string{
	{"Clicker Clicker 4 1 3 3.0", "3 re-renders with inline props"},
	{"Label Clicker 4 1 3 3.0", "0"},
	{"Cart Cart 4 2 2 1.0", "0"},
	{"TodoList TodoList 2 1 1 1.0", "4"},
	{"Toggle Clicker 2 1 1 1.0", "0"},
	{"TodoList.TodoRow TodoList 3 3 0 0.0", "2"},
	{"Badge Badge 2 2 0 0.0", "0"},
	{"Dashboard Dashboard 1 1 0 0.0", "0"},
	{"Layout Dashboard 1 1 0 0.0", "0"},
	{"SettingsPanel SettingsPanel 1 1 0 0.0", "0"},
	{"Sidebar Dashboard 1 1 0 0.0", "0"},
}

func TestReactProfileOnTheFixture(t *testing.T) {
	if testing.Short() {
		t.Skip("runs vitest")
	}
	if _, err := os.Stat(filepath.Join("..", "..", "fixtures", "react", "node_modules")); err != nil {
		t.Skip("fixtures/react/node_modules missing: run fixtures/setup-tests.sh")
	}
	pinAsdfNode(t)
	root := copyFixture(t, "react", "node_modules")
	code, stdout, stderr := runCapture([]string{"react", "--no-coverage", "--profile", "--root", root}, "v")
	if code != 3 {
		t.Fatalf("exit %d, want 3:\n%s\n%s", code, stdout, stderr)
	}
	static, runtime, ok := strings.Cut(stdout, "\nRuntime renders (vitest test run; durations are React actualDuration in ms, noisy)\n\n")
	if !ok || static != reactNoCoverageReport {
		t.Fatalf("the static report changed under --profile:\n%s", stdout)
	}
	lines := strings.Split(runtime, "\n")
	if len(lines) != 2+len(reactProfileRows)+3 || !strings.HasPrefix(lines[0], "Component ") ||
		lines[len(lines)-2] != "Not rendered by the tests: Toolbar, IconButton, ToolbarGroup" || lines[len(lines)-1] != "" {
		t.Fatalf("runtime section:\n%s", runtime)
	}
	for i, want := range reactProfileRows {
		f := strings.Fields(lines[2+i])
		if len(f) < 9 {
			t.Fatalf("row %d: %q", i, lines[2+i])
		}
		for _, ms := range f[6:8] {
			if _, err := strconv.ParseFloat(ms, 64); err != nil {
				t.Errorf("row %d: duration %q is not a number", i, ms)
			}
		}
		if got := [2]string{strings.Join(f[:6], " "), strings.Join(f[8:], " ")}; got != want {
			t.Errorf("row %d = %q, want %q", i, got, want)
		}
	}
	if !strings.HasSuffix(stderr, "React thresholds exceeded: 8 violation(s)\n") {
		t.Fatalf("stderr tail = %q", stderr[max(0, len(stderr)-200):])
	}
	edn, err := os.ReadFile(filepath.Join(root, ".metrics", "react.edn"))
	if err != nil {
		t.Fatal(err)
	}
	ms := regexp.MustCompile(`:total-ms [0-9.]+, :max-ms [0-9.]+`)
	doc := ms.ReplaceAllString(string(edn), ":total-ms T, :max-ms M")
	for _, want := range []string{
		"\n :renders {:runners [\"vitest\"], :not-rendered [\"Toolbar\" \"IconButton\" \"ToolbarGroup\"],\n  :components [\n",
		"\n  {:name \"Clicker\", :namespace \"Clicker\", :path \"src/Clicker.tsx\", :renders 4, :mounts 1, :updates 3, :total-ms T, :max-ms M, :inline-props 3, :ambiguous 0, :flagged true}\n",
		"\n  {:name \"Toggle\", :namespace \"Clicker\", :path \"src/Clicker.tsx\", :renders 2, :mounts 1, :updates 1, :total-ms T, :max-ms M, :inline-props 0, :ambiguous 0, :flagged false}\n",
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("react.edn lacks %q:\n%s", want, doc)
		}
	}
	if !strings.HasSuffix(doc, "]}}\n") || strings.Count(doc, ":renders {") != 1 {
		t.Errorf("react.edn tail:\n%s", doc)
	}
	if setup, err := os.ReadFile(filepath.Join(root, ".metrics", "react-profile", "vitest.config.mjs")); err != nil ||
		!strings.Contains(string(setup), "import base from "+strconv.Quote(filepath.Join(root, "vitest.config.ts"))) {
		t.Errorf("generated config: %v\n%s", err, setup)
	}
	// A failing test makes the profiled run fail: exit 2, nothing written.
	if err := os.Remove(filepath.Join(root, ".metrics", "react.edn")); err != nil {
		t.Fatal(err)
	}
	failing := "import { expect, test } from \"vitest\";\ntest(\"fails\", () => expect(1).toBe(2));\n"
	if err := os.WriteFile(filepath.Join(root, "tests", "Fails.test.ts"), []byte(failing), 0o666); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = runCapture([]string{"react", "--no-coverage", "--profile", "--root", root}, "v")
	if code != 2 || stdout != "" || !strings.HasSuffix(stderr, "\nreact: the profiled test run failed; fix the failing tests or drop --profile\n") {
		t.Fatalf("failing run: exit %d\nstdout=%q\nstderr tail=%q", code, stdout, stderr[max(0, len(stderr)-200):])
	}
	if _, err := os.Stat(filepath.Join(root, ".metrics", "react.edn")); !os.IsNotExist(err) {
		t.Fatalf("no metrics after a failing profiled run: %v", err)
	}
}

func TestReactProfileNeedsVitestOrJest(t *testing.T) {
	root := t.TempDir()
	for name, text := range map[string]string{
		"package.json": `{"scripts":{"test":"mocha"}}`,
		"src/A.tsx":    "export function A() { return <div />; }\n",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0o666); err != nil {
			t.Fatal(err)
		}
	}
	code, stdout, stderr := runCapture([]string{"react", "--no-coverage", "--profile", "--root", root}, "v")
	if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "react: --profile: no Vitest or Jest test setup in ") ||
		strings.Count(stderr, "\n") != 1 {
		t.Fatalf("exit %d\nstdout=%q\nstderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".metrics", "react.edn")); !os.IsNotExist(err) {
		t.Fatalf("no metrics when the profile cannot run: %v", err)
	}
}
