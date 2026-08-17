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
// Flow-style containers are skipped. Block scalars are left alone at this level; their body indent is baked in
// by literal.go/fold.go, which run after this pass. Same-line scalar values (an entry's value on the same line
// as its key) are not "block children" and don't get touched. Idempotent.
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
				indentItemValue(item, depth)
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
		setEntryKeyIndent(v, depth)
	case *tree.SequenceItem:
		setLeadingSpaces(v.Marker, depth)
	case *tree.EmptyNode:
		if v.PrecedingComment != nil && len(v.PrecedingComment.Lines) > 0 {
			setLeadingSpaces(v.PrecedingComment.Lines[0], depth)
		}
	}
}

// setEntryKeyIndent puts a mapping entry's key at the given indent. An explicit `?` marker owns the leading
// whitespace when present; otherwise the key token does.
func setEntryKeyIndent(e *tree.MappingEntry, depth int) {
	if e.ExplicitKeyMarker != nil {
		setLeadingSpaces(e.ExplicitKeyMarker, depth)
		return
	}
	if tok := tree.TokenOf(e.Key); tok != nil {
		setLeadingSpaces(tok, depth)
	}
}

// indentItemValue indents the value of a block sequence item. The common case is a normal block child one
// level deeper than the marker. The compact forms `- key: value` and `- - item` are the exception: the
// value's first child sits inline right after the `- `, so it keeps a single separating space while the
// container's remaining children align one level in. Treating the inline container as an ordinary child
// is what produced `-    key:` output.
func indentItemValue(item *tree.SequenceItem, depth int) {
	inner := depth + indentStep
	switch v := item.Value.(type) {
	case *tree.MappingNode:
		if v.Style == tree.StyleBlock && startsInline(item) {
			indentNode(v, inner)
			lead := inlineLead(item)
			setFirstChildInline(v.Children, func(c tree.Node) {
				if e, ok := c.(*tree.MappingEntry); ok {
					setEntryKeyIndent(e, lead)
				}
			})
			return
		}
	case *tree.SequenceNode:
		if v.Style == tree.StyleBlock && startsInline(item) {
			indentNode(v, inner)
			lead := inlineLead(item)
			setFirstChildInline(v.Children, func(c tree.Node) {
				if it, ok := c.(*tree.SequenceItem); ok {
					setLeadingSpaces(it.Marker, lead)
				}
			})
			return
		}
	}
	indentNode(item.Value, inner)
}

// inlineLead is the leading space the first child of an inline container needs after the marker: zero when
// the marker already ends in a separator space (`- `), otherwise one to form `- key`.
func inlineLead(item *tree.SequenceItem) int {
	if origin := markerOrigin(item); strings.HasSuffix(origin, " ") || strings.HasSuffix(origin, "\t") {
		return 0
	}
	return 1
}

// startsInline reports whether a sequence item's value begins on the same line as its `-` marker. Under the
// lexer's contract, a value that starts on the next line has already pushed its leading newline onto the
// marker's Origin, so a marker that doesn't end in a newline means the value is inline and separated by a
// single space.
func startsInline(item *tree.SequenceItem) bool {
	return item.Marker != nil && !strings.HasSuffix(item.Marker.Origin, "\n")
}

// setFirstChildInline applies fix to the first block child of children (no-op when empty). Used to keep the
// first child of an inline container on the marker's line after the ordinary depth walk positioned it.
func setFirstChildInline(children []tree.Node, fix func(tree.Node)) {
	if len(children) > 0 {
		fix(children[0])
	}
}

// setLeadingSpaces replaces the leading whitespace of t.Origin with exactly `depth` spaces. Under the
// tokenizer contract Origin can only start with same-line spaces/tabs, but a document's first token can still
// carry leading newlines (there is no preceding token for normalizeLeadingNewlines to hand them to), so we skip
// those too - they are insignificant blank lines above the first node. Nil-safe.
func setLeadingSpaces(t *token.Token, depth int) {
	if t == nil {
		return
	}
	i := 0
	for i < len(t.Origin) && (t.Origin[i] == ' ' || t.Origin[i] == '\t' || t.Origin[i] == '\n' || t.Origin[i] == '\r') {
		i++
	}
	t.Origin = strings.Repeat(" ", depth) + t.Origin[i:]
}
