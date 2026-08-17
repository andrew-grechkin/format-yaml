package passes

import (
	"strings"

	"github.com/andrew-grechkin/format-yaml/internal/token"
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// stripTrailingSpace removes trailing spaces/tabs at the end of every rendered line, except inside
// block-scalar bodies where they are significant content. Runs at every mode - "no trailing whitespace" is a
// baseline formatting MUST.
//
// It works token-by-token rather than on the final byte stream so it can tell struct from content: a
// StringNode's body token (Header != nil) is skipped, everything else is stripped.
func stripTrailingSpace(f *tree.File) { tree.Walk(f, stripNodeTrivia) }

// stripNodeTrivia strips line-trailing whitespace from a node's comment slots and from its line-bearing
// content tokens. Structural tokens (colons, `- ` markers, flow brackets, commas) are skipped: they never
// span lines, and their trailing space is a separator, not dirt.
func stripNodeTrivia(n tree.Node) {
	if c, ok := n.(tree.Commented); ok {
		stripCommentLines(c.Preceding())
		stripCommentLines(c.Inline())
	}
	switch v := n.(type) {
	case *tree.StringNode:
		if v.Header != nil {
			stripTokenLines(v.Header)
			return
		}
		stripTokenLines(v.Token)
	case *tree.IntNode:
		stripTokenLines(v.Token)
	case *tree.FloatNode:
		stripTokenLines(v.Token)
	case *tree.BoolNode:
		stripTokenLines(v.Token)
	case *tree.NullNode:
		stripTokenLines(v.Token)
	case *tree.DocHeader:
		stripTokenLines(v.Token)
	case *tree.DocFooter:
		stripTokenLines(v.Token)
	}
}

func stripCommentLines(c *tree.CommentNode) {
	if c == nil {
		return
	}
	for _, ln := range c.Lines {
		stripTokenLines(ln)
	}
}

func stripTokenLines(t *token.Token) {
	if t == nil {
		return
	}
	t.Origin = stripLineTrailing(t.Origin)
}

// stripLineTrailing removes runs of spaces/tabs at the end of every line of s, including a final run with no
// `\n` after it. Called only on scalar/marker content and comment tokens - never on the structural separators
// (`: `, `- `, `, `) that legitimately end in a space, and never on block-scalar bodies. A scalar's separator
// whitespace before an inline comment lives on the comment token, so trimming here cannot merge value and comment.
func stripLineTrailing(s string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t")
	}
	return strings.Join(lines, "\n")
}
