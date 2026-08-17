package tree

// FloatStyle is the surface form of a float scalar.
type FloatStyle int

const (
	FloatRegular  FloatStyle = iota // 3.14
	FloatInfinity                   // .inf / -.inf
	FloatNaN                        // .nan
)

// FloatNode is a float leaf, including infinity and NaN variants.
type FloatNode struct {
	ScalarBase
	Style FloatStyle
}

func (n *FloatNode) Data() any {
	if n.Token == nil {
		return float64(0)
	}
	return parseFloatValue(n.Token)
}
