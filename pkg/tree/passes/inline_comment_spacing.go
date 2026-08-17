package passes

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// normalizeInlineCommentSpacing rewrites the whitespace between a node's content and its inline comment
// to exactly two spaces. Applies to every node that has BOTH a Commented slot AND a single primary
// content token (the Tokened interface): scalars and doc markers. Container/wrapper/entry nodes have
// multi-token content and are skipped for now - inline comments live on their leaves in practice.
//
// Two-step per node: chomp trailing same-line whitespace off ContentToken (the value's trailing spaces
// left over from source alignment), then set the inline comment's first line leading to exactly two
// spaces. Applies uniformly whether the underlying token holds pre-comment whitespace on its trailing
// (goccy's usual output) or on the comment's leading - after this pass, exactly two spaces sit on the
// comment side.
func normalizeInlineCommentSpacing(f *tree.File) {
	tree.Walk(f, func(n tree.Node) {
		c, ok := n.(tree.Commented)
		if !ok {
			return
		}
		t, ok := n.(tree.Tokened)
		if !ok {
			return
		}
		applyInlineSpacing(c.Inline(), t.ContentToken())
	})
}

func applyInlineSpacing(inline *tree.CommentNode, content *token.Token) {
	if inline == nil || len(inline.Lines) == 0 {
		return
	}
	if content != nil {
		i := len(content.Origin)
		for i > 0 && (content.Origin[i-1] == ' ' || content.Origin[i-1] == '\t') {
			i--
		}
		content.Origin = content.Origin[:i]
	}
	first := inline.Lines[0]
	i := 0
	for i < len(first.Origin) && (first.Origin[i] == ' ' || first.Origin[i] == '\t') {
		i++
	}
	first.Origin = "  " + first.Origin[i:]
}
