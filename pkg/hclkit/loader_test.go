package hclkit_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"

	"github.com/donaldgifford/hclkit/internal/testutil"
	"github.com/donaldgifford/hclkit/pkg/hclkit"
)

type appConfig struct {
	Name     string   `hcl:"name,optional"`
	Replicas int      `hcl:"replicas,optional"`
	Tags     []string `hcl:"tags,optional"`
}

type ruleConfig struct {
	Rules []rule `hcl:"rule,block"`
}

type rule struct {
	Name   string `hcl:"name,label"`
	Action string `hcl:"action"`
}

var upperFunc = function.New(&function.Spec{
	Params: []function.Parameter{{Name: "s", Type: cty.String}},
	Type:   function.StaticReturnType(cty.String),
	Impl: func(args []cty.Value, _ cty.Type) (cty.Value, error) {
		return cty.StringVal(strings.ToUpper(args[0].AsString())), nil
	},
})

func TestLoadBytes(t *testing.T) {
	src := "name = \"demo\"\nreplicas = 3\n"

	var cfg appConfig
	diags := hclkit.New().LoadBytes("app.hcl", []byte(src), &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadBytes() diagnostics: %s", diags.Error())
	}
	if cfg.Name != "demo" || cfg.Replicas != 3 {
		t.Errorf("LoadBytes() = %+v, want {Name:demo Replicas:3}", cfg)
	}
}

func TestLoadBytesJSONDispatch(t *testing.T) {
	src := `{"name": "demo", "replicas": 2}`

	var cfg appConfig
	diags := hclkit.New().LoadBytes("app.json", []byte(src), &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadBytes(json) diagnostics: %s", diags.Error())
	}
	if cfg.Name != "demo" || cfg.Replicas != 2 {
		t.Errorf("LoadBytes(json) = %+v, want {Name:demo Replicas:2}", cfg)
	}
}

func TestLoadBytesSyntaxError(t *testing.T) {
	var cfg appConfig
	diags := hclkit.New().LoadBytes("bad.hcl", []byte("name = \n"), &cfg)
	if !diags.HasErrors() {
		t.Fatal("LoadBytes(invalid) HasErrors() = false, want true")
	}
	if !strings.Contains(diags.Error(), "bad.hcl") {
		t.Errorf("LoadBytes(invalid) Error() = %q, want filename in message", diags.Error())
	}
}

func TestZeroValueLoaderUsable(t *testing.T) {
	var l hclkit.Loader

	var cfg appConfig
	if diags := l.LoadBytes("app.hcl", []byte("name = \"zero\"\n"), &cfg); diags.HasErrors() {
		t.Fatalf("zero Loader LoadBytes() diagnostics: %s", diags.Error())
	}
	if cfg.Name != "zero" {
		t.Errorf("zero Loader decoded Name = %q, want %q", cfg.Name, "zero")
	}
}

func TestLoadFile(t *testing.T) {
	var cfg appConfig
	diags := hclkit.New().LoadFile(filepath.Join("testdata", "valid.hcl"), &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadFile(valid.hcl) diagnostics: %s", diags.Error())
	}

	want := appConfig{Name: "demo", Replicas: 3, Tags: []string{"a", "b"}}
	if cfg.Name != want.Name || cfg.Replicas != want.Replicas || len(cfg.Tags) != 2 {
		t.Errorf("LoadFile(valid.hcl) = %+v, want %+v", cfg, want)
	}
}

func TestLoadFileJSON(t *testing.T) {
	var cfg appConfig
	diags := hclkit.New().LoadFile(filepath.Join("testdata", "valid.json"), &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadFile(valid.json) diagnostics: %s", diags.Error())
	}
	if cfg.Name != "demo" || cfg.Replicas != 3 {
		t.Errorf("LoadFile(valid.json) = %+v, want {Name:demo Replicas:3}", cfg)
	}
}

func TestLoadFileMissing(t *testing.T) {
	var cfg appConfig
	diags := hclkit.New().LoadFile(filepath.Join(t.TempDir(), "absent.hcl"), &cfg)
	if !diags.HasErrors() {
		t.Error("LoadFile(absent) HasErrors() = false, want true")
	}
}

