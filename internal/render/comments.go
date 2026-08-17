// Emitter-side comment plumbing: the Doc wrapper the emitter walks, the AST-level comment normalization that
// populates it (extract doc-level head/foot, reattribute adjacent mv.Comment onto prev.FootComment), source-inline
// gap measurement for minimal-mode column preservation, and the low-level line inspection helpers those rely on.
// Everything here compensates for goccy's comment attribution not matching the physical source layout a reader
// would infer.
package render

import (
	"strings"

	"github.com/goccy/go-yaml/ast"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"
)

// Locates a line's inline comment. commentAt is the byte index of `#`; valueEnd is where the value's non-whitespace
// content ends. Both are 0 when the line has no value-then-`#` pattern (no comment, or `#` preceded only by whitespace
// - i.e., a head comment).
func inlineCommentBounds(line string) (commentAt, valueEnd int) {
	commentAt = findInlineCommentStart(line)
	if commentAt <= 0 {
		return 0, 0
	}
	valueEnd = commentAt
	for valueEnd > 0 && (line[valueEnd-1] == ' ' || line[valueEnd-1] == '\t') {
		valueEnd--
	}
	if valueEnd == 0 {
		return 0, 0
	}
	return commentAt, valueEnd
}

// Doc pairs a parsed goccy DocumentNode with the doc-level comment groups lifted out of body-typed slots. Three
// positional buckets, one per slot the emitter recognizes:
//
//   - PreDocComments: rendered above `---`. Populated by HoistPreDocComments when the previous doc's foot slot held
//     comments whose source layout reads as head-of-next-doc.
//   - HeadComments:   rendered between `---` and the body. Populated by ExtractDocLevelHead (mapping-only: the
//     blank-separated portion of the first entry's Comment group).
//   - FootComment:    rendered after the body, before `...` or the next doc's `---`. Populated by
//     ExtractDocLevelFoot (mapping's last-entry FootComment, or sequence's SequenceNode.FootComment).
//
// The wrapper lets the emitter treat every doc-level comment slot as a first-class doc child regardless of body
// type - it never has to reach into a MappingNode or SequenceNode for a trailing/leading slot. FootComment stays as
// a live CommentGroupNode (not raw lines) so the emitter still has source-line positions for the mode-tiered
// leading-blank rule; HeadComments and PreDocComments are pre-flattened because their positioning is determined by
// the pipeline stage that populated them.
type Doc struct {
	Node           *ast.DocumentNode
	PreDocComments []string
	HeadComments   []string
	FootComment    *ast.CommentGroupNode
}

// ExtractDocLevelHead splits the first mapping entry's head comment group by source-line gaps. Comments contiguously
// touching the key (gap == 1 chain up to the key line) stay with the entry. Everything before the first blank-line
// break is doc-level: those comment tokens are removed from the entry and returned as raw text so callers can splice
// them under the `---` header. Extracting before sort keeps them anchored to the document instead of traveling with
// the entry when it moves.
//
// Sequence bodies don't need extraction: goccy attributes their leading comment group to SequenceNode.BaseNode.Comment,
// which sort passes never touch. emitSequence handles the emission inline.
func ExtractDocLevelHead(doc *ast.DocumentNode) []string {
	mn, ok := astutil.UnwrapAnchor(doc.Body).(*ast.MappingNode)
	if !ok || len(mn.Values) == 0 {
		return nil
	}
	first := mn.Values[0]
	if first.Comment == nil || len(first.Comment.Comments) == 0 {
		return nil
	}
	comments := first.Comment.Comments
	keyLine := first.Key.GetToken().Position.Line

	splitAt := 0
	for i := len(comments) - 1; i >= 0; i-- {
		nextLine := keyLine
		if i < len(comments)-1 {
			nextLine = comments[i+1].Token.Position.Line
		}
		if nextLine-comments[i].Token.Position.Line != 1 {
			splitAt = i + 1
			break
		}
	}
	if splitAt == 0 {
		return nil
	}
	docLevel := make([]string, 0, splitAt)
	for _, c := range comments[:splitAt] {
		docLevel = append(docLevel, "#"+c.Token.Value)
	}
	first.Comment.Comments = comments[splitAt:]
	return docLevel
}

