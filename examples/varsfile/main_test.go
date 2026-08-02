//go:build integration

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/zclconf/go-cty/cty"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
	"github.com/donaldgifford/hclkit/pkg/hclkit/funcs"
)

func newLoader(varsPath string) *hclkit.Loader {
	return hclkit.New(
		hclkit.WithFunctions(funcs.Std()),
		hclkit.WithVarsFile(varsPath),
	)
}

// TestVarsFileEndToEnd exercises the full forge consumer flow against
// the example's real config and vars files.
func TestVarsFileEndToEnd(t *testing.T) {
	var cfg config
	diags := newLoader("prod.vars.hcl").LoadFile("config.hcl", &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadFile(config.hcl) diagnostics: %s", diags.Error())
	}

	if cfg.ServiceName != "demo_service" {
		t.Errorf("ServiceName = %q, want %q (snakeCase applied)", cfg.ServiceName, "demo_service")
	}
	if cfg.Environment != "prod" {
		t.Errorf("Environment = %q, want %q", cfg.Environment, "prod")
	}
	if cfg.Replicas != 3 {
		t.Errorf("Replicas = %d, want 3 (vars file overrides default)", cfg.Replicas)
	}
}

// TestVarsFileValidationFailure pins the validation-block behavior: an
// assignment outside the allowed set surfaces the declaration's
// error_message as a diagnostic.
func TestVarsFileValidationFailure(t *testing.T) {
	varsPath := t.TempDir() + "/bad.vars.hcl"
	writeFile(t, varsPath, "environment = \"qa\"\n")

	var cfg config
	diags := newLoader(varsPath).LoadFile("config.hcl", &cfg)
	if !diags.HasErrors() {
		t.Fatal("LoadFile() diagnostics = none, want validation failure")
	}
	if !strings.Contains(diags.Error(), "environment must be dev or prod") {
		t.Errorf("diagnostics = %q, want the validation error_message", diags.Error())
	}
}

// TestVarsFileDeclaredFlow exercises the standalone LoadVarsFile path
// forge's interactive prompting depends on.
func TestVarsFileDeclaredFlow(t *testing.T) {
	result, diags := newLoader("prod.vars.hcl").LoadVarsFile("config.hcl", "prod.vars.hcl")
	if diags.HasErrors() {
		t.Fatalf("LoadVarsFile() diagnostics: %s", diags.Error())
	}

	if len(result.Declared) != 2 {
		t.Fatalf("Declared len = %d, want 2", len(result.Declared))
	}
	env := result.Declared["environment"]
	if env.Type != cty.String || env.Description != "Deployment environment." {
		t.Errorf("Declared[environment] = %+v, want string type with description", env)
	}
	if len(env.Validations) != 1 {
		t.Errorf("Declared[environment] validations = %d, want 1", len(env.Validations))
	}
	if got := result.Values.GetAttr("replicas"); !got.RawEquals(cty.NumberIntVal(3)) {
		t.Errorf("Values.replicas = %#v, want 3", got)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
