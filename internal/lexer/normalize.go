// Post-tokenize normalization. Two jobs, in order:
//
//  1. repairOrigins - the scanner occasionally drops bytes between tokens (e.g. the newline after `---` when
//     the next token is an explicit `?` key). Origins are verbatim source slices in order, so we locate each
//     one in the source and hand any skipped bytes back to the preceding token (or the first token when the
//     gap is at the very start). This makes concatenated Origins reproduce the source exactly.
//  2. normalizeLeadingNewlines - enforce our contract with the tree: a token's Origin may START with same-line
//     indent SPACES but never with a NEWLINE. Any newlines that appeared as leading whitespace on a token get
//     shifted onto the previous token's trailing.
package lexer

import (
	"strings"

	"github.com/andrew-grechkin/format-yaml/internal/token"
)

func TokenizeNormalized(src string) token.Tokens {
	tokens := Tokenize(src)
	repairOrigins(tokens, src)
	normalizeLeadingNewlines(tokens)
	return tokens
}

// repairOrigins restores bytes the scanner left unattributed, walking the source with a cursor. See
// locateOrigin for how each token is matched. Any non-whitespace gap, or an Origin that cannot be located,
// aborts the repair for the remainder so an unexpected scanner shape degrades to the raw Origins instead of
// corrupting offsets.
func repairOrigins(tokens token.Tokens, src string) {
	if len(tokens) == 0 {
		return
	}
	pos := 0
	for i, tk := range tokens {
		if tk.Origin == "" {
			continue
		}
		if pos > len(src) {
			return
		}
		needle, rel := locateOrigin(src, pos, tk.Origin)
		if rel < 0 {
			return
		}
		if rel == 0 {
			tk.Origin = needle
			pos += len(needle)
			continue
		}
		gap := src[pos : pos+rel]
		if strings.TrimSpace(gap) != "" {
			// The match sits past non-whitespace, so it is probably a later duplicate, not this token.
			return
		}
		if i > 0 {
			tokens[i-1].Origin += gap
			tk.Origin = needle
		} else {
			tk.Origin = gap + needle
		}
		pos += rel + len(needle)
	}
	if pos < len(src) {
		tokens[len(tokens)-1].Origin += src[pos:]
	}
}

// locateOrigin finds where the next token's Origin begins at or after pos, returning the matched lexeme and
// its relative offset (or rel < 0 when it cannot be located):
//
//   - an exact prefix match wins immediately;
//   - otherwise a leading-newline mismatch is retried, since the scanner sometimes double-counts a boundary
//     newline (a Tag's Origin ends in `\n` while the next Origin starts with `\n`) and our contract strips
//     leading newlines anyway;
//   - otherwise the earliest of the exact and newline-trimmed lexemes is used, leaving the bytes before it as
//     a gap for the caller to hand back to the previous token.
func locateOrigin(src string, pos int, origin string) (string, int) {
	if strings.HasPrefix(src[pos:], origin) {
		return origin, 0
	}
	trimmed := strings.TrimLeft(origin, "\r\n")
	if trimmed != origin && strings.HasPrefix(src[pos:], trimmed) {
		return trimmed, 0
	}
	needle := origin
	rel := strings.Index(src[pos:], needle)
	if trimmed != origin {
		if r := strings.Index(src[pos:], trimmed); r >= 0 && (rel < 0 || r < rel) {
			needle, rel = trimmed, r
		}
	}
	return needle, rel
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


