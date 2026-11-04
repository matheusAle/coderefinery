package crap

// crap --branches (docs/design-tests.md §6, B3): after the CRAP report, the
// untested arms of executed decisions in the selected files, also written to
// .metrics/branches.edn. The CRAP report, crap.edn and the exit code do not
// change.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/matheusAle/coderefinery/internal/branch"
	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/source"
)

// SourceParts is coverage.Bundle.BindSources' view of the selected files,
// for report-key matching.
func SourceParts(paths []string) [][]string {
	b := coverage.NewBundle()
	b.BindSources(paths)
	return b.SourceParts
}

// BranchFindings maps one file's branch data onto its decisions. file is
// absolute; the findings name it relative to root. A file the parser
// rejects has no findings (DEVIATIONS A2).
func BranchFindings(bundle *coverage.BranchBundle, file, root string, sourceParts [][]string) ([]branch.Finding, error) {
	language := discover.LanguageOf(file)
	rel := sourcePath(file, root)
	data := bundle.File(rel, language, sourceParts)
	if data == nil {
		return nil, nil
	}
	text, err := source.Read(file)
	if err != nil {
		return nil, err
	}
	decs, err := Decisions(text, rel, root)
	if err != nil {
		return nil, nil
	}
	return branch.Map(rel, decs, data, text), nil
}

func branchesReport(root string, files []string, stdout, stderr io.Writer) error {
	bundle, err := coverage.LoadBranches(root)
	if err != nil {
		return err
	}
	rels := make([]string, len(files))
	for i, f := range files {
		rels[i] = sourcePath(f, root)
	}
	parts := SourceParts(rels)
	granularity, missing := map[string]string{}, map[string]string{}
	var findings []branch.Finding
	for _, f := range files {
		language := discover.LanguageOf(f)
		if language == "" {
			continue
		}
		if g, ok := bundle.Granularity[language]; ok {
			granularity[language] = g
		} else {
			missing[language] = bundle.Missing[language]
		}
		got, err := BranchFindings(bundle, f, root, parts)
		if err != nil {
			return err
		}
		findings = append(findings, got...)
	}
	branch.Sort(findings)
	metrics := filepath.Join(root, ".metrics", "branches.edn")
	if err := os.WriteFile(metrics, []byte(branch.RenderEDN(findings)), 0o666); err != nil {
		return err
	}
	fmt.Fprint(stdout, branch.Section(granularity, missing, findings))
	fmt.Fprintf(stderr, "Wrote %s\n", metrics)
	return nil
}
