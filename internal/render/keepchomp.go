package render

import (
	"strings"

	"github.com/goccy/go-yaml/ast"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"
)

// Walks the deepest last-entry chain from n. If the terminal node is a `|+` or `>+` block-scalar LiteralNode it returns
// true - meaning the value's trailing newlines run into the doc boundary and need a `...` marker for disambiguation.
// Any other terminal (plain scalar, quoted scalar, `|`/`|-`/`>`/`>-`, empty mapping, flow collection) returns false:
// either the block scalar's trailing count is unambiguous by chomp rule, or there is no block scalar to disambiguate.
func docTailKeepsChomp(n ast.Node) bool {
	for n != nil {
		switch v := astutil.UnwrapAnchor(n).(type) {
		case *ast.MappingNode:
			if v.IsFlowStyle || len(v.Values) == 0 {
				return false
			}
			n = v.Values[len(v.Values)-1].Value
		case *ast.SequenceNode:
			if v.IsFlowStyle || len(v.Values) == 0 {
				return false
			}
			n = v.Values[len(v.Values)-1]
		case *ast.LiteralNode:
			return strings.HasSuffix(v.Start.Value, "+")
		default:
			return false
		}
	}
	return false
}

// Reports whether the doc's last top-level entry's value subtree contains at least one `|+`/`>+` block scalar. Used by
// the emitter's `...` doc-end decision: a keep-chomp block in the subtree earns a blank line above `...` so the
// doc-tail visual cue survives even when a sibling entry consumed the block's trailing newlines.
func lastEntrySubtreeHasKeepChomp(doc *ast.DocumentNode) bool {
	mn, ok := astutil.UnwrapAnchor(doc.Body).(*ast.MappingNode)
	if !ok || len(mn.Values) == 0 {
		return false
	}
	found := false
	astutil.Walk(mn.Values[len(mn.Values)-1].Value, func(n ast.Node) bool {
		if found {
			return false
		}
		if lit, ok := n.(*ast.LiteralNode); ok && strings.HasSuffix(lit.Start.Value, "+") {
			found = true
			return false
		}
		return true
	})
	return found
}
