package format

import (
	"bytes"
	"testing"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// Smoke test that the composed pipeline (encoding + parse + passes + render) actually produces output for the trivial
// case. Every heavier test lives in the root package's main_test.go; this one exists so `go test ./...` doesn't leave
// the format package unexercised.
func TestBytesSmoke(t *testing.T) {
	src := "a: 1\nb: 2\n"
	out, err := Bytes([]byte(src), config.ModeStandard)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if !bytes.HasPrefix(out, []byte("---\n")) {
		t.Errorf("expected doc header in output, got %q", out)
	}
	// Idempotence guard: the composition must round-trip.
	out2, err := Bytes(out, config.ModeStandard)
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if !bytes.Equal(out, out2) {
		t.Errorf("not idempotent\nfirst:\n%s\nsecond:\n%s", out, out2)
	}
}
