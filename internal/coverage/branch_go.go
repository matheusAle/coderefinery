package coverage

import (
	"fmt"
	"strings"

	"github.com/matheusAle/coderefinery/internal/pyfmt"
)

// ParseGoBlocks reads a Go coverprofile keeping block columns. Go's cover
// tool counts basic blocks: an `if` body is a block, its condition is not.
// Granularity: block (never called a branch). Repeated blocks (one per test
// binary) add up.
func ParseGoBlocks(text string) (*BranchSet, error) {
	set := &BranchSet{}
	index := map[string]map[[4]int]int{}
	for i, raw := range SplitLines(text) {
		line := PyStrip(raw)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		bad := fmt.Errorf("invalid coverage segment on line %d: %s", i+1, pyfmt.PyRepr(raw))
		fields := strings.FieldsFunc(line, isPySpace)
		if len(fields) != 3 {
			return nil, bad
		}
		colon := strings.LastIndexByte(fields[0], ':')
		if colon < 0 {
			return nil, bad
		}
		file := fields[0][:colon]
		var pos [4]int
		var hits, stmts int
		if _, err := fmt.Sscanf(fields[0][colon+1:]+" "+fields[1]+" "+fields[2], "%d.%d,%d.%d %d %d",
			&pos[0], &pos[1], &pos[2], &pos[3], &stmts, &hits); err != nil {
			return nil, bad
		}
		if index[file] == nil {
			index[file] = map[[4]int]int{}
			set.add(file, &BranchFile{Language: "go", Granularity: GranBlock})
		}
		f := set.Files[file]
		if at, ok := index[file][pos]; ok {
			f.Arms[at].Hits += hits
			continue
		}
		index[file][pos] = len(f.Arms)
		f.Arms = append(f.Arms, ArmHit{Line: pos[0], Column: pos[1], EndLine: pos[2], EndColumn: pos[3],
			Kind: KindBlock, Hits: hits, Reached: -1})
	}
	return set, nil
}
