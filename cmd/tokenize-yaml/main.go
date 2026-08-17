// tokenize-yaml reads a YAML document from stdin, runs our (vendored) lexer to produce a flat token stream, and prints
// the tokens as pretty JSON via update-yaml/pkg/dump. Companion probe to cmd/probe-yaml (which dumps the parsed AST):
// the token stream is the input pkg/tree's parser consumes.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/andrew-grechkin/update-yaml/pkg/dump"

	"github.com/andrew-grechkin/format-yaml/internal/lexer"
)

func main() {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	fmt.Println(dump.DumpJson(dump.Inspect(lexer.TokenizeNormalized(string(src)))))
}
