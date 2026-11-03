package dry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/matheusAle/ub-mode/ubgate/internal/edn"
)

// Reports: upstream report.py (spec-dryer §6).

// FormatText is report.format_text (the dry4clj layout).
func FormatText(candidates []Duplicate) string {
	if len(candidates) == 0 {
		return "No duplicate candidates found.\n"
	}
	blocks := make([]string, len(candidates))
	for i, c := range candidates {
		blocks[i] = fmt.Sprintf("DUPLICATE score=%.2f\n  %s:%d-%d\n  %s:%d-%d", c.Score,
			c.Left.File, c.Left.StartLine, c.Left.EndLine, c.Right.File, c.Right.StartLine, c.Right.EndLine)
	}
	return strings.Join(blocks, "\n\n") + "\n"
}

// ednFloat is report._edn_float: f"{v:.12f}" without trailing zeros and
// dot, ".0" re-added when no dot is left.
func ednFloat(v float64) string {
	t := strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.12f", v), "0"), ".")
	if !strings.Contains(t, ".") {
		t += ".0"
	}
	return t
}

func spanEDN(s Span) string {
	return fmt.Sprintf("{:file %s, :start-line %d, :end-line %d}", edn.String(s.File), s.StartLine, s.EndLine)
}

// RenderEDN is report.render_edn.
func RenderEDN(candidates []Duplicate) string {
	if len(candidates) == 0 {
		return "{:candidates []}\n"
	}
	rows := make([]string, len(candidates))
	for i, c := range candidates {
		rows[i] = strings.Join([]string{
			" {:score " + ednFloat(c.Score),
			"  :language " + edn.String(c.Language),
			"  :left " + spanEDN(c.Left),
			"  :right " + spanEDN(c.Right),
			fmt.Sprintf("  :left-nodes %d", c.LeftNodes),
			fmt.Sprintf("  :right-nodes %d}", c.RightNodes),
		}, "\n")
	}
	return "{:candidates [\n" + strings.Join(rows, "\n") + "\n]}\n"
}

// writeMetrics is report.write_metrics: <root>/.metrics/dry.edn, replaced
// on every analysis.
func writeMetrics(candidates []Duplicate, root string) (string, error) {
	path := filepath.Join(root, ".metrics", "dry.edn")
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(RenderEDN(candidates)), 0o666)
}
