package format

import (
	"bytes"
	"fmt"
	"io"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// Runs the full pipeline against stdin and only writes once the entire rendered result is buffered. A parse or render
// error before that point leaves stdout completely untouched, so the caller never sees a half-formatted document.
func Reader(in io.Reader, out io.Writer, m config.Mode) error {
	src, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	formatted, err := Bytes(src, m)
	if err != nil {
		return err
	}
	_, err = out.Write(formatted)
	return err
}

// Formats stdin and compares against the input. Returns ErrCheckDiff if the two differ so main can exit non-zero,
// without writing anything to stdout.
func CheckReader(in io.Reader, _ io.Writer, m config.Mode) error {
	src, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	formatted, err := Bytes(src, m)
	if err != nil {
		return err
	}
	if !bytes.Equal(src, formatted) {
		return ErrCheckDiff
	}
	return nil
}

// Prints a unified diff between stdin and its formatted form to stdout, returning ErrCheckDiff so main exits non-zero
// when any changes exist.
func DiffReader(in io.Reader, out io.Writer, m config.Mode) error {
	src, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	formatted, err := Bytes(src, m)
	if err != nil {
		return err
	}
	return writeDiff(out, "<stdin>", src, formatted)
}
