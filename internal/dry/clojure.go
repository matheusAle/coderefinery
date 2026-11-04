package dry

import (
	"fmt"
	"regexp"
	"strings"
)

// The Clojure reader and normalizer: upstream clojure_read.py
// (spec-dryer §7). No tree-sitter; this is a line-by-line port. Upstream
// reads with an explicit frame stack; this reader recurses (Go stacks grow
// on demand) with the same consumption order and the same errors.

// FormKind tells a Clojure form apart.
type FormKind uint8

const (
	FormSym FormKind = iota
	FormKw
	FormLit
	FormColl
)

// Form is one read Clojure form: upstream Sym, Kw, Lit or Coll.
type Form struct {
	Kind  FormKind
	Name  string // symbol or keyword name
	Coll  string // collection kind: list, vector, map, set
	Line  int
	Items []*Form    // list, vector and set items
	Pairs [][2]*Form // map entries in source order
}

const (
	cljWhitespace  = " \t\n\r,"
	cljTerminators = "()[]{}\";'@^`~\\,#:"
)

var cljNumber = regexp.MustCompile(`^[+-]?(?:` +
	`\p{Nd}+/\p{Nd}+` +
	`|\p{Nd}+\.\p{Nd}+(?:[eE][+-]?\p{Nd}+)?` +
	`|\.\p{Nd}+(?:[eE][+-]?\p{Nd}+)?` +
	`|\p{Nd}+(?:[eE][+-]?\p{Nd}+)?` +
	`|0[xX][0-9A-Fa-f]+` +
	`)[MN]?$`)

type readError struct {
	line int
	msg  string
}

// produced is what reading one form yields: nothing (upstream None), a
// form, a splice (#?@), or "need" (a form was consumed without producing
// anything, so the caller keeps reading in its own context: ^meta, #:ns).
type produced struct {
	form   *Form
	splice []*Form
	isSpl  bool
	need   bool
}

type cljReader struct {
	text []rune
	i    int
	line int
}

func isWS(r rune) bool         { return strings.ContainsRune(cljWhitespace, r) }
func isTerminator(r rune) bool { return isWS(r) || strings.ContainsRune(cljTerminators, r) }

func (r *cljReader) eof() bool { return r.i >= len(r.text) }

// peek returns -1 at EOF.
func (r *cljReader) peek() rune {
	if r.eof() {
		return -1
	}
	return r.text[r.i]
}

func (r *cljReader) get() rune {
	ch := r.text[r.i]
	r.i++
	if ch == '\n' {
		r.line++
	}
	return ch
}

func (r *cljReader) skipWS() {
	for !r.eof() {
		ch := r.peek()
		if isWS(ch) {
			r.get()
			continue
		}
		if ch == ';' {
			for !r.eof() && r.peek() != '\n' {
				r.get()
			}
			continue
		}
		break
	}
}

func (r *cljReader) fail(line int, msg string) { panic(&readError{line, msg}) }

func (r *cljReader) readToken() string {
	start, line := r.i, r.line
	for !r.eof() && !isTerminator(r.peek()) {
		r.get()
	}
	if r.i == start {
		r.fail(line, "expected a symbol")
	}
	return string(r.text[start:r.i])
}

func (r *cljReader) readString() *Form {
	line := r.line
	r.get()
	for !r.eof() {
		ch := r.get()
		if ch == '\\' {
			if r.eof() {
				break
			}
			r.get()
			continue
		}
		if ch == '"' {
			return &Form{Kind: FormLit, Line: line}
		}
	}
	r.fail(line, "unterminated string")
	return nil
}

func (r *cljReader) readChar() *Form {
	line := r.line
	r.get()
	if r.eof() {
		r.fail(line, "incomplete character")
	}
	r.get()
	for !r.eof() && !isTerminator(r.peek()) {
		r.get()
	}
	return &Form{Kind: FormLit, Line: line}
}

func (r *cljReader) readKeyword() *Form {
	line := r.line
	r.get()
	if r.peek() == ':' {
		r.get()
	}
	if r.eof() || isTerminator(r.peek()) {
		return &Form{Kind: FormKw, Line: line}
	}
	return &Form{Kind: FormKw, Name: r.readToken(), Line: line}
}

