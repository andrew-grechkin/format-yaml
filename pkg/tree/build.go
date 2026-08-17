// Package tree defines our own YAML AST built directly from goccy's lexer token stream. Goal is a tree that
// attributes comments by physical source layout, treats markers/comments/values uniformly, and is easy to walk in
// both directions - none of the goccy-inherited pain (token linked lists, mis-attributed head/foot comments, no
// doc-level trailing slot, wrapper-required Body field). Types here are public so the package can eventually move
// to update-yaml and be shared. For now format-yaml owns it while we shape it.
//
// First slice: File, Doc, DocHeader, DocFooter, EmptyNode, ScalarNode, NullNode, CommentNode. No mappings,
// sequences, anchors, tags, block scalars yet - anything the token stream carries beyond scalar bodies + comments +
// doc markers panics, on purpose, so unsupported inputs fail loudly during development.
package tree

import (
	"fmt"

	"github.com/andrew-grechkin/format-yaml/internal/lexer"
	tokenizer "github.com/andrew-grechkin/format-yaml/internal/token"
)

// Build parses src via internal lexer (goccy fork), then constructs tree. Panics if the token stream
// carries any token type we don't yet handle - deliberate, so unsupported inputs surface immediately during
// development instead of getting a silently-dropped subtree. Tokens arrive with our "trailing-only" Origin
// invariant already applied (see lexer.TokenizeNormalized), so downstream emit is just Origin concatenation.
func Build(src []byte) *File {
	tokens := lexer.TokenizeNormalized(string(src))

	b := &builder{tokens: tokens}

	return b.buildFile()
}

type iterator struct {
	tokens tokenizer.Tokens
	idx    int
	commentStash *CommentNode
}

func (i *iterator) peek() *tokenizer.Token {
	if i.idx >= len(i.tokens) {
		return nil
	}
	return i.tokens[i.idx]
}

func (i *iterator) next() bool {
	t := i.peek()
	i.idx++
	return t
}

func (i *iterator) buildFile1() *File {
	result := &File{}

	for token := i.next(); token != nil; token = i.next() {
		if token.Type == tokenizer.CommentType {
			if i.commentStash != nil {
				result.Children = append(result.Children, EmptyNode())
			}

			i.commentStash = i.StandaloneCommentsGroup()
			continue
		}

	}

	if i.commentStash != nil {
		result.Children = append(result.Children, EmptyNode(i.commentStash))
	}
}


// builder walks the token stream sequentially. groups holds the not-yet-attached comment groups in source order.
// The last group is "still open" - subsequent contiguous comment lines extend it; a non-comment or a line gap
// closes it and starts a new one on the next comment.
type builder struct {
	tokens tokenizer.Tokens
	idx    int
	groups []*CommentNode
}

func (b *builder) peek() *tokenizer.Token {
	if b.idx >= len(b.tokens) {
		return nil
	}
	return b.tokens[b.idx]
}

func (b *builder) advance() *tokenizer.Token {
	t := b.tokens[b.idx]
	b.idx++
	return t
}

// buildFile walks the whole stream, producing File.Children. Between Docs, absorbed comment groups split by
// adjacency: any group NOT source-adjacent to the coming node becomes a File-level EmptyNode (island comment
// sitting between/around docs); the adjacent group stays in b.groups so buildDoc can attach it as the coming
// header's PrecedingComment. Trailing groups at end-of-stream flush as File-level EmptyNodes.
func (b *builder) buildFile() *File {
	file := &File{}

	for b.idx < len(b.tokens) {
		b.absorbStandaloneCommentsGroup()

		if b.peek() == nil {
			break
		}

		if b.peek().Type == tokenizer.DirectiveType {
			file.Children = append(file.Children, b.buildDirective())
			continue
		}

		file.Children = append(file.Children, b.drainGroupsAdjacentTo(b.peek().Position.Line)...)

		doc := b.buildDoc()
		if doc == nil {
			break
		}

		file.Children = append(file.Children, doc)
	}

	file.Children = append(file.Children, b.drainGroupsAsEmpties()...)

	return file
}

