package tree

import (
	"fmt"

	tokenizer "github.com/andrew-grechkin/format-yaml/internal/token"
)

// parser is a recursive-descent builder for File. It mirrors the YAML stream grammar directly:
//
//	File  -> (Directive | Doc)*
//	Doc   -> Header? Node? Footer?
//	Node  -> Scalar | BlockMapping | BlockSequence | FlowMapping | FlowSequence | Anchor | Tag | Alias
//
// Every production owns the tokens it consumes and recursively builds its children, so there is no
// central dispatch table and no cross-container bookkeeping beyond the commentBuffer (see
// parse-comments.go), which all productions share through the parser.
type parser struct {
	tokens   tokenizer.Tokens
	idx      int
	comments commentBuffer
}

// peek returns the current token, or nil at end-of-stream. Never panics - every caller must treat nil
// as "no more input".
func (p *parser) peek() *tokenizer.Token {
	if p.idx >= len(p.tokens) {
		return nil
	}
	return p.tokens[p.idx]
}

// peekAt returns the token n positions ahead (peekAt(0) == peek()), or nil past end-of-stream.
func (p *parser) peekAt(n int) *tokenizer.Token {
	if p.idx+n >= len(p.tokens) {
		return nil
	}
	return p.tokens[p.idx+n]
}

// next consumes and returns the current token. At end-of-stream it returns nil and keeps the cursor at
// the end, so callers can loop until peek() reports nil without an extra bounds check.
func (p *parser) next() *tokenizer.Token {
	t := p.peek()
	p.idx++
	return t
}

func (p *parser) eof() bool { return p.idx >= len(p.tokens) }

// fail aborts the build with a user-facing parse error. Messages are part of the CLI contract (pinned
// by test/tree/fail-*.err), hence the stable "tree.Build: " prefix.
func (p *parser) fail(format string, args ...any) {
	panic(fmt.Sprintf("tree.Build: "+format, args...))
}
