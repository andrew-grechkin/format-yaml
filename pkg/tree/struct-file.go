package tree

// File is a YAML stream: an ordered list of children. Children can be DirectiveNodes, Docs, or EmptyNodes (island
// comments that sit between docs, before the first doc, or after the last doc - stream-level comments that don't
// belong to any particular document).
type File struct {
	Children []Node
}
