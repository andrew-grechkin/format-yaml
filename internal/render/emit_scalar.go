package render

import (
	"strings"

	"github.com/goccy/go-yaml/ast"

	"github.com/andrew-grechkin/update-yaml/pkg/patch"
)

// Emits a LiteralNode standalone (used when it's a direct child of something other than a mapping value slot, which is
// rare).
func (e *emitter) emitLiteral(lit *ast.LiteralNode, indent int) {
	e.emitLiteralHeader(lit)
	e.writeByte('\n')
	e.emitLiteralBody(lit, indent)
}

// Writes just the block header indicator (`|`, `|-`, `|+`, `>`, `>-`, `>+`, plus optional indent digit).
func (e *emitter) emitLiteralHeader(lit *ast.LiteralNode) {
	if lit.Start != nil {
		e.writeString(lit.Start.Value)
	}
}

// Writes the block-scalar body indented to targetCol, then exactly `trailing_nl` trailing '\n' after the body content.
// This is the core value-preservation guarantee: whatever trailing_nl the AST value carries, that's what the emitter
// writes.
//
// `|` (literal) blocks preserve newlines byte-for-byte, so the rendered body is Value.Value split by '\n' with each
// line indented. `>` (folded) blocks fold source line breaks into spaces on parse, and the semantic Value.Value is a
// single line (or paragraphs). Their wrap boundaries live in Value.Token.Origin, baked there either by the author or by
// foldLongScalars. For `>` we render Origin directly (with its wrap intact); for `|` we rebuild from Value.Value at the
// target indent.
func (e *emitter) emitLiteralBody(lit *ast.LiteralNode, targetCol int) {
	value := ""
	if lit.Value != nil {
		value = lit.Value.Value
	}
	trailing := patch.TrailingNewlines(value)
	if lit.Start != nil && strings.HasPrefix(lit.Start.Value, ">") && lit.Value != nil && lit.Value.Token != nil {
		e.writeString(strings.TrimRight(lit.Value.Token.Origin, "\n"))
	} else {
		body := strings.TrimRight(value, "\n")
		indent := strings.Repeat(" ", targetCol)
		for i, line := range strings.Split(body, "\n") {
			if i > 0 {
				e.writeByte('\n')
			}
			if line != "" {
				e.writeString(indent)
				e.writeString(line)
			}
		}
	}
	// Write exactly `trailing` newlines after the last content byte. The value semantically ends with `trailing`
	// newlines; anything less rewrites the value.
	for range trailing {
		e.writeByte('\n')
	}
}

// Emits a value that MUST fit inline after a `key: ` prefix. Scalars render as-is (with the node's inline comment
// detached during rendering - the caller emits comments separately so it can control spacing); a block mapping/sequence
// gets its `\n` + nested emission from the caller instead.
func (e *emitter) emitInlineValue(n ast.Node, indent int) {
	switch v := n.(type) {
	case *ast.MappingNode:
		if v.IsFlowStyle {
			e.writeString(v.String())
		}
	case *ast.SequenceNode:
		if v.IsFlowStyle {
			e.writeString(v.String())
		}
	default:
		e.writeString(stringWithoutComment(n))
	}
}

// Renders n via its .String() method with any inline comment temporarily detached, so the caller can emit the comment
// on its own with controlled spacing. Not concurrency-safe; not intended to be (the emitter runs single-threaded per
// file).
func stringWithoutComment(n ast.Node) string {
	c := n.GetComment()
	if c == nil {
		return n.String()
	}
	_ = n.SetComment(nil)
	s := n.String()
	_ = n.SetComment(c)
	return s
}
