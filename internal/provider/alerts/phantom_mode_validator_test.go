// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alerts

import (
	"context"
	"strings"
	"testing"

	alertschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestPhantomModeNotificationGroupValidator(t *testing.T) {
	ctx := context.Background()
	schemaV3 := alertschema.V3()
	rootType := schemaV3.Type().TerraformType(ctx).(tftypes.Object)
	groupType := rootType.AttributeTypes["notification_group"].(tftypes.Object)

	nullGroup := func(overrides map[string]tftypes.Value) tftypes.Value {
		values := map[string]tftypes.Value{}
		for name, attrType := range groupType.AttributeTypes {
			values[name] = tftypes.NewValue(attrType, nil)
		}
		for name, value := range overrides {
			values[name] = value
		}
		return tftypes.NewValue(groupType, values)
	}
	stringList := func(elements ...string) tftypes.Value {
		listType := groupType.AttributeTypes["group_by_keys"].(tftypes.List)
		if len(elements) == 0 {
			return tftypes.NewValue(listType, []tftypes.Value{})
		}
		values := make([]tftypes.Value, 0, len(elements))
		for _, element := range elements {
			values = append(values, tftypes.NewValue(tftypes.String, element))
		}
		return tftypes.NewValue(listType, values)
	}
	webhooks := func(count int) tftypes.Value {
		setType := groupType.AttributeTypes["webhooks_settings"].(tftypes.Set)
		elementType := setType.ElementType.(tftypes.Object)
		elements := make([]tftypes.Value, 0, count)
		for i := 0; i < count; i++ {
			values := map[string]tftypes.Value{}
			for name, attrType := range elementType.AttributeTypes {
				values[name] = tftypes.NewValue(attrType, nil)
			}
			elements = append(elements, tftypes.NewValue(elementType, values))
		}
		return tftypes.NewValue(setType, elements)
	}
	emptyRouter := func() tftypes.Value {
		routerType := groupType.AttributeTypes["router"].(tftypes.Object)
		values := map[string]tftypes.Value{}
		for name, attrType := range routerType.AttributeTypes {
			values[name] = tftypes.NewValue(attrType, nil)
		}
		return tftypes.NewValue(routerType, values)
	}

	for _, testCase := range []struct {
		name              string
		phantomMode       tftypes.Value
		notificationGroup tftypes.Value
		wantError         string
	}{
		{
			name:              "phantom_mode omitted, group names a target",
			phantomMode:       tftypes.NewValue(tftypes.Bool, nil),
			notificationGroup: nullGroup(map[string]tftypes.Value{"group_by_keys": stringList("a")}),
		},
		{
			name:              "phantom_mode unknown",
			phantomMode:       tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue),
			notificationGroup: nullGroup(map[string]tftypes.Value{"group_by_keys": stringList("a")}),
		},
		{
			name:              "phantom_mode false",
			phantomMode:       tftypes.NewValue(tftypes.Bool, false),
			notificationGroup: nullGroup(map[string]tftypes.Value{"group_by_keys": stringList("a")}),
		},
		{
			name:              "phantom_mode true, no notification_group block",
			phantomMode:       tftypes.NewValue(tftypes.Bool, true),
			notificationGroup: tftypes.NewValue(groupType, nil),
		},
		{
			name:              "phantom_mode true, empty notification_group block",
			phantomMode:       tftypes.NewValue(tftypes.Bool, true),
			notificationGroup: nullGroup(nil),
		},
		{
			name:              "phantom_mode true, notification_group unknown",
			phantomMode:       tftypes.NewValue(tftypes.Bool, true),
			notificationGroup: tftypes.NewValue(groupType, tftypes.UnknownValue),
		},
		{
			name:              "phantom_mode true, group_by_keys unknown",
			phantomMode:       tftypes.NewValue(tftypes.Bool, true),
			notificationGroup: nullGroup(map[string]tftypes.Value{"group_by_keys": tftypes.NewValue(groupType.AttributeTypes["group_by_keys"], tftypes.UnknownValue)}),
		},
		{
			name:              "phantom_mode true, group_by_keys empty",
			phantomMode:       tftypes.NewValue(tftypes.Bool, true),
			notificationGroup: nullGroup(map[string]tftypes.Value{"group_by_keys": stringList()}),
		},
		// webhooks_settings carries no SizeAtLeast validator, so an explicit
		// empty set is the case the element-count guard actually has to catch.
		{
			name:              "phantom_mode true, webhooks_settings empty",
			phantomMode:       tftypes.NewValue(tftypes.Bool, true),
			notificationGroup: nullGroup(map[string]tftypes.Value{"webhooks_settings": webhooks(0)}),
		},
		{
			name:              "phantom_mode true, group_by_keys set",
			phantomMode:       tftypes.NewValue(tftypes.Bool, true),
			notificationGroup: nullGroup(map[string]tftypes.Value{"group_by_keys": stringList("a")}),
			wantError:         "group_by_keys",
		},
		{
			name:              "phantom_mode true, webhooks_settings set",
			phantomMode:       tftypes.NewValue(tftypes.Bool, true),
			notificationGroup: nullGroup(map[string]tftypes.Value{"webhooks_settings": webhooks(1)}),
			wantError:         "webhooks_settings",
		},
		{
			name:              "phantom_mode true, empty router object",
			phantomMode:       tftypes.NewValue(tftypes.Bool, true),
			notificationGroup: nullGroup(map[string]tftypes.Value{"router": emptyRouter()}),
			wantError:         "router",
		},
		{
			name:        "phantom_mode true, several targets reported together",
			phantomMode: tftypes.NewValue(tftypes.Bool, true),
			notificationGroup: nullGroup(map[string]tftypes.Value{
				"group_by_keys": stringList("a"),
				"router":        emptyRouter(),
			}),
			wantError: "group_by_keys, router",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			values := map[string]tftypes.Value{}
			for name, attrType := range rootType.AttributeTypes {
				values[name] = tftypes.NewValue(attrType, nil)
			}
			values["phantom_mode"] = testCase.phantomMode
			values["notification_group"] = testCase.notificationGroup

			resp := &resource.ValidateConfigResponse{}
			phantomModeNotificationGroupValidator{}.ValidateResource(
				ctx,
				resource.ValidateConfigRequest{
					Config: tfsdk.Config{Schema: schemaV3, Raw: tftypes.NewValue(rootType, values)},
				},
				resp,
			)

			if testCase.wantError == "" {
				if resp.Diagnostics.HasError() {
					t.Fatalf("expected no error, got %v", resp.Diagnostics.Errors())
				}
				return
			}

			if !resp.Diagnostics.HasError() {
				t.Fatal("expected an error, got none")
			}
			detail := resp.Diagnostics.Errors()[0].Detail()
			if !strings.Contains(detail, "notification_group sets "+testCase.wantError+".") {
				t.Fatalf("expected the diagnostic to name %q, got %q", testCase.wantError, detail)
			}
		})
	}
}
