//go:build integration

package main

import (
	"testing"

	"github.com/zclconf/go-cty/cty/function"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
	"github.com/donaldgifford/hclkit/pkg/hclkit/funcs"
)

func loadConfig(t *testing.T) (config, hclkit.Diagnostics) {
	t.Helper()
	loader := hclkit.New(hclkit.WithFunctions(map[string]function.Function{
		"env": funcs.Env(nil),
	}))
	var cfg config
	return cfg, loader.LoadFile("config.hcl", &cfg)
}

// TestEnvFuncEndToEnd exercises the spt consumer shape against the
// example's real config file with the environment variable set.
func TestEnvFuncEndToEnd(t *testing.T) {
	t.Setenv("ENVFUNC_WORKSPACE", "staging")

	cfg, diags := loadConfig(t)
	if diags.HasErrors() {
		t.Fatalf("LoadFile(config.hcl) diagnostics: %s", diags.Error())
	}
	if cfg.ListenAddr != "127.0.0.1:8080" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, "127.0.0.1:8080")
	}
	if cfg.Workspace != "staging" {
		t.Errorf("Workspace = %q, want %q", cfg.Workspace, "staging")
	}
}

// TestEnvFuncUnsetVariable pins the Unix env semantics: an unset
// variable resolves to "", not a diagnostic.
func TestEnvFuncUnsetVariable(t *testing.T) {
	t.Setenv("ENVFUNC_WORKSPACE", "")

	cfg, diags := loadConfig(t)
	if diags.HasErrors() {
		t.Fatalf("LoadFile(config.hcl) diagnostics: %s", diags.Error())
	}
	if cfg.Workspace != "" {
		t.Errorf("Workspace = %q, want empty string for unset variable", cfg.Workspace)
	}
}
