package render

import (
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// Emits foot comment below the current entry, at the parent indent.
func (e *emitter) emitFootComment(mv *ast.MappingValueNode, indent int) {
	if mv.FootComment == nil {
		return
	}
	for _, c := range mv.FootComment.Comments {
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
