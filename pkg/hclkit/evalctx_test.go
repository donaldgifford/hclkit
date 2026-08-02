package hclkit_test

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
	"github.com/donaldgifford/hclkit/pkg/hclkit/funcs"
)

// parseBody parses native-syntax HCL source into a body for
// WithLocals tests.
func parseBody(t *testing.T, src string) hcl.Body {
	t.Helper()
	file, diags := hclparse.NewParser().ParseHCL([]byte(src), "locals.hcl")
	if diags.HasErrors() {
		t.Fatalf("ParseHCL(%q) diags = %s, want none", src, diags)
	}
	return file.Body
}

func TestBuildEmptyReturnsNil(t *testing.T) {
	ctx, diags := hclkit.NewEvalCtx().Build()
	if ctx != nil {
		t.Errorf("Build() ctx = %#v, want nil (nil-identity for empty builder)", ctx)
	}
	if diags.HasErrors() {
		t.Errorf("Build() diags = %s, want none", diags)
	}
}

func TestBuildZeroValueBuilder(t *testing.T) {
	var b hclkit.EvalCtxBuilder
	ctx, diags := b.WithVar("name", cty.StringVal("zero")).Build()
	if diags.HasErrors() {
		t.Fatalf("Build() diags = %s, want none", diags)
	}
	if got := ctx.Variables["name"]; got != cty.StringVal("zero") {
		t.Errorf(`Variables["name"] = %#v, want "zero"`, got)
	}
}

func TestBuildVarsAndFuncs(t *testing.T) {
	ctx, diags := hclkit.NewEvalCtx().
		WithVar("region", cty.StringVal("us-east-1")).
		WithFunc("env", funcs.Env(func(string) string { return "stub" })).
		Build()
	if diags.HasErrors() {
		t.Fatalf("Build() diags = %s, want none", diags)
	}
	if got := ctx.Variables["region"]; got != cty.StringVal("us-east-1") {
		t.Errorf(`Variables["region"] = %#v, want "us-east-1"`, got)
	}
	if _, ok := ctx.Functions["env"]; !ok {
		t.Error(`Functions["env"] missing, want registered function`)
	}
}

func TestBuildStdFuncs(t *testing.T) {
	ctx, diags := hclkit.NewEvalCtx().WithStdFuncs().Build()
	if diags.HasErrors() {
		t.Fatalf("Build() diags = %s, want none", diags)
	}
	for _, name := range []string{"snakeCase", "camelCase", "pascalCase", "kebabCase", "now", "env"} {
		if _, ok := ctx.Functions[name]; !ok {
			t.Errorf("Functions[%q] missing, want std bundle entry", name)
		}
	}
}

func TestWithFuncBeatsStdBundle(t *testing.T) {
	custom := funcs.Env(func(string) string { return "custom" })

	tests := []struct {
		name  string
		build func() (*hcl.EvalContext, hcl.Diagnostics)
	}{
		{
			name: "WithFunc before WithStdFuncs",
			build: func() (*hcl.EvalContext, hcl.Diagnostics) {
				return hclkit.NewEvalCtx().WithFunc("env", custom).WithStdFuncs().Build()
			},
		},
		{
			name: "WithFunc after WithStdFuncs",
			build: func() (*hcl.EvalContext, hcl.Diagnostics) {
				return hclkit.NewEvalCtx().WithStdFuncs().WithFunc("env", custom).Build()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, diags := tt.build()
			if diags.HasErrors() {
				t.Fatalf("Build() diags = %s, want none", diags)
			}
			got, err := ctx.Functions["env"].Call([]cty.Value{cty.StringVal("ANY")})
			if err != nil {
				t.Fatalf("env(ANY) error = %v, want nil", err)
			}
			if got.AsString() != "custom" {
				t.Errorf("env(ANY) = %q, want %q (WithFunc must beat std bundle)", got.AsString(), "custom")
			}
		})
	}
}

