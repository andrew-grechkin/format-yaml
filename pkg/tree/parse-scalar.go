package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/assert"
	tokenizer "github.com/andrew-grechkin/format-yaml/internal/token"
)

// parseScalar consumes one scalar leaf (or a block scalar's header + optional body) as the node with the
// given PrecedingComment. The typed node is chosen from the token type; source spelling is preserved in
// Token.Origin so emission stays byte-identical.
func (p *parser) parseScalar(pre *CommentNode) Node {
	t := p.peek()
	assert.NotNil(t, "parseScalar called at end of stream")

	switch t.Type {
	case tokenizer.NullType, tokenizer.ImplicitNullType:
		tok := p.next()
		n := &NullNode{}
		n.Token = tok
		n.PrecedingComment = pre
		n.InlineComment = p.takeInline(tok.Position.Line)
		return n

	case tokenizer.LiteralType, tokenizer.FoldedType:
		header := p.next()
		style := StringLiteral
		if header.Type == tokenizer.FoldedType {
			style = StringFolded
		}
		s := &StringNode{Style: style, Header: header}
		s.PrecedingComment = pre
		// A same-line comment after the header is inline on the header; the body lives on later lines.
		s.InlineComment = p.takeInline(header.Position.Line)
		if next := p.peek(); next != nil && next.Type != tokenizer.CommentType && next.Position.Line != header.Position.Line {
			s.Token = p.next()
		}
		return s

	case tokenizer.StringType, tokenizer.SingleQuoteType, tokenizer.DoubleQuoteType, tokenizer.MergeKeyType:
		tok := p.next()
		s := &StringNode{Style: stringStyleFor(tok.Type)}
		s.Token = tok
		s.PrecedingComment = pre
		s.InlineComment = p.takeInline(tok.Position.Line)
		return s

	case tokenizer.IntegerType, tokenizer.BinaryIntegerType, tokenizer.OctetIntegerType, tokenizer.HexIntegerType:
		tok := p.next()
		n := &IntNode{}
		n.Token = tok
		n.PrecedingComment = pre
		n.InlineComment = p.takeInline(tok.Position.Line)
		return n

	case tokenizer.FloatType, tokenizer.InfinityType, tokenizer.NanType:
		tok := p.next()
		n := &FloatNode{}
		n.Token = tok
		n.PrecedingComment = pre
		n.InlineComment = p.takeInline(tok.Position.Line)
		return n

	case tokenizer.BoolType:
		tok := p.next()
		n := &BoolNode{}
		n.Token = tok
		n.PrecedingComment = pre
		n.InlineComment = p.takeInline(tok.Position.Line)
		return n
	}

	p.fail("unsupported token type %s at line %d (%q)", t.Type, t.Position.Line, t.Value)
	return nil
}

// stringStyleFor maps a string-scalar token type to its StringStyle. Block scalar headers (`|`/`>`) are
// handled separately since they arrive as a header + body pair.
func stringStyleFor(t tokenizer.Type) StringStyle {
	switch t {
	case tokenizer.SingleQuoteType:
		return StringSingleQuoted
	case tokenizer.DoubleQuoteType:
		return StringDoubleQuoted
	}
	return StringPlain
}
