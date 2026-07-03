package parser_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/hclkit/internal/parser"
)

func TestParseBytesDispatch(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		src      string
		wantErr  bool
	}{
		{
			name:     "native syntax",
			filename: "config.hcl",
			src:      `name = "demo"`,
		},
		{
			name:     "json syntax",
			filename: "config.json",
			src:      `{"name": "demo"}`,
		},
		{
			name:     "json source under hcl name fails native parse",
			filename: "config.hcl",
			src:      `{"name": "demo"}`,
			wantErr:  true,
		},
		{
			name:     "native syntax error",
			filename: "config.hcl",
			src:      `name = `,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, diags := parser.New().ParseBytes(tt.filename, []byte(tt.src))
			if got := diags.HasErrors(); got != tt.wantErr {
				t.Fatalf("ParseBytes(%s) HasErrors() = %v, want %v (diags: %s)",
					tt.filename, got, tt.wantErr, diags)
			}
			if !tt.wantErr && file == nil {
				t.Errorf("ParseBytes(%s) file = nil, want non-nil", tt.filename)
			}
		})
	}
}

func TestParseFileDispatch(t *testing.T) {
	dir := t.TempDir()
	hclPath := filepath.Join(dir, "a.hcl")
	jsonPath := filepath.Join(dir, "b.json")
	writeFile(t, hclPath, `name = "hcl"`)
	writeFile(t, jsonPath, `{"name": "json"}`)

	p := parser.New()
	for _, path := range []string{hclPath, jsonPath} {
		if _, diags := p.ParseFile(path); diags.HasErrors() {
			t.Errorf("ParseFile(%s) HasErrors() = true, want false (diags: %s)", path, diags)
		}
	}

	if got := len(p.Files()); got != 2 {
		t.Errorf("Files() len = %d, want 2", got)
	}
}

func TestParseFileMissing(t *testing.T) {
	_, diags := parser.New().ParseFile(filepath.Join(t.TempDir(), "absent.hcl"))
	if !diags.HasErrors() {
		t.Error("ParseFile(absent) HasErrors() = false, want true")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}
