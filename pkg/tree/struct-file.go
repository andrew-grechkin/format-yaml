package tree

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
