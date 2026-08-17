package tree

// EmptyNode is a positional slot with no body content - a place in the tree that carries a floating comment.
// Used when a comment is blank-line-separated from any subsequent content, or when a comment sits at the tail
// of a container with nothing to attach forward to. Renders only PrecedingComment on emit.
type EmptyNode struct {
	Comments
}

func (*EmptyNode) Data() any { return nil }