// buildDirective consumes a `%NAME arg1 arg2 ...` directive that sits before any doc header. `%` is the DirectiveType
// token; the name and args follow as scalar tokens on the same line. The directive terminates at the next line's
// first token.
func (b *builder) buildDirective() *DirectiveNode {
	pct := b.advance()
	if b.peek() == nil || !isScalarType(b.peek().Type) {
		panic(fmt.Sprintf("tree.Build: directive `%%` at line %d not followed by a name", pct.Position.Line))
	}
	d := &DirectiveNode{Name: b.advance()}
	for b.peek() != nil && b.peek().Position.Line == d.Name.Position.Line && isScalarType(b.peek().Type) {
		d.Args = append(d.Args, b.advance())
	}
	return d
}

// buildDoc consumes tokens for one document. Header/Footer go into typed slots; Children holds the body node and
// any island EmptyNodes in source order. Assumes buildFile has already sorted preceding comments - any group left
// in b.groups is adjacent to the coming header (or body, if there's no header).
func (b *builder) buildDoc() *Doc {
	if b.peek() == nil {
		return nil
	}

	doc := &Doc{}
	if b.peek().Type == tokenizer.DocumentHeaderType {
		line := b.peek().Position.Line
		preceding := b.takeAdjacentGroup(line)

		doc.Header = &DocHeader{Token: b.advance()}
		doc.Header.PrecedingComment = preceding
		doc.Header.InlineComment = b.takeInlineOn(doc.Header.Token.Position.Line)

		b.absorbStandaloneCommentsGroup()
	}

	if t := b.peek(); t != nil && t.Type != tokenizer.DocumentEndType && t.Type != tokenizer.DocumentHeaderType {
		line := t.Position.Line
		doc.Children = append(doc.Children, b.drainGroupsAdjacentTo(line)...)
		preceding := b.takeAdjacentGroup(line)
		doc.Children = append(doc.Children, b.buildBody(preceding))
		b.absorbStandaloneCommentsGroup()
	} else {
		// No body-typed token for this doc - synthesize an implicit NullNode so every doc has a body child. A null
		// body carries any adjacent preceding comment left in b.groups (the group that would have attached to a
		// visible body node at this position).
		var line int
		if doc.Header != nil && doc.Header.Token != nil && doc.Header.Token.Position != nil {
			line = doc.Header.Token.Position.Line + 1
		}
		preceding := b.takeAdjacentGroup(line)
		n := &NullNode{}
		n.PrecedingComment = preceding
		doc.Children = append(doc.Children, n)
	}

	// End-of-doc handling. Groups still pending here split by adjacency to whatever follows:
	//   - end of stream: no target - drain everything as trailing EmptyNode children.
	//   - `...` footer next: non-adjacent groups become EmptyNode children in this doc; the adjacent group (if any)
	//     stays in b.groups for the footer to take as its PrecedingComment.
	//   - `---` of next doc: same split - non-adjacent → this doc's children, adjacent left in b.groups for the
	//     next buildDoc iteration to pick up as the next DocHeader's PrecedingComment.
	if t := b.peek(); t == nil {
		doc.Children = append(doc.Children, b.drainGroupsAsEmpties()...)
	} else {
		doc.Children = append(doc.Children, b.drainGroupsAdjacentTo(t.Position.Line)...)
	}
	if t := b.peek(); t != nil && t.Type == tokenizer.DocumentEndType {
		line := t.Position.Line
		preceding := b.takeAdjacentGroup(line)
		f := &DocFooter{Token: b.advance()}
		f.PrecedingComment = preceding
		doc.Footer = f
		doc.Footer.InlineComment = b.takeInlineOn(doc.Footer.Token.Position.Line)
	}
	if doc.Header == nil && doc.Footer == nil && len(doc.Children) == 0 {
		return nil
	}
	return doc
}

// buildBody consumes a single body node with the pre-computed PrecedingComment (the group that was source-adjacent
// to this line). Dispatches by looking at the first token, and for scalar keys peeks one ahead to spot the `:` that
// makes it a mapping. Panics on any token type not yet supported.
func (b *builder) buildBody(preceding *CommentNode) Node {
	t := b.peek()
	if t.Type == tokenizer.TagType {
		return b.buildTag(preceding)
	}
	if t.Type == tokenizer.AnchorType {
		return b.buildAnchor(preceding)
	}
	if t.Type == tokenizer.AliasType {
		return b.buildAlias(preceding)
	}
	if t.Type == tokenizer.SequenceEntryType {
		return b.buildBlockSequence(preceding)
	}
	if t.Type == tokenizer.SequenceStartType {
		return b.buildFlowSequence(preceding)
	}
	if t.Type == tokenizer.MappingStartType {
		return b.buildFlowMapping(preceding)
	}
	if b.isMappingKeyHere() {
		return b.buildBlockMapping(preceding)
	}
	return b.buildScalar(preceding)
}

