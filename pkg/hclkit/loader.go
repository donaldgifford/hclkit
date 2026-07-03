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
	mergeMode  MergeMode
	diagWriter io.Writer
}

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
		diags = diags.Extend(l.decode(file.Body, target))
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
		diags = diags.Extend(l.decode(file.Body, target))
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

	switch l.mergeMode {
	case MergeAppend:
		bodies := make([]hcl.Body, len(files))
		for i, file := range files {
			bodies[i] = file.Body
		}
		diags = diags.Extend(l.decode(hcl.MergeBodies(bodies), target))
	default: // MergeOverride
		// Decode every file even after a decode error so one call
		// reports every problem in a single CI pass.
		for _, file := range files {
			diags = diags.Extend(l.decode(file.Body, target))
		}
	}
	return l.finish(diags, p.Files())
}

// decode is the single dispatch point every Load* funnels through.
// Phase 3 grows spec-shaped decoding; today the only arm is gohcl.
func (l *Loader) decode(body hcl.Body, target any) hcl.Diagnostics {
	if diag := checkTarget(target); diag != nil {
		return hcl.Diagnostics{diag}
	}
	return gohcl.DecodeBody(body, l.evalContext(), target)
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
			Detail:   fmt.Sprintf("Decode target must be a non-nil pointer to a struct or map; got %T.", target),
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
