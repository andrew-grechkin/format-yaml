// AST-shape passes. Each file in this package owns one concern (nulls, flow/block conversion, indent, block scalars,
// folding, chomps, quote style, sort). Apply threads the mode-cumulative sequence.
package passes

import (
	"github.com/goccy/go-yaml/ast"

	"github.com/andrew-grechkin/update-yaml/pkg/style"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// Runs the mode-cumulative pass sequence. blockScalarizeMultiline runs before preferSingleQuotes so the latter only
// sees single-line scalars. Blank-line placement is the emitter's per-boundary decision (see internal/render/emit.go),
// not part of this AST-transformation pipeline.
func Apply(root ast.Node, m config.Mode) {
	style.UnquoteSafeStrings(root)
	// canonicalizeBlockScalarChomp runs at every mode: it's lossless, and it protects source-authored `|+` blocks from
	// having their trailing-newline count grown by the standard+ multiline-blank rule when the value only needs `|` or
	// `|-`.
	canonicalizeBlockScalarChomp(root)
	if m >= config.ModeStandard {
		unflowTopLevel(root)
		normalizeNulls(root)
	}
	if m >= config.ModeFull {
		blockScalarizeMultiline(root)
		preferSingleQuotes(root)
		unflowLongFlows(root)
		flowShortBlocks(root)
		sortMappingKeys(root)
	}
	// Runs last so it operates on the final AST shape - block-scalar swaps and sort reordering are already reflected in
	// the token columns we're normalising here.
	normalizeIndent(root, 1, m)
	// blockScalarizeMultiline hard-codes a 2-space body indent into LiteralNode Origin at build time (before it knows
	// the final key column). normalizeIndent can then move the key without touching the baked-in indent, leaving the
	// body under the key. Rerun after normalizeIndent so every LiteralNode's body indent matches its final parent
	// column.
	if m >= config.ModeFull {
		reindentBlockScalars(root)
		// foldLongScalars runs after normalizeIndent + reindent so the parent key column is final and it can bake in
		// the correct body indent from the start. Order relative to reindent is deliberate: reindent operates on
		// existing `|` blocks; this pass produces fresh `>` blocks that reindent must leave alone (its skip-`>` guard
		// depends on that).
		foldLongScalars(root)
	}
}
