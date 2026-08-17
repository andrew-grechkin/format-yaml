package passes

import (
	"strings"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// foldLongScalars rewrites a single-line string value whose rendered `key: value` record exceeds the
// configured line width as a `>-` folded block scalar. The wrap is value-preserving because YAML folds
// interior breaks back to spaces, provided the value has no newline, no leading/trailing whitespace, and at
// least one single-space break point; anything else is left as-is (a `|-`/`|` literal is the job of
// blockScalarizeMultiline).
//
// Runs after normalizeIndent so the parent key column is final and the body indent is baked in correctly.
func foldLongScalars(f *tree.File) {
	noItems := func(*tree.SequenceItem, int) bool { return false }
	forEachDocBody(f, func(n tree.Node) {
		walkBlockValues(n, 0, foldEntry, noItems)
	})
}

// foldEntry rewrites a mapping entry's over-long single-line string value as a folded block, one indent step
// past the entry's depth. Returns true when it fired.
func foldEntry(e *tree.MappingEntry, depth int) bool {
	s, ok := e.Value.(*tree.StringNode)
	if !ok || !canFoldScalar(s) {
		return false
	}
	if !recordOverflows(depth, scalarLexeme(entryKeyOrigin(e)), s.Token.Value) {
		return false
	}
	bodyIndent := depth + indentStep
	e.Value = foldedBlock(s, FoldWrap(s.Token.Value, LineWidth-bodyIndent), bodyIndent)
	clearTrailer(e.Trailer)
	return true
}

// canFoldScalar reports whether s can become a `>-` block and fold back byte-for-byte.
func canFoldScalar(s *tree.StringNode) bool {
	if s.Header != nil || s.Token == nil {
		return false
	}
	switch s.Style {
	case tree.StringPlain, tree.StringSingleQuoted, tree.StringDoubleQuoted:
	default:
		return false
	}
	v := s.Token.Value
	if v == "" || strings.Contains(v, "\n") {
		return false
	}
	if v[0] == ' ' || v[0] == '\t' || v[len(v)-1] == ' ' || v[len(v)-1] == '\t' {
		return false
	}
	return hasSingleSpaceBreak(v)
}

// hasSingleSpaceBreak reports whether v has a position where a single space separates two non-space bytes -
// the only legal fold boundary.
func hasSingleSpaceBreak(v string) bool {
	for i := 0; i < len(v); i++ {
		if v[i] != ' ' {
			continue
		}
		prev := i > 0 && v[i-1] == ' '
		next := i+1 < len(v) && v[i+1] == ' '
		if !prev && !next {
			return true
		}
	}
	return false
}

// recordOverflows reports whether `<key>: <value>` would exceed the line width at the key's column. The
// value width is bumped by two when it would need quotes in plain form, so the decision is close enough
// around the boundary.
func recordOverflows(keyCol int, key, value string) bool {
	w := DisplayWidth(value)
	if NeedsQuoting(value) {
		w += 2
	}
	return keyCol+DisplayWidth(key)+2+w > LineWidth
}