// buildAnchor consumes `&name` and the value it labels. The `&` is a standalone token followed by a scalar name
// token; whatever comes next is the value being anchored (any body kind).
func (b *builder) buildAnchor(preceding *CommentNode) Node {
	amp := b.advance() // consume `&`
	if b.peek() == nil || !isScalarType(b.peek().Type) {
		panic(fmt.Sprintf("tree.Build: anchor `&` at line %d not followed by a name", amp.Position.Line))
	}
	name := b.advance()
	if b.peek() == nil {
		a := &AnchorNode{Amp: amp, Name: name, Value: &NullNode{}}
		a.PrecedingComment = preceding
		return a
	}
	value := b.buildBody(nil)
	a := &AnchorNode{Amp: amp, Name: name, Value: value}
	a.PrecedingComment = preceding
	return a
}

// buildTag consumes `!tag` and the value it tags. The tag is a standalone token followed by any body kind.
func (b *builder) buildTag(preceding *CommentNode) Node {
	tag := b.advance()
	if b.peek() == nil {
		t := &TagNode{Tag: tag, Value: &NullNode{}}
		t.PrecedingComment = preceding
		return t
	}
	value := b.buildBody(nil)
	t := &TagNode{Tag: tag, Value: value}
	t.PrecedingComment = preceding
	return t
}

// buildAlias consumes `*name` - a reference to a previously-defined anchor. Resolution is left to callers.
func (b *builder) buildAlias(preceding *CommentNode) Node {
	star := b.advance() // consume `*`
	if b.peek() == nil || !isScalarType(b.peek().Type) {
		panic(fmt.Sprintf("tree.Build: alias `*` at line %d not followed by a name", star.Position.Line))
	}
	name := b.advance()
	a := &AliasNode{Star: star, Name: name}
	a.PrecedingComment = preceding
	a.InlineComment = b.takeInlineOn(name.Position.Line)
	return a
}

// buildFlowSequence consumes a `[...]` flow sequence: `[` item (`,` item)* `]`. Items can be any body node
// (nested flow container, scalar, null). Commas separate items; trailing comma before `]` is allowed by YAML.
func (b *builder) buildFlowSequence(preceding *CommentNode) Node {
	open := b.advance() // consume `[`
	s := &SequenceNode{Style: StyleFlow, Open: open}
	s.PrecedingComment = preceding
	for {
		if b.peek() == nil {
			panic(fmt.Sprintf("tree.Build: unterminated flow sequence started at line %d", open.Position.Line))
		}
		if b.peek().Type == tokenizer.SequenceEndType {
			s.Close = b.advance()
			break
		}
		item := &SequenceItem{Value: b.buildBody(nil)}
		if b.peek() != nil && b.peek().Type == tokenizer.CollectEntryType {
			item.Trailer = b.advance()
		}
		s.Children = append(s.Children, item)
	}
	s.InlineComment = b.takeInlineOn(open.Position.Line)
	return s
}

// buildFlowMapping consumes a `{...}` flow mapping: `{` entry (`,` entry)* `}` where entry is `key: value`.
// Key can be any scalar; value can be any body node.
func (b *builder) buildFlowMapping(preceding *CommentNode) Node {
	open := b.advance() // consume `{`
	m := &MappingNode{Style: StyleFlow, Open: open}
	m.PrecedingComment = preceding
	for {
		if b.peek() == nil {
			panic(fmt.Sprintf("tree.Build: unterminated flow mapping started at line %d", open.Position.Line))
		}
		if b.peek().Type == tokenizer.MappingEndType {
			m.Close = b.advance()
			break
		}
		entry := &MappingEntry{}
		entry.Key = b.buildScalar(nil)
		if b.peek() == nil || b.peek().Type != tokenizer.MappingValueType {
			panic(fmt.Sprintf("tree.Build: expected ':' after flow mapping key at line %d", open.Position.Line))
		}
		entry.Colon = b.advance()
		if b.peek() == nil {
			panic(fmt.Sprintf("tree.Build: flow mapping key with no value at line %d", open.Position.Line))
		}
		entry.Value = b.buildBody(nil)
		if b.peek() != nil && b.peek().Type == tokenizer.CollectEntryType {
			entry.Trailer = b.advance()
		}
		m.Children = append(m.Children, entry)
	}
	m.InlineComment = b.takeInlineOn(open.Position.Line)
	return m
}

