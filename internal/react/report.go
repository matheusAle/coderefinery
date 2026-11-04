package react

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/matheusAle/coderefinery/internal/edn"
	"github.com/matheusAle/coderefinery/internal/pyfmt"
)

func padRight(s string, w int) string {
	if n := w - utf8.RuneCountInString(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}

func padLeft(s string, w int) string {
	if n := w - utf8.RuneCountInString(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}

// width is the column width: the longest value, at least lo, at most hi.
func width(lo, hi int, values []string) int {
	longest := 0
	for _, v := range values {
		longest = max(longest, utf8.RuneCountInString(v))
	}
	return max(lo, min(hi, longest))
}

func itoa(n int) string { return strconv.Itoa(n) }

func fixed(v *float64, w int, suffix string) string {
	if v == nil {
		return padLeft("N/A", w)
	}
	return pyfmt.FixedWidth(*v, w-len(suffix), 1) + suffix
}

type column struct {
	head  string
	left  bool
	width int
}

func table(cols []column, rows [][]string) []string {
	for i := range cols {
		cols[i].width = max(cols[i].width, utf8.RuneCountInString(cols[i].head))
	}
	line := func(cells []string) string {
		parts := make([]string, len(cells))
		for i, c := range cells {
			if cols[i].left {
				parts[i] = padRight(c, cols[i].width)
			} else {
				parts[i] = padLeft(c, cols[i].width)
			}
		}
		return strings.TrimRight(strings.Join(parts, " "), " ")
	}
	heads := make([]string, len(cols))
	for i, c := range cols {
		heads[i] = c.head
	}
	header := line(heads)
	out := []string{header, strings.Repeat("-", utf8.RuneCountInString(header))}
	for _, r := range rows {
		out = append(out, line(r))
	}
	return out
}

// FormatReport renders the entries table, the components table and the
// violations, each worst first.
func FormatReport(entries []Entry, units []Unit, violations []Violation) string {
	var names, namespaces []string
	for _, e := range entries {
		names, namespaces = append(names, e.Name), append(namespaces, e.Namespace)
	}
	lines := []string{"React Report", "============", ""}
	rows := make([][]string, len(entries))
	for i, e := range entries {
		rows[i] = []string{e.Name, e.Kind, e.Namespace, itoa(e.StartLine), itoa(e.Complexity),
			fixed(e.Coverage, 7, "%"), fixed(e.Crap, 8, "")}
	}
	lines = append(lines, table([]column{
		{"Entry", true, width(30, 48, names)}, {"Kind", true, 9}, {"Namespace", true, width(35, 72, namespaces)},
		{"Line", false, 5}, {"CC", false, 4}, {"Cov%", false, 7}, {"CRAP", false, 8},
	}, rows)...)

	lines = append(lines, "", "Components and hooks", "")
	names, namespaces = nil, nil
	for _, u := range units {
		names, namespaces = append(names, u.Name), append(namespaces, u.Namespace)
	}
	rows = make([][]string, len(units))
	for i, u := range units {
		props := []string{"-", "-", "-", "-", "-", "-"}
		if u.Kind == KindComponent {
			rest := "-"
			if u.Props.Rest {
				rest = "yes"
			}
			props = []string{itoa(u.Props.Count), itoa(u.Props.Boolean), itoa(u.Props.Optional),
				itoa(u.Props.Callback), rest, itoa(len(u.Props.PassThrough))}
		}
		rows[i] = append(append([]string{u.Name, u.Namespace, itoa(u.Render), itoa(u.Logic)}, props...),
			itoa(u.Hooks.State), itoa(u.Hooks.Effects), itoa(u.Hooks.MaxDeps), itoa(u.Hooks.SetterEffects),
			itoa(u.Cost.Depth), itoa(u.Cost.Elements), itoa(u.Cost.InlineFunctions), itoa(u.Cost.InlineObjects),
			itoa(u.Cost.Nested))
	}
	cols := []column{{"Component", true, width(30, 48, names)}, {"Namespace", true, width(35, 72, namespaces)}}
	for _, h := range []string{"Render", "Logic", "Props", "Bool", "Opt", "On", "Rest", "Pass",
		"State", "Eff", "Deps", "SetFx", "Depth", "Elems", "InFn", "InObj", "Nest"} {
		cols = append(cols, column{h, false, 0})
	}
	lines = append(lines, table(cols, rows)...)

	if len(violations) > 0 {
		lines = append(lines, "", "Threshold violations", "")
		rows = make([][]string, len(violations))
		names = nil
		for i, v := range violations {
			names = append(names, v.Name)
			rows[i] = []string{v.Name, v.Namespace, violationText(v)}
		}
		lines = append(lines, table([]column{{"Name", true, width(30, 48, names)},
			{"Namespace", true, width(35, 72, namespaces)}, {"Exceeded", true, 0}}, rows)...)
	}
	return strings.Join(lines, "\n") + "\n"
}

func violationText(v Violation) string {
	if v.Metric == "crap" {
		return "crap " + pyfmt.Fixed(v.Value, 1) + " >= " + pyfmt.Fixed(v.Limit, 1)
	}
	if v.Metric == "complexity" {
		return "complexity " + itoa(int(v.Value)) + " >= " + pyfmt.Fixed(v.Limit, 1) + " (no coverage)"
	}
	return v.Metric + " " + itoa(int(v.Value)) + " > " + itoa(int(v.Limit))
}

func ednInts(keys []string, values ...int) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = ":" + k + " " + itoa(values[i])
	}
	return strings.Join(parts, ", ")
}

