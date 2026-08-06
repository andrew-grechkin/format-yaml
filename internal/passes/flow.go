package passes

import (
	"github.com/goccy/go-yaml/ast"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"
	"github.com/andrew-grechkin/update-yaml/pkg/style"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// Converts flow-style sequences and mappings under mapping keys to block style. Fires when either:
//
//   - the rendered line's display width exceeds the configured line width (the collection has grown past the visual
//   guideline), OR
//   - any scalar item requires quotes that cannot be dropped (e.g. '42' - quoting is forced because unquoted it
//   resolves to int). Quotes amid plain items are noisy in flow form; block form gives each quoted item its own line
//   and reads cleanly.
//
// This is the narrow exception to unflowTopLevel's "leave nested inline collections alone" rule. Mirrored by
// flowShortBlocks: the fold direction refuses collections whose items would need quotes, so the two rules stay
// consistent across a round-trip.
//
// Line width is measured in display columns via DisplayWidth. Runs before sortMappingKeys so the sort operates on the
// final structure.
func unflowLongFlows(root ast.Node) {
	astutil.Walk(root, func(n ast.Node) bool {
		mv, ok := n.(*ast.MappingValueNode)
		if !ok {
			return true
		}
		flow := flowCollection(mv.Value)
		if flow == nil {
			return true
		}
		if !overflowsLineWidth(mv) && !flowHasQuotedItem(flow) {
			return true
		}
		setFlowStyle(flow, false)
		return true
	})
}

// Returns the underlying flow-style SequenceNode or MappingNode inside n (peeling any anchor wrapper), or nil when n is
// not a flow collection.
func flowCollection(n ast.Node) ast.Node {
	switch v := astutil.UnwrapAnchor(n).(type) {
	case *ast.SequenceNode:
		if v.IsFlowStyle {
			return v
		}
	case *ast.MappingNode:
		if v.IsFlowStyle {
			return v
		}
	}
	return nil
}

func setFlowStyle(n ast.Node, flow bool) {
	switch v := n.(type) {
	case *ast.SequenceNode:
		v.SetIsFlowStyle(flow)
	case *ast.MappingNode:
		v.SetIsFlowStyle(flow)
	}
}

// Reports whether mv's rendered `key: value` line crosses the configured line width in display columns.
func overflowsLineWidth(mv *ast.MappingValueNode) bool {
	col := mv.Key.GetToken().Position.Column
	keyW := style.DisplayWidth(mv.Key.String())
	valW := style.DisplayWidth(mv.Value.String())
	return (col-1)+keyW+len(": ")+valW > config.LineWidth
}

// Reports whether n (a flow collection) contains any scalar that carries an explicit quote token. UnquoteSafeStrings
// has already dropped removable quotes, so a surviving quote means the value cannot be represented plain (would resolve
// to bool/int/etc).
func flowHasQuotedItem(n ast.Node) bool {
	found := false
	astutil.Walk(n, func(x ast.Node) bool {
		if found {
			return false
		}
		if s, ok := x.(*ast.StringNode); ok && astutil.IsExplicitQuote(s.Token.Type) {
			found = true
			return false
		}
		return true
	})
	return found
}

// Converts block-style sequences and mappings under mapping keys to flow style. The mirror of unflowLongFlows: fold
// only when the round-trip check would leave the collection folded, i.e.
//
//   - every scalar item is representable as a plain scalar (no explicit quotes surviving UnquoteSafeStrings, no
//   anchors/ tags/comments, no multiline block scalars), AND
//   - the rendered folded line's display width fits within the configured line width.
//
// Runs after unflowLongFlows so a collection just unfolded by the quote-required rule isn't immediately re-folded.
func flowShortBlocks(root ast.Node) {
	astutil.Walk(root, func(n ast.Node) bool {
		mv, ok := n.(*ast.MappingValueNode)
		if !ok {
			return true
		}
		block := blockSequence(mv.Value)
		if block == nil {
			return true
		}
		if !allItemsPlainSafe(block) {
			return true
		}
		if !foldedLineFits(mv, block) {
			return true
		}
		block.SetIsFlowStyle(true)
		// goccy's MappingValueNode renderer puts the value on a new line whenever Value.IndentLevel > Key.IndentLevel,
		// even for flow-style values. A block sequence parsed under a mapping key naturally sits at level+1; align it
		// with the key so the "flow inline" branch is taken and we emit `key: [a, b]` rather than `key:\n  [a, b]`.
		block.Start.Position.IndentLevel = mv.Key.GetToken().Position.IndentLevel
		return true
	})
}

// Returns the underlying block-style SequenceNode inside n, or nil if n isn't one. Mappings could also fold but current
// fixtures only demand sequence folding; adding mapping support is a direct copy when needed.
func blockSequence(n ast.Node) *ast.SequenceNode {
	if v, ok := astutil.UnwrapAnchor(n).(*ast.SequenceNode); ok && !v.IsFlowStyle {
		return v
	}
	return nil
}

// Reports whether every element of seq is a scalar that would render as a bare plain word in flow context. Any
// surviving explicit quote (from UnquoteSafeStrings), anchor, tag, comment, or nested collection disqualifies the whole
// sequence - fold is all or nothing.
func allItemsPlainSafe(seq *ast.SequenceNode) bool {
	for _, item := range seq.Values {
		if !isPlainSafeItem(item) {
			return false
		}
	}
	return true
}

func isPlainSafeItem(n ast.Node) bool {
	if _, ok := n.(*ast.AnchorNode); ok {
		return false
	}
	if _, ok := n.(*ast.TagNode); ok {
		return false
	}
	if n.GetComment() != nil {
		return false
	}
	switch v := n.(type) {
	case *ast.StringNode:
		return !astutil.IsExplicitQuote(v.Token.Type)
	case *ast.IntegerNode, *ast.FloatNode, *ast.BoolNode, *ast.NullNode, *ast.InfinityNode, *ast.NanNode:
		return true
	}
	return false
}

// Computes the width of the mv rendered as `<key>: [item, item, ...]` (in display columns) and reports whether it stays
// within LineWidth at the current key column.
func foldedLineFits(mv *ast.MappingValueNode, seq *ast.SequenceNode) bool {
	col := mv.Key.GetToken().Position.Column
	keyW := style.DisplayWidth(mv.Key.String())
	itemsW := 2 // "[" + "]"
	for i, item := range seq.Values {
		if i > 0 {
			itemsW += len(", ")
		}
		itemsW += style.DisplayWidth(item.String())
	}
	return (col-1)+keyW+len(": ")+itemsW <= config.LineWidth
}

// Converts a top-level flow-style mapping or sequence to block style so a JSON-like source `{"a": "A"}` renders as `a:
// A`. Nested flow constructs (e.g. `key: [1, 2, 3]`) are intentionally left alone - inline collections are usually a
// deliberate stylistic choice. unflowLongFlows at full+ is the narrow exception for lines that exceed the length
// threshold.
func unflowTopLevel(root ast.Node) {
	switch v := astutil.UnwrapAnchor(root).(type) {
	case *ast.MappingNode:
		if v.IsFlowStyle {
			v.SetIsFlowStyle(false)
		}
	case *ast.SequenceNode:
		if v.IsFlowStyle {
			v.SetIsFlowStyle(false)
		}
	}
}
