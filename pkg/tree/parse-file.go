package tree

import (
	tokenizer "github.com/andrew-grechkin/format-yaml/internal/token"
)

// parseFile builds the stream root: a flat run of directives, documents, and the island EmptyNodes that
// separate them. Comment groups that are not adjacent to the next structural token become File-level
// EmptyNodes; the adjacent group is left in the buffer for the coming doc's header (or body) to claim.
func (p *parser) parseFile() *File {
	file := &File{}

	for !p.eof() {
		p.absorbComments()
		if p.eof() {
			break
		}

		if p.peek().Type == tokenizer.DirectiveType {
			// Comments above a directive belong before it; drain them so they are not emitted after the
			// directive (which has no comment slots of its own).
			file.Children = append(file.Children, p.comments.drain()...)
			file.Children = append(file.Children, p.parseDirective())
			continue
		}

		line := p.peek().Position.Line
		file.Children = append(file.Children, p.comments.islandsBefore(line)...)
		file.Children = append(file.Children, p.parseDoc())
	}

	file.Children = append(file.Children, p.comments.drain()...)
	return file
}

// parseDirective consumes a `%NAME arg1 arg2 ...` directive that sits before any doc header. `%` is the
// DirectiveType token; the name and args follow as scalar tokens on the same line. The directive
// terminates at the next line's first token.
func (p *parser) parseDirective() *DirectiveNode {
	pct := p.next()
	name := p.peek()
	if name == nil || !isScalarType(name.Type) {
		p.fail("directive `%%` at line %d not followed by a name", pct.Position.Line)
	}

	d := &DirectiveNode{Name: p.next()}
	for {
		t := p.peek()
		if t == nil || t.Position.Line != d.Name.Position.Line || !isScalarType(t.Type) {
			break
		}
		d.Args = append(d.Args, p.next())
	}
	return d
}
