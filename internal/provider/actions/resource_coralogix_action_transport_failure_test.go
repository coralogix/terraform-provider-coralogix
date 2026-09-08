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

package actions

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	actionss "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/actions_service"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	actionTransportFailureTestID = "8f2c1d4e-5a6b-4c7d-8e9f-0a1b2c3d4e5f"
	actionTransportFailureError  = "dial tcp 127.0.0.1:1: connect: connection refused"
)

type actionTransportErrorRoundTripper struct{}

func (actionTransportErrorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New(actionTransportFailureError)
}

type actionNotFoundRoundTripper struct{}

func (actionNotFoundRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return &http.Response{
		Status:     "404 Not Found",
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"code":5,"message":"action not found"}`)),
		Request:    request,
	}, nil
}

func actionResourceWithTransport(transport http.RoundTripper) *ActionResource {
	cfg := actionss.NewConfiguration()
	cfg.Servers = actionss.ServerConfigurations{{URL: "https://example.com"}}
	cfg.HTTPClient = &http.Client{Transport: transport}

	return &ActionResource{client: actionss.NewAPIClient(cfg).ActionsServiceAPI}
}

func TestActionResourceTransportFailureAndNotFound(t *testing.T) {
	ctx := context.Background()
	resourceSchema := actionTransportFailureSchema(ctx)

	t.Run("read_transport_failure", func(t *testing.T) {
		priorState := actionTransportFailureState(ctx, resourceSchema, actionTransportFailureTestID)
		response := frameworkresource.ReadResponse{State: priorState}

		actionResourceWithTransport(actionTransportErrorRoundTripper{}).
			Read(ctx, frameworkresource.ReadRequest{State: priorState}, &response)

		assertActionTransportFailureDiagnostics(t, response.Diagnostics, "Error reading coralogix_action")
		if response.State.Raw.IsNull() {
			t.Fatal("Read() removed the resource from state, want prior state preserved for a retryable failure")
		}
		if !response.State.Raw.Equal(priorState.Raw) {
			t.Fatalf("Read() state = %#v, want prior state %#v", response.State.Raw, priorState.Raw)
		}
	})

	t.Run("update_transport_failure", func(t *testing.T) {
		plan := actionTransportFailurePlan(ctx, resourceSchema, actionTransportFailureTestID)
		priorState := actionTransportFailureState(ctx, resourceSchema, actionTransportFailureTestID)
		response := frameworkresource.UpdateResponse{State: priorState}

		actionResourceWithTransport(actionTransportErrorRoundTripper{}).
			Update(ctx, frameworkresource.UpdateRequest{Plan: plan, State: priorState}, &response)

		assertActionTransportFailureDiagnostics(t, response.Diagnostics, "Error updating coralogix_action")
		if response.State.Raw.IsNull() {
			t.Fatal("Update() removed the resource from state, want prior state preserved for a retryable failure")
		}
		if !response.State.Raw.Equal(priorState.Raw) {
			t.Fatalf("Update() state = %#v, want prior state %#v", response.State.Raw, priorState.Raw)
		}
	})

	t.Run("delete_transport_failure", func(t *testing.T) {
		priorState := actionTransportFailureState(ctx, resourceSchema, actionTransportFailureTestID)
		response := frameworkresource.DeleteResponse{State: priorState}

		actionResourceWithTransport(actionTransportErrorRoundTripper{}).
			Delete(ctx, frameworkresource.DeleteRequest{State: priorState}, &response)

		assertActionTransportFailureDiagnostics(t, response.Diagnostics, "Error deleting coralogix_action")
	})

	t.Run("delete_not_found", func(t *testing.T) {
		priorState := actionTransportFailureState(ctx, resourceSchema, actionTransportFailureTestID)
		response := frameworkresource.DeleteResponse{State: priorState}

		actionResourceWithTransport(actionNotFoundRoundTripper{}).
			Delete(ctx, frameworkresource.DeleteRequest{State: priorState}, &response)

		if response.Diagnostics.HasError() {
			t.Fatalf("Delete() diagnostics = %v, want none when the action is already gone from the backend", response.Diagnostics)
		}
		if response.Diagnostics.WarningsCount() != 0 {
			t.Fatalf("Delete() warning count = %d, want 0 when the action is already gone from the backend", response.Diagnostics.WarningsCount())
		}
	})
}

func assertActionTransportFailureDiagnostics(t *testing.T, diagnostics diag.Diagnostics, wantSummary string) {
	t.Helper()

	if !diagnostics.HasError() {
		t.Fatalf("diagnostics = %v, want a transport-failure error", diagnostics)
	}
	if diagnostics.WarningsCount() != 0 {
		t.Fatalf("warning count = %d, want 0 for a retryable failure", diagnostics.WarningsCount())
	}

	var text strings.Builder
	for _, diagnostic := range diagnostics {
		text.WriteString(diagnostic.Summary())
		text.WriteString(" ")
		text.WriteString(diagnostic.Detail())
		text.WriteString("\n")
	}
	for _, want := range []string{wantSummary, actionTransportFailureError} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("diagnostics = %q, want context %q", text.String(), want)
		}
	}
}

func actionTransportFailureSchema(ctx context.Context) schema.Schema {
	response := frameworkresource.SchemaResponse{}
	(&ActionResource{}).Schema(ctx, frameworkresource.SchemaRequest{}, &response)

	if response.Diagnostics.HasError() {
		panic("coralogix_action schema could not be built")
	}

	return response.Schema
}

func actionTransportFailurePlan(ctx context.Context, resourceSchema schema.Schema, id string) tfsdk.Plan {
	return tfsdk.Plan{
		Raw:    actionTransportFailureRawValue(ctx, resourceSchema, id),
		Schema: resourceSchema,
	}
}

func actionTransportFailureState(ctx context.Context, resourceSchema schema.Schema, id string) tfsdk.State {
	return tfsdk.State{
		Raw:    actionTransportFailureRawValue(ctx, resourceSchema, id),
		Schema: resourceSchema,
	}
}

func actionTransportFailureRawValue(ctx context.Context, resourceSchema schema.Schema, id string) tftypes.Value {
	terraformType := resourceSchema.Type().TerraformType(ctx)
	objectType, ok := terraformType.(tftypes.Object)
	if !ok {
		panic("coralogix_action schema Terraform type is not an object")
	}

	attributes := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		attributes[name] = tftypes.NewValue(attributeType, nil)
	}
	attributes["id"] = tftypes.NewValue(objectType.AttributeTypes["id"], id)
	attributes["name"] = tftypes.NewValue(objectType.AttributeTypes["name"], "transport failure regression")
	attributes["url"] = tftypes.NewValue(objectType.AttributeTypes["url"], "https://example.com/transport-failure")
	attributes["source_type"] = tftypes.NewValue(objectType.AttributeTypes["source_type"], "Log")

	return tftypes.NewValue(terraformType, attributes)
}
