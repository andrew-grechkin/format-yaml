package passes

import (
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// KeepSourceBlanks controls whether an existing author blank line between siblings survives even when the
// shape rule wouldn't insert one. standard preserves source blanks (collapsed to one); full discards them
// and re-derives spacing purely from the current tree shape. Set by the caller before Apply.
var KeepSourceBlanks = true

// ensureBlankLines inserts one blank line between certain adjacent siblings so multi-line "big" content
// stands apart visually. Rules:
//
//   - Between two adjacent Docs at the File level.
//   - Between two adjacent block-mapping entries in a Doc, when either entry has a multi-line value (a
//     nested block container or a block scalar).
//
// The blank comes from making the tail of the CURRENT node end with `\n\n` (two newlines: one terminates
// the current entry's own line, the second is the blank). Nodes know their own tail via AppendNewline;
// TrailingNewlines lets the loop stay idempotent (won't add a third or fourth).
func ensureBlankLines(f *tree.File) {
	processSiblings(f.Children, true)
	for _, c := range f.Children {
		doc, ok := c.(*tree.Doc)
		if !ok {
			continue
		}
		for _, body := range doc.Children {
			walkBlockSiblings(body, true)
		}
	}
	// The file's last node has no following sibling to normalize against; at full a trailing source blank
	// (possibly dragged to the tail by sorting) is clipped. Only the file tail is touched - a container's
	// last child may legitimately be followed by a doc-level comment whose blank must survive.
	if !KeepSourceBlanks && len(f.Children) > 0 {
		last := f.Children[len(f.Children)-1]
		tree.SetTrailingNewlines(last, minTailNewlines(last))
	}
}

// walkBlockSiblings normalizes blank lines between the direct children of a block container, then recurses
// into nested block containers. topLevel is true only for the doc body: the "multi-line neighbour" wants
// rule is a top-level reading aid (matching full's re-derivation), while deeper levels only preserve source
// blanks (at standard) or discard them (at full).
func walkBlockSiblings(n tree.Node, topLevel bool) {
	switch v := n.(type) {
	case *tree.MappingNode:
		if v.Style != tree.StyleBlock {
			return
		}
		processSiblings(v.Children, topLevel)
		for _, c := range v.Children {
			if e, ok := c.(*tree.MappingEntry); ok {
				walkBlockSiblings(e.Value, false)
			}
		}
	case *tree.SequenceNode:
		if v.Style != tree.StyleBlock {
			return
		}
		// The wants rule never applies to sequence items.
		processSiblings(v.Children, false)
		for _, c := range v.Children {
			if item, ok := c.(*tree.SequenceItem); ok {
				walkBlockSiblings(item.Value, false)
			}
		}
	case *tree.AnchorNode:
		walkBlockSiblings(v.Value, topLevel)
	case *tree.TagNode:
		walkBlockSiblings(v.Value, topLevel)
	}
}

// processSiblings walks a children slice with the (cur, next) two-pointer pattern. For each pair, decides
// whether a blank separator is warranted; if yes, grows cur's trailing to >=2 newlines by asking cur to
// AppendNewline. Idempotent per invocation - a second run does nothing once cur.TrailingNewlines() >= 2.
func processSiblings(children []tree.Node, wants bool) {
	for i := 0; i < len(children)-1; i++ {
		cur := children[i]
		want := 1
		if (wants && shouldBlankBetween(cur, children[i+1])) || (KeepSourceBlanks && tree.TrailingNewlines(cur) >= 2) {
			want = 2
		}
		// Never clip below the node's semantic minimum: a `|+`/`>+` block's trailing newlines are content.
		if min := minTailNewlines(cur); want < min {
			want = min
		}
		tree.SetTrailingNewlines(cur, want)
	}
}

// shouldBlankBetween returns true when a blank line separator belongs between cur and next. When cur ends with
// an explicit `...` and next is a doc whose header already carries a preceding comment, that comment sits
// directly under the marker - adding a blank would isolate it.
func shouldBlankBetween(cur, next tree.Node) bool {
	if nextDoc, ok := next.(*tree.Doc); ok && nextDoc.Header != nil && nextDoc.Header.PrecedingComment != nil {
		if curDoc, ok := cur.(*tree.Doc); ok && curDoc.Footer != nil {
			return false
		}
	}
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
