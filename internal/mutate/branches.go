package mutate

// mutate --branches: why an UNCOVERED site is uncovered
// (docs/design-tests.md §6). Owned by Track B.

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/matheusAle/coderefinery/internal/branch"
	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/crap"
	"github.com/matheusAle/coderefinery/internal/discover"
)

// branchBundles caches coverage.LoadBranches per root for one process.
var branchBundles sync.Map // root -> *coverage.BranchBundle (nil on error)

func branchBundle(root string) *coverage.BranchBundle {
	if b, ok := branchBundles.Load(root); ok {
		return b.(*coverage.BranchBundle)
	}
	b, err := coverage.LoadBranches(root)
	if err != nil {
		b = nil
	}
	branchBundles.Store(root, b)
	return b
}

// branchAnnotator is the --branches site-log suffix for UNCOVERED sites;
// nil = none. lines is the file's covered-line set (nil = no coverage data).
// A site inside the code of an untested arm (branch) or of a never-run
// block/region names that arm; any other uncovered line never ran.
func branchAnnotator(root string, result RunResult, lines lineSet) func(Site, string) string {
	var findings []branch.Finding
	loaded := false
	load := func() {
		loaded = true
		root := discover.Resolve(root)
		bundle := branchBundle(root)
		if bundle == nil {
			return
		}
		file := result.Path
		if !filepath.IsAbs(file) {
			file = filepath.Join(root, file)
		}
		findings, _ = crap.BranchFindings(bundle, discover.Resolve(file), root, crap.SourceParts([]string{result.Path}))
	}
	return func(s Site, status string) string {
		if status != "uncovered" {
			return ""
		}
		if lines == nil {
			return " · no coverage data for file"
		}
		if !loaded {
			load()
		}
		if f, ok := enclosingArm(findings, s.Line); ok {
			return fmt.Sprintf(" · in the %s arm of :%d (%s), never taken", f.Arm, f.Line, f.Decision)
		}
		return " · line never executed"
	}
}

// enclosingArm is the finding with the smallest line span holding line.
func enclosingArm(findings []branch.Finding, line int) (branch.Finding, bool) {
	var best branch.Finding
	found := false
	for _, f := range findings {
		if f.Arm == "" || f.SpanStart == 0 || line < f.SpanStart || line > f.SpanEnd {
			continue
		}
		if !found || f.SpanEnd-f.SpanStart < best.SpanEnd-best.SpanStart {
			best, found = f, true
		}
	}
	return best, found
}
