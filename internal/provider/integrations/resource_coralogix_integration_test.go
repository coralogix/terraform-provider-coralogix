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
	"fmt"
	"math/big"
	"slices"
	"sync"
	"testing"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/integration_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/integration"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

func TestParametersToDynamicWithNilKeysIncludesBackendParameters(t *testing.T) {
	applicationNameKey := "ApplicationName"
	enabledKey := "Enabled"

	parametersValue, diags := parametersToDynamic([]cxsdk.Parameter{
		{Key: &applicationNameKey, StringValue: cxsdk.PtrString("svc-as-code")},
		{Key: &enabledKey, BooleanValue: cxsdk.PtrBool(true)},
	}, nil)
	if diags.HasError() {
		t.Fatalf("expected every backend parameter, got diagnostics: %v", diags)
	}

	parameters, ok := parametersValue.UnderlyingValue().(types.Object)
	if !ok {
		t.Fatalf("expected parameters to be an object, got %T", parametersValue.UnderlyingValue())
	}

	attributes := parameters.Attributes()
	if got := attributes[applicationNameKey].(types.String).ValueString(); got != "svc-as-code" {
		t.Fatalf("expected %s parameter to be %q, got %q", applicationNameKey, "svc-as-code", got)
	}
	if got := attributes[enabledKey].(types.Bool).ValueBool(); !got {
		t.Fatalf("expected %s parameter to be true", enabledKey)
	}
}

func parametersObject(attrs map[string]attr.Value, attrTypes map[string]attr.Type) types.Dynamic {
	return types.DynamicValue(types.ObjectValueMust(attrTypes, attrs))
}

