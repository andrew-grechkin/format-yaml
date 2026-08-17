package passes

import (
	"testing"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// Round-trips src through Build -> Apply(NormalizeNulls) -> Emit and returns the emitted bytes as a string.
// Keeps every test case a two-argument comparison (input vs expected) with no fixture files - the tokenizer and
// tree already have their own fixture-based integration tests; this file's job is only to assert what the
// nulls pass changes.
func runNulls(t *testing.T, src string) string {
	t.Helper()
	f := tree.Build([]byte(src))
	Apply(f, NormalizeNulls)
	return string(tree.Emit(f))
}

func TestNormalizeNulls(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"tilde rewrites to null", "b: ~\n", "b: null\n"},
		{"capitalized rewrites to null", "b: Null\n", "b: null\n"},
		{"all-caps rewrites to null", "b: NULL\n", "b: null\n"},
		{"already null unchanged", "a: null\n", "a: null\n"},
		{"quoted null preserved as string", "d: \"null\"\n", "d: \"null\"\n"},
		{"multiple entries mixed", "a: null\nb: ~\ne: Null\n", "a: null\nb: null\ne: null\n"},
		{"tilde in sequence", "- ~\n- null\n", "- null\n- null\n"},
		{"top-level tilde", "~\n", "null\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runNulls(t, tc.in)
			if got != tc.want {
				t.Fatalf("mismatch\ninput:    %q\nwant:     %q\ngot:      %q", tc.in, tc.want, got)
			}
		})
	}
}

// Implicit null (source `key:\n` with no value) is a separate pass from NormalizeNulls - it needs both
// bits to convert `key:\n` into `key: null\n`. NormalizeNulls alone leaves implicit as-is.
func TestNormalizeNulls_ImplicitAloneIsNoop(t *testing.T) {
	in := "c:\n"
	got := runNulls(t, in)
	if got != in {
		t.Fatalf("implicit null with NormalizeNulls alone should be unchanged\nwant: %q\ngot:  %q", in, got)
	}
}

// With both passes enabled, implicit null becomes explicit `null` with the colon's trailing whitespace
// promoted onto the new token so layout stays correct.
func TestMaterializeImplicitNulls(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"trailing newline only", "c:\n", "c: null\n"},
		{"nested indent", "outer:\n  inner:\n", "outer:\n  inner: null\n"},
		{"among siblings", "a:\nb: 1\n", "a: null\nb: 1\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := tree.Build([]byte(tc.in))
			Apply(f, NormalizeNulls|MaterializeImplicitNulls)
			got := string(tree.Emit(f))
			if got != tc.want {
				t.Fatalf("implicit null:\ninput: %q\nwant:  %q\ngot:   %q", tc.in, tc.want, got)
			}
		})
	}
}

// Set(0) is a no-op: the file emits identically to source. Guards against Apply doing anything unexpected on
// an empty set.
func TestApply_EmptySetIsNoop(t *testing.T) {
	src := "a: ~\nb: null\n"
	f := tree.Build([]byte(src))
	Apply(f, 0)
	got := string(tree.Emit(f))
	if got != src {
		t.Fatalf("empty set changed output: want %q, got %q", src, got)
	}
}
