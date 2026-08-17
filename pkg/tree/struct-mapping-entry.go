package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// MappingEntry is a single key/value pair inside a MappingNode. ExplicitKeyMarker is populated when the entry
// used the `? key\n: value` form. Colon carries the `:` token so its Origin (which holds the whitespace/
// newline separating key from value) round-trips verbatim. Trailer is the post-entry marker: `,` in a flow
// mapping, `\n` in a block mapping (which is how block layout separates one entry from the next). Nil for
// the last flow entry or wherever no explicit trailer is present.
type MappingEntry struct {
	Comments
	ExplicitKeyMarker *token.Token
	Key               Node
	Colon             *token.Token
	Value             Node
	Trailer           *token.Token
}

func (e *MappingEntry) Data() any {
	if e.Value == nil {
		return nil
	}
	return e.Value.Data()
}
