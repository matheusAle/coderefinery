package testrun

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ReadRuns reads .metrics/tests/runs.edn as RenderRuns writes it: one flat
// map per run whose values are strings, ints, booleans, nil or vectors of
// strings. A missing file is (nil, nil).
func ReadRuns(root string) ([]RunRecord, error) {
	raw, err := os.ReadFile(filepath.Join(root, ".metrics", "tests", "runs.edn"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	text := string(raw)
	start := strings.Index(text, ":runs [")
	if start < 0 {
		return nil, fmt.Errorf("runs.edn has no :runs")
	}
	sc := &ednScanner{s: text, i: start + len(":runs [")}
	var out []RunRecord
	for {
		sc.space()
		if sc.i >= len(sc.s) || sc.s[sc.i] == ']' {
			return out, nil
		}
		m, err := sc.flatMap()
		if err != nil {
			return nil, fmt.Errorf("runs.edn: %v", err)
		}
		rec := RunRecord{}
		rec.Language, _ = m["language"].(string)
		rec.Module, _ = m["module"].(string)
		rec.Tool, _ = m["tool"].(string)
		rec.Command, _ = m["command"].(string)
		rec.Results, _ = m["results"].(string)
		rec.Coverage, _ = m["coverage"].(string)
		rec.Granularity, _ = m["granularity"].(string)
		rec.Contexts, _ = m["contexts"].(bool)
		rec.BuildFailed, _ = m["build-failed"].(bool)
		for k, dst := range map[string]*int{"exit": &rec.Exit, "pass": &rec.Pass, "fail": &rec.Fail, "skip": &rec.Skip, "error": &rec.Error} {
			if n, ok := m[k].(int); ok {
				*dst = n
			}
		}
		rec.Flaky, _ = m["flaky"].([]string)
		out = append(out, rec)
	}
}

type ednScanner struct {
	s string
	i int
}

func (e *ednScanner) space() {
	for e.i < len(e.s) && strings.ContainsRune(" \t\r\n,", rune(e.s[e.i])) {
		e.i++
	}
}

func (e *ednScanner) flatMap() (map[string]any, error) {
	if e.s[e.i] != '{' {
		return nil, fmt.Errorf("expected { at %d", e.i)
	}
	e.i++
	m := map[string]any{}
	for {
		e.space()
		if e.i >= len(e.s) {
			return nil, fmt.Errorf("unterminated map")
		}
		if e.s[e.i] == '}' {
			e.i++
			return m, nil
		}
		if e.s[e.i] != ':' {
			return nil, fmt.Errorf("expected a keyword at %d", e.i)
		}
		key := e.token()[1:]
		e.space()
		v, err := e.value()
		if err != nil {
			return nil, err
		}
		m[key] = v
	}
}

func (e *ednScanner) token() string {
	start := e.i
	for e.i < len(e.s) && !strings.ContainsRune(" \t\r\n,{}[]\"", rune(e.s[e.i])) {
		e.i++
	}
	return e.s[start:e.i]
}

func (e *ednScanner) str() (string, error) {
	var b strings.Builder
	e.i++ // opening quote
	for e.i < len(e.s) {
		c := e.s[e.i]
		switch c {
		case '\\':
			if e.i+1 < len(e.s) {
				b.WriteByte(e.s[e.i+1])
			}
			e.i += 2
			continue
		case '"':
			e.i++
			return b.String(), nil
		}
		b.WriteByte(c)
		e.i++
	}
	return "", fmt.Errorf("unterminated string")
}

func (e *ednScanner) value() (any, error) {
	if e.i >= len(e.s) {
		return nil, fmt.Errorf("missing value")
	}
	switch e.s[e.i] {
	case '"':
		return e.str()
	case '[':
		e.i++
		var list []string
		for {
			e.space()
			if e.i >= len(e.s) {
				return nil, fmt.Errorf("unterminated vector")
			}
			if e.s[e.i] == ']' {
				e.i++
				return list, nil
			}
			if e.s[e.i] != '"' {
				return nil, fmt.Errorf("vectors hold strings only")
			}
			s, err := e.str()
			if err != nil {
				return nil, err
			}
			list = append(list, s)
		}
	}
	tok := e.token()
	switch tok {
	case "nil":
		return nil, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if n, err := strconv.Atoi(tok); err == nil {
		return n, nil
	}
	return tok, nil
}
