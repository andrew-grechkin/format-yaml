package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// StringStyle is a rendering choice for a string scalar. Same semantic content across all styles; passes like
// preferSingleQuotes swap Style plus rewrite Token.Origin together to change the source form. Only strings
// need a style enum - int/float/bool/null have exactly one canonical representation apiece (variants like
// `0xff` vs `255` or `.INF` vs `.inf` are baked into Token.Origin and canonicalized by pass-specific rewrite
// logic if desired, not tracked as a "style" here).
type StringStyle int

const (
	StringPlain        StringStyle = iota // bare: `hello`
	StringSingleQuoted                    // 'hello'
	StringDoubleQuoted                    // "hello"
	StringLiteral                         // |, |+, |-
	StringFolded                          // >, >+, >-
)

// StringNode is a string leaf. Header holds the `|`/`>` (with chomp modifier) token for block styles and is
// nil for plain/quoted styles; Token (via ScalarBase) holds the body text.
type StringNode struct {
	ScalarBase
	Style  StringStyle
	Header *token.Token
}

func (s *StringNode) Data() any {
	if s.Token == nil {
		return ""
	}
	return s.Token.Value
}
