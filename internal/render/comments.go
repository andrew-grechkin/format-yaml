// Comment-handling helpers used by the emitter. Doc-level head- comment extraction (called before passes.Apply so sort
// doesn't drag the comment along with the first entry), source-inline gap measurement (used at minimal mode to preserve
// column alignment), and the low-level line inspection helpers those two rely on.
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

// Splits the first mapping entry's head comment group by source-line gaps. Comments contiguously touching the key (gap
// == 1 chain up to the key line) stay with the entry. Everything before the first blank-line break is doc-level: those
// comment tokens are removed from the entry and returned as raw text so the emitter can splice them under the `---`
// header. Extracting before sort keeps them anchored to the document instead of traveling with the entry when it moves.
func ExtractDocLevelHeadComments(doc *ast.DocumentNode) []string {
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
