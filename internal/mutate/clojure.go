package mutate

// Clojure sites. Upstream parses with tree-sitter-clojure; coderefinery reads the
// source with the small reader below, which emits the node types and spans
// the site rules look at (list_lit, sym_lit, bool_lit, num_lit, the quoting
// and comment nodes, ...). Metadata (`^m form`) is a meta_lit child of the
// form it decorates, so a decorated symbol's text includes it, as in
// tree-sitter-clojure.

import (
	"strings"
	"unicode/utf8"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

type headMutant struct{ mutant, category string }

var clojureHeads = map[string]headMutant{
	"+": {"-", "arithmetic"}, "-": {"+", "arithmetic"}, "*": {"/", "arithmetic"},
	"inc": {"dec", "arithmetic"}, "dec": {"inc", "arithmetic"},
	">": {">=", "comparison"}, ">=": {">", "comparison"}, "<": {"<=", "comparison"}, "<=": {"<", "comparison"},
	"=": {"not=", "equality"}, "not=": {"=", "equality"},
	"if": {"if-not", "conditional"}, "if-not": {"if", "conditional"},
	"when": {"when-not", "conditional"}, "when-not": {"when", "conditional"},
	"and": {"or", "logical"}, "or": {"and", "logical"},
	"double": {"int", "coercion"}, "int": {"double", "coercion"},
	"first": {"second", "seq"}, "second": {"first", "seq"},
	"filter": {"remove", "seq"}, "remove": {"filter", "seq"},
	"take": {"drop", "seq"}, "drop": {"take", "seq"},
	"rest": {"next", "seq"}, "next": {"rest", "seq"},
	"every?": {"some", "seq"}, "some": {"every?", "seq"},
	"min": {"max", "numeric"}, "max": {"min", "numeric"},
	"pos?": {"neg?", "predicate"}, "neg?": {"pos?", "predicate"},
	"even?": {"odd?", "predicate"}, "odd?": {"even?", "predicate"},
	"nil?": {"some?", "predicate"}, "some?": {"nil?", "predicate"},
}

// ---- reader ----

type cljReader struct {
	src []byte
	i   int
}

// parseClojure reads the whole file into a `source` node. Unbalanced or
// unreadable input still yields every form read so far.
func parseClojure(src []byte) *syntax.Node {
	r := &cljReader{src: src}
	root := &syntax.Node{Type: "source", Named: true, StartByte: 0, EndByte: len(src)}
	for {
		r.skipSpace()
		if r.i >= len(src) {
			break
		}
		if isCloser(src[r.i]) {
			r.i++ // stray closer: tree-sitter makes it an ERROR
			continue
		}
		root.Children = append(root.Children, r.form())
	}
	return syntax.Finish(root, src)
}

func isCloser(b byte) bool { return b == ')' || b == ']' || b == '}' }

func cljSpace(r rune) bool { return r == ',' || isPySpace(r) }

func (r *cljReader) peekRune() (rune, int) {
	if r.i >= len(r.src) {
		return 0, 0
	}
	return utf8.DecodeRune(r.src[r.i:])
}

func (r *cljReader) skipSpace() {
	for r.i < len(r.src) {
		c, n := r.peekRune()
		if !cljSpace(c) {
			return
		}
		r.i += n
	}
}

func (r *cljReader) startsWith(s string) bool {
	return strings.HasPrefix(string(r.src[r.i:min(len(r.src), r.i+len(s))]), s)
}

func leaf(typ string, start, end int) *syntax.Node { return syntax.Leaf(typ, true, start, end) }

func inner(typ string, start, end int, children ...*syntax.Node) *syntax.Node {
	return &syntax.Node{Type: typ, Named: true, StartByte: start, EndByte: end, Children: children}
}

// form reads one form (or a comment) starting at r.i (not whitespace).
func (r *cljReader) form() *syntax.Node {
	start := r.i
	c := r.src[r.i]
	switch c {
	case ';':
		for r.i < len(r.src) && r.src[r.i] != '\n' {
			r.i++
		}
		if r.i < len(r.src) {
			r.i++
		}
		return leaf("comment", start, r.i)
	case '(':
		return r.coll("list_lit", start, ')')
	case '[':
		return r.coll("vec_lit", start, ']')
	case '{':
		return r.coll("map_lit", start, '}')
	case '"':
		r.str()
		return leaf("str_lit", start, r.i)
	case '\\':
		r.char()
		return leaf("char_lit", start, r.i)
	case '\'':
		return r.prefixed("quoting_lit", start, 1)
	case '`':
		return r.prefixed("syn_quoting_lit", start, 1)
	case '@':
		return r.prefixed("derefing_lit", start, 1)
	case '~':
		if r.startsWith("~@") {
			return r.prefixed("unquote_splicing_lit", start, 2)
		}
		return r.prefixed("unquoting_lit", start, 1)
	case '^':
		return r.meta(start, 1)
	case '#':
		return r.dispatch(start)
	}
	return r.atom(start)
}

func (r *cljReader) coll(typ string, start int, closer byte) *syntax.Node {
	r.i++
	n := inner(typ, start, start)
	for {
		r.skipSpace()
		if r.i >= len(r.src) {
			n.EndByte = r.i
			return n
		}
		if r.src[r.i] == closer {
			r.i++
			n.EndByte = r.i
			return n
		}
		if isCloser(r.src[r.i]) {
			r.i++ // mismatched closer ends the collection
			n.EndByte = r.i
			return n
		}
		n.Children = append(n.Children, r.form())
	}
}

// nextForm reads the next non-comment form, or nil at the end of input.
func (r *cljReader) nextForm(into *syntax.Node) *syntax.Node {
	for {
		r.skipSpace()
		if r.i >= len(r.src) || isCloser(r.src[r.i]) {
			return nil
		}
		f := r.form()
		if f.Type != "comment" {
			return f
		}
		into.Children = append(into.Children, f)
	}
}

func (r *cljReader) prefixed(typ string, start, width int) *syntax.Node {
	r.i += width
	n := inner(typ, start, r.i)
	if f := r.nextForm(n); f != nil {
		n.Children = append(n.Children, f)
		n.EndByte = f.EndByte
	}
	return n
}

// meta reads `^m target`: the meta_lit becomes the first child of the
// target when the target can carry metadata.
func (r *cljReader) meta(start, width int) *syntax.Node {
	r.i += width
	m := inner("meta_lit", start, r.i)
	if f := r.nextForm(m); f != nil {
		m.Children = append(m.Children, f)
		m.EndByte = f.EndByte
	}
	target := r.nextForm(m)
	if target == nil {
		return m
	}
	switch target.Type {
	case "num_lit", "str_lit", "char_lit", "regex_lit":
		// tree-sitter-clojure rejects metadata here (an ERROR around it).
		return inner("ERROR", start, target.EndByte, m, target)
	case "bool_lit", "nil_lit":
		target = inner("sym_lit", target.StartByte, target.EndByte, leaf("sym_name", target.StartByte, target.EndByte))
	}
	target.Children = append([]*syntax.Node{m}, target.Children...)
	target.StartByte = start
	return target
}

func (r *cljReader) dispatch(start int) *syntax.Node {
	switch {
	case r.startsWith("#{"):
		r.i++
		n := r.coll("set_lit", r.i, '}')
		n.StartByte = start
		return n
	case r.startsWith("#("):
		r.i++
		n := r.coll("anon_fn_lit", r.i, ')')
		n.StartByte = start
		return n
	case r.startsWith(`#"`):
		r.i++
		r.str()
		return leaf("regex_lit", start, r.i)
	case r.startsWith("#'"):
		return r.prefixed("var_quoting_lit", start, 2)
	case r.startsWith("#_"):
		return r.prefixed("dis_expr", start, 2)
	case r.startsWith("#?@"):
		return r.prefixed("splicing_read_cond_lit", start, 3)
	case r.startsWith("#?"):
		return r.prefixed("read_cond_lit", start, 2)
	case r.startsWith("#^"):
		return r.meta(start, 2)
	case r.startsWith("#="):
		return r.prefixed("evaling_lit", start, 2)
	case r.startsWith("##"):
		return r.prefixed("sym_val_lit", start, 2)
	case r.startsWith("#!"):
		for r.i < len(r.src) && r.src[r.i] != '\n' {
			r.i++
		}
		return leaf("comment", start, r.i)
	case r.startsWith("#:"):
		r.i++
		n := inner("ns_map_lit", start, r.i)
		if prefix := r.nextForm(n); prefix != nil {
			n.Children = append(n.Children, prefix)
		}
		if body := r.nextForm(n); body != nil {
			n.Children = append(n.Children, body)
			n.EndByte = body.EndByte
		}
		return n
	}
	// #tag form
	r.i++
	n := inner("tagged_or_ctor_lit", start, r.i)
	if tag := r.nextForm(n); tag != nil {
		n.Children = append(n.Children, tag)
		n.EndByte = tag.EndByte
		if body := r.nextForm(n); body != nil {
			n.Children = append(n.Children, body)
			n.EndByte = body.EndByte
		}
	}
	return n
}

func (r *cljReader) str() {
	r.i++ // opening quote
	for r.i < len(r.src) {
		switch r.src[r.i] {
		case '\\':
			r.i += 2
		case '"':
			r.i++
			return
		default:
			r.i++
		}
	}
	r.i = len(r.src)
}

func atomEnd(c rune) bool {
	return cljSpace(c) || strings.ContainsRune("()[]{}\";", c)
}

// char reads `\x`, `\newline`, `é`: one character, then up to a
// delimiter.
func (r *cljReader) char() {
	r.i++
	if _, n := r.peekRune(); n > 0 {
		r.i += n
	}
	for r.i < len(r.src) {
		c, n := r.peekRune()
		if atomEnd(c) {
			return
		}
		r.i += n
	}
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func (r *cljReader) atom(start int) *syntax.Node {
	for r.i < len(r.src) {
		c, n := r.peekRune()
		if atomEnd(c) {
			break
		}
		r.i += n
	}
	if r.i == start {
		r.i++ // unreadable byte
		return leaf("ERROR", start, r.i)
	}
	text := string(r.src[start:r.i])
	switch {
	case isDigit(text[0]) || (text[0] == '+' || text[0] == '-') && len(text) > 1 && isDigit(text[1]):
		return leaf("num_lit", start, r.i)
	case text == "true" || text == "false":
		return leaf("bool_lit", start, r.i)
	case text == "nil":
		return leaf("nil_lit", start, r.i)
	case text[0] == ':':
		return inner("kwd_lit", start, r.i, leaf("kwd_name", start, r.i))
	}
	return inner("sym_lit", start, r.i, leaf("sym_name", start, r.i))
}

// ---- site rules (sites._clojure_*) ----

func clojureHead(n *syntax.Node, data []byte) (string, bool) {
	for _, c := range n.Children {
		if c.Type == "sym_lit" {
			return c.Text(data), true
		}
	}
	return "", false
}

func isHead(n *syntax.Node) bool {
	p := n.Parent
	if p == nil || p.Type != "list_lit" {
		return false
	}
	for _, c := range p.Children {
		if c.Type == "sym_lit" {
			return c == n
		}
	}
	return false
}

func clojureSkipped(n *syntax.Node, data []byte) bool {
	for cur := n; cur != nil; cur = cur.Parent {
		for _, part := range []string{"comment", "quot", "string", "str_lit", "char_lit"} {
			if strings.Contains(cur.Type, part) {
				return true
			}
		}
		if cur.Type == "list_lit" {
			if head, ok := clojureHead(cur, data); ok && (head == "comment" || head == "quote") {
				return true
			}
		}
	}
	return false
}

func (f *siteFinder) clojureNode(n *syntax.Node) {
	if clojureSkipped(n, f.data) {
		return
	}
	text := ""
	if len(n.Children) == 0 || n.Type == "sym_lit" || n.Type == "bool_lit" || n.Type == "num_lit" {
		text = n.Text(f.data)
	}
	if n.Type == "sym_lit" && isHead(n) {
		if h, ok := clojureHeads[text]; ok {
			f.add(n, text, h.mutant, h.category)
			return
		}
	}
	if n.Type == "bool_lit" && booleans[text] != "" {
		f.add(n, text, booleans[text], "boolean")
		return
	}
	if n.Type == "num_lit" && (text == "0" || text == "1") {
		f.add(n, text, flip01(text), "constant")
	}
}
