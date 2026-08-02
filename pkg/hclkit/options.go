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

// WithVarsFile enables vars-file mode: Load* calls strip the
// configuration's variable blocks, resolve them against the literal
// assignments in the file at path, and bind the results as
// var.<name> for the remaining decode. In vars-file mode the
// variable block type belongs to the loader — a consumer struct
// never sees it.
//
// Repeatable: files apply in registration order and a later file
// wins per assignment name. The var binding lives in a child
// context, so it shadows a WithVariables("var") entry. Vars files
// are read from disk on every Load call — including LoadBytes,
// whose main config is otherwise memory-only.
func WithVarsFile(path string) Option {
	return func(l *Loader) { l.varsFiles = append(l.varsFiles, path) }
}

// Validator inspects parsed file bodies before decode and reports
// position-aware diagnostics — cross-block reference resolution,
// uniqueness, and similar structural checks. The validate subpackage
// provides implementations; any type with this Validate method
// satisfies the interface structurally.
//
// Validators receive every file body as written (in vars-file mode
// that includes the variable blocks the decode later strips) together
// with the fully assembled EvalContext, var binding included.
type Validator interface {
	Validate(bodies []hcl.Body, ctx *hcl.EvalContext) hcl.Diagnostics
}

// WithValidators registers validators run by LoadFile, LoadBytes, and
// LoadDir before decoding. Repeatable and accumulating; validators
// run in registration order and their diagnostics merge with decode
// diagnostics (collect-all — validator errors do not stop the
// decode). LoadSpec and LoadVarsFile do not run validators.
func WithValidators(vs ...Validator) Option {
	return func(l *Loader) { l.validators = append(l.validators, vs...) }
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
