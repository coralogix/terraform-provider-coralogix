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
	"testing"

	actionss "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/actions_service"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestFlattenConfiguredStringNullsAPIEmptyUnlessConfigured(t *testing.T) {
	empty := ""
	value := "runbook for disk pressure"

	for _, tc := range []struct {
		name string
		api  *string
		plan types.String
		want types.String
	}{
		{"absent api, null config", nil, types.StringNull(), types.StringNull()},
		{"empty api, null config", &empty, types.StringNull(), types.StringNull()},
		{"empty api, explicit empty config", &empty, types.StringValue(""), types.StringValue("")},
		{"absent api, explicit empty config", nil, types.StringValue(""), types.StringValue("")},
		{"value api, null config", &value, types.StringNull(), types.StringValue(value)},
		{"value api, same config", &value, types.StringValue(value), types.StringValue(value)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := flattenConfiguredString(tc.api, tc.plan); !got.Equal(tc.want) {
				t.Fatalf("flattenConfiguredString() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFlattenDpxlFilterKeepsConfiguredFormAcrossVersionPrefix(t *testing.T) {
	bare := "$d.severity == 'ERROR'"
	prefixed := "<v1> $d.severity == 'ERROR'"
	other := "<v1> $d.severity == 'INFO'"
	empty := ""

	for _, tc := range []struct {
		name string
		api  *string
		plan types.String
		want types.String
	}{
		{"bare config, prefixed api", &prefixed, types.StringValue(bare), types.StringValue(bare)},
		{"prefixed config, prefixed api", &prefixed, types.StringValue(prefixed), types.StringValue(prefixed)},
		{"null config, prefixed api", &prefixed, types.StringNull(), types.StringValue(prefixed)},
		{"drifted api wins", &other, types.StringValue(bare), types.StringValue(other)},
		{"empty api is never prefixed", &empty, types.StringNull(), types.StringNull()},
		{"empty api, explicit empty config", &empty, types.StringValue(""), types.StringValue("")},
		{"absent api, null config", nil, types.StringNull(), types.StringNull()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := flattenDpxlFilter(tc.api, tc.plan); !got.Equal(tc.want) {
				t.Fatalf("flattenDpxlFilter() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestFlattenURLFieldsReconcilesAPIEmptyWithNullConfig(t *testing.T) {
	ctx := context.Background()
	emptyList := types.ListValueMust(actionURLFieldElementType(), []attr.Value{})

	got, diags := flattenURLFields(ctx, []actionss.UrlField{}, types.ListNull(actionURLFieldElementType()))
	if diags.HasError() {
		t.Fatalf("flattenURLFields diagnostics: %v", diags)
	}
	if !got.IsNull() {
		t.Fatalf("api [] with null config = %v, want null", got)
	}

	got, diags = flattenURLFields(ctx, []actionss.UrlField{}, emptyList)
	if diags.HasError() {
		t.Fatalf("flattenURLFields diagnostics: %v", diags)
	}
	if !got.Equal(emptyList) {
		t.Fatalf("api [] with explicit [] config = %v, want known empty list", got)
	}
}

func TestFlattenURLFieldsPreservesAPIOrder(t *testing.T) {
	api := []actionss.UrlField{
		{Name: "zeta", Required: true},
		{Name: "alpha", Required: false},
		{Name: "mike", Required: true},
	}

	got, diags := flattenURLFields(context.Background(), api, types.ListNull(actionURLFieldElementType()))
	if diags.HasError() {
		t.Fatalf("flattenURLFields diagnostics: %v", diags)
	}

	var models []ActionURLFieldModel
	if diags := got.ElementsAs(context.Background(), &models, false); diags.HasError() {
		t.Fatalf("ElementsAs diagnostics: %v", diags)
	}
	if len(models) != 3 {
		t.Fatalf("len(models) = %d, want 3", len(models))
	}
	for i, want := range []string{"zeta", "alpha", "mike"} {
		if models[i].Name.ValueString() != want {
			t.Fatalf("url_fields.%d.name = %q, want %q", i, models[i].Name.ValueString(), want)
		}
	}
	if !models[0].Required.ValueBool() || models[1].Required.ValueBool() {
		t.Fatalf("required flags = [%v, %v], want [true, false]", models[0].Required, models[1].Required)
	}
}

func TestFlattenActionWithoutPlanEchoesBackendValues(t *testing.T) {
	id := "04f60316-7043-466f-93a6-5bc3818ce4df"
	name := "example action"
	url := "https://example.com/x"
	empty := ""
	prefixed := "<v1> $d.severity == 'ERROR'"
	sourceType := actionss.V2SOURCETYPE_SOURCE_TYPE_LOG
	action := &actionss.V2Action{
		Id:          &id,
		Name:        &name,
		Url:         &url,
		Description: &empty,
		DpxlFilter:  &prefixed,
		SourceType:  &sourceType,
		UrlFields:   []actionss.UrlField{},
	}

	got, diags := flattenAction(context.Background(), nil, action)
	if diags.HasError() {
		t.Fatalf("flattenAction diagnostics: %v", diags)
	}
	if got.Description.ValueString() != "" || got.Description.IsNull() {
		t.Fatalf("description = %v, want known empty string", got.Description)
	}
	if got.DpxlFilter.ValueString() != prefixed {
		t.Fatalf("dpxl_filter = %v, want %q", got.DpxlFilter, prefixed)
	}
	if got.URLFields.IsNull() || len(got.URLFields.Elements()) != 0 {
		t.Fatalf("url_fields = %v, want known empty list", got.URLFields)
	}
}

func TestFlattenActionWithPlanNormalizesUnsetOptionalScalars(t *testing.T) {
	id := "04f60316-7043-466f-93a6-5bc3818ce4df"
	name := "example action"
	url := "https://example.com/x"
	empty := ""
	bare := "$d.severity == 'ERROR'"
	prefixed := "<v1> $d.severity == 'ERROR'"
	sourceType := actionss.V2SOURCETYPE_SOURCE_TYPE_LOG
	action := &actionss.V2Action{
		Id:          &id,
		Name:        &name,
		Url:         &url,
		Description: &empty,
		DpxlFilter:  &prefixed,
		SourceType:  &sourceType,
		UrlFields:   []actionss.UrlField{},
	}
	plan := &ActionResourceModel{
		Description: types.StringNull(),
		DpxlFilter:  types.StringValue(bare),
		URLFields:   types.ListNull(actionURLFieldElementType()),
	}

	got, diags := flattenAction(context.Background(), plan, action)
	if diags.HasError() {
		t.Fatalf("flattenAction diagnostics: %v", diags)
	}
	if !got.Description.IsNull() {
		t.Fatalf("description = %v, want null", got.Description)
	}
	if got.DpxlFilter.ValueString() != bare {
		t.Fatalf("dpxl_filter = %v, want the configured %q", got.DpxlFilter, bare)
	}
	if !got.URLFields.IsNull() {
		t.Fatalf("url_fields = %v, want null", got.URLFields)
	}
}
