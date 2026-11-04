package python

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The lexer turns the whole file into one flat token slice, emulating both
// tree-sitter-python's internal lexer and its external scanner
// (src/scanner.c): zero-width NEWLINE / INDENT / DEDENT tokens placed where
// the scanner places them, string pieces (string_start, content runs,
// escape_sequence, escape_interpolation, string_end), f-string interpolation
// tokens, comments and line continuations (the two extras).

type tokKind int

const (
	tkName tokKind = iota
	tkNumber
	tkOp
	tkComment
	tkLineCont
	tkNewline
	tkIndent
	tkDedent
	tkEOF
	tkStringStart
	tkStringEnd
	tkStrPiece   // typ: "" (hidden content), "escape_sequence", "escape_interpolation"
	tkTypeConv   // !r inside an interpolation
	tkFormatText // hidden format-spec text
)

type token struct {
	kind  tokKind
	typ   string // tkNumber: integer|float; tkStrPiece: see above
	text  string
	start int
	end   int
}

func (t token) isOp(s string) bool   { return t.kind == tkOp && t.text == s }
func (t token) isName(s string) bool { return t.kind == tkName && t.text == s }

type parseError struct {
	pos int
	msg string
}

func (e *parseError) Error() string { return fmt.Sprintf("python: offset %d: %s", e.pos, e.msg) }

func fail(pos int, format string, args ...any) {
	panic(&parseError{pos: pos, msg: fmt.Sprintf(format, args...)})
}

type lexer struct {
	src    string
	pos    int
	toks   []token
	indent []int
	depth  int // bracket depth outside strings

	lineHasTokens bool
	lastSig       token
	scanPos       int  // where tree-sitter's scanner would start: end of last token or comment
	pendingDedent bool // a dedent was held back by a comment at the block's indentation
}

var ops3 = []string{"**=", "//=", ">>=", "<<=", "..."}
var ops2 = []string{"!=", "%=", "&=", "**", "*=", "+=", "-=", "->", "//", "/=", ":=", "<<", "<=", "<>", "==", ">=", ">>", "@=", "^=", "|="}

const ops1 = "!%&()*+,-./:;<=>@[]^{|}~"

func lex(src string) (toks []token, err error) {
	defer func() {
		if r := recover(); r != nil {
			pe, ok := r.(*parseError)
			if !ok {
				panic(r)
			}
			err = pe
		}
	}()
	lx := &lexer{src: src, indent: []int{0}}
	lx.run()
	return lx.toks, nil
}

func (lx *lexer) emit(t token) {
	lx.toks = append(lx.toks, t)
}

func (lx *lexer) peekAt(i int) byte {
	if i < len(lx.src) {
		return lx.src[i]
	}
	return 0
}

func (lx *lexer) run() {
	src := lx.src
	for {
		// inline whitespace
		for lx.pos < len(src) {
			c := src[lx.pos]
			if c == ' ' || c == '\t' || c == '\f' || c == '\r' {
				lx.pos++
				continue
			}
			if c == '\n' && lx.depth > 0 {
				lx.pos++
				continue
			}
			break
		}
		if lx.pos >= len(src) {
			lx.lineEnd(true)
			lx.emit(token{kind: tkEOF, start: len(src), end: len(src)})
			return
		}
		c := src[lx.pos]
		switch {
		case c == '\n':
			lx.lineEnd(false)
			lx.pos++
		case c == '#':
			end := strings.IndexByte(src[lx.pos:], '\n')
			if end < 0 {
				end = len(src)
			} else {
				end += lx.pos
			}
			lx.emit(token{kind: tkComment, text: src[lx.pos:end], start: lx.pos, end: end})
			lx.pos = end
			lx.scanPos = end
		case c == '\\' && (lx.peekAt(lx.pos+1) == '\n' || lx.pos+1 >= len(src)):
			end := min(lx.pos+2, len(src))
			// The external scanner skips "\\\n" as whitespace before it tries
			// STRING_START, so a continuation right before a string becomes
			// that token's padding and never shows up as an extra.
			if !lx.stringFollows(end) {
				lx.emit(token{kind: tkLineCont, text: src[lx.pos:end], start: lx.pos, end: end})
			}
			lx.pos = end
		default:
			lx.token()
		}
	}
}

