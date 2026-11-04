package main

// End to end: `coderefinery react` on fixtures/react (scratch copy), through
// the dispatcher: complexity-only (always) and with Vitest coverage (needs
// fixtures/react/node_modules: run fixtures/setup-tests.sh).

import (
	"os"
	"path/filepath"
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
Sidebar                        component Dashboard                              28    2     N/A      N/A
TodoList                       component TodoList                                9    2     N/A      N/A
TodoList.TodoRow               component TodoList                               12    2     N/A      N/A
useCounter#useCallback#1       callback  useCounter                              7    2     N/A      N/A
useCounter#useEffect#1         callback  useCounter                             15    2     N/A      N/A
useCounter#useMemo#1           callback  useCounter                             13    2     N/A      N/A
Cart#useEffect#2               callback  Cart                                   42    1     N/A      N/A
Cart.handleClear               handler   Cart                                   50    1     N/A      N/A
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
Sidebar                        Dashboard                                2     1     4    0   0  1    -    0     0   0    0     0     2     4    0     0    0
TodoList.TodoRow               TodoList                                 2     1     1    0   0  0    -    0     0   0    0     0     1     1    1     1    0
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
