package partial

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

func parseBody(t *testing.T, src string) hcl.Body {
	t.Helper()
	file, diags := hclparse.NewParser().ParseHCL([]byte(src), "test.hcl")
	if diags.HasErrors() {
		t.Fatalf("parsing: %s", diags)
	}
	return file.Body
}

func parseJSONBody(t *testing.T, src string) hcl.Body {
	t.Helper()
	file, diags := hclparse.NewParser().ParseJSON([]byte(src), "test.json")
	if diags.HasErrors() {
		t.Fatalf("parsing json: %s", diags)
	}
	return file.Body
}

var eagerSpec = hcldec.ObjectSpec{
	"name": &hcldec.AttrSpec{Name: "name", Type: cty.String, Required: true},
	"size": &hcldec.AttrSpec{Name: "size", Type: cty.Number},
}

func TestDecodeSpecRetains(t *testing.T) {
	body := parseBody(t, `
name = "demo"
size = 4
when = var.enabled
`)

	val, exprs, diags := DecodeSpec(body, eagerSpec, nil, "when")
	if diags.HasErrors() {
		t.Fatalf("DecodeSpec diags = %s, want none", diags)
	}
	if got := val.GetAttr("name"); got != cty.StringVal("demo") {
		t.Errorf("name = %#v, want demo", got)
	}
	if !val.GetAttr("size").RawEquals(cty.NumberIntVal(4)) {
		t.Errorf("size = %#v, want 4", val.GetAttr("size"))
	}

	when, ok := exprs["when"]
	if !ok {
		t.Fatal(`exprs["when"] missing, want retained expression`)
	}
	// The retained expression evaluates later against a caller ctx.
	result, evalDiags := when.Value(&hcl.EvalContext{Variables: map[string]cty.Value{
		"var": cty.ObjectVal(map[string]cty.Value{"enabled": cty.True}),
	}})
	if evalDiags.HasErrors() || result != cty.True {
		t.Errorf("retained when = %#v (diags %s), want true", result, evalDiags)
	}
}

func TestDecodeSpecRetainAbsent(t *testing.T) {
	body := parseBody(t, "name = \"demo\"\n")

	_, exprs, diags := DecodeSpec(body, eagerSpec, nil, "when")
	if diags.HasErrors() {
		t.Fatalf("DecodeSpec diags = %s, want none (absent retained attr is not an error)", diags)
	}
	if _, ok := exprs["when"]; ok {
		t.Error(`exprs["when"] present, want absent for a missing attribute`)
	}
}

func TestDecodeSpecRetainConflict(t *testing.T) {
	body := parseBody(t, "name = \"demo\"\n")

	_, _, diags := DecodeSpec(body, eagerSpec, nil, "name")
	if !diags.HasErrors() {
		t.Fatal("DecodeSpec diags = none, want conflict error")
	}
	if !strings.Contains(diags.Error(), "Conflicting retained attribute") {
		t.Errorf("diags = %q, want conflict summary", diags.Error())
	}
}

func TestDecodeSpecInvalidArguments(t *testing.T) {
	body := parseBody(t, "name = \"demo\"\n")

	_, _, diags := DecodeSpec(body, nil, nil)
	if !diags.HasErrors() || !strings.Contains(diags.Error(), "Invalid decode spec") {
		t.Errorf("DecodeSpec(nil spec) diags = %q, want invalid-spec error", diags.Error())
	}

	_, _, diags = DecodeSpec(nil, eagerSpec, nil)
	if !diags.HasErrors() || !strings.Contains(diags.Error(), "Invalid decode spec") {
		t.Errorf("DecodeSpec(nil body) diags = %q, want invalid-spec error", diags.Error())
	}
}

func TestDecodeSpecDuplicateRetainNames(t *testing.T) {
	body := parseBody(t, "name = \"demo\"\nwhen = true\n")

	_, exprs, diags := DecodeSpec(body, eagerSpec, nil, "when", "when")
	if diags.HasErrors() {
		t.Fatalf("DecodeSpec diags = %s, want none for duplicate retain names", diags)
	}
	if len(exprs) != 1 {
		t.Errorf("exprs len = %d, want 1", len(exprs))
	}
}

func TestDecodeSpecWithContext(t *testing.T) {
	body := parseBody(t, "name = upper(\"demo\")\n")
	ctx := &hcl.EvalContext{Functions: nil}

	// Eager decode with a function reference and no functions in ctx
	// fails with hcl's own diagnostics — partial value still returns.
	val, _, diags := DecodeSpec(body, eagerSpec, ctx)
	if !diags.HasErrors() {
		t.Fatal("DecodeSpec diags = none, want unknown-function error")
	}
	if val == cty.NilVal {
		t.Error("val = NilVal, want hcldec's partial object value alongside diags")
	}
}

func TestDecodeSpecJSONBody(t *testing.T) {
	body := parseJSONBody(t, `{"name": "demo", "when": "later"}`)

	val, exprs, diags := DecodeSpec(body, eagerSpec, nil, "when")
	if diags.HasErrors() {
		t.Fatalf("DecodeSpec(json) diags = %s, want none", diags)
	}
	if got := val.GetAttr("name"); got != cty.StringVal("demo") {
		t.Errorf("name = %#v, want demo", got)
	}
	if _, ok := exprs["when"]; !ok {
		t.Error(`exprs["when"] missing for JSON-syntax body`)
	}
}
