// Structural helpers the AST-driven emitter (emit.go) leans on: inter-doc comment attribution based on source blank
// layout, and doc-start line lookup used by the source-blank inspection helpers in blanks.go.
//
// The pre-emitter text-layer pipeline (RenderFile + writeMappingDoc + friends) lived here until format.Bytes was
// rewired to EmitFile; see git history for its shape.
package render

import (
	"strings"

	"github.com/goccy/go-yaml/ast"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"
)

// Walks the AST and re-attributes head comments whose physical layout says "foot of the previous entry" - the comment
// sits source-adjacent to Values[i-1] with a blank line separating it from Values[i]'s key. Goccy attributes such a
// comment to Values[i].Comment because its rule follows "the comment sits before the last blank line preceding a
// key," but the source visually reads (and the author intended) foot-of-previous. Without this fix, minimal-mode
// output shuffles the blank line to the wrong side of the comment. Walking recursively catches nested mappings and
// sequences of mappings too.
//
// Runs before passes.Apply so downstream logic (sort, quote normalisation, emit) only ever sees comments in their
// visually-correct slots.
func ReattributeAdjacentHeadComments(root ast.Node, src []byte) {
	srcLines := strings.Split(string(src), "\n")
	astutil.Walk(root, func(n ast.Node) bool {
		mn, ok := n.(*ast.MappingNode)
		if !ok || mn.IsFlowStyle {
			return true
		}
		for i := 1; i < len(mn.Values); i++ {
			maybeMoveHeadToFoot(mn.Values[i-1], mn.Values[i], srcLines)
		}
		return true
	})
}

// Moves next.Comment onto prev.FootComment when the source-blank layout around next.Comment says "foot of prev." The
// diagnostic is symmetric with isFootOfPreviousDoc's inter-doc rule: blank line AFTER the comment (before next's key)
// AND not-blank BEFORE (comment touches prev's value). Any other layout reads as goccy attributed it and the AST
// stays untouched.
func maybeMoveHeadToFoot(prev, next *ast.MappingValueNode, srcLines []string) {
	if next.Comment == nil || len(next.Comment.Comments) == 0 {
		return
	}
	first := next.Comment.Comments[0]
	last := next.Comment.Comments[len(next.Comment.Comments)-1]
	if first.Token == nil || first.Token.Position == nil || last.Token == nil || last.Token.Position == nil {
		return
	}
	firstLine := first.Token.Position.Line - 1
	lastLine := last.Token.Position.Line - 1
	blankBefore := firstLine > 0 && firstLine-1 < len(srcLines) && strings.TrimSpace(srcLines[firstLine-1]) == ""
	blankAfter := lastLine+1 < len(srcLines) && strings.TrimSpace(srcLines[lastLine+1]) == ""
	if !blankAfter || blankBefore {
		return
	}
	if prev.FootComment == nil {
		prev.FootComment = next.Comment
	} else {
		prev.FootComment.Comments = append(prev.FootComment.Comments, next.Comment.Comments...)
	}
	next.Comment = nil
}

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
		// Head-of-next attribution only makes sense when there IS a next doc; on the last doc, any dangling foot
		// comment must stay attached or emitInterDocBlanks (which only fires for docIdx > 0) will drop it.
		if i == len(file.Docs)-1 {
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
