package tree

// Commented is satisfied by every node type that embeds Comments. Passes that want to inspect or mutate
// comment slots on "any node with them" take a Commented instead of listing the fourteen concrete types.
type Commented interface {
	Node
	Preceding() *CommentNode
	Inline() *CommentNode
}