// readForm is read_form seen from a non-collection frame (top level, a
// wrapper, #_, #tag, #?, ^meta): skip whitespace, fail at EOF, read one
// form, and keep going while the form produced nothing.
func (r *cljReader) readForm() produced {
	for {
		r.skipWS()
		if r.eof() {
			r.fail(r.line, "unexpected end of file")
		}
		if p := r.open(); !p.need {
			return p
		}
	}
}

func (r *cljReader) open() produced {
	ch := r.peek()
	line := r.line
	switch {
	case ch == '(':
		r.get()
		return r.readColl(")", "list", line, false)
	case ch == '[':
		r.get()
		return r.readColl("]", "vector", line, false)
	case ch == '{':
		r.get()
		return r.readColl("}", "map", line, false)
	case ch == '"':
		return produced{form: r.readString()}
	case ch == '\\':
		return produced{form: r.readChar()}
	case ch == ':':
		return produced{form: r.readKeyword()}
	case ch == '\'' || ch == '@' || ch == '`':
		r.get()
		return r.wrap(map[rune]string{'\'': "quote", '@': "deref", '`': "syntax-quote"}[ch], line)
	case ch == '~':
		r.get()
		name := "unquote"
		if r.peek() == '@' {
			r.get()
			name = "unquote-splicing"
		}
		return r.wrap(name, line)
	case ch == '^':
		r.get()
		r.readForm() // metadata is read and dropped
		return produced{need: true}
	case ch == '#':
		return r.dispatch(line)
	}
	token := r.readToken()
	if cljNumber.MatchString(token) {
		return produced{form: &Form{Kind: FormLit, Line: line}}
	}
	return produced{form: &Form{Kind: FormSym, Name: token, Line: line}}
}

// readColl is the "coll" frame: items until the closer; nothing is
// dropped, a splice is spliced in.
func (r *cljReader) readColl(end, kind string, line int, fnStar bool) produced {
	closer := []rune(end)[0]
	var items []*Form
	for {
		r.skipWS()
		if r.eof() {
			r.fail(line, "unterminated "+kind)
		}
		if r.peek() == closer {
			r.get()
			return produced{form: closeColl(kind, line, items, fnStar)}
		}
		p := r.open()
		switch {
		case p.need, p.form == nil && !p.isSpl:
		case p.isSpl:
			items = append(items, p.splice...)
		default:
			items = append(items, p.form)
		}
	}
}

func closeColl(kind string, line int, items []*Form, fnStar bool) *Form {
	if fnStar {
		return &Form{Kind: FormColl, Coll: "list", Line: line,
			Items: append([]*Form{{Kind: FormSym, Name: "fn*", Line: line}}, items...)}
	}
	if kind == "map" {
		if len(items)%2 != 0 {
			panic(&readError{line, "map literal has an odd number of forms"})
		}
		pairs := make([][2]*Form, 0, len(items)/2)
		for i := 0; i < len(items); i += 2 {
			pairs = append(pairs, [2]*Form{items[i], items[i+1]})
		}
		return &Form{Kind: FormColl, Coll: "map", Line: line, Pairs: pairs}
	}
	return &Form{Kind: FormColl, Coll: kind, Line: line, Items: items}
}

// wrap is the "wrap" frame: (name form), or (name) when the form is
// nothing or a splice.
func (r *cljReader) wrap(name string, line int) produced {
	p := r.readForm()
	items := []*Form{{Kind: FormSym, Name: name, Line: line}}
	if p.form != nil && !p.isSpl {
		items = append(items, p.form)
	}
	return produced{form: &Form{Kind: FormColl, Coll: "list", Line: line, Items: items}}
}

func (r *cljReader) dispatch(line int) produced {
	r.get()
	ch := r.peek()
	switch ch {
	case -1:
		r.fail(line, "incomplete dispatch")
	case '_':
		r.get()
		r.readForm()
		return produced{}
	case '{':
		brace := r.line
		r.get()
		return r.readColl("}", "set", brace, false)
	case '(':
		r.get()
		return r.readColl(")", "list", line, true)
	case '"':
		r.readString()
		return produced{form: &Form{Kind: FormLit, Line: line}}
	case '?':
		r.get()
		splicing := r.peek() == '@'
		if splicing {
			r.get()
		}
		return r.finishCond(splicing, r.readForm())
	case ':':
		r.get()
		r.readMapQualifier()
		return produced{need: true}
	}
	r.readToken()
	r.readForm()
	return produced{form: &Form{Kind: FormLit, Line: line}}
}

