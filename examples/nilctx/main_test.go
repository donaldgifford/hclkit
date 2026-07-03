//go:build integration

package main

import (
	"testing"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
)

// TestNilCtxEndToEnd exercises the full surveyed nil-ctx consumer
// flow against the example's real config file.
func TestNilCtxEndToEnd(t *testing.T) {
	var cfg config
	diags := hclkit.New().LoadFile("config.hcl", &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadFile(config.hcl) diagnostics: %s", diags.Error())
	}

	if cfg.Project != "nilctx-demo" {
		t.Errorf("Project = %q, want %q", cfg.Project, "nilctx-demo")
	}
	if cfg.Severity != "low" {
		t.Errorf("Severity = %q, want %q", cfg.Severity, "low")
	}
	if len(cfg.Rules) != 2 {
		t.Fatalf("Rules len = %d, want 2", len(cfg.Rules))
	}
	if cfg.Rules[0].ID != "no-empty-readme" || !cfg.Rules[0].Enabled {
		t.Errorf("Rules[0] = %+v, want no-empty-readme enabled", cfg.Rules[0])
	}
	if cfg.Rules[1].ID != "license-present" || cfg.Rules[1].Enabled {
		t.Errorf("Rules[1] = %+v, want license-present with Enabled defaulting false", cfg.Rules[1])
	}
}
