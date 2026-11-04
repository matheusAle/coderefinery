package java

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type tokKind int

const (
	tkEOF       tokKind = iota
	tkIdent             // identifiers and keywords (the parser decides by text)
	tkLit               // number / character literal; typ is the node type
	tkStrStart          // `"` or `"""`
	tkStrEnd            // the closing delimiter
	tkFrag              // string_fragment / multiline_string_fragment (typ)
	tkEscape            // escape_sequence
	tkHiddenEsc         // text-block escape tree-sitter keeps hidden (`\<newline>`, `\0`, ...)
	tkInterp            // `\{` opening a string_interpolation
	tkOp                // punctuation and operators; `>` is always lexed alone
	tkComment           // line_comment / block_comment (typ)
)

type token struct {
	kind       tokKind
	typ        string // node type for literals, fragments and comments
	text       string
	start, end int
}

// frame is one level of the string/interpolation mode stack: either the
// content of a string (text = text block) or the code inside `\{ ... }`
// (braces counts unmatched `{`).
type frame struct {
	str    bool
	text   bool
	braces int
}

type lexer struct {
	src   string
	pos   int
	toks  []token
	stack []frame
}

func lex(src string) ([]token, error) {
	l := &lexer{src: src}
	for {
		if n := len(l.stack); n > 0 && l.stack[n-1].str {
			if err := l.stringPiece(); err != nil {
				return nil, err
			}
			continue
		}
		l.skipSpace()
		if l.pos >= len(l.src) {
			if len(l.stack) > 0 {
				return nil, fmt.Errorf("unterminated string at EOF")
			}
			l.toks = append(l.toks, token{kind: tkEOF, start: l.pos, end: l.pos})
			return l.toks, nil
		}
		if err := l.codeToken(); err != nil {
			return nil, err
		}
	}
}

func (l *lexer) emit(kind tokKind, typ string, start int) {
	l.toks = append(l.toks, token{kind: kind, typ: typ, text: l.src[start:l.pos], start: start, end: l.pos})
}

func (l *lexer) skipSpace() {
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v' {
			l.pos++
			continue
		}
		r, size := utf8.DecodeRuneInString(l.src[l.pos:])
		if r >= 0x80 && unicode.IsSpace(r) {
			l.pos += size
			continue
		}
		return
	}
}

func isIdentStart(r rune) bool {
	return r == '_' || r == '$' || unicode.IsLetter(r) || unicode.Is(unicode.Nl, r)
}

func isIdentPart(r rune) bool {
	return isIdentStart(r) || unicode.IsDigit(r) || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Mc, r) ||
		unicode.Is(unicode.Nd, r) || unicode.Is(unicode.Pc, r) || r == 0xA2
}

func (l *lexer) identEnd(pos int) int {
	for pos < len(l.src) {
		r, size := utf8.DecodeRuneInString(l.src[pos:])
		if !isIdentPart(r) {
			break
		}
		pos += size
	}
	return pos
}

// operators, longest first. `>` combinations are never lexed: the parser
// joins adjacent `>`/`=` tokens in expressions (type arguments need them
// split, as tree-sitter's context-aware lexer does).
var operators = []string{
	"<<=", "...", "->", "::", "++", "--", "&&", "||", "==", "!=", "<=", "<<",
	"+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=",
	"(", ")", "{", "}", "[", "]", ";", ",", ".", "@", "=", "<", ">", "!", "~",
	"?", ":", "+", "-", "*", "/", "&", "|", "^", "%",
}

