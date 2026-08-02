// Package lintschema loads the schema files consumed by
// `hclkit lint --schema` and maps them onto library validators. The
// grammar is deliberately internal: DESIGN-0001 reserves the
// top-level kinds (block, attribute, reference, unique) but the
// attribute names may still evolve before v1.0.
package lintschema

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/typeexpr"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty/convert"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
	"github.com/donaldgifford/hclkit/pkg/hclkit/validate"
)

// Schema is a decoded lint schema: declared block kinds, per-kind
// attribute rules, reference relationships, and uniqueness
// constraints.
type Schema struct {
	Blocks     []BlockRule     `hcl:"block,block"`
	Attributes []AttributeRule `hcl:"attribute,block"`
	References []ReferenceRule `hcl:"reference,block"`
	Uniques    []UniqueRule    `hcl:"unique,block"`
}

// BlockRule declares a permitted top-level block kind and how many
// labels its blocks carry.
type BlockRule struct {
	Kind   string `hcl:"kind,label"`
	Labels int    `hcl:"labels,optional"`
}

// AttributeRule constrains one attribute on blocks of a kind. Type is
// a typeexpr expression ("string", "list(string)"), checked by
// conversion; it stays an expression so absence is detectable.
type AttributeRule struct {
	Name     string         `hcl:"name,label"`
	Block    string         `hcl:"block"`
	Required bool           `hcl:"required,optional"`
	Type     hcl.Expression `hcl:"type,optional"`
}

// ReferenceRule maps onto validate.RefValidator.
type ReferenceRule struct {
	Verb       string `hcl:"verb"`
	TargetKind string `hcl:"target_kind"`
}

// UniqueRule maps onto validate.UniqueValidator.
type UniqueRule struct {
	BlockKind string `hcl:"block_kind"`
	Attribute string `hcl:"attribute"`
}

// Load decodes the schema file at path with a zero-config hclkit
// Loader — the schema grammar is itself plain HCL.
func Load(path string) (*Schema, hclkit.Diagnostics) {
	var schema Schema
	diags := hclkit.New().LoadFile(path, &schema)
	if diags.HasErrors() {
		return nil, diags
	}
	return &schema, diags
}

// Validators returns the validator set the schema declares: the
// structural checker (block kinds, label counts, attribute rules)
// plus one validator per reference and unique declaration.
func (s *Schema) Validators() []hclkit.Validator {
	vs := make([]hclkit.Validator, 0, len(s.References)+len(s.Uniques)+1)
	vs = append(vs, &structureValidator{schema: s})
	for i := range s.References {
		vs = append(vs, validate.NewRefValidator(s.References[i].Verb, s.References[i].TargetKind))
	}
	for i := range s.Uniques {
		vs = append(vs, validate.NewUniqueValidator(s.Uniques[i].BlockKind, s.Uniques[i].Attribute))
	}
	return vs
}

// structureValidator enforces the block and attribute rules. Like the
// validate package it is native-syntax only; JSON bodies are skipped.
type structureValidator struct {
	schema *Schema
}

// Validate checks top-level block kinds and label counts (only when
// the schema declares at least one block rule), then attribute
// presence and type per rule. Expressions that fail to evaluate
// without a context are skipped — lint has no variables in scope.
func (v *structureValidator) Validate(bodies []hcl.Body, _ *hcl.EvalContext) hcl.Diagnostics {
	var diags hcl.Diagnostics

	kinds := make(map[string]int, len(v.schema.Blocks))
	for i := range v.schema.Blocks {
		kinds[v.schema.Blocks[i].Kind] = v.schema.Blocks[i].Labels
	}

	for _, body := range bodies {
		syn, ok := body.(*hclsyntax.Body)
		if !ok {
			continue
		}
		for _, block := range syn.Blocks {
			diags = diags.Extend(v.checkBlock(block, kinds))
		}
	}
	return diags
}

func (v *structureValidator) checkBlock(block *hclsyntax.Block, kinds map[string]int) hcl.Diagnostics {
	var diags hcl.Diagnostics

	if len(kinds) > 0 {
		labels, known := kinds[block.Type]
		defRange := block.DefRange()
		if !known {
			return hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  "Unknown block kind",
				Detail:   fmt.Sprintf("Block kind %q is not declared in the schema.", block.Type),
				Subject:  &defRange,
			}}
		}
		if len(block.Labels) != labels {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Wrong label count",
				Detail: fmt.Sprintf("Block kind %q takes %d label(s); this block has %d.",
					block.Type, labels, len(block.Labels)),
				Subject: &defRange,
			})
		}
	}

	for i := range v.schema.Attributes {
		diags = diags.Extend(v.checkAttrRule(block, &v.schema.Attributes[i]))
	}
	return diags
}

func (*structureValidator) checkAttrRule(block *hclsyntax.Block, rule *AttributeRule) hcl.Diagnostics {
	if block.Type != rule.Block {
		return nil
	}

	attr, ok := block.Body.Attributes[rule.Name]
	if !ok {
		if !rule.Required {
			return nil
		}
		defRange := block.DefRange()
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Missing required attribute",
			Detail:   fmt.Sprintf("Blocks of kind %q require attribute %q.", rule.Block, rule.Name),
			Subject:  &defRange,
		}}
	}
	if rule.Type == nil {
		return nil
	}

	want, tyDiags := typeexpr.Type(rule.Type)
	if tyDiags.HasErrors() {
		return tyDiags // schema author error, anchored in the schema file
	}
	val, valDiags := attr.Expr.Value(nil)
	if valDiags.HasErrors() || val.IsNull() || !val.IsKnown() {
		return nil // lint has no eval context; only literals are checked
	}
	if _, err := convert.Convert(val, want); err != nil {
		exprRange := attr.Expr.Range()
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Invalid attribute type",
			Detail: fmt.Sprintf("Attribute %q on %q blocks must be %s: %s.",
				rule.Name, rule.Block, want.FriendlyName(), err),
			Subject: &exprRange,
		}}
	}
	return nil
}
