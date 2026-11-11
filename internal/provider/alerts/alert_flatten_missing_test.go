// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alerts

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"

	alertapi "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/alert"
)

// A response without the object that holds the webhook integration flattens with null values,
// and one without the alert properties is a missing resource. Before, the generated code read
// through the missing pointer and the provider crashed on refresh and import.
func TestFlattenToleratesMissingInlinedObjects(t *testing.T) {
	id := "3dd36c34-2b3c-4d7d-8b0c-1f6b0c1c0f11"
	if _, diags := alert.Flatten(context.Background(), &alertapi.AlertDef{Id: &id}); !diags.HasError() {
		t.Fatal("flatten without alert properties: want the error for a missing resource, not a value")
	}

	response := &alertapi.AlertDef{Id: &id, AlertDefProperties: &alertapi.AlertDefProperties{
		LogsImmediate:     &alertapi.LogsImmediateType{},
		NotificationGroup: &alertapi.AlertDefNotificationGroup{Webhooks: []alertapi.AlertDefWebhooksSettings{{}}},
	}}
	m, diags := alert.Flatten(context.Background(), response)
	if diags.HasError() {
		t.Fatalf("flatten with a webhook that has no integration: %v", diags)
	}
	group := m.NotificationGroup.Attributes()
	webhooks, ok := group["webhooks_settings"].(types.Set)
	if !ok || len(webhooks.Elements()) != 1 {
		t.Fatalf("webhooks_settings = %v, want one element", group["webhooks_settings"])
	}
	webhook := webhooks.Elements()[0].(types.Object).Attributes()
	if !webhook["integration_id"].IsNull() || !webhook["recipients"].IsNull() {
		t.Errorf("webhook = %v, want a null integration_id and null recipients", webhook)
	}
}

func flowWithAlertDefs(defs ...types.Object) types.Object {
	defType := types.ObjectType{AttrTypes: map[string]attr.Type{"id": types.StringType, "not": types.BoolType}}
	groupType := map[string]attr.Type{"alert_defs": types.SetType{ElemType: defType}}
	stageType := map[string]attr.Type{"flow_stages_groups": types.ListType{ElemType: types.ObjectType{AttrTypes: groupType}}}
	flowType := map[string]attr.Type{"stages": types.ListType{ElemType: types.ObjectType{AttrTypes: stageType}}}
	group := types.ObjectValueMust(groupType, map[string]attr.Value{"alert_defs": types.SetValueMust(defType, objectValues(defs))})
	stage := types.ObjectValueMust(stageType, map[string]attr.Value{"flow_stages_groups": types.ListValueMust(types.ObjectType{AttrTypes: groupType}, []attr.Value{group})})
	flow := types.ObjectValueMust(flowType, map[string]attr.Value{"stages": types.ListValueMust(types.ObjectType{AttrTypes: stageType}, []attr.Value{stage})})
	return types.ObjectValueMust(map[string]attr.Type{"flow": types.ObjectType{AttrTypes: flowType}}, map[string]attr.Value{"flow": flow})
}

func objectValues(objects []types.Object) []attr.Value {
	out := make([]attr.Value, len(objects))
	for i, o := range objects {
		out[i] = o
	}
	return out
}

func alertDef(id string, not types.Bool) types.Object {
	return types.ObjectValueMust(map[string]attr.Type{"id": types.StringType, "not": types.BoolType},
		map[string]attr.Value{"id": types.StringValue(id), "not": not})
}

// The same alert twice in one group is rejected at plan, also when one entry leaves `not` out and
// the other sets it to false: the API keeps one entry for both.
func TestFlowAlertDefsRejectDuplicates(t *testing.T) {
	tests := map[string]struct {
		defs    []types.Object
		wantErr bool
	}{
		"one alert":                           {[]types.Object{alertDef("a", types.BoolNull())}, false},
		"the same alert with and without not": {[]types.Object{alertDef("a", types.BoolNull()), alertDef("a", types.BoolValue(true))}, false},
		"omitted not and false":               {[]types.Object{alertDef("a", types.BoolNull()), alertDef("a", types.BoolValue(false))}, true},
		"two different alerts":                {[]types.Object{alertDef("a", types.BoolNull()), alertDef("b", types.BoolNull())}, false},
		"an unknown flag":                     {[]types.Object{alertDef("a", types.BoolNull()), alertDef("a", types.BoolUnknown())}, false},
	}
	for name, test := range tests {
		diags := checkFlowAlertDefs(flowWithAlertDefs(test.defs...))
		if diags.HasError() != test.wantErr {
			t.Errorf("%s: diagnostics = %v, want an error: %t", name, diags, test.wantErr)
		}
	}
}
