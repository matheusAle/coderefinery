package rust

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/matheusAle/ub-mode/ubgate/internal/syntax"
)

// The lexer is lazy: the parser asks for the token at a byte offset, so it
// can re-lex from the middle of a compound token (`>>` inside generics,
// `||` as closure bars, `&&` as two references), exactly as tree-sitter's
// state-dependent lexer does. Whitespace and comments (the extras) are
// skipped; comments are recorded once by start offset and attached after
// parsing (extras.go). Floats follow tree-sitter-rust's external scanner;
// the parser asks for an integer instead in tuple-index position.

type tkind int

const (
	tkEOF tkind = iota
	tkIdent
	tkPunct
	tkInt
	tkFloat
	tkChar
	tkString    // string_literal, node prebuilt
	tkRawString // raw_string_literal, node prebuilt
)

type token struct {
	kind       tkind
	text       string
	start, end int
	node       *syntax.Node
}

func (t token) is(s string) bool {
	return (t.kind == tkPunct || t.kind == tkIdent) && t.text == s
}

type parseError struct {
	pos int
	msg string
}

func (e *parseError) Error() string { return fmt.Sprintf("rust: offset %d: %s", e.pos, e.msg) }

func fail(pos int, format string, args ...any) {
	panic(&parseError{pos: pos, msg: fmt.Sprintf(format, args...)})
}

type lexer struct {
	src      []byte
	comments map[int]*syntax.Node
	cache    map[int]token
	skips    map[int]int
}

func newLexer(src []byte) *lexer {
	return &lexer{src: src, comments: map[int]*syntax.Node{}, cache: map[int]token{}, skips: map[int]int{}}
}

// puncts, longest first within each leading byte.
var puncts = []string{
	">>=", "<<=", "...", "..=",
	"::", "->", "=>", "==", "!=", "<=", ">=", "&&", "||", "+=", "-=", "*=", "/=", "%=", "^=", "&=", "|=", "<<", ">>", "..",
	"+", "-", "*", "/", "%", "^", "!", "&", "|", "=", "<", ">", "@", ".", ",", ";", ":", "#", "$", "?", "(", ")", "[", "]", "{", "}", "~", "'",
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.Is(unicode.Nl, r) || unicode.Is(unicode.Other_ID_Start, r)
}

func isIdentContinue(r rune) bool {
	return isIdentStart(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) ||
		unicode.Is(unicode.Pc, r) || unicode.Is(unicode.Nd, r) || unicode.Is(unicode.Other_ID_Continue, r)
}

func (lx *lexer) runeAt(off int) (rune, int) {
	if off >= len(lx.src) {
		return -1, 0
	}
	return utf8.DecodeRune(lx.src[off:])
}

func (lx *lexer) byteAt(off int) byte {
	if off < 0 || off >= len(lx.src) {
		return 0
	}
	return lx.src[off]
}

// skip returns the offset of the first non-trivia byte at or after off.
func (lx *lexer) skip(off int) int {
	if v, ok := lx.skips[off]; ok {
		return v
	}
	start := off
	for off < len(lx.src) {
		r, w := lx.runeAt(off)
		if unicode.IsSpace(r) {
			off += w
			continue
		}
		if r == '/' && lx.byteAt(off+1) == '/' {
			off = lx.lineComment(off)
			continue
		}
		if r == '/' && lx.byteAt(off+1) == '*' {
			off = lx.blockComment(off)
			continue
		}
		break
	}
	lx.skips[start] = off
	return off
}

func (lx *lexer) record(n *syntax.Node) {
	n.Extra = true
	lx.comments[n.StartByte] = n
}

func (lx *lexer) lineComment(off int) int {
	end := off
	for end < len(lx.src) && lx.src[end] != '\n' {
		end++
	}
	open := syntax.Leaf("//", false, off, off+2)
	c3, c4 := lx.byteAt(off+2), lx.byteAt(off+3)
	if (c3 == '/' && c4 != '/') || c3 == '!' {
		if end < len(lx.src) {
			end++ // doc content keeps its newline
		}
		typ, text := "outer_doc_comment_marker", "/"
		if c3 == '!' {
			typ, text = "inner_doc_comment_marker", "!"
		}
		field := "outer"
		if c3 == '!' {
			field = "inner"
		}
		marker := syntax.Inner(typ, off+2, syntax.Leaf(text, false, off+2, off+3)).WithField(field)
		doc := syntax.Leaf("doc_comment", true, off+3, end).WithField("doc")
		lx.record(syntax.Inner("line_comment", off, open, marker, doc))
		return end
	}
	n := syntax.Inner("line_comment", off, open)
	n.EndByte = end
	lx.record(n)
	return end
}

