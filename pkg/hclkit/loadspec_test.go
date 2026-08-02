package hclkit_test

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/zclconf/go-cty/cty"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
	"github.com/donaldgifford/hclkit/pkg/hclkit/partial"
)

var loadSpecEager = hcldec.ObjectSpec{
	"project": &hcldec.AttrSpec{Name: "project", Type: cty.String, Required: true},
}

func TestLoadSpec(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "main.hcl", `
project = "demo"
when    = var.enabled
`)

	val, exprs, diags := hclkit.New().LoadSpec(path, loadSpecEager, "when")
	if diags.HasErrors() {
		t.Fatalf("LoadSpec() diags = %s, want none", diags)
	}
	if got := val.GetAttr("project"); got != cty.StringVal("demo") {
		t.Errorf("project = %#v, want demo", got)
	}

	// The retained expression evaluates later against a caller-built
	// context — forge's condition.when flow.
	when, ok := exprs["when"]
	if !ok {
		t.Fatal(`exprs["when"] missing`)
	}
	result, evalDiags := when.Value(&hcl.EvalContext{Variables: map[string]cty.Value{
		"var": cty.ObjectVal(map[string]cty.Value{"enabled": cty.True}),
	}})
	if evalDiags.HasErrors() || result != cty.True {
		t.Errorf("retained when = %#v (diags %s), want true", result, evalDiags)
	}
}

func TestLoadSpecMissingFile(t *testing.T) {
	_, _, diags := hclkit.New().LoadSpec(t.TempDir()+"/absent.hcl", loadSpecEager)
	if !diags.HasErrors() {
		t.Fatal("LoadSpec(missing) diags = none, want error")
	}
}

func TestLoadSpecUsesLoaderContext(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "main.hcl", "project = region\n")

	val, _, diags := hclkit.New(hclkit.WithVariables(map[string]cty.Value{
		"region": cty.StringVal("us-east-1"),
	})).LoadSpec(path, loadSpecEager)
	if diags.HasErrors() {
		t.Fatalf("LoadSpec() diags = %s, want none", diags)
	}
	if got := val.GetAttr("project"); got != cty.StringVal("us-east-1") {
		t.Errorf("project = %#v, want us-east-1 via loader variables", got)
	}
}

func TestLoadSpecVarsFileMode(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "main.hcl", `
variable "env" {
  type = string
}

project = var.env
`)
	varsPath := writeTempFile(t, dir, "app.vars.hcl", "env = \"prod\"\n")

	val, _, diags := hclkit.New(hclkit.WithVarsFile(varsPath)).LoadSpec(path, loadSpecEager)
	if diags.HasErrors() {
		t.Fatalf("LoadSpec() diags = %s, want none", diags)
	}
	if got := val.GetAttr("project"); got != cty.StringVal("prod") {
		t.Errorf("project = %#v, want prod (variable block stripped and var bound)", got)
	}
}

// TestLoadSpecForgeShape exercises the composed nested-retention
// pattern: eager spec decode plus Walk over condition blocks with a
// per-block DecodeSpec retaining "when".
func TestLoadSpecForgeShape(t *testing.T) {
	dir := t.TempDir()
	path := writeTempFile(t, dir, "main.hcl", `
project = "demo"

condition "has_readme" {
  severity = "low"
  when     = var.readme_present
}

condition "has_license" {
  severity = "high"
  when     = !var.license_present
}
`)

	var cfg struct {
		Project    string   `hcl:"project"`
		Conditions hcl.Body `hcl:",remain"`
	}
	if diags := hclkit.New().LoadFile(path, &cfg); diags.HasErrors() {
		t.Fatalf("LoadFile() diags = %s, want none", diags)
	}

	condSpec := hcldec.ObjectSpec{
		"severity": &hcldec.AttrSpec{Name: "severity", Type: cty.String, Required: true},
	}
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{{Type: "condition", LabelNames: []string{"name"}}},
	}

	type cond struct {
		name     string
		severity string
		when     hcl.Expression
	}
	var conds []cond
	walkDiags := partial.Walk(cfg.Conditions, schema, func(block *hcl.Block) hcl.Diagnostics {
		val, exprs, diags := partial.DecodeSpec(block.Body, condSpec, nil, "when")
		if diags.HasErrors() {
			return diags
		}
		conds = append(conds, cond{
			name:     block.Labels[0],
			severity: val.GetAttr("severity").AsString(),
			when:     exprs["when"],
		})
		return nil
	})
	if walkDiags.HasErrors() {
		t.Fatalf("Walk diags = %s, want none", walkDiags)
	}
	if len(conds) != 2 {
		t.Fatalf("conditions = %d, want 2", len(conds))
	}

	ctx := &hcl.EvalContext{Variables: map[string]cty.Value{
		"var": cty.ObjectVal(map[string]cty.Value{
			"readme_present":  cty.True,
			"license_present": cty.True,
		}),
	}}
	first, diags := conds[0].when.Value(ctx)
	if diags.HasErrors() || first != cty.True {
		t.Errorf("conds[0].when = %#v, want true", first)
	}
	second, diags := conds[1].when.Value(ctx)
	if diags.HasErrors() || second != cty.False {
		t.Errorf("conds[1].when = %#v, want false", second)
	}
	if conds[1].severity != "high" {
		t.Errorf("conds[1].severity = %q, want high", conds[1].severity)
	}
}