// lineEnd runs at a physical newline (or EOF) outside brackets.
func (lx *lexer) lineEnd(eof bool) {
	if lx.depth > 0 && !eof {
		return
	}
	if !lx.lineHasTokens {
		if lx.pendingDedent {
			lx.dedents(lx.lookahead())
		}
		if eof {
			for len(lx.indent) > 1 {
				lx.indent = lx.indent[:len(lx.indent)-1]
				lx.emit(token{kind: tkDedent, start: lx.scanPos, end: lx.scanPos})
			}
		}
		return
	}
	lx.lineHasTokens = false
	next, firstComment := lx.lookahead()
	top := lx.indent[len(lx.indent)-1]
	if lx.lastSig.isOp(":") && next > top {
		lx.indent = append(lx.indent, next)
		lx.emit(token{kind: tkIndent, start: lx.scanPos, end: lx.scanPos})
		return
	}
	lx.emit(token{kind: tkNewline, start: lx.scanPos, end: lx.scanPos})
	lx.dedents(next, firstComment)
	if eof {
		for len(lx.indent) > 1 {
			lx.indent = lx.indent[:len(lx.indent)-1]
			lx.emit(token{kind: tkDedent, start: lx.scanPos, end: lx.scanPos})
		}
	}
}

func (lx *lexer) dedents(next, firstComment int) {
	lx.pendingDedent = false
	for len(lx.indent) > 1 {
		top := lx.indent[len(lx.indent)-1]
		if next >= top {
			return
		}
		if firstComment >= top {
			lx.pendingDedent = true
			return
		}
		lx.indent = lx.indent[:len(lx.indent)-1]
		lx.emit(token{kind: tkDedent, start: lx.scanPos, end: lx.scanPos})
	}
}

// lookahead mirrors the scanner's whitespace loop from the current newline:
// the indentation of the next line with a real token (0 at EOF) and the
// indentation of the first comment line met on the way (-1 if none).
func (lx *lexer) lookahead() (int, int) {
	src := lx.src
	i := lx.pos
	indent := 0
	firstComment := -1
	for i < len(src) {
		switch c := src[i]; c {
		case '\n', '\r', '\f':
			indent = 0
			i++
		case ' ':
			indent++
			i++
		case '\t':
			indent += 8
			i++
		case '#':
			if firstComment == -1 {
				firstComment = indent
			}
			for i < len(src) && src[i] != '\n' {
				i++
			}
			indent = 0
		case '\\':
			if i+1 < len(src) && src[i+1] == '\n' {
				i += 2
				continue
			}
			if i+1 >= len(src) {
				return 0, firstComment
			}
			return indent, firstComment
		default:
			return indent, firstComment
		}
	}
	return 0, firstComment
}

func isIdentStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.Is(unicode.Nl, r) || unicode.Is(unicode.Other_ID_Start, r)
}

func isIdentContinue(r rune) bool {
	return isIdentStart(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) ||
		unicode.Is(unicode.Pc, r) || unicode.Is(unicode.Nd, r) || unicode.Is(unicode.Other_ID_Continue, r)
}

func isStringPrefix(s string) bool {
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune("rRbBuUfFtT", rune(s[i])) {
			return false
		}
	}
	return true
}

// token lexes one significant token at lx.pos (depth tracking included).
func (lx *lexer) token() {
	t := lx.next(&lx.depth)
	lx.lineHasTokens = true
	lx.lastSig = t
	lx.scanPos = lx.pos
}

