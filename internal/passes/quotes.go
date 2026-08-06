package passes

import (
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/token"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"
)

// Swaps a double-quoted scalar's quote marker for the single-quoted form when the content can be represented literally
// - i.e. contains no characters that would still need double-quoted escaping (`\n`, `\t` at edges, other control
// chars). Any embedded single quote is doubled per the YAML 1.2 rule.
//
// Assumes blockScalarizeMultiline has already run, so no incoming value contains an embedded newline.
func preferSingleQuotes(root ast.Node) {
	astutil.Walk(root, func(n ast.Node) bool {
		s, ok := n.(*ast.StringNode)
		if !ok {
			return true
		}
		if s.Token.Type != token.DoubleQuoteType {
			return true
		}
		if !canSingleQuote(s.Value) {
			return true
		}
		s.Token.Type = token.SingleQuoteType
		s.Token.Origin = "'" + strings.ReplaceAll(s.Value, "'", "''") + "'"
		return true
	})
}

// Reports whether v can be represented in single-quoted form without changing its value. Single-quoted YAML rejects
// control chars other than plain literal tab; anything below 0x20 (except tab) or the DEL char forces a double-quoted
// representation.
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
