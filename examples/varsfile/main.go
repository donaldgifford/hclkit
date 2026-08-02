// The varsfile example mirrors forge's consumer shape: variable
// declarations with Terraform-style validation blocks in the main
// config, literal assignments in a separate vars file, and the std
// function bundle available to expressions. WithVarsFile makes the
// Loader strip the variable blocks, resolve them, and bind the
// results as var.<name> before decoding; LoadVarsFile exposes the
// declarations standalone for flows like interactive prompting.
package main

import (
	"fmt"
	"os"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
	"github.com/donaldgifford/hclkit/pkg/hclkit/funcs"
)

type config struct {
	ServiceName string `hcl:"service_name"`
	Environment string `hcl:"environment"`
	Replicas    int    `hcl:"replicas"`
}

func main() {
	loader := hclkit.New(
		hclkit.WithFunctions(funcs.Std()),
		hclkit.WithVarsFile("prod.vars.hcl"),
	)

	var cfg config
	diags := loader.LoadFile("config.hcl", &cfg)
	if diags.HasErrors() {
		_, _ = diags.WriteTo(os.Stderr)
		os.Exit(1)
	}

	fmt.Printf("service=%s environment=%s replicas=%d\n",
		cfg.ServiceName, cfg.Environment, cfg.Replicas)

	// The standalone resolve path: forge's prompting flow reads
	// Declared to know what to ask for before a full Load.
	result, varsDiags := loader.LoadVarsFile("config.hcl", "prod.vars.hcl")
	if varsDiags.HasErrors() {
		_, _ = varsDiags.WriteTo(os.Stderr)
		os.Exit(1)
	}
	for name := range result.Declared {
		decl := result.Declared[name]
		fmt.Printf("declared %s (%s): %s\n",
			decl.Name, decl.Type.FriendlyName(), decl.Description)
	}
}
