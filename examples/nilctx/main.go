// The nilctx example mirrors the simplest surveyed consumer shape
// (claudelint, mcp-go-gen): a plain
// struct decode with no EvalContext. The zero-configuration Loader is
// byte-identical to gohcl.DecodeBody(body, nil, &cfg) — that identity
// is the Phase 1 adopter bar.
package main

import (
	"fmt"
	"os"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
)

type config struct {
	Project  string `hcl:"project"`
	Severity string `hcl:"severity,optional"`
	Rules    []rule `hcl:"rule,block"`
}

type rule struct {
	ID      string `hcl:"id,label"`
	Enabled bool   `hcl:"enabled,optional"`
}

func main() {
	var cfg config
	diags := hclkit.New().LoadFile("config.hcl", &cfg)
	if diags.HasErrors() {
		_, _ = diags.WriteTo(os.Stderr)
		os.Exit(1)
	}

	fmt.Printf("project=%s severity=%s rules=%d\n",
		cfg.Project, cfg.Severity, len(cfg.Rules))
}
