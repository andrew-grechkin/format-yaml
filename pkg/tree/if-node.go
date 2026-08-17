package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// Node is the common interface every tree node satisfies.
//   - Data returns the semantic content: for a scalar leaf it's the parsed value; for structural nodes
//     it's the recursive Go representation (map/slice/nil).
//   - ToString renders the node back to source bytes.
//   - AppendNewline appends one `\n` to the node's rendered tail. Each concrete type knows where its
//     "tail" lives (a leaf's Token, a container's last child, a wrapper's value); callers use this to
//     insert blank-line separators without needing to walk into the subtree themselves.
type Node interface {
	Data() any
	ToString() string
	AppendNewline()
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
