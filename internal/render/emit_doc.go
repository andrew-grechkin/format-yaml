package render

import (
	"strings"

	"github.com/goccy/go-yaml/ast"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// Emits a single doc, including `---` header, doc-level comments, body, and optional `...` end marker.
func (e *emitter) emitDoc(doc *ast.DocumentNode, docIdx int, docLevels [][]string) {
	// Comment-only doc (goccy models a top-of-file comment this way).
	if astutil.IsPhantomCommentDoc(doc) {
		cg := doc.Body.(*ast.CommentGroupNode)
		if e.buf.Len() > 0 {
			e.ensureTrailingNLs(1)
		}
		for _, c := range cg.Comments {
			e.writeByte('#')
			e.writeString(strings.TrimRight(c.Token.Value, " \t"))
			e.writeByte('\n')
		}
		e.prevWasComment = true
		return
	}

	e.emitInterDocBlanks(docIdx)
	e.prevWasComment = false
	if docIdx < len(e.perDocBlanks) {
		e.currentDocBoundaries = e.perDocBlanks[docIdx]
	} else {
		e.currentDocBoundaries = nil
	}
	e.writeString("---\n")
	if docIdx < len(docLevels) && len(docLevels[docIdx]) > 0 {
		for _, line := range docLevels[docIdx] {
			e.writeString(line)
			e.writeByte('\n')
		}
		e.writeByte('\n')
	}
	// Snapshot the keep-chomp-in-subtree verdict BEFORE emit descends.
	lastEntryKeepChomp := lastEntrySubtreeHasKeepChomp(doc)

	e.emitBody(doc.Body, 0)
	e.ensureTrailingNLs(1)

	if e.needDocEnd(doc) {
		// A `...` marker sits directly under the last content line UNLESS the last-entry subtree contains a `|+`/`>+`
		// block whose trailing newlines were cut short by a sibling. In that case we insert one blank line above `...`
		// as a visual echo of what the value would have contributed had the block sat at the tail.
		needBlank := lastEntryKeepChomp || e.mode < config.ModePedantic
		// Exception: when the doc tail itself IS keep-chomp, the wrapper's trailing newlines already provide the visual
		// separation and we've written them out. Adding another blank on top would double up.
		if docTailKeepsChomp(doc.Body) {
			needBlank = false
		}
		if needBlank {
			e.ensureTrailingNLs(2)
		}
		e.writeString("...\n")
		e.prevEmittedDocEnd = true
	} else {
		e.prevEmittedDocEnd = false
	}
}

// Ensures the correct number of blank lines above the `---` of the doc at docIdx. Doc 0 gets nothing above. Later docs
// consult the source-derived blank count at minimal, or a fixed policy at standard+ (one blank line between docs). A
// previous doc whose tail is a `|+`/`>+` block has already emitted its trailing newlines - those count toward the
// target, so ensureTrailingNLs (which never clips) does the right thing automatically.
func (e *emitter) emitInterDocBlanks(docIdx int) {
	if docIdx == 0 {
		return
	}
	// A comment-only doc directly precedes this one: no extra blank - the comment lines already sit above the `---`
	// we're about to write, and the fixture rule expects them adjacent.
	if e.prevWasComment {
		e.ensureTrailingNLs(1)
		return
	}
	// Hoisted head-of-next comments from the previous doc's foot slot: emit the comment lines above the coming `---`.
	// If the previous doc emitted a `...` end marker, the marker itself provides the doc boundary (isBlankOrDocEnd
	// treats `...` as blank-equivalent for attribution), so we skip the extra blank line - adding one would make
	// re-parse see the comment as its own doc, breaking idempotence.
	if docIdx-1 < len(e.postDoc) && len(e.postDoc[docIdx-1]) > 0 {
		if e.prevEmittedDocEnd {
			e.ensureTrailingNLs(1)
		} else {
			e.ensureTrailingNLs(2)
		}
		for _, line := range e.postDoc[docIdx-1] {
			e.writeString(line)
			e.writeByte('\n')
		}
		return
	}
	// Target: N blank lines above `---` means (N + 1) '\n' at the tail of buf right before we write "---\n". The +1
	// accounts for the line terminator of the previous doc's last content line.
	var blanks int
	switch {
	case e.mode >= config.ModeStandard:
		blanks = 1
	default:
		if docIdx < len(e.perDocLeadingBlanks) {
			blanks = e.perDocLeadingBlanks[docIdx]
		}
	}
	e.ensureTrailingNLs(blanks + 1)
}

// Reports whether the doc needs an explicit `...` end marker. Pedantic emits at every doc; full+ emits when the tail is
// a `|+`/`>+` block that would otherwise be ambiguous (its trailing newlines run into the next doc boundary with no
// `---` marker to close them). Elsewhere the marker is omitted unless the source already had one (doc.End preserved
// from parse).
func (e *emitter) needDocEnd(doc *ast.DocumentNode) bool {
	if doc.End != nil {
		return true
	}
	if e.mode >= config.ModePedantic {
		return true
	}
	if e.mode >= config.ModeFull && docTailKeepsChomp(doc.Body) {
		return true
	}
	return false
}
