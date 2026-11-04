package typescript

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"github.com/matheusAle/coderefinery/internal/syntax"
)

// The lexer is driven by the parser: tokens are scanned on demand from a byte
// offset, so the parser can re-read a position in another mode (regex instead
// of division, template characters, JSX text, a single `>` out of `>>`) and
// backtrack by restoring an offset. Comments are collected as they are
// skipped; a restore truncates the list.

type tokKind int

const (
	tEOF tokKind = iota
	tIdent
	tPrivate // #name
	tNumber
	tString
	tPunct
	tHashbang
)

type token struct {
	kind  tokKind
	text  string
	start int
	end   int
	nl    bool // a line terminator sits between the previous token and this one
}

type parseError struct {
	pos int
	msg string
}

func (e *parseError) Error() string {
	return fmt.Sprintf("typescript: offset %d: %s", e.pos, e.msg)
}

func fail(pos int, format string, args ...any) {
	panic(&parseError{pos: pos, msg: fmt.Sprintf(format, args...)})
}

var puncts = []string{
	">>>=", "...", "===", "!==", "**=", "<<=", ">>=", ">>>", "&&=", "||=", "??=",
	"=>", "==", "!=", "<=", ">=", "&&", "||", "??", "?.", "++", "--", "+=", "-=", "*=", "/=",
	"%=", "&=", "|=", "^=", "<<", ">>", "**",
}

const punct1 = "{}()[];,<>+-*/%&|^!~?:=.@"

func isIDStart(r rune) bool {
	return r == '$' || r == '_' || r == '\\' || unicode.IsLetter(r) || unicode.Is(unicode.Nl, r) ||
		unicode.Is(unicode.Other_ID_Start, r)
}

func isIDPart(r rune) bool {
	return isIDStart(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) ||
		unicode.Is(unicode.Pc, r) || r == 0x200c || r == 0x200d || unicode.Is(unicode.Other_ID_Continue, r)
}

func isLineTerm(r rune) bool { return r == '\n' || r == '\r' || r == 0x2028 || r == 0x2029 }

func isSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\v', '\f', 0xa0, 0xfeff, 0x2028, 0x2029, 0x2060, 0x200b:
		return true
	}
	return unicode.Is(unicode.Zs, r)
}

func (p *parser) runeAt(i int) (rune, int) {
	if i >= len(p.src) {
		return -1, 0
	}
	b := p.src[i]
	if b < utf8.RuneSelf {
		return rune(b), 1
	}
	return utf8.DecodeRune(p.src[i:])
}

func (p *parser) byteAt(i int) byte {
	if i < len(p.src) && i >= 0 {
		return p.src[i]
	}
	return 0
}

func (p *parser) addComment(typ string, start, end int) {
	for k := len(p.comments) - 1; k >= 0 && p.comments[k].StartByte >= start; k-- {
		if p.comments[k].StartByte == start {
			return // re-scanned region
		}
	}
	c := syntax.Leaf(typ, true, start, end)
	c.Extra = true
	p.comments = append(p.comments, c)
}

// skipTrivia skips whitespace and comments from i, recording comments, and
// returns the next significant offset and whether a line terminator was seen.
func (p *parser) skipTrivia(i int) (int, bool) {
	nl := false
	for i < len(p.src) {
		r, w := p.runeAt(i)
		switch {
		case isSpace(r):
			if isLineTerm(r) {
				nl = true
			}
			i += w
		case r == '/' && p.byteAt(i+1) == '/':
			j := i
			for j < len(p.src) {
				r2, w2 := p.runeAt(j)
				if r2 == '\n' || r2 == '\r' || r2 == 0x2028 || r2 == 0x2029 {
					break
				}
				j += w2
			}
			p.addComment("comment", i, j)
			i = j
		case r == '/' && p.byteAt(i+1) == '*':
			j := i + 2
			for j < len(p.src) && !(p.src[j] == '*' && p.byteAt(j+1) == '/') {
				if p.src[j] == '\n' {
					nl = true
				}
				j++
			}
			if j >= len(p.src) {
				fail(i, "unterminated comment")
			}
			j += 2
			p.addComment("comment", i, j)
			i = j
		default:
			return i, nl
		}
	}
	return i, nl
}

