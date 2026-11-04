package react

import (
	"sort"
)

// limit is one per-unit threshold: a unit offends when value > max.
// max < 0 disables it.
type limit struct {
	flag   string // --max-<flag>
	metric string // EDN :metric and report label
	def    int
	value  func(Unit) int
}

// limits is the table behind the --max-* flags, the checks and the help text.
var limits = []limit{
	{"props", "props", 7, func(u Unit) int { return u.Props.Count }},
	{"bool-props", "boolean-props", 2, func(u Unit) int { return u.Props.Boolean }},
	{"pass-through", "pass-through", 3, func(u Unit) int { return len(u.Props.PassThrough) }},
	{"state", "state", 5, func(u Unit) int { return u.Hooks.State }},
	{"effects", "effects", 3, func(u Unit) int { return u.Hooks.Effects }},
	{"deps", "deps", 6, func(u Unit) int { return u.Hooks.MaxDeps }},
	{"setter-effects", "setter-effects", 0, func(u Unit) int { return u.Hooks.SetterEffects }},
	{"jsx-depth", "jsx-depth", 6, func(u Unit) int { return u.Cost.Depth }},
	{"elements", "elements", -1, func(u Unit) int { return u.Cost.Elements }},
	{"inline-functions", "inline-functions", -1, func(u Unit) int { return u.Cost.InlineFunctions }},
	{"inline-objects", "inline-objects", -1, func(u Unit) int { return u.Cost.InlineObjects }},
	{"nested", "nested-components", 0, func(u Unit) int { return u.Cost.Nested }},
}

// DefaultCrap is the CRAP gate: an entry offends at CRAP >= 10 (the
// skill's "CRAP < 10"); without coverage at CC >= 10, CRAP's lower bound.
const DefaultCrap = 10.0

// Limits are the active thresholds, keyed by limit.flag.
type Limits struct {
	Crap float64
	Max  map[string]int
}

// DefaultLimits returns the defaults of the limits table.
func DefaultLimits() Limits {
	l := Limits{Crap: DefaultCrap, Max: map[string]int{}}
	for _, lim := range limits {
		l.Max[lim.flag] = lim.def
	}
	return l
}

// Violation is one exceeded threshold.
type Violation struct {
	Name      string
	Namespace string
	Path      string
	Line      int
	Metric    string
	Value     float64
	Limit     float64
}

// Check returns the violations: entries' CRAP first, in entries order, then
// units' limits in units order and limits-table order.
func Check(units []Unit, entries []Entry, l Limits) []Violation {
	var out []Violation
	for _, e := range entries {
		switch {
		case e.Crap != nil && *e.Crap >= l.Crap:
			out = append(out, Violation{e.Name, e.Namespace, e.Path, e.StartLine, "crap", *e.Crap, l.Crap})
		case e.Crap == nil && float64(e.Complexity) >= l.Crap:
			out = append(out, Violation{e.Name, e.Namespace, e.Path, e.StartLine, "complexity", float64(e.Complexity), l.Crap})
		}
	}
	for _, u := range units {
		for _, lim := range limits {
			max := l.Max[lim.flag]
			if v := lim.value(u); max >= 0 && v > max {
				out = append(out, Violation{u.Name, u.Namespace, u.Path, u.Line, lim.metric, float64(v), float64(max)})
			}
		}
	}
	return out
}

// SortUnits orders units worst first: most violations, then highest
// render + logic complexity, then namespace and name.
func SortUnits(units []Unit, violations []Violation) []Unit {
	count := map[[2]string]int{}
	for _, v := range violations {
		count[[2]string{v.Namespace, v.Name}]++
	}
	out := append([]Unit(nil), units...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		ca, cb := count[[2]string{a.Namespace, a.Name}], count[[2]string{b.Namespace, b.Name}]
		if ca != cb {
			return ca > cb
		}
		if a.Render+a.Logic != b.Render+b.Logic {
			return a.Render+a.Logic > b.Render+b.Logic
		}
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
	return out
}

// SortEntries orders entries like crap: scored by -CRAP, unscored by -CC,
// then namespace, name, path.
func SortEntries(entries []Entry) []Entry {
	out := append([]Entry(nil), entries...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if (a.Crap == nil) != (b.Crap == nil) {
			return a.Crap != nil
		}
		if a.Crap != nil && *a.Crap != *b.Crap {
			return *a.Crap > *b.Crap
		}
		if a.Crap == nil && a.Complexity != b.Complexity {
			return a.Complexity > b.Complexity
		}
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Path < b.Path
	})
	return out
}
