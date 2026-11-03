package mutate

// Ported from upstream tests/test_edn.py and the EDN part of
// tests/test_boundaries.py.

import (
	"reflect"
	"strings"
	"testing"
)

func mapOf(kv ...any) *ordMap {
	m := newOrdMap()
	for i := 0; i+1 < len(kv); i += 2 {
		m.Set(kv[i], kv[i+1])
	}
	return m
}

func TestEDNRoundTripMatchesTheSnapshotShape(t *testing.T) {
	form := mapOf("id", "defn/place", "kind", "defn", "file", "src/demo/Board.java", "line", 4, "end-line", 9,
		"hash", "abc", "killed", 1, "survived", 0, "uncovered", 2, "sites", 3)
	payload := mapOf("version", 1, "tested-at", "2026-09-30T12:00:00-05:00", "source", `src/demo/say "hi".py`,
		"namespace", "demo.Board",
		"outcomes", mapOf(`["src/a.py","demo.Board","defn/place",1,2,">",">="]`, kw("killed"),
			`["src/a.py","demo.Board","defn-/hide",3,4,"true","false"]`, kw("survived")),
		"forms", []any{form})
	text, err := ednDumps(payload)
	if err != nil {
		t.Fatal(err)
	}
	want := `{:version 1 :tested-at "2026-09-30T12:00:00-05:00" :source "src/demo/say \"hi\".py" :namespace "demo.Board" ` +
		`:outcomes {"[\"src/a.py\",\"demo.Board\",\"defn/place\",1,2,\">\",\">=\"]" :killed ` +
		`"[\"src/a.py\",\"demo.Board\",\"defn-/hide\",3,4,\"true\",\"false\"]" :survived} ` +
		`:forms [{:id "defn/place" :kind "defn" :file "src/demo/Board.java" :line 4 :end-line 9 :hash "abc" :killed 1 :survived 0 :uncovered 2 :sites 3}]}`
	if text != want {
		t.Fatalf("dumps:\n got %s\nwant %s", text, want)
	}
	parsed, err := ednLoads(text)
	if err != nil {
		t.Fatal(err)
	}
	m := parsed.(*ordMap)
	if m.Get("namespace") != "demo.Board" || m.Get("source") != `src/demo/say "hi".py` {
		t.Fatalf("namespace/source: %v %v", m.Get("namespace"), m.Get("source"))
	}
	f := m.Get("forms").([]any)[0].(*ordMap)
	if f.Get("id") != "defn/place" || f.Get("end-line") != int64(9) || f.Get("killed") != int64(1) {
		t.Fatalf("form: %v", pyRepr(f))
	}
	var statuses []string
	for _, p := range m.Get("outcomes").(*ordMap).Pairs() {
		statuses = append(statuses, p.Val.(string))
	}
	if !reflect.DeepEqual(statuses, []string{"killed", "survived"}) {
		t.Fatalf("keywords read back as %v", statuses)
	}
}

func TestEDNAtomsCollectionsAndNil(t *testing.T) {
	cases := map[string]any{
		"true": true, "false": false, "12": int64(12), "-3": int64(-3), "1.50": 1.5, "foo": "foo",
		"; kept\n12": int64(12), `"a\nb\\u00e9\q"`: "a\nb\\u00e9q", ":kw": "kw",
	}
	for src, want := range cases {
		got, err := ednLoads(src)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("loads(%q) = %#v, %v; want %#v", src, got, err, want)
		}
	}
	got, _ := ednLoads("{:forms [] :outcomes {} :note nil}")
	m := got.(*ordMap)
	if pyRepr(m) != "{'forms': [], 'outcomes': {}, 'note': None}" {
		t.Fatalf("got %s", pyRepr(m))
	}
	set, _ := ednLoads("#{1 2 1}")
	if !reflect.DeepEqual(set, ednSet{int64(1), int64(2)}) {
		t.Fatalf("set %#v", set)
	}
	nilKey, _ := ednLoads("{nil 1}")
	if p := nilKey.(*ordMap).Pairs(); len(p) != 1 || p[0].Key != nil || p[0].Val != int64(1) {
		t.Fatalf("nil key %#v", p)
	}
	dup, _ := ednLoads("{:a 1 :b 2 :a 3}")
	if pyRepr(dup) != "{'a': 3, 'b': 2}" {
		t.Fatalf("duplicate keys: %s", pyRepr(dup))
	}
}

func TestEDNErrors(t *testing.T) {
	for src, msg := range map[string]string{
		"": "empty EDN", "; hi": "empty EDN", "{": "unexpected end of EDN", `"hi`: "unterminated string",
		`"\`: "unterminated string escape", ":": "empty keyword", "1 2": "trailing EDN at 2",
		"#foo": "unsupported EDN dispatch",
	} {
		if _, err := ednLoads(src); err == nil || err.Error() != msg {
			t.Errorf("loads(%q) error %v, want %q", src, err, msg)
		}
	}
}

func TestEDNRenderingRules(t *testing.T) {
	text, err := ednDumps(mapOf("ok-key?", 1.0, "1bad", 2.5, "with space", []any{}, "z", newOrdMap(), "n", nil, "b", true))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{:ok-key? 1.0 "1bad" 2.5 "with space" [] :z {} :n nil :b true}`; text != want {
		t.Fatalf("got %s want %s", text, want)
	}
	if _, err := ednDumps(ednSet{int64(1)}); err == nil || !strings.Contains(err.Error(), "set") {
		t.Fatalf("a set must not render, got %v", err)
	}
}