// isMappingKeyHere reports whether the current token starts a mapping entry. Two shapes:
//   - Implicit key: scalar directly followed by `:` (`key: value`).
//   - Explicit key: `?` marker followed by a scalar and `:` (`? key\n: value`).
func (b *builder) isMappingKeyHere() bool {
	if b.idx+1 >= len(b.tokens) {
		return false
	}
	if b.tokens[b.idx].Type == tokenizer.MappingKeyType {
		return true
	}
	if !isScalarType(b.tokens[b.idx].Type) {
		return false
	}
	return b.tokens[b.idx+1].Type == tokenizer.MappingValueType
}

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

// buildScalar consumes one scalar or null token as a leaf node with the given PrecedingComment. Dispatches to
// the typed scalar constructor based on token type: NullType/ImplicitNullType -> NullNode; block-scalar
// headers (`|`, `>`) -> StringNode with block style; the remaining scalar-typed tokens -> IntNode/FloatNode/
// BoolNode/StringNode as classified by the tokenizer.
func (b *builder) buildScalar(preceding *CommentNode) Node {
	t := b.peek()
	switch t.Type {
	case tokenizer.NullType, tokenizer.ImplicitNullType:
		tok := b.advance()
		n := &NullNode{Style: nullStyleFor(tok.Type, tok.Origin)}
		n.Token = tok
		n.PrecedingComment = preceding
		n.InlineComment = b.takeInlineOn(n.Token.Position.Line)
		return n
	case tokenizer.LiteralType, tokenizer.FoldedType:
		header := b.advance()
		style := StringLiteral
		if t.Type == tokenizer.FoldedType {
			style = StringFolded
		}
		s := &StringNode{Style: style, Header: header}
		s.PrecedingComment = preceding
		// A same-line comment after the header is inline on the header, not the body. The body (if any) sits on
		// subsequent lines.
		s.InlineComment = b.takeInlineOn(header.Position.Line)
		if next := b.peek(); next != nil && next.Type != tokenizer.CommentType && next.Position.Line != header.Position.Line {
			s.Token = b.advance()
		}
		return s
	case tokenizer.StringType, tokenizer.SingleQuoteType, tokenizer.DoubleQuoteType, tokenizer.MergeKeyType:
		s := &StringNode{Style: stringStyleFor(t.Type)}
		s.Token = b.advance()
		s.PrecedingComment = preceding
		s.InlineComment = b.takeInlineOn(s.Token.Position.Line)
		return s
	case tokenizer.IntegerType, tokenizer.BinaryIntegerType, tokenizer.OctetIntegerType, tokenizer.HexIntegerType:
		tok := b.advance()
		n := &IntNode{Style: intStyleFor(tok.Type)}
		n.Token = tok
		n.PrecedingComment = preceding
		n.InlineComment = b.takeInlineOn(n.Token.Position.Line)
		return n
	case tokenizer.FloatType, tokenizer.InfinityType, tokenizer.NanType:
		tok := b.advance()
		n := &FloatNode{Style: floatStyleFor(tok.Type)}
		n.Token = tok
		n.PrecedingComment = preceding
		n.InlineComment = b.takeInlineOn(n.Token.Position.Line)
		return n
	case tokenizer.BoolType:
		tok := b.advance()
		n := &BoolNode{Style: boolStyleFor(tok.Origin)}
		n.Token = tok
		n.PrecedingComment = preceding
		n.InlineComment = b.takeInlineOn(n.Token.Position.Line)
		return n
	}
	panic(fmt.Sprintf("tree.Build: unsupported token type %v at line %d (%q)", t.Type, t.Position.Line, t.Value))
}

