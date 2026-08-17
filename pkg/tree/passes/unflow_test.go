package passes

import (
	"testing"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

func runUnflow(t *testing.T, src string) string {
	t.Helper()
	f := tree.Build([]byte(src))
	// Combine with NormalizeIndent because unflow leaves leading-indent finalization to that pass.
	Apply(f, UnflowTopLevel|NormalizeIndent)
	return string(tree.Emit(f))
}

func TestUnflowTopLevel_Mapping(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"simple two-entry mapping", "{a: 1, b: 2}\n", "a: 1\nb: 2\n"},
		{"quoted keys/values (JSON-like)", "{\"a\": \"A\", \"b\": \"B\"}\n", "\"a\": \"A\"\n\"b\": \"B\"\n"},
		{"single entry", "{a: 1}\n", "a: 1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runUnflow(t, tc.in)
			if got != tc.want {
				t.Fatalf("input:%q\nwant: %q\ngot:  %q", tc.in, tc.want, got)
			}
		})
	}
}

func TestUnflowTopLevel_Sequence(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"simple flat sequence", "[1, 2, 3]\n", "- 1\n- 2\n- 3\n"},
		{"single item", "[a]\n", "- a\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runUnflow(t, tc.in)
			if got != tc.want {
				t.Fatalf("input:%q\nwant: %q\ngot:  %q", tc.in, tc.want, got)
			}
		})
	}
}

// Nested flow constructs stay in flow: unflowTopLevel only touches the outermost body.
func TestUnflowTopLevel_LeavesNestedAlone(t *testing.T) {
	in := "a: [1, 2, 3]\n"
	got := runUnflow(t, in)
	if got != in {
		t.Fatalf("nested flow must stay:\ninput:%q\ngot:  %q", in, got)
	}
}

// Already-block containers are unchanged.
func TestUnflowTopLevel_LeavesBlockAlone(t *testing.T) {
	in := "a: 1\nb: 2\n"
	got := runUnflow(t, in)
	if got != in {
		t.Fatalf("block must stay:\ninput:%q\ngot:  %q", in, got)
	}
}