func ednStrings(values []string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = edn.String(v)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

func ednVector(key string, rows []string) string {
	if len(rows) == 0 {
		return ":" + key + " []"
	}
	return ":" + key + " [\n" + strings.Join(rows, "\n") + "\n]"
}

// RenderEDN is .metrics/react.edn.
func RenderEDN(entries []Entry, units []Unit, violations []Violation) string {
	var es, us, vs []string
	for _, e := range entries {
		es = append(es, "  {:name "+edn.String(e.Name)+", :namespace "+edn.String(e.Namespace)+
			", :kind :"+e.Kind+", :unit "+edn.String(e.Unit)+", :path "+edn.String(e.Path)+
			", :line "+itoa(e.StartLine)+", :complexity "+itoa(e.Complexity)+
			", :coverage "+edn.OptNumber4(e.Coverage)+", :crap "+edn.OptNumber4(e.Crap)+"}")
	}
	for _, u := range units {
		props := "nil"
		if u.Kind == KindComponent {
			props = "{" + ednInts([]string{"count", "boolean", "optional", "callback"},
				u.Props.Count, u.Props.Boolean, u.Props.Optional, u.Props.Callback) +
				", :rest " + strconv.FormatBool(u.Props.Rest) + ", :pass-through " + ednStrings(u.Props.PassThrough) + "}"
		}
		us = append(us, "  {:name "+edn.String(u.Name)+", :namespace "+edn.String(u.Namespace)+
			", :kind :"+u.Kind+", :path "+edn.String(u.Path)+", :line "+itoa(u.Line)+
			", :render "+itoa(u.Render)+", :logic "+itoa(u.Logic)+
			",\n   :props "+props+
			",\n   :hooks {"+ednInts([]string{"state", "effects", "max-deps", "setter-effects"},
			u.Hooks.State, u.Hooks.Effects, u.Hooks.MaxDeps, u.Hooks.SetterEffects)+"}"+
			",\n   :render-cost {"+ednInts([]string{"jsx-depth", "elements", "inline-functions", "inline-objects", "nested-components"},
			u.Cost.Depth, u.Cost.Elements, u.Cost.InlineFunctions, u.Cost.InlineObjects, u.Cost.Nested)+"}}")
	}
	for _, v := range violations {
		vs = append(vs, "  {:name "+edn.String(v.Name)+", :namespace "+edn.String(v.Namespace)+
			", :path "+edn.String(v.Path)+", :line "+itoa(v.Line)+", :metric :"+v.Metric+
			", :value "+edn.Number4(v.Value)+", :limit "+edn.Number4(v.Limit)+"}")
	}
	return "{" + ednVector("entries", es) + "\n " + ednVector("components", us) + "\n " + ednVector("violations", vs) + "}\n"
}