// next lexes one significant token (or a whole string) and appends it (and,
// for strings, its pieces). depth is the bracket depth to update.
func (lx *lexer) next(depth *int) token {
	src := lx.src
	start := lx.pos
	c := src[start]
	r, size := utf8.DecodeRuneInString(src[start:])
	switch {
	case c == '"' || c == '\'':
		return lx.str(start, start)
	case isIdentStart(r):
		i := start + size
		for i < len(src) {
			r2, s2 := utf8.DecodeRuneInString(src[i:])
			if !isIdentContinue(r2) {
				break
			}
			i += s2
		}
		if i < len(src) && (src[i] == '"' || src[i] == '\'') && isStringPrefix(src[start:i]) {
			return lx.str(start, i)
		}
		t := token{kind: tkName, text: src[start:i], start: start, end: i}
		lx.emit(t)
		lx.pos = i
		return t
	case c >= '0' && c <= '9' || c == '.' && start+1 < len(src) && src[start+1] >= '0' && src[start+1] <= '9':
		return lx.number(start)
	}
	for _, group := range [][]string{ops3, ops2} {
		for _, op := range group {
			if strings.HasPrefix(src[start:], op) {
				return lx.op(start, op, depth)
			}
		}
	}
	if strings.IndexByte(ops1, c) >= 0 {
		return lx.op(start, src[start:start+1], depth)
	}
	fail(start, "unexpected character %q", r)
	return token{}
}

func (lx *lexer) op(start int, op string, depth *int) token {
	switch op {
	case "(", "[", "{":
		*depth++
	case ")", "]", "}":
		*depth--
	}
	t := token{kind: tkOp, text: op, start: start, end: start + len(op)}
	lx.emit(t)
	lx.pos = t.end
	return t
}

func isDigitRun(src string, i int, ok func(byte) bool) int {
	// repeat1(/_?[x]+/) or repeat1(/[0-9]+_?/) — digits with single underscores.
	for i < len(src) && (ok(src[i]) || src[i] == '_' && i+1 < len(src) && ok(src[i+1])) {
		i++
	}
	return i
}

