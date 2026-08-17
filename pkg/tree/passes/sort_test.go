package passes

import (
	"testing"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

func runSort(t *testing.T, src string) string {
	t.Helper()
	f := tree.Build([]byte(src))
	Apply(f, SortMappingKeys)
	return string(tree.Emit(f))
}

func TestSortMappingKeys_TopLevel(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"reverse order becomes alphabetical",
			"c: 3\nb: 2\na: 1\n",
			"a: 1\nb: 2\nc: 3\n",
		},
		{
			"already sorted stays put",
			"a: 1\nb: 2\nc: 3\n",
			"a: 1\nb: 2\nc: 3\n",
		},
		{
			"mixed order",
			"b: 2\na: 1\nc: 3\n",
			"a: 1\nb: 2\nc: 3\n",
		},
		{
			"case-sensitive: uppercase sorts before lowercase",
			"a: 1\nA: 2\n",
			"A: 2\na: 1\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runSort(t, tc.in)
			if got != tc.want {
				t.Fatalf("mismatch\ninput:  %q\nwant:   %q\ngot:    %q", tc.in, tc.want, got)
			}
		})
	}
}

// Anchors in a mapping's subtree freeze that mapping's order - alphabetization can move an alias before its
// anchor, breaking the reference chain. Both the mapping containing the anchor and any ancestor mapping stay
// put; sortMappingKeys detects anchors anywhere in the subtree.
func TestSortMappingKeys_SkipsMappingWithAnchor(t *testing.T) {
	in := "z: &anc value\na: other\n"
	want := "z: &anc value\na: other\n"
	got := runSort(t, in)
	if got != want {
		t.Fatalf("anchor should freeze mapping\nwant: %q\ngot:  %q", want, got)
	}
}

func TestSortMappingKeys_SkipsMappingWithAlias(t *testing.T) {
	in := "z: &anc value\na: *anc\n"
	got := runSort(t, in)
	if got != in {
		t.Fatalf("alias should freeze mapping\nwant: %q\ngot:  %q", in, got)
	}
}

// Nested-anchor case: outer mapping's subtree contains an anchor, so outer stays put.
func TestSortMappingKeys_NestedAnchorFreezesAncestor(t *testing.T) {
	in := "zebra:\n  a: &anc 1\napple:\n  b: 2\n"
	got := runSort(t, in)
	if got != in {
		t.Fatalf("nested anchor should freeze ancestor\nwant: %q\ngot:  %q", in, got)
	}
}

// Flow style ({a: 1, b: 2}) is left alone: single-line output makes a sort awkward, and flow order in JSON-
// compatible input can be intentional.
func TestSortMappingKeys_SkipsFlow(t *testing.T) {
	in := "{b: 2, a: 1}\n"
	got := runSort(t, in)
	if got != in {
		t.Fatalf("flow mapping should be left alone\nwant: %q\ngot:  %q", in, got)
	}
}

// A comment immediately preceding the mapping (which the tree attaches to MappingNode.PrecedingComment)
// should ride along with the entry it precedes in source. Sort promotes it to the first entry's
// PrecedingComment before reordering so that entry keeps the comment as it moves.
func TestSortMappingKeys_TransfersMappingCommentToFirstEntry(t *testing.T) {
	in := "# sticky comment\nz: 1\na: 2\n"
	want := "a: 2\n# sticky comment\nz: 1\n"
	got := runSort(t, in)
	if got != want {
		t.Fatalf("mapping-level comment should follow its entry through sort\nwant: %q\ngot:  %q", want, got)
	}
}

// Under the tokenizer's leading-indent model each entry owns its own leading indent, so same-depth sibling
// reorders (including reorders inside nested mappings) don't leave stale indent behind - no follow-up
// NormalizeIndent needed. This test exercises that property with sort alone.
func TestSortMappingKeys_NestedSiblingsIndentSafe(t *testing.T) {
	in := "outer:\n  z: 1\n  a: 2\ninner:\n  y: 3\n  x: 4\n"
	want := "inner:\n  x: 4\n  y: 3\nouter:\n  a: 2\n  z: 1\n"
	got := runSort(t, in)
	if got != want {
		t.Fatalf("sort alone on nested:\ninput: %q\nwant:  %q\ngot:   %q", in, want, got)
	}
}

// Empty mapping is a no-op edge case: nothing to sort, no PrecedingComment to transfer.
func TestSortMappingKeys_EmptyIsNoop(t *testing.T) {
	f := tree.Build([]byte("{}\n"))
	Apply(f, SortMappingKeys)
	got := string(tree.Emit(f))
	if got != "{}\n" {
		t.Fatalf("empty flow mapping mismatch: want %q, got %q", "{}\n", got)
	}
}