func TestBuildLocals(t *testing.T) {
	body := parseBody(t, `
greeting = "hi"
shout    = pascalCase("hello world")
msg      = base
`)

	ctx, diags := hclkit.NewEvalCtx().
		WithStdFuncs().
		WithVar("base", cty.StringVal("from-var")).
		WithLocals(body).
		Build()
	if diags.HasErrors() {
		t.Fatalf("Build() diags = %s, want none", diags)
	}

	local := ctx.Variables["local"]
	want := map[string]string{
		"greeting": "hi",
		"shout":    "HelloWorld",
		"msg":      "from-var",
	}
	for name, wantVal := range want {
		if got := local.GetAttr(name); got != cty.StringVal(wantVal) {
			t.Errorf("local.%s = %#v, want %q", name, got, wantVal)
		}
	}
}

func TestBuildLocalsSiblingReferenceFails(t *testing.T) {
	body := parseBody(t, `
a = "x"
b = local.a
`)

	ctx, diags := hclkit.NewEvalCtx().WithLocals(body).Build()
	if !diags.HasErrors() {
		t.Fatal("Build() diags = none, want error (locals are single-pass; sibling refs unsupported)")
	}

	local := ctx.Variables["local"]
	if got := local.GetAttr("a"); got != cty.StringVal("x") {
		t.Errorf(`local.a = %#v, want "x" (collect-all keeps the healthy attrs)`, got)
	}
	if local.Type().HasAttribute("b") {
		t.Error("local.b present, want omitted after failed evaluation")
	}
}

func TestBuildLocalsLaterBodyWins(t *testing.T) {
	ctx, diags := hclkit.NewEvalCtx().
		WithLocals(parseBody(t, `name = "first"`)).
		WithLocals(parseBody(t, `name = "second"`)).
		Build()
	if diags.HasErrors() {
		t.Fatalf("Build() diags = %s, want none", diags)
	}
	if got := ctx.Variables["local"].GetAttr("name"); got != cty.StringVal("second") {
		t.Errorf(`local.name = %#v, want "second" (later WithLocals wins)`, got)
	}
}

func TestBuildLocalsWinOverLocalVar(t *testing.T) {
	ctx, diags := hclkit.NewEvalCtx().
		WithVar("local", cty.ObjectVal(map[string]cty.Value{"name": cty.StringVal("var")})).
		WithLocals(parseBody(t, `name = "locals"`)).
		Build()
	if diags.HasErrors() {
		t.Fatalf("Build() diags = %s, want none", diags)
	}
	if got := ctx.Variables["local"].GetAttr("name"); got != cty.StringVal("locals") {
		t.Errorf(`local.name = %#v, want "locals" (locals bind last)`, got)
	}
}

func TestBuildDoesNotAliasBuilderState(t *testing.T) {
	b := hclkit.NewEvalCtx().WithVar("first", cty.StringVal("1"))

	ctx, diags := b.Build()
	if diags.HasErrors() {
		t.Fatalf("Build() diags = %s, want none", diags)
	}

	b.WithVar("second", cty.StringVal("2"))
	if _, ok := ctx.Variables["second"]; ok {
		t.Error(`Variables["second"] present in earlier ctx, want Build output isolated from later With* calls`)
	}

	rebuilt, diags := b.Build()
	if diags.HasErrors() {
		t.Fatalf("Build() diags = %s, want none", diags)
	}
	if _, ok := rebuilt.Variables["second"]; !ok {
		t.Error(`Variables["second"] missing from rebuilt ctx, want Build repeatable over current state`)
	}
}

func TestBuilderComposesWithLoader(t *testing.T) {
	t.Setenv("HCLKIT_BUILDER_E2E", "composed")

	ctx, diags := hclkit.NewEvalCtx().WithStdFuncs().Build()
	if diags.HasErrors() {
		t.Fatalf("Build() diags = %s, want none", diags)
	}

	var cfg struct {
		Name string `hcl:"name"`
	}
	loadDiags := hclkit.New(hclkit.WithEvalContext(ctx)).
		LoadBytes("config.hcl", []byte(`name = env("HCLKIT_BUILDER_E2E")`), &cfg)
	if loadDiags.HasErrors() {
		t.Fatalf("LoadBytes() diags = %s, want none", loadDiags)
	}
	if cfg.Name != "composed" {
		t.Errorf("cfg.Name = %q, want %q", cfg.Name, "composed")
	}
}
