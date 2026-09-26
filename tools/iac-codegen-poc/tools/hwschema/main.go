// Command hwschema writes the schema of the handwritten coralogix_ai_evaluation
// resource as text (package schemadump).
//
//	cd tools/hwschema && go run . > ../../schemadump/testdata/handwritten.txt
//
// It is a separate module because the handwritten resource is in an internal
// package of terraform-provider-coralogix. Go allows an internal import only
// from a path under github.com/coralogix/terraform-provider-coralogix, so the
// module path starts with it. go.mod points at the local checkout (read only).
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/ai"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/schemadump"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func main() {
	r, ok := ai.NewAIEvaluationResource().(resource.ResourceWithConfigValidators)
	if !ok {
		fmt.Fprintln(os.Stderr, "hwschema: the resource has no config validators")
		os.Exit(1)
	}
	var resp resource.SchemaResponse
	r.Schema(context.Background(), resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		fmt.Fprintln(os.Stderr, "hwschema:", resp.Diagnostics)
		os.Exit(1)
	}
	fmt.Print(schemadump.Text(schemadump.Dump(resp.Schema, r.ConfigValidators(context.Background()))))
}