func (lx *lexer) blockComment(off int) int {
	depth := 0
	i := off
	for i < len(lx.src) {
		if lx.src[i] == '/' && lx.byteAt(i+1) == '*' {
			depth++
			i += 2
			continue
		}
		if lx.src[i] == '*' && lx.byteAt(i+1) == '/' {
			depth--
			i += 2
			if depth == 0 {
				break
			}
			continue
		}
		i++
	}
	if depth != 0 {
		fail(off, "unterminated block comment")
	}
	end := i
	children := []*syntax.Node{syntax.Leaf("/*", false, off, off+2)}
	c2, c3 := lx.byteAt(off+2), lx.byteAt(off+3)
	isOuter := c2 == '*' && c3 != '*' && c3 != '/'
	isInner := c2 == '!'
	if isOuter || isInner {
		typ, field := "outer_doc_comment_marker", "outer"
		if isInner {
			typ, field = "inner_doc_comment_marker", "inner"
		}
		children = append(children, syntax.Leaf(typ, true, off+2, off+3).WithField(field))
		if off+3 < end-2 {
			children = append(children, syntax.Leaf("doc_comment", true, off+3, end-2).WithField("doc"))
		}
	}
	children = append(children, syntax.Leaf("*/", false, end-2, end))
	lx.record(syntax.Inner("block_comment", off, children...))
	return end
}

// at returns the token at the first non-trivia offset >= off, with floats
// recognized (the default: float_literal is valid wherever a literal is).
func (lx *lexer) at(off int) token {
	off = lx.skip(off)
	if t, ok := lx.cache[off]; ok {
		return t
	}
	t := lx.lex(off, true)
	lx.cache[off] = t
	return t
}

func (lx *lexer) lex(off int, floats bool) token {
	if off >= len(lx.src) {
		return token{kind: tkEOF, start: off, end: off}
	}
	c := lx.src[off]
	switch {
	case c >= '0' && c <= '9':
		return lx.number(off, floats)
	case c == '"':
		return lx.str(off, off+1)
	case c == '\'':
		if end := lx.charEnd(off); end > 0 {
			return token{kind: tkChar, text: string(lx.src[off:end]), start: off, end: end}
		}
		return token{kind: tkPunct, text: "'", start: off, end: off + 1}
	}
	r, _ := lx.runeAt(off)
	if isIdentStart(r) {
		if t, ok := lx.prefixed(off); ok {
			return t
		}
		return lx.ident(off)
	}
	for _, p := range puncts {
		if strings.HasPrefix(string(lx.src[off:min(len(lx.src), off+3)]), p) {
			return token{kind: tkPunct, text: p, start: off, end: off + len(p)}
		}
	}
	fail(off, "unexpected character %q", r)
	return token{}
}

func (lx *lexer) ident(off int) token {
	i := off
	if lx.byteAt(i) == 'r' && lx.byteAt(i+1) == '#' {
		if r, _ := lx.runeAt(i + 2); isIdentStart(r) {
			i += 2
		}
	}
	for i < len(lx.src) {
		r, w := lx.runeAt(i)
		if !isIdentContinue(r) {
			break
		}
		i += w
	}
	return token{kind: tkIdent, text: string(lx.src[off:i]), start: off, end: i}
}

// prefixed recognizes b"", c"", r"", r#""#, br"", cr"" and b'x'.
func (lx *lexer) prefixed(off int) (token, bool) {
	c := lx.src[off]
	i := off + 1
	if (c == 'b' || c == 'c') && lx.byteAt(i) == '"' {
		return lx.str(off, i+1), true
	}
	if c == 'b' && lx.byteAt(i) == '\'' {
		if end := lx.charEnd(i); end > 0 {
			return token{kind: tkChar, text: string(lx.src[off:end]), start: off, end: end}, true
		}
	}
	if (c == 'b' || c == 'c') && lx.byteAt(i) == 'r' {
		i++
	} else if c != 'r' {
		return token{}, false
	}
	j := i
	for lx.byteAt(j) == '#' {
		j++
	}
	if lx.byteAt(j) != '"' {
		return token{}, false
	}
	hashes := j - i
	contentStart := j + 1
	closing := "\"" + strings.Repeat("#", hashes)
	k := strings.Index(string(lx.src[contentStart:]), closing)
	if k < 0 {
		fail(off, "unterminated raw string")
	}
	contentEnd := contentStart + k
	end := contentEnd + len(closing)
	// The scanner skips leading whitespace before the content token.
	for contentStart < contentEnd {
		r, w := lx.runeAt(contentStart)
		if !unicode.IsSpace(r) {
			break
		}
		contentStart += w
	}
	content := syntax.Leaf("string_content", true, contentStart, contentEnd)
	n := syntax.Inner("raw_string_literal", off, content)
	n.StartByte, n.EndByte = off, end
	return token{kind: tkRawString, start: off, end: end, node: n}, true
}

// str lexes a string literal whose opening token is [off, body).
func (lx *lexer) str(off, body int) token {
	children := []*syntax.Node{syntax.Leaf(`"`, false, off, body)}
	i := body
	run := i
	flush := func() {
		if i > run {
			children = append(children, syntax.Leaf("string_content", true, run, i))
		}
	}
	for {
		if i >= len(lx.src) {
			fail(off, "unterminated string")
		}
		c := lx.src[i]
		if c == '"' {
			flush()
			children = append(children, syntax.Leaf(`"`, false, i, i+1))
			n := syntax.Inner("string_literal", off, children...)
			return token{kind: tkString, start: off, end: i + 1, node: n}
		}
		if c == '\\' {
			flush()
			end := lx.escapeEnd(i)
			if end < 0 {
				fail(i, "bad escape")
			}
			children = append(children, syntax.Leaf("escape_sequence", true, i, end))
			i = end
			run = i
			continue
		}
		i++
	}
}

