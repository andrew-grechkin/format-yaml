package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// CommentNode is one comment group - a run of contiguous comment lines (no blank lines between them) that share a
// single attribution target. Lines holds one token per source line, in order. A blank-line-separated set of comment
// lines is TWO CommentNodes attached to different targets, not one CommentNode with a gap inside it.
type CommentNode struct {
	Lines []*token.Token
}

func (*CommentNode) Data() any { return nil }
