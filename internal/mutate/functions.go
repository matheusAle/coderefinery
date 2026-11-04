package mutate

// Bind mutation sites to the functions crapper reports (upstream
// mutator/functions.py).

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/matheusAle/coderefinery/internal/coverage"
	"github.com/matheusAle/coderefinery/internal/crap"
	"github.com/matheusAle/coderefinery/internal/discover"
	"github.com/matheusAle/coderefinery/internal/model"
	"github.com/matheusAle/coderefinery/internal/pyjson"
)

// isPySpace is Python str.isspace for one character.
func isPySpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0x1c, 0x1d, 0x1e, 0x1f, 0x85, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

func pyRstrip(s string) string { return strings.TrimRightFunc(s, isPySpace) }
func pyLstrip(s string) string { return strings.TrimLeftFunc(s, isPySpace) }

// lineWindow is source.splitlines()[start-1:end] (Python slice clamping).
func lineWindow(lines []string, start, end int) []string {
	lo, hi := start-1, end
	if lo < 0 {
		lo = 0
	}
	if hi > len(lines) {
		hi = len(lines)
	}
	if lo >= hi {
		return nil
	}
	return lines[lo:hi]
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// digest is functions.digest: sha256 of the function's lines, each rstripped.
func digest(lines []string, start, end int) string {
	window := lineWindow(lines, start, end)
	parts := make([]string, len(window))
	for i, l := range window {
		parts[i] = pyRstrip(l)
	}
	return sha256Hex(strings.Join(parts, "\n"))
}

// header is functions._header: up to seven lines, lstripped, leading
// `#[...]` attribute groups removed.
func header(lines []string, start, end int) string {
	text := pyLstrip(strings.Join(lineWindow(lines, start, min(end, start+6)), "\n"))
	for strings.HasPrefix(text, "#[") {
		close := strings.IndexByte(text, ']')
		if close < 0 {
			break
		}
		text = pyLstrip(text[close+1:])
	}
	return text
}

func isWordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.Is(unicode.Nl, r) || unicode.Is(unicode.No, r)
}

// hasWord is re.search(r"\bWORD\b", text) with Python's Unicode \w.
func hasWord(text, word string) bool {
	for from := 0; ; {
		i := strings.Index(text[from:], word)
		if i < 0 {
			return false
		}
		i += from
		before, _ := utf8.DecodeLastRuneInString(text[:i])
		after, _ := utf8.DecodeRuneInString(text[i+len(word):])
		if (i == 0 || !isWordRune(before)) && (i+len(word) == len(text) || !isWordRune(after)) {
			return true
		}
		from = i + 1
	}
}

func pythonPrivate(name string) bool {
	if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
		return false
	}
	return strings.HasPrefix(name, "_")
}

// isPrivate is functions.is_private.
func isPrivate(language, name string, lines []string, start, end int) bool {
	h := header(lines, start, end)
	switch language {
	case "clojure":
		return strings.HasPrefix(h, "(defn-")
	case "java":
		return hasWord(h, "private")
	case "go":
		r, _ := utf8.DecodeRuneInString(name)
		return name != "" && unicode.IsLower(r)
	case "python":
		return pythonPrivate(name)
	case "rust":
		rest, ok := strings.CutPrefix(h, "pub")
		if !ok {
			return true
		}
		r, _ := utf8.DecodeRuneInString(rest)
		return rest == "" || !isPySpace(r)
	case "typescript":
		return strings.HasPrefix(name, "#") || hasWord(h, "private")
	}
	return false
}

// formID is functions.form_id.
func formID(name string, private bool) string {
	if private {
		return "defn-/" + name
	}
	return "defn/" + name
}

// mutationID is functions.mutation_id: compact json.dumps of the site.
func mutationID(file, namespace, form string, start, end int, original, mutant string) string {
	return pyjson.Dumps([]any{file, namespace, form, start, end, original, mutant}, ",", ":")
}

// mutationField is functions._mutation_field: element index of the decoded
// id when it is a string.
func mutationField(mutation string, index int) (string, bool) {
	var value any
	if err := json.Unmarshal([]byte(mutation), &value); err != nil {
		return "", false
	}
	list, ok := value.([]any)
	if !ok || len(list) <= index {
		return "", false
	}
	s, ok := list[index].(string)
	return s, ok
}

func mutationFile(m string) (string, bool)      { return mutationField(m, 0) }
func mutationNamespace(m string) (string, bool) { return mutationField(m, 1) }

// projectFunctions is functions.project_functions. A file the hand parser
// cannot read contributes no functions (DEVIATIONS.md A2).
func projectFunctions(source, path, root string) []model.Function {
	language := discover.LanguageOf(path)
	if language == "" {
		return nil
	}
	fns, err := crap.FunctionsInFile(language, source, discover.Resolve(path), discover.Resolve(root))
	if err != nil {
		return nil
	}
	return fns
}

