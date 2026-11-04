package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strings"
)

type key struct{ Workload, Repo, Variant string }

type stats struct {
	N                  int
	Wall, CPU, RSS     []float64 // measured runs
	First              float64   // first run (warm-up, or the single slow run)
	TimedOut, Single   bool
	Timeout            string
	Exit               int
	Load1Min, Load1Max float64
	ChildCPU           []float64
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return math.NaN()
	}
	c := append([]float64(nil), xs...)
	sort.Float64s(c)
	n := len(c)
	if n%2 == 1 {
		return c[n/2]
	}
	return (c[n/2-1] + c[n/2]) / 2
}

func minmax(xs []float64) (float64, float64) {
	if len(xs) == 0 {
		return math.NaN(), math.NaN()
	}
	lo, hi := xs[0], xs[0]
	for _, x := range xs {
		lo, hi = math.Min(lo, x), math.Max(hi, x)
	}
	return lo, hi
}

func readJSONL[T any](path string) ([]T, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []T
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var v T
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out = append(out, v)
	}
	return out, sc.Err()
}

func reportMain(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	var (
		in      = fs.String("in", "", "results.jsonl (repeatable via comma list)")
		parityF = fs.String("parity", "", "parity.jsonl (optional)")
		md      = fs.String("md", "", "write markdown summary here (default stdout)")
		csvOut  = fs.String("csv", "", "write per-case CSV here")
		order   = fs.String("order", "help,crap,dry,mutate-scan,mutate-full", "workload order")
		base    = fs.String("base", "upstream", "baseline implementation")
		cand    = fs.String("cand", "coderefinery", "candidate implementation")
	)
	fs.Parse(args)
	var results []Result
	for _, p := range strings.Split(*in, ",") {
		rs, err := readJSONL[Result](p)
		if err != nil {
			return err
		}
		results = append(results, rs...)
	}
	var parities []Parity
	if *parityF != "" {
		ps, err := readJSONL[Parity](*parityF)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		parities = ps
	}

	// Later results for the same (key, impl) replace earlier ones, so a rerun
	// of one case appended to the same file wins.
	type ik struct {
		k    key
		impl string
	}
	batches := map[ik][]Result{}
	var keys []key
	seen := map[key]bool{}
	for _, r := range results {
		k := key{r.Workload, r.Repo, r.Variant}
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
		id := ik{k, r.Impl}
		if cur := batches[id]; len(cur) > 0 && cur[0].Batch != r.Batch {
			batches[id] = nil // a later invocation replaces the earlier one
		}
		batches[id] = append(batches[id], r)
	}
	agg := func(rs []Result) *stats {
		if len(rs) == 0 {
			return nil
		}
		s := &stats{Load1Min: math.Inf(1), Load1Max: math.Inf(-1), First: rs[0].WallSec}
		for _, r := range rs {
			s.Load1Min, s.Load1Max = math.Min(s.Load1Min, r.Load1), math.Max(s.Load1Max, r.Load1)
			if r.Warmup {
				continue
			}
			s.N++
			s.Wall = append(s.Wall, r.WallSec)
			s.CPU = append(s.CPU, r.CPUSec)
			s.RSS = append(s.RSS, float64(r.MaxRSS))
			s.ChildCPU = append(s.ChildCPU, r.ChildCPUSec)
			s.TimedOut = s.TimedOut || r.TimedOut
			s.Single = s.Single || r.Single
			s.Timeout = r.Timeout
			if r.Exit != 0 {
				s.Exit = r.Exit
			}
		}
		return s
	}

	rank := map[string]int{}
	for i, w := range strings.Split(*order, ",") {
		rank[w] = i + 1
	}
	sort.SliceStable(keys, func(i, j int) bool {
		ri, rj := rank[keys[i].Workload], rank[keys[j].Workload]
		if ri == 0 {
			ri = 1000
		}
		if rj == 0 {
			rj = 1000
		}
		return ri < rj
	})

	parityOf := map[key][]Parity{}
	for _, p := range parities {
		k := key{p.Workload, p.Repo, ""}
		parityOf[k] = append(parityOf[k], p)
	}

	var w io.Writer = os.Stdout
	if *md != "" {
		f, err := os.Create(*md)
		if err != nil {
			return err
		}
		defer f.Close()
		w = f
	}
	var cw *csv.Writer
	if *csvOut != "" {
		f, err := os.Create(*csvOut)
		if err != nil {
			return err
		}
		defer f.Close()
		cw = csv.NewWriter(f)
		defer cw.Flush()
		cw.Write([]string{"workload", "repo", "variant", "impl", "runs", "wall_median_s", "wall_min_s", "wall_max_s",
			"first_run_wall_s", "cpu_median_s", "maxrss_median_mib", "maxrss_max_mib", "exit", "timed_out", "single", "load1_min", "load1_max"})
	}

	fmt.Fprintf(w, "| workload | repo | %s wall (min–max) | %s wall (min–max) | speedup | %s CPU | %s CPU | CPU ratio | %s max RSS | %s max RSS | mem ratio | runs | load1 | parity |\n",
		*base, *cand, *base, *cand, *base, *cand)
	fmt.Fprintln(w, "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|---|")
	geo := map[string][]float64{}
	var workloadsSeen []string
	for _, k := range keys {
		a := agg(batches[ik{k, *base}])
		b := agg(batches[ik{k, *cand}])
		for _, pair := range []struct {
			impl string
			s    *stats
		}{{*base, a}, {*cand, b}} {
			if cw == nil || pair.s == nil {
				continue
			}
			s := pair.s
			lo, hi := minmax(s.Wall)
			_, rhi := minmax(s.RSS)
			cw.Write([]string{k.Workload, k.Repo, k.Variant, pair.impl, fmt.Sprint(s.N),
				f3(median(s.Wall)), f3(lo), f3(hi), f3(s.First), f3(median(s.CPU)),
				f1(median(s.RSS) / (1 << 20)), f1(rhi / (1 << 20)), fmt.Sprint(s.Exit), fmt.Sprint(s.TimedOut), fmt.Sprint(s.Single),
				f1(s.Load1Min), f1(s.Load1Max)})
		}
		repo := k.Repo
		if k.Variant != "" {
			repo += " (" + k.Variant + ")"
		}
		speed, cpuR, memR := "–", "–", "–"
		if a != nil && b != nil {
			sp := median(a.Wall) / median(b.Wall)
			speed = ratio(sp, a.TimedOut)
			cpuR = ratio(median(a.CPU)/median(b.CPU), a.TimedOut)
			memR = ratio(median(a.RSS)/median(b.RSS), false)
			if !a.TimedOut && !b.TimedOut {
				if _, ok := geo[k.Workload]; !ok {
					workloadsSeen = append(workloadsSeen, k.Workload)
				}
				geo[k.Workload] = append(geo[k.Workload], sp)
			}
		}
		runs := fmt.Sprintf("%s/%s", runsOf(a), runsOf(b))
		load := "–"
		if a != nil || b != nil {
			lo, hi := math.Inf(1), math.Inf(-1)
			for _, s := range []*stats{a, b} {
				if s != nil {
					lo, hi = math.Min(lo, s.Load1Min), math.Max(hi, s.Load1Max)
				}
			}
			load = fmt.Sprintf("%.1f–%.1f", lo, hi)
		}
		fmt.Fprintf(w, "| %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s | %s |\n",
			k.Workload, repo, wallCell(a), wallCell(b), speed, cpuCell(a), cpuCell(b), cpuR,
			rssCell(a), rssCell(b), memR, runs, load, parityCell(parityOf[key{k.Workload, k.Repo, ""}]))
	}
	if len(workloadsSeen) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "| workload | cases | geomean speedup | min | max |")
		fmt.Fprintln(w, "|---|---:|---:|---:|---:|")
		for _, wl := range workloadsSeen {
			xs := geo[wl]
			lo, hi := minmax(xs)
			fmt.Fprintf(w, "| %s | %d | %s | %s | %s |\n", wl, len(xs), ratio(geomean(xs), false), ratio(lo, false), ratio(hi, false))
		}
	}
	return nil
}

