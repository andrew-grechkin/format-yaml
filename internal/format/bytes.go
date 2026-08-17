// The pipeline's byte-in / byte-out core plus the I/O drivers that wrap it (stdin, file batches, in-place variants).
// Every entry point runs Bytes at its core, guaranteeing an all-or-nothing contract for the caller.
package format

import (
	"errors"
	"fmt"

	"github.com/goccy/go-yaml/parser"

	"github.com/andrew-grechkin/format-yaml/internal/config"
	"github.com/andrew-grechkin/format-yaml/internal/encoding"
	"github.com/andrew-grechkin/format-yaml/internal/passes"
	"github.com/andrew-grechkin/format-yaml/internal/render"
)

// ErrCheckDiff is a sentinel returned by Check* when the input isn't already formatted. main exits 1 silently for it -
// filenames were already printed to stdout (files case) or the caller only cares about the exit code (stdin case).
var ErrCheckDiff = errors.New("check: input would be reformatted")

// Drives the whole pipeline: parse, run mode-cumulative passes over each doc's AST, render into a fresh buffer. Callers
// only see a complete byte slice or an error - never a half-produced doc.
func Bytes(src []byte, m config.Mode) ([]byte, error) {
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