// ExtractDocLevelFoot returns the doc-level trailing comment group, whichever body-typed slot goccy stashed it in,
// and clears that source slot so downstream passes and emit paths don't render it a second time. Handles the two
// body shapes goccy uses: a mapping's last-entry FootComment, and a sequence's own FootComment. Scalar/null bodies
// never carry a trailing slot in goccy - nothing to do there.
func ExtractDocLevelFoot(doc *ast.DocumentNode) *ast.CommentGroupNode {
	body := astutil.UnwrapAnchor(doc.Body)
	switch v := body.(type) {
	case *ast.MappingNode:
		if len(v.Values) == 0 {
			return nil
		}
		last := v.Values[len(v.Values)-1]
		cg := last.FootComment
		last.FootComment = nil
		return cg
	case *ast.SequenceNode:
		cg := v.FootComment
		v.FootComment = nil
		return cg
	}
	return nil
}

// ReattributeAdjacentHeadComments walks the AST and re-attributes head comments whose physical layout says "foot of
// the previous entry" - the comment sits source-adjacent to Values[i-1] with a blank line separating it from
// Values[i]'s key. Goccy attributes such a comment to Values[i].Comment because its rule follows "the comment sits
// before the last blank line preceding a key," but the source visually reads (and the author intended) foot-of-
// previous. Without this fix, minimal-mode output shuffles the blank line to the wrong side of the comment. Walks
// recursively so nested mappings inside sequences of mappings get the same treatment.
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

// maybeMoveHeadToFoot moves next.Comment onto prev.FootComment when the source-blank layout around next.Comment says
// "foot of prev." The diagnostic is symmetric with readsAsHeadOfNextDoc's inter-doc rule: blank line AFTER the
// comment (before next's key) AND not-blank BEFORE (comment touches prev's value). Any other layout reads as goccy
// attributed it and the AST stays untouched.
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

// readsAsHeadOfNextDoc reports whether the comment group's source-blank layout attributes it to the head of the
// following document rather than the foot of the current one. Rule table:
//
//   - blank line BEFORE the comment (or blanks on both sides): head of following doc.
//   - blank line AFTER the comment but NOT before: foot of previous doc (returns false).
//   - neither (comment squeezed between two content lines): default to head of following doc.
//
// A `...` doc-end marker counts as blank-equivalent (see isBlankOrDocEnd): it explicitly closes the previous doc, so
// anything before it lives inside that doc. Without this, pedantic-mode round-trips would flip the attribution on
// the second pass (the emitted `...` replaces the blank line the source had between the comment and the next `---`).
func readsAsHeadOfNextDoc(cg *ast.CommentGroupNode, srcLines []string) bool {
	if cg == nil || len(cg.Comments) == 0 {
		return false
	}
	firstLine := cg.Comments[0].Token.Position.Line - 1
	lastLine := cg.Comments[len(cg.Comments)-1].Token.Position.Line - 1
	blankBefore := firstLine > 0 && firstLine-1 < len(srcLines) && isBlankOrDocEnd(srcLines[firstLine-1])
	blankAfter := lastLine+1 < len(srcLines) && isBlankOrDocEnd(srcLines[lastLine+1])
	return !(blankAfter && !blankBefore)
}

// isBlankOrDocEnd reports whether line is either blank or a YAML doc end marker `...`. Doc end markers are treated as
// blank-equivalents for inter-doc-comment attribution because they carry the same semantic weight - "the previous
// doc ends here" - visually and structurally.
func isBlankOrDocEnd(line string) bool {
	t := strings.TrimSpace(line)
	return t == "" || t == "..." || strings.HasPrefix(t, "... ")
}

// Scans src, and for every line with an inline comment records the count of whitespace characters between the value and
// `#`, keyed by the simple key extracted from that line. Used at minimal mode to reproduce the author's column
// alignment on emit.
func collectSourceInlineSpaces(src []byte) map[string]int {
	out := make(map[string]int)
	for line := range strings.SplitSeq(string(src), "\n") {
		commentAt, valueEnd := inlineCommentBounds(line)
		if commentAt == 0 {
			continue
		}
		if key := extractInlineKey(line); key != "" {
			out[key] = commentAt - valueEnd
		}
	}
	return out
}

// Returns "" for any line that doesn't have a key-colon prefix, so callers can use the empty return as a fast skip.
func extractInlineKey(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	idx := strings.IndexByte(trimmed, ':')
	if idx <= 0 {
		return ""
	}
	return trimmed[:idx]
}

// Returns the byte index of the first `#` character that starts an inline comment on line - i.e., a `#` preceded by
// whitespace and not sitting inside a single- or double-quoted string. Returns -1 if the line has no inline comment.
func findInlineCommentStart(line string) int {
	inSingle, inDouble := false, false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		switch {
		case !inSingle && ch == '"':
			inDouble = !inDouble
		case !inDouble && ch == '\'':
			inSingle = !inSingle
		case !inSingle && !inDouble && ch == '#' && i > 0 && (line[i-1] == ' ' || line[i-1] == '\t'):
			return i
		}
	}
	return -1
}
