// Shared helpers for passes that rewrite a scalar's source lexeme while keeping its surrounding whitespace.
// Under the tokenizer contract a scalar Token.Origin looks like `<leading spaces><lexeme><trailing whitespace>`
// - leading may be same-line indent, trailing may be newlines or same-line spaces before a comment.
// Swapping the lexeme (e.g. `~` -> `null`, `"foo"` -> `'foo'`) has to preserve both sides.
package passes

// replaceScalarLexeme rewrites the content portion of a scalar Origin (everything between leading and
// trailing whitespace) with the given replacement, leaving the surrounding whitespace intact.
func replaceScalarLexeme(origin, replacement string) string {
	lead := 0
	for lead < len(origin) && (origin[lead] == ' ' || origin[lead] == '\t') {
		lead++
	}
	trailStart := len(origin)
	for trailStart > lead && isWhitespace(origin[trailStart-1]) {
		trailStart--
	}
	return origin[:lead] + replacement + origin[trailStart:]
}
