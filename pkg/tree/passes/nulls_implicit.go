package passes

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// materializeImplicitNulls turns every implicit NullNode (Token nil, source shape `key:` with no value on
// the same line) into an explicit `null` scalar. Gated separately from normalizeNulls because it's a
// semantic upgrade - "no value" becomes "explicit null" - and goccy runs it at standard+ but preserves
// implicit nulls at minimal.
//
// The trailing whitespace that lived on the entry's Colon (typically `\n` or `\n<indent>`) gets moved onto
// the new null Token, and the Colon's trailing gets a single space instead. Result: `key:\n` becomes
// `key: null\n` - the trailing whitespace ends up in its natural post-value position rather than being
// stranded between colon and value.
func materializeImplicitNulls(f *tree.File) {
	tree.Walk(f, func(n tree.Node) {
		e, ok := n.(*tree.MappingEntry)
		if !ok {
			return
		}
		null, ok := e.Value.(*tree.NullNode)
		if !ok || null.Token != nil {
			return
		}
		if e.Colon == nil {
			null.Token = &token.Token{Value: "null", Origin: "null"}
			null.Style = tree.NullLower
			return
		}
		trail := trailingWhitespace(e.Colon.Origin)
		e.Colon.Origin = e.Colon.Origin[:len(e.Colon.Origin)-len(trail)] + " "
		null.Token = &token.Token{Value: "null", Origin: "null" + trail}
		null.Style = tree.NullLower
		// Any inline comment that sat on the ENTRY (because there was no explicit value token to hang
		// it on) now belongs on the freshly-materialized null - it's the value slot's inline comment,
		// and downstream passes that look at "the inline comment of the scalar value" need to find it
		// there.
		if e.InlineComment != nil && null.InlineComment == nil {
			null.InlineComment = e.InlineComment
			e.InlineComment = nil
		}
	})
}

// trailingWhitespace returns the maximal whitespace suffix of s. Empty when s doesn't end in whitespace.
func trailingWhitespace(s string) string {
	i := len(s)
	for i > 0 && isWhitespace(s[i-1]) {
		i--
	}
	return s[i:]
}
