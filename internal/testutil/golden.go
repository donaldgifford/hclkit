// Package testutil provides internal-only test helpers: golden-file
// comparison with a -update regeneration flag, and fixture loading.
//
// Import it from _test.go files only — importing it from production
// code would register the -update flag on flag.CommandLine in shipped
// binaries.
package testutil

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

// Golden compares got against testdata/<name>.golden relative to the
// calling test's package directory. With -update it rewrites the
// golden file instead:
//
//	go test ./... -update
func Golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("creating golden dir: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("writing golden file: %v", err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden file %s (run 'go test -update' to create it): %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("Golden(%s) mismatch:\n--- want\n%s\n--- got\n%s", name, want, got)
	}
}