func isHex(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

// escapeEnd matches `\` (/[^xu]/ | /u[0-9a-fA-F]{4}/ | /u\{[0-9a-fA-F]+\}/ |
// /x[0-9a-fA-F]{2}/) at i and returns its end, or -1.
func (lx *lexer) escapeEnd(i int) int {
	if i+1 >= len(lx.src) {
		return -1
	}
	switch lx.src[i+1] {
	case 'x':
		if isHex(lx.byteAt(i+2)) && isHex(lx.byteAt(i+3)) {
			return i + 4
		}
		return -1
	case 'u':
		if lx.byteAt(i+2) == '{' {
			j := i + 3
			for isHex(lx.byteAt(j)) {
				j++
			}
			if j > i+3 && lx.byteAt(j) == '}' {
				return j + 1
			}
		}
		if isHex(lx.byteAt(i+2)) && isHex(lx.byteAt(i+3)) && isHex(lx.byteAt(i+4)) && isHex(lx.byteAt(i+5)) {
			return i + 6
		}
		return -1
	}
	_, w := lx.runeAt(i + 1)
	return i + 1 + w
}

// charEnd matches a char literal body at the quote q, returning its end or 0.
func (lx *lexer) charEnd(q int) int {
	i := q + 1
	if i >= len(lx.src) {
		return 0
	}
	switch lx.src[i] {
	case '\\':
		end := lx.escapeEnd(i)
		if end < 0 {
			return 0
		}
		i = end
	case '\'':
		// '' : the grammar's optional body is empty.
		return q + 2
	default:
		_, w := lx.runeAt(i)
		i += w
	}
	if lx.byteAt(i) == '\'' {
		return i + 1
	}
	return 0
}

var numericSuffixes = []string{"u128", "i128", "usize", "isize", "u16", "i16", "u32", "i32", "u64", "i64", "f32", "f64", "u8", "i8"}

func isNum(c byte) bool { return c == '_' || c >= '0' && c <= '9' }

func (lx *lexer) number(off int, floats bool) token {
	if floats {
		if end, ok := lx.float(off); ok {
			return token{kind: tkFloat, text: string(lx.src[off:end]), start: off, end: end}
		}
	}
	end := lx.integerEnd(off)
	return token{kind: tkInt, text: string(lx.src[off:end]), start: off, end: end}
}

func (lx *lexer) integerEnd(off int) int {
	i := off + 1
	digits := func(ok func(byte) bool) int {
		j := off + 2
		for j < len(lx.src) && ok(lx.src[j]) {
			j++
		}
		if j > off+2 {
			return j
		}
		return -1
	}
	best := off + 1
	for i < len(lx.src) && isNum(lx.src[i]) {
		i++
	}
	best = i
	if lx.src[off] == '0' {
		var j int
		switch lx.byteAt(off + 1) {
		case 'x':
			j = digits(func(c byte) bool { return isHex(c) || c == '_' })
		case 'b':
			j = digits(func(c byte) bool { return c == '0' || c == '1' || c == '_' })
		case 'o':
			j = digits(func(c byte) bool { return c >= '0' && c <= '7' || c == '_' })
		default:
			j = -1
		}
		if j > best {
			best = j
		}
	}
	for _, s := range numericSuffixes {
		if strings.HasPrefix(string(lx.src[best:min(len(lx.src), best+5)]), s) {
			return best + len(s)
		}
	}
	return best
}

// float is tree-sitter-rust's process_float_literal.
func (lx *lexer) float(off int) (int, bool) {
	i := off + 1
	for isNum(lx.byteAt(i)) {
		i++
	}
	hasFraction, hasExponent := false, false
	if lx.byteAt(i) == '.' {
		hasFraction = true
		i++
		r, _ := lx.runeAt(i)
		if r >= 0 && unicode.IsLetter(r) {
			return 0, false
		}
		if lx.byteAt(i) == '.' {
			return 0, false
		}
		for isNum(lx.byteAt(i)) {
			i++
		}
	}
	end := i
	if c := lx.byteAt(i); c == 'e' || c == 'E' {
		hasExponent = true
		i++
		if c := lx.byteAt(i); c == '+' || c == '-' {
			i++
		}
		if !isNum(lx.byteAt(i)) {
			return end, true
		}
		i++
		for isNum(lx.byteAt(i)) {
			i++
		}
		end = i
	}
	if !hasExponent && !hasFraction {
		return 0, false
	}
	if c := lx.byteAt(i); c != 'u' && c != 'i' && c != 'f' {
		return end, true
	}
	i++
	if c := lx.byteAt(i); c < '0' || c > '9' {
		return end, true
	}
	for c := lx.byteAt(i); c >= '0' && c <= '9'; c = lx.byteAt(i) {
		i++
	}
	return i, true
}