func TestLoadInvalidTargets(t *testing.T) {
	notAPointer := appConfig{}
	number := 7

	tests := []struct {
		name   string
		target any
	}{
		{name: "nil interface", target: nil},
		{name: "non-pointer struct", target: notAPointer},
		{name: "typed nil pointer", target: (*appConfig)(nil)},
		{name: "pointer to non-struct", target: &number},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := hclkit.New().LoadBytes("app.hcl", []byte("name = \"x\"\n"), tt.target)
			if !diags.HasErrors() {
				t.Errorf("LoadBytes(target %T) HasErrors() = false, want true", tt.target)
			}
			if !strings.Contains(diags.Error(), "Invalid decode target") {
				t.Errorf("LoadBytes(target %T) Error() = %q, want invalid-target diagnostic",
					tt.target, diags.Error())
			}
		})
	}
}

func TestWithVariablesAndFunctions(t *testing.T) {
	src := "name = upper(env_name)\n"

	var cfg appConfig
	diags := hclkit.New(
		hclkit.WithFunctions(map[string]function.Function{"upper": upperFunc}),
		hclkit.WithVariables(map[string]cty.Value{"env_name": cty.StringVal("prod")}),
	).LoadBytes("app.hcl", []byte(src), &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadBytes() diagnostics: %s", diags.Error())
	}
	if cfg.Name != "PROD" {
		t.Errorf("decoded Name = %q, want %q", cfg.Name, "PROD")
	}
}

func TestWithEvalContextLayering(t *testing.T) {
	callerCtx := &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"region":   cty.StringVal("us-east-1"),
			"greeting": cty.StringVal("from-ctx"),
		},
		Functions: map[string]function.Function{"upper": upperFunc},
	}

	src := "name = upper(region)\ntags = [greeting]\n"

	var cfg appConfig
	diags := hclkit.New(
		hclkit.WithEvalContext(callerCtx),
		// Collides with the caller's greeting: the option must win.
		hclkit.WithVariables(map[string]cty.Value{"greeting": cty.StringVal("from-option")}),
	).LoadBytes("app.hcl", []byte(src), &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadBytes() diagnostics: %s", diags.Error())
	}

	if cfg.Name != "US-EAST-1" {
		t.Errorf("caller ctx function/variable lookup: Name = %q, want %q", cfg.Name, "US-EAST-1")
	}
	if len(cfg.Tags) != 1 || cfg.Tags[0] != "from-option" {
		t.Errorf("collision resolution: Tags = %v, want [from-option]", cfg.Tags)
	}

	// The caller's context must not have been mutated by the load.
	if got := callerCtx.Variables["greeting"]; got != cty.StringVal("from-ctx") {
		t.Errorf("caller ctx mutated: greeting = %#v, want from-ctx", got)
	}
	if len(callerCtx.Variables) != 2 || len(callerCtx.Functions) != 1 {
		t.Errorf("caller ctx maps changed size: vars=%d funcs=%d, want 2 and 1",
			len(callerCtx.Variables), len(callerCtx.Functions))
	}
}

func TestLoadDirOverride(t *testing.T) {
	var cfg appConfig
	diags := hclkit.New().LoadDir(filepath.Join("testdata", "multi"), &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadDir(multi) diagnostics: %s", diags.Error())
	}

	// 01-base.hcl sets all three fields; 02-override.hcl sets only
	// replicas. Later file wins per field; untouched fields survive.
	if cfg.Name != "base" {
		t.Errorf("Name = %q, want %q (from 01-base.hcl)", cfg.Name, "base")
	}
	if cfg.Replicas != 5 {
		t.Errorf("Replicas = %d, want 5 (overridden by 02-override.hcl)", cfg.Replicas)
	}
	if len(cfg.Tags) != 2 {
		t.Errorf("Tags = %v, want the 2 entries from 01-base.hcl", cfg.Tags)
	}
}

