package crap

import (
	"sort"
	"strings"

	"github.com/matheusAle/ub-mode/ubgate/internal/edn"
	"github.com/matheusAle/ub-mode/ubgate/internal/model"
	"github.com/matheusAle/ub-mode/ubgate/internal/pyfmt"
)

// Score is crap.crap_score: CC² × (1 − cov)³ + CC, evaluated left to right.
// The explicit float64 conversion forbids FMA fusion (bit parity with
// CPython on arm64).
func Score(complexity int, coveragePct *float64) *float64 {
	if coveragePct == nil {
		return nil
	}
	cc := float64(complexity)
	u := 1.0 - (*coveragePct / 100.0)
	v := float64(cc*cc*u*u*u) + cc
	return &v
}

// RoundMetric is crap.round_metric: round(x, 4).
func RoundMetric(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := pyfmt.Round(*v, 4)
	return &r
}

// MakeEntry is crap.make_entry: CRAP uses the rounded coverage.
func MakeEntry(fn model.Function, coverage *float64) model.Entry {
	coverage = RoundMetric(coverage)
	return model.Entry{
		Name: fn.Name, Namespace: fn.Namespace, Complexity: fn.Complexity,
		Coverage: coverage, Crap: RoundMetric(Score(fn.Complexity, coverage)),
		Path: fn.Path, Language: fn.Language,
	}
}

// SortEntries is crap.sort_entries (stable): scored rows by -crap, then
// unscored rows by -complexity; ties by namespace, name, path.
func SortEntries(entries []model.Entry) []model.Entry {
	out := append([]model.Entry(nil), entries...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.Crap == nil) != (b.Crap == nil) {
			return a.Crap != nil
		}
		if a.Crap != nil {
			if *a.Crap != *b.Crap {
				return *a.Crap > *b.Crap
			}
		} else if a.Complexity != b.Complexity {
			return a.Complexity > b.Complexity
		}
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Path < b.Path
	})
	return out
}

// RenderEDN is metrics.render_edn.
func RenderEDN(entries []model.Entry) string {
	if len(entries) == 0 {
		return "{:entries []}\n"
	}
	rows := make([]string, len(entries))
	for i, e := range entries {
		rows[i] = "  {:name " + edn.String(e.Name) + ", :namespace " + edn.String(e.Namespace) +
			", :complexity " + itoa(e.Complexity) + ", :coverage " + edn.OptNumber4(e.Coverage) +
			", :crap " + edn.OptNumber4(e.Crap) + "}"
	}
	return "{:entries [\n" + strings.Join(rows, "\n") + "\n]}\n"
}
