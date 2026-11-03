package main

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Normaliser masks what legitimately differs between two runs of the same
// tool on the same fixture. Every rule here is documented in harness/README.md.
type Normaliser struct {
	UBMode string
	Timing bool // mask durations and wall-clock times printed by test runners
}

type rule struct {
	re   *regexp.Regexp
	repl string
}

var timingRules = []rule{
	// vitest summary
	{regexp.MustCompile(`(?m)^(\s*Start at\s+)\d{2}:\d{2}:\d{2}`), "${1}<TIME>"},
	{regexp.MustCompile(`(?m)^(\s*Duration\s+).*$`), "${1}<DUR>"},
	// node --test spec reporter
	{regexp.MustCompile(`duration_ms [0-9.]+`), "duration_ms <DUR>"},
	// go test: "ok  \tpkg\t0.865s" or "(cached)"
	{regexp.MustCompile(`(?m)^(ok\s+\S+\s+)(\d+(\.\d+)?s|\(cached\))`), "${1}<DUR>"},
	// pytest: "9 passed in 0.02s" and "(0:01:02)"
	{regexp.MustCompile(` in \d+(\.\d+)?s\b`), " in <DUR>"},
	{regexp.MustCompile(`\(\d+:\d{2}:\d{2}\)`), "(<DUR>)"},
	// generic durations: 12ms, 0.37675ms, 1.23s
	{regexp.MustCompile(`\b\d+(\.\d+)?ms\b`), "<DUR>"},
	{regexp.MustCompile(`\b\d+\.\d+s\b`), "<DUR>"},
}

// vitest runs test files in parallel and prints each file's result line in
// completion order, so a block of those lines is sorted before comparing.
var vitestFileLine = regexp.MustCompile(`^\s*[✓×❯↓] \S+\.(test|spec)\.\S+ \(`)

func sortVitestFileLines(s string) string {
	lines := strings.Split(s, "\n")
	for i := 0; i < len(lines); {
		j := i
		for j < len(lines) && vitestFileLine.MatchString(lines[j]) {
			j++
		}
		if j > i {
			slices.Sort(lines[i:j])
			i = j
		} else {
			i++
		}
	}
	return strings.Join(lines, "\n")
}

var testedAt = regexp.MustCompile(`:tested-at "[^"]*"`)

// Text normalises stdout/stderr of a run whose fixture copy lived at root.
func (n Normaliser) Text(s, root string) string {
	s = maskRoot(s, root)
	if n.UBMode != "" {
		s = strings.ReplaceAll(s, n.UBMode, "<UBMODE>")
	}
	if n.Timing {
		for _, r := range timingRules {
			s = r.re.ReplaceAllString(s, r.repl)
		}
		s = sortVitestFileLines(s)
	}
	return s
}

// Raw masks only mutator's :tested-at timestamp, the one field of a
// .metrics file that can never repeat. "Raw equal" is judged after this.
func (n Normaliser) Raw(b []byte) []byte {
	return testedAt.ReplaceAll(b, []byte(`:tested-at "<TIME>"`))
}

func maskRoot(s, root string) string {
	if root == "" {
		return s
	}
	variants := []string{root}
	if resolved, err := filepath.EvalSymlinks(root); err == nil && resolved != root {
		variants = append(variants, resolved)
	}
	if !strings.HasPrefix(root, "/private/") {
		variants = append(variants, "/private"+root)
	}
	for _, v := range variants {
		s = strings.ReplaceAll(s, v, "<ROOT>")
	}
	return s
}
