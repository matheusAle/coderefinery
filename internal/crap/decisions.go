package crap

// Decisions: the nodes the complexity predicates count, with their arms
// (docs/design-tests.md §6, B1). `crap --branches` and `mutate --branches`
// map tool branch data onto them (internal/branch).

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/matheusAle/coderefinery/internal/branch"
	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/lang"
	"github.com/matheusAle/coderefinery/internal/model"
	"github.com/matheusAle/coderefinery/internal/syntax"
)

type (
	// Decision is branch.Decision (branch owns the type so crap can import it).
	Decision = branch.Decision
	// Arm is branch.Arm.
	Arm = branch.Arm
)

// armsFunc returns a decision's arms, the byte where its header ends (-1 =
// the node's end) and the node whose execution proves it ran.
type armsFunc func(n *syntax.Node, src []byte) (arms []Arm, headerEnd int, reach *syntax.Node)

type decisionSpec struct {
	grammar    func(path string) string
	isDecision func(*syntax.Node) bool
	arms       armsFunc
}

func fixed(g string) func(string) string { return func(string) string { return g } }

var decisionSpecs = map[string]decisionSpec{
	"python":     {fixed("python"), pythonIsDecision, pythonArms},
	"go":         {fixed("go"), goIsDecision, goArms},
	"typescript": {tsGrammar, tsIsDecision, tsArms},
	"java":       {fixed("java"), javaIsDecision, javaArms},
	"rust":       {fixed("rust"), rustIsDecision, rustArms},
}

// Decisions returns the decisions of one file inside its scored functions,
// in preorder (Clojure: in text order, located by form line).
func Decisions(source, path, projectRoot string) ([]Decision, error) {
	language := discover.LanguageOf(path)
	fns, err := FunctionsInFile(language, source, path, projectRoot)
	if err != nil {
		return nil, err
	}
	var decs []Decision
	if language == "clojure" {
		decs = clojureDecisions(source, path)
	} else if spec, ok := decisionSpecs[language]; ok {
		tree, err := lang.ParseText(spec.grammar(path), source)
		if err != nil {
			return nil, err
		}
		decs = treeDecisions(tree, path, spec)
	}
	return insideFunctions(decs, fns), nil
}

func insideFunctions(decs []Decision, fns []model.Function) []Decision {
	var out []Decision
	for _, d := range decs {
		for _, fn := range fns {
			if d.Line >= fn.StartLine && d.Line <= fn.EndLine {
				out = append(out, d)
				break
			}
		}
	}
	return out
}

func treeDecisions(tree *syntax.Tree, path string, spec decisionSpec) []Decision {
	src := tree.Source
	var out []Decision
	tree.Root.Descendants(func(n *syntax.Node) bool {
		if !spec.isDecision(n) {
			return true
		}
		arms, headerEnd, reach := spec.arms(n, src)
		if reach == nil {
			reach = n
		}
		if headerEnd < 0 {
			headerEnd = n.EndByte
		}
		out = append(out, Decision{
			Path: path, Line: n.StartLine(), Column: n.StartPoint.Column + 1, EndLine: n.EndLine(),
			Kind: n.Type, Header: decisionHeader(string(src[n.StartByte:headerEnd])), Arms: arms,
			ReachLine: reach.StartLine(), ReachColumn: reach.StartPoint.Column + 1,
		})
		return true
	})
	return out
}

// decisionHeader is the header's first line up to ':'/'{', trimmed, at
// most 60 runes.
func decisionHeader(text string) string {
	text, _, _ = strings.Cut(text, "\n")
	text = strings.TrimSpace(text)
	text = strings.TrimSpace(strings.TrimRight(text, ":{"))
	return shorten(text, 60)
}

func shorten(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	return string([]rune(text)[:n]) + "…"
}

// nodeArm spans a node; a nil node gives a spanless arm.
func nodeArm(label string, n *syntax.Node) Arm {
	if n == nil {
		return Arm{Label: label}
	}
	return spanArm(label, n.StartPoint, n.EndPoint)
}

// spanArm spans [start, end). An end at column 0 (a node that takes its
// line's newline, like a Go case) ends with the previous line.
func spanArm(label string, start, end syntax.Point) Arm {
	a := Arm{Label: label, StartLine: start.Row + 1, StartColumn: start.Column + 1,
		EndLine: end.Row + 1, EndColumn: end.Column + 1}
	if end.Column == 0 && end.Row > start.Row {
		a.EndLine, a.EndColumn = end.Row, endOfLine
	}
	return a
}

// endOfLine is a column past any line's end.
const endOfLine = 1 << 30

