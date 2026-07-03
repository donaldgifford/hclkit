package format_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/hclkit/internal/format"
)

const unformatted = "name=\"demo\"\nreplicas   = 3\n"

// canonical is what hclwrite.Format produces for unformatted.
const canonical = "name     = \"demo\"\nreplicas = 3\n"

func TestFilesRewrites(t *testing.T) {
	path := writeTemp(t, "a.hcl", unformatted)

	changed, diags := format.Files([]string{path}, false)
	if diags.HasErrors() {
		t.Fatalf("Files() diagnostics: %s", diags.Error())
	}
	if len(changed) != 1 || changed[0] != path {
		t.Errorf("Files() changed = %v, want [%s]", changed, path)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != canonical {
		t.Errorf("Files() rewrote to %q, want %q", got, canonical)
	}
}

func TestFilesCheckDoesNotRewrite(t *testing.T) {
	path := writeTemp(t, "a.hcl", unformatted)

	changed, diags := format.Files([]string{path}, true)
	if diags.HasErrors() {
		t.Fatalf("Files(check) diagnostics: %s", diags.Error())
	}
	if len(changed) != 1 {
		t.Fatalf("Files(check) changed = %v, want 1 entry", changed)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != unformatted {
		t.Errorf("Files(check) modified the file to %q, want untouched", got)
	}
}

func TestFilesAlreadyCanonical(t *testing.T) {
	path := writeTemp(t, "a.hcl", canonical)

	changed, diags := format.Files([]string{path}, false)
	if diags.HasErrors() {
		t.Fatalf("Files() diagnostics: %s", diags.Error())
	}
	if len(changed) != 0 {
		t.Errorf("Files() changed = %v, want none for canonical input", changed)
	}
}

func TestFilesInvalidSourceSkipped(t *testing.T) {
	const broken = "name = \n"
	path := writeTemp(t, "broken.hcl", broken)

	changed, diags := format.Files([]string{path}, false)
	if !diags.HasErrors() {
		t.Fatal("Files(broken) HasErrors() = false, want true")
	}
	if len(changed) != 0 {
		t.Errorf("Files(broken) changed = %v, want none", changed)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != broken {
		t.Errorf("Files(broken) modified an unparseable file to %q", got)
	}
}

func TestFilesMissing(t *testing.T) {
	_, diags := format.Files([]string{filepath.Join(t.TempDir(), "absent.hcl")}, false)
	if !diags.HasErrors() {
		t.Error("Files(absent) HasErrors() = false, want true")
	}
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
