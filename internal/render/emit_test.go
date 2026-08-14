package render

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/goccy/go-yaml/parser"

	"github.com/andrew-grechkin/format-yaml/internal/config"
	"github.com/andrew-grechkin/format-yaml/internal/passes"
)

// Runs the emitter against every source fixture at every mode and asserts a byte-exact match against the corresponding
// expected fixture. Complements the shell-based fixture suite (`just test`) by exercising the same path directly from
// Go so a regression shows up immediately in `go test ./...`.
func TestEmitAgainstFixtures(t *testing.T) {
	fixDir := findFixtureDir(t)
	sources, err := filepath.Glob(filepath.Join(fixDir, "*-source.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(sources)

	modes := []struct {
		name string
		m    config.Mode
	}{
		{"minimal", config.ModeMinimal},
		{"standard", config.ModeStandard},
		{"full", config.ModeFull},
		{"pedantic", config.ModePedantic},
	}
	for _, srcPath := range sources {
		base := filepath.Base(srcPath)
		fixName := strings.TrimSuffix(base, "-source.yaml")
		src, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range modes {
			expectedPath := filepath.Join(fixDir, fixName+"-expected-"+tc.name+".yaml")
			expected, err := os.ReadFile(expectedPath)
			if err != nil {
				continue
			}
			t.Run(fixName+"/"+tc.name, func(t *testing.T) {
				got := runEmitPipeline(t, src, tc.m)
				if !bytes.Equal(got, expected) {
					t.Errorf("emit output differs from fixture\n=== expected ===\n%s\n=== got ===\n%s", expected, got)
				}
			})
		}
	}
}

// Runs a single fixture at a single mode: parse, apply passes, emit. Returns the emitted bytes.
func runEmitPipeline(t *testing.T, src []byte, m config.Mode) []byte {
	t.Helper()
	file, err := parser.ParseBytes(src, parser.ParseComments)
	if err != nil {
		return nil
	}
	docLevels := make([][]string, len(file.Docs))
	for i, doc := range file.Docs {
		docLevels[i] = ExtractDocLevelHeadComments(doc)
	}
	for _, doc := range file.Docs {
		if doc.Body == nil {
			continue
		}
		ReattributeAdjacentHeadComments(doc.Body, src)
		passes.Apply(doc.Body, m)
	}
	return EmitFile(file, src, m, docLevels)
}

// Seeds a fuzz test with fixture sources so we can measure emit idempotence across the same corpus the fixture tests
// use. Passes if the emitter never panics AND its output re-parses and re-emits to the same bytes.
func FuzzEmit(f *testing.F) {
	entries, _ := filepath.Glob("../../test/fixtures/*-source.yaml")
	for _, path := range entries {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		for m := config.ModeMinimal; m <= config.ModePedantic; m++ {
			f.Add(data, int(m))
		}
	}
	f.Fuzz(func(t *testing.T, src []byte, modeIdx int) {
		m := config.Mode(modeIdx%int(config.ModePedantic) + int(config.ModeMinimal))
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("emit panic on %q: %v", src, r)
			}
		}()
		out1 := runEmitPipelineOrNil(src, m)
		if out1 == nil {
			return
		}
		out2 := runEmitPipelineOrNil(out1, m)
		if out2 == nil {
			return
		}
		if !bytes.Equal(out1, out2) {
			t.Fatalf("emit not idempotent\nfirst:\n%q\nsecond:\n%q", out1, out2)
		}
	})
}

func runEmitPipelineOrNil(src []byte, m config.Mode) []byte {
	defer func() { _ = recover() }()
	file, err := parser.ParseBytes(src, parser.ParseComments)
	if err != nil {
		return nil
	}
	docLevels := make([][]string, len(file.Docs))
	for i, doc := range file.Docs {
		docLevels[i] = ExtractDocLevelHeadComments(doc)
	}
	for _, doc := range file.Docs {
		if doc.Body == nil {
			continue
		}
		ReattributeAdjacentHeadComments(doc.Body, src)
		passes.Apply(doc.Body, m)
	}
	return EmitFile(file, src, m, docLevels)
}

// Finds the test/fixtures directory by walking up from the package dir. Works whether the test runs from the package or
// from the module root.
func findFixtureDir(t *testing.T) string {
	t.Helper()
	dir, _ := os.Getwd()
	for range 5 {
		candidate := filepath.Join(dir, "test", "fixtures")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("test/fixtures directory not found")
	return ""
}
