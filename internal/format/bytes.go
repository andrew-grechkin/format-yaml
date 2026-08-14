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
	// Extract doc-level head comments before passes run so the sort pass in full+ mode doesn't drag them along with the
	// first entry. render.ExtractDocLevelHeadComments handles a nil body via its own type-assertion guard, so no
	// per-doc nil check needed here.
	docLevels := make([][]string, len(file.Docs))
	for i, doc := range file.Docs {
		docLevels[i] = render.ExtractDocLevelHeadComments(doc)
	}
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
	return render.EmitFile(file, src, m, docLevels), nil
}
