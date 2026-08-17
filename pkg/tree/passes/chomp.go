package passes

import (
	"strings"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// narrowBlockChomp downgrades a keep-all chomp indicator (`|+` or `>+`) on a block scalar to its default
// form (`|` or `>`) when the scalar's value ends with exactly one trailing newline. Lossless: both `|+` and
// `|` render `content\n` when the trailing-newline count is 1; the only reason `|+` exists in that case is
// as a source-authored artifact. Narrowing it protects the block from a later pass that touches the
// surrounding whitespace and would otherwise let `|+` absorb the boundary blank line, silently growing the
// trailing-newline count on every format run.
//
// Leaves alone:
//   - `|+` / `>+` where trailing newlines >= 2: `+` is required for values with multiple trailing newlines.
//   - `|` / `>`: already minimal.
//   - `|-` / `>-`: strip-all chomp is a distinct semantic; not our concern here.
//   - Non-block strings (plain/quoted): no chomp to narrow.
func narrowBlockChomp(f *tree.File) {
	tree.Walk(f, func(n tree.Node) {
		s, ok := n.(*tree.StringNode)
		if !ok || s.Header == nil {
			return
		}
		indicator := ""
		switch s.Style {
		case tree.StringLiteral:
			indicator = "|"
		case tree.StringFolded:
			indicator = ">"
		default:
			return
		}
		keepAll := indicator + "+"
		if !strings.Contains(s.Header.Origin, keepAll) {
			return
		}
		value := ""
		if s.Token != nil {
			value = s.Token.Value
		}
		if trailingNewlines(value) != 1 {
			return
		}
		s.Header.Origin = strings.Replace(s.Header.Origin, keepAll, indicator, 1)
	})
}

// trailingNewlines counts consecutive `\n` bytes at the tail of s. Used to decide whether a keep-all chomp
// (`|+`) is redundant with the default clip chomp (`|`); they only agree when the count is exactly 1.
func trailingNewlines(s string) int {
	n := 0
	for i := len(s) - 1; i >= 0 && s[i] == '\n'; i-- {
		n++
	}
	return n
}
