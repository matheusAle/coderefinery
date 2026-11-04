package react

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/matheusAle/coderefinery/internal/edn"
	"github.com/matheusAle/coderefinery/internal/pyfmt"
)

// RenderStats are one component's renders in the profiled test run.
type RenderStats struct {
	Renders int     // commits in which the component rendered
	Mounts  int     // of those, first renders of a new instance
	TotalMs float64 // sum of React's actualDuration over the renders
	MaxMs   float64 // the largest actualDuration
}

// Updates are the renders of instances already mounted (re-renders).
func (s RenderStats) Updates() int { return s.Renders - s.Mounts }

// Profile is the merged profiler output of one `react --profile` run.
type Profile struct {
	Runners    []string               // the test runners used, in package order
	Components map[string]RenderStats // by runtime component name
	Renderers  int                    // react-dom injections seen (0: React never connected)
	Untimed    bool                   // a render had no actualDuration (production build)
	Error      string                 // the profiler hook threw
}

// profileFile is what the generated hook writes (profile_hook.js).
type profileFile struct {
	Renderers  []json.RawMessage `json:"renderers"`
	Components map[string]struct {
		Renders int     `json:"renders"`
		Mounts  int     `json:"mounts"`
		TotalMs float64 `json:"totalMs"`
		MaxMs   float64 `json:"maxMs"`
	} `json:"components"`
	Untimed bool   `json:"untimed"`
	Error   string `json:"error"`
}

// Merge adds one profile-*.json document: counts and totals are summed,
// the maximum is kept.
func (p *Profile) Merge(raw []byte) error {
	var f profileFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return err
	}
	if p.Components == nil {
		p.Components = map[string]RenderStats{}
	}
	p.Renderers += len(f.Renderers)
	p.Untimed = p.Untimed || f.Untimed
	if p.Error == "" {
		p.Error = f.Error
	}
	for name, c := range f.Components {
		s := p.Components[name]
		s.Renders += c.Renders
		s.Mounts += c.Mounts
		s.TotalMs += c.TotalMs
		s.MaxMs = max(s.MaxMs, c.MaxMs)
		p.Components[name] = s
	}
	return nil
}

// ReadProfiles merges every profile-*.json in dir into p.
func (p *Profile) ReadProfiles(dir string) error {
	matches, _ := filepath.Glob(filepath.Join(dir, "profile-*.json"))
	sort.Strings(matches)
	for _, m := range matches {
		raw, err := os.ReadFile(m)
		if err != nil {
			return err
		}
		if err := p.Merge(raw); err != nil {
			return fmt.Errorf("%s: %v", m, err)
		}
	}
	return nil
}

// Problem is the one-line reason the run produced no usable numbers ("" =
// usable).
func (p *Profile) Problem() string {
	switch {
	case p.Error != "":
		return "--profile: the profiler hook failed: " + strings.SplitN(p.Error, "\n", 2)[0]
	case p.Renderers == 0:
		return "--profile: React never connected to the profiler hook (the tests render no React tree, or react-dom was loaded before the setup file)"
	case p.Untimed:
		return "--profile: the tests ran a production React build (no render timings); run them with NODE_ENV=test"
	}
	return ""
}

// HotUpdatesPerMount: a component re-renders often when its updates are at
// least this many times its mounts (re-renders per mounted instance).
const HotUpdatesPerMount = 3

// RenderRow is one profiled component joined to its unit.
type RenderRow struct {
	Name      string // the unit's name; the runtime name when ambiguous
	Namespace string // "" when ambiguous
	Path      string
	Ambiguous int // > 1: this many components share the runtime name; not joined
	Stats     RenderStats
	Inline    int  // inline-functions + inline-objects of the unit
	Flagged   bool // re-renders often and passes inline function/object props
}

// UpdatesPerMount is Updates / Mounts (Updates when nothing mounted).
func (r RenderRow) UpdatesPerMount() float64 {
	if r.Stats.Mounts == 0 {
		return float64(r.Stats.Updates())
	}
	return float64(r.Stats.Updates()) / float64(r.Stats.Mounts)
}

// shortName is the runtime name of a unit: TodoList.TodoRow renders as TodoRow.
func shortName(name string) string { return name[strings.LastIndex(name, ".")+1:] }

