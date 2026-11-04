package branch

import (
	"fmt"
	"strings"
)

// ArmText is the finding text of an arm under branch granularity.
func ArmText(a Arm) string {
	switch a.Label {
	case "true", "false", "error":
		return a.Label + " arm never taken"
	case "body":
		return "body never entered"
	case "exit":
		return "exit never taken"
	case "handler":
		return "handler never entered"
	case "default":
		return "default never matched"
	case "no match":
		return "case always matched"
	}
	if strings.HasPrefix(a.Label, "case ") {
		return a.Label + " never matched"
	}
	if strings.HasPrefix(a.Label, "operand ") {
		if a.Text != "" {
			return fmt.Sprintf("%s (%s) never evaluated", a.Label, a.Text)
		}
		return a.Label + " never evaluated"
	}
	return a.Label + " arm never taken"
}

func blockText(start, end int) string {
	if start == end {
		return fmt.Sprintf("block %d never executed", start)
	}
	return fmt.Sprintf("block %d-%d never executed", start, end)
}

func regionText(l, c, el, ec int) string {
	return fmt.Sprintf("region %d:%d-%d:%d never executed", l, c, el, ec)
}

func countText(missed, total int) string {
	if total == 1 {
		return "1 of 1 branch missed"
	}
	return fmt.Sprintf("%d of %d branches missed", missed, total)
}

func formText(missed, total int) string {
	return fmt.Sprintf("%d of %d forms not covered", missed, total)
}
