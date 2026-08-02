package ctytypes

// Property-based tests via Go-native fuzz targets. `go test` runs the
// seed corpus deterministically in CI; `go test -fuzz=Fuzz<Name>
// ./pkg/hclkit/ctytypes` explores further locally.

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// FuzzValidateDurationRoundTrip checks two properties for any input:
// accepted strings agree with time.ParseDuration and survive a
// String() round trip; rejected strings anchor their diagnostic at
// exactly the caller-supplied range.
func FuzzValidateDurationRoundTrip(f *testing.F) {
	for _, seed := range []string{"30s", "1h30m", "-5m", "0s", "1.5h", "300ms", "", "oops", "10", "999999h"} {
		f.Add(seed)
	}
	subject := hcl.Range{Filename: "prop.hcl", Start: hcl.Pos{Line: 3, Column: 9}}

	f.Fuzz(func(t *testing.T, s string) {
		got, diags := ValidateDuration(s, subject)

		want, err := time.ParseDuration(s)
		if err != nil {
			if !diags.HasErrors() {
				t.Fatalf("ValidateDuration(%q) diags = none, but ParseDuration rejects it", s)
			}
			if subj := diags[0].Subject; subj == nil || *subj != subject {
				t.Errorf("ValidateDuration(%q) Subject = %v, want the supplied range", s, diags[0].Subject)
			}
			return
		}

		if diags.HasErrors() {
			t.Fatalf("ValidateDuration(%q) diags = %s, but ParseDuration accepts it", s, diags)
		}
		if got != want {
			t.Errorf("ValidateDuration(%q) = %v, want %v", s, got, want)
		}
		// Round trip: formatting the parsed value re-parses to itself.
		again, roundDiags := ValidateDuration(got.String(), subject)
		if roundDiags.HasErrors() || again != got {
			t.Errorf("round trip of %q: %v -> %q -> %v", s, got, got.String(), again)
		}
	})
}

// FuzzDecodeDurationThroughHCL checks the cty round trip: any
// time.Duration formatted into real HCL source decodes back to the
// same value, and the expression range reports the real position even
// with leading padding lines.
func FuzzDecodeDurationThroughHCL(f *testing.F) {
	f.Add(int64(30*time.Second), uint8(0))
	f.Add(int64(90*time.Minute), uint8(3))
	f.Add(int64(-5*time.Minute), uint8(1))
	f.Add(int64(0), uint8(7))
	f.Add(int64(time.Nanosecond), uint8(2))

	f.Fuzz(func(t *testing.T, ns int64, padding uint8) {
		d := time.Duration(ns)
		pad := int(padding % 16)
		src := strings.Repeat("\n", pad) + fmt.Sprintf("v = %q\n", d.String())

		file, parseDiags := hclsyntax.ParseConfig([]byte(src), "fuzz.hcl", hcl.InitialPos)
		if parseDiags.HasErrors() {
			t.Fatalf("generated source %q failed to parse: %s", src, parseDiags)
		}
		attrs, attrDiags := file.Body.JustAttributes()
		if attrDiags.HasErrors() {
			t.Fatalf("attributes: %s", attrDiags)
		}
		expr := attrs["v"].Expr

		got, diags := DecodeDuration(expr, nil)
		if diags.HasErrors() {
			t.Fatalf("DecodeDuration(%q) diags = %s, want none", d.String(), diags)
		}
		if got != d {
			t.Errorf("DecodeDuration(%q) = %v, want %v (cty round trip)", d.String(), got, d)
		}
		if line := expr.Range().Start.Line; line != pad+1 {
			t.Errorf("expression range line = %d, want %d", line, pad+1)
		}
	})
}

// FuzzEnumValidateMembership checks the membership invariant against
// a fixed set: Validate accepts s iff s is in Values(), rejections
// anchor at the supplied range, and a case-insensitive near miss
// carries a suggestion.
func FuzzEnumValidateMembership(f *testing.F) {
	for _, seed := range []string{"low", "medium", "high", "Low", "HIGH", "", "urgent", "hi gh"} {
		f.Add(seed)
	}
	e := Enum("severity", []string{"low", "medium", "high"})
	subject := hcl.Range{Filename: "prop.hcl", Start: hcl.Pos{Line: 5, Column: 1}}

	f.Fuzz(func(t *testing.T, s string) {
		diags := e.Validate(s, subject)
		member := slices.Contains(e.Values(), s)

		if member && diags.HasErrors() {
			t.Fatalf("Validate(%q) diags = %s for a member", s, diags)
		}
		if !member {
			if !diags.HasErrors() {
				t.Fatalf("Validate(%q) diags = none for a non-member", s)
			}
			if subj := diags[0].Subject; subj == nil || *subj != subject {
				t.Errorf("Validate(%q) Subject = %v, want the supplied range", s, diags[0].Subject)
			}
			hasFold := slices.ContainsFunc(e.Values(), func(v string) bool { return strings.EqualFold(v, s) })
			if hasFold != strings.Contains(diags[0].Detail, "Did you mean") {
				t.Errorf("Validate(%q) suggestion presence = %v, want %v (detail %q)",
					s, !hasFold, hasFold, diags[0].Detail)
			}
		}
	})
}

// FuzzEnumConstructorInvariants checks Enum over arbitrary value
// lists: Values() is duplicate-free, preserves first-occurrence
// order, and Validate accepts exactly the constructor inputs.
func FuzzEnumConstructorInvariants(f *testing.F) {
	f.Add("a", "b", "a")
	f.Add("", "", "x")
	f.Add("one", "two", "three")

	f.Fuzz(func(t *testing.T, a, b, c string) {
		input := []string{a, b, c}
		e := Enum("fuzzed", input)
		vals := e.Values()

		seen := make(map[string]struct{}, len(vals))
		for _, v := range vals {
			if _, dup := seen[v]; dup {
				t.Fatalf("Values() = %v contains duplicate %q", vals, v)
			}
			seen[v] = struct{}{}
		}

		for _, v := range input {
			if diags := e.Validate(v, hcl.Range{}); diags.HasErrors() {
				t.Errorf("Validate(%q) rejected a constructor input", v)
			}
		}
		if len(vals) > 0 && vals[0] != input[0] {
			t.Errorf("Values()[0] = %q, want first input %q (first-occurrence order)", vals[0], input[0])
		}
	})
}
