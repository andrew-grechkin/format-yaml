package passes

import (
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// ensureBlankLines inserts one blank line between certain adjacent siblings so multi-line "big" content
// stands apart visually. Rules (matching goccy's output):
//
//   - Between two adjacent Docs at the File level.
//   - Between two adjacent block-mapping entries in a Doc, when either entry has a multi-line value (a
//     nested block container or a block scalar).
//
// The blank comes from making the tail of the CURRENT node end with `\n\n` (two newlines: one terminates
// the current entry's own line, the second is the blank). Nodes know their own tail via AppendNewline;
// TrailingNewlines lets the loop stay idempotent (won't add a third or fourth).
func ensureBlankLines(f *tree.File) {
	processSiblings(f.Children)
	for _, c := range f.Children {
		if doc, ok := c.(*tree.Doc); ok {
			processSiblings(doc.Children)
		}
	}
}

// processSiblings walks a children slice with the (cur, next) two-pointer pattern. For each pair, decides
// whether a blank separator is warranted; if yes, grows cur's trailing to >=2 newlines by asking cur to
// AppendNewline. Idempotent per invocation - a second run does nothing once cur.TrailingNewlines() >= 2.
func processSiblings(children []tree.Node) {
	for i := 0; i < len(children)-1; i++ {
		cur := children[i]
		next := children[i+1]
		if !shouldBlankBetween(cur, next) {
			continue
		}
		// Bounded: at most 2 iterations needed to reach TrailingNewlines >= 2, and if AppendNewline is
		// a no-op for this node type (empty container, etc.) the bound prevents an infinite loop.
		for k := 0; k < 2 && tree.TrailingNewlines(cur) < 2; k++ {
			cur.AppendNewline()
		}
	}
}

// shouldBlankBetween returns true when a blank line separator belongs between cur and next.
func shouldBlankBetween(cur, next tree.Node) bool {
	return isBig(cur) || isBig(next)
}

// isBig reports whether n renders as multiple visual lines - either because it's a Doc (always spans
// header + body + optional footer), a block container (spans indented children), a block scalar (spans
// the header line and body lines), or a wrapper around any of the above.
func isBig(n tree.Node) bool {
	switch v := n.(type) {
	case *tree.Doc:
		return true
	case *tree.MappingNode:
		return v.Style == tree.StyleBlock && len(v.Children) > 0
	case *tree.SequenceNode:
		return v.Style == tree.StyleBlock && len(v.Children) > 0
	case *tree.StringNode:
		return v.Style == tree.StringLiteral || v.Style == tree.StringFolded
	case *tree.MappingEntry:
		return isBig(v.Value)
	case *tree.SequenceItem:
		return isBig(v.Value)
	case *tree.AnchorNode:
		return isBig(v.Value)
	case *tree.TagNode:
		return isBig(v.Value)
	}
	return false
}
