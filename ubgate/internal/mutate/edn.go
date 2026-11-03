package mutate

// The EDN subset mutator reads and writes (upstream mutator/edn.py).
//
// Values read: nil, bool, int64, float64, string (keywords become their bare
// name), []any (vectors and lists), *ordMap (maps, Python dict semantics) and
// ednSet. Values written additionally include kw (a keyword) and int.

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/matheusAle/ub-mode/ubgate/internal/edn"
	"github.com/matheusAle/ub-mode/ubgate/internal/pyfmt"
)

// kw renders as an EDN keyword (upstream edn.keyword).
type kw string

// ednSet is a #{} set. It can be read but not rendered (upstream TypeError).
type ednSet []any

type pair struct {
	Key, Val any
}

// ordMap is a Python dict: insertion order, a re-set key keeps its position.
type ordMap struct {
	pairs []pair
	index map[string]int
}

func newOrdMap() *ordMap { return &ordMap{index: map[string]int{}} }

func (m *ordMap) Set(k, v any) {
	h := hashKey(k)
	if i, ok := m.index[h]; ok {
		m.pairs[i].Val = v
		return
	}
	m.index[h] = len(m.pairs)
	m.pairs = append(m.pairs, pair{k, v})
}

func (m *ordMap) Get(k string) any {
	if i, ok := m.index[hashKey(k)]; ok {
		return m.pairs[i].Val
	}
	return nil
}

func (m *ordMap) Pairs() []pair { return m.pairs }

// hashKey gives keys Python considers equal the same identity
// (1 == 1.0 == True).
func hashKey(k any) string {
	switch x := k.(type) {
	case nil:
		return "z"
	case string:
		return "s" + x
	case bool:
		if x {
			return "n1"
		}
		return "n0"
	case int:
		return "n" + strconv.Itoa(x)
	case int64:
		return "n" + strconv.FormatInt(x, 10)
	case float64:
		if x == float64(int64(x)) {
			return "n" + strconv.FormatInt(int64(x), 10)
		}
		return "f" + pyfmt.Repr(x)
	}
	return "o" + pyStr(k)
}

// pyStr is Python str() for the values the reader produces.
func pyStr(v any) string {
	switch x := v.(type) {
	case nil:
		return "None"
	case string:
		return x
	case bool:
		if x {
			return "True"
		}
		return "False"
	}
	return pyRepr(v)
}

