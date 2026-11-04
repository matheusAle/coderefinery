package syntax

import "testing"

func sample() (*Node, []byte) {
	src := []byte("x = 1\n")
	x := Leaf("identifier", true, 0, 1).WithField("left")
	eq := Leaf("=", false, 2, 3)
	one := Leaf("integer", true, 4, 5).WithField("right")
	assign := Inner("assignment", 0, x, eq, one)
	root := Inner("module", 0, assign)
	root.EndByte = len(src)
	return Finish(root, src), src
}

func TestDumpIsTheOracleFormat(t *testing.T) {
	root, src := sample()
	want := "module [0,6) 0:0-1:0\n" +
		"  assignment [0,5) 0:0-0:5\n" +
		"    left: identifier [0,1) 0:0-0:1 \"x\"\n" +
		"    \"=\" [2,3) 0:2-0:3 \"=\"\n" +
		"    right: integer [4,5) 0:4-0:5 \"1\"\n"
	if got := Dump(root, src); got != want {
		t.Fatalf("Dump:\n%s\nwant:\n%s", got, want)
	}
	if got := SExp(root); got != "(module (assignment left: (identifier) right: (integer)))" {
		t.Fatalf("SExp = %s", got)
	}
}

func TestAccessorsMirrorPyTreeSitter(t *testing.T) {
	root, src := sample()
	assign := root.Child(0)
	if assign.ChildByFieldName("right").Text(src) != "1" || assign.NamedChildCount() != 2 || assign.Parent != root {
		t.Fatal("field/named accessors")
	}
	if root.EndLine() != 1 || root.StartLine() != 1 { // ends at column 0 of row 1
		t.Fatalf("lines %d-%d", root.StartLine(), root.EndLine())
	}
	if assign.ChildOfType("=", "integer").Type != "=" || assign.Child(0).NextSibling().Type != "=" {
		t.Fatal("child_of_type / next_sibling")
	}
	var order []string
	root.Descendants(func(n *Node) bool { order = append(order, n.Type); return true })
	if got := len(order); got != 5 || order[1] != "assignment" || order[4] != "integer" {
		t.Fatalf("preorder %v", order)
	}
	if Quote("a\"\\\n\x01é") != `"a\"\\\n\x01é"` {
		t.Fatalf("Quote = %s", Quote("a\"\\\n\x01é"))
	}
}
