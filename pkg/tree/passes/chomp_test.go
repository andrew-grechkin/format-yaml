package passes

import (
	"testing"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

func runChomp(t *testing.T, src string) string {
	t.Helper()
	f := tree.Build([]byte(src))
	Apply(f, NarrowBlockChomp)
	return string(tree.Emit(f))
}

// `|+` with exactly one trailing newline becomes `|`. Value round-trips unchanged; only the header shrinks.
func TestNarrowBlockChomp_LiteralKeepAllOneTrailingNewline(t *testing.T) {
	in := "k: |+\n  hello\n"
	want := "k: |\n  hello\n"
	got := runChomp(t, in)
	if got != want {
		t.Fatalf("literal one trailing:\ninput: %q\nwant:  %q\ngot:   %q", in, want, got)
	}
}

// `>+` gets the same treatment as `|+`. Goccy's version only handled literals; we extended to folded for
// symmetry since the chomp modifier means the same thing on both.
func TestNarrowBlockChomp_FoldedKeepAllOneTrailingNewline(t *testing.T) {
	in := "k: >+\n  hello\n"
	want := "k: >\n  hello\n"
	got := runChomp(t, in)
	if got != want {
		t.Fatalf("folded one trailing:\ninput: %q\nwant:  %q\ngot:   %q", in, want, got)
	}
}

// `|+` with two or more trailing newlines needs the `+` to keep the extras; leave alone.
func TestNarrowBlockChomp_LiteralKeepAllTwoTrailingNewlines(t *testing.T) {
	in := "k: |+\n  hello\n\n"
	got := runChomp(t, in)
	if got != in {
		t.Fatalf("multi trailing must stay:\ninput: %q\ngot:   %q", in, got)
	}
}

// Already-minimal chomps (`|`, `|-`, `>`, `>-`) are left alone.
func TestNarrowBlockChomp_AlreadyMinimalUnchanged(t *testing.T) {
	cases := []string{
		"k: |\n  hello\n",
		"k: |-\n  hello\n",
		"k: >\n  hello\n",
		"k: >-\n  hello\n",
	}
	for _, in := range cases {
		got := runChomp(t, in)
		if got != in {
			t.Fatalf("already-minimal must stay:\ninput: %q\ngot:   %q", in, got)
		}
	}
}

// Non-block strings (plain, quoted) have no chomp indicator - the pass ignores them.
func TestNarrowBlockChomp_SkipsNonBlockStrings(t *testing.T) {
	cases := []string{
		"k: hello\n",
		"k: 'hello'\n",
		"k: \"hello\"\n",
	}
	for _, in := range cases {
		got := runChomp(t, in)
		if got != in {
			t.Fatalf("non-block scalar must stay:\ninput: %q\ngot:   %q", in, got)
		}
	}
}
