// Source-blank inspection helpers used by the emitter's boundary blank-line policy at minimal/standard modes. All
// functions here read the raw source; none write to the output.
package render

import (
	"strings"

	"github.com/goccy/go-yaml/ast"
)

// Marks a top-level mapping entry in a body split into lines. keyLine is the line index of the `key:` line; groupStart
// is the earliest line that visually belongs to the entry - either the key line itself, or the first of any contiguous
// unindented head-comment lines directly above it. Blank counts are anchored to groupStart so a preceding head comment
// counts as part of the entry, not as content between entries.
type topLevelEntry struct {
	keyLine    int
	groupStart int
}

func findTopLevelEntries(lines []string) []topLevelEntry {
	var entries []topLevelEntry
	for i, line := range lines {
		if !isTopLevelLine(line) {
			continue
		}
		groupStart := i
		for j := i - 1; j >= 0 && isCommentLine(lines[j]); j-- {
			groupStart = j
		}
		entries = append(entries, topLevelEntry{keyLine: i, groupStart: groupStart})
	}
	return entries
}

func isCommentLine(line string) bool {
	return len(line) > 0 && line[0] == '#'
}

// Reports whether line is the start of a top-level mapping entry: a non-empty line beginning with a non-whitespace,
// non-comment character. Document markers (`---`, `...`) and comment lines (`#`) are framing/annotation, not entries.
func isTopLevelLine(line string) bool {
	if len(line) == 0 || line[0] == ' ' || line[0] == '\t' || line[0] == '#' {
		return false
	}
	if line == "---" || strings.HasPrefix(line, "--- ") {
		return false
	}
	if line == "..." || strings.HasPrefix(line, "... ") {
		return false
	}
	return true
}

// Slices src along each doc's source-line range and runs countSourceBlanks per slice. Multi-doc files otherwise get a
// flat count that mixes doc N's boundaries with doc N+1's body.
func perDocSourceBlanks(src []byte, file *ast.File) [][]int {
	lines := strings.Split(string(src), "\n")
	out := make([][]int, len(file.Docs))
	for i, doc := range file.Docs {
		startLine := docStartLine(doc)
		endLine := len(lines)
		if i+1 < len(file.Docs) {
			endLine = docStartLine(file.Docs[i+1])
		}
		if startLine >= endLine || startLine < 0 {
			continue
		}
		segment := strings.Join(lines[startLine:endLine], "\n")
		out[i] = countSourceBlanks([]byte(segment))
	}
	return out
}

// Returns, per doc, the number of blank lines immediately preceding that doc's `---` start marker in the source. out[0]
// is the count above doc 0 (typically 0); out[i] for i > 0 is the count between doc i-1 and doc i. Consulted at minimal
// to preserve the author's inter-doc spacing.
func sourceInterDocBlanks(src []byte, file *ast.File) []int {
	lines := strings.Split(string(src), "\n")
	out := make([]int, len(file.Docs))
	for i, doc := range file.Docs {
		startLine := docStartLine(doc)
		if startLine <= 0 {
			continue
		}
		n := 0
		for j := startLine - 1; j >= 0; j-- {
			if strings.TrimSpace(lines[j]) != "" {
				break
			}
			n++
		}
		out[i] = n
	}
	return out
}

// Scans the raw source bytes and, for each pair of consecutive top-level entries, counts contiguous blank lines
// immediately preceding the next entry's group (head comments + key). Returned slice has length (top-level entry count
// - 1); entry i is the count between entries i and i+1.
func countSourceBlanks(src []byte) []int {
	lines := strings.Split(string(src), "\n")
	entries := findTopLevelEntries(lines)
	if len(entries) < 2 {
		return nil
	}
	counts := make([]int, len(entries)-1)
	for i := range len(entries) - 1 {
		for j := entries[i+1].groupStart - 1; j > entries[i].keyLine; j-- {
			if strings.TrimSpace(lines[j]) != "" {
				break
			}
			counts[i]++
		}
	}
	return counts
}

// Reports whether n's rendered form spans more than one line. Nested block-style mappings and sequences count; so do
// block scalars (LiteralNode / FoldedNode). Plain and quoted scalars are always single-line even if their string value
// contains \n escapes - the escape is preserved textually, not expanded. Anchor wrappers are transparent.
func valueIsMultiline(n ast.Node) bool {
	switch v := n.(type) {
	case *ast.MappingNode:
		return !v.IsFlowStyle && len(v.Values) > 0
	case *ast.SequenceNode:
		return !v.IsFlowStyle && len(v.Values) > 0
	case *ast.LiteralNode:
		return true
	case *ast.AnchorNode:
		return valueIsMultiline(v.Value)
	}
	return false
}
