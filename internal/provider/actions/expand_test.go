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

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func unsetActionPlan() ActionResourceModel {
	return ActionResourceModel{
		ID:          types.StringValue("04f60316-7043-466f-93a6-5bc3818ce4df"),
		Name:        types.StringValue("example action"),
		URL:         types.StringValue("https://example.com/x"),
		SourceType:  types.StringValue("Log"),
		IsPrivate:   types.BoolValue(false),
		IsHidden:    types.BoolValue(false),
		Description: types.StringNull(),
		DpxlFilter:  types.StringNull(),
		URLFields:   types.ListNull(actionURLFieldElementType()),
	}
}

func TestExtractCreateActionOmitsUnsetOptionalScalars(t *testing.T) {
	rq, diags := extractCreateAction(context.Background(), unsetActionPlan())
	if diags.HasError() {
		t.Fatalf("extractCreateAction diagnostics: %v", diags)
	}
	if rq.Description != nil {
		t.Fatalf("description = %q, want omitted", *rq.Description)
	}
	if rq.DpxlFilter != nil {
		t.Fatalf("dpxlFilter = %q, want omitted", *rq.DpxlFilter)
	}
	if rq.UrlFields != nil {
		t.Fatalf("urlFields = %v, want omitted", rq.UrlFields)
	}
}

func TestExtractCreateActionSendsExplicitEmptyURLFields(t *testing.T) {
	plan := unsetActionPlan()
	plan.Description = types.StringValue("")
	plan.URLFields = types.ListValueMust(actionURLFieldElementType(), []attr.Value{})

	rq, diags := extractCreateAction(context.Background(), plan)
	if diags.HasError() {
		t.Fatalf("extractCreateAction diagnostics: %v", diags)
	}
	if rq.Description == nil || *rq.Description != "" {
		t.Fatalf("description = %v, want an explicit empty string", rq.Description)
	}
	if rq.UrlFields == nil || len(rq.UrlFields) != 0 {
		t.Fatalf("urlFields = %v, want an explicit empty list", rq.UrlFields)
	}
}

// The replace endpoint merges omitted optional scalars, so an unset attribute
// only clears if the update request carries an explicit empty string.
func TestExtractUpdateActionAlwaysSendsClearableFields(t *testing.T) {
	rq, diags := extractUpdateAction(context.Background(), unsetActionPlan())
	if diags.HasError() {
		t.Fatalf("extractUpdateAction diagnostics: %v", diags)
	}
	if rq.Action.Description == nil || *rq.Action.Description != "" {
		t.Fatalf("description = %v, want an explicit empty string", rq.Action.Description)
	}
	if rq.Action.DpxlFilter == nil || *rq.Action.DpxlFilter != "" {
		t.Fatalf("dpxlFilter = %v, want an explicit empty string", rq.Action.DpxlFilter)
	}
	if rq.Action.UrlFields == nil || len(rq.Action.UrlFields) != 0 {
		t.Fatalf("urlFields = %v, want an explicit empty list", rq.Action.UrlFields)
	}
	if rq.Action.IsHidden == nil {
		t.Fatal("isHidden = nil, want a value — the replace endpoint rejects a body without it")
	}
}

func TestExtractUpdateActionSendsURLFieldsInConfiguredOrder(t *testing.T) {
	plan := unsetActionPlan()
	plan.Description = types.StringValue("runbook for disk pressure")
	plan.DpxlFilter = types.StringValue("$d.severity == 'ERROR'")
	plan.URLFields = types.ListValueMust(actionURLFieldElementType(), []attr.Value{
		types.ObjectValueMust(actionURLFieldAttributeTypes(), map[string]attr.Value{
			"name":     types.StringValue("zeta"),
			"required": types.BoolValue(true),
		}),
		types.ObjectValueMust(actionURLFieldAttributeTypes(), map[string]attr.Value{
			"name":     types.StringValue("alpha"),
			"required": types.BoolValue(false),
		}),
	})

	rq, diags := extractUpdateAction(context.Background(), plan)
	if diags.HasError() {
		t.Fatalf("extractUpdateAction diagnostics: %v", diags)
	}
	if *rq.Action.Description != "runbook for disk pressure" {
		t.Fatalf("description = %q", *rq.Action.Description)
	}
	// The configured expression goes out as written; the API adds the `<v1> ` prefix.
	if *rq.Action.DpxlFilter != "$d.severity == 'ERROR'" {
		t.Fatalf("dpxlFilter = %q", *rq.Action.DpxlFilter)
	}
	if len(rq.Action.UrlFields) != 2 {
		t.Fatalf("len(urlFields) = %d, want 2", len(rq.Action.UrlFields))
	}
	if rq.Action.UrlFields[0].Name != "zeta" || rq.Action.UrlFields[1].Name != "alpha" {
		t.Fatalf("urlFields order = [%q, %q], want [zeta, alpha]", rq.Action.UrlFields[0].Name, rq.Action.UrlFields[1].Name)
	}
	if !rq.Action.UrlFields[0].Required || rq.Action.UrlFields[1].Required {
		t.Fatalf("required flags = [%v, %v], want [true, false]", rq.Action.UrlFields[0].Required, rq.Action.UrlFields[1].Required)
	}
}
