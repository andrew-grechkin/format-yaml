package passes

import (
	"strings"

	"github.com/andrew-grechkin/format-yaml/internal/token"
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// ensureDocFooter guarantees every Doc in the file carries an explicit `...` end marker. Sources that
// already have one keep it; the rest get a canonical footer synthesized.
//
// Before adding the footer it clips the doc's rendered tail down to its semantic minimum (one line
// terminator, or a keep-chomp block's own trailing newlines). That puts `...` immediately after the last
// content/comment and leaves any source separator blank to be re-inserted after the footer by
// ensureBlankLines, giving `...` then one blank then the next `---`.
func ensureDocFooter(f *tree.File) {
	for _, c := range f.Children {
		doc, ok := c.(*tree.Doc)
		if !ok || doc.Footer != nil {
			continue
		}
		tree.SetTrailingNewlines(doc, minTailNewlines(doc))
		doc.Footer = &tree.DocFooter{Token: &token.Token{Value: "...", Origin: "...\n"}}
	}
}

// ensureKeepChompFooter is the one case where `full` (not just pedantic) needs a `...`: the LAST document in
// the stream ends with a keep-chomp block (`|+`/`>+`). Without a marker, the value's trailing newlines are
// indistinguishable from the end of the stream, so a re-parse could not recover them.
func ensureKeepChompFooter(f *tree.File) {
	var last *tree.Doc
	for _, c := range f.Children {
		if d, ok := c.(*tree.Doc); ok {
			last = d
		}
	}
	if last == nil || last.Footer != nil || !tailIsKeepChomp(last) {
		return
	}
	tree.SetTrailingNewlines(last, minTailNewlines(last))
	last.Footer = &tree.DocFooter{Token: &token.Token{Value: "...", Origin: "...\n"}}
}

// minTailNewlines returns the fewest trailing newlines a node's rendered content may end with: a keep-chomp
// block keeps its value's trailing-newline count (they are content), everything else keeps one terminator.
func minTailNewlines(n tree.Node) int {
	if s, ok := n.(*tree.StringNode); ok && isKeepChomp(s) {
		return trailingNewlines(s.Token.Value)
	}
	if child := tailChild(n); child != nil {
		return minTailNewlines(child)
	}
	return 1
}

// tailIsKeepChomp reports whether the node owning a node's rendered tail is a keep-chomp block.
func tailIsKeepChomp(n tree.Node) bool {
	if s, ok := n.(*tree.StringNode); ok {
		return isKeepChomp(s)
	}
	if child := tailChild(n); child != nil {
		return tailIsKeepChomp(child)
	}
	return false
}

// tailChild returns the child that owns a node's rendered tail, or nil for leaves.
func tailChild(n tree.Node) tree.Node {
	switch v := n.(type) {
	case *tree.Doc:
		if v.Footer != nil {
			return v.Footer
		}
		if last := lastNode(v.Children); last != nil {
			return last
		}
		return v.Header
	case *tree.MappingNode:
		return lastNode(v.Children)
	case *tree.MappingEntry:
		return v.Value
	case *tree.SequenceNode:
		return lastNode(v.Children)
	case *tree.SequenceItem:
		return v.Value
	case *tree.AnchorNode:
		return v.Value
	case *tree.TagNode:
		return v.Value
	}
	return nil
}

func lastNode(children []tree.Node) tree.Node {
	if len(children) == 0 {
		return nil
	}
	return children[len(children)-1]
}

func isKeepChomp(s *tree.StringNode) bool {
	return s.Token != nil && s.Header != nil && strings.HasSuffix(strings.TrimSpace(s.Header.Value), "+")
}
