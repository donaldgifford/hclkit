// The envfunc example mirrors spt's consumer shape: a Loader with an
// env() function registered so config files can pull values from the
// process environment. funcs.Env(nil) uses os.Getenv with Unix
// semantics — an unset variable evaluates to "" rather than erroring.
package main

import (
	"fmt"
	"os"

	"github.com/zclconf/go-cty/cty/function"

	"github.com/donaldgifford/hclkit/pkg/hclkit"
	"github.com/donaldgifford/hclkit/pkg/hclkit/funcs"
)

type config struct {
	ListenAddr string `hcl:"listen_addr"`
	Workspace  string `hcl:"workspace"`
}

func main() {
	loader := hclkit.New(hclkit.WithFunctions(map[string]function.Function{
		"env": funcs.Env(nil),
	}))

	var cfg config
	diags := loader.LoadFile("config.hcl", &cfg)
	if diags.HasErrors() {
		_, _ = diags.WriteTo(os.Stderr)
		os.Exit(1)
	}

	fmt.Printf("listen_addr=%s workspace=%s\n", cfg.ListenAddr, cfg.Workspace)
}
