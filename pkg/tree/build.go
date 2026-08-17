// Package tree defines our own YAML AST built directly from the token stream produced by internal/lexer.
// The tree is shaped by the document grammar rather than by any parser library: File holds docs and
// stream-level comments, each Doc holds an optional header, a body node and an optional footer, and body
// nodes recurse through mappings, sequences, wrappers and scalars.
//
// Build is a recursive-descent parser (see parse*.go). Comment attribution is driven by physical source
// layout and lives entirely in commentBuffer (parse-comments.go).
//
// Tokens arrive with the lexer's "trailing-only" Origin invariant already applied (see
// lexer.TokenizeNormalized), so emission is just Origin concatenation (see emit.go). Unsupported input
// panics with a stable "tree.Build: " message - deliberate, so gaps surface immediately during
// development instead of silently dropping a subtree.
package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/lexer"
)

// Build parses src and returns the stream root.
func Build(src []byte) *File {
	return (&parser{tokens: lexer.TokenizeNormalized(string(src))}).parseFile()
}
