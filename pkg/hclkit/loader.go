package hclkit

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"reflect"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	"github.com/donaldgifford/hclkit/internal/parser"
	"github.com/donaldgifford/hclkit/pkg/hclkit/varsfile"
)

// MergeMode controls how LoadDir combines multiple files.
type MergeMode int

const (
	// MergeOverride decodes files one at a time in lexical filename
	// order against the same target, so later files override earlier
	// ones field-by-field. It is deliberately the zero value — the
	// default matches the existing multi-file consumer shape (spt) —
	// so don't "fix" the enum to start at 1.
	MergeOverride MergeMode = iota

	// MergeAppend merges every file into one body before a single
	// decode (hcl.MergeBodies semantics), appending blocks across
	// files.
	MergeAppend
)

// String returns the mode name for logs and test output.
func (m MergeMode) String() string {
	switch m {
	case MergeOverride:
		return "override"
	case MergeAppend:
		return "append"
	default:
		return fmt.Sprintf("MergeMode(%d)", int(m))
	}
}

// Loader decodes HCL sources into Go values with consistent,
// position-aware diagnostics. The zero value is usable; New applies
// options. A Loader is immutable after construction and safe for
// concurrent Load* calls.
type Loader struct {
	evalCtx    *hcl.EvalContext
	funcs      map[string]function.Function
	vars       map[string]cty.Value
	varsFiles  []string
	validators []Validator
	mergeMode  MergeMode
	diagWriter io.Writer
}

// VarsResult aliases varsfile.VarsResult so LoadVarsFile callers
// don't need the subpackage import for the common flow.
type VarsResult = varsfile.VarsResult

// New returns a Loader configured by opts.
func New(opts ...Option) *Loader {
	l := &Loader{}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// LoadFile decodes the file at path into target, a non-nil pointer to
// a struct or map with hcl tags. Files with a .json extension are
// decoded as JSON-syntax HCL; everything else as native syntax.
func (l *Loader) LoadFile(path string, target any) Diagnostics {
	p := parser.New()
	file, diags := p.ParseFile(path)
	if file != nil && !diags.HasErrors() {
		diags = diags.Extend(l.loadBodies(p, []hcl.Body{file.Body}, target))
	}
	return l.finish(diags, p.Files())
}

// LoadBytes decodes raw HCL source into target. filename is used for
// diagnostic positions and syntax dispatch (a .json suffix selects
// JSON-syntax HCL); it does not need to exist on disk.
func (l *Loader) LoadBytes(filename string, src []byte, target any) Diagnostics {
	p := parser.New()
	file, diags := p.ParseBytes(filename, src)
	if file != nil && !diags.HasErrors() {
		diags = diags.Extend(l.loadBodies(p, []hcl.Body{file.Body}, target))
	}
	return l.finish(diags, p.Files())
}

// LoadDir decodes every *.hcl file directly inside dir (lexical
// filename order, subdirectories and other extensions skipped) into
// target, combining files per the configured MergeMode.
//
// Ordering is byte-wise lexical, not numeric — "10-a.hcl" sorts
// before "2-b.hcl". Zero-pad numeric prefixes ("01-", "02-") when
// override order matters. A directory containing no *.hcl files is an
// error.
func (l *Loader) LoadDir(dir string, target any) Diagnostics {
	p := parser.New()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return l.finish(hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Failed to read directory",
			Detail:   fmt.Sprintf("Cannot read %s: %s.", dir, err),
		}}, p.Files())
	}

	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".hcl" {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	if len(paths) == 0 {
		return l.finish(hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "No HCL files found",
			Detail:   fmt.Sprintf("Directory %s contains no *.hcl files.", dir),
		}}, p.Files())
	}

	// Parse everything before deciding to decode, so one call
	// surfaces every syntax error across the directory at once.
	var diags hcl.Diagnostics
	files := make([]*hcl.File, 0, len(paths))
	for _, path := range paths {
		file, fileDiags := p.ParseFile(path)
		diags = diags.Extend(fileDiags)
		if file != nil {
			files = append(files, file)
		}
	}
	if diags.HasErrors() {
		return l.finish(diags, p.Files())
	}

	bodies := make([]hcl.Body, len(files))
	for i, file := range files {
		bodies[i] = file.Body
	}
	return l.finish(diags.Extend(l.loadBodies(p, bodies, target)), p.Files())
}