func (l *lexer) codeToken() error {
	start := l.pos
	rest := l.src[l.pos:]
	switch {
	case strings.HasPrefix(rest, "//"):
		end := strings.IndexByte(rest, '\n')
		if end < 0 {
			end = len(rest)
		}
		l.pos += end
		l.emit(tkComment, "line_comment", start)
		return nil
	case strings.HasPrefix(rest, "/*"):
		end := strings.Index(rest[2:], "*/")
		if end < 0 {
			return fmt.Errorf("unterminated block comment at %d", start)
		}
		l.pos += end + 4
		l.emit(tkComment, "block_comment", start)
		return nil
	case strings.HasPrefix(rest, `"""`):
		l.pos += 3
		l.emit(tkStrStart, "", start)
		l.stack = append(l.stack, frame{str: true, text: true})
		return nil
	case rest[0] == '"':
		l.pos++
		l.emit(tkStrStart, "", start)
		l.stack = append(l.stack, frame{str: true})
		return nil
	case rest[0] == '\'':
		return l.charLiteral()
	case rest[0] >= '0' && rest[0] <= '9', rest[0] == '.' && len(rest) > 1 && rest[1] >= '0' && rest[1] <= '9':
		l.number()
		return nil
	case strings.HasPrefix(rest, "@interface") && !l.identContinues(start+len("@interface")):
		l.pos += len("@interface")
		l.emit(tkOp, "", start)
		return nil
	}
	r, size := utf8.DecodeRuneInString(rest)
	if isIdentStart(r) {
		l.pos = l.identEnd(start + size)
		if l.src[start:l.pos] == "non" && strings.HasPrefix(l.src[l.pos:], "-sealed") && !l.identContinues(l.pos+len("-sealed")) {
			l.pos += len("-sealed")
		}
		l.emit(tkIdent, "", start)
		return nil
	}
	for _, op := range operators {
		if strings.HasPrefix(rest, op) {
			l.pos += len(op)
			l.emit(tkOp, "", start)
			if n := len(l.stack); n > 0 {
				top := &l.stack[n-1]
				switch op {
				case "{":
					top.braces++
				case "}":
					if top.braces == 0 {
						l.stack = l.stack[:n-1]
					} else {
						top.braces--
					}
				}
			}
			return nil
		}
	}
	return fmt.Errorf("unexpected character %q at %d", r, start)
}

func (l *lexer) identContinues(pos int) bool {
	if pos >= len(l.src) {
		return false
	}
	r, _ := utf8.DecodeRuneInString(l.src[pos:])
	return isIdentPart(r)
}

func (l *lexer) charLiteral() error {
	start := l.pos
	l.pos++
	n := 0
	for l.pos < len(l.src) {
		c := l.src[l.pos]
		if c == '\'' && n > 0 {
			l.pos++
			l.emit(tkLit, "character_literal", start)
			return nil
		}
		if c == '\\' && l.pos+1 < len(l.src) {
			_, size := utf8.DecodeRuneInString(l.src[l.pos+1:])
			l.pos += 1 + size
		} else if c == '\n' || c == '\'' {
			break
		} else {
			_, size := utf8.DecodeRuneInString(l.src[l.pos:])
			l.pos += size
		}
		n++
	}
	return fmt.Errorf("bad character literal at %d", start)
}

