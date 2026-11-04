package branch

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/matheusAle/coderefinery/internal/edn"
)

// Section is the text printed after the CRAP report (leading blank line
// included). granularity and missing are keyed by the selected files'
// languages.
func Section(granularity, missing map[string]string, findings []Finding) string {
	var langs []string
	for l, g := range granularity {
		langs = append(langs, l+" "+g)
	}
	sort.Strings(langs)
	header := "Untested branches"
	if len(langs) > 0 {
		header += " (" + strings.Join(langs, ", ") + ")"
	}
	lines := []string{"", header}
	for _, f := range findings {
		lines = append(lines, f.Path+":"+strconv.Itoa(f.Line)+" "+f.Decision+" · "+f.Text)
	}
	if len(findings) == 0 && len(langs) > 0 {
		lines = append(lines, "None in executed code.")
	}
	var gone []string
	for l := range missing {
		gone = append(gone, l)
	}
	sort.Strings(gone)
	for _, l := range gone {
		lines = append(lines, "No branch data: "+l+" ("+missing[l]+")")
	}
	return strings.Join(lines, "\n") + "\n"
}

// GeneratedAt is datetime.now().astimezone().isoformat(timespec="seconds"),
// like mutate's :tested-at. Swappable for tests.
var GeneratedAt = func() string { return time.Now().Format("2006-01-02T15:04:05-07:00") }

// RenderEDN is .metrics/branches.edn.
func RenderEDN(findings []Finding) string {
	head := "{:version 1 :generated-at " + edn.String(GeneratedAt()) + " :branches ["
	if len(findings) == 0 {
		return head + "]}\n"
	}
	rows := make([]string, len(findings))
	for i, f := range findings {
		rows[i] = "  {:file " + edn.String(f.Path) + " :line " + strconv.Itoa(f.Line) +
			" :decision " + edn.String(f.Decision) + " :arm " + edn.String(f.Arm) +
			" :granularity " + edn.String(f.Granularity) + " :text " + edn.String(f.Text) + "}"
	}
	return head + "\n" + strings.Join(rows, "\n") + "\n]}\n"
}