// JoinRenders joins the profile to the component units by name (React
// reports a component's function name or displayName, not its file).
// Runtime names that match no unit (library components, test helpers) are
// dropped; a name shared by several units gets one unjoined row. notRendered
// lists the components the tests never rendered, in units order.
func JoinRenders(units []Unit, p Profile) (rows []RenderRow, notRendered []string) {
	byName := map[string][]Unit{}
	for _, u := range units {
		if u.Kind == KindComponent {
			byName[shortName(u.Name)] = append(byName[shortName(u.Name)], u)
		}
	}
	for name, stats := range p.Components {
		matches := byName[name]
		switch len(matches) {
		case 0:
			continue
		case 1:
			u := matches[0]
			row := RenderRow{Name: u.Name, Namespace: u.Namespace, Path: u.Path, Stats: stats,
				Inline: u.Cost.InlineFunctions + u.Cost.InlineObjects}
			row.Flagged = row.Inline > 0 && row.UpdatesPerMount() >= HotUpdatesPerMount
			rows = append(rows, row)
		default:
			rows = append(rows, RenderRow{Name: name, Ambiguous: len(matches), Stats: stats})
		}
	}
	for _, u := range units {
		if _, ok := p.Components[shortName(u.Name)]; u.Kind == KindComponent && !ok {
			notRendered = append(notRendered, u.Name)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Stats.Updates() != b.Stats.Updates() {
			return a.Stats.Updates() > b.Stats.Updates()
		}
		if a.Stats.Renders != b.Stats.Renders {
			return a.Stats.Renders > b.Stats.Renders
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Namespace < b.Namespace
	})
	return rows, notRendered
}

// FlagText marks a row that confirms the static render-cost risk.
const FlagText = "re-renders with inline props"

// FormatRenders is the "Runtime renders" report section.
func FormatRenders(rows []RenderRow, notRendered []string, runners []string) string {
	lines := []string{"", "Runtime renders (" + strings.Join(runners, ", ") +
		" test run; durations are React actualDuration in ms, noisy)", ""}
	var names, namespaces []string
	data := make([][]string, len(rows))
	for i, r := range rows {
		ns, inline, flag := r.Namespace, itoa(r.Inline), ""
		if r.Ambiguous > 1 {
			ns, inline = "("+itoa(r.Ambiguous)+" components)", "-"
		}
		if r.Flagged {
			flag = FlagText
		}
		names, namespaces = append(names, r.Name), append(namespaces, ns)
		data[i] = []string{r.Name, ns, itoa(r.Stats.Renders), itoa(r.Stats.Mounts), itoa(r.Stats.Updates()),
			pyfmt.Fixed(r.UpdatesPerMount(), 1), pyfmt.Fixed(r.Stats.TotalMs, 1), pyfmt.Fixed(r.Stats.MaxMs, 1),
			inline, flag}
	}
	if len(rows) == 0 {
		lines = append(lines, "No analyzed component rendered in the tests.")
	} else {
		lines = append(lines, table([]column{
			{"Component", true, width(30, 48, names)}, {"Namespace", true, width(35, 72, namespaces)},
			{"Renders", false, 0}, {"Mounts", false, 0}, {"Updates", false, 0}, {"Upd/Mount", false, 0},
			{"TotalMs", false, 0}, {"MaxMs", false, 0}, {"Inline", false, 0}, {"Flag", true, 0},
		}, data)...)
	}
	if len(notRendered) > 0 {
		lines = append(lines, "", "Not rendered by the tests: "+strings.Join(notRendered, ", "))
	}
	return strings.Join(lines, "\n") + "\n"
}

// RendersEDN is the :renders value of .metrics/react.edn.
func RendersEDN(rows []RenderRow, notRendered []string, runners []string) string {
	out := make([]string, len(rows))
	for i, r := range rows {
		ns, path := edn.String(r.Namespace), edn.String(r.Path)
		if r.Ambiguous > 1 {
			ns, path = "nil", "nil"
		}
		out[i] = "  {:name " + edn.String(r.Name) + ", :namespace " + ns + ", :path " + path +
			", :renders " + itoa(r.Stats.Renders) + ", :mounts " + itoa(r.Stats.Mounts) +
			", :updates " + itoa(r.Stats.Updates()) + ", :total-ms " + edn.Number4(r.Stats.TotalMs) +
			", :max-ms " + edn.Number4(r.Stats.MaxMs) + ", :inline-props " + itoa(r.Inline) +
			", :ambiguous " + itoa(r.Ambiguous) + ", :flagged " + fmt.Sprint(r.Flagged) + "}"
	}
	return "{:runners " + ednStrings(runners) + ", :not-rendered " + ednStrings(notRendered) + ",\n  " +
		ednVector("components", out) + "}"
}

// WithRenders adds the :renders key to a RenderEDN document.
func WithRenders(doc, renders string) string {
	return strings.TrimSuffix(doc, "}\n") + "\n :renders " + renders + "}\n"
}
