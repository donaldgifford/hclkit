package hclkit_test

import (
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
	"github.com/donaldgifford/hclkit/pkg/hclkit/validate"
)

// The validate types satisfy the Loader's Validator contract
// structurally; these checks pin that at compile time.
var (
	_ hclkit.Validator = (*validate.RefValidator)(nil)
	_ hclkit.Validator = (*validate.UniqueValidator)(nil)
)

type doctypeConfig struct {
	Doctypes []struct {
		Name     string   `hcl:"name,label"`
		IDPrefix string   `hcl:"id_prefix"`
		Decides  []string `hcl:"decides,optional"`
	} `hcl:"doctype,block"`
}

func loadDirDiags(t *testing.T, dir string, mode hclkit.MergeMode, v hclkit.Validator) hclkit.Diagnostics {
	t.Helper()
	var cfg doctypeConfig
	return hclkit.New(
		hclkit.WithMergeMode(mode),
		hclkit.WithValidators(v),
	).LoadDir(dir, &cfg)
}

func TestLoadDirRefValidator(t *testing.T) {
	for _, mode := range []hclkit.MergeMode{hclkit.MergeOverride, hclkit.MergeAppend} {
		t.Run(mode.String(), func(t *testing.T) {
			diags := loadDirDiags(t, "testdata/refs", mode, validate.NewRefValidator("decides", "doctype"))
			if !diags.HasErrors() {
				t.Fatal("LoadDir diags = none, want unresolved-reference error")
			}
			if !strings.Contains(diags.Error(), `No doctype block named "missing"`) {
				t.Errorf("diags = %q, want undeclared-doctype detail", diags.Error())
			}
			// Anchored at the bad element in 02-use.hcl — cross-file
			// resolution saw 01-decl.hcl's declarations.
			var rendered strings.Builder
			if _, err := diags.WriteTo(&rendered); err != nil {
				t.Fatalf("WriteTo: %v", err)
			}
			if !strings.Contains(rendered.String(), "02-use.hcl") {
				t.Errorf("rendered = %q, want anchor in 02-use.hcl", rendered.String())
			}
			if strings.Contains(diags.Error(), `"policy"`) {
				t.Errorf("diags = %q, must not flag the declared %q", diags.Error(), "policy")
			}
		})
	}
}

func TestLoadDirUniqueValidator(t *testing.T) {
	diags := loadDirDiags(t, "testdata/unique", hclkit.MergeOverride,
		validate.NewUniqueValidator("doctype", "id_prefix"))
	if !diags.HasErrors() {
		t.Fatal("LoadDir diags = none, want duplicate error")
	}
	if !strings.Contains(diags.Error(), "Duplicate doctype id_prefix") {
		t.Errorf("diags = %q, want duplicate summary", diags.Error())
	}
	if !strings.Contains(diags.Error(), "01-a.hcl") {
		t.Errorf("diags = %q, want first occurrence named in detail", diags.Error())
	}
}

func TestValidatorSeesVarsFileContext(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "main.hcl", `
variable "target" {
  type = string
}

doctype "rfc" {
  id_prefix = "RFC"
  decides   = [var.target]
}
`)
	varsPath := writeTempFile(t, t.TempDir(), "v.hcl", "target = \"rfc\"\n")

	var cfg doctypeConfig
	diags := hclkit.New(
		hclkit.WithVarsFile(varsPath),
		hclkit.WithValidators(validate.NewRefValidator("decides", "doctype")),
	).LoadDir(dir, &cfg)
	if diags.HasErrors() {
		t.Fatalf("diags = %s, want none (validator evaluates var.target via the vars-bound ctx)", diags)
	}
}

// TestValidatorDiagsDoNotBlockDecode pins collect-all: the decode
// still populates the target alongside validator errors.
func TestValidatorDiagsDoNotBlockDecode(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "main.hcl", `
doctype "rfc" {
  id_prefix = "RFC"
  decides   = ["missing"]
}
`)

	var cfg doctypeConfig
	diags := hclkit.New(
		hclkit.WithValidators(validate.NewRefValidator("decides", "doctype")),
	).LoadDir(dir, &cfg)
	if !diags.HasErrors() {
		t.Fatal("diags = none, want validator error")
	}
	if len(cfg.Doctypes) != 1 || cfg.Doctypes[0].Name != "rfc" {
		t.Errorf("cfg = %+v, want decode to have proceeded despite validator diags", cfg)
	}
}

type failValidator struct{}

func (failValidator) Validate([]hcl.Body, *hcl.EvalContext) hcl.Diagnostics {
	return hcl.Diagnostics{{Severity: hcl.DiagError, Summary: "Always fails"}}
}

func TestValidatorsRunInRegistrationOrder(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "main.hcl", "doctype \"rfc\" {\n  id_prefix = \"RFC\"\n}\n")

	var cfg doctypeConfig
	diags := hclkit.New(
		hclkit.WithValidators(failValidator{}, failValidator{}),
	).LoadDir(dir, &cfg)
	if got := len(diags.Diagnostics); got != 2 {
		t.Errorf("diags = %d, want both validators' diagnostics collected", got)
	}
}
