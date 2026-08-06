package passes

import "testing"

func TestCanSingleQuote(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"hello", true},
		{"it's", true}, // apostrophe is fine, single-quoted YAML doubles it
		{"tab\there", true},
		{"", true},
		{"line1\nline2", false},   // newline forces double-quote
		{"has\x01control", false}, // control char below 0x20 other than tab
		{"has\x7fdel", false},     // DEL
		{"end\rreturn", false},    // \r is a control char
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := canSingleQuote(tc.in); got != tc.want {
				t.Errorf("canSingleQuote(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
