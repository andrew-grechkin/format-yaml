package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// Node is the common interface every tree node satisfies: it renders itself back to source bytes. Structural
// traversal helpers (Walk) and tail operations (AppendNewline, TrimNewline, TrailingNewlines) are free
// functions over Node rather than interface methods, so node types stay lean and the tail logic lives in one
// place.
type Node interface {
	ToString() string
}

// TrailingNewlines counts the run of `\n` bytes at the very end of n's rendered output. Used together
// with AppendNewline in idempotent "ensure N trailing newlines" loops without exposing internal token
// structure to callers.
func TrailingNewlines(n Node) int {
	s := n.ToString()
	count := 0
	for i := len(s) - 1; i >= 0 && s[i] == '\n'; i-- {
		count++
	}
	return count
}

// TokenOf returns the primary Token of any Scalar node, nil for non-scalar/nil inputs. Convenience wrapper
// over the Scalar interface for passes that want the token without a full type assertion at the call site.
func TokenOf(n Node) *token.Token {
	if s, ok := n.(Scalar); ok {
		return s.ScalarToken()
	}
	return nil
}
