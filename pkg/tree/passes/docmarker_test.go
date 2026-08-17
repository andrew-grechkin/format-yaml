package passes

import (
	"testing"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

func runDocMarker(t *testing.T, src string) string {
	t.Helper()
	f := tree.Build([]byte(src))
	Apply(f, EnsureDocMarker)
	return string(tree.Emit(f))
}

// Source with no `---` gets one prepended.
func TestEnsureDocMarker_AddsMissing(t *testing.T) {
	in := "a: 1\n"
	want := "---\na: 1\n"
	got := runDocMarker(t, in)
	if got != want {
		t.Fatalf("input:%q\nwant: %q\ngot:  %q", in, want, got)
	}
}

// Source that already has `---` is unchanged.
func TestEnsureDocMarker_PreservesExisting(t *testing.T) {
	in := "---\na: 1\n"
	got := runDocMarker(t, in)
	if got != in {
		t.Fatalf("input:%q\nwant unchanged\ngot: %q", in, got)
	}
}

// A pre-doc file-level comment (no `---` in source) gets hoisted INSIDE the doc, so the emitted layout
// is `---\n# comment\nbody` rather than `# comment\n---\nbody`.
func TestEnsureDocMarker_HoistsPrecedingComment(t *testing.T) {
	in := "# header comment\na: 1\n"
	want := "---\n# header comment\na: 1\n"
	got := runDocMarker(t, in)
	if got != want {
		t.Fatalf("input:%q\nwant: %q\ngot:  %q", in, want, got)
	}
}

// Multi-doc source where one doc has `---` and the other doesn't: only the header-less doc gets a
// synthetic marker; the other stays as source.
func TestEnsureDocMarker_MultiDocMixed(t *testing.T) {
	in := "a: 1\n---\nb: 2\n"
	want := "---\na: 1\n---\nb: 2\n"
	got := runDocMarker(t, in)
	if got != want {
		t.Fatalf("input:%q\nwant: %q\ngot:  %q", in, want, got)
	}
}
