package passes

import (
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// ensureDocMarker guarantees every Doc in the file has an explicit `---` Header. Sources that omit the
// marker get one synthesized; sources that have it are left alone.
//
// Also hoists any file-level EmptyNodes that sit immediately before a header-less Doc into that Doc's
// Children as leading nodes. In source layout those EmptyNodes carry comments that logically belong to the
// doc (they sit before its content); with the marker now inserted, moving them inside the Doc puts them
// AFTER the `---` marker and BEFORE the body, matching the expected `---\n# comment\nbody` layout.
func ensureDocMarker(f *tree.File) {
	var newChildren []tree.Node
	var pending []tree.Node
	for _, c := range f.Children {
		if _, isEmpty := c.(*tree.EmptyNode); isEmpty {
			pending = append(pending, c)
			continue
		}
		doc, isDoc := c.(*tree.Doc)
		if !isDoc {
			newChildren = append(newChildren, pending...)
			pending = nil
			newChildren = append(newChildren, c)
			continue
		}
		if doc.Header == nil {
			doc.UpdateHeader(nil)
			doc.Children = append(pending, doc.Children...)
			pending = nil
		} else {
			newChildren = append(newChildren, pending...)
			pending = nil
		}
		newChildren = append(newChildren, doc)
	}
	newChildren = append(newChildren, pending...)
	f.Children = newChildren
}
