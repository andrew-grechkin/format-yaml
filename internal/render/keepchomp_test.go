package render

import (
	"testing"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// The keep-chomp inspection helpers must fire for both `|+` and `>+` tails, and only for those.
func TestKeepChompInspections(t *testing.T) {
	cases := []struct {
		name       string
		src        string
		wantTail   bool
		wantLastEn bool
	}{
		{"top-level |+ tail", "key: |+\n  content\n\n\n", true, true},
		{"top-level >+ tail", "key: >+\n  content\n\n\n", true, true},
		{"|+ followed by sibling breaks tail chain", "first: |+\n  content\n\n\nsibling: value\n", false, false},
		{"|- strip tail: no ambiguity", "key: |-\n  content\n", false, false},
		{"plain scalar tail: nothing to disambiguate", "key: value\n", false, false},
		{"nested |+ at last-entry chain terminus", "outer:\n  inner: |+\n    content\n\n\n", true, true},
		{"nested |+ but sibling breaks chain", "outer:\n  first: |+\n    content\n\n\n  second: value\n", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseBytes([]byte(tc.src), parser.ParseComments)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			doc := file.Docs[0]
			if got := docTailKeepsChomp(doc.Body); got != tc.wantTail {
				t.Errorf("docTailKeepsChomp = %v, want %v", got, tc.wantTail)
			}
			if got := lastEntrySubtreeHasKeepChomp(doc); got != tc.wantLastEn {
				t.Errorf("lastEntrySubtreeHasKeepChomp = %v, want %v", got, tc.wantLastEn)
			}
		})
	}
}

// A doc whose body isn't a mapping (e.g. a root sequence) has no "last entry" concept, so lastEntrySubtreeHasKeepChomp
// returns false without walking.
func TestLastEntrySubtreeNonMappingBody(t *testing.T) {
	src := "- one\n- two\n"
	file, err := parser.ParseBytes([]byte(src), parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := file.Docs[0].Body.(*ast.SequenceNode); !ok {
		t.Fatalf("expected sequence body, got %T", file.Docs[0].Body)
	}
	if lastEntrySubtreeHasKeepChomp(file.Docs[0]) {
		t.Error("sequence-body doc should never register as having last-entry keep-chomp")
	}
}
