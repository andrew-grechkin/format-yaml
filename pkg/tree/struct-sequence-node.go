package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// SequenceNode is a YAML sequence (ordered list). Children hold SequenceItem values plus EmptyNodes for island
// comments between items. Open/Close carry the `[`/`]` tokens for flow style; nil for block.
type SequenceNode struct {
	Comments
	Style    Style
	Open     *token.Token
	Children []Node
	Close    *token.Token
}
