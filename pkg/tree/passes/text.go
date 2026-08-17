package passes

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// LineWidth is the display-column budget the fold/unfold passes target. It mirrors the CLI's
// FORMAT_YAML_LINE_WIDTH and is set by the caller before Apply; kept as a package var rather than an
// import of format-yaml's config so this package stays consumable on its own.
var LineWidth = 120

// DisplayWidth returns the number of monospace columns s occupies. Byte length is wrong for CJK/emoji
// input, where a rune's rendered width and byte length differ.
func DisplayWidth(s string) int { return runewidth.StringWidth(s) }

// NeedsQuoting mirrors a plain scalar's quoting constraints closely enough to estimate the width of the
// scalar if it were written plain. It is only used for the fold budget, never to decide quoting.
func NeedsQuoting(s string) bool {
	if s == "" {
		return true
	}
	switch s[0] {
	case ' ', '\t', '-', '?', ':', ',', '[', ']', '{', '}', '#', '&', '*', '!', '|', '>', '\'', '"', '%', '@', '`':
		return true
	}
	last := s[len(s)-1]
	if last == ' ' || last == '\t' || last == ':' {
		return true
	}
	if strings.Contains(s, ": ") || strings.Contains(s, " #") {
		return true
	}
	switch strings.ToLower(s) {
	case "true", "false", "null", "yes", "no", "on", "off", "~":
		return true
	}
	return false
}

// FoldWrap breaks s across lines no wider than width display columns, splitting only on single spaces.
// Runs of two or more spaces stay atomic inside a segment: splitting a multi-space run would leave a
// wrapped line starting with whitespace, which folded-scalar semantics read as "more indented" and keep
// as a literal newline instead of folding back to a space.
func FoldWrap(s string, width int) []string {
	if s == "" || width < 1 {
		return []string{s}
	}
	atoms := SplitFoldAtoms(s)
	lines := make([]string, 0, len(atoms))
	var cur strings.Builder
	curWidth := 0
	for _, a := range atoms {
		aw := DisplayWidth(a)
		switch {
		case cur.Len() == 0:
			cur.WriteString(a)
			curWidth = aw
		case curWidth+1+aw <= width:
			cur.WriteByte(' ')
			cur.WriteString(a)
			curWidth += 1 + aw
		default:
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(a)
			curWidth = aw
		}
	}
	return append(lines, cur.String())
}

// SplitFoldAtoms splits s at single-space boundaries. Multi-space runs stay glued to their neighbours so
// they survive a fold round-trip byte-for-byte.
func SplitFoldAtoms(s string) []string {
	atoms := make([]string, 0, 8)
	var cur strings.Builder
	i := 0
	for i < len(s) {
		if s[i] != ' ' {
			cur.WriteByte(s[i])
			i++
			continue
		}
		j := i
		for j < len(s) && s[j] == ' ' {
			j++
		}
		if j-i == 1 {
			atoms = append(atoms, cur.String())
			cur.Reset()
		} else {
			cur.WriteString(s[i:j])
		}
		i = j
	}
	return append(atoms, cur.String())
}

// leadingSpaces returns the count of leading space/tab bytes in s - the indent a token's Origin carries.
func leadingSpaces(s string) int {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return i
}

// scalarLexeme returns the content bytes of an Origin: everything between its leading and trailing
// whitespace.
func scalarLexeme(origin string) string {
	lead := leadingSpaces(origin)
	trail := len(origin)
	for trail > lead && isWhitespace(origin[trail-1]) {
		trail--
	}
	return origin[lead:trail]
}
