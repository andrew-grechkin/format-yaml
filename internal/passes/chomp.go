package passes

import (
	"strings"

	"github.com/goccy/go-yaml/ast"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"
	"github.com/andrew-grechkin/update-yaml/pkg/patch"
)

// Downgrades source-authored `|+` blocks to `|` when the value's trailing newline count is exactly 1. This is lossless
// (both chomps encode `content\n`) and protects the block from the standard+ multiline-blank rule, which would
// otherwise absorb the boundary blank line into `|+`'s value and silently grow the trailing-newline count on every
// format pass.
//
// The transformation is a pure in-place field edit on the Start token - no rebuild, no re-parse, no column shifts.
// Node.String() re-emits using the mutated Origin.
//
// `|+` with trailing_nl >= 2 is left alone: `|+` is required for values with multiple trailing newlines. `|` and `|-`
// are already minimal.
func canonicalizeBlockScalarChomp(root ast.Node) {
	astutil.Walk(root, func(n ast.Node) bool {
		lit, ok := n.(*ast.LiteralNode)
		if !ok {
			return true
		}
		if lit.Start.Value != "|+" {
			return true
		}
		s, ok := lit.Value.GetValue().(string)
		if !ok {
			return true
		}
		if patch.TrailingNewlines(s) != 1 {
			return true
		}
		lit.Start.Origin = strings.Replace(lit.Start.Origin, "|+", "|", 1)
		lit.Start.Value = "|"
		return true
	})
}
