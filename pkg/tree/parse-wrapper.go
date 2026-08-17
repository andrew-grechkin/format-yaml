package tree

import (
	tokenizer "github.com/andrew-grechkin/format-yaml/internal/token"
)

// parseAnchor consumes `&name` plus the value it labels. The `&` is a standalone token followed by a
// scalar name; a same-line node is always the value, a following-line node is the value only when it is
// indented past parentCol (otherwise it is a sibling and the anchor labels null).
func (p *parser) parseAnchor(pre *CommentNode, parentCol int, allowSameColSeq bool) Node {
	amp := p.next() // `&`
	name := p.peek()
	if name == nil || !isScalarType(name.Type) {
		p.fail("anchor `&` at line %d not followed by a name", amp.Position.Line)
	}

	a := &AnchorNode{Amp: amp, Name: p.next()}
	a.PrecedingComment = pre
	a.Value = p.parseWrappedValue(a, amp.Position.Line, parentCol, allowSameColSeq)
	return a
}

// parseTag consumes `!tag` plus the value it tags (null when nothing follows or the next line is a sibling).
func (p *parser) parseTag(pre *CommentNode, parentCol int, allowSameColSeq bool) Node {
	tag := p.next()
	t := &TagNode{Tag: tag}
	t.PrecedingComment = pre
	t.Value = p.parseWrappedValue(t, tag.Position.Line, parentCol, allowSameColSeq)
	return t
}

// parseWrappedValue resolves a wrapper's value and routes any same-line comment. A comment after the wrapper
// name (`&x # c`) belongs to the empty wrapper when there is no value, or to the wrapped value's preceding
// comment when the value sits on following lines.
func (p *parser) parseWrappedValue(w Commented, wrapperLine, parentCol int, allowSameColSeq bool) Node {
	cmt := p.takeInline(wrapperLine)
	value := p.takeWrappedValue(wrapperLine, parentCol, allowSameColSeq)
	if cmt == nil {
		return value
	}
	if n, ok := value.(*NullNode); ok && n.Token == nil {
		w.SetInline(mergeComments(w.Inline(), cmt))
		return value
	}
	if s, ok := value.(Commented); ok {
		s.SetPreceding(mergeComments(cmt, s.Preceding()))
	}
	return value
}

// takeWrappedValue returns the value following a wrapper token (`&name` / `!tag`): the next node when it is on
// the wrapper's own line, or when it starts a block on a following line indented past parentCol. Otherwise the
// wrapper has no value and an implicit null is returned.
func (p *parser) takeWrappedValue(wrapperLine, parentCol int, allowSameColSeq bool) Node {
	nested := p.peek()
	if nested == nil || isWrapperTerminator(nested.Type) {
		return &NullNode{}
	}
	if nested.Position.Line == wrapperLine || isNestedToken(parentCol, allowSameColSeq, nested) {
		return p.parseNode(nil, parentCol, allowSameColSeq)
	}
	return &NullNode{}
}

// isWrapperTerminator reports whether a token can only belong to an enclosing construct, never be a wrapper's
// value: document markers and flow closers/separators. This is what makes `[&x]`, `{a: !t}` and `!t\n---` parse
// as a wrapper over an empty node.
func isWrapperTerminator(t tokenizer.Type) bool {
	switch t {
	case tokenizer.DocumentHeaderType, tokenizer.DocumentEndType,
		tokenizer.CollectEntryType, tokenizer.SequenceEndType, tokenizer.MappingEndType:
		return true
	}
	return false
}

// parseAlias consumes `*name`, a reference to a previously-defined anchor.
func (p *parser) parseAlias(pre *CommentNode) Node {
	star := p.next() // `*`
	name := p.peek()
	if name == nil || !isScalarType(name.Type) {
		p.fail("alias `*` at line %d not followed by a name", star.Position.Line)
	}

	a := &AliasNode{Star: star, Name: p.next()}
	a.PrecedingComment = pre
	a.InlineComment = p.takeInline(a.Name.Position.Line)
	return a
}
