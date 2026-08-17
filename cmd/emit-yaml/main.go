// emit-yaml reads a YAML document from stdin, builds our tree, then emits YAML bytes from the tree. First-pass
// goal: byte-identical output for every non-fail fixture (round-trip identity). Divergences drive tree design
// (which tokens the tree needs to preserve).
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

func main() {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, r)
			os.Exit(2)
		}
	}()
	os.Stdout.Write(tree.Emit(tree.Build(src)))
}