// scan reads the token starting at or after offset i.
func (p *parser) scan(i int) token {
	i, nl := p.skipTrivia(i)
	t := token{start: i, nl: nl}
	if i >= len(p.src) {
		t.kind, t.end = tEOF, i
		return t
	}
	r, w := p.runeAt(i)
	switch {
	case r == '#' && i == 0 && p.byteAt(1) == '!':
		j := i
		for j < len(p.src) && p.src[j] != '\n' {
			j++
		}
		t.kind, t.end = tHashbang, j
	case r == '#':
		j := p.scanIdent(i + 1)
		if j == i+1 {
			fail(i, "stray #")
		}
		t.kind, t.end = tPrivate, j
	case isIDStart(r):
		t.kind, t.end = tIdent, p.scanIdent(i)
	case r >= '0' && r <= '9' || r == '.' && p.byteAt(i+1) >= '0' && p.byteAt(i+1) <= '9':
		t.kind, t.end = tNumber, p.scanNumber(i)
	case r == '"' || r == '\'':
		t.kind, t.end = tString, p.scanString(i)
	default:
		t.kind = tPunct
		for _, op := range puncts {
			if hasPrefixAt(p.src, i, op) {
				t.end = i + len(op)
				break
			}
		}
		if t.end == 0 {
			if r == '`' || containsByte(punct1, byte(r)) && r < 0x80 {
				t.end = i + w
			} else {
				fail(i, "unexpected character %q", r)
			}
		}
		// `?.` followed by a decimal digit is `?` then a number.
		if t.end == i+2 && p.src[i] == '?' && p.src[i+1] == '.' && p.byteAt(i+2) >= '0' && p.byteAt(i+2) <= '9' {
			t.end = i + 1
		}
	}
	t.text = string(p.src[t.start:t.end])
	return t
}

func hasPrefixAt(src []byte, i int, s string) bool {
	if i+len(s) > len(src) {
		return false
	}
	return string(src[i:i+len(s)]) == s
}

func containsByte(s string, b byte) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return true
		}
	}
	return false
}

func (p *parser) scanIdent(i int) int {
	for i < len(p.src) {
		r, w := p.runeAt(i)
		if r == '\\' && p.byteAt(i+1) == 'u' {
			j := i + 2
			if p.byteAt(j) == '{' {
				for j < len(p.src) && p.src[j] != '}' {
					j++
				}
				i = j + 1
			} else {
				i = j + 4
			}
			continue
		}
		if !isIDPart(r) || r == '\\' {
			break
		}
		i += w
	}
	return i
}

func isDigitFor(b byte, base int) bool {
	switch base {
	case 16:
		return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
	case 8:
		return b >= '0' && b <= '7'
	case 2:
		return b == '0' || b == '1'
	}
	return b >= '0' && b <= '9'
}

func (p *parser) scanDigits(i, base int) int {
	for i < len(p.src) && (isDigitFor(p.src[i], base) || p.src[i] == '_') {
		i++
	}
	return i
}

func (p *parser) scanNumber(i int) int {
	if p.src[i] == '0' {
		switch p.byteAt(i + 1) {
		case 'x', 'X':
			i = p.scanDigits(i+2, 16)
			if p.byteAt(i) == 'n' {
				i++
			}
			return i
		case 'o', 'O':
			i = p.scanDigits(i+2, 8)
			if p.byteAt(i) == 'n' {
				i++
			}
			return i
		case 'b', 'B':
			i = p.scanDigits(i+2, 2)
			if p.byteAt(i) == 'n' {
				i++
			}
			return i
		}
	}
	i = p.scanDigits(i, 10)
	if p.byteAt(i) == 'n' {
		return i + 1
	}
	if p.byteAt(i) == '.' {
		i = p.scanDigits(i+1, 10)
	}
	if b := p.byteAt(i); b == 'e' || b == 'E' {
		j := i + 1
		if c := p.byteAt(j); c == '+' || c == '-' {
			j++
		}
		if c := p.byteAt(j); c >= '0' && c <= '9' {
			i = p.scanDigits(j, 10)
		}
	}
	return i
}

func (p *parser) scanString(i int) int {
	q := p.src[i]
	j := i + 1
	for j < len(p.src) {
		c := p.src[j]
		switch {
		case c == q:
			return j + 1
		case c == '\\':
			j += 2
			if j <= len(p.src) && p.src[j-1] == '\r' && p.byteAt(j) == '\n' {
				j++
			}
		case c == '\n':
			fail(i, "unterminated string")
		default:
			j++
		}
	}
	fail(i, "unterminated string")
	return 0
}

// escapeEnd returns the end of the escape sequence starting at the backslash
// at i (tree-sitter-javascript's escape_sequence token).
func (p *parser) escapeEnd(i int) int {
	j := i + 1
	c := p.byteAt(j)
	switch {
	case c == 'x':
		if isDigitFor(p.byteAt(j+1), 16) && isDigitFor(p.byteAt(j+2), 16) {
			return j + 3
		}
		return j + 1
	case c == 'u':
		if p.byteAt(j+1) == '{' {
			k := j + 2
			for isDigitFor(p.byteAt(k), 16) {
				k++
			}
			if p.byteAt(k) == '}' && k > j+2 {
				return k + 1
			}
			return j + 1
		}
		for k := 1; k <= 4; k++ {
			if !isDigitFor(p.byteAt(j+k), 16) {
				return j + 1
			}
		}
		return j + 5
	case c >= '0' && c <= '7':
		k := j
		for k < j+3 && p.byteAt(k) >= '0' && p.byteAt(k) <= '7' {
			k++
		}
		return k
	case c == '\r' && p.byteAt(j+1) == '\n':
		return j + 2
	}
	_, w := p.runeAt(j)
	if w == 0 {
		w = 1
	}
	return j + w
}