// stringStyleFor maps a string-scalar token type to its StringStyle. Block scalar headers (`|`/`>`) are
// handled separately in buildScalar since they arrive as a header + body pair.
func stringStyleFor(t tokenizer.Type) StringStyle {
	switch t {
	case tokenizer.SingleQuoteType:
		return StringSingleQuoted
	case tokenizer.DoubleQuoteType:
		return StringDoubleQuoted
	}
	return StringPlain
}

// intStyleFor maps an integer-scalar token type to its IntStyle. Straight enum-to-enum since the tokenizer
// already distinguishes the four bases.
func intStyleFor(t tokenizer.Type) IntStyle {
	switch t {
	case tokenizer.HexIntegerType:
		return IntHex
	case tokenizer.OctetIntegerType:
		return IntOctal
	case tokenizer.BinaryIntegerType:
		return IntBinary
	}
	return IntDecimal
}

// floatStyleFor maps a float-scalar token type to its FloatStyle.
func floatStyleFor(t tokenizer.Type) FloatStyle {
	switch t {
	case tokenizer.InfinityType:
		return FloatInfinity
	case tokenizer.NanType:
		return FloatNaN
	}
	return FloatRegular
}

// boolStyleFor inspects the raw content of a bool token's Origin (stripped of leading/trailing whitespace) to
// pick the lexeme case. The tokenizer lumps all bool spellings under BoolType so we look at bytes.
func boolStyleFor(origin string) BoolStyle {
	switch scalarLexeme(origin) {
	case "True", "False":
		return BoolTitle
	case "TRUE", "FALSE":
		return BoolUpper
	}
	return BoolLower
}

// nullStyleFor picks the null lexeme's Style from the token type + Origin. NullImplicit is a distinct token
// type; the other four share NullType and differ only in spelling.
func nullStyleFor(t tokenizer.Type, origin string) NullStyle {
	if t == tokenizer.ImplicitNullType {
		return NullImplicit
	}
	switch scalarLexeme(origin) {
	case "~":
		return NullTilde
	case "Null":
		return NullTitle
	case "NULL":
		return NullUpper
	}
	return NullLower
}

// scalarLexeme strips leading whitespace (spaces/tabs) and takes the content up to the first whitespace
// character. Used by Style-detection helpers to isolate the "value" bytes of a token's Origin when the
// tokenizer's Value field doesn't preserve the source case.
func scalarLexeme(origin string) string {
	i := 0
	for i < len(origin) && (origin[i] == ' ' || origin[i] == '\t') {
		i++
	}
	body := origin[i:]
	j := 0
	for j < len(body) {
		c := body[j]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			break
		}
		j++
	}
	return body[:j]
}

// buildBlockMapping consumes a run of block-style `key: value` entries at the same indent column. Stops when the
// next token isn't a mapping key at this column (dedent, doc marker, EOF, or a token type that isn't a scalar
// followed by `:`). PrecedingComment goes on the outer MappingNode; each entry's own head/foot comments attach to
// the MappingEntry via the same adjacency rules used elsewhere. Any trailing comment groups at the mapping's tail
// become EmptyNode children of the mapping (their visual home is inside the container that just ended).
func (b *builder) buildBlockMapping(preceding *CommentNode) Node {
	m := &MappingNode{Style: StyleBlock}
	m.PrecedingComment = preceding
	baseCol := b.peek().Position.Column
	for {
		b.absorbStandaloneCommentsGroup()
		if b.peek() == nil {
			break
		}
		if !b.isMappingKeyHere() || b.peek().Position.Column != baseCol {
			break
		}
		keyLine := b.peek().Position.Line
		m.Children = append(m.Children, b.drainGroupsAdjacentTo(keyLine)...)
		entryPreceding := b.takeAdjacentGroup(keyLine)
		m.Children = append(m.Children, b.buildMappingEntry(entryPreceding))
	}
	m.Children = append(m.Children, b.trailingEmptiesAtContainerEnd()...)
	return m
}

