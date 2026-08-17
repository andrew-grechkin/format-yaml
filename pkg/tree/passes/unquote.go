package passes

import (
	"regexp"
	"strings"

	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"

	"github.com/andrew-grechkin/format-yaml/pkg/tree"
)

// unquoteSafeStrings walks the tree, tracking flow context, and demotes any explicitly-quoted StringNode
// whose value would parse back to itself as a plain scalar. Uses goccy's parser as the safety oracle: if
// ParseBytes(v) round-trips to a StringNode with the same value, unquoting v is byte-safe. Skips YAML 1.1
// boolean spellings and sexagesimal patterns explicitly - goccy is 1.2 and reads them as plain strings, but
// older parsers (Ansible, PyYAML, Ruby psych, docker-compose) would resolve them to non-string types, so
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
	safe := safeToUnquote(v)
	if inFlow {
		safe = safeToUnquoteInFlow(v)
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

// safeToUnquote asks goccy: would parsing v as a YAML doc give back a single StringNode with the same
// value? If yes, v is a valid plain scalar that survives a round-trip. Guards against the empty string
// (unquoting `""` would lose the empty-vs-null distinction) and the YAML 1.1 forms above.
func safeToUnquote(v string) bool {
	if v == "" {
		return false
	}
	if yaml11Bools[v] {
		return false
	}
	if sexagesimalPattern.MatchString(v) {
		return false
	}
	file, err := parser.ParseBytes([]byte(v), 0)
	if err != nil || len(file.Docs) != 1 {
		return false
	}
	sn, ok := file.Docs[0].Body.(*ast.StringNode)
	if !ok {
		return false
	}
	return sn.Value == v
}

// safeToUnquoteInFlow adds flow-context checks on top of safeToUnquote: no bare flow indicators (`,[]{}`)
// which would terminate the scalar early, and a round-trip through `[v]` to catch edge cases (whitespace-
// only strings that flow resolves to null, adjacent-indicator patterns) that the character check misses.
func safeToUnquoteInFlow(v string) bool {
	if !safeToUnquote(v) {
		return false
	}
	if strings.ContainsAny(v, "[]{},") {
		return false
	}
	file, err := parser.ParseBytes([]byte("["+v+"]"), 0)
	if err != nil || len(file.Docs) != 1 {
		return false
	}
	seq, ok := file.Docs[0].Body.(*ast.SequenceNode)
	if !ok || !seq.IsFlowStyle || len(seq.Values) != 1 {
		return false
	}
	sn, ok := seq.Values[0].(*ast.StringNode)
	if !ok {
		return false
	}
	return sn.Value == v
}
