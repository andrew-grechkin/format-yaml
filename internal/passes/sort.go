package passes

import (
	"sort"

	"github.com/goccy/go-yaml/ast"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"
)

// Alphabetically reorders every block-style mapping's entries, with one deliberate exception: mappings that contain an
// anchor or alias anywhere in their children's subtrees keep source order. YAML anchors are position-sensitive - an
// alias must textually follow its definition - and the user opted into anchors for a reason. Better to respect their
// layout than risk a subtle reordering bug. Flow-style mappings are left alone.
func sortMappingKeys(root ast.Node) {
	astutil.Walk(root, func(n ast.Node) bool {
		mn, ok := n.(*ast.MappingNode)
		if !ok || mn.IsFlowStyle {
			return true
		}
		if mappingTouchesAnchors(mn) {
			return true
		}
		sort.SliceStable(mn.Values, func(i, j int) bool {
			return astutil.KeyString(mn.Values[i].Key) < astutil.KeyString(mn.Values[j].Key)
		})
		return true
	})
}

// Reports whether any child's subtree contains an AnchorNode or AliasNode. When true, sortMappingKeys skips this level.
func mappingTouchesAnchors(mn *ast.MappingNode) bool {
	for _, mv := range mn.Values {
		if hasAnchorOrAlias(mv.Value) {
			return true
		}
	}
	return false
}

func hasAnchorOrAlias(n ast.Node) bool {
	found := false
	astutil.Walk(n, func(x ast.Node) bool {
		switch x.(type) {
		case *ast.AnchorNode, *ast.AliasNode:
			found = true
			return false
		}
		return !found
	})
	return found
}
