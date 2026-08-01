// Package parser wraps hashicorp/hcl's hclparse with the small
// conveniences hclkit needs: extension-based dispatch between native
// and JSON syntax, and access to the parsed-file map that diagnostic
// rendering requires.
package parser

import (
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

// Parser parses HCL sources and remembers every file it has seen so
// diagnostics can render source snippets. It is not safe for
// concurrent use; create one per load operation.
type Parser struct {
	inner *hclparse.Parser
}

// New returns an empty Parser.
func New() *Parser {
	return &Parser{inner: hclparse.NewParser()}
}

// ParseFile parses the file at path, dispatching on extension: an
// exact ".json" suffix is parsed as JSON-syntax HCL, everything else
// as native syntax.
func (p *Parser) ParseFile(path string) (*hcl.File, hcl.Diagnostics) {
	if filepath.Ext(path) == ".json" {
		return p.inner.ParseJSONFile(path)
	}
	return p.inner.ParseHCLFile(path)
}

// ParseBytes parses src with the same extension dispatch as
// ParseFile. filename is used for diagnostic positions and the file
// map key; it does not need to exist on disk.
func (p *Parser) ParseBytes(filename string, src []byte) (*hcl.File, hcl.Diagnostics) {
	if filepath.Ext(filename) == ".json" {
		return p.inner.ParseJSON(src, filename)
	}
	return p.inner.ParseHCL(src, filename)
}

// Files returns every file this Parser has parsed, keyed by filename
// — the exact shape hcl.NewDiagnosticTextWriter needs to render
// source snippets.
func (p *Parser) Files() map[string]*hcl.File {
	return p.inner.Files()
}