func pyRepr(v any) string {
	switch x := v.(type) {
	case string:
		return pyfmt.PyRepr(x)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return pyfmt.Repr(x)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = pyRepr(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case ednSet:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = pyRepr(e)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	case *ordMap:
		parts := make([]string, len(x.pairs))
		for i, p := range x.pairs {
			parts[i] = pyRepr(p.Key) + ": " + pyRepr(p.Val)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return pyStr(v)
}

// ---- writer (edn.dumps) ----

var keywordKey = regexp.MustCompile(`^[A-Za-z*+!_?][A-Za-z0-9*+!_?\-./]*$`)

func ednDumps(v any) (string, error) {
	var b strings.Builder
	if err := render(&b, v); err != nil {
		return "", err
	}
	return b.String(), nil
}

func render(b *strings.Builder, v any) error {
	switch x := v.(type) {
	case nil:
		b.WriteString("nil")
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int:
		b.WriteString(strconv.Itoa(x))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case float64:
		b.WriteString(edn.Number4(x))
	case string:
		b.WriteString(edn.String(x))
	case kw:
		b.WriteString(":" + string(x))
	case []any:
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(' ')
			}
			if err := render(b, e); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case *ordMap:
		b.WriteByte('{')
		for i, p := range x.pairs {
			if i > 0 {
				b.WriteByte(' ')
			}
			if s, ok := p.Key.(string); ok && keywordKey.MatchString(s) {
				b.WriteString(":" + s)
			} else if err := render(b, p.Key); err != nil {
				return err
			}
			b.WriteByte(' ')
			if err := render(b, p.Val); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	case ednSet:
		return errors.New("cannot render set as EDN")
	default:
		return fmt.Errorf("cannot render %T as EDN", v)
	}
	return nil
}

// ---- reader (edn.loads) ----

type reader struct {
	text []rune
	i    int
}

var (
	ednInt   = regexp.MustCompile(`^-?[0-9]+$`)
	ednFloat = regexp.MustCompile(`^-?[0-9]+\.[0-9]+$`)
)

// ednLoads parses one EDN value; keywords become their names.
func ednLoads(text string) (v any, err error) {
	if !utf8.ValidString(text) {
		text = strings.ToValidUTF8(text, "�")
	}
	r := &reader{text: []rune(text)}
	r.skip()
	if r.i >= len(r.text) {
		return nil, errors.New("empty EDN")
	}
	v, err = r.value()
	if err != nil {
		return nil, err
	}
	r.skip()
	if r.i < len(r.text) {
		return nil, fmt.Errorf("trailing EDN at %d", r.i)
	}
	return v, nil
}

func (r *reader) peek() (rune, error) {
	if r.i >= len(r.text) {
		return 0, errors.New("unexpected end of EDN")
	}
	return r.text[r.i], nil
}

func (r *reader) skip() {
	for r.i < len(r.text) {
		ch := r.text[r.i]
		if isPySpace(ch) || ch == ',' {
			r.i++
			continue
		}
		if ch == ';' {
			for r.i < len(r.text) && r.text[r.i] != '\n' {
				r.i++
			}
			continue
		}
		break
	}
}

func (r *reader) value() (any, error) {
	r.skip()
	ch, err := r.peek()
	if err != nil {
		return nil, err
	}
	switch ch {
	case '"':
		return r.str()
	case ':':
		return r.keyword()
	case '{':
		items, err := r.collection('}', true)
		if err != nil {
			return nil, err
		}
		m := newOrdMap()
		for i := 0; i+1 < len(items); i += 2 {
			m.Set(mapKey(items[i]), items[i+1])
		}
		return m, nil
	case '[':
		return r.collection(']', false)
	case '(':
		return r.collection(')', false)
	case '#':
		r.i++
		c, err := r.peek()
		if err != nil {
			return nil, err
		}
		if c == '{' {
			items, err := r.collection('}', false)
			if err != nil {
				return nil, err
			}
			return dedupeSet(items), nil
		}
		return nil, errors.New("unsupported EDN dispatch")
	}
	return r.atom()
}

// mapKey is edn._key: str/int/float/None stay, anything else becomes str().
func mapKey(v any) any {
	switch v.(type) {
	case string, int64, float64, nil, bool:
		return v
	}
	return pyStr(v)
}

func dedupeSet(items []any) ednSet {
	seen := map[string]bool{}
	var out ednSet
	for _, it := range items {
		h := hashKey(it)
		if !seen[h] {
			seen[h] = true
			out = append(out, it)
		}
	}
	return out
}

// collection reads items up to closing; a map reads key/value pairs.
func (r *reader) collection(closing rune, pairs bool) ([]any, error) {
	r.i++
	var values []any
	for {
		r.skip()
		ch, err := r.peek()
		if err != nil {
			return nil, err
		}
		if ch == closing {
			r.i++
			if values == nil {
				values = []any{}
			}
			return values, nil
		}
		v, err := r.value()
		if err != nil {
			return nil, err
		}
		values = append(values, v)
		if pairs {
			r.skip()
			val, err := r.value()
			if err != nil {
				return nil, err
			}
			values = append(values, val)
		}
	}
}

func (r *reader) str() (string, error) {
	r.i++
	var b strings.Builder
	for r.i < len(r.text) {
		ch := r.text[r.i]
		r.i++
		if ch == '"' {
			return b.String(), nil
		}
		if ch == '\\' {
			if r.i >= len(r.text) {
				return "", errors.New("unterminated string escape")
			}
			esc := r.text[r.i]
			r.i++
			switch esc {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			default:
				b.WriteRune(esc)
			}
			continue
		}
		b.WriteRune(ch)
	}
	return "", errors.New("unterminated string")
}

func symbolChar(ch rune) bool {
	return !strings.ContainsRune(" \t\r\n,{}[]()\"\\;", ch)
}

func (r *reader) keyword() (string, error) {
	r.i++
	start := r.i
	for r.i < len(r.text) && symbolChar(r.text[r.i]) {
		r.i++
	}
	if r.i == start {
		return "", errors.New("empty keyword")
	}
	return string(r.text[start:r.i]), nil
}

func (r *reader) atom() (any, error) {
	start := r.i
	for r.i < len(r.text) && symbolChar(r.text[r.i]) {
		r.i++
	}
	token := string(r.text[start:r.i])
	if token == "" {
		ch, err := r.peek()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("unexpected character %s", pyfmt.PyRepr(string(ch)))
	}
	switch token {
	case "nil":
		return nil, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if ednInt.MatchString(token) {
		n, err := strconv.ParseInt(token, 10, 64)
		if err != nil {
			// Python ints are unbounded; an int64 overflow is unreadable here.
			return nil, fmt.Errorf("integer out of range: %s", token)
		}
		return n, nil
	}
	if ednFloat.MatchString(token) {
		f, _ := strconv.ParseFloat(token, 64)
		return f, nil
	}
	return token, nil
}

// sortedKeys returns a string-keyed map's keys in code point order.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
