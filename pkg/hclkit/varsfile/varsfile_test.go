package varsfile_test

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	"github.com/donaldgifford/hclkit/pkg/hclkit/funcs"
	"github.com/donaldgifford/hclkit/pkg/hclkit/varsfile"
)

func parseBody(t *testing.T, filename, src string) hcl.Body {
	t.Helper()
	file, diags := hclparse.NewParser().ParseHCL([]byte(src), filename)
	if diags.HasErrors() {
		t.Fatalf("ParseHCL(%s) diags = %s, want none", filename, diags)
	}
	return file.Body
}

const declSrc = `
variable "name" {
  type    = string
  default = "untitled"
}

variable "count" {
  type = number

  validation {
    condition     = var.count > 0
    error_message = "count must be positive"
  }
}

service "web" {
  port = 8080
}
`

func decodeDecls(t *testing.T, src string) (map[string]varsfile.Variable, hcl.Body) {
	t.Helper()
	decls, remain, diags := varsfile.DecodeVariables(parseBody(t, "main.hcl", src))
	if diags.HasErrors() {
		t.Fatalf("DecodeVariables() diags = %s, want none", diags)
	}
	return decls, remain
}

func TestDecodeVariables(t *testing.T) {
	decls, remain := decodeDecls(t, declSrc)

	if len(decls) != 2 {
		t.Fatalf("DecodeVariables() len = %d, want 2", len(decls))
	}

	name := decls["name"]
	if name.Type != cty.String {
		t.Errorf("name.Type = %#v, want cty.String", name.Type)
	}
	if name.Default == nil {
		t.Error("name.Default = nil, want default expression")
	}

	count := decls["count"]
	if len(count.Validations) != 1 {
		t.Fatalf("count.Validations len = %d, want 1", len(count.Validations))
	}
	if got := count.Validations[0].ErrorMessage; got != "count must be positive" {
		t.Errorf("ErrorMessage = %q, want %q", got, "count must be positive")
	}

	// The remain body must still carry the service block and no
	// variable blocks — the consumer's decode sees a stripped body.
	content, _, diags := remain.PartialContent(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{Type: "service", LabelNames: []string{"name"}},
			{Type: "variable", LabelNames: []string{"name"}},
		},
	})
	if diags.HasErrors() {
		t.Fatalf("remain.PartialContent() diags = %s, want none", diags)
	}
	kinds := map[string]int{}
	for _, b := range content.Blocks {
		kinds[b.Type]++
	}
	if kinds["service"] != 1 || kinds["variable"] != 0 {
		t.Errorf("remain blocks = %v, want service present and variable stripped", kinds)
	}
}

func TestDecodeVariablesDiagnostics(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		wantIn string
	}{
		{
			name: "duplicate declaration",
			src: `
variable "region" { type = string }
variable "region" { type = string }
`,
			wantIn: "Duplicate variable declaration",
		},
		{
			name:   "missing type",
			src:    `variable "region" {}`,
			wantIn: "Missing required argument",
		},
		{
			name: "validation missing error_message",
			src: `
variable "region" {
  type = string
  validation {
    condition = var.region != ""
  }
}
`,
			wantIn: "Missing required argument",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, diags := varsfile.DecodeVariables(parseBody(t, "main.hcl", tt.src))
			if !diags.HasErrors() {
				t.Fatal("DecodeVariables() diags = none, want error")
			}
			if !strings.Contains(diags.Error(), tt.wantIn) {
				t.Errorf("DecodeVariables() diags = %q, want mention of %q", diags.Error(), tt.wantIn)
			}
		})
	}
}

func TestDecodeAssignmentsLiteralsOnly(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
	}{
		{name: "literals", src: "name = \"demo\"\nsize = 3"},
		{name: "variable reference rejected", src: "name = other", wantErr: true},
		{name: "function call rejected", src: `name = env("HOME")`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assigns, diags := varsfile.DecodeAssignments(parseBody(t, "vars.hcl", tt.src))
			if got := diags.HasErrors(); got != tt.wantErr {
				t.Fatalf("DecodeAssignments() HasErrors() = %v, want %v (diags: %s)", got, tt.wantErr, diags)
			}
			if !tt.wantErr && len(assigns) == 0 {
				t.Error("DecodeAssignments() = empty, want assignments")
			}
		})
	}
}

