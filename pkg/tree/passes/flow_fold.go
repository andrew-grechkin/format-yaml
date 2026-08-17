package passes

import (
	"strings"

	"github.com/andrew-grechkin/format-yaml/internal/token"
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// unflowLongFlows converts a flow-style collection under a mapping key to block layout when its rendered
// `key: [...]` line exceeds the line width, or when any item carries a surviving explicit quote (quotes
// amid plain items read better one per line). Mirrors flowShortBlocks so the two directions agree.
func unflowLongFlows(f *tree.File) {
	tree.Walk(f, func(n tree.Node) {
		e, ok := n.(*tree.MappingEntry)
		if !ok {
			return
		}
		switch v := e.Value.(type) {
		case *tree.MappingNode:
			if v.Style == tree.StyleFlow && v.PrecedingComment == nil && v.InlineComment == nil &&
				(entryRecordOverflows(e, v) || flowHasQuotedItem(v)) {
				flowMappingToBlock(v)
				valueOnNewLine(e)
			}
		case *tree.SequenceNode:
			if v.Style == tree.StyleFlow && v.PrecedingComment == nil && v.InlineComment == nil &&
				(entryRecordOverflows(e, v) || flowHasQuotedItem(v)) {
				flowSequenceToBlock(v)
				valueOnNewLine(e)
			}
		}
	})
}

// flowShortBlocks is the mirror of unflowLongFlows: a block sequence under a mapping key whose items are
// all plain-safe scalars and whose folded form fits the line width is collapsed to inline flow
// (`key: [a, b]`).
func flowShortBlocks(f *tree.File) {
	tree.Walk(f, func(n tree.Node) {
		e, ok := n.(*tree.MappingEntry)
		if !ok {
			return
		}
		s, ok := e.Value.(*tree.SequenceNode)
		if !ok || s.Style != tree.StyleBlock || !allFlowSafeItems(s) {
			return
		}
		// A comment on the sequence itself (e.g. `key: # c` above a block sequence) has no correct place in
		// inline flow, so leave the block form alone.
		if s.PrecedingComment != nil || s.InlineComment != nil {
			return
		}
		if !foldedSequenceFits(e, s) {
			return
		}
		sequenceToInline(e, s)
	})
}

// entryRecordOverflows reports whether `key: <value>` exceeds the line width at the key's column.
func entryRecordOverflows(e *tree.MappingEntry, value tree.Node) bool {
	keyOrigin := entryKeyOrigin(e)
	return leadingSpaces(keyOrigin)+DisplayWidth(scalarLexeme(keyOrigin))+2+DisplayWidth(value.ToString()) > LineWidth
}

// flowHasQuotedItem reports whether a flow collection still holds an explicitly-quoted string after
// unquoting has had its chance. A surviving quote means the value can't render plain, which is the cue to
// lay it out as block.
func flowHasQuotedItem(n tree.Node) bool {
	found := false
	tree.Walk(n, func(x tree.Node) {
		if s, ok := x.(*tree.StringNode); ok && (s.Style == tree.StringSingleQuoted || s.Style == tree.StringDoubleQuoted) {
			found = true
		}
	})
	return found
}

// allFlowSafeItems reports whether every item of a block sequence can render bare in flow: a plain scalar
// with no flow indicators and no comments, or a number/bool/null.
func allFlowSafeItems(s *tree.SequenceNode) bool {
	for _, c := range s.Children {
		item, ok := c.(*tree.SequenceItem)
		if !ok || item.PrecedingComment != nil || item.InlineComment != nil {
			return false
		}
		if !flowSafeScalar(item.Value) {
			return false
		}
	}
	return true
}

func flowSafeScalar(n tree.Node) bool {
	// Any scalar carrying a comment can't be folded: the comment would render mid-flow (and for `#` would
	// comment out the rest of the line). This covers Int/Float/Bool/Null as well as String - the old check
	// only looked at StringNode.
	if c, ok := n.(tree.Commented); ok && (c.Preceding() != nil || c.Inline() != nil) {
		return false
	}
	switch v := n.(type) {
	case *tree.StringNode:
		return v.Header == nil && v.Style == tree.StringPlain && v.Token != nil &&
			!strings.ContainsAny(v.Token.Value, "[]{},")
	case *tree.IntNode, *tree.FloatNode, *tree.BoolNode, *tree.NullNode:
		return true
	}
	return false
}

// foldedSequenceFits reports whether `key: [item, item, ...]` fits the line width.
func foldedSequenceFits(e *tree.MappingEntry, s *tree.SequenceNode) bool {
	keyOrigin := entryKeyOrigin(e)
	w := leadingSpaces(keyOrigin) + DisplayWidth(scalarLexeme(keyOrigin)) + len(": ") + len("[]")
	for i, c := range s.Children {
		if i > 0 {
			w += len(", ")
		}
		if item, ok := c.(*tree.SequenceItem); ok && item.Value != nil {
			w += DisplayWidth(strings.TrimSpace(item.Value.ToString()))
		}
	}
	return w <= LineWidth
}

// sequenceToInline rewrites a block sequence under a mapping key into inline flow form: drop the `-`
// markers, join items with `, `, wrap in brackets, and put a single space after the key's colon. Item value
// Origins lose their block leading/trailing whitespace (the `- ` marker and line terminator the block form
// carried) so concatenation yields `[a, b, c]`.
func sequenceToInline(e *tree.MappingEntry, s *tree.SequenceNode) {
	s.Style = tree.StyleFlow
	s.Open = &token.Token{Value: "[", Origin: "["}
	// The close bracket owns the line terminator the block form's last item carried; without it the next
	// entry would concatenate onto this line.
	s.Close = &token.Token{Value: "]", Origin: "]\n"}
	if e.Colon != nil {
		e.Colon.Origin = ": "
	}
	for i, c := range s.Children {
		item, ok := c.(*tree.SequenceItem)
		if !ok {
			continue
		}
		item.Marker = nil
		if tok := tree.TokenOf(item.Value); tok != nil {
			tok.Origin = strings.Trim(tok.Origin, " \t\r\n")
		}
		switch {
		case i < len(s.Children)-1:
			if item.Trailer == nil {
				item.Trailer = &token.Token{}
			}
			item.Trailer.Origin = ", "
		case item.Trailer != nil:
			item.Trailer.Origin = ""
		}
	}
}

// valueOnNewLine rewrites an entry's colon so its value starts on the next line (`key:\n<block>`), used when
// a flow value is expanded to block layout.
func valueOnNewLine(e *tree.MappingEntry) {
	if e.Colon != nil {
		e.Colon.Origin = ":\n"
	}
}
