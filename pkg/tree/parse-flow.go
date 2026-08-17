package tree

import (
	tokenizer "github.com/andrew-grechkin/format-yaml/internal/token"
)

// parseFlowSequence consumes a `[...]` flow sequence: `[` item (`,` item)* `]`. Items can be any body
// node; a trailing comma before `]` is allowed. Comments between items are attached to the preceding item's
// inline slot (or held for the next item), so they round-trip instead of aborting the parse.
func (p *parser) parseFlowSequence(pre *CommentNode) Node {
	open := p.next() // `[`
	s := &SequenceNode{Style: StyleFlow, Open: open}
	s.PrecedingComment = pre

	var pending *CommentNode
	for {
		for c := p.takeFlowComment(); c != nil; c = p.takeFlowComment() {
			pending = mergeComments(pending, c)
		}
		t := p.peek()
		if t == nil {
			p.fail("unterminated flow sequence started at line %d", open.Position.Line)
		}
		if t.Type == tokenizer.SequenceEndType {
			s.Close = p.next()
			break
		}

		item := &SequenceItem{Value: p.parseNode(nil, open.Position.Column, false)}
		item.PrecedingComment = pending
		pending = nil
		if p.peek() != nil && p.peek().Type == tokenizer.CollectEntryType {
			item.Trailer = p.next()
		}
		// A comment after the item's comma rides the item's inline slot (rendered after the trailer).
		for c := p.takeFlowComment(); c != nil; c = p.takeFlowComment() {
			item.InlineComment = mergeComments(item.InlineComment, c)
		}
		s.Children = append(s.Children, item)
	}

	// Comments just before `]` had no following item to attach to; keep them after the close.
	s.InlineComment = mergeComments(p.takeInline(open.Position.Line), pending)
	return s
}

// parseFlowMapping consumes a `{...}` flow mapping: `{` entry (`,` entry)* `}` where each entry is a
// scalar key followed by `:` and any body value. Comments are attached as in parseFlowSequence.
func (p *parser) parseFlowMapping(pre *CommentNode) Node {
	open := p.next() // `{`
	m := &MappingNode{Style: StyleFlow, Open: open}
	m.PrecedingComment = pre

	var pending *CommentNode
	for {
		for c := p.takeFlowComment(); c != nil; c = p.takeFlowComment() {
			pending = mergeComments(pending, c)
		}
		t := p.peek()
		if t == nil {
			p.fail("unterminated flow mapping started at line %d", open.Position.Line)
		}
		if t.Type == tokenizer.MappingEndType {
			m.Close = p.next()
			break
		}

		entry := &MappingEntry{Key: p.parseScalar(nil)}
		entry.PrecedingComment = pending
		pending = nil
		colon := p.peek()
		if colon == nil || colon.Type != tokenizer.MappingValueType {
			p.fail("expected ':' after flow mapping key at line %d", open.Position.Line)
		}
		entry.Colon = p.next()

		if p.peek() == nil {
			p.fail("flow mapping key with no value at line %d", open.Position.Line)
		}
		entry.Value = p.parseNode(nil, open.Position.Column, false)
		if p.peek() != nil && p.peek().Type == tokenizer.CollectEntryType {
			entry.Trailer = p.next()
		}
		for c := p.takeFlowComment(); c != nil; c = p.takeFlowComment() {
			entry.InlineComment = mergeComments(entry.InlineComment, c)
		}
		m.Children = append(m.Children, entry)
	}

	m.InlineComment = mergeComments(p.takeInline(open.Position.Line), pending)
	return m
}
