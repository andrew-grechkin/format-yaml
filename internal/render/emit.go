// AST-driven emitter. Walks the AST and writes bytes directly, controlling composition (mapping/sequence traversal,
// inter-entry spacing, comment placement) ourselves while delegating scalar rendering to goccy's per-node .String().
// Wired into format.Bytes as the sole render path.
//
// The whole point: no render-then-normalize. Blank-line policy is a per-boundary emitter decision. `|+`/`>+` trailing
// newlines are written literally from the AST value. Inline comment spacing is a mode-specific constant. Nothing
// text-scans for headers or normalizes after the fact (except a final stripTrailingWhitespace pass to defend against
// goccy's .String() emitting stray trailing whitespace on non-block-scalar lines from pathological input).
//
// Layout: this file holds the entry, the emitter struct, and the generic helpers (buffer writes, dispatch, source-line
// inspection, child-indent resolution, inline-ability check). Per-node-type emission lives in emit_doc.go,
// emit_mapping.go, emit_sequence.go, emit_scalar.go, emit_comment.go.
package render

import (
	"strings"

	"github.com/goccy/go-yaml/ast"

	astutil "github.com/andrew-grechkin/update-yaml/pkg/ast"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// EmitFile is the pipeline's render entry. Walks file.Docs and returns the final byte slice.
func EmitFile(file *ast.File, src []byte, m config.Mode, docLevels [][]string) []byte {
	// Goccy attaches inter-doc comments as FootComment on the previous doc's last mapping entry, but source blank-line
	// context often makes them read as HEAD of the following doc. detachInterDocFootComments separates the two
	// attributions: head-of-next comments get pulled off the AST and returned in postDoc[prev] as raw lines we splice
	// between docs; foot-of- previous comments stay attached and render via emitFootComment.
	e := &emitter{
		mode:                m,
		perDocLeadingBlanks: sourceInterDocBlanks(src, file),
		sourceInline:        collectSourceInlineSpaces(src),
		postDoc:             detachInterDocFootComments(file, src),
		perDocBlanks:        perDocSourceBlanks(src, file),
		srcLines:            strings.Split(string(src), "\n"),
	}
	for docIdx, doc := range file.Docs {
		e.emitDoc(doc, docIdx, docLevels)
	}
	// Final cleanup: stripTrailingWhitespace defends against goccy .String() outputs that end with whitespace on
	// non-block-scalar lines (some pathological inputs attach comment `#` tokens to anchor names, whose .String()
	// preserves the trailing space and would grow on every round-trip otherwise). Block-scalar interior whitespace is
	// preserved by the pass itself.
	return []byte(stripTrailingWhitespace(e.buf.String()))
}

type emitter struct {
	buf                 strings.Builder
	mode                config.Mode
	perDocLeadingBlanks []int
	// Per-key source-observed whitespace between value and inline `#`. Consulted at minimal mode to preserve
	// author-authored column alignment; at standard+ we ignore it and use a fixed 2-space gap.
	sourceInline map[string]int
	// True immediately after emitting a comment-only doc. The next mapping doc's `---` should sit directly under the
	// comment without any additional blank line insertion.
	prevWasComment bool
	// True immediately after the previous doc emitted its `...` end marker. When true, a hoisted head-of-next comment
	// doesn't need an additional blank line above it - the `...` already provides the doc boundary, and adding a blank
	// changes re-parse attribution (comment becomes its own doc).
	prevEmittedDocEnd bool
	// Hoisted foot comments, keyed by the docIdx of the doc they followed. Non-empty entries are emitted between the
	// previous doc and the next doc's `---`.
	postDoc [][]string
	// Per-doc source-observed blank-line counts between adjacent top-level entries. Consulted at minimal (preserve
	// verbatim) and standard (collapse to <=1) modes; ignored at full+ where the wants-rule from AST shape is
	// authoritative.
	perDocBlanks [][]int
	// currentDocBoundaries is the source-blank slice for the doc currently being emitted, set by emitDoc before
	// descending into the mapping. emitInterEntryBlank at top level indexes it by the boundary counter (0-based).
	currentDocBoundaries []int
	// Source lines. Used to look up blank counts immediately above any AST-anchored source line, so nested
	// sequences/mappings can preserve author blank lines at minimal mode without the top-level-only shortcut.
	srcLines []string
}

// Number of trailing '\n' bytes currently at the tail of buf. Used by ensureTrailingNLs to pad up to a target without
// over- counting the last write's own newlines.
func (e *emitter) trailingNLs() int {
	b := e.buf.String()
	n := 0
	for i := len(b) - 1; i >= 0 && b[i] == '\n'; i-- {
		n++
	}
	return n
}

// Writes exactly enough '\n' to bring the tail count up to n. A no-op when the tail already has n or more.
func (e *emitter) ensureTrailingNLs(n int) {
	for e.trailingNLs() < n {
		e.buf.WriteByte('\n')
	}
}

func (e *emitter) writeString(s string) { e.buf.WriteString(s) }
func (e *emitter) writeByte(b byte)     { e.buf.WriteByte(b) }
func (e *emitter) writeIndent(n int) {
	for range n {
		e.buf.WriteByte(' ')
	}
}

// Returns the number of consecutive blank lines in the source immediately above `line` (1-indexed source line). Used by
// minimal-mode nested-boundary blank preservation.
func (e *emitter) sourceBlanksBefore(line int) int {
	if line <= 1 {
		return 0
	}
	n := 0
	for i := line - 2; i >= 0 && i < len(e.srcLines); i-- {
		if strings.TrimSpace(e.srcLines[i]) != "" {
			break
		}
		n++
	}
	return n
}

// emitBody is the top-level dispatch called once per doc from emitDoc. Its `topLevel=true` propagates only into the
// immediate MappingNode/SequenceNode; nested calls to emitNode use topLevel=false.
func (e *emitter) emitBody(n ast.Node, indent int) {
	e.emitNodeAt(n, indent, true)
}

// emitNode is the nested-context entry: no top-level status.
func (e *emitter) emitNode(n ast.Node, indent int) {
	e.emitNodeAt(n, indent, false)
}

// Dispatches on node type. Structural nodes (Mapping, Sequence) walk their children with our own composition. Leaf
// nodes (scalars, anchors, tags) delegate to goccy's .String() - it handles the atomic representation correctly, only
// composition was ever the problem. `topLevel` flags whether the wants-based blank-line rule should fire between direct
// children.
func (e *emitter) emitNodeAt(n ast.Node, indent int, topLevel bool) {
	if n == nil {
		return
	}
	switch v := n.(type) {
	case *ast.MappingNode:
		e.emitMapping(v, indent, topLevel)
	case *ast.SequenceNode:
		e.emitSequence(v, indent)
	case *ast.LiteralNode:
		e.emitLiteral(v, indent)
	case *ast.AnchorNode:
		// Anchor wraps a value. Emit `&name ` prefix then the value.
		e.writeByte('&')
		e.writeString(v.Name.String())
		if v.Value != nil {
			e.writeByte(' ')
			e.emitInlineValue(v.Value, indent)
		}
	default:
		// Scalar or other leaf: delegate to goccy.
		e.writeString(n.String())
	}
}

// True when n renders as a single line and can sit inline after a `key: ` prefix. Block-style mappings/sequences with
// entries require a newline + indented body. LiteralNodes have their header inline but their body indented - treated as
// "inline header" by emitMappingValue.
func inlineable(n ast.Node) bool {
	n = astutil.UnwrapAnchor(n)
	switch v := n.(type) {
	case *ast.MappingNode:
		return v.IsFlowStyle || len(v.Values) == 0
	case *ast.SequenceNode:
		return v.IsFlowStyle || len(v.Values) == 0
	case *ast.LiteralNode:
		return false // header inline, body indented — handled specially
	}
	return true
}

// Returns the target indent (in 0-based columns) for the children of a value that will render as a block-style nested
// collection or a block scalar body. At minimal mode we read the actual AST column from the first child's Key/Start
// token, preserving the source's indent choice (4-space, tab-based, etc.). At standard+ we normalize to parent+2 for
// consistency.
//
// A column read of 0 or 1 (missing or top-level) falls back to parent+2 - the source's own indent is either
// uncomputable or meaningless (a top-level value has no deeper parent-relative indent to preserve).
func (e *emitter) childIndent(val ast.Node, parentIndent int) int {
	if e.mode >= config.ModeStandard {
		return parentIndent + 2
	}
	var col int
	switch v := astutil.UnwrapAnchor(val).(type) {
	case *ast.MappingNode:
		if len(v.Values) > 0 {
			if t := v.Values[0].Key.GetToken(); t != nil && t.Position != nil {
				col = t.Position.Column
			}
		}
	case *ast.SequenceNode:
		if v.Start != nil && v.Start.Position != nil {
			col = v.Start.Position.Column
		}
	case *ast.LiteralNode:
		if v.Value != nil && v.Value.Token != nil && v.Value.Token.Position != nil {
			col = v.Value.Token.Position.Column
		}
	}
	if col <= 1 {
		return parentIndent + 2
	}
	return col - 1
}
