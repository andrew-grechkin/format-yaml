package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// TagNode wraps a value with a YAML tag (`!tag`, `!!str`, `!<uri>`). Data() returns the underlying Value's
// Data - tagging is a type-annotation, not its own semantic content.
type TagNode struct {
	Comments
	Tag   *token.Token
	Value Node
}

func (t *TagNode) Data() any {
	if t.Value == nil {
		return nil
	}
	return t.Value.Data()
}
