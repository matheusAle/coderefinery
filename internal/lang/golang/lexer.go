package golang

import (
	"fmt"
	"go/scanner"
	"go/token"
)

// tok is one go/scanner token with its byte span. Automatic semicolons keep
// lit "\n"; their span is fixed up by the parser (tree-sitter puts the
// terminator on the newline character, after any trailing comment).
type tok struct {
	t          token.Token
	start, end int
	lit        string
}

func (t tok) auto() bool { return t.t == token.SEMICOLON && t.lit != ";" }

// lex tokenizes src with go/scanner (ScanComments). Comments go to extras.
func lex(src []byte) (toks, comments []tok, err error) {
	if len(src) >= 3 && src[0] == 0xEF && src[1] == 0xBB && src[2] == 0xBF {
		return nil, nil, fmt.Errorf("byte order mark is not supported")
	}
	fset := token.NewFileSet()
	file := fset.AddFile("", -1, len(src))
	var firstErr error
	var s scanner.Scanner
	s.Init(file, src, func(pos token.Position, msg string) {
		if firstErr == nil {
			firstErr = fmt.Errorf("%d:%d: %s", pos.Line, pos.Column, msg)
		}
	}, scanner.ScanComments)
	for {
		pos, t, lit := s.Scan()
		if t == token.EOF {
			break
		}
		off := file.Offset(pos)
		var end int
		switch {
		case t == token.SEMICOLON:
			if lit == ";" {
				end = off + 1
			} else {
				end = off
			}
		case lit != "":
			end = off + len(lit)
		default:
			end = off + len(t.String())
		}
		k := tok{t: t, start: off, end: end, lit: string(src[off:end])}
		if t == token.SEMICOLON && lit != ";" {
			k.lit = "\n"
		}
		if t == token.COMMENT {
			comments = append(comments, k)
			continue
		}
		toks = append(toks, k)
	}
	if firstErr != nil {
		return nil, nil, firstErr
	}
	toks = append(toks, tok{t: token.EOF, start: len(src), end: len(src)})
	return toks, comments, nil
}
