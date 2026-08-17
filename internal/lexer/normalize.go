// Post-tokenize normalization enforcing our contract with the scanner: a token's Origin may START with same-
// line indent SPACES but never with a NEWLINE. Any newlines that appeared as leading whitespace on a token
// get shifted onto the previous token's trailing
package lexer

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

func TokenizeNormalized(src string) token.Tokens {
	tokens := Tokenize(src)
	normalizeLeadingNewlines(tokens)
	absorbUncoveredSourceSuffix(tokens, src)
	return tokens
}

func normalizeLeadingNewlines(tokens token.Tokens) {
	for i := 1; i < len(tokens); i++ {
		prev := tokens[i-1]
		cur := tokens[i]
		prev.Origin, cur.Origin = transferLeadingNewlinesUp(prev.Origin, cur.Origin)
		prev.Origin, cur.Origin = transferTrailingSpacesDown(prev.Origin, cur.Origin)
	}
}

func transferLeadingNewlinesUp(first, second string) (string, string) {
	var idx int
	for idx < len(second) && (second[idx] == '\n' || second[idx] == '\r') {
		idx++
	}

	leadingNewlines := second[:idx]
	updatedSecond := second[idx:]
	updatedFirst := first + leadingNewlines

	return updatedFirst, updatedSecond
}

func transferTrailingSpacesDown(first, second string) (string, string) {
	end := len(first)
	for end > 0 && first[end-1] == ' ' {
		end--
	}

	trailingSpaces := first[end:]
	updatedFirst := first[:end]
	updatedSecond := trailingSpaces + second

	return updatedFirst, updatedSecond
}

// absorbUncoveredSourceSuffix appends any source bytes past the concatenation of Origins to the last token's
// Origin. Goccy's scanner drops trailing whitespace past the final content token (e.g. `a: 1\n` tokenizes with
// last Origin `1`, not `1\n`); this fixup puts those bytes back so concat = source holds end-to-end.
func absorbUncoveredSourceSuffix(tokens token.Tokens, src string) {
	if len(tokens) == 0 {
		return
	}
	covered := 0
	for _, t := range tokens {
		covered += len(t.Origin)
	}
	if covered < len(src) {
		tokens[len(tokens)-1].Origin += src[covered:]
	}
}
