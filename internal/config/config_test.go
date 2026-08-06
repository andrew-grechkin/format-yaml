package config

import "testing"

func TestParseMode(t *testing.T) {
	cases := []struct {
		in      string
		want    Mode
		wantErr bool
	}{
		{"", ModeStandard, false},
		{"standard", ModeStandard, false},
		{"minimal", ModeMinimal, false},
		{"full", ModeFull, false},
		{"pedantic", ModePedantic, false},
		{"nope", 0, true},
		{"Standard", 0, true}, // case-sensitive
		{"MINIMAL", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseMode(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseMode(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("ParseMode(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseLineWidth(t *testing.T) {
	cases := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"", DefaultLineWidth, false},
		{"80", 80, false},
		{"200", 200, false},
		{"1", 1, false},
		{"0", 0, true},
		{"-1", 0, true},
		{"abc", 0, true},
		{"80.5", 0, true},
		{" 80 ", 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseLineWidth(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseLineWidth(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("ParseLineWidth(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
