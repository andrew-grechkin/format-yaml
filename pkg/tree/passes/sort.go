package passes

import (
	"sort"

	"github.com/andrew-grechkin/format-yaml/internal/token"
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// Alphabetically reorders each block mapping's entries by key. Skips mappings where reordering would break
// meaning or leave orphaned bytes:
//   - Flow-style mappings ({a: 1, b: 2}): single-line output makes a sort visually odd, and flow order can be
//     meaningful in JSON-compatible contexts.
//   - Subtree contains an AnchorNode or AliasNode: YAML anchors are position-sensitive - an alias must textually
//     follow its definition. The user opted into anchors; leaving order alone respects that.
//   - Any child is an EmptyNode (island comment between entries): moving entries around leaves the island
//     floating with no correct new home. Skip until layout passes give us a better place to put it.
//   - Any entry's Key is not a plain scalar: complex keys (explicit-key `? [seq]`, mappings-as-keys, etc.)
//     don't have a natural sort string; skip rather than guess.
//
// Before sorting, transfers any comment on the mapping itself (MappingNode.PrecedingComment) onto the first
// entry - in block style there's no separator between "before mapping" and "before first entry", so those two
// positions are the same bytes and the comment logically belongs to the entry. This lets it move WITH that entry
// during sort (matching goccy's behavior on the sort fixture).
func sortMappingKeys(f *tree.File) {
	tree.Walk(f, func(n tree.Node) {
		m, ok := n.(*tree.MappingNode)
		if !ok || m.Style != tree.StyleBlock {
			return
		}
		entries, ok := sortableEntries(m)
		if !ok {
			return
		}
		if m.PrecedingComment != nil && len(entries) > 0 {
			entries[0].PrecedingComment = mergeComments(m.PrecedingComment, entries[0].PrecedingComment)
			m.PrecedingComment = nil
		}
		sort.SliceStable(entries, func(i, j int) bool {
			return entryKey(entries[i]) < entryKey(entries[j])
		})
		m.Children = entriesToNodes(entries)
	})
}

// Extracts the entries slice from m if every child is a MappingEntry with a plain-scalar key and no anchor/
// alias in its subtree; returns ok=false otherwise so the pass leaves the mapping alone.
func sortableEntries(m *tree.MappingNode) ([]*tree.MappingEntry, bool) {
	entries := make([]*tree.MappingEntry, 0, len(m.Children))
	for _, c := range m.Children {
		e, ok := c.(*tree.MappingEntry)
		if !ok {
			return nil, false
		}
		if _, ok := e.Key.(tree.Scalar); !ok {
			return nil, false
		}
		if subtreeHasAnchorOrAlias(e) {
			return nil, false
		}
		entries = append(entries, e)
	}
	return entries, true
}

// Returns true if walking from n visits any AnchorNode or AliasNode. Used to detect entries whose ordering
// matters because they participate in YAML's anchor/alias reference system.
func subtreeHasAnchorOrAlias(n tree.Node) bool {
	found := false
	tree.Walk(n, func(x tree.Node) {
		switch x.(type) {
		case *tree.AnchorNode, *tree.AliasNode:
			found = true
		}
	})
	return found
}

// Returns the scalar Value used for lexical comparison. Guaranteed non-panicking because sortableEntries has
// already filtered to Scalar keys. Reads Token.Value directly (via the Scalar interface's ScalarToken) rather
// than parsed Data() so int/float keys sort by their source text form (`"10" < "2"`, same as goccy behavior).
func entryKey(e *tree.MappingEntry) string {
	tok := tree.TokenOf(e.Key)
	if tok == nil {
		return ""
	}
	return tok.Value
}

// Concatenates two CommentNode groups head + tail (either may be nil). Returned CommentNode holds the union
// of lines in source order.
func mergeComments(head, tail *tree.CommentNode) *tree.CommentNode {
	if head == nil {
		return tail
	}
	if tail == nil {
		return head
	}
	return &tree.CommentNode{Lines: append(append([]*token.Token{}, head.Lines...), tail.Lines...)}
}

func entriesToNodes(entries []*tree.MappingEntry) []tree.Node {
	out := make([]tree.Node, len(entries))
	for i, e := range entries {
		out[i] = e
	}
	return out
}
