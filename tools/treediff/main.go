// Command treediff proves parser parity: it parses files with coderefinery and
// with upstream tree-sitter (tools/oracle/oracle.py) and reports the first
// divergence per file.
//
//	go run ./tools/treediff [-lang L] [-python PY] [-oracle ORACLE] [-q] PATH...
//
// Exit status: 0 all equal, 1 any divergence or parse error, 2 usage.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/matheusAle/coderefinery/internal/treediff"
)

func main() {
	cwd, _ := os.Getwd()
	root, _ := treediff.FindRoot(cwd)
	cfg := treediff.DefaultConfig(root)
	flag.StringVar(&cfg.Python, "python", cfg.Python, "upstream venv python (tree-sitter 0.26.0)")
	flag.StringVar(&cfg.Oracle, "oracle", cfg.Oracle, "path to tools/oracle/oracle.py")
	flag.StringVar(&cfg.Lang, "lang", "", "force grammar (default: by extension)")
	quiet := flag.Bool("q", false, "only print failures and the summary")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: treediff [-lang L] [-python PY] [-oracle ORACLE] [-q] PATH...")
		os.Exit(2)
	}
	files, err := treediff.Collect(flag.Args(), cfg.Lang)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	results, err := treediff.Run(cfg, files)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	failed := 0
	for _, r := range results {
		if r.OK {
			if !*quiet {
				fmt.Printf("PASS %s\n", r.Path)
			}
			continue
		}
		failed++
		if r.Err != nil {
			fmt.Printf("FAIL %s\n  %s\n", r.Path, r.Context)
			continue
		}
		fmt.Printf("FAIL %s (dump line %d)\n%s", r.Path, r.Line, r.Context)
	}
	fmt.Printf("%d/%d files identical\n", len(results)-failed, len(results))
	if failed > 0 {
		os.Exit(1)
	}
}
