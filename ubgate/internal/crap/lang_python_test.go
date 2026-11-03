package crap

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

type fnRow struct {
	Namespace, Name string
	CC              int
}

func rowsOf(t *testing.T, source, path, root string) []fnRow {
	t.Helper()
	fns, err := PythonFunctions(source, path, root)
	if err != nil {
		t.Fatalf("PythonFunctions: %v", err)
	}
	var rows []fnRow
	for _, f := range fns {
		if f.Language != "python" || f.Path != path || f.StartByte != -1 {
			t.Fatalf("unexpected metadata %+v", f)
		}
		rows = append(rows, fnRow{f.Namespace, f.Name, f.Complexity})
	}
	return rows
}

func expectRows(t *testing.T, got []fnRow, want ...fnRow) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("functions:\n got  %v\n want %v", got, want)
	}
}

// Ported from upstream tests/test_python.py.
func TestPythonFunctionMethodAndNestedFunction(t *testing.T) {
	source := `
def choose(x, ready):
    if x > 0 and x < 10:
        return 1
    elif x == 0 or ready:
        return 0
    for i in xs:
        if i == 2:
            return i
    while ready:
        break
    try:
        return x
    except ValueError:
        return 0
    except KeyError:
        return -1
    match x:
        case 1:
            return 1
        case 2 | 3:
            return 2
        case _:
            return 0
    return x if x > 0 else 0

class Box:
    def open(self, flag=False):
        return 1 if flag else 0

    @staticmethod
    def shut(self):
        xs = [n for n in range(3) if n]
        return xs

def outer():
    def inner(n):
        if n:
            return 1
        return 0
    return inner(1)

async def load():
    if True:
        return 1
`
	expectRows(t, rowsOf(t, source, "src/demo/box.py", "/proj"),
		fnRow{"demo.box", "choose", 14}, fnRow{"demo.box.Box", "open", 2}, fnRow{"demo.box.Box", "shut", 2},
		fnRow{"demo.box", "outer", 2}, fnRow{"demo.box", "load", 2})
}

func TestPythonNestedClassMethodIsItsOwnEntry(t *testing.T) {
	source := "class Outer:\n    class Inner:\n        def tick(self, ready, ok):\n            if ready and ok:\n                return 1\n            return 0\n\ndef factory():\n    class Hidden:\n        def secret(self):\n            if True:\n                return 1\n            return 0\n    return Hidden()\n"
	expectRows(t, rowsOf(t, source, "src/demo/sample.py", "/proj"),
		fnRow{"demo.sample.Outer.Inner", "tick", 3}, fnRow{"demo.sample", "factory", 1}, fnRow{"demo.sample.Hidden", "secret", 2})
}

func TestPythonNamespaceStripsRootsSrcAndInit(t *testing.T) {
	cases := []struct{ path, root, want string }{
		{"/proj/src/demo/app.py", "/proj", "demo.app"},
		{"./src/demo/app.py", ".", "demo.app"},
		{"pkg/src/demo/__init__.py", "", "demo"},
		{"__init__.py", "", "__init__"},
		{"src/demo/__init__.py", "", "demo"},
		{"/other/x.py", "/proj", ".other.x"}, // spec §9.17: leading dot preserved
	}
	for _, c := range cases {
		if got := PythonModuleNamespace(c.path, c.root); got != c.want {
			t.Errorf("namespace(%q, %q) = %q, want %q", c.path, c.root, got, c.want)
		}
	}
	rows := rowsOf(t, "def f():\n    return 1\n", "/proj/src/demo/app.py", "/proj")
	expectRows(t, rows, fnRow{"demo.app", "f", 1})
}

func TestPythonKeywordsInStringsAndCommentsDoNotCount(t *testing.T) {
	source := "def literal():\n    \"\"\"if and or for while except\"\"\"\n    text = \"if x and y\"\n    # if x: return 1\n    return text\n"
	expectRows(t, rowsOf(t, source, "src/demo/__init__.py", "/proj"), fnRow{"demo", "literal", 1})
}

func TestPythonHugeExpressionDoesNotOverflow(t *testing.T) {
	source := "def huge():\n    return " + strings.Repeat("1 + ", 1199) + "1\n"
	expectRows(t, rowsOf(t, source, "huge.py", "/proj"), fnRow{"huge", "huge", 1})
}

