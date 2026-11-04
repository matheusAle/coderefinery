package crap

// Clojure entries (upstream crapper/languages/clojure.py, ported from
// crap4clj). Text based, no tree: top-level defn/defn- forms, a regex count
// of decision forms, and the clauses of cond-like forms. Indexes are code
// points, as in the Python.

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/matheusAle/coderefinery/internal/model"
)

// pyWS is Python's Unicode \s, for the regexes below.
const pyWS = `[\t\n\v\f\r \x{1c}-\x{1f}\x{85}\x{a0}\x{1680}\x{2000}-\x{200a}\x{2028}\x{2029}\x{202f}\x{205f}\x{3000}]`

var (
	cljDecision = regexp.MustCompile(`\((if-not|if-let|if-some|when-not|when-let|when-some|when-first|if|when|and|or|loop|catch)(?:` + pyWS + `|\))`)
	cljCond     = regexp.MustCompile(`\((some->>|some->|cond->>|cond->|cond|condp|case)(?:` + pyWS + `|\))`)
	cljNS       = regexp.MustCompile(`\(` + pyWS + `*ns` + pyWS + `+([A-Za-z0-9*+!_?.\-/]+)`)
	cljInNS     = regexp.MustCompile(`\(` + pyWS + `*in-ns` + pyWS + `+'([A-Za-z0-9*+!_?.\-/]+)` + pyWS + `*\)`)
	cljInNSQ    = regexp.MustCompile(`\(` + pyWS + `*in-ns` + pyWS + `+\(quote` + pyWS + `+([A-Za-z0-9*+!_?.\-/]+)\)` + pyWS + `*\)`)
	cljSuffix   = regexp.MustCompile(`\.(?:clj[cs]?|bb)$`)
)

var cljFormSkip = map[string]int{"condp": 2, "case": 1, "cond->": 1, "cond->>": 1, "some->": 1, "some->>": 1}

func cljIsSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

func runesHasPrefix(s []rune, i int, prefix string) bool {
	for _, r := range prefix {
		if i >= len(s) || s[i] != r {
			return false
		}
		i++
	}
	return true
}

func cljCharLiteralEnd(s []rune, i int) int {
	first := i + 1
	if first >= len(s) {
		return len(s)
	}
	for j := first + 1; j < len(s); j++ {
		if cljIsSpace(s[j]) || strings.ContainsRune("()[]{}\";,", s[j]) {
			return j
		}
	}
	return len(s)
}

// cljConsumeString returns the index after the string at i and the line
// count advanced by its newlines.
func cljConsumeString(s []rune, i, line int) (int, int) {
	i++
	escaped := false
	for i < len(s) {
		ch := s[i]
		switch {
		case escaped:
			escaped = false
		case ch == '\\':
			escaped = true
		case ch == '"':
			return i + 1, line
		}
		if ch == '\n' {
			line++
		}
		i++
	}
	return i, line
}

func cljAdvancePastReaderToken(s []rune, i int) (int, bool) {
	switch s[i] {
	case '"':
		end, _ := cljConsumeString(s, i, 1)
		return end, true
	case '\\':
		return cljCharLiteralEnd(s, i), true
	case ';':
		for i < len(s) && s[i] != '\n' {
			i++
		}
		return i, true
	}
	return 0, false
}

