package crap

import (
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/matheusAle/ub-mode/ubgate/internal/model"
	"github.com/matheusAle/ub-mode/ubgate/internal/pyfmt"
)

func itoa(n int) string { return strconv.Itoa(n) }

func padRight(s string, w int) string {
	if n := w - utf8.RuneCountInString(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func padLeft(s string, w int) string {
	if n := w - utf8.RuneCountInString(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}

func formatCoverage(c *float64) string {
	if c == nil {
		return "  N/A "
	}
	return pyfmt.FixedWidth(*c, 5, 1) + "%"
}

func formatCrap(c *float64) string {
	if c == nil {
		return "     N/A"
	}
	return pyfmt.FixedWidth(*c, 8, 1)
}

// FormatReport is report.format_report.
func FormatReport(entries []model.Entry) string {
	nameWidth, nsWidth := 30, 35
	if len(entries) > 0 {
		maxName, maxNs := 0, 0
		for _, e := range entries {
			maxName = max(maxName, utf8.RuneCountInString(e.Name))
			maxNs = max(maxNs, utf8.RuneCountInString(e.Namespace))
		}
		nameWidth = max(nameWidth, min(48, maxName))
		nsWidth = max(nsWidth, min(72, maxNs))
	}
	header := padRight("Function", nameWidth) + " " + padRight("Namespace", nsWidth) + " " +
		padLeft("CC", 4) + " " + padLeft("Cov%", 7) + " " + padLeft("CRAP", 8)
	lines := []string{"CRAP Report", "==========="}
	langSet := map[string]bool{}
	for _, e := range entries {
		langSet[e.Language] = true
	}
	if len(langSet) > 0 {
		langs := make([]string, 0, len(langSet))
		for l := range langSet {
			langs = append(langs, l)
		}
		sort.Strings(langs)
		lines = append(lines, "Languages: "+strings.Join(langs, ", "), "")
	}
	lines = append(lines, header, strings.Repeat("-", utf8.RuneCountInString(header)))
	for _, e := range entries {
		lines = append(lines, padRight(e.Name, nameWidth)+" "+padRight(e.Namespace, nsWidth)+" "+
			padLeft(itoa(e.Complexity), 4)+" "+formatCoverage(e.Coverage)+" "+formatCrap(e.Crap))
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}
