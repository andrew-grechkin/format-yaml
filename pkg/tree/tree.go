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
	"strings"

	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/token"
)

// Node is the common interface every tree node satisfies. Data returns the semantic content of this node - for a
// scalar leaf it's the parsed value; for structural nodes it's the recursive Go representation (map/slice/nil).
// Access to source position, comments, and style stays on the concrete types.
type Node interface {
	Data() any
	ToString() string
}

// Build parses src via goccy's lexer and constructs our tree. Panics if the token stream carries any token type we
// don't yet handle - deliberate, so unsupported inputs surface immediately during development instead of getting a
// silently-dropped subtree.
func Build(src []byte) *File {
	tokens := lexer.Tokenize(string(src))
	alignOrigins(tokens, src)
	b := &builder{tokens: tokens}
	return b.buildFile()
}

// alignOrigins normalizes each token's Origin so that concatenating all Origins in order equals the source (up
// to any goccy-dropped trailing bytes and orphan whitespace goccy discards between tokens). Goccy's lexer
// sometimes double-encodes source bytes in adjacent tokens (a tag `!\n` and the following scalar `\n0000` both
// claim the same `\n`); it also sometimes drops leading whitespace that isn't part of any token. We walk left-
// to-right with a running position: for each token we find where its Origin appears in source at or after pos,
// stripping the Origin's leading chars if needed. Orphan bytes between tokens (unmatched by any Origin) get
// absorbed silently - the tree emits only what tokens carry.
func alignOrigins(tokens token.Tokens, src []byte) {
	srcStr := string(src)
	pos := 0
	for i, t := range tokens {
		origin := t.Origin
		bestPos := -1
		for len(origin) > 0 {
			// Search from pos forward for origin's first match.
			idx := strings.Index(srcStr[pos:], origin)
			if idx >= 0 {
				bestPos = pos + idx
				break
			}
			// Not found - strip a leading char and retry (goccy may have overlapped this Origin with prior).
			origin = origin[1:]
		}
		if bestPos < 0 {
			tokens[i].Origin = ""
			continue
		}
		tokens[i].Origin = origin
		pos = bestPos + len(origin)
	}
}

// File is a YAML stream: an ordered list of children. Children can be Docs or EmptyNodes (island comments that sit
// between docs, before the first doc, or after the last doc - stream-level comments that don't belong to any
// particular document). Data only counts Docs; EmptyNodes are not documents.
type File struct {
	Children []Node
}

func (f *File) Data() any {
	out := make([]any, 0)
	for _, c := range f.Children {
		if _, ok := c.(*EmptyNode); ok {
			continue
		}
		out = append(out, c.Data())
	}
	return out
}

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

// DocHeader represents the `---` marker at the start of a document. PrecedingComment holds any comment group that
// sat above the marker in source; InlineComment holds a same-line trailing comment (`--- # note`).
type DocHeader struct {
	Token            *token.Token
	PrecedingComment *CommentNode
	InlineComment    *CommentNode
}

func (*DocHeader) Data() any { return nil }

// DocFooter represents the `...` marker at the end of a document. Same comment-slot semantics as DocHeader.
type DocFooter struct {
	Token            *token.Token
	PrecedingComment *CommentNode
	InlineComment    *CommentNode
}

func (*DocFooter) Data() any { return nil }

// EmptyNode is a positional slot with no body content - a place in the tree that carries a floating comment. Used
// when a comment is blank-line-separated from any subsequent content, or when a comment sits at the tail of a
// container with nothing to attach forward to. The node itself renders nothing; PrecedingComment carries the text
// on emit.
type EmptyNode struct {
	PrecedingComment *CommentNode
}

func (*EmptyNode) Data() any { return nil }

// ScalarStyle is a rendering choice for a scalar leaf. Same semantic content across all styles; the emitter picks
// bytes based on the flag.
type ScalarStyle int

