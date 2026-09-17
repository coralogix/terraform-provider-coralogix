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

package dataengine

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	retss "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/retentions_service"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const archiveRetentionsTransportFailureError = "dial tcp 127.0.0.1:1: connect: connection refused"

// archiveRetentionsTransportErrorRoundTripper simulates a transport failure:
// the HTTP call returns (nil response, error), exactly the shape that used to
// panic when Read dereferenced httpResponse.StatusCode.
type archiveRetentionsTransportErrorRoundTripper struct{}

func (archiveRetentionsTransportErrorRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New(archiveRetentionsTransportFailureError)
}

func archiveRetentionsResourceWithTransport(transport http.RoundTripper) *ArchiveRetentionsResource {
	cfg := retss.NewConfiguration()
	cfg.Servers = retss.ServerConfigurations{{URL: "https://example.com"}}
	cfg.HTTPClient = &http.Client{Transport: transport}

	return &ArchiveRetentionsResource{client: retss.NewAPIClient(cfg).RetentionsServiceAPI}
}

// TestArchiveRetentionsResourceReadTransportFailure covers FINDING 2: on a
// transport error the OpenAPI response is nil, and Read must surface an error
// diagnostic rather than panic on httpResponse.StatusCode. Prior state must be
// preserved (the resource is not removed) because the failure is retryable.
func TestArchiveRetentionsResourceReadTransportFailure(t *testing.T) {
	ctx := context.Background()
	resourceSchema := archiveRetentionsSchema(ctx)
	priorState := archiveRetentionsState(ctx, resourceSchema)
	response := frameworkresource.ReadResponse{State: priorState}

	archiveRetentionsResourceWithTransport(archiveRetentionsTransportErrorRoundTripper{}).
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

	var text strings.Builder
	for _, diagnostic := range response.Diagnostics {
		text.WriteString(diagnostic.Summary())
		text.WriteString(" ")
		text.WriteString(diagnostic.Detail())
		text.WriteString("\n")
	}
	for _, want := range []string{"Error reading coralogix_archive_retentions", archiveRetentionsTransportFailureError} {
		if !strings.Contains(text.String(), want) {
			t.Errorf("Read() diagnostics = %q, want context %q", text.String(), want)
		}
	}
}

// TestExtractCreateArchiveRetentions covers FINDING 3: extractCreateArchiveRetentions
// zips the plan's retentions onto the backend's existing retentions by position.
// When the backend returns fewer retentions than the plan it must return a clear
// error diagnostic rather than panic on existingRetentions[i] or retentions[0].
func TestExtractCreateArchiveRetentions(t *testing.T) {
	ctx := context.Background()
	planWithFour := archiveRetentionsPlanModel(ctx, t, []string{"", "warm", "cold", "frozen"})

	t.Run("zero_backend_retentions", func(t *testing.T) {
		rq, diags := extractCreateArchiveRetentions(ctx, planWithFour, nil)
		if !diags.HasError() {
			t.Fatalf("extractCreateArchiveRetentions() diagnostics = %v, want an error for zero backend retentions", diags)
		}
		if rq != nil {
			t.Fatalf("extractCreateArchiveRetentions() request = %#v, want nil so no partial update is sent", rq)
		}
	})

	t.Run("two_backend_retentions", func(t *testing.T) {
		rq, diags := extractCreateArchiveRetentions(ctx, planWithFour, archiveV1Retentions("a", "b"))
		if !diags.HasError() {
			t.Fatalf("extractCreateArchiveRetentions() diagnostics = %v, want an error when backend has fewer retentions than the plan", diags)
		}
		if rq != nil {
			t.Fatalf("extractCreateArchiveRetentions() request = %#v, want nil so no partial update is sent", rq)
		}
	})

	t.Run("four_backend_retentions_happy_path", func(t *testing.T) {
		rq, diags := extractCreateArchiveRetentions(ctx, planWithFour, archiveV1Retentions("a", "b", "c", "d"))
		if diags.HasError() {
			t.Fatalf("extractCreateArchiveRetentions() diagnostics = %v, want none for a valid four-retention response", diags)
		}
		if rq == nil {
			t.Fatal("extractCreateArchiveRetentions() request = nil, want a populated update request")
		}
		if got := len(rq.RetentionUpdateElements); got != 4 {
			t.Fatalf("extractCreateArchiveRetentions() produced %d update elements, want 4", got)
		}
		// The default (first) retention is always renamed to "Default".
		if got := rq.RetentionUpdateElements[0].Name; got == nil || *got != "Default" {
			t.Fatalf("extractCreateArchiveRetentions() first retention name = %v, want \"Default\"", got)
		}
		// Ids come from the backend retentions, by position.
		wantIDs := []string{"a", "b", "c", "d"}
		for i, want := range wantIDs {
			if got := rq.RetentionUpdateElements[i].Id; got == nil || *got != want {
				t.Fatalf("extractCreateArchiveRetentions() retention[%d].Id = %v, want %q", i, got, want)
			}
		}
	})
}

