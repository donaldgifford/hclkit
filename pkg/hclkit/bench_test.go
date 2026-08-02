package hclkit_test

// Load+decode benchmarks over representative consumer configs: a
// forge-shaped blueprint (vars file + std funcs + eager decode + Walk
// with retained `when` expressions) and a repo-guardian-shaped policy
// (locals-first walk feeding an EvalCtxBuilder, then per-rule spec
// decode). `just bench` runs these.

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
	"github.com/donaldgifford/hclkit/pkg/hclkit/funcs"
	"github.com/donaldgifford/hclkit/pkg/hclkit/partial"
)

const (
	benchBlueprintPath = "testdata/bench/blueprint.hcl"
	benchVarsPath      = "testdata/bench/blueprint.vars.hcl"
	benchPolicyPath    = "testdata/bench/policy.hcl"
)

type benchBlueprint struct {
	Name        string   `hcl:"name"`
	Description string   `hcl:"description,optional"`
	Version     string   `hcl:"version,optional"`
	Tags        []string `hcl:"tags,optional"`
	Remain      hcl.Body `hcl:",remain"`
}

var (
	benchCondSpec = hcldec.ObjectSpec{
		"exclude": &hcldec.AttrSpec{Name: "exclude", Type: cty.List(cty.String)},
	}
	benchRenameSpec = hcldec.ObjectSpec{
		"from": &hcldec.AttrSpec{Name: "from", Type: cty.String, Required: true},
		"to":   &hcldec.AttrSpec{Name: "to", Type: cty.String, Required: true},
	}
	benchBlueprintSchema = &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "condition"}, {Type: "rename"}},
	}
)

// BenchmarkLoadBlueprint is the full forge blueprint flow: resolve
// the vars file, load the eager attributes with variable blocks
// stripped, walk condition/rename blocks retaining `when`, and
// evaluate the retained expressions against a caller-built context.
func BenchmarkLoadBlueprint(b *testing.B) {
	loader := hclkit.New(
		hclkit.WithFunctions(funcs.Std()),
		hclkit.WithVarsFile(benchVarsPath),
	)

	b.ReportAllocs()
	for b.Loop() {
		vars, diags := loader.LoadVarsFile(benchBlueprintPath, benchVarsPath)
		if diags.HasErrors() {
			b.Fatalf("LoadVarsFile: %s", diags.Error())
		}
		ctx, ctxDiags := hclkit.NewEvalCtx().
			WithStdFuncs().
			WithVar("var", vars.Values).
			Build()
		if ctxDiags.HasErrors() {
			b.Fatalf("Build: %s", ctxDiags)
		}

		var bp benchBlueprint
		if diags := loader.LoadFile(benchBlueprintPath, &bp); diags.HasErrors() {
			b.Fatalf("LoadFile: %s", diags.Error())
		}

		walkDiags := partial.Walk(bp.Remain, benchBlueprintSchema, func(block *hcl.Block) hcl.Diagnostics {
			if block.Type == "condition" {
				_, exprs, diags := partial.DecodeSpec(block.Body, benchCondSpec, ctx, "when")
				if diags.HasErrors() {
					return diags
				}
				if _, evalDiags := exprs["when"].Value(ctx); evalDiags.HasErrors() {
					return evalDiags
				}
				return nil
			}
			_, _, diags := partial.DecodeSpec(block.Body, benchRenameSpec, ctx)
			return diags
		})
		if walkDiags.HasErrors() {
			b.Fatalf("Walk: %s", walkDiags)
		}
	}
}

type benchGuardian struct {
	LogLevel         string `hcl:"log_level,optional"`
	DryRun           bool   `hcl:"dry_run,optional"`
	WorkerCount      int    `hcl:"worker_count,optional"`
	QueueSize        int    `hcl:"queue_size,optional"`
	ScheduleInterval string `hcl:"schedule_interval,optional"`
	SkipForks        bool   `hcl:"skip_forks,optional"`
}

type benchPolicyShell struct {
	Guardian benchGuardian `hcl:"guardian,block"`
	Remain   hcl.Body      `hcl:",remain"`
}

var (
	benchPolicySchema = &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "locals"},
			{Type: "rule", LabelNames: []string{"kind", "name"}},
		},
	}
	benchRuleSpec = hcldec.ObjectSpec{
		"path":     &hcldec.AttrSpec{Name: "path", Type: cty.String, Required: true},
		"mode":     &hcldec.AttrSpec{Name: "mode", Type: cty.String},
		"team":     &hcldec.AttrSpec{Name: "team", Type: cty.String},
		"labels":   &hcldec.AttrSpec{Name: "labels", Type: cty.List(cty.String)},
		"template": &hcldec.AttrSpec{Name: "template", Type: cty.String},
	}
)

// BenchmarkLoadPolicy is the repo-guardian flow: decode the guardian
// block eagerly, walk locals first into an EvalCtxBuilder, then
// spec-decode every rule block against the locals-populated context.
func BenchmarkLoadPolicy(b *testing.B) {
	loader := hclkit.New()

	b.ReportAllocs()
	for b.Loop() {
		var shell benchPolicyShell
		if diags := loader.LoadFile(benchPolicyPath, &shell); diags.HasErrors() {
			b.Fatalf("LoadFile: %s", diags.Error())
		}

		builder := hclkit.NewEvalCtx().WithStdFuncs()
		var rules []*hcl.Block
		walkDiags := partial.Walk(shell.Remain, benchPolicySchema, func(block *hcl.Block) hcl.Diagnostics {
			if block.Type == "locals" {
				builder.WithLocals(block.Body)
				return nil
			}
			rules = append(rules, block)
			return nil
		})
		if walkDiags.HasErrors() {
			b.Fatalf("Walk: %s", walkDiags)
		}

		ctx, ctxDiags := builder.Build()
		if ctxDiags.HasErrors() {
			b.Fatalf("Build: %s", ctxDiags)
		}
		for _, rule := range rules {
			if _, _, diags := partial.DecodeSpec(rule.Body, benchRuleSpec, ctx); diags.HasErrors() {
				b.Fatalf("DecodeSpec(%v): %s", rule.Labels, diags.Error())
			}
		}
		if len(rules) != 4 {
			b.Fatalf("rules = %d, want 4", len(rules))
		}
	}
}

// BenchmarkLoadVarsFileResolve isolates the vars-file resolution path
// (declarations, assignments, defaults, validations).
func BenchmarkLoadVarsFileResolve(b *testing.B) {
	loader := hclkit.New(hclkit.WithFunctions(funcs.Std()))

	b.ReportAllocs()
	for b.Loop() {
		if _, diags := loader.LoadVarsFile(benchBlueprintPath, benchVarsPath); diags.HasErrors() {
			b.Fatalf("LoadVarsFile: %s", diags.Error())
		}
	}
}
