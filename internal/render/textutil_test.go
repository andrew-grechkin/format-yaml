package render

import "testing"

func TestBlockScalarHeaderIndent(t *testing.T) {
	cases := []struct {
		line string
		want int
	}{
		{"key: |", 0},
		{"key: |-", 0},
		{"key: |+", 0},
		{"key: >", 0},
		{"key: >-", 0},
		{"key: >+", 0},
		{"key: |2", 0},
		{"key: |  # comment", 0},
		{"  nested: |", 2},
		{"    deep: >-", 4},
		{"- |", 0},
		{"- >-", 0},
		{"  - |", 2},
		{"key: value", -1},
		{`key: "value with | inside"`, -1},
		{"key:", -1},
		{"# comment", -1},
		{"", -1},
		{"---", -1},
		{":no-key", -1},
		{"- value", -1},
	}
	for _, tc := range cases {
		t.Run(tc.line, func(t *testing.T) {
			if got := blockScalarHeaderIndent(tc.line); got != tc.want {
				t.Errorf("blockScalarHeaderIndent(%q) = %d, want %d", tc.line, got, tc.want)
			}
		})
	}
}

func TestStripTrailingWhitespace(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty input passes through",
			in:   "",
			want: "",
		},
		{
			name: "trailing spaces stripped on plain lines",
			in:   "a: 1   \nb: 2\t\n",
			want: "a: 1\nb: 2\n",
		},
		{
			name: "trailing spaces stripped on comment lines",
			in:   "# header   \na: 1  # inline   \n",
			want: "# header\na: 1  # inline\n",
		},
		{
			name: "block scalar content preserves trailing whitespace",
			in:   "data: |\n  keep spaces   \n  another line  \n",
			want: "data: |\n  keep spaces   \n  another line  \n",
		},
		{
			name: "header line trailing whitespace stripped but block body preserved",
			in:   "data: |   \n  keep   \nnext: value  \n",
			want: "data: |\n  keep   \nnext: value\n",
		},
		{
			name: "dedent closes block, subsequent lines strip",
			in:   "data: |\n  in block   \nafter: value   \n",
			want: "data: |\n  in block   \nafter: value\n",
		},
		{
			name: "folded block scalar (>) preserves trailing whitespace",
			in:   "data: >\n  folded   \n  content  \n",
			want: "data: >\n  folded   \n  content  \n",
		},
		{
			name: "nested block scalar honours outer indent",
			in:   "outer:\n  inner: |\n    deep   \n  other: value  \n",
			want: "outer:\n  inner: |\n    deep   \n  other: value\n",
		},
		{
			name: "sequence item block scalar preserves content",
			in:   "items:\n  - |\n      first   \n      second  \n  - simple  \n",
			want: "items:\n  - |\n      first   \n      second  \n  - simple\n",
		},
		{
			name: "quoted-string containing | doesn't open a block",
			in:   `key: "has | inside"   ` + "\n",
			want: `key: "has | inside"` + "\n",
		},
		{
			name: "blank line inside block stays blank, next content still in block",
			in:   "data: |\n  first   \n\n  second   \nafter: value  \n",
			want: "data: |\n  first   \n\n  second   \nafter: value\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripTrailingWhitespace(tc.in); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
