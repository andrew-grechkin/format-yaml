package passes

import (
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/token"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"
)

// Forces every null-valued node to render as the literal `null`, replacing empty scalars and `~`. Rewrites the token's
// type, value, and raw lexeme so goccy's serialiser can't fall back to re-emitting the original representation.
func normalizeNulls(root ast.Node) {
	astutil.Walk(root, func(n ast.Node) bool {
		nn, ok := n.(*ast.NullNode)
		if !ok {
			return true
		}
		nn.Token.Type = token.NullType
		nn.Token.Value = "null"
		nn.Token.Origin = "null"
		return true
	})
}