func TestLoadDirAppend(t *testing.T) {
	var cfg ruleConfig
	diags := hclkit.New(hclkit.WithMergeMode(hclkit.MergeAppend)).
		LoadDir(filepath.Join("testdata", "append"), &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadDir(append) diagnostics: %s", diags.Error())
	}

	if len(cfg.Rules) != 2 {
		t.Fatalf("Rules len = %d, want 2 (blocks appended across files)", len(cfg.Rules))
	}
	if cfg.Rules[0].Name != "first" || cfg.Rules[1].Name != "second" {
		t.Errorf("Rules = %+v, want [first second] in lexical file order", cfg.Rules)
	}
}

func TestLoadDirSkipsNonHCL(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a.hcl"), "name = \"a\"\n")
	writeFile(t, filepath.Join(dir, "notes.txt"), "not hcl")
	if err := os.Mkdir(filepath.Join(dir, "sub.hcl"), 0o750); err != nil {
		t.Fatal(err)
	}

	var cfg appConfig
	diags := hclkit.New().LoadDir(dir, &cfg)
	if diags.HasErrors() {
		t.Fatalf("LoadDir() diagnostics: %s", diags.Error())
	}
	if cfg.Name != "a" {
		t.Errorf("Name = %q, want %q", cfg.Name, "a")
	}
}

func TestLoadDirEmpty(t *testing.T) {
	var cfg appConfig
	diags := hclkit.New().LoadDir(t.TempDir(), &cfg)
	if !diags.HasErrors() {
		t.Error("LoadDir(empty) HasErrors() = false, want true")
	}
}

func TestLoadDirMissing(t *testing.T) {
	var cfg appConfig
	diags := hclkit.New().LoadDir(filepath.Join(t.TempDir(), "absent"), &cfg)
	if !diags.HasErrors() {
		t.Error("LoadDir(absent) HasErrors() = false, want true")
	}
}

func TestLoadDirAggregatesParseErrors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "one.hcl"), "name = \n")
	writeFile(t, filepath.Join(dir, "two.hcl"), "replicas = \n")

	var cfg appConfig
	diags := hclkit.New().LoadDir(dir, &cfg)
	if !diags.HasErrors() {
		t.Fatal("LoadDir(two invalid files) HasErrors() = false, want true")
	}

	var buf bytes.Buffer
	if _, err := diags.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo() error: %v", err)
	}
	out := buf.String()
	for _, name := range []string{"one.hcl", "two.hcl"} {
		if !strings.Contains(out, name) {
			t.Errorf("rendered diagnostics missing %s:\n%s", name, out)
		}
	}
}

func TestWithDiagnosticWriterTee(t *testing.T) {
	var tee bytes.Buffer

	var cfg appConfig
	diags := hclkit.New(hclkit.WithDiagnosticWriter(&tee)).
		LoadBytes("bad.hcl", []byte("name = \n"), &cfg)
	if !diags.HasErrors() {
		t.Fatal("LoadBytes(invalid) HasErrors() = false, want true")
	}

	var direct bytes.Buffer
	if _, err := diags.WriteTo(&direct); err != nil {
		t.Fatalf("WriteTo() error: %v", err)
	}
	if tee.String() != direct.String() {
		t.Errorf("tee output differs from WriteTo output:\ntee:\n%s\ndirect:\n%s",
			tee.String(), direct.String())
	}
}

func TestDiagnosticRendererGolden(t *testing.T) {
	var cfg appConfig
	diags := hclkit.New().LoadFile(filepath.Join("testdata", "invalid.hcl"), &cfg)
	if !diags.HasErrors() {
		t.Fatal("LoadFile(invalid.hcl) HasErrors() = false, want true")
	}

	var buf bytes.Buffer
	if _, err := diags.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo() error: %v", err)
	}
	testutil.Golden(t, "diag_invalid", buf.Bytes())
}

func TestMergeModeString(t *testing.T) {
	tests := []struct {
		mode hclkit.MergeMode
		want string
	}{
		{mode: hclkit.MergeOverride, want: "override"},
		{mode: hclkit.MergeAppend, want: "append"},
		{mode: hclkit.MergeMode(9), want: "MergeMode(9)"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mode.String(); got != tt.want {
				t.Errorf("MergeMode(%d).String() = %q, want %q", int(tt.mode), got, tt.want)
			}
		})
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