func archiveV1Retentions(ids ...string) []retss.ArchiveV1Retention {
	retentions := make([]retss.ArchiveV1Retention, 0, len(ids))
	for _, id := range ids {
		id := id
		retentions = append(retentions, retss.ArchiveV1Retention{Id: &id})
	}
	return retentions
}

// archiveRetentionsPlanModel builds a plan model whose Retentions list holds one
// object per supplied name, matching the resource's four-retention shape.
func archiveRetentionsPlanModel(ctx context.Context, t *testing.T, names []string) *ArchiveRetentionsResourceModel {
	t.Helper()
	objects := make([]types.Object, 0, len(names))
	for _, name := range names {
		object, diags := types.ObjectValueFrom(ctx, archiveRetentionAttributes(), &ArchiveRetentionResourceModel{
			ID:       types.StringNull(),
			Order:    types.Int64Null(),
			Name:     types.StringValue(name),
			Editable: types.BoolNull(),
		})
		if diags.HasError() {
			t.Fatalf("building retention object: %v", diags)
		}
		objects = append(objects, object)
	}
	list, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: archiveRetentionAttributes()}, objects)
	if diags.HasError() {
		t.Fatalf("building retentions list: %v", diags)
	}
	return &ArchiveRetentionsResourceModel{
		Retentions: list,
		ID:         types.StringValue(RESOURCE_ID_ARCHIVE_RETENTIONS),
	}
}

func archiveRetentionsSchema(ctx context.Context) schema.Schema {
	response := frameworkresource.SchemaResponse{}
	(&ArchiveRetentionsResource{}).Schema(ctx, frameworkresource.SchemaRequest{}, &response)
	if response.Diagnostics.HasError() {
		panic("coralogix_archive_retentions schema could not be built")
	}
	return response.Schema
}

func archiveRetentionsState(ctx context.Context, resourceSchema schema.Schema) tfsdk.State {
	return tfsdk.State{
		Raw:    archiveRetentionsRawValue(ctx, resourceSchema),
		Schema: resourceSchema,
	}
}

func archiveRetentionsRawValue(ctx context.Context, resourceSchema schema.Schema) tftypes.Value {
	terraformType := resourceSchema.Type().TerraformType(ctx)
	objectType, ok := terraformType.(tftypes.Object)
	if !ok {
		panic("coralogix_archive_retentions schema Terraform type is not an object")
	}

	retentionElemType := objectType.AttributeTypes["retentions"].(tftypes.List).ElementType.(tftypes.Object)
	retentionValues := make([]tftypes.Value, 0, 4)
	for i, name := range []string{"Default", "warm", "cold", "frozen"} {
		retentionValues = append(retentionValues, tftypes.NewValue(retentionElemType, map[string]tftypes.Value{
			"id":       tftypes.NewValue(retentionElemType.AttributeTypes["id"], name+"-id"),
			"order":    tftypes.NewValue(retentionElemType.AttributeTypes["order"], int64(i+1)),
			"name":     tftypes.NewValue(retentionElemType.AttributeTypes["name"], name),
			"editable": tftypes.NewValue(retentionElemType.AttributeTypes["editable"], i != 0),
		}))
	}

	attributes := map[string]tftypes.Value{
		"id": tftypes.NewValue(objectType.AttributeTypes["id"], RESOURCE_ID_ARCHIVE_RETENTIONS),
		"retentions": tftypes.NewValue(
			tftypes.List{ElementType: retentionElemType},
			retentionValues,
		),
	}

	return tftypes.NewValue(terraformType, attributes)
}