func TestResolve(t *testing.T) {
	decls, _ := decodeDecls(t, declSrc)

	assigns, diags := varsfile.DecodeAssignments(parseBody(t, "vars.hcl", `count = 3`))
	if diags.HasErrors() {
		t.Fatalf("DecodeAssignments() diags = %s, want none", diags)
	}

	result, diags := varsfile.Resolve(decls, assigns, nil)
	if diags.HasErrors() {
		t.Fatalf("Resolve() diags = %s, want none", diags)
	}

	if got := result.Values.GetAttr("name"); got != cty.StringVal("untitled") {
		t.Errorf(`var.name = %#v, want default "untitled"`, got)
	}
	if got := result.Values.GetAttr("count"); !got.RawEquals(cty.NumberIntVal(3)) {
		t.Errorf("var.count = %#v, want 3", got)
	}
	if len(result.Declared) != 2 {
		t.Errorf("Declared len = %d, want 2", len(result.Declared))
	}
}

func TestResolveDiagnosticsMatrix(t *testing.T) {
	tests := []struct {
		name     string
		declSrc  string
		varsSrc  string
		wantIn   string
		wantFile string // file the diagnostic must anchor in
	}{
		{
			name: "undeclared assignment anchors in vars file",
			declSrc: `
variable "known" {
  type    = string
  default = "x"
}
`,
			varsSrc:  `unknown = "y"`,
			wantIn:   "Assignment to undeclared variable",
			wantFile: "vars.hcl",
		},
		{
			name:     "missing required anchors at declaration",
			declSrc:  `variable "required" { type = string }`,
			varsSrc:  ``,
			wantIn:   "Missing value for required variable",
			wantFile: "main.hcl",
		},
		{
			name:     "conversion failure anchors at assignment",
			declSrc:  `variable "port" { type = number }`,
			varsSrc:  `port = "not-a-number"`,
			wantIn:   "Invalid value for variable",
			wantFile: "vars.hcl",
		},
		{
			name: "failed validation reports error_message",
			declSrc: `
variable "count" {
  type = number
  validation {
    condition     = var.count > 0
    error_message = "count must be positive"
  }
}
`,
			varsSrc:  `count = -1`,
			wantIn:   "count must be positive",
			wantFile: "main.hcl",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decls, _, diags := varsfile.DecodeVariables(parseBody(t, "main.hcl", tt.declSrc))
			if diags.HasErrors() {
				t.Fatalf("DecodeVariables() diags = %s, want none", diags)
			}
			assigns, diags := varsfile.DecodeAssignments(parseBody(t, "vars.hcl", tt.varsSrc))
			if diags.HasErrors() {
				t.Fatalf("DecodeAssignments() diags = %s, want none", diags)
			}

			_, diags = varsfile.Resolve(decls, assigns, nil)
			if !diags.HasErrors() {
				t.Fatal("Resolve() diags = none, want error")
			}
			if !strings.Contains(diags.Error(), tt.wantIn) {
				t.Errorf("Resolve() diags = %q, want mention of %q", diags.Error(), tt.wantIn)
			}

			found := false
			for _, d := range diags {
				if d.Subject != nil && d.Subject.Filename == tt.wantFile {
					found = true
				}
			}
			if !found {
				t.Errorf("Resolve() diags anchor = none in %s, want at least one (diags: %s)", tt.wantFile, diags)
			}
		})
	}
}

func TestResolveDefaultUsesCallerContext(t *testing.T) {
	decls, _ := decodeDecls(t, `
variable "home" {
  type    = string
  default = env("VARSFILE_TEST_HOME")
}
`)

	ctx := &hcl.EvalContext{Functions: map[string]function.Function{
		"env": funcs.Env(func(string) string { return "/stub-home" }),
	}}

	result, diags := varsfile.Resolve(decls, nil, ctx)
	if diags.HasErrors() {
		t.Fatalf("Resolve() diags = %s, want none", diags)
	}
	if got := result.Values.GetAttr("home"); got != cty.StringVal("/stub-home") {
		t.Errorf(`var.home = %#v, want "/stub-home" (default must evaluate against caller ctx)`, got)
	}
}

func TestValidationConditionUsesCallerFunctions(t *testing.T) {
	decls, _ := decodeDecls(t, `
variable "name" {
  type = string
  validation {
    condition     = snakeCase(var.name) == var.name
    error_message = "name must be snake_case"
  }
}
`)

	ctx := &hcl.EvalContext{Functions: map[string]function.Function{
		"snakeCase": funcs.SnakeCaseFunc,
	}}

	assigns, _ := varsfile.DecodeAssignments(parseBody(t, "vars.hcl", `name = "AlreadyPascal"`))
	_, diags := varsfile.Resolve(decls, assigns, ctx)
	if !diags.HasErrors() {
		t.Fatal("Resolve() diags = none, want validation failure via caller function")
	}
	if !strings.Contains(diags.Error(), "name must be snake_case") {
		t.Errorf("Resolve() diags = %q, want error_message surfaced", diags.Error())
	}
}