// LoadVarsFile resolves the variable declarations in the config at
// configPath against the assignments in the vars file at varsPath,
// without decoding the rest of the configuration — the standalone
// entry point for flows that need Declared before a full Load (e.g.
// forge's interactive prompting). Paths configured via WithVarsFile
// are not consulted; varsPath is explicit here.
//
// DESIGN-0001 sketched LoadVarsFile(path) with a single path; the
// two-path signature is a recorded deviation (IMPL-0001) —
// declarations live in the main config, which a vars-file path
// alone cannot supply.
func (l *Loader) LoadVarsFile(configPath, varsPath string) (*VarsResult, Diagnostics) {
	p := parser.New()

	var diags hcl.Diagnostics
	configFile, configDiags := p.ParseFile(configPath)
	diags = diags.Extend(configDiags)
	varsParsed, varsDiags := p.ParseFile(varsPath)
	diags = diags.Extend(varsDiags)
	if configFile == nil || varsParsed == nil || diags.HasErrors() {
		return nil, l.finish(diags, p.Files())
	}

	decls, _, declDiags := varsfile.DecodeVariables(configFile.Body)
	diags = diags.Extend(declDiags)
	assigns, assignDiags := varsfile.DecodeAssignments(varsParsed.Body)
	diags = diags.Extend(assignDiags)
	if diags.HasErrors() {
		return nil, l.finish(diags, p.Files())
	}

	result, resolveDiags := varsfile.Resolve(decls, assigns, l.evalContext())
	return result, l.finish(diags.Extend(resolveDiags), p.Files())
}

// loadBodies is the shared decode pipeline: assemble the effective
// EvalContext (resolving and binding vars files when configured),
// then decode the bodies per the merge mode.
func (l *Loader) loadBodies(p *parser.Parser, bodies []hcl.Body, target any) hcl.Diagnostics {
	ctx := l.evalContext()

	// Validators walk the raw parsed bodies: the stripped remains
	// bindVars produces aren't *hclsyntax.Body, so the as-written
	// bodies are the only ones validators can enumerate.
	rawBodies := bodies

	var diags hcl.Diagnostics
	if len(l.varsFiles) > 0 {
		var varsDiags hcl.Diagnostics
		bodies, ctx, varsDiags = l.bindVars(p, bodies, ctx)
		diags = diags.Extend(varsDiags)
		if diags.HasErrors() {
			return diags
		}
	}

	for _, v := range l.validators {
		diags = diags.Extend(v.Validate(rawBodies, ctx))
	}

	if l.mergeMode == MergeAppend {
		return diags.Extend(decodeWith(hcl.MergeBodies(bodies), ctx, target))
	}
	// MergeOverride: decode every body even after a decode error so
	// one call reports every problem in a single CI pass.
	for _, body := range bodies {
		diags = diags.Extend(decodeWith(body, ctx, target))
	}
	return diags
}

// bindVars strips variable blocks from bodies, resolves them against
// the configured vars files, and returns the stripped bodies plus a
// child context with var bound. Declarations are collected across
// every body before resolving once — a later file's declaration must
// be visible while decoding an earlier file — and later vars files
// win per assignment name. The var binding lives in a child context,
// so it shadows any WithVariables("var") entry by design.
func (l *Loader) bindVars(p *parser.Parser, bodies []hcl.Body, ctx *hcl.EvalContext) ([]hcl.Body, *hcl.EvalContext, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	remains := make([]hcl.Body, len(bodies))
	decls := make(map[string]varsfile.Variable)
	for i, body := range bodies {
		bodyDecls, remain, declDiags := varsfile.DecodeVariables(body)
		diags = diags.Extend(declDiags)
		remains[i] = remain
		for name := range bodyDecls {
			decl := bodyDecls[name]
			if prev, ok := decls[name]; ok {
				declRange := decl.DeclRange
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Duplicate variable declaration",
					Detail: fmt.Sprintf("Variable %q was already declared at %s.",
						name, prev.DeclRange),
					Subject: &declRange,
				})
				continue
			}
			decls[name] = decl
		}
	}

	assigns := make(map[string]varsfile.Assignment)
	for _, path := range l.varsFiles {
		file, fileDiags := p.ParseFile(path)
		diags = diags.Extend(fileDiags)
		if file == nil || fileDiags.HasErrors() {
			continue
		}
		fileAssigns, assignDiags := varsfile.DecodeAssignments(file.Body)
		diags = diags.Extend(assignDiags)
		maps.Copy(assigns, fileAssigns) // later vars file wins
	}
	if diags.HasErrors() {
		return remains, ctx, diags
	}

	result, resolveDiags := varsfile.Resolve(decls, assigns, ctx)
	diags = diags.Extend(resolveDiags)
	if resolveDiags.HasErrors() {
		return remains, ctx, diags
	}

	varCtx := &hcl.EvalContext{}
	if ctx != nil {
		varCtx = ctx.NewChild()
	}
	varCtx.Variables = map[string]cty.Value{"var": result.Values}
	return remains, varCtx, diags
}

