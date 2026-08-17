// Structural helpers the AST-driven emitter (emit.go) leans on: inter-doc comment hoisting on the Doc wrapper, and
// doc-start line lookup used by the source-blank inspection helpers in blanks.go.
package render

import (
	"strings"

	"github.com/goccy/go-yaml/ast"
)

// HoistPreDocComments walks docs[i].FootComment for i in 0..len-2 and, when the source layout says "head of next
// doc," moves the group to docs[i+1].PreDocComments (raw lines - the position is decided by this pass, not by the
// emitter, so source-line info stops being useful). The loop bound is the whole story: the last doc's FootComment
// is skipped structurally, never promoted to a next-doc slot that doesn't exist, so the trailing comment stays
// where the extractor put it.
func HoistPreDocComments(docs []Doc, src []byte) {
	srcLines := strings.Split(string(src), "\n")
	for i := 0; i < len(docs)-1; i++ {
		cg := docs[i].FootComment
		if !readsAsHeadOfNextDoc(cg, srcLines) {
			continue
		}
		lines := make([]string, 0, len(cg.Comments))
		for _, c := range cg.Comments {
			lines = append(lines, "#"+c.Token.Value)
		}
		docs[i+1].PreDocComments = append(docs[i+1].PreDocComments, lines...)
		docs[i].FootComment = nil
	}
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
