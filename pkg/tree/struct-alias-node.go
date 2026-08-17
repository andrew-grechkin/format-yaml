package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// AliasNode is a reference to a previously-defined anchor (`*name`). Star holds the `*` token. Data() returns
// nil so callers who need the resolved value do their own lookup via the anchor name.
type AliasNode struct {
	Comments
	Star *token.Token
	Name *token.Token
}

func (*AliasNode) Data() any { return nil }
