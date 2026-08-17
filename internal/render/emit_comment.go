package render

import (
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// Emits the doc-level trailing comment group lifted out of the body-typed slot into Doc.FootComment. Uniform across
// body types: mapping bodies (last entry's FootComment), sequence bodies (SequenceNode.FootComment), and any future
// slot goccy might grow all land here. Uses the same mode-tiered leading-blank policy as interior foot comments.
func (e *emitter) emitDocLevelFootComment(cg *ast.CommentGroupNode) {
	e.emitCommentGroup(cg, 0)
}

// Emits foot comment below the current entry, at the parent indent. Blank line above the first comment line follows
// the same mode-tiered policy as inter-entry blanks: minimal preserves the source count verbatim, standard collapses
// any source blank to exactly one, full/pedantic drop it (the comment sits adjacent to the value).
func (e *emitter) emitFootComment(mv *ast.MappingValueNode, indent int) {
	e.emitCommentGroup(mv.FootComment, indent)
}

// Shared emission for a CommentGroupNode taking a doc-level or entry-level slot. Applies the mode-tiered leading-
// blank rule (minimal preserves source, standard collapses to at most 1, full/pedantic drop it) then writes each
// comment line at the given indent. Nil or empty groups are a no-op so callers don't need their own guard.
func (e *emitter) emitCommentGroup(cg *ast.CommentGroupNode, indent int) {
	if cg == nil || len(cg.Comments) == 0 {
		return
	}
	if first := cg.Comments[0]; first.Token != nil && first.Token.Position != nil {
		desired := 0
		n := e.sourceBlanksBefore(first.Token.Position.Line)
		switch {
		case e.mode < config.ModeStandard:
			desired = n
		case e.mode < config.ModeFull && n > 0:
			desired = 1
		}
		e.ensureTrailingNLs(1 + desired)
	}
	for _, c := range cg.Comments {
		e.writeIndent(indent)
		e.writeByte('#')
		e.writeString(strings.TrimRight(c.Token.Value, " \t"))
		e.writeByte('\n')
	}
}

// Writes the inline comment attached to n (via n.GetComment()) with the mode-appropriate gap, if one exists. Standard+
// uses a fixed 2 spaces; minimal preserves the source's observed gap for this key. Trailing whitespace is stripped from
// the comment text - source-preserved trailing spaces on a comment line would re-parse as part of the comment content
// and grow on every round-trip.
func (e *emitter) emitInlineComment(n ast.Node, key string) {
	c := n.GetComment()
	if c == nil {
		return
	}
	e.writeString(strings.Repeat(" ", e.inlineCommentGap(key)))
	e.writeString(strings.TrimRight(c.String(), " \t"))
}

// Same as emitInlineComment but for a block-scalar header line (`key: |+  # comment`).
func (e *emitter) emitInlineCommentOnHeader(n ast.Node, key string) {
	e.emitInlineComment(n, key)
}

// Spaces to write between a value's last char and its inline `#`. Standard+ uses a fixed 2. Minimal consults
// sourceInline for the specific key, falling back to 1 when the key wasn't observed with an inline comment in the
// source (matches goccy's default).
func (e *emitter) inlineCommentGap(key string) int {
	if e.mode >= config.ModeStandard {
		return 2
	}
	if n, ok := e.sourceInline[key]; ok && n > 0 {
		return n
	}
	return 1
}
