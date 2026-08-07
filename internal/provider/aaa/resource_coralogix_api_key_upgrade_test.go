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

package aaa

import (
	"context"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestUpgradeApiKeyStateV0ToV1(t *testing.T) {
	ctx := context.Background()

	upgrader, ok := (&ApiKeyResource{}).UpgradeState(ctx)[0]
	if !ok {
		t.Fatal("no state upgrader registered for schema version 0")
	}
	if upgrader.PriorSchema == nil {
		t.Fatal("version 0 state upgrader has no PriorSchema")
	}
	if upgrader.StateUpgrader == nil {
		t.Fatal("version 0 state upgrader has no StateUpgrader")
	}
	priorSchema := *upgrader.PriorSchema

	var schemaResp resource.SchemaResponse
	(&ApiKeyResource{}).Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	currentSchema := schemaResp.Schema

	sendDataPermissions := []string{
		"cloud-metadata-ingress:SendData",
		"logs.data-ingress:SendData",
		"metrics.data-ingress:SendData",
		"spans.data-ingress:SendData",
	}

	teamID := "12345"
	userID := "67890"

	t.Run("team_owned_key", func(t *testing.T) {
		raw := apiKeyV0RawState(ctx, t, priorSchema, true, &teamID, nil, []string{"Send Data"})
		upgraded := runApiKeyV0Upgrade(ctx, t, upgrader, priorSchema, currentSchema, raw)

		assertStringAttr(ctx, t, upgraded, path.Root("id"), &apiKeyV0ID)
		assertStringAttr(ctx, t, upgraded, path.Root("name"), &apiKeyV0Name)
		assertStringAttr(ctx, t, upgraded, path.Root("value"), &apiKeyV0Value)
		assertStringAttr(ctx, t, upgraded, path.Root("owner").AtName("team_id"), &teamID)
		assertStringAttr(ctx, t, upgraded, path.Root("owner").AtName("user_id"), nil)
		assertStringAttr(ctx, t, upgraded, path.Root("owner").AtName("organisation_id"), nil)
		assertStringAttr(ctx, t, upgraded, path.Root("access_policy"), nil)
		assertStringSet(ctx, t, upgraded, path.Root("permissions"), sendDataPermissions)
		assertNullSet(ctx, t, upgraded, path.Root("presets"))
		assertBoolAttr(ctx, t, upgraded, path.Root("active"), true)
		assertBoolAttr(ctx, t, upgraded, path.Root("hashed"), false)
	})

	t.Run("user_owned_key", func(t *testing.T) {
		raw := apiKeyV0RawState(ctx, t, priorSchema, true, nil, &userID, []string{"Send Data"})
		upgraded := runApiKeyV0Upgrade(ctx, t, upgrader, priorSchema, currentSchema, raw)

		assertStringAttr(ctx, t, upgraded, path.Root("owner").AtName("user_id"), &userID)
		assertStringAttr(ctx, t, upgraded, path.Root("owner").AtName("team_id"), nil)
		assertStringAttr(ctx, t, upgraded, path.Root("owner").AtName("organisation_id"), nil)
		assertStringSet(ctx, t, upgraded, path.Root("permissions"), sendDataPermissions)
	})

	t.Run("absent_owner", func(t *testing.T) {
		raw := apiKeyV0RawState(ctx, t, priorSchema, false, nil, nil, []string{"Send Data"})
		upgraded := runApiKeyV0Upgrade(ctx, t, upgrader, priorSchema, currentSchema, raw)

		var owner types.Object
		diags := upgraded.GetAttribute(ctx, path.Root("owner"), &owner)
		if diags.HasError() {
			t.Fatalf("read upgraded owner: %v", diags)
		}
		if !owner.IsNull() {
			t.Fatalf("owner = %#v, want null", owner)
		}
		assertStringSet(ctx, t, upgraded, path.Root("permissions"), sendDataPermissions)
	})

	t.Run("empty_roles", func(t *testing.T) {
		raw := apiKeyV0RawState(ctx, t, priorSchema, true, &teamID, nil, nil)
		upgraded := runApiKeyV0Upgrade(ctx, t, upgrader, priorSchema, currentSchema, raw)

		assertStringSet(ctx, t, upgraded, path.Root("permissions"), []string{})
	})
}

var (
	apiKeyV0ID    = "api-key-id"
	apiKeyV0Name  = "team-managed-key"
	apiKeyV0Value = "api-key-value"
)

func runApiKeyV0Upgrade(ctx context.Context, t *testing.T, upgrader resource.StateUpgrader, priorSchema, currentSchema schema.Schema, raw tftypes.Value) tfsdk.State {
	t.Helper()

	priorState := tfsdk.State{Raw: raw, Schema: priorSchema}
	resp := resource.UpgradeStateResponse{State: tfsdk.State{Schema: currentSchema}}

	upgrader.StateUpgrader(ctx, resource.UpgradeStateRequest{State: &priorState}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("state upgrade diagnostics = %v", resp.Diagnostics)
	}
	return resp.State
}

func apiKeyV0RawState(ctx context.Context, t *testing.T, priorSchema schema.Schema, ownerPresent bool, teamID, userID *string, roles []string) tftypes.Value {
	t.Helper()

	terraformType := priorSchema.Type().TerraformType(ctx)
	objectType, ok := terraformType.(tftypes.Object)
	if !ok {
		t.Fatalf("api key v0 schema Terraform type is %T, want tftypes.Object", terraformType)
	}

	attributes := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for attrName, attributeType := range objectType.AttributeTypes {
		attributes[attrName] = tftypes.NewValue(attributeType, nil)
	}
	attributes["id"] = tftypes.NewValue(objectType.AttributeTypes["id"], apiKeyV0ID)
	attributes["name"] = tftypes.NewValue(objectType.AttributeTypes["name"], apiKeyV0Name)
	attributes["value"] = tftypes.NewValue(objectType.AttributeTypes["value"], apiKeyV0Value)
	attributes["active"] = tftypes.NewValue(objectType.AttributeTypes["active"], true)
	attributes["hashed"] = tftypes.NewValue(objectType.AttributeTypes["hashed"], false)

	ownerType, ok := objectType.AttributeTypes["owner"].(tftypes.Object)
	if !ok {
		t.Fatalf("api key v0 owner Terraform type is %T, want tftypes.Object", objectType.AttributeTypes["owner"])
	}
	if ownerPresent {
		attributes["owner"] = tftypes.NewValue(ownerType, map[string]tftypes.Value{
			"team_id": nullableString(ownerType.AttributeTypes["team_id"], teamID),
			"user_id": nullableString(ownerType.AttributeTypes["user_id"], userID),
		})
	} else {
		attributes["owner"] = tftypes.NewValue(ownerType, nil)
	}

	rolesType, ok := objectType.AttributeTypes["roles"].(tftypes.Set)
	if !ok {
		t.Fatalf("api key v0 roles Terraform type is %T, want tftypes.Set", objectType.AttributeTypes["roles"])
	}
	roleValues := make([]tftypes.Value, 0, len(roles))
	for _, role := range roles {
		roleValues = append(roleValues, tftypes.NewValue(rolesType.ElementType, role))
	}
	attributes["roles"] = tftypes.NewValue(rolesType, roleValues)

	return tftypes.NewValue(terraformType, attributes)
}

func nullableString(attributeType tftypes.Type, value *string) tftypes.Value {
	if value == nil {
		return tftypes.NewValue(attributeType, nil)
	}
	return tftypes.NewValue(attributeType, *value)
}

func assertStringAttr(ctx context.Context, t *testing.T, state tfsdk.State, p path.Path, want *string) {
	t.Helper()

	var got types.String
	diags := state.GetAttribute(ctx, p, &got)
	if diags.HasError() {
		t.Fatalf("read %s: %v", p, diags)
	}
	if want == nil {
		if !got.IsNull() {
			t.Fatalf("%s = %#v, want null", p, got)
		}
		return
	}
	if got.IsNull() {
		t.Fatalf("%s = null, want %q", p, *want)
	}
	if got.ValueString() != *want {
		t.Fatalf("%s = %q, want %q", p, got.ValueString(), *want)
	}
}

func assertBoolAttr(ctx context.Context, t *testing.T, state tfsdk.State, p path.Path, want bool) {
	t.Helper()

	var got types.Bool
	diags := state.GetAttribute(ctx, p, &got)
	if diags.HasError() {
		t.Fatalf("read %s: %v", p, diags)
	}
	if got.IsNull() || got.ValueBool() != want {
		t.Fatalf("%s = %#v, want %t", p, got, want)
	}
}

func assertStringSet(ctx context.Context, t *testing.T, state tfsdk.State, p path.Path, want []string) {
	t.Helper()

	var set types.Set
	diags := state.GetAttribute(ctx, p, &set)
	if diags.HasError() {
		t.Fatalf("read %s: %v", p, diags)
	}
	if set.IsNull() {
		t.Fatalf("%s = null, want %v", p, want)
	}

	got := make([]string, 0, len(set.Elements()))
	for _, element := range set.Elements() {
		value, ok := element.(types.String)
		if !ok {
			t.Fatalf("%s element is %T, want types.String", p, element)
		}
		got = append(got, value.ValueString())
	}
	sort.Strings(got)

	sorted := append([]string(nil), want...)
	sort.Strings(sorted)

	if len(got) != len(sorted) {
		t.Fatalf("%s = %v, want %v", p, got, sorted)
	}
	for i := range got {
		if got[i] != sorted[i] {
			t.Fatalf("%s = %v, want %v", p, got, sorted)
		}
	}
}

func assertNullSet(ctx context.Context, t *testing.T, state tfsdk.State, p path.Path) {
	t.Helper()

	var set types.Set
	diags := state.GetAttribute(ctx, p, &set)
	if diags.HasError() {
		t.Fatalf("read %s: %v", p, diags)
	}
	if !set.IsNull() {
		t.Fatalf("%s = %#v, want null", p, set)
	}
}
