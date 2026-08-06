package render

import (
	"reflect"
	"testing"
)

func TestIsTopLevelLine(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"key: value", true},
		{"container:", true},
		{"a: A", true},
		{"", false},
		{"  indented: value", false},
		{"\tindented: value", false},
		{"# comment", false},
		{"---", false},
		{"--- content", false},
		{"...", false},
		{"... trailing", false},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			if got := isTopLevelLine(tc.line); got != tc.want {
				t.Errorf("isTopLevelLine(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

func TestIsCommentLine(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"# comment", true},
		{"#no space", true},
		{"", false},
		{"  # indented", false}, // isCommentLine looks at column 0
		{"key: # trailing", false},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			if got := isCommentLine(tc.line); got != tc.want {
				t.Errorf("isCommentLine(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}

func TestCountSourceBlanks(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []int
	}{
		{"no blanks", "a: 1\nb: 2\nc: 3\n", []int{0, 0}},
		{"one blank between", "a: 1\nb: 2\n\nc: 3\n", []int{0, 1}},
		{"multiple blanks preserved as count", "a: 1\n\n\n\nb: 2\n", []int{3}},
		{"single entry gives nil", "only: entry\n", nil},
		{"head comment counts toward entry group start", "a: 1\n\n# comment\nb: 2\n", []int{1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := countSourceBlanks([]byte(tc.src))
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("countSourceBlanks(%q) = %v, want %v", tc.src, got, tc.want)
			}
		})
	}
}
