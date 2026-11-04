package mutate

// Mutation sites and per-function counts (upstream mutator/model.py).

import "sort"

// Site is one replacement inside a function crapper scores.
type Site struct {
	File       string
	Namespace  string
	FormID     string
	Name       string
	Line       int
	Start, End int // UTF-8 byte offsets into the file, end exclusive
	Original   string
	Mutant     string
	Category   string
	ID         string // mutation id (the :outcomes key)
	AnchorLine int    // line its statement starts on
	Label      string // react operators: the description (react_sites.go)
}

// Description is "delete <orig>" or "<orig> -> <mutant>"; a React
// operator's site describes itself (`drop render <X/>`).
func (s Site) Description() string {
	if s.Label != "" {
		return s.Label
	}
	if s.Mutant == "" {
		return "delete " + s.Original
	}
	return s.Original + " -> " + s.Mutant
}

// FormResult is the killed/survived/uncovered totals of one operation.
type FormResult struct {
	ID        string
	Namespace string
	Name      string
	Private   bool
	File      string
	Line      int
	EndLine   int
	Digest    string
	Killed    int
	Survived  int
	Uncovered int
	Sites     int
}

// Score is killed / executed; ok=false when nothing executed.
func (f FormResult) Score() (float64, bool) {
	executed := f.Killed + f.Survived
	if executed == 0 {
		return 0, false
	}
	return float64(f.Killed) / float64(executed), true
}

type formKey struct{ Namespace, ID string }

// History is the previous results for one source file.
type History struct {
	Forms    map[formKey]string // prior digest
	Outcomes map[string]string  // mutation id -> killed|survived
}

// RunResult is one file's mutate outcome.
type RunResult struct {
	Path            string
	Forms           []FormResult
	Written         []string
	Sites           []Site
	Statuses        map[string]string
	KilledBy        map[string][]string // :killed-by of this file's killed mutants (carried + --attribute)
	Hint            string              // --attribute cannot run: one line naming what to install (exit 1)
	BaselineFailed  bool
	BaselineMessage string
	Skipped         string
}

func sortStable[T any](s []T, less func(a, b T) bool) {
	sort.SliceStable(s, func(i, j int) bool { return less(s[i], s[j]) })
}
