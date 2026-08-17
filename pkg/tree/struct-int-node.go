package tree

// IntNode is an integer leaf. The source spelling (decimal / hex / octal / binary) lives in Token.Origin and
// is preserved verbatim; no style enum is tracked because no pass rewrites integer bases.
type IntNode struct {
	ScalarBase
}
