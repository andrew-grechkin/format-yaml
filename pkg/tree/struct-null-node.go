package tree

// NullStyle is the surface form of a null scalar. NullImplicit means the source had no value at all (`key:`
// with a trailing newline and nothing after `:`); the other four are explicit lexemes.
type NullStyle int

const (
	NullLower    NullStyle = iota // null
	NullTilde                     // ~
	NullTitle                     // Null
	NullUpper                     // NULL
	NullImplicit                  // (source `key:` with no value)
)

// NullNode represents an explicit or implicit YAML null. Distinct type so callers can distinguish "explicit
// null in source" from a Go zero-value. Token may be nil for NullImplicit.
type NullNode struct {
	ScalarBase
	Style NullStyle
}

func (*NullNode) Data() any { return nil }