func (r *cljReader) readMapQualifier() {
	ch := r.peek()
	if ch == ':' {
		r.get()
		return
	}
	if ch == -1 || ch == '{' || isWS(ch) {
		return
	}
	r.readToken()
}

func (r *cljReader) finishCond(splicing bool, p produced) produced {
	if p.form == nil || p.isSpl || p.form.Kind != FormColl || p.form.Coll != "list" {
		r.fail(r.line, "reader conditional body must be a list")
	}
	chosen := chosenBranch(p.form.Items)
	if chosen == nil {
		return produced{}
	}
	if !splicing {
		return produced{form: chosen}
	}
	if chosen.Kind != FormColl || (chosen.Coll != "list" && chosen.Coll != "vector") {
		r.fail(r.line, "splicing reader conditional needs a collection")
	}
	return produced{isSpl: true, splice: append([]*Form(nil), chosen.Items...)}
}

// chosenBranch is _chosen_branch: the first :clj or :default feature wins;
// a trailing odd feature is ignored.
func chosenBranch(items []*Form) *Form {
	for i := 0; i+1 < len(items); i += 2 {
		if f := items[i]; f.Kind == FormKw && (f.Name == "clj" || f.Name == "default") {
			return items[i+1]
		}
	}
	return nil
}

// ReadClojure is read_source: the top-level forms, plus a warning
// "<line>: <msg>" when a form could not be read (reading stops there).
func ReadClojure(text string) (forms []*Form, warning string) {
	text = strings.TrimPrefix(text, "\uFEFF")
	r := &cljReader{text: []rune(text), line: 1}
	if strings.HasPrefix(text, "#!") {
		for p := r.peek(); p != -1 && p != '\n'; p = r.peek() {
			r.get()
		}
	}
	defer func() {
		if rec := recover(); rec != nil {
			e, ok := rec.(*readError)
			if !ok {
				panic(rec)
			}
			warning = fmt.Sprintf("%d: %s", e.line, e.msg)
		}
	}()
	for {
		r.skipWS()
		if r.eof() {
			return forms, ""
		}
		p := r.readForm()
		if p.form != nil && !p.isSpl {
			forms = append(forms, p.form)
		}
	}
}

func formChildren(f *Form) []*Form {
	if f.Kind != FormColl {
		return nil
	}
	if f.Coll == "map" {
		out := make([]*Form, 0, 2*len(f.Pairs))
		for _, kv := range f.Pairs {
			out = append(out, kv[0], kv[1])
		}
		return out
	}
	return f.Items
}

// MaxLine is max_line: the largest line over the form and its descendants
// (closing delimiters carry no line).
func MaxLine(f *Form) int {
	best := max(f.Line, 1)
	stack := []*Form{f}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		best = max(best, cur.Line)
		stack = append(stack, formChildren(cur)...)
	}
	return best
}

// NormalizeClojure is clojure_read.normalize (dry4clj normalize-form).
func NormalizeClojure(f *Form) *Value { return normalizeClj(f, false) }

func normalizeClj(f *Form, head bool) *Value {
	switch f.Kind {
	case FormSym:
		if head {
			return symbolOf(f.Name)
		}
		return K("symbol")
	case FormKw:
		return K("keyword")
	case FormLit:
		return K("literal")
	}
	switch f.Coll {
	case "list":
		if len(f.Items) == 0 {
			return List(K("list"), K("literal"))
		}
		out := List(K("list"), normalizeClj(f.Items[0], true))
		for _, arg := range f.Items[1:] {
			out.Items = append(out.Items, normalizeClj(arg, false))
		}
		return out
	case "vector", "set":
		out := List(K(f.Coll))
		for _, item := range f.Items {
			out.Items = append(out.Items, normalizeClj(item, false))
		}
		return out
	case "map":
		out := List(K("map"))
		for _, kv := range f.Pairs {
			out.Items = append(out.Items, List(normalizeClj(kv[0], false), normalizeClj(kv[1], false)))
		}
		return out
	}
	return List(K("literal"))
}

// IsCandidateForm is is_candidate_form: a non-empty list whose head is not
// the symbol ns.
func IsCandidateForm(f *Form) bool {
	if f.Kind != FormColl || f.Coll != "list" || len(f.Items) == 0 {
		return false
	}
	h := f.Items[0]
	return !(h.Kind == FormSym && h.Name == "ns")
}
