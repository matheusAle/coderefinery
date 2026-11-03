package typescript

import "github.com/matheusAle/ub-mode/ubgate/internal/syntax"

// Automatic semicolons. They are hidden zero-width tokens, but their
// position decides which node keeps a trailing same-line comment:
// tree-sitter's scanner gives up when it meets a comment, the comment is
// shifted as an extra, and the semicolon lands after it. The typescript and
// javascript grammars ship different scanner versions, emulated separately.
// The position is kept as a hidden `_asi` child (removed after the extras
// are attached), so the enclosing node's span reaches it.

const hiddenASI = "_asi"

// asiMarker returns the hidden marker for an automatic semicolon after the
// token ending at from, or nil when it sits right there.
func (p *parser) asiMarker(from int) *syntax.Node {
	end := p.asiEnd(from)
	if end == from {
		return nil
	}
	return syntax.Leaf(hiddenASI, false, end, end)
}

func (p *parser) asiEnd(from int) int {
	if p.ts {
		return p.asiEndTS(from)
	}
	return p.asiEndJS(from)
}

func (p *parser) commentEnd(i int) int {
	if p.byteAt(i+1) == '/' {
		for i < len(p.src) && p.src[i] != '\n' {
			i++
		}
		return i
	}
	for j := i + 2; j+1 < len(p.src); j++ {
		if p.src[j] == '*' && p.src[j+1] == '/' {
			return j + 2
		}
	}
	return len(p.src)
}

func (p *parser) atComment(i int) bool {
	return p.byteAt(i) == '/' && (p.byteAt(i+1) == '/' || p.byteAt(i+1) == '*')
}

func isHSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\v' || b == '\f' || b == '\r' }

// asiEndTS: the typescript scanner stops at any comment on the same line,
// so every comment that starts on the line of the last token is absorbed.
func (p *parser) asiEndTS(from int) int {
	pos, i := from, from
	for i < len(p.src) {
		switch {
		case isHSpace(p.src[i]):
			i++
		case p.atComment(i):
			i = p.commentEnd(i)
			pos = i
		default:
			return pos
		}
	}
	return pos
}

type wsResult int

const (
	wsReject wsResult = iota
	wsNoNewline
	wsAccept
)

// scanWS is scan_whitespace_and_comments(consume=false) of the javascript
// scanner.
func (p *parser) scanWS(i int) (wsResult, int) {
	sawBlockNewline := false
	for {
		for i < len(p.src) && isSpaceByte(p.src[i]) {
			i++
		}
		if !p.atComment(i) {
			if p.byteAt(i) == '/' {
				return wsReject, i
			}
			return wsAccept, i
		}
		if p.src[i+1] == '/' {
			i = p.commentEnd(i)
			continue
		}
		j := p.commentEnd(i)
		for k := i; k < j; k++ {
			if p.src[k] == '\n' {
				sawBlockNewline = true
			}
		}
		i = j
		if p.byteAt(i) != '/' {
			if sawBlockNewline {
				return wsAccept, i
			}
			return wsNoNewline, i
		}
	}
}

func isSpaceByte(b byte) bool { return isHSpace(b) || b == '\n' }

// asiEndJS emulates the javascript scanner's scan_automatic_semicolon,
// re-run after each comment it declines.
func (p *parser) asiEndJS(from int) int {
	pos := from
	for {
		if p.jsASIHere(pos) {
			return pos
		}
		// the comment that made the scanner decline becomes an extra
		i := pos
		for i < len(p.src) && isSpaceByte(p.src[i]) {
			i++
		}
		if !p.atComment(i) {
			return pos
		}
		pos = p.commentEnd(i)
	}
}

func (p *parser) jsASIHere(i int) bool {
	for {
		if i >= len(p.src) {
			return true
		}
		c := p.src[i]
		if p.atComment(i) {
			r, j := p.scanWS(i)
			if r != wsNoNewline {
				return false
			}
			i = j
			continue
		}
		if c == '}' || c == '\n' {
			return true
		}
		if !isHSpace(c) {
			return false
		}
		i++
	}
}

// asiAfterBlock reports whether an automatic semicolon is valid after a
// block's `}` ending at from: what follows the line (after comments) must
// not continue an expression.
func (p *parser) asiAfterBlock(from int) bool {
	i := from
	for i < len(p.src) {
		if isHSpace(p.src[i]) {
			i++
		} else if p.atComment(i) && p.src[i+1] == '*' {
			i = p.commentEnd(i)
		} else if p.atComment(i) {
			i = p.commentEnd(i)
		} else {
			break
		}
	}
	if i >= len(p.src) || p.src[i] == '}' {
		return true
	}
	if p.src[i] != '\n' {
		return false
	}
	for i < len(p.src) && (isSpaceByte(p.src[i]) || p.atComment(i)) {
		if p.atComment(i) {
			i = p.commentEnd(i)
		} else {
			i++
		}
	}
	if i >= len(p.src) {
		return true
	}
	switch p.src[i] {
	case '`', ',', ':', ';', '*', '%', '>', '<', '=', '[', '(', '?', '^', '|', '&', '/':
		return false
	case '.':
		return p.byteAt(i+1) >= '0' && p.byteAt(i+1) <= '9'
	case '+':
		return p.byteAt(i+1) == '+'
	case '-':
		return p.byteAt(i+1) == '-'
	case '!':
		return p.byteAt(i+1) != '='
	case 'i':
		if p.byteAt(i+1) == 'n' {
			j := i + 2
			if hasPrefixAt(p.src, i, "instanceof") {
				j = i + 10
			}
			b := p.byteAt(j)
			return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b == '_' || b == '$' || b >= '0' && b <= '9'
		}
	}
	return true
}

// blockASI returns the marker for a statement_block's optional automatic
// semicolon. In javascript only empty blocks in some positions take it
// (observed behaviour of tree-sitter-javascript 0.25).
func (p *parser) blockASI(block *syntax.Node, jsAllowed bool) *syntax.Node {
	if !p.ts && (!jsAllowed || len(block.Children) > 2) {
		return nil
	}
	if !p.asiAfterBlock(block.EndByte) {
		return nil
	}
	return p.asiMarker(block.EndByte)
}

// removeASI drops the hidden markers once extras are attached.
func removeASI(root *syntax.Node) {
	syntax.Walk(root, func(n *syntax.Node) bool {
		kept := n.Children[:0]
		for _, c := range n.Children {
			if c.Type != hiddenASI {
				kept = append(kept, c)
			}
		}
		n.Children = kept
		return true
	})
}