func isDec(b byte) bool { return b >= '0' && b <= '9' }
func isHex(b byte) bool { return isDec(b) || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F' }

func (lx *lexer) number(start int) token {
	src := lx.src
	i := start
	typ := "integer"
	lower := func(j int) byte {
		if j < len(src) {
			return src[j] | 0x20
		}
		return 0
	}
	if src[i] == '0' && (lower(i+1) == 'x' || lower(i+1) == 'o' || lower(i+1) == 'b') {
		var ok func(byte) bool
		switch lower(i + 1) {
		case 'x':
			ok = isHex
		case 'o':
			ok = func(b byte) bool { return b >= '0' && b <= '7' }
		default:
			ok = func(b byte) bool { return b == '0' || b == '1' }
		}
		j := i + 2
		if j < len(src) && src[j] == '_' {
			j++
		}
		i = isDigitRun(src, j, ok)
		if lower(i) == 'l' {
			i++
		}
	} else {
		decRun := func(j int) int {
			for j < len(src) && isDec(src[j]) {
				for j < len(src) && isDec(src[j]) {
					j++
				}
				if j < len(src) && src[j] == '_' {
					j++
				}
			}
			return j
		}
		i = decRun(i)
		if i < len(src) && src[i] == '.' {
			typ = "float"
			i = decRun(i + 1)
		}
		if lower(i) == 'e' {
			j := i + 1
			if j < len(src) && (src[j] == '+' || src[j] == '-') {
				j++
			}
			if j < len(src) && isDec(src[j]) {
				typ = "float"
				i = decRun(j)
			}
		}
		if lower(i) == 'l' || lower(i) == 'j' {
			i++
		}
	}
	t := token{kind: tkNumber, typ: typ, text: src[start:i], start: start, end: i}
	lx.emit(t)
	lx.pos = i
	return t
}

// str lexes a string literal whose prefix starts at start and whose opening
// quote is at q. It emits string_start, pieces, interpolation tokens and
// string_end, and returns the string_start token.
func (lx *lexer) str(start, q int) token {
	src := lx.src
	prefix := src[start:q]
	format := strings.ContainsAny(prefix, "fFtT")
	raw := strings.ContainsAny(prefix, "rR")
	bytesLit := strings.ContainsAny(prefix, "bB")
	quote := src[q]
	triple := strings.HasPrefix(src[q:], strings.Repeat(string(quote), 3))
	open := q + 1
	if triple {
		open = q + 3
	}
	st := token{kind: tkStringStart, text: src[start:open], start: start, end: open}
	lx.emit(st)
	i := open
	piece := func(typ string, a, b int) {
		lx.emit(token{kind: tkStrPiece, typ: typ, text: src[a:b], start: a, end: b})
	}
	contentStart := -1
	hasContent := false // scanner.c's has_content: raw "\\x" skips do not set it
	flush := func() {
		if contentStart >= 0 && contentStart < i {
			piece("", contentStart, i)
		}
		contentStart = -1
		hasContent = false
	}
	// relexRaw: raw backslash pairs right before "{" or "}" in an f-string.
	// The scanner returns no content token there, so the internal lexer
	// re-reads them as escape_sequence / "\\" tokens.
	relexRaw := func() {
		if contentStart >= 0 && !hasContent {
			// Raw backslash pairs right before "{": the scanner returns no
			// content token, so the internal lexer re-reads them as
			// escape_sequence / "\\" tokens.
			for j := contentStart; j < i; {
				switch n := escapeLen(src, j); {
				case src[j] != '\\':
					k := j
					for k < i && src[k] != '\\' {
						k++
					}
					piece("", j, k)
					j = k
				case n > 0 && j+n <= i:
					piece("escape_sequence", j, j+n)
					j += n
				default:
					piece("\\", j, j+1)
					j++
				}
			}
			contentStart = -1
		}
	}
	for {
		if i >= len(src) {
			fail(start, "unterminated string")
		}
		c := src[i]
		switch {
		case c == quote && (!triple || strings.HasPrefix(src[i:], strings.Repeat(string(quote), 3))):
			endStart := i
			if contentStart >= 0 && !hasContent {
				// Only raw backslash pairs since the last token: the scanner
				// folds them into string_end.
				endStart = contentStart
				contentStart = -1
			}
			flush()
			end := i + 1
			if triple {
				end = i + 3
			}
			lx.emit(token{kind: tkStringEnd, text: src[endStart:end], start: endStart, end: end})
			lx.pos = end
			return st
		case format && (c == '{' || c == '}'):
			if raw {
				relexRaw()
			}
			if i+1 < len(src) && src[i+1] == c {
				flush()
				piece("escape_interpolation", i, i+2)
				i += 2
				continue
			}
			if c == '}' {
				if contentStart < 0 {
					contentStart = i
				}
				hasContent = true
				i++
				continue
			}
			flush()
			lx.pos = i
			lx.interpolation()
			i = lx.pos
		case c == '\\':
			if raw {
				if contentStart < 0 {
					contentStart = i
				}
				i++
				if i < len(src) && (src[i] == quote || src[i] == '\\') {
					i++
				}
				if i < len(src) && src[i] == '\n' {
					i++
				}
				continue
			}
			if bytesLit && i+1 < len(src) && strings.IndexByte("NuU", src[i+1]) >= 0 {
				if contentStart < 0 {
					contentStart = i
				}
				hasContent = true
				i += 2
				continue
			}
			flush()
			if n := escapeLen(src, i); n > 0 {
				piece("escape_sequence", i, i+n)
				i += n
			} else {
				piece("\\", i, i+1) // _not_escape_sequence: anonymous "\\"
				i++
			}
		case c == '\n' && !triple:
			fail(i, "newline in string")
		default:
			if contentStart < 0 {
				contentStart = i
			}
			hasContent = true
			i++
		}
	}
}

// escapeLen matches tree-sitter-python's escape_sequence at src[i] == '\\'.
func escapeLen(src string, i int) int {
	if i+1 >= len(src) {
		return 0
	}
	c := src[i+1]
	hexN := func(n int) int {
		if i+2+n > len(src) {
			return 0
		}
		for k := 0; k < n; k++ {
			if !isHex(src[i+2+k]) {
				return 0
			}
		}
		return 2 + n
	}
	switch {
	case c == 'u':
		return hexN(4)
	case c == 'U':
		return hexN(8)
	case c == 'x':
		return hexN(2)
	case isDec(c):
		n := 1
		for n < 3 && i+1+n < len(src) && isDec(src[i+1+n]) {
			n++
		}
		return 1 + n
	case c == '\n':
		return 2
	case strings.IndexByte("'\"abfrntv\\", c) >= 0:
		return 2
	case c == 'N' && i+2 < len(src) && src[i+2] == '{':
		j := i + 3
		for j < len(src) && src[j] != '}' {
			j++
		}
		if j < len(src) && j > i+3 {
			return j + 1 - i
		}
	}
	return 0
}

// interpolation lexes "{ expr [=] [!x] [:spec] }" starting at lx.pos == '{'.
func (lx *lexer) interpolation() {
	src := lx.src
	depth := 0
	lx.op(lx.pos, "{", &depth)
	depth = 0
	for {
		for lx.pos < len(src) && (src[lx.pos] == ' ' || src[lx.pos] == '\t' || src[lx.pos] == '\n' || src[lx.pos] == '\f') {
			lx.pos++
		}
		if lx.pos >= len(src) {
			fail(lx.pos, "unterminated interpolation")
		}
		c := src[lx.pos]
		if depth == 0 {
			switch {
			case c == '}':
				d := 0
				lx.op(lx.pos, "}", &d)
				return
			case c == '!' && lx.pos+1 < len(src) && src[lx.pos+1] >= 'a' && src[lx.pos+1] <= 'z':
				lx.emit(token{kind: tkTypeConv, text: src[lx.pos : lx.pos+2], start: lx.pos, end: lx.pos + 2})
				lx.pos += 2
				continue
			case c == ':' && !strings.HasPrefix(src[lx.pos:], ":="):
				d := 0
				lx.op(lx.pos, ":", &d)
				lx.formatSpec()
				return
			}
		}
		if c == '#' {
			fail(lx.pos, "comment in interpolation")
		}
		if c == '\\' && lx.pos+1 < len(src) && src[lx.pos+1] == '\n' {
			lx.emit(token{kind: tkLineCont, text: src[lx.pos : lx.pos+2], start: lx.pos, end: lx.pos + 2})
			lx.pos += 2
			continue
		}
		lx.next(&depth)
	}
}

// formatSpec lexes format-spec text and nested {…} up to and including the
// closing '}' of the interpolation.
func (lx *lexer) formatSpec() {
	src := lx.src
	textStart := -1
	flush := func() {
		if textStart >= 0 && textStart < lx.pos {
			lx.emit(token{kind: tkFormatText, text: src[textStart:lx.pos], start: textStart, end: lx.pos})
		}
		textStart = -1
	}
	for {
		if lx.pos >= len(src) {
			fail(lx.pos, "unterminated format spec")
		}
		switch src[lx.pos] {
		case '{':
			flush()
			lx.interpolation()
		case '}':
			flush()
			d := 0
			lx.op(lx.pos, "}", &d)
			return
		case '\n':
			fail(lx.pos, "newline in format spec")
		default:
			if textStart < 0 {
				textStart = lx.pos
			}
			lx.pos++
		}
	}
}

// stringFollows reports whether a string literal starts at i, after
// spaces, tabs, form feeds and further line continuations.
func (lx *lexer) stringFollows(i int) bool {
	src := lx.src
	for i < len(src) {
		switch {
		case src[i] == ' ' || src[i] == '\t' || src[i] == '\f':
			i++
			continue
		case src[i] == '\\' && i+1 < len(src) && src[i+1] == '\n':
			i += 2
			continue
		}
		break
	}
	j := i
	for j < len(src) && strings.IndexByte("rRbBuUfFtT", src[j]) >= 0 {
		j++
	}
	return j < len(src) && (src[j] == '"' || src[j] == '\'') && (j == i || isStringPrefix(src[i:j]))
}