// buildMappingEntry consumes a `key: value` pair. The value may be a scalar on the same line or a nested container
// on subsequent lines at a deeper indent. Handles both implicit (`key: value`) and explicit (`? key\n: value`)
// forms; the explicit `?` marker is preserved on the entry for round-trip.
func (b *builder) buildMappingEntry(preceding *CommentNode) *MappingEntry {
	entry := &MappingEntry{}
	entry.PrecedingComment = preceding
	if b.peek() != nil && b.peek().Type == tokenizer.MappingKeyType {
		entry.ExplicitKeyMarker = b.advance()
	}
	entry.Key = b.buildScalar(nil)
	// Consume the `:` separator.
	colon := b.peek()
	if colon == nil || colon.Type != tokenizer.MappingValueType {
		panic(fmt.Sprintf("tree.Build: expected ':' after mapping key at line %d", TokenOf(entry.Key).Position.Line))
	}
	keyLine := colon.Position.Line
	entry.Colon = b.advance()
	// Value: scalar or null on the same line, or a nested container on subsequent lines.
	t := b.peek()
	if t == nil {
		entry.Value = &NullNode{}
		return entry
	}
	if t.Position.Line == keyLine {
		if t.Type == tokenizer.CommentType {
			// `key:            # inline comment` - implicit null value, comment attaches inline.
			entry.Value = &NullNode{}
		} else {
			// Inline value: scalar, flow mapping `{...}`, or flow sequence `[...]` - dispatch through buildBody so
			// all three shapes work uniformly.
			entry.Value = b.buildBody(nil)
		}
	} else {
		// Nested container or scalar on next line.
		b.absorbStandaloneCommentsGroup()
		nested := b.peek()
		if nested == nil {
			entry.Value = &NullNode{}
			return entry
		}
		// Column check: a following token at the same or shallower column as our key is a SIBLING (or belongs
		// to an enclosing container), not a nested value. Common case: `a:\nb: 1` at column 1 - `b` is a
		// sibling of `a`, and `a`'s value is implicit null. Without this guard the builder greedily consumes
		// `b: 1` as `a`'s nested mapping value, mismatching goccy's parser and YAML semantics.
		if nested.Position.Column <= TokenOf(entry.Key).Position.Column {
			entry.Value = &NullNode{}
			return entry
		}
		// The nested value's preceding-comment adjacency check uses its own line.
		nestedLine := nested.Position.Line
		// Island groups between colon and nested value don't currently have a home - panic to surface it.
		if len(b.groups) > 0 {
			nonAdjacent := b.drainGroupsAdjacentTo(nestedLine)
			if len(nonAdjacent) > 0 {
				panic(fmt.Sprintf("tree.Build: island comment inside mapping entry value not yet supported (line %d)", nestedLine))
			}
		}
		valuePreceding := b.takeAdjacentGroup(nestedLine)
		entry.Value = b.buildBody(valuePreceding)
	}
	// Inline comment on the entry's line.
	entry.InlineComment = b.takeInlineOn(keyLine)
	return entry
}

// buildBlockSequence consumes a run of block-style `- item` entries at the same indent column. Same stop rules as
// buildBlockMapping: dedent, doc marker, non-sequence-entry-token, or EOF. Trailing groups become EmptyNode
// children of this sequence.
func (b *builder) buildBlockSequence(preceding *CommentNode) Node {
	s := &SequenceNode{Style: StyleBlock}
	s.PrecedingComment = preceding
	baseCol := b.peek().Position.Column
	defer func() {
		s.Children = append(s.Children, b.trailingEmptiesAtContainerEnd()...)
	}()
	for {
		b.absorbStandaloneCommentsGroup()
		if b.peek() == nil {
			break
		}
		if b.peek().Type != tokenizer.SequenceEntryType || b.peek().Position.Column != baseCol {
			break
		}
		itemMarkerLine := b.peek().Position.Line
		s.Children = append(s.Children, b.drainGroupsAdjacentTo(itemMarkerLine)...)
		itemPreceding := b.takeAdjacentGroup(itemMarkerLine)
		marker := b.advance() // consume `-`
		item := &SequenceItem{Marker: marker}
		item.PrecedingComment = itemPreceding
		t := b.peek()
		switch {
		case t == nil:
			item.Value = &NullNode{}
		case t.Position.Line == itemMarkerLine:
			// Inline item.
			item.Value = b.buildBody(nil)
		default:
			// Nested content on next line.
			b.absorbStandaloneCommentsGroup()
			nested := b.peek()
			if nested == nil {
				item.Value = &NullNode{}
			} else {
				if len(b.groups) > 0 {
					b.drainGroupsAdjacentTo(nested.Position.Line)
				}
				item.Value = b.buildBody(b.takeAdjacentGroup(nested.Position.Line))
			}
		}
		item.InlineComment = b.takeInlineOn(itemMarkerLine)
		s.Children = append(s.Children, item)
	}
	return s
}