// decodeWith is the single dispatch point every Load* funnels
// through. Phase 3 grows spec-shaped decoding via LoadSpec; the
// gohcl arm lives here.
func decodeWith(body hcl.Body, ctx *hcl.EvalContext, target any) hcl.Diagnostics {
	if diag := checkTarget(target); diag != nil {
		return hcl.Diagnostics{diag}
	}
	return gohcl.DecodeBody(body, ctx, target)
}

// checkTarget rejects targets gohcl.DecodeBody would panic on: nil,
// non-pointers, and pointers to anything but a struct or map. A
// caller mistake must surface as a diagnostic, not a panic.
func checkTarget(target any) *hcl.Diagnostic {
	rv := reflect.ValueOf(target)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid decode target",
			Detail: fmt.Sprintf(
				"Decode target must be a non-nil pointer to a struct or map; got %T.", target),
		}
	}
	switch rv.Elem().Kind() {
	case reflect.Struct, reflect.Map:
		return nil
	default:
		return &hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid decode target",
			Detail:   fmt.Sprintf("Decode target must point to a struct or map; got %T.", target),
		}
	}
}

// evalContext assembles the effective *hcl.EvalContext for one
// decode. It returns nil when nothing is configured, keeping the
// zero-config path byte-identical to gohcl.DecodeBody(body, nil,
// target) — the behavior nil-ctx consumers depend on.
func (l *Loader) evalContext() *hcl.EvalContext {
	switch {
	case l.evalCtx == nil && len(l.funcs) == 0 && len(l.vars) == 0:
		return nil
	case len(l.funcs) == 0 && len(l.vars) == 0:
		return l.evalCtx
	}

	ctx := &hcl.EvalContext{}
	if l.evalCtx != nil {
		ctx = l.evalCtx.NewChild()
	}

	// Variable lookup falls through to parent contexts per name, so
	// only the loader's own variables need to live at this level.
	ctx.Variables = maps.Clone(l.vars)

	// Function lookup consults only the nearest non-nil Functions
	// map — no per-name fallthrough — so the caller's functions must
	// be flattened into this level or they'd be shadowed away.
	fns := make(map[string]function.Function, len(l.funcs))
	maps.Copy(fns, nearestFunctions(l.evalCtx))
	maps.Copy(fns, l.funcs)
	ctx.Functions = fns

	return ctx
}

// nearestFunctions returns the function map hcl would consult for
// ctx: the first non-nil Functions map walking up the parent chain.
func nearestFunctions(ctx *hcl.EvalContext) map[string]function.Function {
	for ; ctx != nil; ctx = ctx.Parent() {
		if ctx.Functions != nil {
			return ctx.Functions
		}
	}
	return nil
}

// finish wraps diags for return and, when WithDiagnosticWriter is
// configured, tees the rendered output there best-effort.
func (l *Loader) finish(diags hcl.Diagnostics, files map[string]*hcl.File) Diagnostics {
	d := NewDiagnostics(diags, files)
	if l.diagWriter != nil && len(diags) > 0 {
		//nolint:errcheck // tee is best-effort by contract; caller still gets d
		_, _ = d.WriteTo(l.diagWriter)
	}
	return d
}
