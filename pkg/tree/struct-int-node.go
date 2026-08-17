package tree

// IntStyle is the base representation of an integer scalar. Passes that want to canonicalize integer form
// (e.g. force decimal) read/write this field along with Token.Origin.
type IntStyle int

const (
	IntDecimal IntStyle = iota // 42
	IntHex                     // 0xff
	IntOctal                   // 0o17
	IntBinary                  // 0b1010
)

// IntNode is an integer leaf.
type IntNode struct {
	ScalarBase
	Style IntStyle
}

func (n *IntNode) Data() any {
	if n.Token == nil {
		return int64(0)
	}
	return parseIntValue(n.Token.Value)
}
