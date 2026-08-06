package render

import (
	"strings"

	"github.com/goccy/go-yaml/ast"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// Emits a block-style mapping's entries. Blank-line policy between adjacent entries only fires at TOP LEVEL - the
// "multiline neighbour" wants rule applies exclusively to direct doc.Body children. Nested entries preserve source
// blanks at minimal/ standard and sit adjacent at full+.
func (e *emitter) emitMapping(mn *ast.MappingNode, indent int, topLevel bool) {
	if mn.IsFlowStyle {
		e.writeString(mn.String())
		return
	}
	for i, mv := range mn.Values {
		if i > 0 {
			e.emitInterEntryBlank(mn.Values[i-1], mv, topLevel, i-1)
		}
		e.emitMappingValue(mv, indent)
	}
}

// Emits blank lines between two adjacent mapping entries per the active mode's policy. Whatever trailing newlines the
// previous entry already contributed (a `|+` block's trailing content, for instance) count toward the target -
// ensureTrailingNLs never clips, only pads.
//
// Policy per mode (top-level uses the wants rule; nested only preserves source blanks at minimal/standard):
//   - full/pedantic: top-level 2 iff wantBlankBetween; nested 1
//   - standard:      top-level 2 iff source>=1 or wants; nested 1
//   - minimal:       source's exact count (>=1) at every level
func (e *emitter) emitInterEntryBlank(prev, next *ast.MappingValueNode, topLevel bool, boundaryIdx int) {
	desired := 1
	if topLevel {
		desired = e.desiredTopLevelBlanks(prev, next, boundaryIdx)
	} else if e.mode < config.ModeFull {
		// Nested boundary at minimal/standard: preserve source blanks above the next key. Full+ discards source blanks
		// wholesale (sort has invalidated their positional meaning).
		if line := e.entryStartLine(next); line > 0 {
			n := e.sourceBlanksBefore(line)
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

// Returns the source line above which any inter-entry blank belongs - the head-comment's first line if the entry has
// one, otherwise the key's own line. Used by nested-boundary blank preservation.
func (e *emitter) entryStartLine(mv *ast.MappingValueNode) int {
	if mv.Comment != nil && len(mv.Comment.Comments) > 0 {
		if t := mv.Comment.Comments[0].Token; t != nil && t.Position != nil {
			return t.Position.Line
		}
	}
	if t := mv.Key.GetToken(); t != nil && t.Position != nil {
		return t.Position.Line
	}
	return 0
}

// Returns the target trailing-NL count above the next top-level entry, expressed as "1 + blank_lines_above". A return
// of 1 means entries sit directly adjacent (0 blank lines).
func (e *emitter) desiredTopLevelBlanks(prev, next *ast.MappingValueNode, boundaryIdx int) int {
	sourceBlanks := 0
	if boundaryIdx < len(e.currentDocBoundaries) {
		sourceBlanks = e.currentDocBoundaries[boundaryIdx]
	}
	switch {
	case e.mode >= config.ModeFull:
		if e.wantBlankBetween(prev, next) {
			return 2
		}
		return 1
	case e.mode >= config.ModeStandard:
		// Standard: preserve any source blank (collapsing 2+ to 1) AND additionally insert when the current shape
		// demands.
		if sourceBlanks >= 1 || e.wantBlankBetween(prev, next) {
			return 2
		}
		return 1
	default:
		// Minimal: preserve source's blank count verbatim. Floor at 0 blanks (baseline 1 newline separator).
		if sourceBlanks < 0 {
			sourceBlanks = 0
		}
		return 1 + sourceBlanks
	}
}

// Reports whether the layout rule wants a blank line between two adjacent top-level mapping entries. Fires when either
// side is multiline (nested mapping/sequence, block scalar) or the previous entry carries a foot comment.
func (e *emitter) wantBlankBetween(prev, next *ast.MappingValueNode) bool {
	return valueIsMultiline(prev.Value) || valueIsMultiline(next.Value) ||
		prev.FootComment != nil
}

// Emits one `key: value` entry plus any attached comments.
func (e *emitter) emitMappingValue(mv *ast.MappingValueNode, indent int) {
	e.emitMappingValueAt(mv, indent, true)
}

// emitMappingValueAt is the shared path used by both direct mapping emission and sequence-of-mapping emission. When
// writePrefix is true we write `<indent>` before the key; when false, the caller has already positioned us at the key
// column (e.g., right after `- ` in a sequence item). Head comments only apply when the caller wrote the prefix - a
// mid-sequence-item head comment is a separate concern the current fixtures don't exercise.
func (e *emitter) emitMappingValueAt(mv *ast.MappingValueNode, indent int, writePrefix bool) {
	if writePrefix && mv.Comment != nil {
		for _, c := range mv.Comment.Comments {
			e.writeIndent(indent)
			e.writeByte('#')
			e.writeString(strings.TrimRight(c.Token.Value, " \t"))
			e.writeByte('\n')
		}
	}
	if writePrefix {
		e.writeIndent(indent)
	}
	e.writeString(mv.Key.String())
	e.writeByte(':')

	// Peel any anchor wrapper for type dispatch, but retain the `&name ` prefix so it lands right after the colon-space
	// and before the value proper.
	anchorPrefix := ""
	val := mv.Value
	if a, ok := val.(*ast.AnchorNode); ok {
		anchorPrefix = "&" + a.Name.String() + " "
		val = a.Value
	}
	if val == nil {
		e.writeByte('\n')
		return
	}

	if lit, ok := val.(*ast.LiteralNode); ok {
		// Header on the same line as the key; body indented below.
		e.writeByte(' ')
		e.writeString(anchorPrefix)
		e.emitLiteralHeader(lit)
		e.emitInlineCommentOnHeader(mv.Value, astutil.KeyString(mv.Key))
		e.writeByte('\n')
		e.emitLiteralBody(lit, e.childIndent(val, indent))
		e.emitFootComment(mv, indent)
		return
	}

	if inlineable(val) {
		// Empty-representation values (NullNode with empty origin, empty flow collections) render to "". The `: `
		// separator applies only when there's real content after it; a bare `key:` with a trailing inline comment
		// measures the gap from right after `:`, so an extra pre-value space would throw off the source-preserved
		// alignment by one column.
		rendered := stringWithoutComment(val)
		if rendered != "" || anchorPrefix != "" {
			e.writeByte(' ')
			e.writeString(anchorPrefix)
			e.writeString(rendered)
		}
		e.emitInlineComment(mv.Value, astutil.KeyString(mv.Key))
		e.writeByte('\n')
		e.emitFootComment(mv, indent)
		return
	}

	// Block-style nested collection. Anchor prefix (if any) lands after `key:` on the same line, with the collection
	// body indented below on subsequent lines.
	if anchorPrefix != "" {
		e.writeByte(' ')
		e.writeString(strings.TrimRight(anchorPrefix, " "))
	}
	e.writeByte('\n')
	e.emitNode(val, e.childIndent(val, indent))
	e.emitFootComment(mv, indent)
}
