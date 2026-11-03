// Package edn parses the EDN subset the upstream tools write (maps, vectors,
// lists, sets, strings, integers, floats, keywords, symbols, nil, booleans,
// characters and tagged literals) and compares two values structurally.
package edn

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Kind is the EDN type of a Value.
type Kind int

const (
	Nil Kind = iota
	Bool
	Int
	Float
	String
	Char
	Keyword
	Symbol
	List
	Vector
	Set
	Map
	Tagged
)

var kindNames = [...]string{"nil", "bool", "int", "float", "string", "char", "keyword", "symbol", "list", "vector", "set", "map", "tagged"}

func (k Kind) String() string { return kindNames[k] }

// Value is one parsed EDN value.
type Value struct {
	Kind  Kind
	Bool  bool
	Int   int64
	Float float64
	Text  string  // String, Char, Keyword (without ':'), Symbol, Tag name, Int/Float source text
	Items []Value // List, Vector, Set, Tagged (one item)
	Keys  []Value // Map keys, parallel to Vals
	Vals  []Value
}

// Parse reads exactly one top-level value (surrounding whitespace allowed).
func Parse(src string) (Value, error) {
	p := &parser{src: src}
	p.skip()
	v, err := p.value()
	if err != nil {
		return Value{}, err
	}
	p.skip()
	if p.pos < len(p.src) {
		return Value{}, p.errorf("trailing data")
	}
	return v, nil
}

type parser struct {
	src string
	pos int
}

func (p *parser) errorf(format string, args ...any) error {
	line := 1 + strings.Count(p.src[:p.pos], "\n")
	return fmt.Errorf("edn: line %d (offset %d): %s", line, p.pos, fmt.Sprintf(format, args...))
}

func (p *parser) skip() {
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch {
		case c == ',' || c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f':
			p.pos++
		case c == ';':
			for p.pos < len(p.src) && p.src[p.pos] != '\n' {
				p.pos++
			}
		default:
			return
		}
	}
}

func isDelim(c byte) bool {
	return strings.IndexByte(" \t\r\n\f,()[]{}\";", c) >= 0
}

func (p *parser) token() string {
	start := p.pos
	for p.pos < len(p.src) && !isDelim(p.src[p.pos]) {
		p.pos++
	}
	return p.src[start:p.pos]
}

func (p *parser) value() (Value, error) {
	if p.pos >= len(p.src) {
		return Value{}, p.errorf("unexpected end of input")
	}
	switch c := p.src[p.pos]; c {
	case '{':
		p.pos++
		return p.mapBody()
	case '[':
		p.pos++
		items, err := p.seq(']')
		return Value{Kind: Vector, Items: items}, err
	case '(':
		p.pos++
		items, err := p.seq(')')
		return Value{Kind: List, Items: items}, err
	case '"':
		return p.str()
	case '\\':
		return p.char()
	case ':':
		p.pos++
		t := p.token()
		if t == "" {
			return Value{}, p.errorf("empty keyword")
		}
		return Value{Kind: Keyword, Text: t}, nil
	case '#':
		return p.dispatch()
	case ')', ']', '}':
		return Value{}, p.errorf("unexpected %q", c)
	default:
		return p.atom()
	}
}

func (p *parser) dispatch() (Value, error) {
	p.pos++
	if p.pos >= len(p.src) {
		return Value{}, p.errorf("unexpected end after #")
	}
	switch p.src[p.pos] {
	case '{':
		p.pos++
		items, err := p.seq('}')
		return Value{Kind: Set, Items: items}, err
	case '_':
		p.pos++
		p.skip()
		if _, err := p.value(); err != nil {
			return Value{}, err
		}
		p.skip()
		return p.value()
	default:
		tag := p.token()
		if tag == "" {
			return Value{}, p.errorf("bad dispatch")
		}
		p.skip()
		inner, err := p.value()
		if err != nil {
			return Value{}, err
		}
		return Value{Kind: Tagged, Text: tag, Items: []Value{inner}}, nil
	}
}