func TestDynamicToParametersSortsKeysAndMapsTypes(t *testing.T) {
	parameters := parametersObject(map[string]attr.Value{
		"b": types.BoolValue(true),
		"a": types.StringValue("x"),
		"n": types.NumberValue(big.NewFloat(3)),
		"l": types.TupleValueMust([]attr.Type{types.StringType}, []attr.Value{types.StringValue("y")}),
	}, map[string]attr.Type{
		"b": types.BoolType, "a": types.StringType, "n": types.NumberType,
		"l": types.TupleType{ElemTypes: []attr.Type{types.StringType}},
	})

	got, diags := dynamicToParameters(parameters)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	var keys []string
	for _, p := range got {
		keys = append(keys, *p.Key)
	}
	if want := []string{"a", "b", "l", "n"}; !slices.Equal(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	if *got[0].StringValue != "x" || !*got[1].BooleanValue || got[2].StringList.Values[0] != "y" || *got[3].NumericValue != 3 {
		t.Fatalf("parameters = %+v", got)
	}
}

func TestDynamicToParametersRejectsOtherShapes(t *testing.T) {
	_, diags := dynamicToParameters(types.DynamicValue(types.StringValue("x")))
	if !diags.HasError() || diags[0].Summary() != "Parameters have to be an object" {
		t.Fatalf("diagnostics = %v, want Parameters have to be an object", diags)
	}
	nested := parametersObject(map[string]attr.Value{
		"o": types.ObjectValueMust(map[string]attr.Type{}, map[string]attr.Value{}),
	}, map[string]attr.Type{"o": types.ObjectType{AttrTypes: map[string]attr.Type{}}})
	_, diags = dynamicToParameters(nested)
	if !diags.HasError() || diags[0].Summary() != "Invalid parameter type" {
		t.Fatalf("diagnostics = %v, want Invalid parameter type", diags)
	}
}

func TestParametersToDynamicReadsSecretsAndKeys(t *testing.T) {
	keys := []string{"Secret", "Token", "List"}
	got, diags := parametersToDynamic([]cxsdk.Parameter{
		{Key: cxsdk.PtrString("Secret"), SensitiveData: map[string]interface{}{}},
		{Key: cxsdk.PtrString("Token"), ApiKey: &cxsdk.ApiKeyData{Value: cxsdk.PtrString("token-value")}},
		{Key: cxsdk.PtrString("List"), StringList: &cxsdk.StringList{Values: []string{"x"}}},
		{Key: cxsdk.PtrString("Other"), StringValue: cxsdk.PtrString("left out")},
	}, keys)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	attributes := got.UnderlyingValue().(types.Object).Attributes()
	if len(attributes) != 3 || attributes["Secret"].(types.String).ValueString() != "<redacted>" ||
		attributes["Token"].(types.String).ValueString() != "token-value" {
		t.Fatalf("attributes = %v", attributes)
	}
}

// stateWith returns a state of the generated schema with the id and the given parameters.
func stateWith(parameters types.Dynamic) *tfsdk.State {
	state := &tfsdk.State{Schema: integration.Schema(), Raw: tftypes.NewValue(integration.Schema().Type().TerraformType(context.Background()), nil)}
	ctx := context.Background()
	state.SetAttribute(ctx, path.Root("id"), "the-id")
	state.SetAttribute(ctx, path.Root("parameters"), parameters)
	return state
}

func TestSetStateParametersKeepsTheValueOfTheConfiguration(t *testing.T) {
	ctx, _ := withHandoff(context.Background())
	h := &integrationHooks{}
	configured := parametersObject(map[string]attr.Value{"a": types.StringValue("x")}, map[string]attr.Type{"a": types.StringType})
	if diags := h.rememberParameters(ctx, &cxsdk.DeployedIntegrationInformation{
		Id:         "the-id",
		Parameters: []cxsdk.Parameter{{Key: cxsdk.PtrString("a"), StringValue: cxsdk.PtrString("changed")}, {Key: cxsdk.PtrString("b"), StringValue: cxsdk.PtrString("y")}},
	}); diags.HasError() {
		t.Fatal(diags)
	}
	state := stateWith(types.DynamicNull())
	if diags := h.setStateParameters(ctx, state, stateWith(configured)); diags.HasError() {
		t.Fatal(diags)
	}
	var got types.Dynamic
	state.GetAttribute(ctx, path.Root("parameters"), &got)
	if !got.Equal(configured) {
		t.Fatalf("parameters = %v, want the configured value", got)
	}
}

func TestSetStateParametersReadsTheBackendWithoutAPriorValue(t *testing.T) {
	ctx, _ := withHandoff(context.Background())
	h := &integrationHooks{}
	if diags := h.rememberParameters(ctx, &cxsdk.DeployedIntegrationInformation{
		Id:         "the-id",
		Parameters: []cxsdk.Parameter{{Key: cxsdk.PtrString("a"), StringValue: cxsdk.PtrString("x")}, {Key: cxsdk.PtrString("b"), BooleanValue: cxsdk.PtrBool(true)}},
	}); diags.HasError() {
		t.Fatal(diags)
	}
	state := stateWith(types.DynamicNull())
	if diags := h.setStateParameters(ctx, state, stateWith(types.DynamicNull())); diags.HasError() {
		t.Fatal(diags)
	}
	var got types.Dynamic
	state.GetAttribute(ctx, path.Root("parameters"), &got)
	if n := len(got.UnderlyingValue().(types.Object).Attributes()); n != 2 {
		t.Fatalf("parameters = %v, want both backend keys", got)
	}
}

// Two reads of the same integration at once each keep their own parameters: the handoff belongs to
// the call, not to the integration id.
func TestParameterHandoffIsPerCall(t *testing.T) {
	h := &integrationHooks{}
	const calls = 50
	var wg sync.WaitGroup
	errs := make(chan string, calls)
	for i := 0; i < calls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, _ := withHandoff(context.Background())
			if diags := h.rememberParameters(ctx, &cxsdk.DeployedIntegrationInformation{
				Id:         "the-id",
				Parameters: []cxsdk.Parameter{{Key: cxsdk.PtrString("a"), StringValue: cxsdk.PtrString("x")}},
			}); diags.HasError() {
				errs <- diags.Errors()[0].Detail()
				return
			}
			state := stateWith(types.DynamicNull())
			if diags := h.setStateParameters(ctx, state, nil); diags.HasError() {
				errs <- diags.Errors()[0].Detail()
				return
			}
			var got types.Dynamic
			state.GetAttribute(ctx, path.Root("parameters"), &got)
			obj, ok := got.UnderlyingValue().(types.Object)
			if !ok || len(obj.Attributes()) != 1 {
				errs <- fmt.Sprintf("parameters = %v, want the one backend key", got)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

// A value from another resource is unknown in the configuration while Terraform plans, and the
// plan has the resolved object: the request carries the parameters of the plan.
func TestSetParametersExpandsTheParametersOfThePlan(t *testing.T) {
	ctx, handoff := withHandoff(context.Background())
	planned := parametersObject(map[string]attr.Value{"RoleArn": types.StringValue("arn:aws:iam::1:role/r")}, map[string]attr.Type{"RoleArn": types.StringType})
	handoff.planned = planned

	schema := integration.Schema()
	raw := tftypes.NewValue(schema.Type().TerraformType(ctx), nil)
	config := tfsdk.Config{Schema: schema, Raw: raw}
	state := &tfsdk.State{Schema: schema, Raw: raw}
	state.SetAttribute(ctx, path.Root("parameters"), types.DynamicUnknown())
	config.Raw = state.Raw

	body := &cxsdk.SaveIntegrationRequest{}
	if diags := (&integrationHooks{}).setParameters(ctx, config, body); diags.HasError() {
		t.Fatal(diags)
	}
	got := body.Metadata.IntegrationParameters.Parameters
	if len(got) != 1 || got[0].GetKey() != "RoleArn" || got[0].GetStringValue() != "arn:aws:iam::1:role/r" {
		t.Fatalf("parameters = %+v, want RoleArn from the plan", got)
	}
}
