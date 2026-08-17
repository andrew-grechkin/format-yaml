package tree

import (
	tokenizer "github.com/andrew-grechkin/format-yaml/internal/token"
)

// parseDoc consumes one document: an optional `---` header, an optional body node, and an optional `...`
// footer. Comments adjacent to the header/footer/body attach to that slot; non-adjacent groups become
// island EmptyNode children in source order.
//
// When a doc has no body-typed token (only markers/comments), an implicit NullNode body is synthesized
// so every Doc has a body child, matching the emitter's expectation.
func (p *parser) parseDoc() *Doc {
	doc := &Doc{}

	if p.peek() != nil && p.peek().Type == tokenizer.DocumentHeaderType {
		line := p.peek().Position.Line
		pre := p.comments.takeIfAdjacent(line)

		doc.Header = &DocHeader{Token: p.next()}
		doc.Header.PrecedingComment = pre
		doc.Header.InlineComment = p.takeInline(line)
		p.absorbComments()
	}

	if t := p.peek(); t != nil && t.Type != tokenizer.DocumentEndType && t.Type != tokenizer.DocumentHeaderType {
		line := t.Position.Line
		doc.Children = append(doc.Children, p.comments.islandsBefore(line)...)
		pre := p.comments.takeIfAdjacent(line)
		doc.Children = append(doc.Children, p.parseNode(pre, 0, true))
		p.absorbComments()
	} else {
		var line int
		if doc.Header != nil {
			line = doc.Header.Token.Position.Line + 1
		}
		pre := p.comments.takeIfAdjacent(line)
		n := &NullNode{}
		n.PrecedingComment = pre
		doc.Children = append(doc.Children, n)
	}

	// End-of-doc split: at end-of-stream there is no forward target, so everything drains as trailing
	// islands; otherwise only the groups not adjacent to whatever follows belong to this doc, and the
	// adjacent one stays buffered for the footer (or the next doc's header) to claim.
	if t := p.peek(); t == nil {
		doc.Children = append(doc.Children, p.comments.drain()...)
	} else {
		doc.Children = append(doc.Children, p.comments.islandsBefore(t.Position.Line)...)
	}

	if t := p.peek(); t != nil && t.Type == tokenizer.DocumentEndType {
		line := t.Position.Line
		pre := p.comments.takeIfAdjacent(line)

		doc.Footer = &DocFooter{Token: p.next()}
		doc.Footer.PrecedingComment = pre
		doc.Footer.InlineComment = p.takeInline(line)
	}

	return doc
}