const (
	ScalarPlain        ScalarStyle = iota // bare: `hello`
	ScalarSingleQuoted                    // 'hello'
	ScalarDoubleQuoted                    // "hello"
	ScalarLiteral                         // |, |+, |-
	ScalarFolded                          // >, >+, >-
)

// ScalarNode is a leaf value: string, integer, float, bool, infinity, nan, or block-scalar text. Null uses NullNode
// instead so null-vs-missing is a type distinction, not a value check. Header is the block-scalar indicator token
// (`|`, `>`, or their chomp variants) - nil for plain/quoted styles, populated for ScalarLiteral/ScalarFolded so
// the chomp modifier and its source position round-trip.
type ScalarNode struct {
	Style            ScalarStyle
	Token            *token.Token
	Header           *token.Token
	PrecedingComment *CommentNode
	InlineComment    *CommentNode
}

func (s *ScalarNode) Data() any {
	if s.Token == nil {
		return nil
	}
	return s.Token.Value
}

// NullNode represents an explicit or implicit YAML null. Distinct type so callers can distinguish "explicit null in
// source" from a Go zero-value.
type NullNode struct {
	Token            *token.Token
	PrecedingComment *CommentNode
	InlineComment    *CommentNode
}

func (*NullNode) Data() any { return nil }

// Style is a rendering choice for container nodes: block layout (indented, one entry per line) or flow layout
// (inline, comma-separated). Same semantic content either way; the emitter picks bytes based on the flag.
type Style int

const (
	StyleBlock Style = iota
	StyleFlow
)

