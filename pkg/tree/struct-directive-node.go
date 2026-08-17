package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// DirectiveNode is a stream-level `%NAME args` directive (`%YAML 1.2`, `%TAG !e! tag:...`). Lives in
// File.Children, not inside any Doc. Data() returns nil - directives affect parsing, not semantic content.
type DirectiveNode struct {
	Name *token.Token
	Args []*token.Token
}

func (*DirectiveNode) Data() any { return nil }
