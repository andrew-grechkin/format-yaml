// Tail operations: every rendered tree ends in exactly one token (a scalar's Token, a container's closing or
// last child, a comment group's last line). tailToken resolves that token once; AppendNewline, TrimNewline and
// SetTrailingNewlines are thin operations over it. Keeping the tail logic in one type switch replaces the
// per-node AppendNewline/TrimNewline methods that used to be duplicated across every node type.
package tree

import (
	"strings"

	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// tailToken returns the token that owns n's rendered tail, or nil for an empty node. It mirrors render order:
// an inline comment renders last when present; otherwise the node's own token, closing token, or last child.
// The type space is split across small helpers so each stays under the complexity budget.
func tailToken(n Node) *token.Token {
	if t, ok := containerTail(n); ok {
		return t
	}
	if t, ok := entryTail(n); ok {
		return t
	}
	if t, ok := wrapperTail(n); ok {
		return t
	}
	if t, ok := simpleTail(n); ok {
		return t
	}
	return scalarTail(n)
}

// containerTail resolves stream/document/collection nodes: their tail is their last child (or closing token).
func containerTail(n Node) (*token.Token, bool) {
	switch v := n.(type) {
	case *File:
		return lastTail(v.Children), true
	case *Doc:
		if v.Footer != nil {
			return tailToken(v.Footer), true
		}
		if len(v.Children) > 0 {
			return lastTail(v.Children), true
		}
		if v.Header != nil {
			return tailToken(v.Header), true
		}
		return nil, true
	case *MappingNode:
		if t := commentTail(v.InlineComment); t != nil {
			return t, true
		}
		if v.Close != nil {
			return v.Close, true
		}
		return lastTail(v.Children), true
	case *SequenceNode:
		if t := commentTail(v.InlineComment); t != nil {
			return t, true
		}
		if v.Close != nil {
			return v.Close, true
		}
		return lastTail(v.Children), true
	}
	return nil, false
}

// entryTail resolves mapping entries and sequence items: inline comment, trailer, value, then colon/marker.
func entryTail(n Node) (*token.Token, bool) {
	switch v := n.(type) {
	case *MappingEntry:
		if t := commentTail(v.InlineComment); t != nil {
			return t, true
		}
		if v.Trailer != nil {
			return v.Trailer, true
		}
		if v.Value != nil {
			if t := tailToken(v.Value); t != nil {
				return t, true
			}
		}
		return v.Colon, true
	case *SequenceItem:
		if t := commentTail(v.InlineComment); t != nil {
			return t, true
		}
		if v.Trailer != nil {
			return v.Trailer, true
		}
		if v.Value != nil {
			if t := tailToken(v.Value); t != nil {
				return t, true
			}
		}
		return v.Marker, true
	}
	return nil, false
}

// wrapperTail resolves anchor/tag/alias wrappers: inline comment, wrapped value, then the wrapper's own token.
func wrapperTail(n Node) (*token.Token, bool) {
	switch v := n.(type) {
	case *AnchorNode:
		if t := commentTail(v.InlineComment); t != nil {
			return t, true
		}
		if v.Value != nil {
			if t := tailToken(v.Value); t != nil {
				return t, true
			}
		}
		return v.Name, true
	case *TagNode:
		if t := commentTail(v.InlineComment); t != nil {
			return t, true
		}
		if v.Value != nil {
			if t := tailToken(v.Value); t != nil {
				return t, true
			}
		}
		return v.Tag, true
	case *AliasNode:
		if t := commentTail(v.InlineComment); t != nil {
			return t, true
		}
		return v.Name, true
	}
	return nil, false
}

// simpleTail resolves doc markers, island comment nodes, and directives.
func simpleTail(n Node) (*token.Token, bool) {
	switch v := n.(type) {
	case *DocHeader:
		return inlineOrToken(v.InlineComment, v.Token), true
	case *DocFooter:
		return inlineOrToken(v.InlineComment, v.Token), true
	case *EmptyNode:
		return commentTail(v.PrecedingComment), true
	case *DirectiveNode:
		if len(v.Args) > 0 {
			return v.Args[len(v.Args)-1], true
		}
		return v.Name, true
	}
	return nil, false
}

// scalarTail resolves the tail token of a scalar leaf. A block scalar's rendered tail is its body, not the
// inline comment (which renders on the header); every other scalar renders its inline comment last when set.
func scalarTail(n Node) *token.Token {
	switch v := n.(type) {
	case *StringNode:
		if v.Header != nil {
			if v.Token != nil {
				return v.Token
			}
			if t := commentTail(v.InlineComment); t != nil {
				return t
			}
			return v.Header
		}
		return inlineOrToken(v.InlineComment, v.Token)
	case *IntNode:
		return inlineOrToken(v.InlineComment, v.Token)
	case *FloatNode:
		return inlineOrToken(v.InlineComment, v.Token)
	case *BoolNode:
		return inlineOrToken(v.InlineComment, v.Token)
	case *NullNode:
		return inlineOrToken(v.InlineComment, v.Token)
	}
	return nil
}

// lastTail returns the first non-nil tail scanning children from the end: a child that renders nothing (an
// empty null, a bare island) must not hide the token actually at the end of the rendered output.
func lastTail(children []Node) *token.Token {
	for i := len(children) - 1; i >= 0; i-- {
		if t := tailToken(children[i]); t != nil {
			return t
		}
	}
	return nil
}

func inlineOrToken(inline *CommentNode, tok *token.Token) *token.Token {
	if t := commentTail(inline); t != nil {
		return t
	}
	return tok
}

func commentTail(c *CommentNode) *token.Token {
	if c == nil || len(c.Lines) == 0 {
		return nil
	}
	return c.Lines[len(c.Lines)-1]
}

// AppendNewline appends one `\n` to n's rendered tail (a no-op when n is empty).
func AppendNewline(n Node) {
	if t := tailToken(n); t != nil {
		t.Origin += "\n"
	}
}

// TrimNewline removes one trailing `\n` from n's rendered tail, reporting whether it removed one.
func TrimNewline(n Node) bool {
	t := tailToken(n)
	if t == nil || !strings.HasSuffix(t.Origin, "\n") {
		return false
	}
	t.Origin = t.Origin[:len(t.Origin)-1]
	return true
}

// SetTrailingNewlines adjusts n's rendered output to end with exactly count `\n` bytes. It is the clipping
// counterpart to AppendNewline/TrailingNewlines: structural boundaries (blank-line policy, doc footer) can
// set the separator exactly instead of only growing it.
//
// Callers that must not corrupt content (e.g. a `|+` block's value newlines) pass a count that is a floor
// for the content - SetTrailingNewlines stops as soon as a tail has no newline left to remove.
func SetTrailingNewlines(n Node, count int) {
	for TrailingNewlines(n) > count {
		if !TrimNewline(n) {
			return
		}
	}
	for TrailingNewlines(n) < count {
		before := TrailingNewlines(n)
		AppendNewline(n)
		if TrailingNewlines(n) == before {
			return
		}
	}
}
