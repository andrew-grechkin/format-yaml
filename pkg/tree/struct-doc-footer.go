package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// DocFooter represents the `...` marker at the end of a document. Same comment slots as DocHeader.
type DocFooter struct {
	Comments
	Token *token.Token
}

func (*DocFooter) Data() any { return nil }

// ContentToken returns the `...` Token, satisfying the Tokened interface.
func (d *DocFooter) ContentToken() *token.Token { return d.Token }
