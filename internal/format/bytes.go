// The pipeline's byte-in / byte-out core plus the I/O drivers that wrap it (stdin, file batches, in-place variants).
// Every entry point runs Bytes at its core, guaranteeing an all-or-nothing contract for the caller.
package format

import (
	"errors"
	"fmt"
	"os"

	"github.com/goccy/go-yaml/parser"

	"github.com/andrew-grechkin/format-yaml/internal/config"
	"github.com/andrew-grechkin/format-yaml/internal/encoding"
	"github.com/andrew-grechkin/format-yaml/internal/passes"
	"github.com/andrew-grechkin/format-yaml/internal/render"
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
	treepasses "github.com/andrew-grechkin/format-yaml/pkg/tree/passes"
)

// ErrCheckDiff is a sentinel returned by Check* when the input isn't already formatted. main exits 1 silently for it -
// filenames were already printed to stdout (files case) or the caller only cares about the exit code (stdin case).
var ErrCheckDiff = errors.New("check: input would be reformatted")

// EnvUseTree opts the caller into the new pkg/tree-based pipeline. Set FORMAT_YAML_TREE=1 to route through
// bytesTree instead of the goccy path. Temporary dev flag - once all passes are ported and fixtures agree with
// goccy output, this becomes the default and gets removed along with the goccy path.
const EnvUseTree = "FORMAT_YAML_TREE"

// Drives the whole pipeline: parse, run mode-cumulative passes over each doc's AST, render into a fresh buffer. Callers
// only see a complete byte slice or an error - never a half-produced doc.
func Bytes(src []byte, m config.Mode) ([]byte, error) {
	if os.Getenv(EnvUseTree) == "1" {
		return bytesTree(src, m)
	}
	src, err := encoding.NormalizeToUTF8(src)
	if err != nil {
		return nil, err
	}
	file, err := parser.ParseBytes(src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	// Doc-level comment extraction runs before passes.Apply so the sort pass in full+ mode doesn't drag the extracted
	// tokens along with the first/last entry. Every foot slot gets drained into Doc.FootComment first; then
	// HoistPreDocComments walks the slice and promotes any group whose source layout reads as head-of-next-doc onto
	// docs[i+1].PreDocComments. Loop bound (< len-1) inside Hoist is the reason the last doc's trailing comment
	// never gets orphaned - no explicit last-doc guard needed.
	docs := make([]render.Doc, len(file.Docs))
	for i, doc := range file.Docs {
		docs[i] = render.Doc{
			Node:         doc,
			HeadComments: render.ExtractDocLevelHead(doc),
			FootComment:  render.ExtractDocLevelFoot(doc),
		}
	}
	render.HoistPreDocComments(docs, src)
	for _, doc := range file.Docs {
		if doc.Body == nil {
			continue
		}
		// Fix goccy's head/foot attribution for comments whose physical layout says "foot of previous entry" - the
		// comment is source-adjacent to entry N with a blank line separating it from entry N+1. Runs before passes so
		// sort and quote passes see the corrected attribution.
		render.ReattributeAdjacentHeadComments(doc.Body, src)
		passes.Apply(doc.Body, m)
	}
	return render.EmitFile(file, docs, src, m), nil
}

// bytesTree runs the incoming pkg/tree pipeline: build our AST directly from our internal lexer, apply the
// mode-cumulative pass Set, then Emit. Panics on any unsupported token type surface as CLI panics under
// FORMAT_YAML_TREE=1 - acceptable while this is an opt-in dev flag.
func bytesTree(src []byte, m config.Mode) ([]byte, error) {
	src, err := encoding.NormalizeToUTF8(src)
	if err != nil {
		return nil, err
	}
	file := tree.Build(src)
	treepasses.Apply(file, treePassesFor(m))
	return tree.Emit(file), nil
}

// Translates format-yaml's Mode enum into the pass bitmap that pkg/tree/passes understands. Passes get added
// here as they're ported from internal/passes; missing entries mean that behavior is still handled by the goccy
// path (which the tree path won't match until the port is complete). NormalizeIndent fires at all modes because
// leading-whitespace changes are `diff -w`-invisible and running format-yaml implies the user opted into
// formatting - preserving arbitrary source indent at minimal mode would make the tool inconsistent.
func treePassesFor(m config.Mode) treepasses.Set {
	// Gate aligned with what goccy actually produces at each mode (encoded in the fixture expected files),
	// NOT with the aspirational tier framework in project_mode_diff_tiers memory. Minimal covers null
	// canonicalization, chomp narrowing, quote stripping, and doc marker (all `diff -w`-hidden or single-
	// character content changes). Standard adds indent canonicalization and top-level flow-to-block.
	s := treepasses.UnquoteSafeStrings
	s |= treepasses.EnsureBlankLines
	s |= treepasses.EnsureDocMarker
	s |= treepasses.NarrowBlockChomp
	s |= treepasses.NormalizeNulls

	if m >= config.ModeStandard {
		s |= treepasses.MaterializeImplicitNulls
		s |= treepasses.NormalizeIndent
		s |= treepasses.NormalizeInlineCommentSpacing
		s |= treepasses.UnflowTopLevel
	}

	if m >= config.ModeFull {
		s |= treepasses.PreferSingleQuotes
		s |= treepasses.SortMappingKeys
	}

	return s
}
