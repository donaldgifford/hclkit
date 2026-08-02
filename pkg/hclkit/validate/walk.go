package validate

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

// walkBodies visits every attribute (attrFn) and block (blockFn) in
// bodies at any depth, in source order, blocks depth-first. Either
// callback may be nil. Non-hclsyntax bodies are skipped — validation
// is native-syntax only.
func walkBodies(bodies []hcl.Body, attrFn func(*hclsyntax.Attribute), blockFn func(*hclsyntax.Block)) {
	for _, body := range bodies {
		if syn, ok := body.(*hclsyntax.Body); ok {
			walkBody(syn, attrFn, blockFn)
		}
	}
}

func walkBody(body *hclsyntax.Body, attrFn func(*hclsyntax.Attribute), blockFn func(*hclsyntax.Block)) {
	if attrFn != nil {
		for _, attr := range body.Attributes {
			attrFn(attr)
		}
	}
	for _, block := range body.Blocks {
		if blockFn != nil {
			blockFn(block)
		}
		walkBody(block.Body, attrFn, blockFn)
	}
}
