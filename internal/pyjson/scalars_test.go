package pyjson

import (
	"math"
	"strings"
	"testing"
)

func TestDumpsScalarsAndSeparatorsMatchPythonJSON(t *testing.T) {
	// CPython: json.dumps([None, True, False, 2**40, 1.0, 0.1, 1e22,
	// inf, -inf, nan, "é", [[]]])
	v := []any{nil, true, false, int64(1) << 40, 1.0, 0.1, 1e22, math.Inf(1), math.Inf(-1), math.NaN(), "é", []any{[]any{}}}
	want := `[null, true, false, 1099511627776, 1.0, 0.1, 1e+22, Infinity, -Infinity, NaN, "` + bs + `u00e9", [[]]]`
	if got := Dumps(v, ", ", ": "); got != want {
		t.Errorf("Dumps defaults:\n got  %s\n want %s", got, want)
	}
	// json.dumps([1, [2, "a"]], separators=(",", ":"))
	if got := Dumps([]any{1, []any{2, "a"}}, ",", ":"); got != `[1,[2,"a"]]` {
		t.Errorf("Dumps compact = %s", got)
	}
}

func TestDumpsPanicsOnTypesNoCallerWrites(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(r.(string), "unsupported type map[string]int") {
			t.Fatalf("recovered %v", r)
		}
	}()
	Dumps(map[string]int{}, ",", ":")
}