func (p *parser) seq(end byte) ([]Value, error) {
	var items []Value
	for {
		p.skip()
		if p.pos >= len(p.src) {
			return nil, p.errorf("unterminated collection, want %q", end)
		}
		if p.src[p.pos] == end {
			p.pos++
			return items, nil
		}
		if p.src[p.pos] == '#' && p.pos+1 < len(p.src) && p.src[p.pos+1] == '_' {
			p.pos += 2
			p.skip()
			if _, err := p.value(); err != nil {
				return nil, err
			}
			continue
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		items = append(items, v)
	}
}

func (p *parser) mapBody() (Value, error) {
	items, err := p.seq('}')
	if err != nil {
		return Value{}, err
	}
	if len(items)%2 != 0 {
		return Value{}, p.errorf("map with odd number of forms")
	}
	m := Value{Kind: Map}
	for i := 0; i < len(items); i += 2 {
		m.Keys = append(m.Keys, items[i])
		m.Vals = append(m.Vals, items[i+1])
	}
	return m, nil
}

func (p *parser) str() (Value, error) {
	p.pos++ // opening quote
	var b strings.Builder
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		switch c {
		case '"':
			p.pos++
			return Value{Kind: String, Text: b.String()}, nil
		case '\\':
			p.pos++
			if p.pos >= len(p.src) {
				return Value{}, p.errorf("unterminated escape")
			}
			e := p.src[p.pos]
			p.pos++
			switch e {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case 'b':
				b.WriteByte('\b')
			case 'f':
				b.WriteByte('\f')
			case '"', '\\', '/':
				b.WriteByte(e)
			case 'u':
				if p.pos+4 > len(p.src) {
					return Value{}, p.errorf("short \\u escape")
				}
				r, err := strconv.ParseUint(p.src[p.pos:p.pos+4], 16, 32)
				if err != nil {
					return Value{}, p.errorf("bad \\u escape")
				}
				b.WriteRune(rune(r))
				p.pos += 4
			default:
				return Value{}, p.errorf("unknown escape \\%c", e)
			}
		default:
			b.WriteByte(c)
			p.pos++
		}
	}
	return Value{}, p.errorf("unterminated string")
}

var namedChars = map[string]string{"newline": "\n", "space": " ", "tab": "\t", "return": "\r", "backspace": "\b", "formfeed": "\f"}

func (p *parser) char() (Value, error) {
	p.pos++
	if p.pos >= len(p.src) {
		return Value{}, p.errorf("bad character literal")
	}
	r, size := utf8.DecodeRuneInString(p.src[p.pos:])
	start := p.pos
	p.pos += size
	for p.pos < len(p.src) && !isDelim(p.src[p.pos]) {
		p.pos++
	}
	name := p.src[start:p.pos]
	if s, ok := namedChars[name]; ok {
		return Value{Kind: Char, Text: s}, nil
	}
	if strings.HasPrefix(name, "u") && len(name) == 5 {
		if n, err := strconv.ParseUint(name[1:], 16, 32); err == nil {
			return Value{Kind: Char, Text: string(rune(n))}, nil
		}
	}
	if len(name) != size {
		return Value{}, p.errorf("bad character literal %q", name)
	}
	return Value{Kind: Char, Text: string(r)}, nil
}

func (p *parser) atom() (Value, error) {
	t := p.token()
	if t == "" {
		return Value{}, p.errorf("unexpected %q", p.src[p.pos])
	}
	switch t {
	case "nil":
		return Value{Kind: Nil}, nil
	case "true":
		return Value{Kind: Bool, Bool: true}, nil
	case "false":
		return Value{Kind: Bool, Bool: false}, nil
	}
	c := t[0]
	if unicode.IsDigit(rune(c)) || ((c == '-' || c == '+') && len(t) > 1 && unicode.IsDigit(rune(t[1]))) {
		return p.number(t)
	}
	return Value{Kind: Symbol, Text: t}, nil
}

func (p *parser) number(t string) (Value, error) {
	body := strings.TrimSuffix(strings.TrimSuffix(t, "N"), "M")
	if !strings.ContainsAny(body, ".eE") && !strings.HasSuffix(t, "M") {
		n, err := strconv.ParseInt(body, 10, 64)
		if err != nil {
			return Value{}, p.errorf("bad integer %q", t)
		}
		return Value{Kind: Int, Int: n, Text: t}, nil
	}
	f, err := strconv.ParseFloat(body, 64)
	if err != nil {
		return Value{}, p.errorf("bad float %q", t)
	}
	return Value{Kind: Float, Float: f, Text: t}, nil
}

// String renders a compact, canonical EDN form (map keys in source order).
func (v Value) String() string {
	var b strings.Builder
	v.write(&b)
	return b.String()
}

func (v Value) write(b *strings.Builder) {
	switch v.Kind {
	case Nil:
		b.WriteString("nil")
	case Bool:
		b.WriteString(strconv.FormatBool(v.Bool))
	case Int, Float:
		b.WriteString(v.Text)
	case String:
		b.WriteString(strconv.Quote(v.Text))
	case Char:
		b.WriteString("\\" + v.Text)
	case Keyword:
		b.WriteString(":" + v.Text)
	case Symbol:
		b.WriteString(v.Text)
	case List, Vector, Set:
		open, close := "(", ")"
		if v.Kind == Vector {
			open, close = "[", "]"
		} else if v.Kind == Set {
			open, close = "#{", "}"
		}
		b.WriteString(open)
		for i, it := range v.Items {
			if i > 0 {
				b.WriteByte(' ')
			}
			it.write(b)
		}
		b.WriteString(close)
	case Map:
		b.WriteByte('{')
		for i := range v.Keys {
			if i > 0 {
				b.WriteString(", ")
			}
			v.Keys[i].write(b)
			b.WriteByte(' ')
			v.Vals[i].write(b)
		}
		b.WriteByte('}')
	case Tagged:
		b.WriteString("#" + v.Text + " ")
		v.Items[0].write(b)
	}
}

