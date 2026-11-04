// Package branch maps tool branch data (internal/coverage) onto the decision
// nodes crap counts for cyclomatic complexity (internal/crap/decisions.go)
// and renders the `crap --branches` section and .metrics/branches.edn
// (docs/design-tests.md §6). Only arms of decisions that executed are
// reported: a decision on a line that never ran is plain uncovered code.
package branch

// Arm is one way out of a decision. A zero span (StartLine 0) is "the
// rest": an `if` without else, a loop exit, a case that did not match.
// Columns are 1-based bytes; the end is exclusive.
type Arm struct {
	Label                                      string // "true" "false" "body" "exit" "case <n>" "default" "no match" "handler" "error" "operand <n>"
	Text                                       string // the operand's source, shortened ("operand <n>" arms)
	StartLine, StartColumn, EndLine, EndColumn int
}

// Decision is one node a complexity predicate counts, with its arms.
// Reach is where execution proves the decision ran: the decision itself,
// or for a case its switch.
type Decision struct {
	Path                   string
	Line, Column, EndLine  int
	Kind, Header           string
	Arms                   []Arm
	ReachLine, ReachColumn int
}

// Finding is one untested arm, block, region or count.
type Finding struct {
	Path               string
	Line, Column       int // the decision
	Decision           string
	Arm                string
	Granularity        string
	Text               string
	SpanStart, SpanEnd int // lines of the untested code (arm, block or region); 0 when unknown
}

func (a Arm) spanned() bool { return a.StartLine > 0 }

func (a Arm) contains(line, col int) bool {
	return a.spanned() && !less(line, col, a.StartLine, a.StartColumn) && less(line, col, a.EndLine, a.EndColumn)
}

func (a Arm) containsLine(line int) bool {
	return a.spanned() && line >= a.StartLine && line <= a.EndLine
}

func less(l1, c1, l2, c2 int) bool { return l1 < l2 || l1 == l2 && c1 < c2 }
