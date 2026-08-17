package tree

// BoolStyle is the lexeme case of a boolean scalar. YAML 1.2 accepts all three; passes that want case
// canonicalization set this field along with rewriting Token.Origin.
type BoolStyle int

const (
	BoolLower BoolStyle = iota // true, false
	BoolTitle                  // True, False
	BoolUpper                  // TRUE, FALSE
)

// BoolNode is a boolean leaf.
type BoolNode struct {
	ScalarBase
	Style BoolStyle
}

func (n *BoolNode) Data() any {
	if n.Token == nil {
		return false
	}
	return parseBoolValue(n.Token.Value)
}
