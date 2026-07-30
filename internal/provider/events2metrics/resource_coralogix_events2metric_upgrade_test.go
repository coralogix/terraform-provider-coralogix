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

package events2metrics

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestUpgradeE2MPermutationsV0ToV1ConvertsLimitStringToInt64(t *testing.T) {
	ctx := context.Background()
	prior := permutationsV0List(t, "42", false)

	upgraded, diags := upgradeE2MPermutationsV0ToV1(ctx, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if upgraded.IsNull() {
		t.Fatal("expected non-null permutations object")
	}

	var model PermutationsModel
	diags = upgraded.As(ctx, &model, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("decode upgraded permutations: %v", diags)
	}
	if model.Limit.ValueInt64() != 42 {
		t.Fatalf("limit = %d, want 42", model.Limit.ValueInt64())
	}
	if model.HasExceedLimit.ValueBool() {
		t.Fatal("has_exceed_limit = true, want false")
	}
}

func TestUpgradeE2MPermutationsV0ToV1EmptyList(t *testing.T) {
	ctx := context.Background()
	prior := types.ListValueMust(
		types.ObjectType{AttrTypes: permutationsV0AttrTypes()},
		[]attr.Value{},
	)

	upgraded, diags := upgradeE2MPermutationsV0ToV1(ctx, prior)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if !upgraded.IsNull() {
		t.Fatalf("expected null permutations, got %#v", upgraded)
	}
}

func TestUpgradeE2MPermutationsV0ToV1InvalidLimit(t *testing.T) {
	ctx := context.Background()
	prior := permutationsV0List(t, "not-a-number", false)

	_, diags := upgradeE2MPermutationsV0ToV1(ctx, prior)
	if !diags.HasError() {
		t.Fatal("expected diagnostics for invalid limit string")
	}
}

func TestUpgradeE2MStateV0ToV1AcceptsStringPermutationsLimit(t *testing.T) {
	ctx := context.Background()
	priorSchema := e2mSchemaV0()

	var schemaResp resource.SchemaResponse
	(&Events2MetricResource{}).Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	currentSchema := schemaResp.Schema

	priorState := tfsdk.State{
		Raw:    e2mV0StateWithPermutations(ctx, priorSchema, "e2m-id", "metric_name", "100", false),
		Schema: priorSchema,
	}
	resp := resource.UpgradeStateResponse{State: tfsdk.State{Schema: currentSchema}}

	upgradeE2MStateV0ToV1(ctx, resource.UpgradeStateRequest{State: &priorState}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("state upgrade diagnostics = %v", resp.Diagnostics)
	}

	var limit types.Int64
	diags := resp.State.GetAttribute(ctx, path.Root("permutations").AtName("limit"), &limit)
	if diags.HasError() {
		t.Fatalf("read upgraded permutations.limit: %v", diags)
	}
	if limit.ValueInt64() != 100 {
		t.Fatalf("permutations.limit = %d, want 100", limit.ValueInt64())
	}

	var name types.String
	diags = resp.State.GetAttribute(ctx, path.Root("name"), &name)
	if diags.HasError() {
		t.Fatalf("read upgraded name: %v", diags)
	}
	if name.ValueString() != "metric_name" {
		t.Fatalf("name = %q, want metric_name", name.ValueString())
	}
}

func permutationsV0AttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"limit":            types.StringType,
		"has_exceed_limit": types.BoolType,
	}
}

func permutationsV0List(t *testing.T, limit string, hasExceed bool) types.List {
	t.Helper()
	obj, diags := types.ObjectValue(permutationsV0AttrTypes(), map[string]attr.Value{
		"limit":            types.StringValue(limit),
		"has_exceed_limit": types.BoolValue(hasExceed),
	})
	if diags.HasError() {
		t.Fatalf("build v0 permutations object: %v", diags)
	}
	list, diags := types.ListValue(types.ObjectType{AttrTypes: permutationsV0AttrTypes()}, []attr.Value{obj})
	if diags.HasError() {
		t.Fatalf("build v0 permutations list: %v", diags)
	}
	return list
}

func e2mV0StateWithPermutations(ctx context.Context, resourceSchema schema.Schema, id, name, limit string, hasExceed bool) tftypes.Value {
	terraformType := resourceSchema.Type().TerraformType(ctx)
	objectType, ok := terraformType.(tftypes.Object)
	if !ok {
		panic("e2m schema Terraform type is not an object")
	}

	attributes := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for attrName, attributeType := range objectType.AttributeTypes {
		attributes[attrName] = tftypes.NewValue(attributeType, nil)
	}
	attributes["id"] = tftypes.NewValue(objectType.AttributeTypes["id"], id)
	attributes["name"] = tftypes.NewValue(objectType.AttributeTypes["name"], name)

	permutationsType := objectType.AttributeTypes["permutations"].(tftypes.List)
	permutationsElem := permutationsType.ElementType.(tftypes.Object)
	attributes["permutations"] = tftypes.NewValue(permutationsType, []tftypes.Value{
		tftypes.NewValue(permutationsElem, map[string]tftypes.Value{
			"limit":            tftypes.NewValue(permutationsElem.AttributeTypes["limit"], limit),
			"has_exceed_limit": tftypes.NewValue(permutationsElem.AttributeTypes["has_exceed_limit"], hasExceed),
		}),
	})

	return tftypes.NewValue(terraformType, attributes)
}
