package passes

import (
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"
	"github.com/andrew-grechkin/update-yaml/pkg/patch"
)

// Rewrites string scalars containing '\n' as block scalars, choosing the smallest lossless chomp indicator:
//   - `|-` when the value has no trailing newline
//   - `|`  when it has exactly one
//   - `|+` when it has two or more
//
// The folded form `>` is never emitted - goccy's round-tripping of `>` is unreliable and folding can silently rewrap
// paragraphs.
//
// The new literal node is built by rendering a top-level block-scalar source, parsing it, then shifting its column so
// the tokens line up with the slot the StringNode is vacating.
func blockScalarizeMultiline(root ast.Node) {
	astutil.ReplaceLeaves(root, func(n ast.Node) ast.Node {
		s, ok := n.(*ast.StringNode)
		if !ok {
			return nil
		}
		if !strings.Contains(s.Value, "\n") {
			return nil
		}
		return newLiteralNode(s)
	})
}

func newLiteralNode(orig *ast.StringNode) ast.Node {
	return buildLiteralNode(orig.Value, orig.GetToken().Position.Column, orig.GetComment())
}

// Constructs a fresh LiteralNode encoding value at the smallest lossless chomp indicator based on the value's trailing
// newline count. Used by both blockScalarizeMultiline (promoting a StringNode with '\n' in its value) and
// canonicalizeBlockScalarChomp (re-encoding an existing LiteralNode with a suboptimal chomp).
//
// The node is built by rendering a synthetic block-scalar source at column 1, re-parsing, then shifting the result's
// column to match col. Any inline comment attached to the original node is carried over so goccy's renderer emits `|- #
// comment` on the header line.
func buildLiteralNode(value string, col int, comment *ast.CommentGroupNode) ast.Node {
	trailing := patch.TrailingNewlines(value)
	var chomp string
	switch trailing {
	case 0:
		chomp = "-"
	case 1:
		chomp = ""
	default:
		chomp = "+"
	}
	body := strings.TrimRight(value, "\n")

	var sb strings.Builder
	sb.WriteByte('|')
	sb.WriteString(chomp)
	sb.WriteByte('\n')
	for ln := range strings.SplitSeq(body, "\n") {
		sb.WriteString("  ")
		sb.WriteString(ln)
		sb.WriteByte('\n')
	}
	// |+ keeps every trailing newline. The last content line already ends with one \n; represent additional trailing
	// newlines by appending empty lines inside the block.
	for i := 1; i < trailing; i++ {
		sb.WriteByte('\n')
	}

	file, err := parser.ParseBytes([]byte(sb.String()), 0)
	if err != nil || len(file.Docs) != 1 {
		return nil
	}
	node := file.Docs[0].Body
	if col > 1 {
		node.AddColumn(col - 1)
	}
	if comment != nil {
		_ = node.SetComment(comment)
	}
	return node
}

// Rewrites each LiteralNode body's leading indent to match its final parent context. blockScalarizeMultiline bakes a
// fixed 2-space indent into Value.Token.Origin at build time, before the final key column is known. Origin is a raw
// text field and AddColumn doesn't touch it - so any later shift (normalizeIndent moving keys, JSON-source `"..."`
// values whose original column bears no relation to the target block-scalar body column) leaves the body stuck at the
// wrong indent, silently producing invalid YAML. This pass runs after normalizeIndent so it operates on the final
// parent positions.
//
// Body indent is always parent_col + 2 (one level deeper than the parent key or sequence marker). We rebuild Origin
// from the parsed semantic value (Value.Value already carries relative indent), not from the current Origin's leading
// whitespace - the parent column is the authority.
func reindentBlockScalars(root ast.Node) {
	astutil.Walk(root, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.MappingValueNode:
			reindentIfLiteral(v.Value, v.Key.GetToken().Position.Column+2)
		case *ast.SequenceNode:
			if v.IsFlowStyle {
				return true
			}
			for _, item := range v.Values {
				reindentIfLiteral(item, v.Start.Position.Column+2)
			}
		}
		return true
	})
}

func reindentIfLiteral(n ast.Node, bodyCol int) {
	lit, ok := astutil.UnwrapAnchor(n).(*ast.LiteralNode)
	if !ok || lit.Value == nil {
		return
	}
	// `>` folded blocks use goccy's raw Origin to preserve author line breaks (or foldLongScalars's baked-in wrap).
	// Rebuilding Origin from Value.Value would flatten the wrap to a single line - drop the fold entirely. Only `|`
	// blocks get reindented.
	if strings.HasPrefix(lit.Start.Value, ">") {
		return
	}
	lit.Value.Token.Origin = renderLiteralOrigin(lit.Value.Value, bodyCol-1)
}

// Serialises a block scalar's semantic value into a LiteralNode's Origin form: each content line prefixed with
// `targetSpaces` spaces, then any extra trailing empty lines that a `|+` chomp needs to preserve. Empty interior lines
// are emitted as bare `\n` (no indent) - a block-scalar parser strips trailing whitespace on content lines, so
// Value.Value never has a line with trailing spaces, and indenting an empty line would only leak
// stripTrailingWhitespace-invisible dirt into the output.
func renderLiteralOrigin(value string, targetSpaces int) string {
	trailing := patch.TrailingNewlines(value)
	body := strings.TrimRight(value, "\n")
	indent := strings.Repeat(" ", targetSpaces)
	var sb strings.Builder
	for ln := range strings.SplitSeq(body, "\n") {
		if ln != "" {
			sb.WriteString(indent)
		}
		sb.WriteString(ln)
		sb.WriteByte('\n')
	}
	for i := 1; i < trailing; i++ {
		sb.WriteByte('\n')
	}
	return sb.String()
}
