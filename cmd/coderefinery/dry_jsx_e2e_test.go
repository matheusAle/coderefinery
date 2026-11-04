package main

// End to end: `coderefinery dry --jsx` on fixtures/react-jsx (scratch
// copy), through the dispatcher: cards duplicated across files, option rows
// repeated in one component, a list item in a .jsx .map repeated behind &&,
// a near-duplicate card; a small exact copy (Badge) and different markup
// (Table) stay out.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const dryFunctionReport = `DUPLICATE score=1.00
  src/Settings.tsx:24-30
  src/TeamCard.tsx:21-27
`

const dryJSXSection = `
Duplicate JSX
=============

DUPLICATE score=1.00
  src/NavMenu.jsx:7-12 <li> 4 elements
  src/NavMenu.jsx:23-28 <li> 4 elements

DUPLICATE score=1.00
  src/ProfileCard.tsx:6-17 <section> 7 elements
  src/TeamCard.tsx:5-16 <section> 7 elements

DUPLICATE score=1.00
  src/Settings.tsx:9-13 <label> 4 elements
  src/Settings.tsx:14-18 <label> 4 elements

DUPLICATE score=0.84
  src/ArticleCard.tsx:7-19 <section> 8 elements
  src/ProfileCard.tsx:6-17 <section> 7 elements

DUPLICATE score=0.84
  src/ArticleCard.tsx:7-19 <section> 8 elements
  src/TeamCard.tsx:5-16 <section> 7 elements
`

func TestDryJSXOnTheFixture(t *testing.T) {
	root := copyFixture(t, "react-jsx")
	dryEDN := filepath.Join(root, ".metrics", "dry.edn")
	jsxEDN := filepath.Join(root, ".metrics", "dry-jsx.edn")

	code, stdout, stderr := runCapture([]string{"dry", "--root", root}, "v")
	if code != 0 || stdout != dryFunctionReport || stderr != "Wrote "+dryEDN+"\n" {
		t.Fatalf("plain dry: exit %d\n%s\n%s", code, stdout, stderr)
	}
	plain, _ := os.ReadFile(dryEDN)
	if _, err := os.Stat(jsxEDN); err == nil {
		t.Fatal("plain dry wrote dry-jsx.edn")
	}

	code, stdout, stderr = runCapture([]string{"dry", "--root", root, "--jsx"}, "v")
	if code != 0 || stdout != dryFunctionReport+dryJSXSection {
		t.Fatalf("--jsx: exit %d\n%s\nwant:\n%s%s", code, stdout, dryFunctionReport, dryJSXSection)
	}
	if stderr != "Wrote "+dryEDN+"\nWrote "+jsxEDN+"\n" {
		t.Fatalf("stderr %q", stderr)
	}
	if again, _ := os.ReadFile(dryEDN); string(again) != string(plain) {
		t.Fatalf("dry.edn differs under --jsx:\n%s", again)
	}
	// ArticleCard's extra <time> adds 5 fingerprints (time, its tag, "time",
	// and the changed header and section) to its side, 2 to the other: 37/44
	edn, _ := os.ReadFile(jsxEDN)
	wantFirst := "{:jsx-candidates [\n {:score 1.0\n" +
		`  :left {:file "src/NavMenu.jsx", :start-line 7, :end-line 12, :tag "li", :elements 4, :nodes 41}` + "\n" +
		`  :right {:file "src/NavMenu.jsx", :start-line 23, :end-line 28, :tag "li", :elements 4, :nodes 41}}` + "\n"
	if !strings.HasPrefix(string(edn), wantFirst) || strings.Count(string(edn), ":score") != 5 ||
		!strings.Contains(string(edn), " {:score 0.840909090909\n  :left {:file \"src/ArticleCard.tsx\"") {
		t.Fatalf("dry-jsx.edn:\n%s", edn)
	}
	// smoke: the small exact copy and the table never show up
	if strings.Contains(stdout, "Badge") || strings.Contains(stdout, "Table.tsx") || strings.Contains(stdout, "<span>") {
		t.Fatalf("unexpected pair:\n%s", stdout)
	}

	// stricter threshold: only the exact copies
	_, stdout, _ = runCapture([]string{"dry", "--root", root, "--jsx-threshold", "0.9"}, "v")
	if strings.Count(stdout, "DUPLICATE score=1.00") != 4 || strings.Contains(stdout, "ArticleCard") {
		t.Fatalf("--jsx-threshold 0.9:\n%s", stdout)
	}
}