// Observed upstream behaviour, spec-crapper §8.1 [obs].
func TestPythonObservedComplexities(t *testing.T) {
	cases := []struct {
		source string
		want   []fnRow
	}{
		{"def f(a,b):\n    return f\"{a if b else 0}\"\n", []fnRow{{"a", "f", 2}}},
		{"def f(x):\n    match x:\n        case 1 if x > 0:\n            pass\n        case _:\n            pass\n", []fnRow{{"a", "f", 4}}},
		{"def f():\n    try:\n        pass\n    except* ValueError:\n        pass\n", []fnRow{{"a", "f", 2}}},
		{"async def f(xs):\n    async for x in xs:\n        pass\n    async with a:\n        pass\n    return [y async for y in xs if y]\n", []fnRow{{"a", "f", 3}}},
		{"def f(a,b,c):\n    return a and b or c and not a\n", []fnRow{{"a", "f", 4}}},
		{"def f(xs):\n    return [x for ys in xs if ys for x in ys if x if x > 1]\n", []fnRow{{"a", "f", 4}}},
		{"def f(x):\n    while x:\n        pass\n    else:\n        pass\n    for i in x:\n        pass\n    else:\n        pass\n    assert x and x\n    with open(x) as fh: pass\n    y = (z := 3)\n", []fnRow{{"a", "f", 4}}},
		{"match = 1\ndef f(match):\n    match(match)\n    return match\n", []fnRow{{"a", "f", 1}}},
		{"def f():\n    def g():\n        class C:\n            def m(self):\n                if a: pass\n        return C\n    return g\n", []fnRow{{"a", "f", 1}, {"a.C", "m", 2}}},
		{"class A:\n    def m(self):\n        class B:\n            def n(self):\n                if a: pass\n        return B\n", []fnRow{{"a.A", "m", 1}, {"a.A.B", "n", 2}}},
		{"def f(x):\n    return {k: v for k, v in x if k} or {a for a in x if a} or (g for g in x if g)\n", []fnRow{{"a", "f", 6}}},
		{"def f(x):\n    if x:\n        pass\n    elif x:\n        pass\n    else:\n        pass\n", []fnRow{{"a", "f", 3}}},
		{"def f(x):\n    print(f\"{x!r:>{10 if x else 5}}\")\n", []fnRow{{"a", "f", 2}}},
		{"def f(x):\n    return f\"{'a' if x else 'b'}\" + rf\"{x and y}\"\n", []fnRow{{"a", "f", 3}}},
		{"def f(x):\n    try:\n        pass\n    except (A, B):\n        pass\n    except:\n        pass\n    finally:\n        pass\n", []fnRow{{"a", "f", 3}}},
		{"def f(x):\n    match x:\n        case [1, 2] | {\"a\": 1}:\n            pass\n        case Point(x=0) if x and y:\n            pass\n", []fnRow{{"a", "f", 5}}},
		{"class A:\n    x = 1 if a else 2\n    def m(self): pass\n", []fnRow{{"a.A", "m", 1}}},
	}
	for i, c := range cases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			expectRows(t, rowsOf(t, c.source, "a.py", "/proj"), c.want...)
		})
	}
}

func TestPythonLines(t *testing.T) {
	cases := []struct {
		source string
		want   [][2]int
	}{
		{"g = lambda x: 1 if x else 0\ndef f(x=1 if a else 2):\n    h = lambda y: y and 1\n    return h\n", [][2]int{{2, 4}}},
		{"class A:\n    @property\n    @other\n    def m(self):\n        return 1\n", [][2]int{{4, 5}}},
		{"def f():\n    return 1\n    # trailing comment\n\n# outer\ndef g():\n    pass\n", [][2]int{{1, 3}, {6, 7}}},
	}
	for _, c := range cases {
		fns, err := PythonFunctions(c.source, "a.py", "/proj")
		if err != nil {
			t.Fatal(err)
		}
		var got [][2]int
		for _, f := range fns {
			got = append(got, [2]int{f.StartLine, f.EndLine})
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("lines of %q = %v, want %v", c.source, got, c.want)
		}
	}
	fns, _ := PythonFunctions("g = lambda x: 1 if x else 0\ndef f(x=1 if a else 2):\n    h = lambda y: y and 1\n    return h\n", "a.py", "/proj")
	if len(fns) != 1 || fns[0].Complexity != 3 {
		t.Errorf("lambda/default decisions: %+v", fns)
	}
}
