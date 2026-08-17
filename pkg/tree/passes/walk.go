package passes

import "github.com/andrew-grechkin/format-yaml/pkg/tree"

// forEachDocBody calls visit for every top-level body node of every document in f.
func forEachDocBody(f *tree.File, visit func(tree.Node)) {
	for _, c := range f.Children {
		doc, ok := c.(*tree.Doc)
		if !ok {
			continue
		}
		for _, body := range doc.Children {
			visit(body)
		}
	}
}

// walkBlockValues descends a body node, calling onEntry/onItem for every mapping entry and sequence item that
// sits in BLOCK context, passing the indentation at which that entry/item sits. Flow collections are not
// descended into: block-only rewrites (block scalars, folded blocks) are illegal inside flow, so the callers
// simply never see those nodes. A callback returning true signals "handled, do not descend into this node's
// value".
//
// depth is the container's child indentation, matching normalizeIndent's notion of depth. It is authoritative
// even for the first entry of an inline mapping (`- key: v`), whose Origin leading is relative to the marker
// rather than an absolute column.
func walkBlockValues(n tree.Node, depth int, onEntry func(*tree.MappingEntry, int) bool, onItem func(*tree.SequenceItem, int) bool) {
	switch v := n.(type) {
	case *tree.MappingNode:
		if v.Style == tree.StyleFlow {
			return
		}
		for _, c := range v.Children {
			walkBlockValues(c, depth, onEntry, onItem)
		}
	case *tree.MappingEntry:
		if onEntry(v, depth) {
			return
		}
		walkBlockValues(v.Value, depth+indentStep, onEntry, onItem)
	case *tree.SequenceNode:
		if v.Style == tree.StyleFlow {
			return
		}
		for _, c := range v.Children {
			walkBlockValues(c, depth, onEntry, onItem)
		}
	case *tree.SequenceItem:
		if onItem(v, depth) {
			return
		}
		walkBlockValues(v.Value, depth+indentStep, onEntry, onItem)
	case *tree.AnchorNode:
		walkBlockValues(v.Value, depth, onEntry, onItem)
	case *tree.TagNode:
		walkBlockValues(v.Value, depth, onEntry, onItem)
	}
}