// MappingNode is a YAML mapping (key-value collection). Children hold MappingEntry values (the entries themselves)
// and EmptyNodes for any island comments that sit between entries. Open/Close carry the `{`/`}` tokens for flow
// style (nil for block); we keep the tokens rather than emitting literals because their Origins carry the
// leading whitespace between the container and whatever precedes it.
type MappingNode struct {
	Style            Style
	Open             *token.Token
	Children         []Node
	Close            *token.Token
	PrecedingComment *CommentNode
	InlineComment    *CommentNode
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

// MappingEntry is a single key/value pair inside a MappingNode. ExplicitKeyMarker is populated when the entry
// used the `? key\n: value` form; nil for the common `key: value` shape. Comma carries the `,` token that
// followed this entry inside a flow mapping (nil for block or the last flow entry). Preserving these lets the
// emitter round-trip the source exactly, including the whitespace those tokens absorbed.
type MappingEntry struct {
	ExplicitKeyMarker *token.Token
	Key               Node
	Value             Node
	Comma             *token.Token
	PrecedingComment  *CommentNode
	InlineComment     *CommentNode
}

func (e *MappingEntry) Data() any {
	if e.Value == nil {
		return nil
	}
	return e.Value.Data()
}

// SequenceNode is a YAML sequence (ordered list). Children hold SequenceItem values plus any EmptyNodes for island
// comments between items. Open/Close carry the `[`/`]` tokens for flow style; nil for block.
type SequenceNode struct {
	Style            Style
	Open             *token.Token
	Children         []Node
	Close            *token.Token
	PrecedingComment *CommentNode
	InlineComment    *CommentNode
}

func (s *SequenceNode) Data() any {
	out := make([]any, 0, len(s.Children))
	for _, c := range s.Children {
		if _, ok := c.(*EmptyNode); ok {
			continue
		}
		out = append(out, c.Data())
	}
	return out
}

// SequenceItem wraps a single sequence entry so its `-` marker (block style) can be preserved verbatim, along with
// its own comment slots. Flow-style items still use SequenceItem for uniformity - Marker is nil there. Comma
// carries the `,` following this item in a flow sequence.
type SequenceItem struct {
	Marker           *token.Token
	Value            Node
	Comma            *token.Token
	PrecedingComment *CommentNode
	InlineComment    *CommentNode
}

func (i *SequenceItem) Data() any {
	if i.Value == nil {
		return nil
	}
	return i.Value.Data()
}

// TagNode wraps a value with a YAML tag (`!tag`, `!!str`, `!<uri>`). Tag is the tag indicator token; Value is the
// tagged node. Data() returns the underlying Value's Data (tagging is a type-annotation, not its own semantic
// content). Can coexist with AnchorNode by nesting (tag outside anchor or vice versa - both are wrappers).
type TagNode struct {
	Tag              *token.Token
	Value            Node
	PrecedingComment *CommentNode
	InlineComment    *CommentNode
}

func (t *TagNode) Data() any {
	if t.Value == nil {
		return nil
	}
	return t.Value.Data()
}

// DirectiveNode is a stream-level `%NAME args` directive (`%YAML 1.2`, `%TAG !e! tag:...`). Lives in File.Children,
// not inside any Doc. Data() returns nil - directives affect parsing, not semantic content.
type DirectiveNode struct {
	Name *token.Token
	Args []*token.Token
}

func (*DirectiveNode) Data() any { return nil }

// AnchorNode wraps another node with an anchor name (`&name`). The Value field is the actual anchored content,
// which can be any node type. Amp holds the `&` token so its Origin (with any leading whitespace) round-trips
// verbatim. Data() returns the Value's Data - the anchor is a labeling wrapper, not its own semantic content.
type AnchorNode struct {
	Amp              *token.Token
	Name             *token.Token
	Value            Node
	PrecedingComment *CommentNode
	InlineComment    *CommentNode
}

func (a *AnchorNode) Data() any {
	if a.Value == nil {
		return nil
	}
	return a.Value.Data()
}

// AliasNode is a reference to a previously-defined anchor (`*name`). Resolution to the referenced value is a
// downstream concern (the tree itself doesn't chase aliases); Data() returns nil so callers who need the resolved
// value do their own lookup via the anchor name. Star holds the `*` token so its Origin round-trips.
type AliasNode struct {
	Star             *token.Token
	Name             *token.Token
	PrecedingComment *CommentNode
	InlineComment    *CommentNode
}

func (*AliasNode) Data() any { return nil }

// CommentNode is one comment group - a run of contiguous comment lines (no blank lines between them) that share a
// single attribution target. Lines holds one token per source line, in order. A blank-line-separated set of comment
// lines is TWO CommentNodes attached to different targets, not one CommentNode with a gap inside it.
type CommentNode struct {
	Lines []*token.Token
}

func (*CommentNode) Data() any { return nil }

// builder walks the token stream sequentially. groups holds the not-yet-attached comment groups in source order.
// The last group is "still open" - subsequent contiguous comment lines extend it; a non-comment or a line gap
// closes it and starts a new one on the next comment.
type builder struct {
	tokens token.Tokens
	idx    int
	groups []*CommentNode
}

func (b *builder) peek() *token.Token {
	if b.idx >= len(b.tokens) {
		return nil
	}
	return b.tokens[b.idx]
}

func (b *builder) advance() *token.Token {
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
		b.absorbComments()
		if b.peek() == nil {
			break
		}
		if b.peek().Type == token.DirectiveType {
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
	if b.peek().Type == token.DocumentHeaderType {
		line := b.peek().Position.Line
		preceding := b.takeAdjacentGroup(line)
		doc.Header = &DocHeader{Token: b.advance(), PrecedingComment: preceding}
		doc.Header.InlineComment = b.takeInlineOn(doc.Header.Token.Position.Line)
		b.absorbComments()
	}
	if t := b.peek(); t != nil && t.Type != token.DocumentEndType && t.Type != token.DocumentHeaderType {
		line := t.Position.Line
		doc.Children = append(doc.Children, b.drainGroupsAdjacentTo(line)...)
		preceding := b.takeAdjacentGroup(line)
		doc.Children = append(doc.Children, b.buildBody(preceding))
		b.absorbComments()
	} else {
		// No body-typed token for this doc - synthesize an implicit NullNode so every doc has a body child. A null
		// body carries any adjacent preceding comment left in b.groups (the group that would have attached to a
		// visible body node at this position).
		var line int
		if doc.Header != nil && doc.Header.Token != nil && doc.Header.Token.Position != nil {
			line = doc.Header.Token.Position.Line + 1
		}
		preceding := b.takeAdjacentGroup(line)
		doc.Children = append(doc.Children, &NullNode{PrecedingComment: preceding})
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
	if t := b.peek(); t != nil && t.Type == token.DocumentEndType {
		line := t.Position.Line
		preceding := b.takeAdjacentGroup(line)
		doc.Footer = &DocFooter{Token: b.advance(), PrecedingComment: preceding}
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
	if t.Type == token.TagType {
		return b.buildTag(preceding)
	}
	if t.Type == token.AnchorType {
		return b.buildAnchor(preceding)
	}
	if t.Type == token.AliasType {
		return b.buildAlias(preceding)
	}
	if t.Type == token.SequenceEntryType {
		return b.buildBlockSequence(preceding)
	}
	if t.Type == token.SequenceStartType {
		return b.buildFlowSequence(preceding)
	}
	if t.Type == token.MappingStartType {
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
		return &AnchorNode{Amp: amp, Name: name, Value: &NullNode{}, PrecedingComment: preceding}
	}
	value := b.buildBody(nil)
	return &AnchorNode{Amp: amp, Name: name, Value: value, PrecedingComment: preceding}
}

// buildTag consumes `!tag` and the value it tags. The tag is a standalone token followed by any body kind.
func (b *builder) buildTag(preceding *CommentNode) Node {
	tag := b.advance()
	if b.peek() == nil {
		return &TagNode{Tag: tag, Value: &NullNode{}, PrecedingComment: preceding}
	}
	value := b.buildBody(nil)
	return &TagNode{Tag: tag, Value: value, PrecedingComment: preceding}
}

// buildAlias consumes `*name` - a reference to a previously-defined anchor. Resolution is left to callers.
func (b *builder) buildAlias(preceding *CommentNode) Node {
	star := b.advance() // consume `*`
	if b.peek() == nil || !isScalarType(b.peek().Type) {
		panic(fmt.Sprintf("tree.Build: alias `*` at line %d not followed by a name", star.Position.Line))
	}
	name := b.advance()
	a := &AliasNode{Star: star, Name: name, PrecedingComment: preceding}
	a.InlineComment = b.takeInlineOn(name.Position.Line)
	return a
}

// buildFlowSequence consumes a `[...]` flow sequence: `[` item (`,` item)* `]`. Items can be any body node
// (nested flow container, scalar, null). Commas separate items; trailing comma before `]` is allowed by YAML.
func (b *builder) buildFlowSequence(preceding *CommentNode) Node {
	open := b.advance() // consume `[`
	s := &SequenceNode{Style: StyleFlow, Open: open, PrecedingComment: preceding}
	for {
		if b.peek() == nil {
			panic(fmt.Sprintf("tree.Build: unterminated flow sequence started at line %d", open.Position.Line))
		}
		if b.peek().Type == token.SequenceEndType {
			s.Close = b.advance()
			break
		}
		item := &SequenceItem{Value: b.buildBody(nil)}
		if b.peek() != nil && b.peek().Type == token.CollectEntryType {
			item.Comma = b.advance()
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
	m := &MappingNode{Style: StyleFlow, Open: open, PrecedingComment: preceding}
	for {
		if b.peek() == nil {
			panic(fmt.Sprintf("tree.Build: unterminated flow mapping started at line %d", open.Position.Line))
		}
		if b.peek().Type == token.MappingEndType {
			m.Close = b.advance()
			break
		}
		entry := &MappingEntry{}
		entry.Key = b.buildScalar(nil)
		if b.peek() == nil || b.peek().Type != token.MappingValueType {
			panic(fmt.Sprintf("tree.Build: expected ':' after flow mapping key at line %d", open.Position.Line))
		}
		b.advance() // consume `:`
		if b.peek() == nil {
			panic(fmt.Sprintf("tree.Build: flow mapping key with no value at line %d", open.Position.Line))
		}
		entry.Value = b.buildBody(nil)
		if b.peek() != nil && b.peek().Type == token.CollectEntryType {
			entry.Comma = b.advance()
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
	if b.tokens[b.idx].Type == token.MappingKeyType {
		return true
	}
	if !isScalarType(b.tokens[b.idx].Type) {
		return false
	}
	return b.tokens[b.idx+1].Type == token.MappingValueType
}

func isScalarType(t token.Type) bool {
	switch t {
	case token.NullType, token.ImplicitNullType,
		token.StringType, token.SingleQuoteType, token.DoubleQuoteType,
		token.BoolType,
		token.IntegerType, token.BinaryIntegerType, token.OctetIntegerType, token.HexIntegerType,
		token.FloatType, token.InfinityType, token.NanType,
		token.MergeKeyType:
		return true
	}
	return false
}

// buildScalar consumes one scalar or null token as a leaf node with the given PrecedingComment. Also handles block
// scalars (`|` and `>` variants), which arrive as two consecutive tokens (header + body); the pair becomes a single
// BlockScalarNode.
func (b *builder) buildScalar(preceding *CommentNode) Node {
	t := b.peek()
	switch t.Type {
	case token.NullType, token.ImplicitNullType:
		n := &NullNode{Token: b.advance(), PrecedingComment: preceding}
		n.InlineComment = b.takeInlineOn(n.Token.Position.Line)
		return n
	case token.LiteralType, token.FoldedType:
		header := b.advance()
		style := ScalarLiteral
		if t.Type == token.FoldedType {
			style = ScalarFolded
		}
		s := &ScalarNode{Style: style, Header: header, PrecedingComment: preceding}
		// A same-line comment after the header is inline on the header, not the body. The body (if any) sits on
		// subsequent lines.
		s.InlineComment = b.takeInlineOn(header.Position.Line)
		if next := b.peek(); next != nil && next.Type != token.CommentType && next.Position.Line != header.Position.Line {
			s.Token = b.advance()
		}
		return s
	case token.StringType, token.SingleQuoteType, token.DoubleQuoteType,
		token.BoolType,
		token.IntegerType, token.BinaryIntegerType, token.OctetIntegerType, token.HexIntegerType,
		token.FloatType, token.InfinityType, token.NanType,
		token.MergeKeyType:
		s := &ScalarNode{Style: scalarStyleFor(t.Type), Token: b.advance(), PrecedingComment: preceding}
		s.InlineComment = b.takeInlineOn(s.Token.Position.Line)
		return s
	}
	panic(fmt.Sprintf("tree.Build: unsupported token type %v at line %d (%q)", t.Type, t.Position.Line, t.Value))
}

// scalarStyleFor maps a scalar token type to its rendering style. Block scalar headers are handled separately in
// buildScalar since they arrive as a header + body pair, not a single scalar token.
func scalarStyleFor(t token.Type) ScalarStyle {
	switch t {
	case token.SingleQuoteType:
		return ScalarSingleQuoted
	case token.DoubleQuoteType:
		return ScalarDoubleQuoted
	}
	return ScalarPlain
}

// buildBlockMapping consumes a run of block-style `key: value` entries at the same indent column. Stops when the
// next token isn't a mapping key at this column (dedent, doc marker, EOF, or a token type that isn't a scalar
// followed by `:`). PrecedingComment goes on the outer MappingNode; each entry's own head/foot comments attach to
// the MappingEntry via the same adjacency rules used elsewhere. Any trailing comment groups at the mapping's tail
// become EmptyNode children of the mapping (their visual home is inside the container that just ended).
func (b *builder) buildBlockMapping(preceding *CommentNode) Node {
	m := &MappingNode{Style: StyleBlock, PrecedingComment: preceding}
	baseCol := b.peek().Position.Column
	for {
		b.absorbComments()
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
	entry := &MappingEntry{PrecedingComment: preceding}
	if b.peek() != nil && b.peek().Type == token.MappingKeyType {
		entry.ExplicitKeyMarker = b.advance()
	}
	entry.Key = b.buildScalar(nil)
	// Consume the `:` separator.
	colon := b.peek()
	if colon == nil || colon.Type != token.MappingValueType {
		panic(fmt.Sprintf("tree.Build: expected ':' after mapping key at line %d", entry.Key.(*ScalarNode).Token.Position.Line))
	}
	keyLine := colon.Position.Line
	b.advance()
	// Value: scalar or null on the same line, or a nested container on subsequent lines.
	t := b.peek()
	if t == nil {
		entry.Value = &NullNode{}
		return entry
	}
	if t.Position.Line == keyLine {
		if t.Type == token.CommentType {
			// `key:            # inline comment` - implicit null value, comment attaches inline.
			entry.Value = &NullNode{}
		} else {
			// Inline value: scalar, flow mapping `{...}`, or flow sequence `[...]` - dispatch through buildBody so
			// all three shapes work uniformly.
			entry.Value = b.buildBody(nil)
		}
	} else {
		// Nested container or scalar on next line.
		b.absorbComments()
		nested := b.peek()
		if nested == nil {
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
	s := &SequenceNode{Style: StyleBlock, PrecedingComment: preceding}
	baseCol := b.peek().Position.Column
	defer func() {
		s.Children = append(s.Children, b.trailingEmptiesAtContainerEnd()...)
	}()
	for {
		b.absorbComments()
		if b.peek() == nil {
			break
		}
		if b.peek().Type != token.SequenceEntryType || b.peek().Position.Column != baseCol {
			break
		}
		itemMarkerLine := b.peek().Position.Line
		s.Children = append(s.Children, b.drainGroupsAdjacentTo(itemMarkerLine)...)
		itemPreceding := b.takeAdjacentGroup(itemMarkerLine)
		marker := b.advance() // consume `-`
		item := &SequenceItem{Marker: marker, PrecedingComment: itemPreceding}
		t := b.peek()
		switch {
		case t == nil:
			item.Value = &NullNode{}
		case t.Position.Line == itemMarkerLine:
			// Inline item.
			item.Value = b.buildBody(nil)
		default:
			// Nested content on next line.
			b.absorbComments()
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

// absorbComments reads own-line comment tokens into b.groups. Contiguous lines (source-line gap of 1) join the
// current open group; a larger gap closes the current group and starts a new one.
func (b *builder) absorbComments() {
	for {
		t := b.peek()
		if t == nil || t.Type != token.CommentType {
			return
		}
		b.advance()
		if last := b.lastGroup(); last != nil && b.lineOfLastComment(last)+1 == t.Position.Line {
			last.Lines = append(last.Lines, t)
			continue
		}
		b.groups = append(b.groups, &CommentNode{Lines: []*token.Token{t}})
	}
}

// trailingEmptiesAtContainerEnd returns the pending comment groups that belong to the just-closed container: all
// of them if there's no more content or a doc marker follows, otherwise only the non-adjacent groups (the adjacent
// group is left in b.groups for the outer container's next node to claim as its PrecedingComment).
func (b *builder) trailingEmptiesAtContainerEnd() []Node {
	next := b.peek()
	if next == nil || next.Type == token.DocumentEndType || next.Type == token.DocumentHeaderType {
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
		empties = append(empties, &EmptyNode{PrecedingComment: g})
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
		empties = append(empties, &EmptyNode{PrecedingComment: g})
	}
	b.groups = b.groups[:0]
	return empties
}

// takeInlineOn consumes the next token if it's a same-line inline comment and returns it as a single-line
// CommentNode. Otherwise returns nil.
func (b *builder) takeInlineOn(line int) *CommentNode {
	t := b.peek()
	if t == nil || t.Type != token.CommentType || t.Position.Line != line {
		return nil
	}
	return &CommentNode{Lines: []*token.Token{b.advance()}}
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
