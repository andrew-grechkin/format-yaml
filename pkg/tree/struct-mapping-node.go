package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// Style is a rendering choice for container nodes: block layout (indented, one entry per line) or flow layout
// (inline, comma-separated). Same semantic content either way; the emitter picks bytes based on the flag.
type Style int

const (
	StyleBlock Style = iota
	StyleFlow
)

// MappingNode is a YAML mapping (key-value collection). Children hold MappingEntry values (the entries) and
// EmptyNodes for any island comments between entries. Open/Close carry the `{`/`}` tokens for flow style
// (nil for block).
type MappingNode struct {
	Comments
	Style    Style
	Open     *token.Token
	Children []Node
	Close    *token.Token
}

func (m *MappingNode) Data() any {
	out := make(map[string]any, len(m.Children))
	for _, c := range m.Children {
		e, ok := c.(*MappingEntry)
		if !ok {
			continue
		}
		key := ""
		if e.Key != nil {
			if s, ok := e.Key.Data().(string); ok {
				key = s
			}
		}
		if e.Value != nil {
			out[key] = e.Value.Data()
		} else {
			out[key] = nil
		}
	}
	return out
}
