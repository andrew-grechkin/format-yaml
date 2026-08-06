package render

import (
	"reflect"
	"testing"
)

func TestFindInlineCommentStart(t *testing.T) {
	cases := []struct {
		line string
		want int
	}{
		{"key: value # comment", 11},
		{"key: value  # comment", 12},
		{"key: value", -1},
		{"# head comment", -1},
		{"  # indented head", 2},
		{"", -1},
		{`key: "with # inside" # actual`, 21},
		{`key: 'with # inside' # actual`, 21},
		{`key: "no comment"`, -1},
		{"key: value\t# after tab", 11},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			if got := findInlineCommentStart(tc.line); got != tc.want {
				t.Errorf("findInlineCommentStart(%q) = %d, want %d", tc.line, got, tc.want)
			}
		})
	}
}

func TestExtractInlineKey(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"key: value", "key"},
		{"  nested_key: value", "nested_key"},
		{"    deep: value", "deep"},
		{"key: value # comment", "key"},
		{"", ""},
		{"# just a comment", ""},
		{"no colon here", ""},
		{":leading colon", ""},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			if got := extractInlineKey(tc.line); got != tc.want {
				t.Errorf("extractInlineKey(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}

func TestCollectSourceInlineSpaces(t *testing.T) {
	src := "a: 1    # aligned\nb: 22  # closer\nc: 333 # single\nd: no comment\n"
	want := map[string]int{
		"a": 4,
		"b": 2,
		"c": 1,
	}
	got := collectSourceInlineSpaces([]byte(src))
	if !reflect.DeepEqual(got, want) {
		t.Errorf("collectSourceInlineSpaces(...) = %v, want %v", got, want)
	}
}
