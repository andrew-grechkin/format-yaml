package tree

import (
	"github.com/andrew-grechkin/format-yaml/internal/token"
)

// Scalar is the sentinel + accessor interface every scalar leaf implements. Passes that want to handle any
// scalar polymorphically ("give me the token of whatever scalar this is") take a Scalar instead of listing
// the five concrete types.
type Scalar interface {
	Node
	scalar()
	ScalarToken() *token.Token
}
