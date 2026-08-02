package ctytypes

import (
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// These structs mirror spt's internal/config shape (nested blocks,
// labeled blocks, everything optional) with the duration-carrying
// string fields swapped for hcl.Expression — the IMPL-0001 gohcl
// compat spike. spt today decodes durations as plain strings and
// parses them post-decode via Parsed*() methods that report field
// paths, not source ranges.
type sptSpikeConfig struct {
	API     *sptSpikeAPI    `hcl:"api,block"`
	Watches []sptSpikeWatch `hcl:"watch,block"`
}

type sptSpikeAPI struct {
	Addr         string         `hcl:"addr,optional"`
	ReadTimeout  hcl.Expression `hcl:"read_timeout,optional"`
	WriteTimeout hcl.Expression `hcl:"write_timeout,optional"`
}

type sptSpikeWatch struct {
	Name    string         `hcl:"name,label"`
	Query   string         `hcl:"query,optional"`
	Cadence hcl.Expression `hcl:"cadence,optional"`
}

// TestSptShapeSpike is the recorded outcome of the IMPL-0001 spike:
// hcl.Expression fields survive gohcl struct-tag decode through spt's
// nested/labeled block shapes with source ranges intact, and an
// absent optional attribute gets a static null expression (anchored
// at the body's MissingItemRange, not nil) — which the ctytypes
// helpers decode as a zero value, matching spt's
// parseOptionalDuration("") fallback semantics.
func TestSptShapeSpike(t *testing.T) {
	src := `
api {
  addr         = ":8080"
  read_timeout = "15s"
}

watch "underpriced" {
  query   = "gpu"
  cadence = "10m"
}

watch "broken" {
  cadence = "not-a-duration"
}
`
	file, parseDiags := hclsyntax.ParseConfig([]byte(src), "spt.hcl", hcl.InitialPos)
	if parseDiags.HasErrors() {
		t.Fatalf("parse: %s", parseDiags)
	}

	var cfg sptSpikeConfig
	if diags := gohcl.DecodeBody(file.Body, nil, &cfg); diags.HasErrors() {
		t.Fatalf("DecodeBody: %s", diags)
	}

	readTimeout, diags := DecodeDuration(cfg.API.ReadTimeout, nil)
	if diags.HasErrors() {
		t.Fatalf("DecodeDuration(read_timeout) diags = %s, want none", diags)
	}
	if readTimeout != 15*time.Second {
		t.Errorf("read_timeout = %v, want 15s", readTimeout)
	}

	// Absent optional attribute: gohcl assigns a static null
	// expression and the helper returns zero with no diagnostics —
	// spt's empty-string fallback shape.
	writeTimeout, diags := DecodeDuration(cfg.API.WriteTimeout, nil)
	if diags.HasErrors() {
		t.Fatalf("DecodeDuration(absent write_timeout) diags = %s, want none", diags)
	}
	if writeTimeout != 0 {
		t.Errorf("write_timeout = %v, want zero for absent attribute", writeTimeout)
	}

	if len(cfg.Watches) != 2 {
		t.Fatalf("watches = %d, want 2", len(cfg.Watches))
	}
	cadence, diags := DecodeDuration(cfg.Watches[0].Cadence, nil)
	if diags.HasErrors() || cadence != 10*time.Minute {
		t.Errorf("watch[0] cadence = %v (diags %s), want 10m", cadence, diags)
	}

	// The failure spt cannot produce today: a real source position
	// instead of a hand-built field path.
	_, diags = DecodeDuration(cfg.Watches[1].Cadence, nil)
	if !diags.HasErrors() {
		t.Fatal("DecodeDuration(bad cadence) diags = none, want parse error")
	}
	subj := diags[0].Subject
	if subj == nil || subj.Filename != "spt.hcl" || subj.Start.Line != 13 {
		t.Errorf("Subject = %v, want spt.hcl line 13 (range preserved through labeled nested block)", subj)
	}
	if !strings.Contains(diags.Error(), `Cannot parse "not-a-duration"`) {
		t.Errorf("diags = %q, want duration parse detail", diags.Error())
	}
}

func TestDecodeDurationNilExpression(t *testing.T) {
	d, diags := DecodeDuration(nil, nil)
	if diags.HasErrors() || d != 0 {
		t.Errorf("DecodeDuration(nil) = (%v, %s), want (0, no diags)", d, diags)
	}
}

func TestEnumDecodeExprNilExpression(t *testing.T) {
	e := Enum("mode", []string{"fast"})
	s, diags := e.DecodeExpr(nil, nil)
	if diags.HasErrors() || s != "" {
		t.Errorf(`DecodeExpr(nil) = (%q, %s), want ("", no diags)`, s, diags)
	}
}
