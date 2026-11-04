// Command bench times commands and summarises upstream-vs-ubgate results.
// Stdlib only.
//
//	bench run    [flags] -- cmd args...   time a command N times, append JSON lines
//	bench report [flags]                  results.jsonl (+ parity.jsonl) -> markdown + CSV
//	bench parity [flags]                  compare two output files after path masking
//	bench count  -root DIR                files/LOC of the set the tools analyse
//
// See bench/README.md.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "run":
		err = runMain(os.Args[2:])
	case "report":
		err = reportMain(os.Args[2:])
	case "parity":
		err = parityMain(os.Args[2:])
	case "count":
		err = countMain(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "bench:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage:
  bench run    [flags] -- cmd args...   time a command (see bench run -h)
  bench report [flags]                  summarise results (see bench report -h)
  bench parity [flags]                  compare two outputs (see bench parity -h)
  bench count  -root DIR                count files/LOC the tools analyse
`)
}
