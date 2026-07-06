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

package integrations

import (
	"context"
	"testing"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/integration_service"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestIntegrationVersionAttributeDoesNotForceReplacement(t *testing.T) {
	resp := &resource.SchemaResponse{}
	NewIntegrationResource().Schema(context.Background(), resource.SchemaRequest{}, resp)

	versionAttr, ok := resp.Schema.Attributes["version"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("expected version to be a schema.StringAttribute, got %T", resp.Schema.Attributes["version"])
	}
	if len(versionAttr.PlanModifiers) != 0 {
		t.Fatalf("version must carry no plan modifiers so a version change updates in place via Update; "+
			"got %d (a RequiresReplace plan modifier forces destroy-and-recreate, which deletes the integration's managed service account)", len(versionAttr.PlanModifiers))
	}

	keyAttr, ok := resp.Schema.Attributes["integration_key"].(schema.StringAttribute)
	if !ok {
		t.Fatalf("expected integration_key to be a schema.StringAttribute, got %T", resp.Schema.Attributes["integration_key"])
	}
	if len(keyAttr.PlanModifiers) == 0 {
		t.Fatalf("integration_key must keep its RequiresReplace plan modifier: changing the integration type is a different integration")
	}
}

func TestKeysFromPlanAllowsImportedStateWithoutParameters(t *testing.T) {
	model := &IntegrationResourceModel{}

	keys, diags := KeysFromPlan(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("expected imported state without parameters to be accepted, got diagnostics: %v", diags)
	}
	if keys != nil {
		t.Fatalf("expected nil keys for imported state without parameters, got %v", keys)
	}
}

func TestIntegrationDetailWithNilKeysIncludesBackendParameters(t *testing.T) {
	id := "integration-id"
	definitionKey := "slack-central"
	definitionVersion := "1.0.0"
	applicationNameKey := "ApplicationName"
	enabledKey := "Enabled"

	state, diags := integrationDetail(&cxsdk.GetDeployedIntegrationResponse{
		Integration: &cxsdk.DeployedIntegrationInformation{
			Id:                &id,
			DefinitionKey:     &definitionKey,
			DefinitionVersion: &definitionVersion,
			Parameters: []cxsdk.Parameter{
				{
					Key:         &applicationNameKey,
					StringValue: cxsdk.PtrString("svc-as-code"),
				},
				{
					Key:          &enabledKey,
					BooleanValue: cxsdk.PtrBool(true),
				},
			},
		},
	}, nil)

	if diags.HasError() {
		t.Fatalf("expected integration detail to include all backend parameters, got diagnostics: %v", diags)
	}

	parameters, ok := state.Parameters.UnderlyingValue().(types.Object)
	if !ok {
		t.Fatalf("expected parameters to be an object, got %T", state.Parameters.UnderlyingValue())
	}

	attributes := parameters.Attributes()
	if got := attributes[applicationNameKey].(types.String).ValueString(); got != "svc-as-code" {
		t.Fatalf("expected %s parameter to be %q, got %q", applicationNameKey, "svc-as-code", got)
	}
	if got := attributes[enabledKey].(types.Bool).ValueBool(); !got {
		t.Fatalf("expected %s parameter to be true", enabledKey)
	}
}
