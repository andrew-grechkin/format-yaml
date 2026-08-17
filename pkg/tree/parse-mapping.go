package tree

import (
	tokenizer "github.com/andrew-grechkin/format-yaml/internal/token"
)

// parseBlockMapping consumes a run of block-style `key: value` entries at the same indent column. It
// stops when the next token is not a mapping key at that column (dedent, doc marker, EOF, or a
// non-key shape). Island comments between entries become EmptyNode children; the group adjacent to an
// entry becomes that entry's PrecedingComment.
func (p *parser) parseBlockMapping(pre *CommentNode) Node {
	m := &MappingNode{Style: StyleBlock}
	m.PrecedingComment = pre
	baseCol := p.peek().Position.Column

	for {
		p.absorbComments()
		if p.eof() {
			break
		}
		t := p.peek()
		if !p.isMappingKeyHere() || t.Position.Column != baseCol {
			break
		}

		line := t.Position.Line
		m.Children = append(m.Children, p.comments.islandsBefore(line)...)
		entryPre := p.comments.takeIfAdjacent(line)
		m.Children = append(m.Children, p.parseMappingEntry(entryPre))
	}

	m.Children = append(m.Children, p.trailingEmpties(baseCol)...)
	return m
}

// parseMappingEntry consumes one `key: value` pair (implicit or explicit `? key`). The value may be a
// same-line scalar/flow node, an implicit null, or a nested node on a following line at a deeper
// indent. A following token at the same or shallower column than the key is a sibling - not a value -
// so the entry gets an implicit null.
func (p *parser) parseMappingEntry(pre *CommentNode) *MappingEntry {
	entry := &MappingEntry{}
	entry.PrecedingComment = pre

	keyPre := (*CommentNode)(nil)
	entryCol := 0
	if p.peek() != nil && p.peek().Type == tokenizer.MappingKeyType {
		entry.ExplicitKeyMarker = p.next()
		// A comment may sit between `?` and the key (`? # c\n  key`); it belongs above the key.
		keyPre = p.takeInline(entry.ExplicitKeyMarker.Position.Line)
		entryCol = entry.ExplicitKeyMarker.Position.Column
	} else {
		entryCol = p.peek().Position.Column
	}
	keyStartLine := p.peek().Position.Line
	entry.Key = p.parseScalar(keyPre)

	colon := p.peek()
	if colon == nil || colon.Type != tokenizer.MappingValueType {
		p.fail("expected ':' after mapping key at line %d", keyStartLine)
	}
	keyLine := colon.Position.Line
	entry.Colon = p.next()

	// A comment on the colon's line precedes the value. Capture it up front; if the value turns out to sit on
	// the same line it will have consumed the comment itself, so this stays nil for the common `k: v # c` case.
	colonComment := p.takeInline(keyLine)

	switch t := p.peek(); {
	case t == nil:
		entry.Value = nullWithComment(colonComment)
	case t.Position.Line == keyLine:
		entry.Value = p.parseNode(nil, entryCol, true)
	default:
		p.absorbComments()
		nested := p.peek()
		if nested == nil || !isNestedToken(entryCol, true, nested) {
			entry.Value = nullWithComment(colonComment)
			return entry
		}
		islands := p.comments.islandsBefore(nested.Position.Line)
		valuePre := p.comments.takeIfAdjacent(nested.Position.Line)
		entry.Value = p.parseNode(mergeComments(islandsComment(islands, colonComment), valuePre), entryCol, true)
	}

	return entry
}

// nullWithComment builds an implicit NullNode, folding a comment that sat on the `key:` line into the null's
// inline slot. At minimal that renders `key: # c`; once materialized at standard+ it renders `key: null  # c`.
func nullWithComment(c *CommentNode) *NullNode {
	n := &NullNode{}
	n.InlineComment = c
	return n
}

// isNestedToken reports whether nested, which starts on a later line than its owner, is that owner's value
// rather than a sibling. The general rule is strict indentation, but YAML makes one exception: a block
// sequence may sit at the SAME column as its parent key (`key:\n- item`), so a sequence-entry token at the
// owner's column is still that owner's value.
func isNestedToken(parentCol int, allowSameColSeq bool, nested *tokenizer.Token) bool {
	if nested.Position.Column > parentCol {
		return true
	}
	return allowSameColSeq && nested.Type == tokenizer.SequenceEntryType && nested.Position.Column == parentCol
}

// trailingEmpties returns the pending comment groups that belong to a just-closed container: all of them
// when no forward target exists (EOF or a doc marker), otherwise only the non-adjacent ones - the
// adjacent group is left in the buffer for the enclosing container's next node to claim.
//
// baseCol is the container's indentation. Groups dedented below it are left buffered: a comment that sits a
// level up (e.g. a doc-trailing comment after a nested mapping) must bubble to the enclosing scope instead
// of being adopted and indent-shifted by this container.
func (p *parser) trailingEmpties(baseCol int) []Node {
	next := p.peek()
	if next == nil {
		return p.comments.drainAtLeast(baseCol)
	}
	// A group source-adjacent to the following `---`/`...` must be left for that marker to claim as its
	// PrecedingComment; only the non-adjacent islands belong to the closing container.
	return p.comments.islandsBeforeAtLeast(next.Position.Line, baseCol)
}
