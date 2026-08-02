package validate

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
)

// UniqueValidator asserts that Attribute is unique across every block
// of kind BlockKind. Values are converted to string, so numeric IDs
// work. Duplicates anchor at the later occurrence with the first's
// range in the detail.
type UniqueValidator struct {
	BlockKind string
	Attribute string
}

// NewUniqueValidator returns a validator asserting attribute
// uniqueness across all blockKind blocks.
func NewUniqueValidator(blockKind, attribute string) *UniqueValidator {
	return &UniqueValidator{BlockKind: blockKind, Attribute: attribute}
}

// Validate implements the hclkit.Validator contract. A block missing
// the attribute is skipped — required-ness is the decode schema's
// job — as are unknown or null values and expressions that fail to
// evaluate (the decode pass reports those).
func (v *UniqueValidator) Validate(bodies []hcl.Body, ctx *hcl.EvalContext) hcl.Diagnostics {
	seen := make(map[string]hcl.Range)
	var diags hcl.Diagnostics

	walkBodies(bodies, nil, func(block *hclsyntax.Block) {
		if block.Type != v.BlockKind {
			return
		}
		attr, ok := block.Body.Attributes[v.Attribute]
		if !ok {
			return
		}

		val, valDiags := attr.Expr.Value(ctx)
		if valDiags.HasErrors() || val.IsNull() || !val.IsKnown() {
			return
		}
		exprRange := attr.Expr.Range()
		converted, err := convert.Convert(val, cty.String)
		if err != nil {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid attribute value",
				Detail: fmt.Sprintf("Attribute %q on %s blocks must be convertible to string.",
					v.Attribute, v.BlockKind),
				Subject: &exprRange,
			})
			return
		}

		s := converted.AsString()
		if first, dup := seen[s]; dup {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Duplicate %s %s", v.BlockKind, v.Attribute),
				Detail:   fmt.Sprintf("Value %q was already used at %s.", s, first),
				Subject:  &exprRange,
			})
			return
		}
		seen[s] = exprRange
	})
	return diags
}
