// The pipeline's byte-in / byte-out core plus the I/O drivers that wrap it (stdin, file batches, in-place
// variants). Every entry point runs Bytes at its core, guaranteeing an all-or-nothing contract for the caller.
package format

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/andrew-grechkin/format-yaml/internal/config"
	"github.com/andrew-grechkin/format-yaml/internal/encoding"
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
	treepasses "github.com/andrew-grechkin/format-yaml/pkg/tree/passes"
)

// ErrCheckDiff is a sentinel returned by Check* when the input isn't already formatted. main exits 1 silently
// for it - filenames were already printed to stdout (files case) or the caller only cares about the exit code
// (stdin case).
var ErrCheckDiff = errors.New("check: input would be reformatted")

// Bytes drives the whole pipeline: transcode to UTF-8, build our tree from the in-repo lexer, apply the
// mode-cumulative pass set, emit. Callers only see a complete byte slice or an error - never a half-produced
// doc. The builder signals unsupported/malformed input by panicking (so development surfaces gaps loudly);
// this boundary converts that into the parse-error path main maps to exit 2.
func Bytes(src []byte, m config.Mode) (out []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("parse yaml: %v", r)
		}
	}()

	src, err = encoding.NormalizeToUTF8(src)
	if err != nil {
		return nil, err
	}
	// Output is always LF; normalize Windows line endings before the builder captures Origins verbatim.
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n"))

	treepasses.LineWidth = config.LineWidth
	// standard preserves source blanks (collapsed to one); full discards them and re-derives spacing from
	// the sorted tree shape.
	treepasses.KeepSourceBlanks = m < config.ModeFull
	file := tree.Build(src)
	treepasses.Apply(file, treePassesFor(m))
	return tree.Emit(file), nil
}

// treePassesFor translates format-yaml's Mode enum into the pass bitmap. Modes are cumulative: minimal carries
// the lossless/one-character changes, standard adds structural layout, full adds the shape-changing passes
// (block scalars, folding, flow/block conversion, sorting), and pedantic adds the `...` footer.
func treePassesFor(m config.Mode) treepasses.Set {
	s := treepasses.UnquoteSafeStrings
	s |= treepasses.EnsureDocMarker
	s |= treepasses.NarrowBlockChomp
	s |= treepasses.NormalizeNulls
	s |= treepasses.StripTrailingSpace

	if m >= config.ModeStandard {
		s |= treepasses.EnsureBlankLines
		s |= treepasses.MaterializeImplicitNulls
		s |= treepasses.NormalizeIndent
		s |= treepasses.NormalizeInlineCommentSpacing
		s |= treepasses.UnflowTopLevel
	}

	if m >= config.ModeFull {
		s |= treepasses.PreferSingleQuotes
		s |= treepasses.SortMappingKeys
		s |= treepasses.BlockScalarizeMultiline
		s |= treepasses.FoldLongScalars
		s |= treepasses.UnflowLongFlows
		s |= treepasses.FlowShortBlocks
		s |= treepasses.EnsureKeepChompFooter
	}

	if m >= config.ModePedantic {
		s |= treepasses.EnsureDocFooter
	}

	return s
}