func cljBalancedEnd(s []rune, i int) int {
	opener := s[i]
	closer := map[rune]rune{'(': ')', '[': ']', '{': '}'}[opener]
	depth := 0
	for i < len(s) {
		if next, ok := cljAdvancePastReaderToken(s, i); ok {
			i = next
			continue
		}
		ch := s[i]
		i++
		if ch == opener {
			depth++
		} else if ch == closer {
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return i
}

func cljSkipSpaceAndComment(s []rune, i int) int {
	for i < len(s) && cljIsSpace(s[i]) {
		i++
	}
	if i < len(s) && s[i] == ';' {
		for i < len(s) && s[i] != '\n' {
			i++
		}
	}
	return i
}

func cljSkipIgnorable(s []rune, i int) int {
	for {
		next := cljSkipSpaceAndComment(s, i)
		if next == i {
			return i
		}
		i = next
	}
}

func cljEndOfAtom(s []rune, i int) int {
	for i < len(s) && !cljIsSpace(s[i]) && !strings.ContainsRune("()[]{}\";", s[i]) {
		i++
	}
	return i
}

func cljEndOfOneForm(s []rune, i int) int {
	switch ch := s[i]; {
	case ch == '(' || ch == '[' || ch == '{':
		return cljBalancedEnd(s, i)
	case ch == '"':
		end, _ := cljConsumeString(s, i, 1)
		return end
	case ch == '\\':
		return cljCharLiteralEnd(s, i)
	case runesHasPrefix(s, i, "#_"):
		return cljFormEnd(s, i+2)
	}
	return cljEndOfAtom(s, i)
}

// cljFormEnd is the index just after the next form.
func cljFormEnd(s []rune, i int) int {
	i = cljSkipIgnorable(s, i)
	if i >= len(s) {
		return i
	}
	return cljEndOfOneForm(s, i)
}

func cljBlank(s []rune, start, end int, out []rune) []rune {
	for _, ch := range s[start:end] {
		if ch == '\n' {
			out = append(out, '\n')
		} else {
			out = append(out, ' ')
		}
	}
	return out
}

// WithoutStringsAndComments is clojure.without_strings_and_comments: blank
// strings, comments, character literals and #_ discarded forms.
func WithoutStringsAndComments(source string) string {
	s := []rune(source)
	out := make([]rune, 0, len(s))
	for i := 0; i < len(s); {
		switch {
		case runesHasPrefix(s, i, "#_"):
			end := cljFormEnd(s, i+2)
			out = cljBlank(s, i, end, out)
			i = end
		case s[i] == '"':
			end, _ := cljConsumeString(s, i, 1)
			out = cljBlank(s, i, end, out)
			i = end
		case s[i] == '\\':
			end := cljCharLiteralEnd(s, i)
			out = cljBlank(s, i, end, out)
			i = end
		case s[i] == ';':
			for i < len(s) && s[i] != '\n' {
				out = append(out, ' ')
				i++
			}
		default:
			out = append(out, s[i])
			i++
		}
	}
	return string(out)
}

func cljCountTopLevelForms(s []rune, start int) int {
	depth, forms, inForm := 1, 0, false
	for i := start; i < len(s) && depth != 0; i++ {
		ch := s[i]
		switch {
		case ch == '(' || ch == '{' || ch == '[':
			if depth == 1 && !inForm {
				forms++
			}
			depth, inForm = depth+1, true
		case ch == ')' || ch == '}' || ch == ']':
			if depth == 1 {
				inForm = false
			}
			depth--
		case cljIsSpace(ch):
			inForm = depth != 1
		default:
			if depth == 1 && !inForm {
				forms++
			}
			inForm = true
		}
	}
	return forms
}

func cljCountClauses(s []rune, formType string, matchStart int) int {
	i := matchStart + 1
	for i < len(s) && !cljIsSpace(s[i]) && s[i] != ')' {
		i++
	}
	remaining := cljCountTopLevelForms(s, i) - cljFormSkip[formType]
	if formType == "some->" || formType == "some->>" {
		return remaining
	}
	base := pyFloorDiv(remaining, 2)
	if formType == "case" && pyMod(remaining, 2) == 1 {
		return base + 1
	}
	return base
}

func pyFloorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func pyMod(a, b int) int { return a - pyFloorDiv(a, b)*b }

// ClojureComplexity is clojure.cyclomatic_complexity.
func ClojureComplexity(fnText string) int {
	clean := WithoutStringsAndComments(fnText)
	simple := len(cljDecision.FindAllStringIndex(clean, -1))
	runes := []rune(clean)
	total := 0
	for _, m := range cljCond.FindAllStringSubmatchIndex(clean, -1) {
		start := utf8.RuneCountInString(clean[:m[0]])
		total += cljCountClauses(runes, clean[m[2]:m[3]], start)
	}
	return 1 + simple + total
}

const cljNameStop = " \t\n\r()[]{}\""

func cljSkipSpace(s []rune, i int) int {
	for i < len(s) && cljIsSpace(s[i]) {
		i++
	}
	return i
}

func cljDefnKeywordEnd(s []rune, i int) (int, bool) {
	for _, cand := range []string{"defn-", "defn"} {
		after := i + utf8.RuneCountInString(cand)
		if runesHasPrefix(s, i, cand) && (after >= len(s) || strings.ContainsRune(cljNameStop, s[after])) {
			return after, true
		}
	}
	return 0, false
}

func cljSkipLeadingMetadata(s []rune, i int) int {
	for {
		i = cljSkipSpace(s, i)
		if i < len(s) && s[i] == '^' {
			i++
			if i < len(s) && s[i] == '{' {
				i = cljBalancedEnd(s, i)
				continue
			}
			for i < len(s) && !strings.ContainsRune(cljNameStop, s[i]) {
				i++
			}
			continue
		}
		return i
	}
}

func cljDefnName(form []rune) (string, bool) {
	if len(form) == 0 || form[0] != '(' {
		return "", false
	}
	at, ok := cljDefnKeywordEnd(form, cljSkipSpace(form, 1))
	if !ok {
		return "", false
	}
	i := cljSkipLeadingMetadata(form, at)
	if i >= len(form) || strings.ContainsRune("()[]{}\";", form[i]) {
		return "", false
	}
	start := i
	for i < len(form) && !strings.ContainsRune(cljNameStop, form[i]) {
		i++
	}
	return string(form[start:i]), true
}

// ClojureDefn is one extracted top-level defn.
type ClojureDefn struct {
	Name               string
	StartLine, EndLine int
	Text               string
}

// cljSkipReader skips a comment, string, character or #_ form at i.
func cljSkipReader(s []rune, i, line int) (int, int, bool) {
	switch {
	case s[i] == ';':
		i++
		for i < len(s) && s[i] != '\n' {
			i++
		}
		if i < len(s) {
			return i + 1, line + 1, true
		}
		return i, line, true
	case s[i] == '"':
		end, l := cljConsumeString(s, i, line)
		return end, l, true
	case s[i] == '\\':
		return cljCharLiteralEnd(s, i), line, true
	case runesHasPrefix(s, i, "#_"):
		end := cljFormEnd(s, i+2)
		return end, line + strings.Count(string(s[i:end]), "\n"), true
	}
	return 0, 0, false
}

// ClojureTopLevelDefns is clojure._extract_top_level_defns.
func ClojureTopLevelDefns(source string) []ClojureDefn {
	s := []rune(source)
	var forms []ClojureDefn
	line, depth := 1, 0
	formStart, formLine := -1, -1
	for i := 0; i < len(s); {
		if next, l, ok := cljSkipReader(s, i, line); ok {
			i, line = next, l
			continue
		}
		switch s[i] {
		case '(':
			if depth == 0 {
				formStart, formLine = i, line
			}
			depth++
		case ')':
			if depth == 1 && formStart >= 0 && formLine >= 0 {
				text := s[formStart : i+1]
				if name, ok := cljDefnName(text); ok {
					forms = append(forms, ClojureDefn{name, formLine, line, string(text)})
				}
				depth, formStart, formLine = 0, -1, -1
			} else {
				depth = max(0, depth-1)
			}
		}
		if s[i] == '\n' {
			line++
		}
		i++
	}
	return forms
}

// ClojureDeclaredNamespace is clojure.declared_namespace.
func ClojureDeclaredNamespace(source string) (string, bool) {
	clean := WithoutStringsAndComments(source)
	for _, re := range []*regexp.Regexp{cljNS, cljInNS, cljInNSQ} {
		if m := re.FindStringSubmatch(clean); m != nil {
			return m[1], true
		}
	}
	return "", false
}

// ClojureNamespaceFromPath is clojure.namespace_from_path.
func ClojureNamespaceFromPath(path, sourceRoot string) string {
	relative := strings.ReplaceAll(path, `\`, "/")
	root := strings.TrimRight(strings.ReplaceAll(sourceRoot, `\`, "/"), "/")
	if root != "" && root != "." && strings.HasPrefix(relative, root+"/") {
		relative = relative[len(root)+1:]
	}
	if strings.HasPrefix(relative, "src/") {
		relative = relative[4:]
	} else if _, after, ok := strings.Cut(relative, "/src/"); ok {
		relative = after
	}
	relative = cljSuffix.ReplaceAllString(relative, "")
	return strings.ReplaceAll(strings.ReplaceAll(relative, "/", "."), "_", "-")
}

// ClojureFunctions is clojure.functions_in_source.
func ClojureFunctions(source, path, sourceRoot string) ([]model.Function, error) {
	namespace, ok := ClojureDeclaredNamespace(source)
	if !ok || namespace == "" {
		namespace = ClojureNamespaceFromPath(path, sourceRoot)
	}
	var found []model.Function
	for _, d := range ClojureTopLevelDefns(source) {
		found = append(found, model.Function{
			Name:       d.Name,
			Namespace:  namespace,
			Complexity: ClojureComplexity(d.Text),
			StartLine:  d.StartLine,
			EndLine:    d.EndLine,
			Path:       path,
			Language:   "clojure",
			StartByte:  -1,
			EndByte:    -1,
		})
	}
	return found, nil
}
