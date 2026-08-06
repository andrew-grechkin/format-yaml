package passes

import (
	"github.com/goccy/go-yaml/ast"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// Shifts every block-style child so it sits at parent_key_col + 2. Called with nextCol==1 at the doc body: recurses
// down, deriving each level's target from the parent key's column.
//
// Mapping keys move only from mode standard upward - minimal preserves the source's mapping indent so a 4-space file
// stays 4-space when the user opted out of restructuring. Block sequences move in every mode: dashes flush with their
// parent key are always shifted to the indented form regardless of mode, because normalising that specific style is
// what this pass exists for.
//
// Comments follow the shift for free: goccy renders head/foot comments with spacing derived from the entry key's
// column, and inline comments sit tight against the value.
func normalizeIndent(container ast.Node, nextCol int, m config.Mode) {
	switch v := astutil.UnwrapAnchor(container).(type) {
	case *ast.MappingNode:
		if v.IsFlowStyle {
			return
		}
		for _, mv := range v.Values {
			if m >= config.ModeStandard {
				if delta := nextCol - mv.Key.GetToken().Position.Column; delta != 0 {
					mv.AddColumn(delta)
				}
			}
			normalizeIndent(mv.Value, mv.Key.GetToken().Position.Column+2, m)
		}
	case *ast.SequenceNode:
		if v.IsFlowStyle {
			return
		}
		if delta := nextCol - v.Start.Position.Column; delta != 0 {
			v.AddColumn(delta)
		}
		for _, elem := range v.Values {
			normalizeIndent(elem, v.Start.Position.Column+2, m)
		}
	}
}
