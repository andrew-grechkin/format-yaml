package tree

import (
	tokenizer "github.com/andrew-grechkin/format-yaml/internal/token"
)

// commentBuffer holds standalone comment groups that have been read but not yet attached to a node.
// Each group is a run of source-contiguous comment lines; a blank line starts a new group. Because
// groups keep source order, at most the LAST group can be source-adjacent to the next structural token
// - every earlier group is an island that renders at its own position as an EmptyNode.
//
// All comment attribution in the parser goes through this one type, which is why the recursion stays
// simple: productions just ask "what comments sit before my next token?" instead of threading comment
// state through every call.
type commentBuffer struct {
	groups []*CommentNode
}

// absorbComments drains leading comment tokens into groups. Contiguous comment lines extend the open
// (last) group; a line gap starts a new one.
func (p *parser) absorbComments() {
	for {
		t := p.peek()
		if t == nil || t.Type != tokenizer.CommentType {
			return
		}
		p.next()

		if last := p.comments.last(); last != nil && lastLine(last)+1 == t.Position.Line {
			last.Lines = append(last.Lines, t)
			continue
		}
		p.comments.groups = append(p.comments.groups, &CommentNode{Lines: []*tokenizer.Token{t}})
	}
}

func (cb *commentBuffer) last() *CommentNode {
	if len(cb.groups) == 0 {
		return nil
	}
	return cb.groups[len(cb.groups)-1]
}

// islandsBefore drains every group that is NOT source-adjacent to line as an EmptyNode, in source
// order. When the last group IS adjacent, it is left in the buffer so the caller can claim it as the
// upcoming node's PrecedingComment.
func (cb *commentBuffer) islandsBefore(line int) []Node {
	split := len(cb.groups)
	if last := cb.last(); last != nil && lastLine(last)+1 == line {
		split = len(cb.groups) - 1
	}
	return cb.drainThrough(split)
}

// takeIfAdjacent removes and returns the last group when it is source-adjacent to line, else nil. Call
// it right before consuming the node the comment sits above.
func (cb *commentBuffer) takeIfAdjacent(line int) *CommentNode {
	last := cb.last()
	if last == nil || lastLine(last)+1 != line {
		return nil
	}
	cb.groups = cb.groups[:len(cb.groups)-1]
	return last
}

// drain flushes every pending group as its own EmptyNode. Used where no following structural node can
// claim them (end of stream, doc boundary).
func (cb *commentBuffer) drain() []Node { return cb.drainThrough(len(cb.groups)) }

// drainAtLeast claims the trailing groups whose comment column is at or inside col, as EmptyNodes in source
// order. Groups dedented below col are left buffered for an enclosing container to claim - a comment a level
// up must not be adopted (and indented) by a container it doesn't belong to.
func (cb *commentBuffer) drainAtLeast(col int) []Node {
	n := len(cb.groups)
	for n > 0 && column(cb.groups[n-1]) >= col {
		n--
	}
	if n == len(cb.groups) {
		return nil
	}
	out := make([]Node, 0, len(cb.groups)-n)
	for _, g := range cb.groups[n:] {
		out = append(out, &EmptyNode{Comments: Comments{PrecedingComment: g}})
	}
	cb.groups = cb.groups[:n]
	return out
}

// islandsBeforeAtLeast is islandsBefore with the same column guard as drainAtLeast: non-adjacent groups
// dedented below col stay for the enclosing container.
func (cb *commentBuffer) islandsBeforeAtLeast(line, col int) []Node {
	split := len(cb.groups)
	if last := cb.last(); last != nil && lastLine(last)+1 == line {
		split = len(cb.groups) - 1
	}
	n := split
	for n > 0 && column(cb.groups[n-1]) >= col {
		n--
	}
	if n == split {
		return nil
	}
	out := make([]Node, 0, split-n)
	for _, g := range cb.groups[n:split] {
		out = append(out, &EmptyNode{Comments: Comments{PrecedingComment: g}})
	}
	// Keep the shallower prefix and any adjacent group, preserving source order.
	remaining := append(cb.groups[:n:n], cb.groups[split:]...)
	cb.groups = remaining
	return out
}

// column returns the source column of a group's first comment line (1-based).
func column(c *CommentNode) int {
	return c.Lines[0].Position.Column
}

func (cb *commentBuffer) drainThrough(n int) []Node {
	if n == 0 {
		return nil
	}
	out := make([]Node, 0, n)
	for _, g := range cb.groups[:n] {
		out = append(out, &EmptyNode{Comments: Comments{PrecedingComment: g}})
	}
	cb.groups = cb.groups[n:]
	return out
}

// takeInline consumes the next token as an inline comment when it is a comment on line, else nil.
func (p *parser) takeInline(line int) *CommentNode {
	t := p.peek()
	if t == nil || t.Type != tokenizer.CommentType || t.Position.Line != line {
		return nil
	}
	p.next()
	return &CommentNode{Lines: []*tokenizer.Token{t}}
}

func lastLine(c *CommentNode) int {
	return c.Lines[len(c.Lines)-1].Position.Line
}

// mergeComments concatenates two comment groups head-then-tail into one group. Nil-safe. Used where a value
// has to carry more than one comment source (the colon-line comment plus any comment already above it).
func mergeComments(head, tail *CommentNode) *CommentNode {
	switch {
	case head == nil:
		return tail
	case tail == nil:
		return head
	}
	lines := make([]*tokenizer.Token, 0, len(head.Lines)+len(tail.Lines))
	lines = append(lines, head.Lines...)
	lines = append(lines, tail.Lines...)
	return &CommentNode{Lines: lines}
}

// islandsComment merges the comment groups carried by island EmptyNodes (in source order) plus an optional
// adjacent group into a single CommentNode, so they can ride a nested value's PrecedingComment slot. The
// blank-line separation between groups survives because the newline run lives on the preceding group's last
// token Origin.
func islandsComment(islands []Node, adjacent *CommentNode) *CommentNode {
	out := adjacent
	for i := len(islands) - 1; i >= 0; i-- {
		e, ok := islands[i].(*EmptyNode)
		if !ok {
			continue
		}
		out = mergeComments(e.PrecedingComment, out)
	}
	return out
}

// takeFlowComment consumes one comment token when the stream is positioned on one, returning it as a
// single-line CommentNode. Comments inside flow collections are trivia: the flow parsers attach them to the
// previous item/entry (or hold them for the next) so they survive instead of leaking into the block buffer.
func (p *parser) takeFlowComment() *CommentNode {
	t := p.peek()
	if t == nil || t.Type != tokenizer.CommentType {
		return nil
	}
	p.next()
	return &CommentNode{Lines: []*tokenizer.Token{t}}
}

