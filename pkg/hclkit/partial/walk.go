package partial

import (
	"github.com/hashicorp/hcl/v2"
)

// WalkFunc is called once per visited block. Returned diagnostics are
// collected and the walk continues — collect-all, so one pass reports
// every problem.
type WalkFunc func(block *hcl.Block) hcl.Diagnostics

// Walk iterates body's blocks one kind at a time: kinds in
// schema.Blocks slice order, blocks within a kind in source order.
// List the kind that must decode first — e.g. locals — first in the
// schema.
//
// The walk is strict: body is checked against schema via Content, so
// blocks or attributes not in the schema are error diagnostics and
// nothing is visited. Attributes declared in the schema are validated
// but never delivered to fn — Walk is block-only.
func Walk(body hcl.Body, schema *hcl.BodySchema, fn WalkFunc) hcl.Diagnostics {
	if body == nil || schema == nil || fn == nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Invalid walk arguments",
			Detail:   "Walk requires a non-nil body, schema, and callback.",
		}}
	}

	content, diags := body.Content(schema)
	if diags.HasErrors() {
		return diags
	}

	for i := range schema.Blocks {
		for _, block := range content.Blocks.OfType(schema.Blocks[i].Type) {
			diags = diags.Extend(fn(block))
		}
	}
	return diags
}
