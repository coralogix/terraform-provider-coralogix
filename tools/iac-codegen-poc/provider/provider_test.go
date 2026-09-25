package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// TestProviderSchema starts the provider server and checks that it serves
// only the generated resource, with no errors. It needs no network access.
func TestProviderSchema(t *testing.T) {
	srv, err := factories[TypeName]()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := srv.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		t.Errorf("%s: %s", d.Summary, d.Detail)
	}
	if len(resp.ResourceSchemas) != 1 || resp.ResourceSchemas["coralogix_ai_evaluation"] == nil {
		t.Errorf("resources = %v, want only coralogix_ai_evaluation", resp.ResourceSchemas)
	}
}
