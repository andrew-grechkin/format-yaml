package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// DocHeader represents the `---` marker at the start of a document.
type DocHeader struct {
	Comments
	Token *token.Token
}

func (*DocHeader) Data() any { return nil }

// ContentToken returns the `---` Token, satisfying the Tokened interface.
func (d *DocHeader) ContentToken() *token.Token { return d.Token }
