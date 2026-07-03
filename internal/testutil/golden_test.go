package testutil

import "testing"

// The sample fixture and its golden twin are checked in with identical
// content, so a plain run exercises the comparison path end-to-end
// (and -update exercises regeneration against the same file).
func TestGoldenMatchesFixture(t *testing.T) {
	got := ReadFixture(t, "sample.txt")
	Golden(t, "sample", got)
}
