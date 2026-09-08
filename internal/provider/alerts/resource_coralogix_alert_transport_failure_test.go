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

package alerts

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	alerts "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	alertschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const (
	alertTransportFailureTestID = "3dd36c34-2b3c-4d7d-8b0c-1f6b0c1c0f11"
	alertTransportFailureError  = "dial tcp 127.0.0.1:1: connect: connection refused"
)

type alertTransportErrorRoundTripper struct{}

func (alertTransportErrorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New(alertTransportFailureError)
}

func alertTransportFailureResource() *AlertResource {
	cfg := alerts.NewConfiguration()
	cfg.Servers = alerts.ServerConfigurations{{URL: "https://example.com"}}
	cfg.HTTPClient = &http.Client{Transport: alertTransportErrorRoundTripper{}}

	return &AlertResource{client: alerts.NewAPIClient(cfg).AlertDefinitionsServiceAPI}
}

func TestAlertResourceTransportFailureDoesNotPanic(t *testing.T) {
	ctx := context.Background()
	resourceSchema := alertschema.V3()

	t.Run("read", func(t *testing.T) {
		priorState := alertTransportFailureState(ctx, resourceSchema, alertTransportFailureTestID)
		response := frameworkresource.ReadResponse{State: priorState}

		alertTransportFailureResource().Read(ctx, frameworkresource.ReadRequest{State: priorState}, &response)

		assertAlertTransportFailureDiagnostics(t, response.Diagnostics, "Error reading coralogix_alert")
		if response.State.Raw.IsNull() {
			t.Fatal("Read() removed the resource from state, want prior state preserved for a retryable failure")
		}
		if !response.State.Raw.Equal(priorState.Raw) {
			t.Fatalf("Read() state = %#v, want prior state %#v", response.State.Raw, priorState.Raw)
		}
	})

	t.Run("update", func(t *testing.T) {
		plan := alertTransportFailurePlan(ctx, resourceSchema, alertTransportFailureTestID)
		priorState := alertTransportFailureState(ctx, resourceSchema, alertTransportFailureTestID)
		response := frameworkresource.UpdateResponse{State: priorState}

		alertTransportFailureResource().Update(ctx, frameworkresource.UpdateRequest{Plan: plan, State: priorState}, &response)

		assertAlertTransportFailureDiagnostics(t, response.Diagnostics, "Error replacing coralogix_alert")
		if response.State.Raw.IsNull() {
			t.Fatal("Update() removed the resource from state, want prior state preserved for a retryable failure")
		}
		if !response.State.Raw.Equal(priorState.Raw) {
			t.Fatalf("Update() state = %#v, want prior state %#v", response.State.Raw, priorState.Raw)
		}
	})
}

func assertAlertTransportFailureDiagnostics(t *testing.T, diagnostics diag.Diagnostics, wantSummary string) {
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
	for _, want := range []string{wantSummary, alertTransportFailureError} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("diagnostics = %q, want context %q", text.String(), want)
		}
	}
}

func alertTransportFailurePlan(ctx context.Context, resourceSchema schema.Schema, id string) tfsdk.Plan {
	return tfsdk.Plan{
		Raw:    alertTransportFailureRawValue(ctx, resourceSchema, id),
		Schema: resourceSchema,
	}
}

func alertTransportFailureState(ctx context.Context, resourceSchema schema.Schema, id string) tfsdk.State {
	return tfsdk.State{
		Raw:    alertTransportFailureRawValue(ctx, resourceSchema, id),
		Schema: resourceSchema,
	}
}

func alertTransportFailureRawValue(ctx context.Context, resourceSchema schema.Schema, id string) tftypes.Value {
	terraformType := resourceSchema.Type().TerraformType(ctx)
	objectType, ok := terraformType.(tftypes.Object)
	if !ok {
		panic("alert schema Terraform type is not an object")
	}

	attributes := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attributeType := range objectType.AttributeTypes {
		attributes[name] = tftypes.NewValue(attributeType, nil)
	}
	attributes["id"] = tftypes.NewValue(objectType.AttributeTypes["id"], id)
	attributes["name"] = tftypes.NewValue(objectType.AttributeTypes["name"], "transport failure regression")

	return tftypes.NewValue(terraformType, attributes)
}
