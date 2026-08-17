package passes

import (
	"regexp"
	"strings"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// unquoteSafeStrings walks the tree, tracking flow context, and demotes any explicitly-quoted StringNode
// whose value would parse back to itself as a plain scalar. Uses our own parser as the safety oracle: if
// re-parsing v yields a single plain StringNode with the same value, unquoting v is byte-safe. Skips YAML 1.1
// boolean spellings and sexagesimal patterns explicitly - this parser is 1.2 and reads them as plain strings,
// but older parsers (Ansible, PyYAML, Ruby psych, docker-compose) would resolve them to non-string types, so
// the quotes must survive to preserve semantics for downstream 1.1 consumers.
//
// Runs at every mode - unquoting is a low-friction whitespace-ish change (single characters, hidden by
// diff -w on the quote positions) and keeps output consistent regardless of mode.
func unquoteSafeStrings(f *tree.File) {
	for _, c := range f.Children {
		walkUnquote(c, false)
	}
}

// walkUnquote traverses n and tracks whether we're inside a flow-style container. In flow, plain scalars
// have stricter constraints (no bare commas, brackets, braces) so we use a tighter safety check.
func walkUnquote(n tree.Node, inFlow bool) {
	switch v := n.(type) {
	case *tree.Doc:
		for _, c := range v.Children {
			walkUnquote(c, inFlow)
		}
	case *tree.MappingNode:
		childFlow := inFlow || v.Style == tree.StyleFlow
		for _, c := range v.Children {
			walkUnquote(c, childFlow)
		}
	case *tree.MappingEntry:
		walkUnquote(v.Key, inFlow)
		walkUnquote(v.Value, inFlow)
	case *tree.SequenceNode:
		childFlow := inFlow || v.Style == tree.StyleFlow
		for _, c := range v.Children {
			walkUnquote(c, childFlow)
		}
	case *tree.SequenceItem:
		walkUnquote(v.Value, inFlow)
	case *tree.AnchorNode:
		walkUnquote(v.Value, inFlow)
	case *tree.TagNode:
		walkUnquote(v.Value, inFlow)
	case *tree.StringNode:
		maybeUnquote(v, inFlow)
	}
}

// maybeUnquote demotes s from explicit-quoted to plain if the value is safe. Block-scalar styles (`|`, `>`)
// aren't considered "explicitly quoted" and are left alone.
func maybeUnquote(s *tree.StringNode, inFlow bool) {
	if s.Token == nil {
		return
	}
	if s.Style != tree.StringSingleQuoted && s.Style != tree.StringDoubleQuoted {
		return
	}
	v := s.Token.Value
	safe := SafeToUnquote(v)
	if inFlow {
		safe = SafeToUnquoteInFlow(v)
	}
	if !safe {
		return
	}
	s.Token.Origin = replaceScalarLexeme(s.Token.Origin, v)
	s.Style = tree.StringPlain
}

// yaml11Bools is the exhaustive set of YAML 1.1 boolean spellings that older parsers resolve to bool.
// Kept as a plain map so lookup is O(1); the sixteen entries were verified against the YAML 1.1 spec.
var yaml11Bools = map[string]bool{
	"y": true, "Y": true, "yes": true, "Yes": true, "YES": true,
	"n": true, "N": true, "no": true, "No": true, "NO": true,
	"on": true, "On": true, "ON": true,
	"off": true, "Off": true, "OFF": true,
}

// sexagesimalPattern matches YAML 1.1 base-60 integers (`80:80`, `1:30:00`). Docker-compose port mappings
// and Ansible time notation both use this; keeping quotes preserves their string-ness for 1.1 consumers.
var sexagesimalPattern = regexp.MustCompile(`^[-+]?[1-9][0-9_]*(:[0-5]?[0-9])+$`)

// SafeToUnquote asks our own parser: would parsing v as a document give back a single plain StringNode with
// the same value? If yes, v survives a round-trip unquoted. Guards against the empty string (unquoting `""`
// would lose the empty-vs-null distinction) and the YAML 1.1 forms above.
func SafeToUnquote(v string) bool {
	if v == "" || yaml11Bools[v] || sexagesimalPattern.MatchString(v) {
		return false
	}
	s, ok := parseSingleScalar(v)
	return ok && s.Style == tree.StringPlain && s.Token != nil && s.Token.Value == v
}

// SafeToUnquoteInFlow adds flow-context checks on top of SafeToUnquote: no bare flow indicators (`,[]{}`)
// which would terminate the scalar early, and a round-trip through `[v]` to catch edge cases (whitespace-only
// strings that flow resolves to null, adjacent-indicator patterns) the character check misses.
func SafeToUnquoteInFlow(v string) bool {
	if !SafeToUnquote(v) {
		return false
	}
	if strings.ContainsAny(v, "[]{},") {
		return false
	}
	doc, ok := parseSingleDoc("[" + v + "]")
	if !ok {
		return false
	}
	seq, ok := doc.Children[0].(*tree.SequenceNode)
	if !ok || seq.Style != tree.StyleFlow || len(seq.Children) != 1 {
		return false
	}
	item, ok := seq.Children[0].(*tree.SequenceItem)
	if !ok {
		return false
	}
	s, ok := item.Value.(*tree.StringNode)
	return ok && s.Token != nil && s.Token.Value == v
}

// parseSingleScalar parses v and returns its single body scalar, or ok=false when v isn't exactly one
// scalar document (a mapping/sequence, multiple docs, or a parse failure all disqualify).
func parseSingleScalar(v string) (*tree.StringNode, bool) {
	doc, ok := parseSingleDoc(v)
	if !ok {
		return nil, false
	}
	s, ok := doc.Children[0].(*tree.StringNode)
	return s, ok
}

// parseSingleDoc parses src with our recursive builder, recovering from the builder's panics (unsupported
// input) and rejecting streams that don't resolve to exactly one non-empty document.
func parseSingleDoc(src string) (doc *tree.Doc, ok bool) {
	defer func() {
		if recover() != nil {
			doc, ok = nil, false
		}
	}()
	file := tree.Build([]byte(src))
	if len(file.Children) != 1 {
		return nil, false
	}
	d, isDoc := file.Children[0].(*tree.Doc)
	if !isDoc || len(d.Children) == 0 {
		return nil, false
	}
	return d, true
}
