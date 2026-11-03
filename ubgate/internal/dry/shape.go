package dry

import "strings"

// Kind is the kind of a normalized value (upstream shape.py): a keyword
// `K(name)`, a string, or a list.
type Kind uint8

const (
	KindKeyword Kind = iota
	KindString
	KindList
)

// Value is one node of a normalized tree: upstream's K / str / list.
type Value struct {
	Kind  Kind
	Text  string   // keyword name (without ":") or string contents
	Items []*Value // list elements
}

// K is shape.K(name).
func K(name string) *Value { return &Value{Kind: KindKeyword, Text: name} }

// Str is a string atom (a kept name).
func Str(s string) *Value { return &Value{Kind: KindString, Text: s} }

// List is a list value.
func List(items ...*Value) *Value { return &Value{Kind: KindList, Items: items} }

// symbolOf is `[K("symbol"), name]`, the form every kept name takes.
func symbolOf(name string) *Value { return List(K("symbol"), Str(name)) }

func prAtom(v *Value) string {
	if v.Kind == KindKeyword {
		return ":" + v.Text
	}
	s := strings.ReplaceAll(v.Text, `\`, `\\`)
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// walkPrinted calls visit with the printed form of every list in v,
// children first, and returns v's printed form. Go stacks grow on demand,
// so the 1,200-deep trees upstream guards against are fine here.
func walkPrinted(v *Value, visit func(list string), atom func(string)) string {
	if v.Kind != KindList {
		s := prAtom(v)
		atom(s)
		return s
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, item := range v.Items {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(walkPrinted(item, visit, atom))
	}
	b.WriteByte(']')
	s := b.String()
	visit(s)
	return s
}

// Pr is shape.pr: Clojure pr-str of a normalized value.
func Pr(v *Value) string {
	return walkPrinted(v, func(string) {}, func(string) {})
}

// NodeCount is shape.node_count: 1 per atom, 1 + sum of elements per list.
func NodeCount(v *Value) int {
	total := 1
	for _, item := range v.Items {
		total += NodeCount(item)
	}
	return total
}

// Fingerprints is shape.fingerprints: the printed form of every list
// subtree (the root included) and of every atom.
func Fingerprints(v *Value) map[string]struct{} {
	set := map[string]struct{}{}
	add := func(s string) { set[s] = struct{}{} }
	walkPrinted(v, add, add)
	return set
}

// Jaccard is shape.jaccard: |A∩B| / |A∪B|, 0.0 for two empty sets.
func Jaccard(left, right map[string]struct{}) float64 {
	shared := 0
	for k := range left {
		if _, ok := right[k]; ok {
			shared++
		}
	}
	union := len(left) + len(right) - shared
	if union == 0 {
		return 0.0
	}
	return float64(shared) / float64(union)
}
