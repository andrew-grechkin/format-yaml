package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// SequenceItem wraps a single sequence entry so its `-` marker (block style) can be preserved verbatim.
// Flow-style items still use SequenceItem for uniformity - Marker is nil there. Trailer is the post-item
// marker: `,` in flow, `\n` in block.
type SequenceItem struct {
	Comments
	Marker  *token.Token
	Value   Node
	Trailer *token.Token
}

func (i *SequenceItem) Data() any {
	if i.Value == nil {
		return nil
	}
	return i.Value.Data()
}
