// Package textdiff renders a small unified line diff without dependencies.
package textdiff

import (
	"fmt"
	"strings"
)

type op struct {
	kind byte // ' ', '-', '+'
	text string
}

// Unified returns a unified-style diff of want → got with `context` lines
// around each change, truncated to maxLines output lines (0 = unlimited).
// It returns "" when the inputs are equal.
func Unified(wantName, gotName, want, got string, context, maxLines int) string {
	if want == got {
		return ""
	}
	a := splitLines(want)
	b := splitLines(got)
	ops := diffOps(a, b)
	var out []string
	out = append(out, "--- "+wantName, "+++ "+gotName)
	for _, h := range hunks(ops, context) {
		out = append(out, h...)
	}
	if maxLines > 0 && len(out) > maxLines {
		extra := len(out) - maxLines
		out = append(out[:maxLines], fmt.Sprintf("... (%d more diff lines)", extra))
	}
	return strings.Join(out, "\n") + "\n"
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for i, l := range lines {
		if strings.HasSuffix(l, "\n") {
			lines[i] = strings.TrimSuffix(l, "\n")
		} else {
			lines[i] = l + "  \\ (no newline at end)"
		}
	}
	return lines
}

func diffOps(a, b []string) []op {
	// Trim the common prefix and suffix so the quadratic table stays small.
	pre := 0
	for pre < len(a) && pre < len(b) && a[pre] == b[pre] {
		pre++
	}
	suf := 0
	for suf < len(a)-pre && suf < len(b)-pre && a[len(a)-1-suf] == b[len(b)-1-suf] {
		suf++
	}
	var ops []op
	for _, l := range a[:pre] {
		ops = append(ops, op{' ', l})
	}
	ops = append(ops, middle(a[pre:len(a)-suf], b[pre:len(b)-suf])...)
	for _, l := range a[len(a)-suf:] {
		ops = append(ops, op{' ', l})
	}
	return ops
}

func middle(a, b []string) []op {
	n, m := len(a), len(b)
	if n*m > 4_000_000 {
		var ops []op
		for _, l := range a {
			ops = append(ops, op{'-', l})
		}
		for _, l := range b {
			ops = append(ops, op{'+', l})
		}
		return ops
	}
	lcs := make([][]int32, n+1)
	for i := range lcs {
		lcs[i] = make([]int32, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var ops []op
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, op{' ', a[i]})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, op{'-', a[i]})
			i++
		default:
			ops = append(ops, op{'+', b[j]})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, op{'-', a[i]})
	}
	for ; j < m; j++ {
		ops = append(ops, op{'+', b[j]})
	}
	return ops
}

func hunks(ops []op, context int) [][]string {
	var result [][]string
	i := 0
	for i < len(ops) {
		for i < len(ops) && ops[i].kind == ' ' {
			i++
		}
		if i >= len(ops) {
			break
		}
		start := max(0, i-context)
		end := i
		for end < len(ops) {
			if ops[end].kind != ' ' {
				end++
				continue
			}
			run := end
			for run < len(ops) && ops[run].kind == ' ' {
				run++
			}
			if run >= len(ops) || run-end > 2*context {
				end = min(len(ops), end+context)
				break
			}
			end = run
		}
		aLine, bLine := 1, 1
		for _, o := range ops[:start] {
			if o.kind != '+' {
				aLine++
			}
			if o.kind != '-' {
				bLine++
			}
		}
		var aCount, bCount int
		var body []string
		for _, o := range ops[start:end] {
			if o.kind != '+' {
				aCount++
			}
			if o.kind != '-' {
				bCount++
			}
			body = append(body, string(o.kind)+o.text)
		}
		header := fmt.Sprintf("@@ -%d,%d +%d,%d @@", aLine, aCount, bLine, bCount)
		result = append(result, append([]string{header}, body...))
		i = end
	}
	return result
}
