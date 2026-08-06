package passes

import (
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"
	"github.com/andrew-grechkin/update-yaml/pkg/style"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// Rewrites plain and quoted single-line StringNodes as `>-` folded block scalars when their rendered `key: value` line
// crosses the configured line width. YAML folds interior line breaks back to spaces on parse, so the wrap is
// value-preserving as long as:
//
//   - the value has no `\n` (blockScalarizeMultiline handles those as literal `|` blocks instead)
//   - the value doesn't start or end with whitespace (folded scalars strip trailing whitespace per line and use the
//   first content line's indent as the block's base - leading whitespace on the value would either misparse or get
//   eaten)
//   - the value contains at least one single-space break point (multi-space runs stay glued and no-space values have
//   nowhere to wrap - leave both as long plain scalars)
//
// The wrap algorithm splits at single-space boundaries only. Multi-space runs stay with their surrounding words on a
// single emitted line so fold-back preserves them byte-for-byte. Existing block scalars (`|` or `>`) from the source or
// from earlier passes are left alone.
//
// Runs after normalizeIndent so the parent key column is final and the folded body can bake in the correct
// 2-space-past-key indent from the start. reindentBlockScalars skips `>` blocks precisely so this pass's baked-in
// Origin survives to the emitter untouched.
func foldLongScalars(root ast.Node) {
	astutil.Walk(root, func(n ast.Node) bool {
		mv, ok := n.(*ast.MappingValueNode)
		if !ok {
			return true
		}
		if folded := maybeFoldScalar(mv); folded != nil {
			mv.Value = folded
		}
		return true
	})
}

func maybeFoldScalar(mv *ast.MappingValueNode) ast.Node {
	s, ok := astutil.UnwrapAnchor(mv.Value).(*ast.StringNode)
	if !ok {
		return nil
	}
	if !canFoldPlain(s.Value) {
		return nil
	}
	if !overflowsLineWidth(mv) {
		return nil
	}
	bodyCol := mv.Key.GetToken().Position.Column + 2
	return buildFoldedNode(s.Value, bodyCol, s.GetComment())
}

// Reports whether v can be encoded as a `>-` folded block and folded back byte-for-byte. See foldLongScalars for the
// reasoning behind each condition.
func canFoldPlain(v string) bool {
	if v == "" || strings.Contains(v, "\n") {
		return false
	}
	if v[0] == ' ' || v[len(v)-1] == ' ' {
		return false
	}
	return hasSingleSpaceBreak(v)
}

// Reports whether v contains at least one position where a single space separates two non-space chars. Only those
// positions are valid wrap boundaries; a value made entirely of a single word or a single multi-space run has none.
func hasSingleSpaceBreak(v string) bool {
	for i := 0; i < len(v); i++ {
		if v[i] != ' ' {
			continue
		}
		prev := i > 0 && v[i-1] == ' '
		next := i+1 < len(v) && v[i+1] == ' '
		if !prev && !next {
			return true
		}
	}
	return false
}

// Synthesizes a `>-` block scalar carrying value with wrap lines whose display width fits within LineWidth when
// indented to bodyCol. The synthetic source has the target indent baked in from the start, so the resulting Origin
// renders correctly without a follow-up reindent pass.
func buildFoldedNode(value string, bodyCol int, comment *ast.CommentGroupNode) ast.Node {
	lines := wrapForFold(value, bodyCol)
	if len(lines) == 0 {
		return nil
	}
	indent := strings.Repeat(" ", bodyCol-1)
	var sb strings.Builder
	sb.WriteString(">-\n")
	for _, ln := range lines {
		sb.WriteString(indent)
		sb.WriteString(ln)
		sb.WriteByte('\n')
	}
	file, err := parser.ParseBytes([]byte(sb.String()), 0)
	if err != nil || len(file.Docs) != 1 {
		return nil
	}
	node := file.Docs[0].Body
	if comment != nil {
		_ = node.SetComment(comment)
	}
	return node
}

// Adapts style.FoldWrap to format-yaml's bodyCol-based interface: converts (value, bodyCol) into the (value, width)
// form the shared primitive uses, computing the display-column budget from the active LineWidth setting.
func wrapForFold(value string, bodyCol int) []string {
	if value == "" {
		return nil
	}
	return style.FoldWrap(value, max(1, config.LineWidth-(bodyCol-1)))
}
