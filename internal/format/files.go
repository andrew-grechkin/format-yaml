package format

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// Buffers every file's rendered output before writing any of it. A failure on file N aborts the whole run without
// emitting partial content for files 1..N-1, matching the "all-or-nothing" guarantee used for stdin.
func FilesToStdout(paths []string, out io.Writer, m config.Mode) error {
	rendered := make([][]byte, len(paths))
	for i, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		formatted, err := Bytes(src, m)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		rendered[i] = formatted
	}
	for _, b := range rendered {
		if _, err := out.Write(b); err != nil {
			return err
		}
	}
	return nil
}

// Formats each file in memory, prints the paths of files that would be modified to stdout (gofmt -l style), and returns
// ErrCheckDiff if any diffs were found. Files themselves are never touched.
func CheckFiles(paths []string, out io.Writer, m config.Mode) error {
	anyDiff := false
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		formatted, err := Bytes(src, m)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if !bytes.Equal(src, formatted) {
			fmt.Fprintln(out, p)
			anyDiff = true
		}
	}
	if anyDiff {
		return ErrCheckDiff
	}
	return nil
}

// Prints a unified diff for every file that would change. Errors on the first read/format failure; missing files or
// malformed YAML bubble up like elsewhere.
func DiffFiles(paths []string, out io.Writer, m config.Mode) error {
	anyDiff := false
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		formatted, err := Bytes(src, m)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		if !bytes.Equal(src, formatted) {
			if err := writeDiff(out, p, src, formatted); err != nil && err != ErrCheckDiff {
				return err
			}
			anyDiff = true
		}
	}
	if anyDiff {
		return ErrCheckDiff
	}
	return nil
}

// Emits a unified diff of orig vs formatted to out, tagging the sides with `<name> (original)` and `<name>
// (formatted)`. Returns ErrCheckDiff when the two differ so callers can propagate a non-zero exit status without adding
// their own bookkeeping.
func writeDiff(out io.Writer, name string, orig, formatted []byte) error {
	if bytes.Equal(orig, formatted) {
		return nil
	}
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(orig)),
		B:        difflib.SplitLines(string(formatted)),
		FromFile: name + " (original)",
		ToFile:   name + " (formatted)",
		Context:  3,
	}
	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return fmt.Errorf("diff %s: %w", name, err)
	}
	if _, err := io.WriteString(out, text); err != nil {
		return err
	}
	return ErrCheckDiff
}
