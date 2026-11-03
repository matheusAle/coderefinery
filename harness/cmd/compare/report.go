package main

import (
	"fmt"
	"strings"
)

const rowFormat = "%-34s %-7s %-6s %-6s %-10s %-7s %-6s %s\n"

func printHeader() {
	fmt.Printf(rowFormat, "CASE", "EXIT", "STDOUT", "STDERR", "METRICS", "RAW", "RESULT", "TIME")
	fmt.Println(strings.Repeat("-", 92))
}

func printRow(r Result, strict bool) {
	exit := "ok"
	if !r.ExitOK() {
		exit = fmt.Sprintf("%d!=%d", r.WantExit, r.GotExit)
	}
	stderr := "ok"
	if !r.StderrOK {
		stderr = "diff"
		if strict {
			stderr = "DIFF"
		}
	}
	semantic, raw := 0, 0
	for _, f := range r.Files {
		if !f.Missing && !f.Extra && f.Semantic {
			semantic++
		}
		if f.Raw {
			raw++
		}
	}
	metrics := fmt.Sprintf("%s %d/%d", okWord(r.MetricsOK()), semantic, len(r.Files))
	result := "PASS"
	if r.Err != "" {
		result = "ERROR"
	} else if !r.Pass(strict) {
		result = "FAIL"
	}
	fmt.Printf(rowFormat, r.Case.Name(), exit, okWord(r.StdoutOK), stderr, metrics,
		fmt.Sprintf("%d/%d", raw, len(r.Files)), result, r.Duration)
}

func printDiffs(results []Result, s Settings, max int, showStderr bool) {
	shown := 0
	for _, r := range results {
		failing := !r.Pass(s.StrictStderr)
		stderrOnly := !failing && !r.StderrOK && showStderr
		if !failing && !stderrOnly {
			continue
		}
		if shown >= max {
			fmt.Printf("\n(%s and later mismatches not shown; raise -max-diffs)\n", r.Case.Name())
			return
		}
		shown++
		label := "FAIL"
		if !failing {
			label = "STDERR"
		}
		fmt.Printf("\n=== %s %s  args=%q\n", label, r.Case.Name(), r.Case.Args)
		if r.Err != "" {
			fmt.Printf("error: %s\n", r.Err)
		}
		if !r.ExitOK() {
			fmt.Printf("exit code: want %d, got %d\n", r.WantExit, r.GotExit)
		}
		if !r.StdoutOK {
			fmt.Print("stdout:\n" + indent(r.StdoutDiff, "  "))
		}
		if !r.StderrOK && (s.StrictStderr || showStderr) {
			fmt.Print("stderr:\n" + indent(r.StderrDiff, "  "))
		}
		for _, f := range r.Files {
			if f.Missing || f.Extra || !f.Semantic {
				fmt.Println("metrics:\n" + f.describe())
			}
		}
	}
}

func printSummary(mode string, results []Result, strict bool) int {
	pass, fail, errs := 0, 0, 0
	files, semantic, raw, stderrOK := 0, 0, 0, 0
	for _, r := range results {
		switch {
		case r.Err != "":
			errs++
		case r.Pass(strict):
			pass++
		default:
			fail++
		}
		if r.StderrOK {
			stderrOK++
		}
		for _, f := range r.Files {
			files++
			if !f.Missing && !f.Extra && f.Semantic {
				semantic++
			}
			if f.Raw {
				raw++
			}
		}
	}
	fmt.Printf("\nSUMMARY mode=%s cases=%d PASS=%d FAIL=%d ERROR=%d | metrics files=%d semantic-equal=%d raw-equal=%d | stderr-equal=%d/%d\n",
		mode, len(results), pass, fail, errs, files, semantic, raw, stderrOK, len(results))
	if fail+errs > 0 {
		return 1
	}
	return 0
}
