package hclkit

import (
	"maps"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	"github.com/donaldgifford/hclkit/pkg/hclkit/funcs"
)

// EvalCtxBuilder accumulates functions, variables, and locals into a
// single *hcl.EvalContext for use with WithEvalContext. The zero
// value is usable; NewEvalCtx is a convenience.
//
// Unlike the Loader, a builder is mutable: With* methods modify the
// receiver in place and return it for chaining, and the builder is
// not safe for concurrent use. Nothing is evaluated until Build.
type EvalCtxBuilder struct {
	funcs    map[string]function.Function
	vars     map[string]cty.Value
	locals   []hcl.Body
	stdFuncs bool
}

// NewEvalCtx returns an empty EvalCtxBuilder.
func NewEvalCtx() *EvalCtxBuilder {
	return &EvalCtxBuilder{}
}

// WithStdFuncs includes the standard function bundle (funcs.Std) in
// the built context. Functions registered with WithFunc always win
// name collisions against the bundle, regardless of call order.
func (b *EvalCtxBuilder) WithStdFuncs() *EvalCtxBuilder {
	b.stdFuncs = true
	return b
}

// WithFunc registers fn under the given HCL-visible name.
func (b *EvalCtxBuilder) WithFunc(name string, fn function.Function) *EvalCtxBuilder {
	if b.funcs == nil {
		b.funcs = make(map[string]function.Function)
	}
	b.funcs[name] = fn
	return b
}

// WithVar binds val to the given variable name.
func (b *EvalCtxBuilder) WithVar(name string, val cty.Value) *EvalCtxBuilder {
	if b.vars == nil {
		b.vars = make(map[string]cty.Value)
	}
	b.vars[name] = val
	return b
}

// WithLocals registers a locals body — typically the body of a
// locals block — whose attributes Build evaluates and exposes as
// local.<name> references. It mirrors repo-guardian's decodeLocals:
// attributes are evaluated single-pass against the builder's
// variables and functions, so a local cannot reference a sibling
// local. Bodies registered across multiple calls are evaluated in
// registration order, and later definitions of a name win. A nil
// body is ignored.
func (b *EvalCtxBuilder) WithLocals(body hcl.Body) *EvalCtxBuilder {
	if body != nil {
		b.locals = append(b.locals, body)
	}
	return b
}

// Build assembles the accumulated state into an *hcl.EvalContext.
// It returns nil when nothing was configured, keeping
// WithEvalContext(b.Build()) on an empty builder a true no-op (the
// same nil-identity the zero-config Loader guarantees).
//
// Build is the single fallible point: locals evaluation happens
// here, and its diagnostics are returned rather than rendered — the
// builder has no parsed-file map, so wrap them with NewDiagnostics
// and the caller's files for source snippets. Build is repeatable;
// each call re-evaluates locals (a now() in a locals body yields a
// fresh timestamp) and returns a context that does not alias the
// builder's internal state, so later With* calls never mutate a
// context already handed to a Loader.
//
// The design sketched Build() (*hcl.EvalContext) with no error path;
// the diagnostics return is a recorded deviation (IMPL-0001) — the
// locals arm made Build fallible.
func (b *EvalCtxBuilder) Build() (*hcl.EvalContext, hcl.Diagnostics) {
	if !b.stdFuncs && len(b.funcs) == 0 && len(b.vars) == 0 && len(b.locals) == 0 {
		return nil, nil
	}

	var fns map[string]function.Function
	if b.stdFuncs {
		fns = funcs.Std()
	} else {
		fns = make(map[string]function.Function, len(b.funcs))
	}
	maps.Copy(fns, b.funcs) // WithFunc beats the std bundle on collision

	vars := make(map[string]cty.Value, len(b.vars)+1)
	maps.Copy(vars, b.vars)

	ctx := &hcl.EvalContext{Variables: vars, Functions: fns}

	locals, diags := b.evalLocals(ctx)
	if locals != nil {
		// Bound last, so locals win a collision with WithVar("local").
		vars["local"] = cty.ObjectVal(locals)
	}

	return ctx, diags
}

// evalLocals evaluates every registered locals body against ctx.
// Evaluation is collect-all: a failing attribute contributes its
// diagnostics and the rest still evaluate, mirroring the Loader's
// one-pass-reports-everything behavior. Returns nil when no locals
// bodies were registered.
func (b *EvalCtxBuilder) evalLocals(ctx *hcl.EvalContext) (map[string]cty.Value, hcl.Diagnostics) {
	if len(b.locals) == 0 {
		return nil, nil
	}

	locals := make(map[string]cty.Value)
	var diags hcl.Diagnostics
	for _, body := range b.locals {
		attrs, attrDiags := body.JustAttributes()
		diags = diags.Extend(attrDiags)
		for _, attr := range attrs {
			val, valDiags := attr.Expr.Value(ctx)
			diags = diags.Extend(valDiags)
			if valDiags.HasErrors() {
				continue
			}
			locals[attr.Name] = val
		}
	}

	return locals, diags
}
