package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// Tokened is satisfied by every node whose rendered content is a SINGLE primary token (ScalarBase's Token,
// DocHeader/DocFooter's Token). Container types, wrappers, and entries don't implement it - their content
// spans multiple tokens. Passes that operate on "the token adjacent to this node's inline comment"
// (chomp trailing whitespace, etc.) filter for Tokened.
type Tokened interface {
	Node
	ContentToken() *token.Token
}
