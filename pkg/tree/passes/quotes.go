package passes

import (
	"strings"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// preferSingleQuotes rewrites double-quoted string scalars to single-quoted when the parsed value can be
// represented safely: no embedded newlines, no control characters below 0x20 except tab, no DEL. Embedded
// single quotes are doubled per YAML 1.2. Leaves plain, already-single-quoted, and block-style strings alone.
// Preserves the Origin's surrounding whitespace via replaceScalarLexeme so a following inline comment or
// trailing newline stays attached.
func preferSingleQuotes(f *tree.File) {
	tree.Walk(f, func(n tree.Node) {
		s, ok := n.(*tree.StringNode)
		if !ok || s.Style != tree.StringDoubleQuoted || s.Token == nil {
			return
		}
		if !canSingleQuote(s.Token.Value) {
			return
		}
		s.Token.Origin = replaceScalarLexeme(s.Token.Origin, singleQuoteLexeme(s.Token.Value))
		s.Style = tree.StringSingleQuoted
	})
}

// singleQuoteLexeme wraps v in single quotes and doubles any embedded single quotes per YAML 1.2's escape
// rule ('' inside a single-quoted string represents a literal ').
func singleQuoteLexeme(v string) string {
	return "'" + strings.ReplaceAll(v, "'", "''") + "'"
}

// canSingleQuote reports whether v can round-trip as a single-quoted string. Single-quoted YAML rejects any
// control character other than plain literal tab; the check also excludes DEL (0x7F).
func canSingleQuote(v string) bool {
	for _, r := range v {
		if r == '\n' {
			return false
		}
		if r < 0x20 && r != '\t' {
			return false
		}
		if r == 0x7F {
			return false
		}
	}
	return true
}
