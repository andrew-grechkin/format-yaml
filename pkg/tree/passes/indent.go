package passes

import (
	"strings"

	"github.com/andrew-grechkin/format-yaml/internal/token"
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// indentStep is the canonical number of spaces added per nesting level. Two is what format-yaml has always
// produced; making it a constant here keeps every callsite in this pass agreeing without threading a config
// value through the tree walk. If users ever need 4-space output we lift this to a config knob.
const indentStep = 2

// Canonicalizes indent throughout the tree. Depth-tracked structural walk - never reads source Position info
// (stale after any reordering pass, and irrelevant for emit). Each block-container child owns its own leading
// indent (on the child's first token's Origin), so this pass rewrites those leadings to the canonical
// `depth` spaces. Under the tokenizer's "no leading newlines" contract, a block child's Origin starts with
// N leading SPACES (indent) followed by its content; this pass forces N = depth.
//
// Flow-style containers are skipped. Block scalars are left alone at this level; their body indent is a
// separate concern (a future reindentBlockScalars pass will handle it). Same-line scalar values (an entry's
// value on the same line as its key) are not "block children" and don't get touched. Idempotent.
func normalizeIndent(f *tree.File) {
	for _, c := range f.Children {
		indentNode(c, 0)
	}
}

// indentNode canonicalizes indent within n. depth is the number of leading spaces every direct block child of
// n should sit at (0 for top-level containers, indentStep for a container nested one entry deep, etc.). n
// itself doesn't get indented here - the caller decided that when it recursed with the current depth.
func indentNode(n tree.Node, depth int) {
	switch v := n.(type) {
	case *tree.Doc:
		for _, c := range v.Children {
			indentNode(c, depth)
		}
	case *tree.MappingNode:
		if v.Style != tree.StyleBlock {
			return
		}
		for _, c := range v.Children {
			setChildIndent(c, depth)
			if e, ok := c.(*tree.MappingEntry); ok {
				indentNode(e.Value, depth+indentStep)
			}
		}
	case *tree.SequenceNode:
		if v.Style != tree.StyleBlock {
			return
		}
		for _, c := range v.Children {
			setChildIndent(c, depth)
			if item, ok := c.(*tree.SequenceItem); ok {
				indentNode(item.Value, depth+indentStep)
			}
		}
	case *tree.AnchorNode:
		indentNode(v.Value, depth)
	case *tree.TagNode:
		indentNode(v.Value, depth)
	}
}

// setChildIndent rewrites the leading whitespace of the token that starts child c (its Key for a mapping
// entry, its Marker for a sequence item, its first comment line for an EmptyNode island) so it becomes
// exactly `depth` spaces. Nil-safe: unknown/leaf child types are ignored.
func setChildIndent(c tree.Node, depth int) {
	switch v := c.(type) {
	case *tree.MappingEntry:
		if v.ExplicitKeyMarker != nil {
			setLeadingSpaces(v.ExplicitKeyMarker, depth)
			return
		}
		if tok := tree.TokenOf(v.Key); tok != nil {
			setLeadingSpaces(tok, depth)
		}
	case *tree.SequenceItem:
		setLeadingSpaces(v.Marker, depth)
	case *tree.EmptyNode:
		if v.PrecedingComment != nil && len(v.PrecedingComment.Lines) > 0 {
			setLeadingSpaces(v.PrecedingComment.Lines[0], depth)
		}
	}
}

// setLeadingSpaces replaces the leading whitespace of t.Origin with exactly `depth` spaces. Under the
// tokenizer contract Origin can only start with spaces/tabs (never a newline), so this operates only on
// same-line whitespace. Nil-safe.
func setLeadingSpaces(t *token.Token, depth int) {
	if t == nil {
		return
	}
	i := 0
	for i < len(t.Origin) && (t.Origin[i] == ' ' || t.Origin[i] == '\t') {
		i++
	}
	t.Origin = strings.Repeat(" ", depth) + t.Origin[i:]
}
