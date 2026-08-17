package passes

import (
	"testing"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

func runQuotes(t *testing.T, src string) string {
	t.Helper()
	f := tree.Build([]byte(src))
	Apply(f, PreferSingleQuotes)
	return string(tree.Emit(f))
}

func TestPreferSingleQuotes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain double becomes single", "k: \"hello\"\n", "k: 'hello'\n"},
		{"already single unchanged", "k: 'hello'\n", "k: 'hello'\n"},
		{"plain scalar unchanged", "k: hello\n", "k: hello\n"},
		{"embedded single quote gets doubled", "k: \"can't\"\n", "k: 'can''t'\n"},
		{"multiple entries", "a: \"one\"\nb: \"two\"\n", "a: 'one'\nb: 'two'\n"},
		{"empty double becomes empty single", "k: \"\"\n", "k: ''\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runQuotes(t, tc.in)
			if got != tc.want {
				t.Fatalf("input:%q\nwant: %q\ngot:  %q", tc.in, tc.want, got)
			}
		})
	}
}

// Double-quoted values containing escape sequences that resolve to characters unavailable in single-quoted
// form (embedded newline) must stay double-quoted.
func TestPreferSingleQuotes_SkipsUnsafeContent(t *testing.T) {
	in := "k: \"foo\\nbar\"\n" // \n in source -> real newline in value -> can't single-quote
	got := runQuotes(t, in)
	if got != in {
		t.Fatalf("input:%q\nwant: %q (unchanged)\ngot:  %q", in, in, got)
	}
}

// Block scalars (`|`, `>`) are strings but the pass shouldn't touch them - Style is StringLiteral / StringFolded,
// not StringDoubleQuoted.
func TestPreferSingleQuotes_SkipsBlockScalars(t *testing.T) {
	in := "k: |\n  hello\n"
	got := runQuotes(t, in)
	if got != in {
		t.Fatalf("input:%q\nwant: %q (unchanged)\ngot:  %q", in, in, got)
	}
}

// Preserves inline comments on the same line as the value.
func TestPreferSingleQuotes_PreservesInlineComment(t *testing.T) {
	in := "k: \"hello\" # note\n"
	want := "k: 'hello' # note\n"
	got := runQuotes(t, in)
	if got != want {
		t.Fatalf("input:%q\nwant: %q\ngot:  %q", in, want, got)
	}
}
