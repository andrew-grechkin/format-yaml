package tree

// BoolNode is a boolean leaf. The lexeme case (`true` / `True` / `TRUE`) lives in Token.Origin and is preserved
// verbatim; no case enum is tracked because no pass canonicalizes it.
type BoolNode struct {
	ScalarBase
}
