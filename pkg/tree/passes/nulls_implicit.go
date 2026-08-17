package passes

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// materializeImplicitNulls turns every implicit NullNode (Token nil, source shape `key:` with no value on
// the same line) into an explicit `null` scalar. Gated separately from normalizeNulls because it's a
// semantic upgrade - "no value" becomes "explicit null" - only applied at standard+.
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
			return
		}
		trail := trailingWhitespace(e.Colon.Origin)
		e.Colon.Origin = e.Colon.Origin[:len(e.Colon.Origin)-len(trail)] + " "
		null.Token = &token.Token{Value: "null", Origin: "null" + trail}
		// A flow mapping may have hung an inline comment on the entry (there was no value token to hang it
		// on); move it onto the freshly-materialized null so passes that look at the value's inline comment
		// find it there.
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
