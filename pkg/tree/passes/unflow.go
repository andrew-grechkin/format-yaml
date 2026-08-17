package passes

import (
	"strings"

	"github.com/andrew-grechkin/format-yaml/internal/token"
	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// unflowTopLevel converts a flow-style top-level container to block. JSON-like `{"a": "A"}` becomes
// `a: A`; `[1, 2, 3]` becomes a `- item` list. Nested flow constructs (inline collections under a block
// mapping/sequence) are left alone - inlining nested content is usually a deliberate stylistic choice.
//
// Algorithm, per YAML's own flow/block correspondence:
//  1. Drop the opening bracket (`{` / `[`).
//  2. Drop the closing bracket (`}` / `]`).
//  3. Rewrite each entry/item's Trailer from `,` to `\n` (block layout separates entries with newlines).
//  4. For sequences, synthesize a `- ` Marker on each item so block emit gets its dash.
//
// Layout finalization (leading indent on entry keys / sequence markers, terminal newline on file end) is
// left to normalizeIndent which runs later.
func unflowTopLevel(f *tree.File) {
	for _, c := range f.Children {
		doc, ok := c.(*tree.Doc)
		if !ok {
			continue
		}
		for _, body := range doc.Children {
			target := body
			if a, ok := target.(*tree.AnchorNode); ok {
				target = a.Value
			}
			switch v := target.(type) {
			case *tree.MappingNode:
				if v.Style == tree.StyleFlow {
					flowMappingToBlock(v)
				}
			case *tree.SequenceNode:
				if v.Style == tree.StyleFlow {
					flowSequenceToBlock(v)
				}
			}
		}
	}
}

// flowMappingToBlock rewrites m from flow to block layout by dropping brackets and turning each entry's
// comma Trailer into a newline. The last entry has no Trailer (flow's last comma is optional and absent
// on any well-formed input); block form's tail newline for that entry falls back to Emit's file-level
// trailing-`\n` guard.
func flowMappingToBlock(m *tree.MappingNode) {
	m.Style = tree.StyleBlock
	m.Open = nil
	m.Close = nil
	for _, c := range m.Children {
		e, ok := c.(*tree.MappingEntry)
		if !ok {
			continue
		}
		if e.Trailer != nil {
			e.Trailer.Origin = "\n"
		}
	}
}

// flowSequenceToBlock rewrites s from flow to block. Block sequences differ from block mappings in one
// extra way: each item needs a `-` marker on its own line. We synthesize the marker with Origin `- `
// (dash + one space) so `[a, b]` becomes `- a\n- b\n` after normalizeIndent fills in the leading indent
// for each `- `.
func flowSequenceToBlock(s *tree.SequenceNode) {
	s.Style = tree.StyleBlock
	s.Open = nil
	s.Close = nil
	for _, c := range s.Children {
		item, ok := c.(*tree.SequenceItem)
		if !ok {
			continue
		}
		if item.Trailer != nil {
			item.Trailer.Origin = "\n"
		}
		if item.Marker == nil {
			item.Marker = &token.Token{Origin: "- "}
		}
		// Item Values in flow may carry a leading space that was the `, ` separator between items. In
		// block form the `- ` Marker provides that separation, so strip the value's leading whitespace
		// to avoid double-space output like `-  2`.
		if tok := tree.TokenOf(item.Value); tok != nil {
			tok.Origin = strings.TrimLeft(tok.Origin, " \t")
		}
	}
}
