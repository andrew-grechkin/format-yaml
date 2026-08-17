package passes

import (
	"testing"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

func runUnquote(t *testing.T, src string) string {
	t.Helper()
	f := tree.Build([]byte(src))
	Apply(f, UnquoteSafeStrings)
	return string(tree.Emit(f))
}

func TestUnquoteSafeStrings_Unquotes(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"double-quoted plain word", "k: \"hello\"\n", "k: hello\n"},
		{"single-quoted plain word", "k: 'hello'\n", "k: hello\n"},
		{"double-quoted phrase with space", "k: \"hello world\"\n", "k: hello world\n"},
		{"double-quoted key", "\"port\": 9090\n", "port: 9090\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := runUnquote(t, tc.in)
			if got != tc.want {
				t.Fatalf("input: %q\nwant:  %q\ngot:   %q", tc.in, tc.want, got)
			}
		})
	}
}

// Values that would resolve to a non-string when unquoted must keep their quotes: numbers, booleans, null,
// YAML 1.1 booleans (yes/no/on/off variants), sexagesimal patterns, and the empty string.
func TestUnquoteSafeStrings_Preserves(t *testing.T) {
	cases := []string{
		"k: \"123\"\n",       // int
		"k: \"3.14\"\n",      // float
		"k: \"true\"\n",      // bool
		"k: \"null\"\n",      // null
		"k: \"~\"\n",         // null-tilde
		"k: \"yes\"\n",       // YAML 1.1 bool
		"k: \"On\"\n",        // YAML 1.1 bool, mixed case
		"k: \"OFF\"\n",       // YAML 1.1 bool, upper
		"k: \"1:30:00\"\n",   // sexagesimal time (each section 0-59)
		"k: \"\"\n",          // empty string
		"k: \"[a, b]\"\n",    // looks like flow sequence
		"k: \"a: b\"\n",      // colon inside would misparse plain
	}
	for _, in := range cases {
		got := runUnquote(t, in)
		if got != in {
			t.Fatalf("input:%q\nwant: %q (unchanged)\ngot:  %q", in, in, got)
		}
	}
}

// Flow context is stricter: a value like `foo,bar` unquotes fine at block level but stays quoted inside
// a flow container because the bare comma would terminate the scalar.
func TestUnquoteSafeStrings_FlowContextStricter(t *testing.T) {
	// At block level, `hello` unquotes.
	blockIn := "k: \"hello\"\n"
	blockWant := "k: hello\n"
	if got := runUnquote(t, blockIn); got != blockWant {
		t.Fatalf("block: want %q got %q", blockWant, got)
	}
	// Inside a flow mapping, `hello` (no flow indicators) still unquotes.
	flowIn := "{k: \"hello\"}\n"
	flowWant := "{k: hello}\n"
	if got := runUnquote(t, flowIn); got != flowWant {
		t.Fatalf("flow safe: want %q got %q", flowWant, got)
	}
}

// Block scalars are strings but not "explicitly quoted" - the pass leaves them alone.
func TestUnquoteSafeStrings_SkipsBlockScalars(t *testing.T) {
	in := "k: |\n  hello\n"
	got := runUnquote(t, in)
	if got != in {
		t.Fatalf("block scalar: want %q got %q", in, got)
	}
}

// Preserves surrounding whitespace and inline comments on the same line.
func TestUnquoteSafeStrings_PreservesInlineComment(t *testing.T) {
	in := "k: \"hello\" # note\n"
	want := "k: hello # note\n"
	got := runUnquote(t, in)
	if got != want {
		t.Fatalf("inline comment: want %q got %q", want, got)
	}
}
