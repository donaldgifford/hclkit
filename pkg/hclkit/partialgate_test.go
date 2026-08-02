package hclkit_test

// The partial-decode v1.0 gate (IMPL-0001 Phase 4, OQ-6): run the
// partial surfaces end-to-end against vendored snapshots of real
// consumer configs, including EvalContext and late-bound expression
// flows. Fixture provenance: internal/testutil/fixtures/README.md.

import (
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
	"github.com/donaldgifford/hclkit/pkg/hclkit/ctytypes"
	"github.com/donaldgifford/hclkit/pkg/hclkit/partial"
)

const (
	forgeBlueprintFixture = "../../internal/testutil/fixtures/forge-blueprint.hcl"
	guardianPolicyFixture = "../../internal/testutil/fixtures/repo-guardian-policy.hcl"
)

// TestPartialGateForgeBlueprint mirrors forge's real flow: eager
// top-level decode, walk over variable/condition/rename blocks, and
// late-bound evaluation of retained expressions once variables are
// bound.
func TestPartialGateForgeBlueprint(t *testing.T) {
	var shell struct {
		Name        string   `hcl:"name"`
		Description string   `hcl:"description,optional"`
		Version     string   `hcl:"version,optional"`
		Tags        []string `hcl:"tags,optional"`
		Remain      hcl.Body `hcl:",remain"`
	}
	if diags := hclkit.New().LoadFile(forgeBlueprintFixture, &shell); diags.HasErrors() {
		t.Fatalf("LoadFile: %s", diags.Error())
	}
	if shell.Name != "go-api" || shell.Version != "2.0.0" || len(shell.Tags) != 3 {
		t.Errorf("eager decode = %+v, want go-api/2.0.0/3 tags", shell)
	}

	schema := &hcl.BodySchema{Blocks: []hcl.BlockHeaderSchema{
		{Type: "variable", LabelNames: []string{"name"}},
		{Type: "condition"},
		{Type: "rename"},
	}}
	condSpec := hcldec.ObjectSpec{
		"exclude": &hcldec.AttrSpec{Name: "exclude", Type: cty.List(cty.String)},
	}
	entrySchema := &hcl.BodySchema{Blocks: []hcl.BlockHeaderSchema{{Type: "entry"}}}

	var varNames []string
	var whens []hcl.Expression
	var entryFroms []hcl.Expression
	walkDiags := partial.Walk(shell.Remain, schema, func(block *hcl.Block) hcl.Diagnostics {
		switch block.Type {
		case "variable":
			varNames = append(varNames, block.Labels[0])
			return nil
		case "condition":
			_, exprs, diags := partial.DecodeSpec(block.Body, condSpec, nil, "when")
			if !diags.HasErrors() {
				whens = append(whens, exprs["when"])
			}
			return diags
		default: // rename { entry { from, to } }
			return partial.Walk(block.Body, entrySchema, func(entry *hcl.Block) hcl.Diagnostics {
				_, exprs, diags := partial.DecodeSpec(entry.Body, hcldec.ObjectSpec{}, nil, "from", "to")
				if !diags.HasErrors() {
					entryFroms = append(entryFroms, exprs["from"])
				}
				return diags
			})
		}
	})
	if walkDiags.HasErrors() {
		t.Fatalf("Walk: %s", walkDiags)
	}

	if len(varNames) != 3 || varNames[0] != "project_name" {
		t.Errorf("variables = %v, want [project_name go_module use_grpc]", varNames)
	}
	if len(whens) != 1 || len(entryFroms) != 1 {
		t.Fatalf("retained: whens=%d entryFroms=%d, want 1 and 1", len(whens), len(entryFroms))
	}

	// Late-bound flow: the context exists only now, after "variable
	// resolution" — forge's exact shape (bare names, not var.*).
	ctx := &hcl.EvalContext{Variables: map[string]cty.Value{
		"use_grpc":     cty.True,
		"project_name": cty.StringVal("payments"),
	}}
	when, diags := whens[0].Value(ctx)
	if diags.HasErrors() || when != cty.False {
		t.Errorf("when = %#v (diags %s), want false with use_grpc=true", when, diags)
	}
	from, diags := entryFroms[0].Value(ctx)
	if diags.HasErrors() || from != cty.StringVal("payments/") {
		t.Errorf("entry.from = %#v (diags %s), want %q via template interpolation", from, diags, "payments/")
	}
}

// TestPartialGateGuardianPolicy mirrors repo-guardian's real flow:
// ordered block-kind walk with per-block PartialContent, plus the
// refined-duration tie-in on a real attribute position.
func TestPartialGateGuardianPolicy(t *testing.T) {
	var shell struct {
		Remain hcl.Body `hcl:",remain"`
	}
	if diags := hclkit.New().LoadFile(guardianPolicyFixture, &shell); diags.HasErrors() {
		t.Fatalf("LoadFile: %s", diags.Error())
	}

	schema := &hcl.BodySchema{Blocks: []hcl.BlockHeaderSchema{
		{Type: "guardian"},
		{Type: "defaults"},
		{Type: "ignore"},
		{Type: "rule", LabelNames: []string{"type", "name"}},
	}}

	var order []string
	var ruleNames []string
	var scheduleInterval time.Duration
	walkDiags := partial.Walk(shell.Remain, schema, func(block *hcl.Block) hcl.Diagnostics {
		order = append(order, block.Type)
		if block.Type == "rule" {
			ruleNames = append(ruleNames, block.Labels[1])
			return nil
		}
		if block.Type != "guardian" {
			return nil
		}
		// repo-guardian's per-kind PartialContent + eval shape, with
		// the ctytypes duration helper on a real source position.
		content, _, diags := block.Body.PartialContent(&hcl.BodySchema{
			Attributes: []hcl.AttributeSchema{{Name: "schedule_interval"}},
		})
		if diags.HasErrors() {
			return diags
		}
		d, dDiags := ctytypes.DecodeDuration(content.Attributes["schedule_interval"].Expr, nil)
		scheduleInterval = d
		return dDiags
	})
	if walkDiags.HasErrors() {
		t.Fatalf("Walk: %s", walkDiags)
	}

	// Kinds visit in schema order regardless of source interleaving.
	if order[0] != "guardian" || order[1] != "defaults" || order[2] != "ignore" {
		t.Errorf("order = %v, want guardian, defaults, ignore first", order[:3])
	}
	if len(ruleNames) != 12 {
		t.Errorf("rules = %d (%v), want 12", len(ruleNames), ruleNames)
	}
	if scheduleInterval != 168*time.Hour {
		t.Errorf("schedule_interval = %v, want 168h", scheduleInterval)
	}
}
