// Tree transformation passes. Each file owns one concern (nulls today; quotes, sort, flow/block conversion, indent,
// block scalars, chomps, folding as they're ported). Callers assemble a bitmap Set of the passes they want and
// call Apply, which runs them in a fixed order chosen to match dependencies: shape-changing passes run before
// layout-changing passes; block-scalar reindent runs after the parent column is final; etc.
//
// The bitmap is deliberately decoupled from format-yaml's Mode enum - this package lives under pkg/ so it can be
// consumed by update-yaml (and anything else) without dragging format-yaml's config in. Callers translate their
// own mode/config into a Set at the boundary.
package passes

import (
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// Set is a bitmap of enabled passes. Zero value runs no passes and the tree comes out identical to what went in.
// Add new pass bits as they're ported; keep the order in Apply matching declaration order here.
type Set uint32

const (
	// NormalizeNulls forces every null-valued node (explicit `null`, `~`, `Null`, `NULL`, and implicit-empty
	// mapping values) to the canonical lowercase `null` literal. Whitespace surrounding the token stays
	// untouched; a following inline comment or trailing newline continues to sit on the same token's Origin.
	NormalizeNulls Set = 1 << iota
	// SortMappingKeys reorders each block mapping's entries alphabetically by key. Skips flow mappings,
	// mappings whose subtree contains an anchor/alias, mappings with island comment children, and mappings
	// whose keys are not plain scalars. See sort.go for detail.
	SortMappingKeys
	// NormalizeIndent canonicalizes every block container's indent to 2 spaces per nesting level. Structural
	// walk - reads no source Position info; each block-container child owns its own leading indent on its
	// first token's Origin, so this pass rewrites those leadings to `depth` spaces. Same-depth sibling
	// reorders are indent-safe on their own (each entry carries its indent); this pass still runs to
	// canonicalize non-standard source indents (4-space, tabs) to the tree's 2-space convention.
	NormalizeIndent
	// PreferSingleQuotes swaps double-quoted string scalars to single-quoted whenever the parsed value can
	// be represented safely in single-quoted form (no embedded newlines, no control chars other than tab,
	// no DEL). Embedded single quotes get doubled per YAML 1.2. Plain, already-single-quoted, and block
	// scalars are left alone.
	PreferSingleQuotes
	// UnquoteSafeStrings drops the quotes from every explicitly-quoted string whose value would parse
	// back to itself as a plain scalar. Uses goccy's parser as the safety oracle. Skips YAML 1.1
	// boolean spellings and sexagesimal patterns to preserve semantics for older parsers. Tracks flow
	// context and applies a stricter check inside flow containers.
	UnquoteSafeStrings
	// NarrowBlockChomp downgrades `|+` or `>+` block-scalar chomp indicators to their default `|`/`>`
	// form when the value's trailing-newline count is exactly 1. Lossless simplification; also protects
	// keep-all blocks from silently absorbing a boundary blank line under later whitespace-touching
	// passes. Leaves `|+`/`>+` with >=2 trailing newlines alone (the `+` is required there).
	NarrowBlockChomp
	// UnflowTopLevel converts a flow-style top-level container (JSON-like `{...}` or `[...]`) to block
	// layout. Drops the enclosing brackets, rewrites each entry's Trailer from `,` to `\n`, and for
	// sequences synthesizes a `- ` Marker on each item. Nested flow constructs (inline collections
	// under a block parent) are LEFT ALONE - inlining nested content is usually deliberate.
	UnflowTopLevel
	// EnsureDocMarker synthesizes a canonical `---` Header on any Doc that lacks one, and hoists any
	// file-level EmptyNodes immediately preceding that Doc into its Children as leading nodes so their
	// comments sit AFTER the marker and BEFORE the body.
	EnsureDocMarker
	// MaterializeImplicitNulls turns every implicit NullNode (source `key:` with no value) into an
	// explicit `null` scalar. Gated separately from NormalizeNulls because it's a semantic upgrade
	// ("no value" -> "explicit null") that only applies at standard+ modes.
	MaterializeImplicitNulls
	// NormalizeInlineCommentSpacing rewrites the whitespace between a value and its inline comment to
	// exactly two spaces. Ad-hoc alignment in source loses meaning after other passes change value
	// widths; canonicalizing keeps output stable.
	NormalizeInlineCommentSpacing
	// EnsureBlankLines inserts one blank line between adjacent Docs at the file level, and between
	// adjacent mapping entries in a doc where at least one entry's value is a nested block container
	// or block scalar. Idempotent: won't add more than a single blank if one already exists.
	EnsureBlankLines
)

// Apply runs the enabled passes over f in the pinned order below. Callers passing Set(0) get a no-op.
func Apply(f *tree.File, s Set) {
	if s&NormalizeNulls != 0 {
		normalizeNulls(f)
	}
	// if s&MaterializeImplicitNulls != 0 {
	// 	materializeImplicitNulls(f)
	// }
	// if s&SortMappingKeys != 0 {
	// 	sortMappingKeys(f)
	// }
	// if s&PreferSingleQuotes != 0 {
	// 	preferSingleQuotes(f)
	// }
	// if s&UnquoteSafeStrings != 0 {
	// 	unquoteSafeStrings(f)
	// }
	// if s&NarrowBlockChomp != 0 {
	// 	narrowBlockChomp(f)
	// }
	// if s&UnflowTopLevel != 0 {
	// 	unflowTopLevel(f)
	// }
	// if s&EnsureDocMarker != 0 {
	// 	ensureDocMarker(f)
	// }
	// if s&NormalizeInlineCommentSpacing != 0 {
	// 	normalizeInlineCommentSpacing(f)
	// }
	// if s&EnsureBlankLines != 0 {
	// 	ensureBlankLines(f)
	// }
	// if s&NormalizeIndent != 0 {
	// 	normalizeIndent(f)
	// }
}
