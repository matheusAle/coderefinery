package crap

import "github.com/matheusAle/ub-mode/ubgate/internal/syntax"

// Complexity is upstream treesitter.complexity: 1 plus each decision node
// in node's subtree, preorder, dropping skipped nodes with their subtrees
// (the root goes through skip too). Iterative, so 1200-deep trees are fine.
func Complexity(node *syntax.Node, isDecision, skip func(*syntax.Node) bool) int {
	score := 1
	syntax.Walk(node, func(n *syntax.Node) bool {
		if skip(n) {
			return false
		}
		if isDecision(n) {
			score++
		}
		return true
	})
	return score
}

// BinaryLogic is upstream treesitter.binary_logic: a binary_expression with
// a direct && or || child token.
func BinaryLogic(n *syntax.Node) bool {
	return n.Type == "binary_expression" && n.HasChildOfType("&&", "||")
}