func contains(fn model.Function, line, byteOffset int) bool {
	if fn.StartLine > line || fn.EndLine < line {
		return false
	}
	if fn.StartByte < 0 || fn.EndByte < 0 {
		return true
	}
	return fn.StartByte <= byteOffset && byteOffset < fn.EndByte
}

func tightness(fn model.Function) int {
	if fn.StartByte >= 0 && fn.EndByte >= fn.StartByte {
		return fn.EndByte - fn.StartByte
	}
	return (fn.EndLine - fn.StartLine) * 1_000_000
}

// owner is functions._owner: the tightest function containing the site,
// first in crapper's order on ties; -1 when none.
func owner(functions []model.Function, line, byteOffset int) int {
	best := -1
	for i, fn := range functions {
		if !contains(fn, line, byteOffset) {
			continue
		}
		if best < 0 {
			best = i
			continue
		}
		b := functions[best]
		if t, bt := tightness(fn), tightness(b); t < bt || t == bt && fn.StartLine < b.StartLine {
			best = i
		}
	}
	return best
}

// fileUnit is one source file with its crapper functions, the input every
// per-file step shares (upstream re-parses for each step).
type fileUnit struct {
	path      string // resolved absolute path
	key       string // path relative to root, posix
	language  string
	source    string
	lines     []string // source.splitlines()
	functions []model.Function
	private   []bool // per function
}

func newFileUnit(source, path, root, key string) *fileUnit {
	u := &fileUnit{path: path, key: key, language: discover.LanguageOf(path), source: source}
	u.lines = coverage.SplitLines(source)
	u.functions = projectFunctions(source, path, root)
	u.private = make([]bool, len(u.functions))
	for i, fn := range u.functions {
		u.private[i] = isPrivate(u.language, fn.Name, u.lines, fn.StartLine, fn.EndLine)
	}
	return u
}

func (u *fileUnit) formKey(i int) formKey {
	fn := u.functions[i]
	return formKey{fn.Namespace, formID(fn.Name, u.private[i])}
}

// sites is functions.sites_in_file.
func (u *fileUnit) sites() []Site {
	if u.language == "" {
		return nil
	}
	var found []Site
	for _, raw := range discoverRaw(u.source, u.language, u.path) {
		i := owner(u.functions, raw.Line, raw.Start)
		if i < 0 {
			continue
		}
		fn := u.functions[i]
		form := formID(fn.Name, u.private[i])
		found = append(found, Site{
			File:       u.key,
			Namespace:  fn.Namespace,
			FormID:     form,
			Name:       fn.Name,
			Line:       raw.Line,
			Start:      raw.Start,
			End:        raw.End,
			Original:   raw.Original,
			Mutant:     raw.Mutant,
			Category:   raw.Category,
			ID:         mutationID(u.key, fn.Namespace, form, raw.Start, raw.End, raw.Original, raw.Mutant),
			AnchorLine: raw.AnchorLine,
		})
	}
	return found
}

// digests is functions.form_digests: (namespace, form id) -> hash of the
// hashes of its functions' text in (start_line, end_line) order.
func (u *fileUnit) digests() map[formKey]string {
	grouped := map[formKey][]model.Function{}
	var order []formKey
	for i, fn := range u.functions {
		k := u.formKey(i)
		if _, ok := grouped[k]; !ok {
			order = append(order, k)
		}
		grouped[k] = append(grouped[k], fn)
	}
	out := map[formKey]string{}
	for _, k := range order {
		fns := grouped[k]
		sortStable(fns, func(a, b model.Function) bool {
			return a.StartLine < b.StartLine || a.StartLine == b.StartLine && a.EndLine < b.EndLine
		})
		var b strings.Builder
		for _, fn := range fns {
			b.WriteString(digest(u.lines, fn.StartLine, fn.EndLine))
		}
		out[k] = sha256Hex(b.String())
	}
	return out
}

// formSpan is one functions.form_spans value.
type formSpan struct {
	key        formKey
	start, end int
	name       string
	private    bool
}

// spans is functions.form_spans, in first-seen order.
func (u *fileUnit) spans() []formSpan {
	var out []formSpan
	at := map[formKey]int{}
	for i, fn := range u.functions {
		k := u.formKey(i)
		if j, ok := at[k]; ok {
			out[j].start = min(out[j].start, fn.StartLine)
			out[j].end = max(out[j].end, fn.EndLine)
			continue
		}
		at[k] = len(out)
		out = append(out, formSpan{k, fn.StartLine, fn.EndLine, fn.Name, u.private[i]})
	}
	return out
}
