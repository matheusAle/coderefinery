package main

import (
	"reflect"
	"testing"
)

func TestMutateArgsReachOnlyMutateCasesAndLeaveTheMatrixRowAlone(t *testing.T) {
	row := Case{Tool: "mutate", Fixture: "go", Variant: "no-coverage", Args: []string{"--no-coverage"}}
	argv, err := expandTemplate("bin/coderefinery {tool} {args}", row.withMutateArgs([]string{"--min-timeout", "300"}), "/r")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"bin/coderefinery", "mutate", "--no-coverage", "--min-timeout", "300"}; !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv %q, want %q", argv, want)
	}
	if !reflect.DeepEqual(row.Args, []string{"--no-coverage"}) {
		t.Fatalf("the matrix row was modified: %q", row.Args)
	}
	crap := Case{Tool: "crap", Fixture: "go", Variant: "default"}
	if got := crap.withMutateArgs([]string{"--min-timeout", "300"}); len(got.Args) != 0 {
		t.Fatalf("crap got mutate args: %q", got.Args)
	}
	if got := row.withMutateArgs(nil); !reflect.DeepEqual(got.Args, row.Args) {
		t.Fatalf("no extra args: %q", got.Args)
	}
}
