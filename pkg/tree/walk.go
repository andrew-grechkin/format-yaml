// Depth-first pre-order visitor for the tree. Kept as a free function rather than an interface method so callers
// can supply a plain closure per pass and node types stay lean (no Walk method per type). Comment nodes attached
// as PrecedingComment/InlineComment slots are NOT visited - passes concerned with comments touch those slots
// directly on the owning node. Same for structural token pointers (Colon, Open, Close, Marker, etc.) - those are
// data on the node, not children.
package tree

import "reflect"

// Walk visits n and every descendant in depth-first pre-order. Nil-safe: both a bare nil Node and a
// typed nil pointer (e.g. `(*DocFooter)(nil)` from an optional field) are skipped so callers don't have
// to guard optional fields at every call site. visit is called on every non-nil node in the tree,
// including EmptyNode and leaf types - passes filter by type inside visit.
func Walk(n Node, visit func(Node)) {
	if isNilNode(n) {
		return
	}
	visit(n)
	switch v := n.(type) {
	case *File:
		for _, c := range v.Children {
			Walk(c, visit)
		}
	case *Doc:
		Walk(v.Header, visit)
		for _, c := range v.Children {
			Walk(c, visit)
		}
		Walk(v.Footer, visit)
	case *MappingNode:
		for _, c := range v.Children {
			Walk(c, visit)
		}
	case *MappingEntry:
		Walk(v.Key, visit)
		Walk(v.Value, visit)
	case *SequenceNode:
		for _, c := range v.Children {
			Walk(c, visit)
		}
	case *SequenceItem:
		Walk(v.Value, visit)
	case *AnchorNode:
		Walk(v.Value, visit)
	case *TagNode:
		Walk(v.Value, visit)
	}
}

// isNilNode reports whether n is either a bare nil interface OR a typed nil pointer wrapped in a Node
// interface. The latter comes up when passing an optional field (Doc.Header/Footer, entry values, etc.)
// directly to Walk even when the field is nil; the interface value is non-nil (it has type info) but the
// underlying pointer is nil, and calling any method on it would panic. Every Node concrete type is a
// pointer, so a single reflect check covers all of them.
func isNilNode(n Node) bool {
	if n == nil {
		return true
	}
	v := reflect.ValueOf(n)
	return v.Kind() == reflect.Ptr && v.IsNil()
}
