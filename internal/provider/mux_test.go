// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// The mux serves the resources of both providers, as the binary does.
func TestMuxServerHasTheResourcesOfBothProviders(t *testing.T) {
	ctx := context.Background()
	server, err := MuxServer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server().GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Fatalf("%s: %s", d.Summary, d.Detail)
		}
	}
	for _, name := range []string{"coralogix_data_set", "coralogix_global_router"} { // SDKv2, framework
		if _, ok := resp.ResourceSchemas[name]; !ok {
			t.Errorf("the mux has no %s", name)
		}
	}
}
