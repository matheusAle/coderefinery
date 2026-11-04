// Command compare checks coderefinery against the upstream crapper / dryer /
// mutator, case by case, from golden outputs or side by side (-live).
//
//	compare -record                      run upstream, (re)write golden/
//	compare                              run ours, diff against golden/
//	compare -live                        run upstream and ours at the same time
//	compare -ours upstream               self-test: "ours" is upstream itself
//
// See harness/README.md for the matrix, the normalisation rules and the
// environment isolation.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"coderefinery/harness/internal/edn"
)

const upstreamTemplate = "{upstream}/{upstream_tool}/{upstream_tool} {args}"

func main() {
	os.Exit(run())
}

func run() int {
	repoRoot := envOr("REPO_ROOT", "/Users/matheusale/workspaces/ub-mode")
	var (
		matrix    = flag.String("matrix", filepath.Join(repoRoot, "harness/matrix.txt"), "matrix file")
		golden    = flag.String("golden", filepath.Join(repoRoot, "harness/golden"), "golden directory")
		fixtures  = flag.String("fixtures", filepath.Join(repoRoot, "fixtures"), "pristine fixture projects")
		scratch   = flag.String("scratch", envOr("SCRATCH_ROOT", filepath.Join(repoRoot, ".local/scratch")), "where fixture copies are made")
		tools     = flag.String("tool", "all", "comma list of tools: crap,dry,mutate")
		fixtureF  = flag.String("fixture", "all", "comma list of fixtures")
		variants  = flag.String("variant", "all", "comma list of variants")
		ours      = flag.String("ours", filepath.Join(repoRoot, "bin/coderefinery")+" {tool} {args}", "command template for ours; 'upstream' = the upstream tool (self-test)")
		upstream  = flag.String("upstream", upstreamTemplate, "command template for upstream")
		record    = flag.Bool("record", false, "run upstream and write golden outputs")
		live      = flag.Bool("live", false, "run upstream and ours side by side instead of reading golden")
		serial    = flag.Bool("serial", false, "with -live: run upstream, then ours, instead of at the same time (avoids CPU-contention timeouts in mutate)")
		floatTol  = flag.Float64("float-tol", 0, "absolute tolerance for EDN floats (0 = exact)")
		ignore    = flag.String("ignore-keys", "tested-at", "comma list of EDN map keys ignored at any depth")
		maxDiffs  = flag.Int("max-diffs", 5, "print detailed diffs for at most this many failing cases")
		diffLines = flag.Int("diff-lines", 60, "truncate each text diff to this many lines")
		strictErr = flag.Bool("strict-stderr", false, "a stderr mismatch fails the case (default: reported only)")
		showErr   = flag.Bool("show-stderr", false, "print stderr diffs even when stderr is not strict")
		noNorm    = flag.Bool("no-normalize", false, "do not mask test-runner durations/clock times (paths are always masked)")
		keep      = flag.Bool("keep", false, "keep scratch copies")
		timeout   = flag.Duration("timeout", 10*time.Minute, "per-run timeout")
		list      = flag.Bool("list", false, "print the selected cases and exit")
		verbose   = flag.Bool("v", false, "print each command")
	)
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "compare: unexpected arguments %q\n", flag.Args())
		return 2
	}
	if *record && *live {
		fmt.Fprintln(os.Stderr, "compare: -record and -live are exclusive")
		return 2
	}
	if *ours == "upstream" {
		*ours = upstreamTemplate
	}
	all, err := loadMatrix(*matrix)
	if err != nil {
		fmt.Fprintln(os.Stderr, "compare:", err)
		return 2
	}
	cases := filterCases(all, *tools, *fixtureF, *variants)
	if len(cases) == 0 {
		fmt.Fprintln(os.Stderr, "compare: no cases selected")
		return 2
	}
	if *list {
		for _, c := range cases {
			fmt.Printf("%-32s %s\n", c.Name(), strings.Join(c.Args, " "))
		}
		return 0
	}
	runID := fmt.Sprintf("%s-%d", time.Now().Format("20060102-150405"), os.Getpid())
	runDir := filepath.Join(*scratch, runID)
	if !*keep {
		defer os.RemoveAll(runDir)
	}
	exec1 := func(side string, tmpl string, c Case) Run {
		argv, err := expandTemplate(tmpl, c, repoRoot)
		if err != nil {
			return Run{Err: err.Error(), Exit: -1}
		}
		root := filepath.Join(runDir, side, c.Tool+"__"+c.Fixture+"__"+c.Variant, c.Fixture)
		r := execute(argv, filepath.Join(*fixtures, c.Fixture), root, *timeout, *verbose)
		if !*keep {
			os.RemoveAll(filepath.Dir(root))
		}
		return r
	}

	if *record {
		return doRecord(cases, *golden, *upstream, exec1)
	}

	ignoreKeys := map[string]bool{}
	for _, k := range strings.Split(*ignore, ",") {
		if k = strings.TrimSpace(strings.TrimPrefix(k, ":")); k != "" {
			ignoreKeys[k] = true
		}
	}
	settings := Settings{
		Norm:         Normaliser{RepoRoot: repoRoot, Timing: !*noNorm},
		EDN:          edn.Options{FloatTolerance: *floatTol, IgnoreKeys: ignoreKeys, MaxDiffs: 25},
		StrictStderr: *strictErr,
		DiffLines:    *diffLines,
	}
	mode := "golden"
	if *live {
		mode = "live"
		if *serial {
			mode = "live-serial"
		}
	}
	fmt.Printf("mode=%s ours=%q cases=%d\n", mode, *ours, len(cases))
	printHeader()
	var results []Result
	for _, c := range cases {
		var want, got Run
		if *live && *serial {
			want = exec1("upstream", *upstream, c)
			got = exec1("ours", *ours, c)
		} else if *live {
			var wg sync.WaitGroup
			wg.Add(2)
			go func() { defer wg.Done(); want = exec1("upstream", *upstream, c) }()
			go func() { defer wg.Done(); got = exec1("ours", *ours, c) }()
			wg.Wait()
		} else {
			g, err := readGolden(goldenDir(*golden, c))
			if err != nil {
				res := Result{Case: c, Err: err.Error(), WantExit: -1, GotExit: -1}
				results = append(results, res)
				printRow(res, settings.StrictStderr)
				continue
			}
			want = g
			got = exec1("ours", *ours, c)
		}
		res := compareRuns(c, want, got, settings)
		results = append(results, res)
		printRow(res, settings.StrictStderr)
	}
	printDiffs(results, settings, *maxDiffs, *showErr)
	return printSummary(mode, results, settings.StrictStderr)
}

func doRecord(cases []Case, golden, tmpl string, exec1 func(string, string, Case) Run) int {
	fmt.Printf("%-34s %5s %9s %s\n", "RECORD", "EXIT", "TIME", "METRICS")
	failed := 0
	for _, c := range cases {
		r := exec1("golden", tmpl, c)
		if r.Err != "" {
			fmt.Printf("%-34s %5s %9s ERROR %s\n", c.Name(), "-", r.Duration.Round(time.Millisecond), r.Err)
			failed++
			continue
		}
		if err := writeGolden(goldenDir(golden, c), r); err != nil {
			fmt.Printf("%-34s ERROR writing golden: %v\n", c.Name(), err)
			failed++
			continue
		}
		fmt.Printf("%-34s %5d %9s %s\n", c.Name(), r.Exit, r.Duration.Round(time.Millisecond), strings.Join(sortedNames(r.Metrics), " "))
	}
	fmt.Printf("SUMMARY recorded=%d errors=%d golden=%s\n", len(cases)-failed, failed, golden)
	if failed > 0 {
		return 1
	}
	return 0
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
