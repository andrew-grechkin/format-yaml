package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// Doc is a single YAML document. Header/Footer are typed attributes (single, nilable) so the tree can't hold more
// than one of each - preventing whole classes of malformed-tree bugs at compile time. Children is the ordered list
// of everything inside the doc: the body node (ScalarNode, NullNode, and later MappingNode/SequenceNode) plus any
// EmptyNodes for island comments that render between/around it. YAML forbids more than one body-typed child; the
// builder enforces that.
type Doc struct {
	Header   *DocHeader
	Children []Node
	Footer   *DocFooter
}

func (d *Doc) Data() any {
	for _, c := range d.Children {
		if _, ok := c.(*EmptyNode); ok {
			continue
		}
		return c.Data()
	}
	return nil
}

// UpdateHeader sets d.Header to a canonical `---` DocHeader, replacing any existing Header. The
// synthesized Token.Origin is `---\n` (marker plus its terminating newline). Optional preceding comment
// gets attached as the header's PrecedingComment so any comment group that logically sits above the
// marker keeps its position on emit.
//
// Named Update rather than Create because the operation is idempotent from the caller's perspective:
// "make d.Header be the canonical marker with this preceding comment"; it doesn't matter whether one was
// there before. Callers that only want to add a header when missing should guard with `if d.Header == nil`.
func (d *Doc) UpdateHeader(preceding *CommentNode) *DocHeader {
	h := &DocHeader{Token: &token.Token{Value: "---", Origin: "---\n"}}
	h.PrecedingComment = preceding
	d.Header = h
	return h
}
