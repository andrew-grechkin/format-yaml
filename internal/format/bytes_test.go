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

// TestBytesIdempotentOnTrickyValidYAML guards the composed pipeline against the class of bugs that motivated
// the tree rewrite: valid YAML that the old goccy-parser pipeline mangled. For each known-good source and every
// mode, the first pass must succeed and a second pass over its own output must be byte-identical. The sources
// are deliberately stored inline (rather than as fixtures) so the cause of a regression is obvious.
func TestBytesIdempotentOnTrickyValidYAML(t *testing.T) {
	sources := []string{
		"---\n? a\n: b\n",
		"---\n? explicit_key\n: value1\nregular: value2\n",
		"a: # c\n  b: 1\n",
		"a: # c\n  - 1\n",
		"a: # c\nb: 2\n",
		"- # c\n  a: 1\n",
		"a:\n  - # c\n    b: 1\n",
		"? # c\n  a\n: b\n",
		"[1, # c\n 2]\n",
		"{a: 1, # c\n b: 2}\n",
		"a:\n  # island\n\n  b: 1\n",
		"a: &x\nb: 1\n",
		"a: !t\nb: 1\n",
		"a: &x\n  b: 1\n",
		"a: !t\n---\nb: 1\n",
		"[&x]\n",
		"{a: &x}\n",
		"[&x, 1]\n",
		"a: &x # c\n  b: 1\n",
		"a: !t # c\n  b: 1\n",
		"# pre\n%YAML 1.2\n---\na: 1\n",
		"a: |\nb: 1\n",
		"key:\n  - 1 # c\n  - 2\n",
		"key:\n  - null # c\n  - 2\n",
	}
	modes := []config.Mode{config.ModeMinimal, config.ModeStandard, config.ModeFull, config.ModePedantic}
	for _, src := range sources {
		for _, m := range modes {
			first, err := Bytes([]byte(src), m)
			if err != nil {
				t.Errorf("mode=%d parse %q: %v", m, src, err)
				continue
			}
			second, err := Bytes(first, m)
			if err != nil {
				t.Errorf("mode=%d reparse of own output for %q: %v\noutput: %q", m, src, err, first)
				continue
			}
			if !bytes.Equal(first, second) {
				t.Errorf("mode=%d not idempotent for %q\nfirst:  %q\nsecond: %q", m, src, first, second)
			}
		}
	}
}

