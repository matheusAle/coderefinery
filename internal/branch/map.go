package branch

import (
	"sort"
	"strings"

	"github.com/matheusAle/coderefinery/internal/coverage"
)

// Map turns one file's branch data into findings. decs come from
// crap.Decisions for the same file; source is its decoded text (block
// lines are shown without the line that only opens them).
func Map(path string, decs []Decision, f *coverage.BranchFile, source string) []Finding {
	if f == nil || len(decs) == 0 {
		return nil
	}
	m := mapper{path: path, decs: decs, gran: f.Granularity, lines: strings.Split(source, "\n")}
	var arcs, positional, llvm, typed []coverage.ArmHit
	for _, h := range f.Arms {
		switch h.Kind {
		case coverage.KindArc:
			arcs = append(arcs, h)
		case coverage.KindBlock, coverage.KindRegion:
			positional = append(positional, h)
		case coverage.KindLLVM:
			llvm = append(llvm, h)
		case coverage.KindLineCount, coverage.KindForm:
			m.count(h)
		default:
			typed = append(typed, h)
		}
	}
	m.arcs(arcs)
	m.istanbul(typed)
	m.positional(positional)
	m.llvm(llvm)
	order := make([]int, len(m.out))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := m.out[order[i]], m.out[order[j]]
		if a.Line != b.Line || a.Column != b.Column {
			return a.Line < b.Line || a.Line == b.Line && a.Column < b.Column
		}
		return m.rank[order[i]] < m.rank[order[j]]
	})
	sorted := make([]Finding, len(order))
	for i, k := range order {
		sorted[i] = m.out[k]
	}
	return dedupe(sorted)
}

type mapper struct {
	path  string
	decs  []Decision
	gran  string
	lines []string
	out   []Finding
	rank  []int // the arm's index in its decision: orders a decision's findings
}

func (m *mapper) add(d Decision, arm, text string, start, end int) {
	rank := 0
	for i, a := range d.Arms {
		if a.Label == arm {
			rank = i
		}
	}
	m.rank = append(m.rank, rank)
	m.out = append(m.out, Finding{Path: m.path, Line: d.Line, Column: d.Column, Decision: d.Header, Arm: arm,
		Granularity: m.gran, Text: text, SpanStart: start, SpanEnd: end})
}

func (m *mapper) addArm(d Decision, a Arm) {
	start, end := 0, 0
	if a.spanned() {
		start, end = a.StartLine, a.EndLine
	}
	m.add(d, a.Label, ArmText(a), start, end)
}

// onLine returns the decisions on line whose kind passes keep, by column.
func (m *mapper) onLine(line int, keep func(Decision) bool) []Decision {
	var out []Decision
	for _, d := range m.decs {
		if d.Line == line && (keep == nil || keep(d)) {
			out = append(out, d)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Column < out[j].Column })
	return out
}

// arcStatements are the Python nodes coverage.py measures arcs from.
var arcStatements = map[string]bool{"if_statement": true, "elif_clause": true, "for_statement": true,
	"while_statement": true, "case_clause": true, "except_clause": true}

// arcs: an arc (from, to) belongs to the decision on line from, and to the
// arm whose span holds to; otherwise to its spanless arm (false/exit/no match).
func (m *mapper) arcs(hits []coverage.ArmHit) {
	for _, h := range hits {
		if h.Hits > 0 || h.Reached <= 0 {
			continue
		}
		on := m.onLine(h.FromLine, func(d Decision) bool { return arcStatements[d.Kind] })
		if len(on) == 0 {
			continue
		}
		d := on[0]
		var arm *Arm
		for i := range d.Arms {
			if a := &d.Arms[i]; h.ToLine > 0 && a.containsLine(h.ToLine) && (arm == nil || a.StartLine > arm.StartLine) {
				arm = a
			}
		}
		if arm == nil {
			for i := range d.Arms {
				if !d.Arms[i].spanned() {
					arm = &d.Arms[i]
					break
				}
			}
		}
		if arm != nil {
			m.addArm(d, *arm)
		}
	}
}

// istanbulKinds are the decision node types an Istanbul branch type maps to.
var istanbulKinds = map[string]map[string]bool{
	"if":          {"if_statement": true},
	"cond-expr":   {"ternary_expression": true},
	"binary-expr": {"binary_expression": true},
	"switch":      {"switch_case": true, "switch_default": true},
}

