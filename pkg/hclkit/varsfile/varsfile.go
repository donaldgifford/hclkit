// Package varsfile implements the Terraform-style vars-file decode
// path: variable declarations in the main config, literal assignments
// in a separate user-input file, resolved and bound as var.<name>.
//
// The package exposes split primitives — DecodeVariables,
// DecodeAssignments, Resolve — so consumers like forge's interactive
// prompt flow can run declaration decode and resolution separately.
// Loader.LoadVarsFile and WithVarsFile in pkg/hclkit compose them for
// the common case.
//
// This package must import only hcl/cty and the standard library —
// never hclkit itself — so pkg/hclkit can depend on it without a
// cycle.
package varsfile

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

// Variable is one variable declaration from the main config.
//
// Default is the declaration's default expression (nil when absent);
// it is evaluated during Resolve against the caller's EvalContext, so
// defaults may use loader functions like env() but cannot reference
// other variables.
type Variable struct {
	Name        string
	Type        cty.Type
	Default     hcl.Expression
	Description string
	Validations []Validation
	DeclRange   hcl.Range
}

// Validation is one validation block on a variable declaration.
// Condition references the value under validation as var.<name>; a
// false result surfaces ErrorMessage as a diagnostic anchored at
// DeclRange.
type Validation struct {
	Condition    hcl.Expression
	ErrorMessage string
	DeclRange    hcl.Range
}

// Assignment is one attribute from a vars file, retained with source
// ranges so resolution diagnostics anchor in the user's file.
type Assignment struct {
	Value     cty.Value
	NameRange hcl.Range
	ExprRange hcl.Range
}

// VarsResult is the outcome of Resolve: the values object to bind as
// var in an EvalContext, and the declarations for downstream flows
// (interactive prompting, documentation).
type VarsResult struct {
	Values   cty.Value
	Declared map[string]Variable
}

var variableSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "type", Required: true},
		{Name: "default"},
		{Name: "description"},
	},
	Blocks: []hcl.BlockHeaderSchema{{Type: "validation"}},
}

var validationSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{Name: "condition", Required: true},
		{Name: "error_message", Required: true},
	},
}

// DecodeVariables extracts every variable block from body, returning
// the declarations, the remaining body with variable blocks stripped
// (for the consumer's own decode), and diagnostics. Decoding is
// collect-all: a malformed declaration contributes its diagnostics
// and the rest still decode. Duplicate declarations of a name are an
// error anchored at the later block.
func DecodeVariables(body hcl.Body) (map[string]Variable, hcl.Body, hcl.Diagnostics) {
	content, remain, diags := body.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "variable", LabelNames: []string{"name"}}},
	})

	decls := make(map[string]Variable, len(content.Blocks))
	for _, block := range content.Blocks {
		decl, declDiags := decodeVariableBlock(block)
		diags = diags.Extend(declDiags)
		if declDiags.HasErrors() {
			continue
		}
		if prev, ok := decls[decl.Name]; ok {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Duplicate variable declaration",
				Detail: fmt.Sprintf("Variable %q was already declared at %s.",
					decl.Name, prev.DeclRange),
				Subject: &decl.DeclRange,
			})
			continue
		}
		decls[decl.Name] = decl
	}
	return decls, remain, diags
}

func decodeVariableBlock(block *hcl.Block) (Variable, hcl.Diagnostics) {
	decl := Variable{Name: block.Labels[0], DeclRange: block.DefRange}

	content, diags := block.Body.Content(variableSchema)

	if attr, ok := content.Attributes["type"]; ok {
		ty, tyDiags := typeexpr.Type(attr.Expr)
		diags = diags.Extend(tyDiags)
		decl.Type = ty
	}
	if attr, ok := content.Attributes["default"]; ok {
		decl.Default = attr.Expr
	}
	if attr, ok := content.Attributes["description"]; ok {
		val, descDiags := attr.Expr.Value(nil)
		diags = diags.Extend(descDiags)
		if !descDiags.HasErrors() && val.Type() == cty.String {
			decl.Description = val.AsString()
		}
	}

	for _, vb := range content.Blocks {
		validation, vDiags := decodeValidationBlock(vb)
		diags = diags.Extend(vDiags)
		if !vDiags.HasErrors() {
			decl.Validations = append(decl.Validations, validation)
		}
	}

	return decl, diags
}

func decodeValidationBlock(block *hcl.Block) (Validation, hcl.Diagnostics) {
	validation := Validation{DeclRange: block.DefRange}

	content, diags := block.Body.Content(validationSchema)

	if attr, ok := content.Attributes["condition"]; ok {
		validation.Condition = attr.Expr
	}
	if attr, ok := content.Attributes["error_message"]; ok {
		val, msgDiags := attr.Expr.Value(nil)
		diags = diags.Extend(msgDiags)
		if !msgDiags.HasErrors() && val.Type() == cty.String {
			validation.ErrorMessage = val.AsString()
		}
	}

	return validation, diags
}

