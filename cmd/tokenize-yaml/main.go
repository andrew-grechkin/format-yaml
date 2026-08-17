// tokenize-yaml reads a YAML document from stdin, runs goccy's lexer to
// produce a flat token stream, and prints the tokens as pretty JSON via
// update-yaml/pkg/dump. Companion to update-yaml/cmd/probe-yaml (which
// dumps the parsed AST): tokens are the input goccy's own parser consumes
// and the source we'd feed into a custom AST builder if we wanted to skip
// goccy's parser layer.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/goccy/go-yaml/lexer"

	"github.com/andrew-grechkin/update-yaml/pkg/dump"
)

func main() {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	fmt.Println(dump.DumpJson(dump.Inspect(lexer.Tokenize(string(src)))))
}
