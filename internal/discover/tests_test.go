package discover

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIterFilesWithTestsDescendsIntoTestDirsOnly(t *testing.T) {
	root, _ := filepath.EvalSymlinks(t.TempDir())
	for _, rel := range []string{"src/app.py", "tests/test_app.py", "tests/unit/helpers.py", "target/gen.py", "node_modules/x.py"} {
		p := filepath.Join(root, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		_ = os.WriteFile(p, nil, 0o644)
	}
	any := func(p string) bool { return LanguageOf(p) != "" }
	want := []string{
		filepath.Join(root, "src", "app.py"),
		filepath.Join(root, "tests", "test_app.py"),
		filepath.Join(root, "tests", "unit", "helpers.py"),
	}
	if got := IterFilesWith([]string{root}, any, true); !reflect.DeepEqual(got, want) {
		t.Fatalf("with tests: %v", got)
	}
	if got := IterFilesWith([]string{root}, any, false); !reflect.DeepEqual(got, want[:1]) {
		t.Fatalf("without tests: %v", got)
	}
	if got, plain := IterFilesWith([]string{root}, IsSource, false), IterSourceFiles([]string{root}); !reflect.DeepEqual(got, plain) {
		t.Fatalf("includeTests=false is IterFiles: %v vs %v", got, plain)
	}
}
