package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// ScalarBase is the shared body of every scalar leaf: a source Token plus the Comments slots. Embedded by
// StringNode / IntNode / FloatNode / BoolNode / NullNode. The scalar() sentinel method is what makes a type
// satisfy the Scalar interface below.
type ScalarBase struct {
	Comments
	Token *token.Token
}

func (*ScalarBase) scalar() {}

// ScalarToken returns the primary token of any scalar-embedding node. Used via the Scalar interface below by
// passes that want "the token of any scalar" without caring which typed kind it is.
func (b *ScalarBase) ScalarToken() *token.Token { return b.Token }

// ContentToken returns the primary Token for scalar leaves. Same signature as DocHeader/DocFooter so callers
// can use the Tokened interface without caring which kind of leaf they've got.
func (b *ScalarBase) ContentToken() *token.Token { return b.Token }
