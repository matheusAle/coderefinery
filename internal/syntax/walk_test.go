package syntax

import (
	"reflect"
	"testing"
)

func TestWalkSkipsTheSubtreeWhenVisitSaysNo(t *testing.T) {
	root, _ := sample()
	var seen []string
	Walk(root, func(n *Node) bool { seen = append(seen, n.Type); return n.Type != "assignment" })
	if !reflect.DeepEqual(seen, []string{"module", "assignment"}) {
		t.Fatalf("pruned walk %v", seen)
	}
	seen = nil
	Walk(root, func(n *Node) bool { seen = append(seen, n.Type); return true })
	if want := []string{"module", "assignment", "identifier", "=", "integer"}; !reflect.DeepEqual(seen, want) {
		t.Fatalf("full walk %v, want %v", seen, want)
	}
}

func TestSiblingParentAndChildListAccessors(t *testing.T) {
	root, _ := sample()
	assign := root.Child(0)
	x, eq, one := assign.Child(0), assign.Child(1), assign.Child(2)
	if one.PrevSibling() != eq || eq.PrevSibling() != x || x.PrevSibling() != nil || root.PrevSibling() != nil {
		t.Error("PrevSibling")
	}
	var up []string
	one.Ancestors(func(n *Node) bool { up = append(up, n.Type); return true })
	if !reflect.DeepEqual(up, []string{"assignment", "module"}) {
		t.Errorf("Ancestors %v", up)
	}
	up = nil
	one.Ancestors(func(n *Node) bool { up = append(up, n.Type); return false })
	if !reflect.DeepEqual(up, []string{"assignment"}) {
		t.Errorf("Ancestors stops when yield says so: %v", up)
	}
	if got := assign.NamedChildren(); !reflect.DeepEqual(got, []*Node{x, one}) {
		t.Errorf("NamedChildren %v", types(got))
	}
	if got := assign.ChildrenByFieldName("right"); !reflect.DeepEqual(got, []*Node{one}) || assign.ChildrenByFieldName("body") != nil {
		t.Errorf("ChildrenByFieldName %v", types(got))
	}
}

func TestHasErrorFindsErrorAndMissingNodesAnywhere(t *testing.T) {
	root, _ := sample()
	if root.HasError() {
		t.Fatal("a clean tree has no error")
	}
	root.Child(0).Child(2).Missing = true
	if !root.HasError() || root.Child(0).Child(0).HasError() {
		t.Fatal("a missing leaf is an error of its ancestors only")
	}
	root.Child(0).Child(2).Missing = false
	root.Child(0).Child(1).Type = "ERROR"
	if !root.HasError() {
		t.Fatal("an ERROR node is an error")
	}
}
