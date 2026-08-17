package tree

import (
	tokenizer "github.com/andrew-grechkin/format-yaml/internal/token"
)

// parseBlockSequence consumes a run of block-style `- item` entries at the same indent column. It stops
// when the next token is not a sequence entry at that column (dedent, doc marker, EOF). Island comments
// between items become EmptyNode children; the group adjacent to an item becomes its PrecedingComment.
func (p *parser) parseBlockSequence(pre *CommentNode) Node {
	s := &SequenceNode{Style: StyleBlock}
	s.PrecedingComment = pre
	baseCol := p.peek().Position.Column

	for {
		p.absorbComments()
		if p.eof() {
			break
		}
		t := p.peek()
		if t.Type != tokenizer.SequenceEntryType || t.Position.Column != baseCol {
			break
		}

		markerLine := t.Position.Line
		markerCol := t.Position.Column
		s.Children = append(s.Children, p.comments.islandsBefore(markerLine)...)
		itemPre := p.comments.takeIfAdjacent(markerLine)

		item := &SequenceItem{Marker: p.next()}
		item.PrecedingComment = itemPre

		// A comment may sit on the `-` line before a value on a later line (`- # c\n  x`). For an item with no
		// value it stays on the item; for a nested value it rides the value's PrecedingComment.
		markerComment := p.takeInline(markerLine)

		switch v := p.peek(); {
		case v == nil:
			item.Value = &NullNode{}
			item.InlineComment = markerComment
		case v.Position.Line == markerLine:
			item.Value = p.parseNode(nil, markerCol, false)
		default:
			p.absorbComments()
			nested := p.peek()
			if nested == nil {
				item.Value = &NullNode{}
				item.InlineComment = markerComment
			} else {
				islands := p.comments.islandsBefore(nested.Position.Line)
				valuePre := p.comments.takeIfAdjacent(nested.Position.Line)
				item.Value = p.parseNode(mergeComments(islandsComment(islands, markerComment), valuePre), markerCol, false)
			}
		}

		s.Children = append(s.Children, item)
	}

	s.Children = append(s.Children, p.trailingEmpties(baseCol)...)
	return s
}
