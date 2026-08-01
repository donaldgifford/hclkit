package hclkit

import (
	"io"
	"maps"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// Option configures a Loader. Options are applied once by New; a
// Loader is immutable afterward.
type Option func(*Loader)

// WithEvalContext supplies a base *hcl.EvalContext for decoding.
// Load* calls never mutate it: functions and variables registered via
// WithFunctions / WithVariables are layered on top at decode time and
// win on name collision.
func WithEvalContext(ctx *hcl.EvalContext) Option {
	return func(l *Loader) { l.evalCtx = ctx }
}

// WithFunctions registers functions available to HCL expressions.
// The map is copied; repeated options merge, later entries winning.
func WithFunctions(fns map[string]function.Function) Option {
	return func(l *Loader) {
		if l.funcs == nil {
			l.funcs = make(map[string]function.Function, len(fns))
		}
		maps.Copy(l.funcs, fns)
	}
}

// WithVariables registers variables available to HCL expressions.
// The map is copied; repeated options merge, later entries winning.
func WithVariables(vars map[string]cty.Value) Option {
	return func(l *Loader) {
		if l.vars == nil {
			l.vars = make(map[string]cty.Value, len(vars))
		}
		maps.Copy(l.vars, vars)
	}
}

// WithMergeMode sets how LoadDir combines multiple files. The default
// is MergeOverride.
func WithMergeMode(m MergeMode) Option {
	return func(l *Loader) { l.mergeMode = m }
}

// WithDiagnosticWriter tees rendered diagnostics to w. Every Load*
// call that produces diagnostics also writes them to w (best-effort)
// before returning the same Diagnostics to the caller — the return
// value is unaffected.
func WithDiagnosticWriter(w io.Writer) Option {
	return func(l *Loader) { l.diagWriter = w }
}
