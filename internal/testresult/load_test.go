package testresult

import (
	"path/filepath"
	"testing"

	"github.com/matheusAle/coderefinery/internal/testutil"
)

func TestLoadFindsAGoJSONStreamBehindBuildOutput(t *testing.T) {
	path := testutil.WriteFile(t, filepath.Join(t.TempDir(), "results.json"),
		"# m/p\nbuilding...\n"+
			`{"Time":"2026-01-01T00:00:00Z","Action":"run","Package":"m/p","Test":"TestA"}`+"\n"+
			`{"Action":"pass","Package":"m/p","Test":"TestA","Elapsed":0.5}`+"\n")
	r, err := Load(path, "", "")
	if err != nil || len(r.Cases) != 1 {
		t.Fatalf("Load = %+v, %v", r, err)
	}
	if c := r.Cases[0]; c.ID != "m/p::TestA" || c.Status != Pass || c.Duration != 0.5 {
		t.Errorf("case %+v", c)
	}
}

func TestLoadRejectsTextThatIsNeitherFormat(t *testing.T) {
	path := testutil.WriteFile(t, filepath.Join(t.TempDir(), "results.txt"), "PASS\nok  m/p 0.1s\n")
	if r, err := Load(path, "", ""); r != nil || err == nil || err.Error() != path+" is neither JUnit XML nor go test -json" {
		t.Fatalf("Load = %+v, %v", r, err)
	}
	// A tool hint other than go never sniffs for a buried stream.
	stream := testutil.WriteFile(t, filepath.Join(t.TempDir(), "r.json"), "noise\n"+`{"Action":"run","Package":"m/p","Test":"TestA"}`+"\n")
	if _, err := Load(stream, "", "pytest"); err == nil {
		t.Fatal("pytest hint: want an error")
	}
}
