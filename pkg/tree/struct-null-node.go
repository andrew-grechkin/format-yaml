package tree

// NullNode represents an explicit or implicit YAML null. Distinct type so callers can distinguish "explicit
// null in source" from a Go zero-value. Token is nil for an implicit null (source `key:` with no value); the
// explicit spelling (`null` / `~` / `Null` / `NULL`) lives in Token.Origin and is normalized to `null` by the
// normalizeNulls pass, not tracked as a field.
type NullNode struct {
	ScalarBase
}
