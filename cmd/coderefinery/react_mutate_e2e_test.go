package main

// End to end: `mutate --react` on fixtures/react (scratch copy) with the
// fixture's Testing Library test for the dashboard. Needs
// fixtures/react/node_modules (fixtures/setup-tests.sh); skipped under -short.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const dashboardTest = "tests/Dashboard.test.tsx"

// reactMutateLog is the site log of lines 23 and 31-33 of Dashboard.tsx:
// Sidebar's props, its avatar, the notification badge and the button.
var reactMutateLog = []string{
	"KILLED    src/Dashboard.tsx:23 drop prop user from <Sidebar/>",
	"SURVIVED  src/Dashboard.tsx:23 drop prop locale from <Sidebar/>",
	"KILLED    src/Dashboard.tsx:23 drop prop notifications from <Sidebar/>",
	"KILLED    src/Dashboard.tsx:23 noop onLogout",
	"SURVIVED  src/Dashboard.tsx:31 drop prop src from <img/>",
	"SURVIVED  src/Dashboard.tsx:31 drop prop alt from <img/>",
	"KILLED    src/Dashboard.tsx:32 drop render <span>",
	"SURVIVED  src/Dashboard.tsx:32 always render <span>",
	"SURVIVED  src/Dashboard.tsx:32 > -> >=",
	"SURVIVED  src/Dashboard.tsx:32 0 -> 1",
	"KILLED    src/Dashboard.tsx:32 && -> ||",
	"KILLED    src/Dashboard.tsx:33 noop onClick",
}

func TestMutateReactOnTheFixtureKillsBehaviourAndShowsWeakTests(t *testing.T) {
	if testing.Short() {
		t.Skip("runs vitest")
	}
	if _, err := os.Stat(filepath.Join("..", "..", "fixtures", "react", "node_modules")); err != nil {
		t.Skip("fixtures/react/node_modules missing: run fixtures/setup-tests.sh")
	}
	pinAsdfNode(t)
	root := copyFixture(t, "react", "node_modules")
	args := []string{"mutate", "--react", "--attribute", "--no-coverage", "--max-workers", "4",
		"--lines", "23,31,32,33", "--root", root, "src/Dashboard.tsx"}
	code, stdout, stderr := runCapture(args, "v")
	if code != 3 {
		t.Fatalf("exit %d, want 3 (survivors):\n%s\n%s", code, stdout, stderr)
	}
	var log []string
	for _, line := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(line, "KILLED") || strings.HasPrefix(line, "SURVIVED") || strings.HasPrefix(line, "UNCOVERED") {
			// --attribute names the killer; it is checked below.
			head, by, _ := strings.Cut(line, " · by ")
			if strings.HasPrefix(line, "KILLED") && !strings.HasPrefix(by, dashboardTest) {
				t.Errorf("kill not credited to the dashboard test: %q", line)
			}
			log = append(log, head)
		}
	}
	if strings.Join(log, "\n") != strings.Join(reactMutateLog, "\n") {
		t.Fatalf("site log:\n%s\nwant:\n%s\nstderr:\n%s", strings.Join(log, "\n"), strings.Join(reactMutateLog, "\n"), stderr)
	}
	// Line 23's sites sit in Layout's JSX (<Sidebar ... />), so they are
	// Layout's; lines 31-33 are Sidebar's.
	table := "Function  Namespace  Killed  Survived  Uncovered  Sites   Score\n" +
		"---------------------------------------------------------------\n" +
		"Dashboard Dashboard       0         0          0      5    n/a\n" +
		"Sidebar   Dashboard       3         5          0      9  37.5%\n" +
		"Layout    Dashboard       3         1          0      4  75.0%\n" +
		"---------------------------------------------------------------\n" +
		"Total                     6         6          0     18   50.0%\n"
	if !strings.HasSuffix(stdout, "\n"+table) {
		t.Errorf("report:\n%s\nwant:\n%s", stdout, table)
	}

	// The React outcomes live in the usual snapshot under crapper's form id;
	// :killed-by credits the drop-render kill to the test.
	snap, err := os.ReadFile(filepath.Join(root, ".metrics", "mutate", "Dashboard.edn"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`{:id "defn/Sidebar" :kind "defn" :file "src/Dashboard.tsx" :line 28 :end-line 36 `,
		`"[\"src/Dashboard.tsx\",\"Dashboard\",\"defn/Sidebar\",894,902,\"onLogout\",\"() => {}\"]" :killed`,
		`"[\"src/Dashboard.tsx\",\"Dashboard\",\"defn/Sidebar\",794,809,\"alt={user.name}\",\"\"]" :survived`,
		`"[\"src/Dashboard.tsx\",\"Dashboard\",\"defn/Sidebar\",820,869,\"notifications > 0 && <span>{notifications}</span>\",\"false\"]" ["tests/Dashboard.test.tsx::logs out from the sidebar"]`,
	} {
		if !strings.Contains(string(snap), want) {
			t.Errorf("snapshot lacks %s:\n%s", want, snap)
		}
	}
}
