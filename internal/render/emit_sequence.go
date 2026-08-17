package render

import (
	"strings"

	"github.com/goccy/go-yaml/ast"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// Emits a block-style sequence's items. Items may be simple scalars (`- value`), block scalars (`- |+\n  body`), nested
// mappings (`- key: value` with subsequent keys aligned under the first), or nested sequences (compact `- - inner`
// form).
func (e *emitter) emitSequence(seq *ast.SequenceNode, indent int) {
	if seq.IsFlowStyle {
		e.writeString(seq.String())
		return
	}
	// Sequence's own head comment: goccy stores a leading comment that sits above the first item on the SequenceNode
	// itself (BaseNode.Comment), not on ValueHeadComments[0]. Emit inline before the first item so an adjacent-in-
	// source layout stays adjacent - and so doc-level heads don't need to be extracted from sequence bodies (unlike
	// mapping bodies where extraction protects against sort dragging the comment along with the first entry).
	if seq.BaseNode != nil && seq.BaseNode.Comment != nil {
		e.emitCommentGroup(seq.BaseNode.Comment, indent)
	}
	for i, item := range seq.Values {
		if i > 0 {
			e.emitInterItemBlank(item)
		}
		if i < len(seq.ValueHeadComments) {
			e.emitSequenceItemHeadComment(seq.ValueHeadComments[i], indent)
		}
		e.writeIndent(indent)
		e.writeString("- ")
		e.emitSequenceItemBody(item, indent+2)
	}
}

// Emits the head-comment block (if any) that sits above sequence item i. Goccy stores these on SequenceNode.
// ValueHeadComments[i]; nothing else does. Shares the CommentGroupNode emission with foot comments so the mode-tiered
// leading-blank rule stays uniform (minimal preserves source count, standard collapses to 1, full/pedantic drop).
func (e *emitter) emitSequenceItemHeadComment(cg *ast.CommentGroupNode, indent int) {
	e.emitCommentGroup(cg, indent)
}

// Emits the spacing before a sequence item after item[0]. Minimal preserves the source's blank-line count verbatim;
// standard preserves any source blank (collapsing to 1); full+ keeps items adjacent. A preceding `|+` block's trailing
// content already provides blanks - ensureTrailingNLs never clips those, so the effective count is
// max(existing_trailing, target).
func (e *emitter) emitInterItemBlank(item ast.Node) {
	desired := 1
	if e.mode < config.ModeFull {
		if t := item.GetToken(); t != nil && t.Position != nil {
			n := e.sourceBlanksBefore(t.Position.Line)
			switch {
			case e.mode < config.ModeStandard:
				desired += n // minimal: verbatim
			case n > 0:
				desired++ // standard: collapse to 1
			}
		}
	}
	e.ensureTrailingNLs(desired)
}

// Emits one sequence item's body, positioned after the caller wrote `- `. itemCol is the column where scalars or the
// first mapping key of a mapping item lands and where subsequent mapping keys of the same item align. Type-dispatch is
// factored out of emitSequence to keep both under the cyclomatic budget.
func (e *emitter) emitSequenceItemBody(item ast.Node, itemCol int) {
	anchorPrefix := ""
	inner := item
	if a, ok := inner.(*ast.AnchorNode); ok {
		anchorPrefix = "&" + a.Name.String() + " "
		inner = a.Value
	}
	switch v := inner.(type) {
	case *ast.LiteralNode:
		e.writeString(anchorPrefix)
		e.emitLiteralHeader(v)
		e.writeByte('\n')
		e.emitLiteralBody(v, itemCol)
	case *ast.MappingNode:
		if v.IsFlowStyle || len(v.Values) == 0 {
			e.writeString(anchorPrefix)
			e.writeString(inner.String())
			e.writeByte('\n')
			return
		}
		e.emitSequenceItemMapping(v, itemCol, anchorPrefix)
	case *ast.SequenceNode:
		if v.IsFlowStyle || len(v.Values) == 0 {
			e.writeString(anchorPrefix)
			e.writeString(inner.String())
			e.writeByte('\n')
			return
		}
		// Nested block-sequence: compact `- - first_item` form.
		if anchorPrefix != "" {
			e.writeString(anchorPrefix)
		}
		e.emitNestedSequenceCompact(v, itemCol)
	default:
		if inlineable(inner) {
			e.writeString(anchorPrefix)
			e.writeString(inner.String())
			e.writeByte('\n')
			return
		}
		if anchorPrefix != "" {
			e.writeString(strings.TrimRight(anchorPrefix, " "))
		}
		e.writeByte('\n')
		e.emitNode(inner, itemCol)
	}
}

// Emits a mapping that's a sequence item: first key inline with the outer `- ` prefix, subsequent keys indented at
// itemCol so they align visually under the first.
func (e *emitter) emitSequenceItemMapping(mn *ast.MappingNode, itemCol int, anchorPrefix string) {
	for j, mv := range mn.Values {
		if j > 0 {
			// Inside a sequence item's mapping we are by definition NOT at top level, so the wants rule never fires
			// here.
			e.emitInterEntryBlank(mn.Values[j-1], mv, false, j-1)
			e.emitMappingValueAt(mv, itemCol, true)
			continue
		}
		if anchorPrefix != "" {
			e.writeString(anchorPrefix)
		}
		e.emitMappingValueAt(mv, itemCol, false)
	}
}

// Emits an inner sequence in the compact `- - item` form. The caller has already written the outer `- ` prefix; this
// function writes the first inner item on the same line and any subsequent items on their own lines at itemCol (the
// outer sequence's item-body column).
func (e *emitter) emitNestedSequenceCompact(seq *ast.SequenceNode, itemCol int) {
	for i, item := range seq.Values {
		if i > 0 {
			e.ensureTrailingNLs(1)
			e.writeIndent(itemCol)
		}
		e.writeString("- ")
		inner := astutil.UnwrapAnchor(item)
		if inlineable(inner) {
			e.writeString(stringWithoutComment(inner))
			e.writeByte('\n')
			continue
		}
		// Fall back to full emit for complex nested items (mapping under nested seq, block scalar, etc.); the resulting
		// output isn't compact but stays correct.
		e.writeByte('\n')
		e.emitNode(inner, itemCol+2)
	}
}
