package testresult

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Load reads a results file or a directory of JUnit files (Surefire's
// TEST-*.xml). It sniffs '<' (JUnit XML) or '{' (go test -json).
func Load(path, root, toolHint string) (*Run, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		matches, _ := filepath.Glob(filepath.Join(path, "*.xml"))
		sort.Strings(matches)
		if len(matches) == 0 {
			return nil, fmt.Errorf("no JUnit XML files in %s", path)
		}
		var runs []*Run
		for _, m := range matches {
			r, err := loadFile(m, root, toolHint)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", m, err)
			}
			runs = append(runs, r)
		}
		return Merge(runs...), nil
	}
	return loadFile(path, root, toolHint)
}

func loadFile(path, root, toolHint string) (*Run, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	br := bufio.NewReader(f)
	first, err := firstByte(br)
	if err != nil {
		return nil, fmt.Errorf("%s is empty", path)
	}
	if first == '{' || toolHint == "go" {
		return ParseGoJSON(br, root)
	}
	if first != '<' {
		// go test -json with build output before the stream
		if toolHint == "" && looksLikeGoJSON(path) {
			f2, err := os.Open(path)
			if err != nil {
				return nil, err
			}
			defer f2.Close()
			return ParseGoJSON(f2, root)
		}
		return nil, fmt.Errorf("%s is neither JUnit XML nor go test -json", path)
	}
	return ParseJUnit(br, root, toolHint)
}

func firstByte(br *bufio.Reader) (byte, error) {
	for {
		b, err := br.ReadByte()
		if err != nil {
			return 0, err
		}
		if b == 0xEF { // UTF-8 BOM
			_, _ = br.Discard(2)
			continue
		}
		if b != ' ' && b != '\n' && b != '\r' && b != '\t' {
			return b, br.UnreadByte()
		}
	}
}

func looksLikeGoJSON(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for i := 0; i < 200 && sc.Scan(); i++ {
		if strings.HasPrefix(strings.TrimSpace(sc.Text()), `{"Time"`) || strings.HasPrefix(strings.TrimSpace(sc.Text()), `{"Action"`) {
			return true
		}
	}
	return false
}
