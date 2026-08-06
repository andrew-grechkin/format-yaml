package encoding

import (
	"bytes"
	"strings"
	"testing"
)

// Confirms UTF-8 fast path: the leading BOM is stripped and every other byte survives unchanged.
func TestNormalizeToUTF8UTF8Passthrough(t *testing.T) {
	src := []byte("a: 1\n")
	out, err := NormalizeToUTF8(src)
	if err != nil {
		t.Fatalf("NormalizeToUTF8: %v", err)
	}
	if !bytes.Equal(out, src) {
		t.Errorf("plain UTF-8 changed: got %q, want %q", out, src)
	}

	withBOM := append(append([]byte{}, BOMUTF8...), src...)
	out, err = NormalizeToUTF8(withBOM)
	if err != nil {
		t.Fatalf("NormalizeToUTF8 (BOM): %v", err)
	}
	if bytes.HasPrefix(out, BOMUTF8) {
		t.Errorf("UTF-8 BOM survived: %q", out)
	}
	if !bytes.Equal(out, src) {
		t.Errorf("UTF-8 body changed: got %q, want %q", out, src)
	}
}

// Truncated UTF-16/UTF-32 payloads must error, not silently produce garbage.
func TestNormalizeToUTF8Malformed(t *testing.T) {
	cases := []struct {
		name string
		src  []byte
		want string
	}{
		{"utf-16-be odd length", append([]byte{0xFE, 0xFF}, 'a'), "odd byte length"},
		{"utf-16-le odd length", append([]byte{0xFF, 0xFE}, 'a'), "odd byte length"},
		{"utf-32-be misaligned", append([]byte{0x00, 0x00, 0xFE, 0xFF}, 'a', 'b'), "not a multiple of 4"},
		{"utf-32-le misaligned", append([]byte{0xFF, 0xFE, 0x00, 0x00}, 'a', 'b'), "not a multiple of 4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NormalizeToUTF8(tc.src)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}
