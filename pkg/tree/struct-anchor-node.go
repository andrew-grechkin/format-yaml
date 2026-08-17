package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// AnchorNode wraps another node with an anchor name (`&name`). Amp holds the `&` token. Data() returns the
// Value's Data - the anchor is a labeling wrapper, not its own semantic content.
type AnchorNode struct {
	Comments
	Amp   *token.Token
	Name  *token.Token
	Value Node
}

func (a *AnchorNode) Data() any {
	if a.Value == nil {
		return nil
	}
	return a.Value.Data()
}
