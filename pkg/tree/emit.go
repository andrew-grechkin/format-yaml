// Emit: every node implements ToString() so file.ToString() reconstructs bytes by simple recursive concatenation.
// No central dispatch, no separate emit logic - each node type owns its rendering and calls into its children.
// Round-trip identity: as long as every node writes back what it captured (Token.Origin for lexer tokens, literal
// markers for structural characters), concat produces byte-identical source.
package tree

import (
	"strings"

	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// Emit renders any node as a complete-output byte slice: n.ToString() with a guaranteed trailing `\n` if the
// output isn't empty and doesn't already end with one. Used for the final write-to-file step. Trailing content
// (including significant multi-newline output from `|+` block scalars) is preserved verbatim - we only ADD a
// terminator when goccy's tokenizer dropped one, never strip.
func Emit(n Node) []byte {
	out := n.ToString()
	if out == "" {
		return nil
	}
	if out[len(out)-1] != '\n' {
		out += "\n"
	}
	return []byte(out)
}

func (f *File) ToString() string {
	var sb strings.Builder
	for _, c := range f.Children {
		sb.WriteString(c.ToString())
	}
	return sb.String()
}

func (d *Doc) ToString() string {
	var sb strings.Builder
	if d.Header != nil {
		sb.WriteString(d.Header.ToString())
	}
	for _, c := range d.Children {
		sb.WriteString(c.ToString())
	}
	if d.Footer != nil {
		sb.WriteString(d.Footer.ToString())
	}
	return sb.String()
}

func (h *DocHeader) ToString() string {
	return commentPrefix(h.PrecedingComment) + h.Token.Origin + commentSuffix(h.InlineComment)
}

func (f *DocFooter) ToString() string {
	return commentPrefix(f.PrecedingComment) + f.Token.Origin + commentSuffix(f.InlineComment)
}

func (d *DirectiveNode) ToString() string {
	var sb strings.Builder
	sb.WriteString("%")
	sb.WriteString(d.Name.Origin)
	for _, a := range d.Args {
		sb.WriteString(a.Origin)
	}
	return sb.String()
}

func (s *StringNode) ToString() string {
	var sb strings.Builder
	sb.WriteString(commentPrefix(s.PrecedingComment))
	if s.Header != nil {
		hdr := s.Header.Origin
		sb.WriteString(hdr)
		if s.InlineComment != nil {
			cmt := commentSuffix(s.InlineComment)
			// Guarantee whitespace between the block-scalar header and a same-line inline comment. Source like
			// `|  # c` has trailing whitespace baked into the header Origin, but goccy sometimes emits `|`
			// (no trailing space) + `#c` (no leading), which concat back into `|#` and the next parse pass
			// tokenizes as a single Invalid token. A single space between them keeps the re-parse honest.
			if len(hdr) > 0 && len(cmt) > 0 && !isSpace(hdr[len(hdr)-1]) && !isSpace(cmt[0]) {
				sb.WriteByte(' ')
			}
			sb.WriteString(cmt)
		}
		if s.Token != nil {
			sb.WriteString(s.Token.Origin)
		}
		return sb.String()
	}
	if s.Token != nil {
		sb.WriteString(s.Token.Origin)
	}
	sb.WriteString(commentSuffix(s.InlineComment))
	return sb.String()
}

func (n *IntNode) ToString() string   { return scalarLeafToString(n.PrecedingComment, n.Token, n.InlineComment) }
func (n *FloatNode) ToString() string { return scalarLeafToString(n.PrecedingComment, n.Token, n.InlineComment) }
func (n *BoolNode) ToString() string  { return scalarLeafToString(n.PrecedingComment, n.Token, n.InlineComment) }

// scalarLeafToString is the shared render path for IntNode/FloatNode/BoolNode (and equivalent to the non-block
// branch of StringNode.ToString). NullNode keeps its own ToString because its Token may be nil (implicit null)
// which requires a slightly different branch.
func scalarLeafToString(pre *CommentNode, tok *token.Token, inline *CommentNode) string {
	var sb strings.Builder
	sb.WriteString(commentPrefix(pre))
	if tok != nil {
		sb.WriteString(tok.Origin)
	}
	sb.WriteString(commentSuffix(inline))
	return sb.String()
}

func (n *NullNode) ToString() string {
	out := commentPrefix(n.PrecedingComment)
	if n.Token != nil {
		out += n.Token.Origin
	}
	return out + commentSuffix(n.InlineComment)
}

func (e *EmptyNode) ToString() string {
	return commentPrefix(e.PrecedingComment)
}

func (c *CommentNode) ToString() string { return commentPrefix(c) }

func (a *AnchorNode) ToString() string {
	out := commentPrefix(a.PrecedingComment) + a.Amp.Origin + a.Name.Origin
	if a.Value != nil {
		out += a.Value.ToString()
	}
	return out + commentSuffix(a.InlineComment)
}

func (a *AliasNode) ToString() string {
	return commentPrefix(a.PrecedingComment) + a.Star.Origin + a.Name.Origin + commentSuffix(a.InlineComment)
}

func (t *TagNode) ToString() string {
	out := commentPrefix(t.PrecedingComment) + t.Tag.Origin
	if t.Value != nil {
		out += t.Value.ToString()
	}
	return out + commentSuffix(t.InlineComment)
}

func (m *MappingNode) ToString() string {
	var sb strings.Builder
	sb.WriteString(commentPrefix(m.PrecedingComment))
	if m.Open != nil {
		sb.WriteString(m.Open.Origin)
	}
	for _, c := range m.Children {
		sb.WriteString(c.ToString())
	}
	if m.Close != nil {
		sb.WriteString(m.Close.Origin)
	}
	sb.WriteString(commentSuffix(m.InlineComment))
	return sb.String()
}

func (e *MappingEntry) ToString() string {
	var sb strings.Builder
	sb.WriteString(commentPrefix(e.PrecedingComment))
	if e.ExplicitKeyMarker != nil {
		sb.WriteString(e.ExplicitKeyMarker.Origin)
	}
	if e.Key != nil {
		sb.WriteString(e.Key.ToString())
	}
	if e.Colon != nil {
		sb.WriteString(e.Colon.Origin)
	}
	if e.Value != nil {
		sb.WriteString(e.Value.ToString())
	}
	if e.Trailer != nil {
		sb.WriteString(e.Trailer.Origin)
	}
	sb.WriteString(commentSuffix(e.InlineComment))
	return sb.String()
}

func (s *SequenceNode) ToString() string {
	var sb strings.Builder
	sb.WriteString(commentPrefix(s.PrecedingComment))
	if s.Open != nil {
		sb.WriteString(s.Open.Origin)
	}
	for _, c := range s.Children {
		sb.WriteString(c.ToString())
	}
	if s.Close != nil {
		sb.WriteString(s.Close.Origin)
	}
	sb.WriteString(commentSuffix(s.InlineComment))
	return sb.String()
}

func (i *SequenceItem) ToString() string {
	var sb strings.Builder
	sb.WriteString(commentPrefix(i.PrecedingComment))
	if i.Marker != nil {
		sb.WriteString(i.Marker.Origin)
	}
	if i.Value != nil {
		sb.WriteString(i.Value.ToString())
	}
	if i.Trailer != nil {
		sb.WriteString(i.Trailer.Origin)
	}
	sb.WriteString(commentSuffix(i.InlineComment))
	return sb.String()
}

// commentPrefix concatenates a comment group's line Origins for rendering "before" a node (PrecedingComment slots).
// commentSuffix is the same shape but used for inline/trailing positions - they render identically today, kept
// separate as two names for clarity at call sites.
func commentPrefix(c *CommentNode) string {
	if c == nil {
		return ""
	}
	var sb strings.Builder
	for _, ln := range c.Lines {
		sb.WriteString(ln.Origin)
	}
	return sb.String()
}

func commentSuffix(c *CommentNode) string { return commentPrefix(c) }

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' }
