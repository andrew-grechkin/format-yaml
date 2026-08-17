package passes

import (
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// normalizeNulls rewrites every EXPLICIT NullNode's spelling to the canonical lowercase `null`. Explicit
// means Token is non-nil - the source had `~`, `Null`, `NULL`, or already `null`. Implicit nulls (Token
// nil, source shape `key:` with no value) are LEFT ALONE by this pass; materializing them into `null` is
// a distinct semantic change handled by materializeImplicitNulls at a higher mode.
func normalizeNulls(f *tree.File) {
	tree.Walk(f, func(n tree.Node) {
		null, ok := n.(*tree.NullNode)
		if !ok || null.Token == nil {
			return
		}
		null.Token.Origin = replaceScalarLexeme(null.Token.Origin, "null")
		null.Token.Value = "null"
		null.Style = tree.NullLower
	})
}

func isWhitespace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