func isDigit(c byte) bool    { return c >= '0' && c <= '9' }
func isHexDigit(c byte) bool { return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F' }

func (l *lexer) run(ok func(byte) bool) {
	for l.pos < len(l.src) && (ok(l.src[l.pos]) || l.src[l.pos] == '_') {
		l.pos++
	}
}

func (l *lexer) at(i int) byte {
	if l.pos+i < len(l.src) {
		return l.src[l.pos+i]
	}
	return 0
}

func (l *lexer) exponent() bool {
	c := l.at(0)
	if c != 'e' && c != 'E' && c != 'p' && c != 'P' {
		return false
	}
	i := 1
	if l.at(1) == '+' || l.at(1) == '-' {
		i = 2
	}
	if !isDigit(l.at(i)) {
		return false
	}
	l.pos += i
	l.run(isDigit)
	return true
}

func (l *lexer) floatSuffix() bool {
	switch l.at(0) {
	case 'f', 'F', 'd', 'D':
		l.pos++
		return true
	}
	return false
}

func (l *lexer) longSuffix() {
	if l.at(0) == 'l' || l.at(0) == 'L' {
		l.pos++
	}
}

func (l *lexer) number() {
	start := l.pos
	if l.at(0) == '0' && (l.at(1) == 'x' || l.at(1) == 'X') {
		l.pos += 2
		l.run(isHexDigit)
		isFloat := false
		if l.at(0) == '.' {
			l.pos++
			l.run(isHexDigit)
			isFloat = true
		}
		if l.exponent() {
			isFloat = true
			l.floatSuffix()
		}
		if isFloat {
			l.emit(tkLit, "hex_floating_point_literal", start)
			return
		}
		l.longSuffix()
		l.emit(tkLit, "hex_integer_literal", start)
		return
	}
	if l.at(0) == '0' && (l.at(1) == 'b' || l.at(1) == 'B') {
		l.pos += 2
		l.run(func(c byte) bool { return c == '0' || c == '1' })
		l.longSuffix()
		l.emit(tkLit, "binary_integer_literal", start)
		return
	}
	l.run(isDigit)
	isFloat := false
	if l.at(0) == '.' && l.at(1) != '.' {
		l.pos++
		l.run(isDigit)
		isFloat = true
	}
	if l.exponent() {
		isFloat = true
	}
	if l.floatSuffix() {
		isFloat = true
	}
	if isFloat {
		l.emit(tkLit, "decimal_floating_point_literal", start)
		return
	}
	l.longSuffix()
	text := strings.TrimRight(l.src[start:l.pos], "lL")
	typ := "decimal_integer_literal"
	if len(text) > 1 && text[0] == '0' && strings.Trim(text, "01234567_") == "" {
		typ = "octal_integer_literal"
	}
	l.emit(tkLit, typ, start)
}

// stringPiece lexes one token of string content (tree-sitter-java's
// string_fragment / multiline_string_fragment / escape rules).
func (l *lexer) stringPiece() error {
	top := &l.stack[len(l.stack)-1]
	start := l.pos
	rest := l.src[l.pos:]
	if rest == "" {
		return fmt.Errorf("unterminated string")
	}
	if top.text && strings.HasPrefix(rest, `"""`) || !top.text && rest[0] == '"' {
		if top.text {
			l.pos += 3
		} else {
			l.pos++
		}
		l.emit(tkStrEnd, "", start)
		l.stack = l.stack[:len(l.stack)-1]
		return nil
	}
	if strings.HasPrefix(rest, `\{`) {
		l.pos += 2
		l.emit(tkInterp, "", start)
		l.stack = append(l.stack, frame{})
		return nil
	}
	if rest[0] == '\\' {
		return l.escape(top.text)
	}
	if top.text && rest[0] == '"' {
		// /"([^"\\]|\\")*/
		l.pos++
		for l.pos < len(l.src) {
			c := l.src[l.pos]
			if c == '"' {
				break
			}
			if c == '\\' {
				if l.at(1) != '"' {
					break
				}
				l.pos += 2
				continue
			}
			l.pos++
		}
		l.emit(tkFrag, "multiline_string_fragment", start)
		return nil
	}
	end := strings.IndexAny(rest, `"\`)
	if end < 0 {
		end = len(rest)
	}
	l.pos += end
	if top.text {
		l.emit(tkFrag, "multiline_string_fragment", start)
	} else {
		l.emit(tkFrag, "string_fragment", start)
	}
	return nil
}

func (l *lexer) escape(text bool) error {
	start := l.pos
	if l.pos+1 >= len(l.src) {
		return fmt.Errorf("bad escape at %d", start)
	}
	c := l.src[l.pos+1]
	if text && !strings.ContainsRune(`bfnrtsux'"\`, rune(c)) {
		_, size := utf8.DecodeRuneInString(l.src[l.pos+1:])
		l.pos += 1 + size
		l.emit(tkHiddenEsc, "", start)
		return nil
	}
	l.pos++
	switch {
	case c == 'x':
		if !isHexDigit(l.at(1)) || !isHexDigit(l.at(2)) {
			return fmt.Errorf("bad escape at %d", start)
		}
		l.pos += 3
	case c == 'u':
		if l.at(1) == '{' {
			i := 2
			for isHexDigit(l.at(i)) {
				i++
			}
			if i == 2 || l.at(i) != '}' {
				return fmt.Errorf("bad escape at %d", start)
			}
			l.pos += i + 1
			break
		}
		for i := 1; i <= 4; i++ {
			if !isHexDigit(l.at(i)) {
				return fmt.Errorf("bad escape at %d", start)
			}
		}
		l.pos += 5
	case c >= '0' && c <= '7':
		l.pos++
		for i := 0; i < 2 && l.at(0) >= '0' && l.at(0) <= '7'; i++ {
			l.pos++
		}
	default:
		_, size := utf8.DecodeRuneInString(l.src[l.pos:])
		l.pos += size
	}
	l.emit(tkEscape, "escape_sequence", start)
	return nil
}
