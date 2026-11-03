package mutate

// Text reports for a mutation run and for --scan (upstream mutator/report.py).

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/matheusAle/ub-mode/ubgate/internal/pyfmt"
)

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

func scoreText(f FormResult) string {
	score, ok := f.Score()
	if !ok {
		return "   n/a"
	}
	return pyfmt.FixedWidth(score*100, 5, 1) + "%"
}

// formatResults is report.format_results.
func formatResults(forms []FormResult) string {
	if len(forms) == 0 {
		return "No functions to mutate.\n"
	}
	longestName, longestNS := 0, 0
	for _, f := range forms {
		longestName = max(longestName, utf8.RuneCountInString(f.Name))
		longestNS = max(longestNS, utf8.RuneCountInString(f.Namespace))
	}
	nw := max(8, min(48, longestName))
	nsw := max(9, min(72, longestNS))
	header := fmt.Sprintf("%s %s %7s %9s %10s %6s %7s", padRight("Function", nw), padRight("Namespace", nsw),
		"Killed", "Survived", "Uncovered", "Sites", "Score")
	ordered := append([]FormResult(nil), forms...)
	sortStable(ordered, func(a, b FormResult) bool {
		sa, oka := a.Score()
		sb, okb := b.Score()
		if oka != okb {
			return !oka
		}
		if sa != sb {
			return sa < sb
		}
		if a.Survived != b.Survived {
			return a.Survived > b.Survived
		}
		return a.Name < b.Name
	})
	rule := strings.Repeat("-", utf8.RuneCountInString(header))
	lines := []string{header, rule}
	killed, survived, uncovered, sites := 0, 0, 0, 0
	for _, f := range ordered {
		lines = append(lines, fmt.Sprintf("%s %s %7d %9d %10d %6d %s", padRight(f.Name, nw), padRight(f.Namespace, nsw),
			f.Killed, f.Survived, f.Uncovered, f.Sites, scoreText(f)))
		killed += f.Killed
		survived += f.Survived
		uncovered += f.Uncovered
		sites += f.Sites
	}
	total := "n/a"
	if executed := killed + survived; executed > 0 {
		total = pyfmt.Fixed(float64(killed)/float64(executed)*100, 1) + "%"
	}
	lines = append(lines, rule, fmt.Sprintf("%s %s %7d %9d %10d %6d %s", padRight("Total", nw), padRight("", nsw),
		killed, survived, uncovered, sites, padLeft(total, 7)), "")
	return strings.Join(lines, "\n")
}

// formatSiteLog is report.format_site_log.
func formatSiteLog(sites []Site, status map[string]string) string {
	var lines []string
	for _, s := range sites {
		label, ok := status[s.ID]
		if !ok {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s %s:%d %s", padRight(strings.ToUpper(label), 9), s.File, s.Line, s.Description()))
	}
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// formatScan is report.format_scan. covered is nil when there is no
// coverage map.
func formatScan(path string, sites []Site, changed map[formKey]bool, covered map[string]bool) string {
	lines := []string{fmt.Sprintf("Scan: %d mutation sites in %s", len(sites), path)}
	for _, s := range sites {
		mark := " "
		if changed[formKey{s.Namespace, s.FormID}] {
			mark = "*"
		}
		cov := ""
		if covered != nil {
			if c, ok := covered[s.ID]; ok && !c {
				cov = " uncovered"
			}
		}
		lines = append(lines, fmt.Sprintf("%s %s:%d %s%s  [%s]", mark, path, s.Line, s.Description(), cov, s.FormID))
	}
	if len(changed) > 0 {
		lines = append(lines, "* marks a function whose text changed since the last snapshot.")
	}
	lines = append(lines, "")
	return strings.Join(lines, "\n")
}