// DecodeAssignments reads every attribute of a parsed vars-file body.
// Vars files are literals-only: expressions are evaluated with a nil
// EvalContext, so variable references and function calls produce
// hcl's natural "not allowed" diagnostics anchored in the vars file.
func DecodeAssignments(body hcl.Body) (map[string]Assignment, hcl.Diagnostics) {
	attrs, diags := body.JustAttributes()

	assigns := make(map[string]Assignment, len(attrs))
	for name, attr := range attrs {
		val, valDiags := attr.Expr.Value(nil)
		diags = diags.Extend(valDiags)
		if valDiags.HasErrors() {
			continue
		}
		assigns[name] = Assignment{
			Value:     val,
			NameRange: attr.NameRange,
			ExprRange: attr.Expr.Range(),
		}
	}
	return assigns, diags
}

// Resolve applies assignments to declarations: defaults for missing
// assignments, conversion to the declared type, then validation
// blocks. ctx is the caller's EvalContext (typically the Loader's) —
// defaults evaluate against it directly, and validation conditions
// against a child of it with var bound to the resolved values, so
// conditions reference var.<name> and loader functions fall through.
//
// Resolution is collect-all and follows the anchoring matrix:
// undeclared assignments error at the vars-file name, missing
// required variables at the declaration, conversion failures at the
// assignment expression, and validations run only over variables
// that resolved cleanly.
func Resolve(declared map[string]Variable, assigns map[string]Assignment, ctx *hcl.EvalContext) (*VarsResult, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	for name := range assigns {
		if _, ok := declared[name]; !ok {
			nameRange := assigns[name].NameRange
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Assignment to undeclared variable",
				Detail:   fmt.Sprintf("No variable %q is declared in the configuration.", name),
				Subject:  &nameRange,
			})
		}
	}

	values := make(map[string]cty.Value, len(declared))
	for name := range declared {
		decl := declared[name]
		val, ok, valDiags := resolveOne(&decl, assigns, ctx)
		diags = diags.Extend(valDiags)
		if ok {
			values[name] = val
		}
	}

	valuesObj := cty.ObjectVal(values)
	result := &VarsResult{Values: valuesObj, Declared: declared}
	diags = diags.Extend(validate(declared, values, valuesObj, ctx))
	return result, diags
}

// resolveOne produces the final value for one declaration: the
// assignment if present, otherwise the default, otherwise an error.
func resolveOne(decl *Variable, assigns map[string]Assignment, ctx *hcl.EvalContext) (cty.Value, bool, hcl.Diagnostics) {
	if assign, ok := assigns[decl.Name]; ok {
		converted, err := convert.Convert(assign.Value, decl.Type)
		if err != nil {
			exprRange := assign.ExprRange
			return cty.NilVal, false, hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  "Invalid value for variable",
				Detail: fmt.Sprintf("Variable %q expects %s: %s.",
					decl.Name, decl.Type.FriendlyName(), err),
				Subject: &exprRange,
			}}
		}
		return converted, true, nil
	}

	if decl.Default != nil {
		val, diags := decl.Default.Value(ctx)
		if diags.HasErrors() {
			return cty.NilVal, false, diags
		}
		converted, err := convert.Convert(val, decl.Type)
		if err != nil {
			defRange := decl.Default.Range()
			return cty.NilVal, false, hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  "Invalid default for variable",
				Detail: fmt.Sprintf("Variable %q expects %s: %s.",
					decl.Name, decl.Type.FriendlyName(), err),
				Subject: &defRange,
			}}
		}
		return converted, true, diags
	}

	declRange := decl.DeclRange
	return cty.NilVal, false, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "Missing value for required variable",
		Detail:   fmt.Sprintf("Variable %q has no assignment and no default.", decl.Name),
		Subject:  &declRange,
	}}
}

// validate runs every validation block over the variables that
// resolved cleanly. Conditions evaluate in a child context with var
// bound; Functions stays nil so lookup falls through to the caller's
// functions.
func validate(declared map[string]Variable, values map[string]cty.Value, valuesObj cty.Value, ctx *hcl.EvalContext) hcl.Diagnostics {
	var diags hcl.Diagnostics

	valCtx := &hcl.EvalContext{}
	if ctx != nil {
		valCtx = ctx.NewChild()
	}
	valCtx.Variables = map[string]cty.Value{"var": valuesObj}

	for name := range declared {
		if _, ok := values[name]; !ok {
			continue // resolution already failed; avoid cascading noise
		}
		validations := declared[name].Validations
		for i := range validations {
			validation := &validations[i]
			result, condDiags := validation.Condition.Value(valCtx)
			diags = diags.Extend(condDiags)
			if condDiags.HasErrors() {
				continue
			}
			cond, err := convert.Convert(result, cty.Bool)
			if err != nil || cond.IsNull() || !cond.IsKnown() {
				declRange := validation.DeclRange
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid validation condition",
					Detail:   fmt.Sprintf("Condition for variable %q must produce a known boolean.", name),
					Subject:  &declRange,
				})
				continue
			}
			if cond.False() {
				declRange := validation.DeclRange
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  validation.ErrorMessage,
					Detail:   fmt.Sprintf("Validation for variable %q failed.", name),
					Subject:  &declRange,
				})
			}
		}
	}
	return diags
}
