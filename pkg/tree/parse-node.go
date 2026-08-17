package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/assert"
	tokenizer "github.com/andrew-grechkin/format-yaml/internal/token"
)

// parseNode builds a single body node with pre as its PrecedingComment. Dispatch is driven purely by the
// current token: wrappers first, then containers, then scalars. Block containers take their base indent
// column from the token that starts them, so their own nesting is encoded by the token stream.
//
// parentCol is the indentation of the node that owns this value (the mapping key or sequence marker column,
// 0 at document level). allowSameColSeq enables YAML's exception that a block sequence may sit at the SAME
// column as its parent key (`key:\n- item`); it is false for sequence items, where a same-column `-` is a
// sibling. Wrappers need both to decide whether a following line is their value.
func (p *parser) parseNode(pre *CommentNode, parentCol int, allowSameColSeq bool) Node {
	t := p.peek()
	assert.NotNil(t, "parseNode called at end of stream")

	switch {
	case t.Type == tokenizer.TagType:
		return p.parseTag(pre, parentCol, allowSameColSeq)
	case t.Type == tokenizer.AnchorType:
		return p.parseAnchor(pre, parentCol, allowSameColSeq)
	case t.Type == tokenizer.AliasType:
		return p.parseAlias(pre)
	case t.Type == tokenizer.SequenceEntryType:
		return p.parseBlockSequence(pre)
	case t.Type == tokenizer.SequenceStartType:
		return p.parseFlowSequence(pre)
	case t.Type == tokenizer.MappingStartType:
		return p.parseFlowMapping(pre)
	case p.isMappingKeyHere():
		return p.parseBlockMapping(pre)
	default:
		return p.parseScalar(pre)
	}
}

// isMappingKeyHere reports whether the current token starts a mapping entry: an implicit `key:` (scalar
// directly followed by `:`) or an explicit `?` key marker.
func (p *parser) isMappingKeyHere() bool {
	t := p.peek()
	if t == nil {
		return false
	}
	if t.Type == tokenizer.MappingKeyType {
		return true
	}
	if !isScalarType(t.Type) {
		return false
	}
	next := p.peekAt(1)
	return next != nil && next.Type == tokenizer.MappingValueType
}

// isScalarType reports whether a token type is a scalar leaf value (including null and merge-key). It is
// the one place that knows which token types parseScalar can consume.
func isScalarType(t tokenizer.Type) bool {
	switch t {
	case tokenizer.NullType, tokenizer.ImplicitNullType,
		tokenizer.StringType, tokenizer.SingleQuoteType, tokenizer.DoubleQuoteType,
		tokenizer.BoolType,
		tokenizer.IntegerType, tokenizer.BinaryIntegerType, tokenizer.OctetIntegerType, tokenizer.HexIntegerType,
		tokenizer.FloatType, tokenizer.InfinityType, tokenizer.NanType,
		tokenizer.MergeKeyType:
		return true
	}
	return false
}