// Options control Compare.
type Options struct {
	FloatTolerance float64         // absolute tolerance for float vs float; 0 means exact
	IgnoreKeys     map[string]bool // map keys (keyword names without ':' or string values) skipped at any depth
	MaxDiffs       int             // stop collecting after this many differences; 0 = unlimited
}

// Diff is one structural difference.
type Diff struct {
	Path string
	Want string // golden / upstream
	Got  string // ours
	Note string
}

func (d Diff) String() string {
	s := fmt.Sprintf("%s: want %s, got %s", d.Path, d.Want, d.Got)
	if d.Note != "" {
		s += " (" + d.Note + ")"
	}
	return s
}

// Compare walks want and got and returns their differences.
func Compare(want, got Value, opt Options) []Diff {
	c := &cmp{opt: opt}
	c.walk("$", want, got)
	return c.diffs
}

type cmp struct {
	opt   Options
	diffs []Diff
}

func (c *cmp) full() bool { return c.opt.MaxDiffs > 0 && len(c.diffs) >= c.opt.MaxDiffs }

func (c *cmp) add(path string, want, got Value, note string) {
	if c.full() {
		return
	}
	c.diffs = append(c.diffs, Diff{Path: path, Want: abbrev(want.String()), Got: abbrev(got.String()), Note: note})
}

func abbrev(s string) string {
	if len(s) > 160 {
		return s[:157] + "..."
	}
	return s
}

func (c *cmp) walk(path string, want, got Value) {
	if c.full() {
		return
	}
	if want.Kind != got.Kind {
		c.add(path, want, got, fmt.Sprintf("type %s vs %s", want.Kind, got.Kind))
		return
	}
	switch want.Kind {
	case Nil:
	case Bool:
		if want.Bool != got.Bool {
			c.add(path, want, got, "")
		}
	case Int:
		if want.Int != got.Int {
			c.add(path, want, got, "")
		}
	case Float:
		if !floatEqual(want.Float, got.Float, c.opt.FloatTolerance) {
			c.add(path, want, got, fmt.Sprintf("delta %g", math.Abs(want.Float-got.Float)))
		}
	case String, Char, Keyword, Symbol:
		if want.Text != got.Text {
			c.add(path, want, got, "")
		}
	case Tagged:
		if want.Text != got.Text {
			c.add(path, want, got, "tag")
			return
		}
		c.walk(path+"#"+want.Text, want.Items[0], got.Items[0])
	case List, Vector:
		n := min(len(want.Items), len(got.Items))
		for i := 0; i < n; i++ {
			c.walk(fmt.Sprintf("%s[%d]", path, i), want.Items[i], got.Items[i])
		}
		for i := n; i < len(want.Items); i++ {
			c.add(fmt.Sprintf("%s[%d]", path, i), want.Items[i], Value{Kind: Symbol, Text: "<missing>"}, "missing element")
		}
		for i := n; i < len(got.Items); i++ {
			c.add(fmt.Sprintf("%s[%d]", path, i), Value{Kind: Symbol, Text: "<absent>"}, got.Items[i], "extra element")
		}
	case Set:
		wantSet := map[string]bool{}
		for _, it := range want.Items {
			wantSet[it.String()] = true
		}
		gotSet := map[string]bool{}
		for _, it := range got.Items {
			gotSet[it.String()] = true
		}
		for _, k := range sortedKeys(wantSet) {
			if !gotSet[k] {
				c.add(path+"#{}", Value{Kind: Symbol, Text: k}, Value{Kind: Symbol, Text: "<missing>"}, "missing member")
			}
		}
		for _, k := range sortedKeys(gotSet) {
			if !wantSet[k] {
				c.add(path+"#{}", Value{Kind: Symbol, Text: "<absent>"}, Value{Kind: Symbol, Text: k}, "extra member")
			}
		}
	case Map:
		gotIndex := map[string]int{}
		for i, k := range got.Keys {
			gotIndex[k.String()] = i
		}
		seen := map[string]bool{}
		for i, k := range want.Keys {
			ks := k.String()
			seen[ks] = true
			if c.ignored(k) {
				continue
			}
			j, ok := gotIndex[ks]
			if !ok {
				c.add(path+" "+ks, want.Vals[i], Value{Kind: Symbol, Text: "<missing>"}, "missing key")
				continue
			}
			c.walk(path+" "+ks, want.Vals[i], got.Vals[j])
		}
		for j, k := range got.Keys {
			ks := k.String()
			if seen[ks] || c.ignored(k) {
				continue
			}
			c.add(path+" "+ks, Value{Kind: Symbol, Text: "<absent>"}, got.Vals[j], "extra key")
		}
	}
}

func (c *cmp) ignored(k Value) bool {
	if len(c.opt.IgnoreKeys) == 0 {
		return false
	}
	return (k.Kind == Keyword || k.Kind == String) && c.opt.IgnoreKeys[k.Text]
}

func floatEqual(a, b, tol float64) bool {
	if a == b {
		return true
	}
	if math.IsNaN(a) && math.IsNaN(b) {
		return true
	}
	return tol > 0 && math.Abs(a-b) <= tol
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
