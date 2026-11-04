package coverage

import (
	"encoding/json"
	"fmt"
)

type llvmExport struct {
	Data []struct {
		Files []struct {
			Filename string  `json:"filename"`
			Branches [][]int `json:"branches"`
		} `json:"files"`
		Functions []struct {
			Filenames []string `json:"filenames"`
			Regions   [][]int  `json:"regions"`
		} `json:"functions"`
	} `json:"data"`
}

// llvm RegionKind values (llvm/ProfileData/Coverage/CoverageMapping.h).
const llvmCodeRegion = 0

// ParseLlvmCov reads `cargo llvm-cov report --json` (llvm-cov export). With
// branch records (--branch, nightly) a file gets one true and one false
// ArmHit per condition: granularity branch. Otherwise its code regions:
// granularity region. Regions of generic instantiations add up.
func ParseLlvmCov(data []byte) (*BranchSet, error) {
	var doc llvmExport
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("llvm-cov JSON: %v", err)
	}
	set := &BranchSet{}
	branched := map[string]bool{}
	for _, d := range doc.Data {
		for _, f := range d.Files {
			for _, b := range f.Branches {
				if len(b) < 6 {
					continue
				}
				bf := &BranchFile{Language: "rust", Granularity: GranBranch}
				for k, hits := range []int{b[4], b[5]} {
					bf.Arms = append(bf.Arms, ArmHit{Line: b[0], Column: b[1], EndLine: b[2], EndColumn: b[3],
						Index: k, Of: 2, Kind: KindLLVM, Hits: hits, Reached: b[4] + b[5]})
				}
				set.add(f.Filename, bf)
				branched[f.Filename] = true
			}
		}
	}
	index := map[string]map[[4]int]int{}
	for _, d := range doc.Data {
		for _, fn := range d.Functions {
			for _, r := range fn.Regions {
				if len(r) < 8 || r[7] != llvmCodeRegion || r[5] < 0 || r[5] >= len(fn.Filenames) {
					continue
				}
				name := fn.Filenames[r[5]]
				if branched[name] {
					continue
				}
				if index[name] == nil {
					index[name] = map[[4]int]int{}
					set.add(name, &BranchFile{Language: "rust", Granularity: GranRegion})
				}
				pos := [4]int{r[0], r[1], r[2], r[3]}
				f := set.Files[name]
				if at, ok := index[name][pos]; ok {
					f.Arms[at].Hits += r[4]
					continue
				}
				index[name][pos] = len(f.Arms)
				f.Arms = append(f.Arms, ArmHit{Line: r[0], Column: r[1], EndLine: r[2], EndColumn: r[3],
					Kind: KindRegion, Hits: r[4], Reached: -1})
			}
		}
	}
	return set, nil
}
