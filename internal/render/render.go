// Structural helpers the AST-driven emitter (emit.go) leans on: inter-doc comment attribution based on source blank
// layout, and doc-start line lookup used by the source-blank inspection helpers in blanks.go.
//
// The pre-emitter text-layer pipeline (RenderFile + writeMappingDoc + friends) lived here until format.Bytes was
// rewired to EmitFile; see git history for its shape.
package render

import (
	"strings"

	"github.com/goccy/go-yaml/ast"
)

// Strips the FootComment from the last entry of each mapping doc IF the comment attributes as head-of-following-doc,
// and returns the raw comment lines to emit between the two docs. Foot-of- previous-doc comments are LEFT ATTACHED so
// the emitter renders them inline with the previous doc's body (and, at pedantic, before the `...` footer).
//
// Attribution rule, per source blank layout around the comment:
//   - blank line BEFORE the comment (or blanks on both sides): head of following doc - detach.
//   - blank line AFTER the comment but NOT before: foot of previous doc - leave attached.
//   - neither (comment squeezed between two content lines): default to head of following doc - detach.
func detachInterDocFootComments(file *ast.File, src []byte) [][]string {
	srcLines := strings.Split(string(src), "\n")
	out := make([][]string, len(file.Docs))
	for i, doc := range file.Docs {
		mn, ok := doc.Body.(*ast.MappingNode)
		if !ok || len(mn.Values) == 0 {
			continue
		}
		last := mn.Values[len(mn.Values)-1]
		if last.FootComment == nil {
			continue
		}
		if isFootOfPreviousDoc(last.FootComment, srcLines) {
			continue // keep attached
		}
		lines := make([]string, 0, len(last.FootComment.Comments))
		for _, c := range last.FootComment.Comments {
			lines = append(lines, "#"+c.Token.Value)
		}
		out[i] = lines
		last.FootComment = nil
	}
	return out
}

// Reports whether the foot-comment block should belong to the previous doc per the source-blank rule: blank-after AND
// not-blank-before. Any other layout (blank-before, both sides blank, or neither) reads as head-of-next.
//
// A `...` doc-end marker counts as blank-equivalent: it explicitly closes the previous doc, so anything before it lives
// inside that doc. Without this, pedantic-mode round-trips would flip the attribution on the second pass (the emitted
// `...` replaces the blank line the source had between the comment and the next `---`).
func isFootOfPreviousDoc(cg *ast.CommentGroupNode, srcLines []string) bool {
	if cg == nil || len(cg.Comments) == 0 {
		return false
	}
	firstLine := cg.Comments[0].Token.Position.Line - 1
	lastLine := cg.Comments[len(cg.Comments)-1].Token.Position.Line - 1
	blankBefore := firstLine > 0 && isBlankOrDocEnd(srcLines[firstLine-1])
	blankAfter := lastLine+1 < len(srcLines) && isBlankOrDocEnd(srcLines[lastLine+1])
	return blankAfter && !blankBefore
}

// Reports whether line is either blank or a YAML doc end marker `...`. Doc end markers are treated as blank-equivalents
// for inter-doc-comment attribution because they carry the same semantic weight - "the previous doc ends here" -
// visually and structurally.
func isBlankOrDocEnd(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || t == "..." || strings.HasPrefix(t, "... ")
}

// Returns the 0-indexed source line where doc begins. Uses the `---` marker if present, otherwise falls back to the
// body's first token (for comment-only docs).
func docStartLine(doc *ast.DocumentNode) int {
	if doc.Start != nil {
		return doc.Start.Position.Line - 1
	}
	if doc.Body != nil {
		return doc.Body.GetToken().Position.Line - 1
	}
	return -1
}
