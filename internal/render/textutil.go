package render

import "strings"

// Removes trailing spaces and tabs from every line that isn't inside a block scalar. Trailing whitespace inside a `|`
// or `>` block is part of the string content and must be preserved; everywhere else it's semantically invisible and
// shows only when a reviewer looks at raw bytes.
//
// Detection is textual: a line matching `<indent><key>: <chomp>...` or `<indent>- <chomp>...` opens a block; content
// lines indented deeper than the header stay in the block; a dedented line closes it. Blank lines don't affect
// membership - they simply have no whitespace to touch.
//
// Runs in every mode: trailing whitespace is subtractive-only and diff-invisible, which fits even minimal's "no reflow"
// contract.
func stripTrailingWhitespace(body string) string {
	if body == "" {
		return body
	}
	lines := strings.Split(body, "\n")
	inBlock := false
	blockOuterIndent := 0
	for i, line := range lines {
		trimmedLeft := strings.TrimLeft(line, " \t")
		if inBlock {
			if trimmedLeft == "" {
				continue
			}
			indent := len(line) - len(trimmedLeft)
			if indent > blockOuterIndent {
				continue
			}
			inBlock = false
		}
		lines[i] = strings.TrimRight(line, " \t")
		if col := blockScalarHeaderIndent(lines[i]); col >= 0 {
			inBlock = true
			blockOuterIndent = col
		}
	}
	return strings.Join(lines, "\n")
}

// Returns the 0-indexed column where a block scalar's owner starts (the key or the `-`), or -1 if the line isn't a
// block scalar header. Handles:
//
//   - mapping form: `key: |` / `key: >`
//   - sequence form (block value on the item itself): `- |` / `- >`
//   - sequence-of-mapping form (block value on a mapping key inside a sequence item): `- key: |` / `- key: >`
//
// Any chomp/indent indicator or trailing comment after `|`/`>` is tolerated.
func blockScalarHeaderIndent(line string) int {
	trimmed := strings.TrimLeft(line, " \t")
	if trimmed == "" {
		return -1
	}
	indent := len(line) - len(trimmed)
	// Peel any sequence markers so we can uniformly check for either a bare block indicator or a `key: |` after them.
	inner := trimmed
	for strings.HasPrefix(inner, "- ") {
		inner = strings.TrimLeft(inner[2:], " \t")
	}
	if inner != "" && (inner[0] == '|' || inner[0] == '>') {
		return indent
	}
	colonIdx := strings.IndexByte(inner, ':')
	if colonIdx <= 0 {
		return -1
	}
	rest := strings.TrimLeft(inner[colonIdx+1:], " \t")
	if rest == "" || (rest[0] != '|' && rest[0] != '>') {
		return -1
	}
	return indent
}
