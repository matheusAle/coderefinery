package coverage

import (
	"encoding/xml"
	"strings"
)

// ParseJacocoLines reads jacoco.xml <line nr mi ci mb cb> per sourcefile.
// JaCoCo says how many branches of a line were missed, not which ones.
// Granularity: branch-count. Keys are "<package>/<sourcefile>".
func ParseJacocoLines(text string) (*BranchSet, error) {
	dec := xml.NewDecoder(strings.NewReader(doctype.ReplaceAllString(text, "")))
	dec.Strict = true
	var root xmlElem
	if err := dec.Decode(&root); err != nil {
		return nil, err
	}
	set := &BranchSet{}
	var visit func(e xmlElem, pkg string) error
	visit = func(e xmlElem, pkg string) error {
		switch e.XMLName.Local {
		case "package":
			pkg, _ = e.attr("name")
		case "sourcefile":
			name, _ := e.attr("name")
			key := name
			if pkg != "" {
				key = pkg + "/" + name
			}
			f := &BranchFile{Language: "java", Granularity: GranBranchCount}
			for _, l := range e.Children {
				if l.XMLName.Local != "line" {
					continue
				}
				var v [4]int
				for i, a := range []string{"nr", "ci", "mb", "cb"} {
					n, err := attrInt(l, a)
					if err != nil {
						return err
					}
					v[i] = n
				}
				if v[2]+v[3] > 0 {
					f.Arms = append(f.Arms, ArmHit{Line: v[0], Kind: KindLineCount, Of: v[2] + v[3], Hits: v[3], Reached: v[1]})
				}
			}
			if len(f.Arms) > 0 {
				set.add(key, f)
			}
			return nil
		}
		for _, c := range e.Children {
			if err := visit(c, pkg); err != nil {
				return err
			}
		}
		return nil
	}
	return set, visit(root, "")
}