// childrenArm spans a run of sibling nodes (an if's elif/else chain).
func childrenArm(label string, nodes []*syntax.Node) Arm {
	if len(nodes) == 0 {
		return Arm{Label: label}
	}
	return spanArm(label, nodes[0].StartPoint, nodes[len(nodes)-1].EndPoint)
}

// afterToken spans from the end of n's first child of type tok to n's end
// (a case body after its ':'); without the token, all of n.
func afterToken(label string, n *syntax.Node, tok string) (Arm, int) {
	if t := n.ChildOfType(tok); t != nil {
		return spanArm(label, t.EndPoint, n.EndPoint), t.EndByte
	}
	return nodeArm(label, n), -1
}

func operandArms(src []byte, operands []*syntax.Node) []Arm {
	arms := make([]Arm, len(operands))
	for i, o := range operands {
		arms[i] = nodeArm("operand "+itoa(i+1), o)
		text, _, _ := strings.Cut(o.Text(src), "\n")
		arms[i].Text = shorten(strings.TrimSpace(text), 30)
	}
	return arms
}

// logicalOperands flattens a chain of logical binary expressions through
// parentheses, as Istanbul does for binary-expr: `a && (b || c)` is a, b, c.
func logicalOperands(n *syntax.Node, logical func(*syntax.Node) bool) []*syntax.Node {
	var out []*syntax.Node
	var visit func(*syntax.Node)
	visit = func(x *syntax.Node) {
		inner := x
		for inner.Type == "parenthesized_expression" && inner.NamedChildCount() == 1 {
			inner = inner.NamedChildren()[0]
		}
		if !logical(inner) {
			out = append(out, x)
			return
		}
		visit(inner.ChildByFieldName("left"))
		visit(inner.ChildByFieldName("right"))
	}
	visit(n)
	return out
}

func binaryArms(n *syntax.Node, src []byte) []Arm {
	return operandArms(src, []*syntax.Node{n.ChildByFieldName("left"), n.ChildByFieldName("right")})
}

// caseIndex is the 1-based position of n among its parent's children of the given types.
func caseIndex(n *syntax.Node, types map[string]bool) int {
	k := 0
	for _, c := range n.Parent.Children {
		if types[c.Type] {
			k++
		}
		if c == n {
			return k
		}
	}
	return k
}

func ifArms(n *syntax.Node) []Arm {
	return []Arm{nodeArm("true", n.ChildByFieldName("consequence")),
		childrenArm("false", n.ChildrenByFieldName("alternative"))}
}

func headerAt(n *syntax.Node) int {
	if n == nil {
		return -1
	}
	return n.StartByte
}

func loopArms(n *syntax.Node) ([]Arm, int) {
	body := n.ChildByFieldName("body")
	return []Arm{nodeArm("body", body), {Label: "exit"}}, headerAt(body)
}

func pythonArms(n *syntax.Node, src []byte) ([]Arm, int, *syntax.Node) {
	switch n.Type {
	case "if_statement", "elif_clause":
		return ifArms(n), headerAt(n.ChildByFieldName("consequence")), nil
	case "for_statement", "while_statement":
		body := n.ChildByFieldName("body")
		return []Arm{nodeArm("body", body), nodeArm("exit", n.ChildByFieldName("alternative"))}, headerAt(body), nil
	case "case_clause":
		cons := n.ChildByFieldName("consequence")
		label := "case " + itoa(caseIndex(n, map[string]bool{"case_clause": true}))
		return []Arm{nodeArm(label, cons), {Label: "no match"}}, headerAt(cons), nil
	case "except_clause":
		block := n.ChildOfType("block")
		return []Arm{nodeArm("handler", block)}, headerAt(block), nil
	case "conditional_expression":
		named := n.NamedChildren()
		if len(named) == 3 {
			return []Arm{nodeArm("true", named[0]), nodeArm("false", named[2])}, -1, nil
		}
	case "boolean_operator":
		return binaryArms(n, src), -1, nil
	case "if_clause":
		return []Arm{{Label: "true"}, {Label: "false"}}, -1, nil
	}
	return nil, -1, nil
}

var goCases = map[string]bool{"expression_case": true, "type_case": true, "communication_case": true, "default_case": true}

func goArms(n *syntax.Node, src []byte) ([]Arm, int, *syntax.Node) {
	switch n.Type {
	case "if_statement":
		return ifArms(n), headerAt(n.ChildByFieldName("consequence")), nil
	case "for_statement":
		arms, end := loopArms(n)
		return arms, end, nil
	case "binary_expression":
		return binaryArms(n, src), -1, nil
	}
	label := "case " + itoa(caseIndex(n, goCases))
	if n.Type == "default_case" {
		label = "default"
	}
	arm, end := afterToken(label, n, ":")
	return []Arm{arm}, end, n.Parent
}

