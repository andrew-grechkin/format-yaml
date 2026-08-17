// AppendNewline implementations for every Node concrete type. Each type knows where its rendered tail is
// (its own last token, its inline comment if any, its last child for containers) and appends a `\n` there.
// Callers use AppendNewline plus TrailingNewlines (in tree.go) to build higher-level operations like "ensure
// this node's tail has N blank-line-separating newlines" without ever needing to inspect internal tokens.
package tree

// appendToInlineTail appends `\n` to the last line of c and reports whether c had any lines to append to.
// Shared helper for every type that has an inline comment slot - inline comment renders LAST when present,
// so it owns the trailing whitespace.
func appendToInlineTail(c *CommentNode) bool {
	if c == nil || len(c.Lines) == 0 {
		return false
	}
	c.Lines[len(c.Lines)-1].Origin += "\n"
	return true
}

// ScalarBase covers StringNode/IntNode/FloatNode/BoolNode/NullNode via embedding.
func (b *ScalarBase) AppendNewline() {
	if appendToInlineTail(b.InlineComment) {
		return
	}
	if b.Token != nil {
		b.Token.Origin += "\n"
	}
}

func (f *File) AppendNewline() {
	if len(f.Children) == 0 {
		return
	}
	f.Children[len(f.Children)-1].AppendNewline()
}

func (d *Doc) AppendNewline() {
	if d.Footer != nil {
		d.Footer.AppendNewline()
		return
	}
	if len(d.Children) > 0 {
		d.Children[len(d.Children)-1].AppendNewline()
		return
	}
	if d.Header != nil {
		d.Header.AppendNewline()
	}
}

func (d *DocHeader) AppendNewline() {
	if appendToInlineTail(d.InlineComment) {
		return
	}
	if d.Token != nil {
		d.Token.Origin += "\n"
	}
}

func (d *DocFooter) AppendNewline() {
	if appendToInlineTail(d.InlineComment) {
		return
	}
	if d.Token != nil {
		d.Token.Origin += "\n"
	}
}

func (e *EmptyNode) AppendNewline() {
	if e.PrecedingComment != nil && len(e.PrecedingComment.Lines) > 0 {
		last := e.PrecedingComment.Lines[len(e.PrecedingComment.Lines)-1]
		last.Origin += "\n"
	}
}

func (m *MappingNode) AppendNewline() {
	if appendToInlineTail(m.InlineComment) {
		return
	}
	if m.Close != nil {
		m.Close.Origin += "\n"
		return
	}
	if len(m.Children) > 0 {
		m.Children[len(m.Children)-1].AppendNewline()
	}
}

func (e *MappingEntry) AppendNewline() {
	if appendToInlineTail(e.InlineComment) {
		return
	}
	if e.Trailer != nil {
		e.Trailer.Origin += "\n"
		return
	}
	if e.Value != nil {
		e.Value.AppendNewline()
		return
	}
	if e.Colon != nil {
		e.Colon.Origin += "\n"
	}
}

func (s *SequenceNode) AppendNewline() {
	if appendToInlineTail(s.InlineComment) {
		return
	}
	if s.Close != nil {
		s.Close.Origin += "\n"
		return
	}
	if len(s.Children) > 0 {
		s.Children[len(s.Children)-1].AppendNewline()
	}
}

func (i *SequenceItem) AppendNewline() {
	if appendToInlineTail(i.InlineComment) {
		return
	}
	if i.Trailer != nil {
		i.Trailer.Origin += "\n"
		return
	}
	if i.Value != nil {
		i.Value.AppendNewline()
		return
	}
	if i.Marker != nil {
		i.Marker.Origin += "\n"
	}
}

func (a *AnchorNode) AppendNewline() {
	if appendToInlineTail(a.InlineComment) {
		return
	}
	if a.Value != nil {
		a.Value.AppendNewline()
		return
	}
	if a.Name != nil {
		a.Name.Origin += "\n"
	}
}

func (t *TagNode) AppendNewline() {
	if appendToInlineTail(t.InlineComment) {
		return
	}
	if t.Value != nil {
		t.Value.AppendNewline()
		return
	}
	if t.Tag != nil {
		t.Tag.Origin += "\n"
	}
}

func (a *AliasNode) AppendNewline() {
	if appendToInlineTail(a.InlineComment) {
		return
	}
	if a.Name != nil {
		a.Name.Origin += "\n"
	}
}

func (c *CommentNode) AppendNewline() {
	if len(c.Lines) == 0 {
		return
	}
	c.Lines[len(c.Lines)-1].Origin += "\n"
}

func (d *DirectiveNode) AppendNewline() {
	if len(d.Args) > 0 {
		d.Args[len(d.Args)-1].Origin += "\n"
		return
	}
	if d.Name != nil {
		d.Name.Origin += "\n"
	}
}
