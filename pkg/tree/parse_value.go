// Parsers for scalar node Data() methods. Called from the typed nodes so each type's Data() returns a
// properly-typed Go value (int64/float64/bool/string). Parse failures fall back to a sensible zero rather
// than propagating an error, because Data() is a value accessor and the tokenizer has already classified
// the Token by type - a failure here would be a bug in that classification, not a caller error.
package tree

import (
	"math"
	"strconv"
	"strings"

	"github.com/andrew-grechkin/format-yaml/internal/token"
)

func parseIntValue(s string) int64 {
	body := s
	neg := false
	if strings.HasPrefix(body, "-") {
		neg = true
		body = body[1:]
	} else if strings.HasPrefix(body, "+") {
		body = body[1:]
	}
	base := 10
	switch {
	case strings.HasPrefix(body, "0x"), strings.HasPrefix(body, "0X"):
		base = 16
		body = body[2:]
	case strings.HasPrefix(body, "0o"), strings.HasPrefix(body, "0O"):
		base = 8
		body = body[2:]
	case strings.HasPrefix(body, "0b"), strings.HasPrefix(body, "0B"):
		base = 2
		body = body[2:]
	}
	v, err := strconv.ParseInt(body, base, 64)
	if err != nil {
		return 0
	}
	if neg {
		return -v
	}
	return v
}

func parseFloatValue(t *token.Token) float64 {
	switch t.Type {
	case token.InfinityType:
		if strings.HasPrefix(t.Value, "-") {
			return math.Inf(-1)
		}
		return math.Inf(1)
	case token.NanType:
		return math.NaN()
	}
	v, err := strconv.ParseFloat(t.Value, 64)
	if err != nil {
		return 0
	}
	return v
}

func parseBoolValue(s string) bool {
	return strings.EqualFold(s, "true")
}
