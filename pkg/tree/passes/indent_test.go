package passes

import (
	"testing"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

func runIndent(t *testing.T, src string) string {
	t.Helper()
	f := tree.Build([]byte(src))
	Apply(f, NormalizeIndent)
	return string(tree.Emit(f))
}

func TestNormalizeIndent_TopLevelUnchanged(t *testing.T) {
	// Flat mapping at column 1: nothing to indent.
	src := "a: 1\nb: 2\n"
	got := runIndent(t, src)
	if got != src {
		t.Fatalf("flat mapping should be unchanged\nwant: %q\ngot:  %q", src, got)
	}
}

func TestNormalizeIndent_NestedMappingShrinks(t *testing.T) {
	// Source uses 4-space indent, pass canonicalizes to 2.
	in := "outer:\n    a: 1\n    b: 2\n"
	want := "outer:\n  a: 1\n  b: 2\n"
	got := runIndent(t, in)
	if got != want {
		t.Fatalf("indent shrink:\ninput: %q\nwant:  %q\ngot:   %q", in, want, got)
	}
}

func TestNormalizeIndent_NestedMappingExpands(t *testing.T) {
	// Source uses 1-space indent (unusual but valid), pass canonicalizes to 2.
	in := "outer:\n a: 1\n b: 2\n"
	want := "outer:\n  a: 1\n  b: 2\n"
	got := runIndent(t, in)
	if got != want {
		t.Fatalf("indent expand:\ninput: %q\nwant:  %q\ngot:   %q", in, want, got)
	}
}

func TestNormalizeIndent_DeeplyNested(t *testing.T) {
	in := "a:\n    b:\n        c: 1\n"
	want := "a:\n  b:\n    c: 1\n"
	got := runIndent(t, in)
	if got != want {
		t.Fatalf("deep nest:\ninput: %q\nwant:  %q\ngot:   %q", in, want, got)
	}
}

// Flow mappings preserve their single-line layout regardless of indent normalization at the block level.
func TestNormalizeIndent_FlowLeftAlone(t *testing.T) {
	in := "a: {b: 1, c: 2}\n"
	got := runIndent(t, in)
	if got != in {
		t.Fatalf("flow should be unchanged\nwant: %q\ngot:  %q", in, got)
	}
}

// Idempotence: two runs produce the same output as one.
func TestNormalizeIndent_Idempotent(t *testing.T) {
	src := "outer:\n    a: 1\n    b:\n        c: 2\n"
	once := runIndent(t, src)
	f := tree.Build([]byte(once))
	Apply(f, NormalizeIndent)
	twice := string(tree.Emit(f))
	if once != twice {
		t.Fatalf("not idempotent\nonce:  %q\ntwice: %q", once, twice)
	}
}

// Combined pipeline: sort + indent produces clean nested-mapping sort output. With leading indent on each
// child, sort alone would suffice for same-depth reorders, but indent still catches any residual source
// artifacts and keeps the pipeline output canonical regardless of input formatting.
func TestApply_SortThenIndent_NestedMapping(t *testing.T) {
	in := "outer:\n  z: 1\n  a: 2\ninner:\n  y: 3\n  x: 4\n"
	want := "inner:\n  x: 4\n  y: 3\nouter:\n  a: 2\n  z: 1\n"
	f := tree.Build([]byte(in))
	Apply(f, SortMappingKeys|NormalizeIndent)
	got := string(tree.Emit(f))
	if got != want {
		t.Fatalf("sort+indent:\ninput: %q\nwant:  %q\ngot:   %q", in, want, got)
	}
}
