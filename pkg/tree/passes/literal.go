package passes

import (
	"strings"

	"github.com/andrew-grechkin/format-yaml/internal/token"
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// blockScalarizeMultiline rewrites every string leaf whose value contains a newline as a literal `|`
// block scalar, picking the smallest lossless chomp:
//
//	|-   value has no trailing newline
//	|    value has exactly one
//	|+   value has two or more
//
// Runs after normalizeIndent so the body indent it bakes in matches the parent's final column. Values that
// are already block scalars are left alone.
func blockScalarizeMultiline(f *tree.File) {
	forEachDocBody(f, func(n tree.Node) {
		walkBlockValues(n, 0, literalizeEntry, literalizeItem)
	})
}

// literalizeEntry rewrites a mapping entry's multiline string value as a block scalar, one indent step past
// the entry's depth. Returns true when it fired.
func literalizeEntry(e *tree.MappingEntry, depth int) bool {
	s, ok := e.Value.(*tree.StringNode)
	if !ok || !isMultilineString(s) {
		return false
	}
	e.Value = literalBlock(s, depth+indentStep)
	clearTrailer(e.Trailer)
	return true
}

// clearTrailer drops a flow-comma trailer once the value becomes a block ending in its own newline; keeping
// the comma's `\n` would insert a blank line.
func clearTrailer(t *token.Token) {
	if t != nil {
		t.Origin = ""
	}
}

// literalizeItem is the sequence-item counterpart of literalizeEntry.
func literalizeItem(item *tree.SequenceItem, depth int) bool {
	s, ok := item.Value.(*tree.StringNode)
	if !ok || !isMultilineString(s) {
		return false
	}
	item.Value = literalBlock(s, depth+indentStep)
	clearTrailer(item.Trailer)
	return true
}

// isMultilineString reports whether s is a non-block string whose value spans lines. Block scalars already
// carry a Header and are excluded.
func isMultilineString(s *tree.StringNode) bool {
	if s.Header != nil || s.Token == nil {
		return false
	}
	switch s.Style {
	case tree.StringPlain, tree.StringSingleQuoted, tree.StringDoubleQuoted:
		return strings.Contains(s.Token.Value, "\n")
	}
	return false
}

// literalBlock builds the `|` replacement for s with body lines indented by indent spaces.
func literalBlock(s *tree.StringNode, indent int) *tree.StringNode {
	return buildBlockString(s, "|", indent, nil)
}

// foldedBlock builds the `>` replacement for s wrapping lines at the caller-computed width.
func foldedBlock(s *tree.StringNode, lines []string, indent int) *tree.StringNode {
	return buildBlockString(s, ">", indent, lines)
}

// buildBlockString is the shared constructor for literal (`|`) and folded (`>`) blocks. When wrap is nil
// the value's own lines are used (literal); otherwise wrap is used verbatim (folded). The chomp is chosen
// from the trailing-newline count for literals and defaults to strip (`>-`) for folds, matching the fold
// pass's decision that folded content carries no trailing newline.
func buildBlockString(s *tree.StringNode, family string, indent int, wrap []string) *tree.StringNode {
	value := ""
	if s.Token != nil {
		value = s.Token.Value
	}
	trailing := trailingNewlines(value)
	indicator := chompIndicator(family, trailing)

	var body strings.Builder
	if wrap != nil {
		for _, ln := range wrap {
			writeBlockLine(&body, ln, indent)
		}
	} else {
		for _, ln := range strings.Split(strings.TrimRight(value, "\n"), "\n") {
			writeBlockLine(&body, ln, indent)
		}
		for i := 1; i < trailing; i++ {
			body.WriteByte('\n')
		}
	}

	header := &token.Token{Value: indicator, Origin: " " + indicator}
	if s.InlineComment != nil {
		// The inline comment moves from the scalar onto the block header; canonicalize the gap to two spaces
		// and let its own trailing newline separate header from body.
		s.InlineComment.Lines[0].Origin = "  " + strings.TrimLeft(s.InlineComment.Lines[0].Origin, " \t")
	} else {
		header.Origin += "\n"
	}

	out := &tree.StringNode{
		ScalarBase: tree.ScalarBase{Token: &token.Token{Value: value, Origin: body.String()}},
		Style:      tree.StringLiteral,
		Header:     header,
	}
	out.PrecedingComment = s.PrecedingComment
	out.InlineComment = s.InlineComment
	if family == ">" {
		out.Style = tree.StringFolded
	}
	return out
}

// writeBlockLine writes one block-scalar body line: empty lines are emitted bare (a block parser strips
// trailing whitespace, so indenting a blank line would only add invisible dirt).
func writeBlockLine(sb *strings.Builder, line string, indent int) {
	if line != "" {
		sb.WriteString(strings.Repeat(" ", indent))
		sb.WriteString(line)
	}
	sb.WriteByte('\n')
}

// chompIndicator maps a trailing-newline count to the smallest lossless chomp for the given block family.
func chompIndicator(family string, trailing int) string {
	switch {
	case trailing == 0:
		return family + "-"
	case trailing == 1:
		return family
	default:
		return family + "+"
	}
}

// entryKeyOrigin returns the Origin of the token that carries a mapping entry's leading indent: the
// explicit `?` marker when present, otherwise the key token.
func entryKeyOrigin(e *tree.MappingEntry) string {
	if e.ExplicitKeyMarker != nil {
		return e.ExplicitKeyMarker.Origin
	}
	if tok := tree.TokenOf(e.Key); tok != nil {
		return tok.Origin
	}
	return ""
}

// markerOrigin returns a sequence item's `-` marker Origin.
func markerOrigin(item *tree.SequenceItem) string {
	if item.Marker != nil {
		return item.Marker.Origin
	}
	return ""
}
