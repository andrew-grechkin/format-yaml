package tree

// FloatNode is a float leaf, including infinity and NaN variants. The exact lexeme lives in Token.Origin.
type FloatNode struct {
	ScalarBase
}
