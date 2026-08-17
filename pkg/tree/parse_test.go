package tree

import "testing"

// TestBuildEmitRoundTrip pins byte-identical round-tripping for valid YAML shapes that the parser used to get
// wrong. Build + Emit is a pure concatenation of captured Origins, so every known-good source must come back
// exactly as it went in. These cases were all valid YAML that previously parsed incorrectly, crashed, or lost
// comments; keep them here as the regression guard.
func TestBuildEmitRoundTrip(t *testing.T) {
	cases := []string{
		// Explicit key immediately after a document marker (the scanner drops the newline after `---`).
		"---\n? a\n: b\n",
		"---\n? explicit_key\n: value1\nregular: value2\n",
		// A comment on the `key:` line with the value on following lines.
		"a: # c\n  b: 1\n",
		"a: # c\n  - 1\n",
		"a: # c\nb: 2\n",
		// A comment on the `-` line before a nested value.
		"- # c\n  a: 1\n",
		"a:\n  - # c\n    b: 1\n",
		// A comment between `?` and its explicit key.
		"? # c\n  a\n: b\n",
		// Comments inside flow collections.
		"[1, # c\n 2]\n",
		"{a: 1, # c\n b: 2}\n",
		// An island comment (blank-line separated) above a nested value.
		"a:\n  # island\n\n  b: 1\n",
		// Wrapper (anchor/tag) values: same-indent following lines are siblings, not the wrapper's value.
		"a: &x\nb: 1\n",
		"a: !t\nb: 1\n",
		"a: &x\n  b: 1\n",
		"a: !t\n---\nb: 1\n",
		// Wrappers over empty nodes, including inside flow and with an inline comment.
		"[&x]\n",
		"{a: &x}\n",
		"[&x, 1]\n",
		"[&x # c\n, 1]\n",
		"a: &x # c\n  b: 1\n",
		"a: !t # c\n  b: 1\n",
		"&x # c\n  1\n",
		// A comment above a directive keeps its position.
		"# pre\n%YAML 1.2\n---\na: 1\n",
		// An empty block scalar followed by a sibling key.
		"a: |\nb: 1\n",
	}
	for _, src := range cases {
		got := string(Emit(Build([]byte(src))))
		if got != src {
			t.Errorf("round-trip mismatch\nsrc: %q\ngot: %q", src, got)
		}
	}
}
