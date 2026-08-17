// probe-yaml reads a YAML document from stdin, builds our own AST via pkg/tree (which walks the goccy lexer's
// token stream directly, bypassing goccy's parser and its comment-attribution quirks), and prints the resulting
// tree as pretty JSON. Companion visualizer and fixture-test harness for iterating on the tree design; once
// stabilized, expected to replace update-yaml/cmd/probe-yaml (which dumps goccy's own AST).
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/andrew-grechkin/update-yaml/pkg/dump"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

func main() {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	// Recover panics from tree.Build (the current signal for "unsupported input") and print just the message on
	// stderr. Keeps fail-* fixture .err files stable across code moves that would shift stack-trace line numbers.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, r)
			os.Exit(2)
		}
	}()
	fmt.Println(dump.DumpJson(dump.Inspect(tree.Build(src))))
}