func geomean(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += math.Log(x)
	}
	return math.Exp(s / float64(len(xs)))
}

func runsOf(s *stats) string {
	switch {
	case s == nil:
		return "0"
	case s.Single:
		return "1*"
	}
	return fmt.Sprint(s.N)
}

func f3(x float64) string { return fmt.Sprintf("%.3f", x) }
func f1(x float64) string { return fmt.Sprintf("%.1f", x) }

func dur(sec float64) string {
	switch {
	case math.IsNaN(sec):
		return "–"
	case sec < 1:
		return fmt.Sprintf("%.0f ms", sec*1000)
	case sec < 100:
		return fmt.Sprintf("%.2f s", sec)
	}
	return fmt.Sprintf("%.0f s", sec)
}

func wallCell(s *stats) string {
	if s == nil {
		return "–"
	}
	if s.TimedOut {
		return "> " + s.Timeout + " (timeout)"
	}
	lo, hi := minmax(s.Wall)
	cell := dur(median(s.Wall))
	if s.N > 1 {
		cell += fmt.Sprintf(" (%s–%s)", strings.TrimSuffix(strings.TrimSuffix(dur(lo), " s"), " ms"), dur(hi))
	}
	if s.Exit != 0 {
		cell += fmt.Sprintf(" exit %d", s.Exit)
	}
	return cell
}

func cpuCell(s *stats) string {
	if s == nil {
		return "–"
	}
	if s.TimedOut {
		return "> " + dur(median(s.CPU))
	}
	return dur(median(s.CPU))
}

func rssCell(s *stats) string {
	if s == nil {
		return "–"
	}
	mib := median(s.RSS) / (1 << 20)
	if mib >= 1024 {
		return fmt.Sprintf("%.2f GiB", mib/1024)
	}
	return fmt.Sprintf("%.0f MiB", mib)
}

func ratio(x float64, lowerBound bool) string {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return "–"
	}
	p := ""
	if lowerBound {
		p = "> "
	}
	switch {
	case x >= 100:
		return fmt.Sprintf("%s%.0f×", p, x)
	case x >= 10:
		return fmt.Sprintf("%s%.1f×", p, x)
	}
	return fmt.Sprintf("%s%.2f×", p, x)
}

func parityCell(ps []Parity) string {
	if len(ps) == 0 {
		return "–"
	}
	// keep the last comparison per artifact
	last := map[string]Parity{}
	var order []string
	for _, p := range ps {
		if _, ok := last[p.Artifact]; !ok {
			order = append(order, p.Artifact)
		}
		last[p.Artifact] = p
	}
	ok := 0
	var bad []string
	for _, a := range order {
		p := last[a]
		if p.Identical {
			ok++
			continue
		}
		if p.Missing != "" {
			bad = append(bad, fmt.Sprintf("%s missing in %s", short(a), p.Missing))
		} else {
			bad = append(bad, fmt.Sprintf("%s %.1f%%", short(a), 100*p.MatchRate))
		}
	}
	if len(bad) == 0 {
		return fmt.Sprintf("identical (%d/%d)", ok, len(order))
	}
	return fmt.Sprintf("%d/%d identical; %s", ok, len(order), strings.Join(bad, ", "))
}

func short(a string) string {
	a = strings.TrimPrefix(a, ".metrics/")
	return a
}
