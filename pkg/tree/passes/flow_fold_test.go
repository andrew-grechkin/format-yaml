package passes

import (
	"testing"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

func runFlowFold(t *testing.T, src string) string {
	t.Helper()
	f := tree.Build([]byte(src))
	Apply(f, FlowShortBlocks)
	return string(tree.Emit(f))
}

// A plain block sequence under a key folds to inline flow.
func TestFlowShortBlocks_FoldsPlainSequence(t *testing.T) {
	in := "key:\n  - a\n  - b\n"
	want := "key: [a, b]\n"
	if got := runFlowFold(t, in); got != want {
		t.Fatalf("want %q, got %q", want, got)
	}
}

// Any item carrying a comment must stop the fold: comments rendered inside flow would leak onto the wrong
// element (and `#` would comment out the rest of the line). This covers non-string leaves too - the check is
// on every Commented scalar, not just strings.
func TestFlowShortBlocks_SkipsCommentedItems(t *testing.T) {
	unchanged := []string{
		"key:\n  - 1 # c\n  - 2\n",
		"key:\n  - null # c\n  - 2\n",
		"key:\n  - true # c\n  - false\n",
		"key:\n  - a # c\n  - b\n",
		"key:\n  # leading\n  - a\n  - b\n",
	}
	for _, in := range unchanged {
		if got := runFlowFold(t, in); got != in {
			t.Fatalf("commented sequence must not fold\ninput: %q\ngot:   %q", in, got)
		}
	}
}
