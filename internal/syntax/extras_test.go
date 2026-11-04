package syntax

import (
	"reflect"
	"testing"
)

func types(ns []*Node) []string {
	var out []string
	for _, n := range ns {
		out = append(out, n.Type)
	}
	return out
}

func TestBuildSkipsNilChildrenAndSetFieldIsNilSafe(t *testing.T) {
	a, b := Leaf("a", true, 2, 3), Leaf("b", true, 5, 9)
	n := Build("pair", nil, SetField("left", a), nil, b)
	if !reflect.DeepEqual(types(n.Children), []string{"a", "b"}) || n.StartByte != 2 || n.EndByte != 9 || !n.Named {
		t.Fatalf("Build = %s %v [%d,%d)", n.Type, types(n.Children), n.StartByte, n.EndByte)
	}
	if a.Field != "left" || SetField("x", nil) != nil {
		t.Fatalf("SetField: %q", a.Field)
	}
}

// block [0,20) holds call [2,12) = ( name [3,7) args [8,11) ) and end [15,20).
func extrasTree() (block, call *Node) {
	name, args := Leaf("name", true, 3, 7), Leaf("args", true, 8, 11)
	call = Inner("call", 0, Leaf("(", false, 2, 3), name, args, Leaf(")", false, 11, 12))
	return Inner("block", 0, call, Inner("end", 0, Leaf("x", true, 15, 20))), call
}

func TestExtraParentPicksTheDeepestEnclosingInnerChild(t *testing.T) {
	block, call := extrasTree()
	cases := []struct {
		name           string
		start, end     int
		startInclusive bool
		want           *Node
	}{
		{"between two children of call", 7, 8, false, call},
		{"between block's children", 12, 14, false, block},
		{"starts with call: strict", 2, 3, false, block},
		{"starts with call: inclusive", 2, 3, true, call},
		{"past the end", 21, 22, false, block},
	}
	for _, c := range cases {
		ex := Leaf("comment", true, c.start, c.end)
		if got := ExtraParent(block, ex, c.startInclusive); got != c.want {
			t.Errorf("%s: ExtraParent = %s, want %s", c.name, got.Type, c.want.Type)
		}
	}
}

func TestInsertExtraKeepsStartOrder(t *testing.T) {
	_, call := extrasTree()
	InsertExtra(call, Leaf("comment", true, 7, 8))
	InsertExtra(call, Leaf("tail", true, 12, 13))
	if got, want := types(call.Children), []string{"(", "name", "comment", "args", ")", "tail"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("children %v, want %v", got, want)
	}
}

func TestRemoveHiddenDropsMarkedNodesAtEveryDepth(t *testing.T) {
	block, call := extrasTree()
	hidden := map[*Node]bool{call.Children[0]: true, block.Children[1]: true}
	RemoveHidden(block, hidden)
	if got := types(block.Children); !reflect.DeepEqual(got, []string{"call"}) {
		t.Fatalf("block children %v", got)
	}
	if got := types(call.Children); !reflect.DeepEqual(got, []string{"name", "args", ")"}) {
		t.Fatalf("call children %v", got)
	}
}
