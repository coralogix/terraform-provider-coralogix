// Copyright 2026 Coralogix Ltd.
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

package notifications

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	connectors "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/connectors_service"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const connectorTransportFailureError = "connection refused"

// connectorTransportErrorRoundTripper simulates a transport failure: the HTTP
// call returns (nil response, error), exactly the shape that used to panic when
// Read and Update dereferenced httpResponse.StatusCode. Mirrors #722.
type connectorTransportErrorRoundTripper struct{}

func (connectorTransportErrorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New(connectorTransportFailureError)
}

func connectorResourceWithTransport(transport http.RoundTripper) *ConnectorResource {
	cfg := connectors.NewConfiguration()
	cfg.Servers = connectors.ServerConfigurations{{URL: "https://example.com"}}
	cfg.HTTPClient = &http.Client{Transport: transport}

	return &ConnectorResource{client: connectors.NewAPIClient(cfg).ConnectorsServiceAPI}
}

// TestConnectorResourceReadTransportFailure covers the nil-response guard: on a
// transport error the OpenAPI response is nil, and Read must surface an error
// diagnostic rather than panic on httpResponse.StatusCode. Prior state must be
// preserved (the resource is not removed) because the failure is retryable.
func TestConnectorResourceReadTransportFailure(t *testing.T) {
	ctx := context.Background()
	resourceSchema := connectorSchema(ctx)
	priorState := connectorState(ctx, resourceSchema)
	response := frameworkresource.ReadResponse{State: priorState}

	connectorResourceWithTransport(connectorTransportErrorRoundTripper{}).
		Read(ctx, frameworkresource.ReadRequest{State: priorState}, &response)

	if !response.Diagnostics.HasError() {
		t.Fatalf("Read() diagnostics = %v, want a transport-failure error", response.Diagnostics)
	}
	if response.Diagnostics.WarningsCount() != 0 {
		t.Fatalf("Read() warning count = %d, want 0 for a retryable failure", response.Diagnostics.WarningsCount())
	}
	if response.State.Raw.IsNull() {
		t.Fatal("Read() removed the resource from state, want prior state preserved for a retryable failure")
	}
	if !response.State.Raw.Equal(priorState.Raw) {
		t.Fatalf("Read() state = %#v, want prior state %#v", response.State.Raw, priorState.Raw)
	}
	assertDiagnosticsContain(t, response.Diagnostics, "Error reading coralogix_connector", connectorTransportFailureError)
}

func assertDiagnosticsContain(t *testing.T, diagnostics diag.Diagnostics, wants ...string) {
	t.Helper()
	var text strings.Builder
	for _, diagnostic := range diagnostics {
		text.WriteString(diagnostic.Summary())
		text.WriteString(" ")
		text.WriteString(diagnostic.Detail())
		text.WriteString("\n")
	}
	for _, want := range wants {
		if !strings.Contains(text.String(), want) {
			t.Errorf("diagnostics = %q, want context %q", text.String(), want)
		}
	}
}

// connectorSchema builds the resource schema, matching the archive_retentions
// transport-failure test's helper style.
func connectorSchema(ctx context.Context) schema.Schema {
	response := frameworkresource.SchemaResponse{}
	(&ConnectorResource{}).Schema(ctx, frameworkresource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		panic("coralogix_connector schema could not be built")
	}
	return response.Schema
}

// connectorState builds a minimal prior state carrying just the id; every other
// attribute is null. That is enough for Read/Update to reach the client call
// whose transport fails.
func connectorState(ctx context.Context, resourceSchema schema.Schema) tfsdk.State {
	terraformType := resourceSchema.Type().TerraformType(ctx)
	objectType, ok := terraformType.(tftypes.Object)
	if !ok {
		panic("coralogix_connector schema Terraform type is not an object")
	}

	attributes := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attrType := range objectType.AttributeTypes {
		attributes[name] = tftypes.NewValue(attrType, nil)
	}
	attributes["id"] = tftypes.NewValue(objectType.AttributeTypes["id"], "connector-id")

	return tfsdk.State{
		Raw:    tftypes.NewValue(terraformType, attributes),
		Schema: resourceSchema,
	}
}
