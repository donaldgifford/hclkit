package hclkit

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/donaldgifford/hclkit/internal/parser"
	"github.com/donaldgifford/hclkit/pkg/hclkit/partial"
)

// LoadSpec decodes the file at path against an hcldec.Spec, returning
// the decoded cty.Value plus retained hcl.Expression handles for the
// attributes named in retain (see partial.DecodeSpec). It is the
// dedicated spec entry point per IMPL-0001 OQ-7 — spec decoding
// produces a cty.Value, which the Load*(path, target) shape has no
// way to return.
//
// The loader's configuration applies exactly as in LoadFile:
// functions and variables from options are in scope, and vars-file
// mode strips variable blocks and binds var.<name> before the spec
// decode. Retained expressions stay unevaluated regardless of
// context — that is the point of retaining them.
func (l *Loader) LoadSpec(path string, spec hcldec.Spec, retain ...string) (cty.Value, partial.ExprMap, Diagnostics) {
	p := parser.New()
	file, diags := p.ParseFile(path)
	if file == nil || diags.HasErrors() {
		return cty.NilVal, nil, l.finish(diags, p.Files())
	}

	body := file.Body
	ctx := l.evalContext()
	if len(l.varsFiles) > 0 {
		bodies, varCtx, varsDiags := l.bindVars(p, []hcl.Body{body}, ctx)
		diags = diags.Extend(varsDiags)
		if diags.HasErrors() {
			return cty.NilVal, nil, l.finish(diags, p.Files())
		}
		body, ctx = bodies[0], varCtx
	}

	val, exprs, decDiags := partial.DecodeSpec(body, spec, ctx, retain...)
	return val, exprs, l.finish(diags.Extend(decDiags), p.Files())
}
