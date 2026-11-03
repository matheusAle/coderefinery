package mutate

// Read and write .metrics/mutate snapshots (upstream mutator/metrics.py).
// One file per namespace; other files' forms and outcomes in a namespace
// are preserved on every write.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/matheusAle/ub-mode/ubgate/internal/discover"
	"github.com/matheusAle/ub-mode/ubgate/internal/pyfmt"
)

// sourceKey is metrics.source_key: path relative to root, posix.
func sourceKey(path, root string) string {
	rel, ok := discover.RelativeTo(discover.Resolve(path), discover.Resolve(root))
	if !ok {
		return path
	}
	return rel
}

// snapshotPath is metrics.snapshot_path.
func snapshotPath(root, namespace string) (string, error) {
	ps := strings.Split(strings.ReplaceAll(strings.ReplaceAll(namespace, `\`, "/"), "::", "/"), "/")
	for _, p := range ps {
		if p == "" || p == "." || p == ".." {
			return "", fmt.Errorf("unsafe namespace %s", pyfmt.PyRepr(namespace))
		}
	}
	elems := append([]string{root, ".metrics", "mutate"}, ps[:len(ps)-1]...)
	return filepath.Join(append(elems, ps[len(ps)-1]+".edn")...), nil
}

func ednFiles(root string) []string {
	dir := filepath.Join(root, ".metrics", "mutate")
	if !discover.IsDir(dir) {
		return nil
	}
	var out []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && strings.HasSuffix(d.Name(), ".edn") && discover.IsFile(p) {
			out = append(out, p)
		}
		return nil
	})
	sortPaths(out)
	return out
}

// sortPaths is sorted() over pathlib paths: component-wise, so "a/b" sorts
// before "a.b".
func sortPaths(paths []string) {
	sortStable(paths, func(a, b string) bool {
		pa, pb := strings.Split(a, "/"), strings.Split(b, "/")
		for i := 0; i < len(pa) && i < len(pb); i++ {
			if pa[i] != pb[i] {
				return pa[i] < pb[i]
			}
		}
		return len(pa) < len(pb)
	})
}

// truthy is Python bool() for read EDN values.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case int64:
		return x != 0
	case int:
		return x != 0
	case float64:
		return x != 0
	case string:
		return x != ""
	case []any:
		return len(x) > 0
	case ednSet:
		return len(x) > 0
	case *ordMap:
		return len(x.pairs) > 0
	}
	return true
}

type snapshot struct {
	path string
	data *ordMap
}

func readSnapshot(path string) *ordMap {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	v, err := ednLoads(string(raw))
	if err != nil {
		return nil
	}
	m, _ := v.(*ordMap)
	return m
}

func readSnapshots(root string) []snapshot {
	var out []snapshot
	for _, p := range ednFiles(root) {
		if data := readSnapshot(p); data != nil {
			out = append(out, snapshot{p, data})
		}
	}
	return out
}

func namespaceOf(data *ordMap) (string, bool) {
	ns, ok := data.Get("namespace").(string)
	return ns, ok
}

func formsOf(data *ordMap) []*ordMap {
	list, ok := data.Get("forms").([]any)
	if !ok {
		return nil
	}
	var out []*ordMap
	for _, f := range list {
		if m, ok := f.(*ordMap); ok {
			out = append(out, m)
		}
	}
	return out
}

type outcome struct{ id, status string }

// outcomesOf is metrics._outcomes_of, in file order.
func outcomesOf(data *ordMap) []outcome {
	raw, ok := data.Get("outcomes").(*ordMap)
	if !ok {
		return nil
	}
	var out []outcome
	for _, p := range raw.Pairs() {
		key, ok := p.Key.(string)
		if !ok {
			continue
		}
		status, _ := p.Val.(string)
		status = strings.TrimPrefix(status, ":")
		if status == "killed" || status == "survived" {
			out = append(out, outcome{key, status})
		}
	}
	return out
}

// formFile is metrics._form_file; ok=false is None.
func formFile(form, data *ordMap) (string, bool) {
	if f, ok := form.Get("file").(string); ok && f != "" {
		return f, true
	}
	s, ok := data.Get("source").(string)
	return s, ok
}

func formFileIs(form, data *ordMap, fileKey string) bool {
	f, ok := formFile(form, data)
	return ok && f == fileKey
}

// loadHistory is metrics.load_history.
func loadHistory(root, fileKey string) History {
	h := History{Forms: map[formKey]string{}, Outcomes: map[string]string{}}
	for _, p := range ednFiles(root) {
		data := readSnapshot(p)
		if data == nil {
			continue
		}
		ns, ok := namespaceOf(data)
		if !ok {
			continue
		}
		for _, form := range formsOf(data) {
			if !formFileIs(form, data, fileKey) {
				continue
			}
			id, ok1 := form.Get("id").(string)
			hash, ok2 := form.Get("hash").(string)
			if ok1 && ok2 {
				h.Forms[formKey{ns, id}] = hash
			}
		}
		for _, o := range outcomesOf(data) {
			if f, ok := mutationFile(o.id); ok && f == fileKey {
				h.Outcomes[o.id] = o.status
			}
		}
	}
	return h
}

func renderForm(f FormResult) *ordMap {
	kind := "defn"
	if f.Private {
		kind = "defn-"
	}
	m := newOrdMap()
	m.Set("id", f.ID)
	m.Set("kind", kind)
	m.Set("file", f.File)
	m.Set("line", f.Line)
	m.Set("end-line", f.EndLine)
	m.Set("hash", f.Digest)
	m.Set("killed", f.Killed)
	m.Set("survived", f.Survived)
	m.Set("uncovered", f.Uncovered)
	m.Set("sites", f.Sites)
	return m
}

// testedAt is datetime.now().astimezone().isoformat(timespec="seconds").
var testedAt = func() string { return time.Now().Format("2006-01-02T15:04:05-07:00") }

func renderSnapshot(namespace, source string, forms []*ordMap, outcomes map[string]string) (string, error) {
	out := newOrdMap()
	for _, k := range sortedKeys(outcomes) {
		out.Set(k, kw(outcomes[k]))
	}
	list := make([]any, len(forms))
	for i, f := range forms {
		list[i] = f
	}
	payload := newOrdMap()
	payload.Set("version", 1)
	payload.Set("tested-at", testedAt())
	payload.Set("source", source)
	payload.Set("namespace", namespace)
	payload.Set("outcomes", out)
	payload.Set("forms", list)
	text, err := ednDumps(payload)
	if err != nil {
		return "", err
	}
	return text + "\n", nil
}

type retainedNS struct {
	forms    []*ordMap
	outcomes map[string]string
}

// pyInt is int(value or 0) for a form's :line.
func pyInt(v any) (int64, error) {
	switch x := v.(type) {
	case nil:
		return 0, nil
	case bool:
		if x {
			return 1, nil
		}
		return 0, nil
	case int:
		return int64(x), nil
	case int64:
		return x, nil
	case float64:
		return int64(x), nil
	case string:
		if x == "" {
			return 0, nil
		}
		n, err := strconv.ParseInt(strings.ReplaceAll(strings.TrimFunc(x, isPySpace), "_", ""), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid literal for int() with base 10: %s", pyfmt.PyRepr(x))
		}
		return n, nil
	case []any:
		if len(x) == 0 {
			return 0, nil
		}
	case *ordMap:
		if len(x.pairs) == 0 {
			return 0, nil
		}
	}
	return 0, fmt.Errorf("int() argument must be a string, a bytes-like object or a real number")
}

func dedupeForms(forms []*ordMap) []*ordMap {
	seen := map[string]bool{}
	var out []*ordMap
	for _, f := range forms {
		k := strings.Join([]string{hashKey(f.Get("file")), hashKey(f.Get("id")), hashKey(f.Get("line")), hashKey(f.Get("hash"))}, "\x00")
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, f)
	}
	return out
}

func sortForms(forms []*ordMap) error {
	lines := make(map[*ordMap]int64, len(forms))
	for _, f := range forms {
		n, err := pyInt(f.Get("line"))
		if err != nil {
			return err
		}
		lines[f] = n
	}
	sortStable(forms, func(a, b *ordMap) bool {
		fa, fb := pyStr(a.Get("file")), pyStr(b.Get("file"))
		if fa != fb {
			return fa < fb
		}
		if lines[a] != lines[b] {
			return lines[a] < lines[b]
		}
		return pyStr(a.Get("id")) < pyStr(b.Get("id"))
	})
	return nil
}

// writeNamespace is metrics._write_namespace; "" = nothing written.
func writeNamespace(root, namespace string, kept retainedNS, fresh []FormResult, outcomes map[string]string, fileKey string) (string, error) {
	rendered := append([]*ordMap(nil), kept.forms...)
	for _, f := range fresh {
		rendered = append(rendered, renderForm(f))
	}
	rendered = dedupeForms(rendered)
	if err := sortForms(rendered); err != nil {
		return "", err
	}
	merged := map[string]string{}
	for k, v := range kept.outcomes {
		merged[k] = v
	}
	if len(fresh) > 0 {
		for k, v := range outcomes {
			if ns, ok := mutationNamespace(k); ok && ns == namespace {
				merged[k] = v
			}
		}
	}
	path, err := snapshotPath(root, namespace)
	if err != nil {
		return "", err
	}
	if len(rendered) == 0 && len(merged) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		return "", nil
	}
	var src string
	switch {
	case len(fresh) > 0:
		src = fresh[0].File
	case len(kept.forms) > 0:
		src = fileKey
		if f := kept.forms[0].Get("file"); truthy(f) {
			src = pyStr(f)
		}
	default:
		return "", errors.New("list index out of range")
	}
	text, err := renderSnapshot(namespace, src, rendered, merged)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, []byte(text), 0o666)
}

// formFileOnly is metrics._form_file_only.
func formFileOnly(data *ordMap, fileKey string) bool {
	forms := formsOf(data)
	if len(forms) == 0 {
		s, ok := data.Get("source").(string)
		return ok && s == fileKey
	}
	for _, f := range forms {
		if !formFileIs(f, data, fileKey) {
			return false
		}
	}
	return true
}

// writeResults is metrics.write_results: replace this file's forms and
// outcomes, keep everything else, return the paths written.
func writeResults(root, fileKey string, forms []FormResult, outcomes map[string]string) ([]string, error) {
	root = discover.Resolve(root)
	incoming := map[string][]FormResult{}
	for _, f := range forms {
		incoming[f.Namespace] = append(incoming[f.Namespace], f)
	}
	incomingOutcomes := map[string]string{}
	for k, v := range outcomes {
		if f, ok := mutationFile(k); ok && f == fileKey && (v == "killed" || v == "survived") {
			incomingOutcomes[k] = v
		}
	}
	existing := readSnapshots(root)
	retained := map[string]*retainedNS{}
	touched := map[string]bool{}
	for ns := range incoming {
		touched[ns] = true
	}
	for _, s := range existing {
		ns, ok := namespaceOf(s.data)
		if !ok {
			continue
		}
		var keptForms []*ordMap
		mentions := false
		for _, f := range formsOf(s.data) {
			if formFileIs(f, s.data, fileKey) {
				mentions = true
			} else {
				keptForms = append(keptForms, f)
			}
		}
		if mentions {
			touched[ns] = true
		}
		keptOutcomes := map[string]string{}
		for _, o := range outcomesOf(s.data) {
			if f, ok := mutationFile(o.id); !ok || f != fileKey {
				keptOutcomes[o.id] = o.status
			}
		}
		if len(keptForms) == 0 && len(keptOutcomes) == 0 {
			continue
		}
		r := retained[ns]
		if r == nil {
			r = &retainedNS{outcomes: map[string]string{}}
			retained[ns] = r
		}
		r.forms = append(r.forms, keptForms...)
		for k, v := range keptOutcomes {
			r.outcomes[k] = v
		}
		touched[ns] = true
	}
	names := make([]string, 0, len(touched))
	for ns := range touched {
		names = append(names, ns)
	}
	sort.Strings(names)
	var written []string
	canonical := map[string]bool{}
	for _, ns := range names {
		kept := retainedNS{}
		if r := retained[ns]; r != nil {
			kept = *r
		}
		path, err := writeNamespace(root, ns, kept, incoming[ns], incomingOutcomes, fileKey)
		if err != nil {
			return nil, err
		}
		if path != "" {
			written = append(written, path)
		}
	}
	for _, ns := range names {
		p, err := snapshotPath(root, ns)
		if err != nil {
			return nil, err
		}
		canonical[discover.Resolve(p)] = true
	}
	for _, s := range existing {
		if canonical[discover.Resolve(s.path)] {
			continue
		}
		ns, ok := namespaceOf(s.data)
		if ok && touched[ns] || formFileOnly(s.data, fileKey) {
			_ = os.Remove(s.path)
		}
	}
	removeEmptyDirs(filepath.Join(root, ".metrics", "mutate"))
	return written, nil
}

// removeEmptyDirs removes empty directories below dir, deepest first.
func removeEmptyDirs(dir string) {
	if !discover.IsDir(dir) {
		return
	}
	var all []string
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && p != dir {
			all = append(all, p)
		}
		return nil
	})
	sort.Sort(sort.Reverse(sort.StringSlice(all)))
	for _, p := range all {
		if !discover.IsDir(p) || isSymlink(p) {
			continue
		}
		if entries, err := os.ReadDir(p); err == nil && len(entries) == 0 {
			_ = os.Remove(p)
		}
	}
}