// reads own-line comment tokens into b.groups. Contiguous lines join the current open group
func (b *builder) absorbStandaloneCommentsGroup() {
	for {
		t := b.peek()
		if t == nil || t.Type != tokenizer.CommentType {
			return
		}
		b.advance()
		if last := b.lastGroup(); last != nil && b.lineOfLastComment(last)+1 == t.Position.Line {
			last.Lines = append(last.Lines, t)
			continue
		}
		b.groups = append(b.groups, &CommentNode{Lines: []*tokenizer.Token{t}})
	}
}

// trailingEmptiesAtContainerEnd returns the pending comment groups that belong to the just-closed container: all
// of them if there's no more content or a doc marker follows, otherwise only the non-adjacent groups (the adjacent
// group is left in b.groups for the outer container's next node to claim as its PrecedingComment).
func (b *builder) trailingEmptiesAtContainerEnd() []Node {
	next := b.peek()
	if next == nil || next.Type == tokenizer.DocumentEndType || next.Type == tokenizer.DocumentHeaderType {
		return b.drainGroupsAsEmpties()
	}
	return b.drainGroupsAdjacentTo(next.Position.Line)
}

// drainGroupsAdjacentTo returns pending comment groups that are NOT source-adjacent to nodeLine, each wrapped as
// an EmptyNode, in source order. These groups are floating islands - they render at their own source position with
// no attachment to the following structural node. The (possibly-nil) adjacent group is left in b.groups for
// takeAdjacentGroup to claim.
func (b *builder) drainGroupsAdjacentTo(nodeLine int) []Node {
	if len(b.groups) == 0 {
		return nil
	}
	// Find the split point: the last group is "adjacent" if its last line + 1 == nodeLine; anything before that is
	// definitely non-adjacent (island). If the last group is not adjacent, everything is island.
	split := len(b.groups)
	if b.lineOfLastComment(b.groups[len(b.groups)-1])+1 == nodeLine {
		split = len(b.groups) - 1
	}
	empties := make([]Node, 0, split)
	for _, g := range b.groups[:split] {
		e := &EmptyNode{}
		e.PrecedingComment = g
		empties = append(empties, e)
	}
	b.groups = b.groups[split:]
	return empties
}

// takeAdjacentGroup returns the last pending group if it's source-adjacent to nodeLine (attach as this node's
// PrecedingComment), else nil. Must be called AFTER drainGroupsAdjacentTo - by then the only group left is either
// the adjacent one or nothing.
func (b *builder) takeAdjacentGroup(nodeLine int) *CommentNode {
	if len(b.groups) == 0 {
		return nil
	}
	last := b.groups[len(b.groups)-1]
	if b.lineOfLastComment(last)+1 != nodeLine {
		return nil
	}
	b.groups = b.groups[:len(b.groups)-1]
	return last
}

// drainGroupsAsEmpties flushes every pending group as its own EmptyNode, in source order. Used at end-of-doc where
// there's no next structural target to attach forward to.
func (b *builder) drainGroupsAsEmpties() []Node {
	if len(b.groups) == 0 {
		return nil
	}
	empties := make([]Node, 0, len(b.groups))
	for _, g := range b.groups {
		e := &EmptyNode{}
		e.PrecedingComment = g
		empties = append(empties, e)
	}
	b.groups = b.groups[:0]
	return empties
}

// takeInlineOn consumes the next token if it's a same-line inline comment and returns it as a single-line
// CommentNode. Otherwise returns nil.
func (b *builder) takeInlineOn(line int) *CommentNode {
	t := b.peek()
	if t == nil || t.Type != tokenizer.CommentType || t.Position.Line != line {
		return nil
	}
	return &CommentNode{Lines: []*tokenizer.Token{b.advance()}}
}

func (b *builder) lastGroup() *CommentNode {
	if len(b.groups) == 0 {
		return nil
	}
	return b.groups[len(b.groups)-1]
}

func (b *builder) lineOfLastComment(g *CommentNode) int {
	return g.Lines[len(g.Lines)-1].Position.Line
}