// istanbul: an entry belongs to the decision of a matching kind on its line,
// nearest column (ties: the one with as many arms); the arm is Index. A
// switch entry is located at each case, whose single arm it is.
func (m *mapper) istanbul(hits []coverage.ArmHit) {
	for _, h := range hits {
		kinds := istanbulKinds[h.Kind]
		if h.Hits > 0 || h.Reached <= 0 || kinds == nil {
			continue
		}
		on := m.onLine(h.Line, func(d Decision) bool { return kinds[d.Kind] })
		if len(on) == 0 {
			continue
		}
		best := on[0]
		for _, d := range on[1:] {
			db, dd := absInt(best.Column-h.Column), absInt(d.Column-h.Column)
			if dd < db || dd == db && len(d.Arms) == h.Of && len(best.Arms) != h.Of {
				best = d
			}
		}
		index := h.Index
		if h.Kind == "switch" {
			index = 0
		}
		if index < len(best.Arms) {
			m.addArm(best, best.Arms[index])
		}
	}
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func within(h coverage.ArmHit, line, col int) bool {
	return !less(line, col, h.Line, h.Column) && less(line, col, h.EndLine, h.EndColumn)
}

func span(h coverage.ArmHit) [2]int { return [2]int{h.EndLine - h.Line, h.EndColumn - h.Column} }

func smaller(a, b [2]int) bool { return a[0] < b[0] || a[0] == b[0] && a[1] < b[1] }

// innermost returns the index of the smallest hit holding (line, col), -1 if none.
func innermost(hits []coverage.ArmHit, line, col, skip int) int {
	best := -1
	for i, h := range hits {
		if i != skip && within(h, line, col) && (best < 0 || smaller(span(h), span(hits[best]))) {
			best = i
		}
	}
	return best
}

// positional: Go blocks and llvm regions. A decision executed when the
// innermost block/region holding its reach point ran. A zero-hit block (the
// outermost of nested zero regions) belongs to the innermost arm holding its
// start.
func (m *mapper) positional(hits []coverage.ArmHit) {
	if len(hits) == 0 {
		return
	}
	executed := func(d Decision) bool {
		i := innermost(hits, d.ReachLine, d.ReachColumn, -1)
		return i >= 0 && hits[i].Hits > 0
	}
	for i, h := range hits {
		if h.Hits > 0 {
			continue
		}
		if p := innermost(hits, h.Line, h.Column, i); p >= 0 && hits[p].Hits == 0 && smaller(span(h), span(hits[p])) {
			continue
		}
		var owner Decision
		var arm *Arm
		for _, d := range m.decs {
			for k := range d.Arms {
				a := &d.Arms[k]
				if !a.contains(h.Line, h.Column) {
					continue
				}
				if arm == nil || less(arm.StartLine, arm.StartColumn, a.StartLine, a.StartColumn) {
					owner, arm = d, a
				}
			}
		}
		if arm == nil || !executed(owner) {
			continue
		}
		if h.Kind == coverage.KindRegion {
			m.add(owner, arm.Label, regionText(h.Line, h.Column, h.EndLine, h.EndColumn), h.Line, h.EndLine)
			continue
		}
		start := h.Line
		if h.EndLine > h.Line && m.opensOnly(h.Line, h.Column) {
			start++
		}
		m.add(owner, arm.Label, blockText(start, h.EndLine), start, h.EndLine)
	}
}

// opensOnly: the rest of line from col (1-based byte) is only "{" or nothing.
func (m *mapper) opensOnly(line, col int) bool {
	if line < 1 || line > len(m.lines) {
		return false
	}
	text := m.lines[line-1]
	if col-1 > len(text) || col < 1 {
		return false
	}
	rest := strings.TrimSpace(text[col-1:])
	return rest == "" || rest == "{"
}

// llvmDecisions are the Rust decisions whose condition llvm-cov --branch
// measures as a true/false pair.
var llvmDecisions = map[string]bool{"if_expression": true, "while_expression": true}

// llvm: llvm-cov --branch measures each condition. A condition that is an
// operand of a && / || chain is reported as that operand; otherwise it
// belongs to the nearest if/while on its line that starts before it, true
// being the decision's first arm and false its second.
func (m *mapper) llvm(hits []coverage.ArmHit) {
	for _, h := range hits {
		if h.Hits > 0 || h.Reached <= 0 {
			continue
		}
		if d, a, ok := m.operandAt(h.Line, h.Column); ok {
			outcome := []string{"true", "false"}[min(h.Index, 1)]
			text := a.Label + " never " + outcome
			if a.Text != "" {
				text = a.Label + " (" + a.Text + ") never " + outcome
			}
			m.add(d, a.Label, text, a.StartLine, a.EndLine)
			continue
		}
		var owner *Decision
		for _, d := range m.onLine(h.Line, func(d Decision) bool { return llvmDecisions[d.Kind] }) {
			if d.Column <= h.Column {
				owner = &d
			}
		}
		if owner != nil && h.Index < len(owner.Arms) {
			m.addArm(*owner, owner.Arms[h.Index])
		}
	}
}

// operandAt finds the operand arm of a logical chain starting at (line, col).
func (m *mapper) operandAt(line, col int) (Decision, Arm, bool) {
	for _, d := range m.decs {
		for _, a := range d.Arms {
			if strings.HasPrefix(a.Label, "operand ") && a.StartLine == line && a.StartColumn == col {
				return d, a, true
			}
		}
	}
	return Decision{}, Arm{}, false
}

// count: JaCoCo/BRDA/V8 line counts and Cloverage forms belong to the first
// decision on the line (cases last), else to the first case whose switch is
// on it.
func (m *mapper) count(h coverage.ArmHit) {
	reached := h.Reached > 0 || h.Reached < 0 && h.Hits > 0
	if h.Hits >= h.Of || !reached {
		return
	}
	on := m.onLine(h.Line, nil)
	sort.SliceStable(on, func(i, j int) bool { return selfReached(on[i]) && !selfReached(on[j]) })
	if len(on) == 0 {
		for _, d := range m.decs {
			if d.ReachLine == h.Line {
				on = append(on, d)
				break
			}
		}
	}
	if len(on) == 0 {
		return
	}
	text := countText(h.Of-h.Hits, h.Of)
	if h.Kind == coverage.KindForm {
		text = formText(h.Of-h.Hits, h.Of)
	}
	m.add(on[0], "", text, 0, 0)
}

// selfReached: not a case (whose reach point is its switch).
func selfReached(d Decision) bool { return d.ReachLine == d.Line && d.ReachColumn == d.Column }

// Sort orders findings by (path, line, column); stable, so a decision's
// findings keep Map's arm order.
func Sort(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Column < b.Column
	})
}

func dedupe(fs []Finding) []Finding {
	var out []Finding
	seen := map[Finding]bool{}
	for _, f := range fs {
		if !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	return out
}
