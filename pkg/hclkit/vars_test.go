package hclkit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zclconf/go-cty/cty"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
)

type varsAppConfig struct {
	Name string `hcl:"name"`
	Port int    `hcl:"port"`
}

const varsMainSrc = `
variable "app_name" {
  type    = string
  default = "fallback"
}

variable "port" {
  type = number
}

name = var.app_name
port = var.port
`

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	return path
}

func TestLoadFileWithVarsFile(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTempFile(t, dir, "main.hcl", varsMainSrc)
	varsPath := writeTempFile(t, dir, "app.vars.hcl", "app_name = \"demo\"\nport = 8080\n")

	var cfg varsAppConfig
	diags := hclkit.New(hclkit.WithVarsFile(varsPath)).LoadFile(configPath, &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadFile() diags = %s, want none", diags)
	}
	if cfg.Name != "demo" || cfg.Port != 8080 {
		t.Errorf("cfg = %+v, want {Name:demo Port:8080}", cfg)
	}
}

func TestLoadFileWithVarsFileDefault(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTempFile(t, dir, "main.hcl", varsMainSrc)
	varsPath := writeTempFile(t, dir, "app.vars.hcl", "port = 1\n")

	var cfg varsAppConfig
	diags := hclkit.New(hclkit.WithVarsFile(varsPath)).LoadFile(configPath, &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadFile() diags = %s, want none", diags)
	}
	if cfg.Name != "fallback" {
		t.Errorf("cfg.Name = %q, want default %q", cfg.Name, "fallback")
	}
}

func TestWithVarsFileLaterFileWins(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTempFile(t, dir, "main.hcl", varsMainSrc)
	base := writeTempFile(t, dir, "base.vars.hcl", "app_name = \"base\"\nport = 1\n")
	override := writeTempFile(t, dir, "override.vars.hcl", "app_name = \"override\"\n")

	var cfg varsAppConfig
	diags := hclkit.New(
		hclkit.WithVarsFile(base),
		hclkit.WithVarsFile(override),
	).LoadFile(configPath, &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadFile() diags = %s, want none", diags)
	}
	if cfg.Name != "override" || cfg.Port != 1 {
		t.Errorf("cfg = %+v, want {Name:override Port:1} (later vars file wins per name)", cfg)
	}
}

func TestVarsFileBindingShadowsWithVariables(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTempFile(t, dir, "main.hcl", varsMainSrc)
	varsPath := writeTempFile(t, dir, "app.vars.hcl", "app_name = \"from-file\"\nport = 1\n")

	var cfg varsAppConfig
	diags := hclkit.New(
		hclkit.WithVariables(map[string]cty.Value{
			"var": cty.ObjectVal(map[string]cty.Value{"app_name": cty.StringVal("from-option")}),
		}),
		hclkit.WithVarsFile(varsPath),
	).LoadFile(configPath, &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadFile() diags = %s, want none", diags)
	}
	if cfg.Name != "from-file" {
		t.Errorf("cfg.Name = %q, want %q (vars-file binding wins over WithVariables)", cfg.Name, "from-file")
	}
}

func TestLoadDirWithVarsFileCollectsDeclarationsAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	// The declaration lives in a LATER file than the usage — the
	// loader must collect declarations across the directory before
	// decoding any file. Override mode decodes each stripped body
	// separately, so the decls-only file needs optional fields.
	writeTempFile(t, dir, "01-app.hcl", "name = var.app_name\n")
	writeTempFile(t, dir, "02-decls.hcl", `
variable "app_name" {
  type = string
}
`)
	varsPath := writeTempFile(t, t.TempDir(), "app.vars.hcl", "app_name = \"dir-scoped\"\n")

	var cfg struct {
		Name string `hcl:"name,optional"`
	}
	diags := hclkit.New(hclkit.WithVarsFile(varsPath)).LoadDir(dir, &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadDir() diags = %s, want none", diags)
	}
	if cfg.Name != "dir-scoped" {
		t.Errorf("cfg.Name = %q, want %q", cfg.Name, "dir-scoped")
	}
}

func TestLoadDirDuplicateDeclarationAcrossFiles(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "01-a.hcl", `
variable "app_name" {
  type = string
}
`)
	writeTempFile(t, dir, "02-b.hcl", `
variable "app_name" {
  type = string
}
`)
	varsPath := writeTempFile(t, t.TempDir(), "app.vars.hcl", "app_name = \"x\"\n")

	var cfg struct{}
	diags := hclkit.New(hclkit.WithVarsFile(varsPath)).LoadDir(dir, &cfg)
	if !diags.HasErrors() {
		t.Fatal("LoadDir() diags = none, want duplicate-declaration error")
	}
	if !strings.Contains(diags.Error(), "Duplicate variable declaration") {
		t.Errorf("LoadDir() diags = %q, want duplicate-declaration mention", diags.Error())
	}
}

func TestLoadVarsFileStandalone(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTempFile(t, dir, "main.hcl", varsMainSrc)
	varsPath := writeTempFile(t, dir, "app.vars.hcl", "port = 9000\n")

	result, diags := hclkit.New().LoadVarsFile(configPath, varsPath)
	if diags.HasErrors() {
		t.Fatalf("LoadVarsFile() diags = %s, want none", diags)
	}
	if got := result.Values.GetAttr("app_name"); got != cty.StringVal("fallback") {
		t.Errorf(`Values.app_name = %#v, want default "fallback"`, got)
	}
	if len(result.Declared) != 2 {
		t.Errorf("Declared len = %d, want 2", len(result.Declared))
	}
	if result.Declared["port"].Type != cty.Number {
		t.Errorf("Declared[port].Type = %#v, want cty.Number", result.Declared["port"].Type)
	}
}

func TestLoadVarsFileRendersVarsFileSnippet(t *testing.T) {
	dir := t.TempDir()
	configPath := writeTempFile(t, dir, "main.hcl", `
variable "port" {
  type = number
}
`)
	varsPath := writeTempFile(t, dir, "app.vars.hcl", "port = \"not-a-number\"\n")

	_, diags := hclkit.New().LoadVarsFile(configPath, varsPath)
	if !diags.HasErrors() {
		t.Fatal("LoadVarsFile() diags = none, want conversion error")
	}

	var rendered strings.Builder
	if _, err := diags.WriteTo(&rendered); err != nil {
		t.Fatalf("WriteTo() error = %v, want nil", err)
	}
	// The vars file was parsed by the same parser instance, so the
	// rendered diagnostic must include a source snippet from it.
	if !strings.Contains(rendered.String(), "not-a-number") {
		t.Errorf("rendered diagnostics = %q, want vars-file source snippet", rendered.String())
	}
}