func tsLogical(n *syntax.Node) bool {
	return n.Type == "binary_expression" && n.HasChildOfType("&&", "||", "??")
}

func tsArms(n *syntax.Node, src []byte) ([]Arm, int, *syntax.Node) {
	switch n.Type {
	case "if_statement":
		return ifArms(n), headerAt(n.ChildByFieldName("consequence")), nil
	case "for_statement", "for_in_statement", "while_statement", "do_statement":
		arms, end := loopArms(n)
		return arms, end, nil
	case "catch_clause":
		body := n.ChildByFieldName("body")
		return []Arm{nodeArm("handler", body)}, headerAt(body), nil
	case "ternary_expression":
		return []Arm{nodeArm("true", n.ChildByFieldName("consequence")),
			nodeArm("false", n.ChildByFieldName("alternative"))}, -1, nil
	case "switch_case", "switch_default":
		label := "case " + itoa(caseIndex(n, map[string]bool{"switch_case": true, "switch_default": true}))
		if n.Type == "switch_default" {
			label = "default"
		}
		arm, end := afterToken(label, n, ":")
		return []Arm{arm}, end, n.Parent.Parent
	case "binary_expression":
		return operandArms(src, logicalOperands(n, tsLogical)), -1, nil
	}
	return nil, -1, nil // optional chains have no arms a tool reports
}

func javaArms(n *syntax.Node, src []byte) ([]Arm, int, *syntax.Node) {
	switch n.Type {
	case "if_statement":
		return ifArms(n), headerAt(n.ChildByFieldName("consequence")), nil
	case "for_statement", "enhanced_for_statement", "while_statement", "do_statement":
		arms, end := loopArms(n)
		return arms, end, nil
	case "catch_clause":
		body := n.ChildByFieldName("body")
		return []Arm{nodeArm("handler", body)}, headerAt(body), nil
	case "ternary_expression":
		return []Arm{nodeArm("true", n.ChildByFieldName("consequence")),
			nodeArm("false", n.ChildByFieldName("alternative"))}, -1, nil
	case "switch_label":
		group := n.Parent
		label := "case " + itoa(caseIndex(group, map[string]bool{"switch_block_statement_group": true, "switch_rule": true}))
		if n.NamedChildCount() == 0 {
			label = "default"
		}
		var reach *syntax.Node
		if group.Parent != nil {
			reach = group.Parent.Parent
		}
		return []Arm{spanArm(label, n.EndPoint, group.EndPoint)}, -1, reach
	case "binary_expression":
		return binaryArms(n, src), -1, nil
	}
	return nil, -1, nil
}

func rustArms(n *syntax.Node, src []byte) ([]Arm, int, *syntax.Node) {
	switch n.Type {
	case "if_expression":
		return ifArms(n), headerAt(n.ChildByFieldName("consequence")), nil
	case "for_expression", "while_expression", "loop_expression":
		arms, end := loopArms(n)
		return arms, end, nil
	case "match_arm":
		label := "case " + itoa(caseIndex(n, map[string]bool{"match_arm": true}))
		value := n.ChildByFieldName("value")
		var reach *syntax.Node
		if n.Parent != nil {
			reach = n.Parent.Parent
		}
		return []Arm{nodeArm(label, value)}, headerAt(value), reach
	case "try_expression":
		return []Arm{nodeArm("error", n.ChildOfType("?"))}, -1, nil
	case "binary_expression":
		return binaryArms(n, src), -1, nil
	}
	return nil, -1, nil
}

// clojureDecisions locates the forms crap counts (cljDecision, cljCond) by
// line; Cloverage reports forms per line, so they carry no arms. The
// cleaned text keeps every rune and newline of the source.
func clojureDecisions(source, path string) []Decision {
	clean := WithoutStringsAndComments(source)
	runes := []rune(source)
	var out []Decision
	for _, re := range []*regexp.Regexp{cljDecision, cljCond} {
		for _, m := range re.FindAllStringSubmatchIndex(clean, -1) {
			line := strings.Count(clean[:m[0]], "\n") + 1
			col := utf8.RuneCountInString(clean[strings.LastIndexByte(clean[:m[0]], '\n')+1:m[0]]) + 1
			at := min(utf8.RuneCountInString(clean[:m[0]]), len(runes))
			out = append(out, Decision{Path: path, Line: line, Column: col, EndLine: line, Kind: clean[m[2]:m[3]],
				Header: decisionHeader(string(runes[at:])), ReachLine: line, ReachColumn: col})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Line < out[j].Line || out[i].Line == out[j].Line && out[i].Column < out[j].Column
	})
	return out
}
