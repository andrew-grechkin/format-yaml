// Post-tokenize normalization enforcing our contract with the scanner: each token's Origin holds trailing
// whitespace/newlines but never leading. Source bytes get partitioned so each belongs to exactly one token, and
// concatenating all Origins in order reproduces source verbatim (up to any content the scanner drops entirely).
// Kept in this separate file so lexer.go (a verbatim copy from goccy) stays pristine and easy to diff against
// upstream if we ever need to re-sync. When we eventually rewrite the scanner to produce trailing-only Origins
// directly, this pass becomes a no-op and can be deleted.
package lexer

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// TokenizeNormalized runs Tokenize and then rebases every token's Origin to hold only trailing whitespace.
// Preferred entry point for callers building an AST - the invariant "no leading whitespace in Origin" lets the
// tree emit by simple Origin concatenation with no de-overlap or alignment logic.
func TokenizeNormalized(src string) token.Tokens {
	tokens := Tokenize(src)
	normalizeLeadingWhitespace(tokens)
	return tokens
}

// normalizeLeadingWhitespace shifts each token's leading whitespace onto the previous token's trailing. The very
// first token's leading whitespace stays where it is (nothing before it in source to absorb it). After this pass,
// concatenating Origins gives back the source bytes the scanner reported without overlap.
func normalizeLeadingWhitespace(tokens token.Tokens) {
	for i := 1; i < len(tokens); i++ {
		origin := tokens[i].Origin
		lead := 0
		for lead < len(origin) && isWhitespace(origin[lead]) {
			lead++
		}
		if lead == 0 {
			continue
		}
		tokens[i-1].Origin += origin[:lead]
		tokens[i].Origin = origin[lead:]
	}
}

func isWhitespace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
