package testutil

import (
	"os"
	"path/filepath"
	"testing"
)

// ReadFixture returns the contents of testdata/<name> relative to the
// calling test's package directory.
func ReadFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", name, err)
	}
	return data
}
