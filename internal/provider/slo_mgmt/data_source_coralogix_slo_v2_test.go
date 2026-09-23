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

package slo_mgmt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	slos "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/slos_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestSLOV2DataSourceReadNotFoundReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer server.Close()

	configuration := slos.NewConfiguration()
	configuration.Servers = slos.ServerConfigurations{{URL: server.URL}}
	configuration.HTTPClient = server.Client()
	dataSource := &SLOV2DataSource{client: slos.NewAPIClient(configuration).SlosServiceAPI}

	ctx := context.Background()
	var schemaResponse datasource.SchemaResponse
	dataSource.Schema(ctx, datasource.SchemaRequest{}, &schemaResponse)

	configState := tfsdk.State{Schema: schemaResponse.Schema}
	diagnostics := configState.Set(ctx, &SLOV2ResourceModel{
		ID:                        types.StringValue("legacy-slo-id"),
		Name:                      types.StringNull(),
		Description:               types.StringNull(),
		Labels:                    types.MapNull(types.StringType),
		Grouping:                  types.ObjectNull(groupingAttr()),
		TargetThresholdPercentage: types.Float32Null(),
		SLI:                       types.ObjectNull(sliAttr()),
		Window:                    types.ObjectNull(map[string]attr.Type{"slo_time_frame": types.StringType}),
		ProductType:               types.StringNull(),
		OwnershipTags:             types.ObjectNull(ownershipTagsAttr()),
		ApmSliMetadata:            types.ObjectNull(apmSliAttr()),
	})
	if diagnostics.HasError() {
		t.Fatalf("failed to build data source config: %v", diagnostics)
	}
	config := tfsdk.Config{Schema: schemaResponse.Schema, Raw: configState.Raw}

	response := datasource.ReadResponse{State: tfsdk.State{Schema: schemaResponse.Schema}}
	dataSource.Read(ctx, datasource.ReadRequest{Config: config}, &response)

	if !response.Diagnostics.HasError() {
		t.Fatalf("Read() diagnostics = %v, want a not-found error", response.Diagnostics)
	}
	if response.Diagnostics.WarningsCount() != 0 {
		t.Fatalf("Read() warning count = %d, want 0", response.Diagnostics.WarningsCount())
	}

	var diagnosticText strings.Builder
	for _, diagnostic := range response.Diagnostics.Errors() {
		diagnosticText.WriteString(diagnostic.Summary())
		diagnosticText.WriteString(" ")
		diagnosticText.WriteString(diagnostic.Detail())
	}
	for _, expected := range []string{"Error reading coralogix_slo_v2", "legacy-slo-id", "not found in the V2 API", "legacy coralogix_slo ID is not valid"} {
		if !strings.Contains(diagnosticText.String(), expected) {
			t.Errorf("Read() diagnostics = %q, want %q", diagnosticText.String(), expected)
		}
	}
}
