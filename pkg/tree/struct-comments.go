package tree

// Comments is embedded by every node type that carries a preceding + inline comment slot. Preceding is a
// comment group sitting above the node in source; Inline is a same-line trailing comment (` # note`).
// Sharing this struct via embedding keeps the field names uniform across the tree so passes can access
// n.PrecedingComment / n.InlineComment on any node type that has them without repeating declarations.
type Comments struct {
	PrecedingComment *CommentNode
	InlineComment    *CommentNode
}

// Preceding returns the preceding comment slot. Provides the same access via method so passes can
// operate polymorphically on any node that embeds Comments (via the Commented interface below).
func (c *Comments) Preceding() *CommentNode { return c.PrecedingComment }

// Inline returns the inline comment slot. Same rationale as Preceding.
func (c *Comments) Inline() *CommentNode { return c.InlineComment }
