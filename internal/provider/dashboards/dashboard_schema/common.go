// Copyright 2025 Coralogix Ltd.
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

package dashboard_schema

import (
	"context"
	"fmt"
	"time"

	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboardjson"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type NormalizeEmptyListToNull struct{}

func (m NormalizeEmptyListToNull) Description(_ context.Context) string {
	return "Treats an explicit empty list as null so the backend's equivalent representations don't trigger an inconsistent-result diff."
}

func (m NormalizeEmptyListToNull) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m NormalizeEmptyListToNull) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}
	if len(req.PlanValue.Elements()) == 0 {
		resp.PlanValue = types.ListNull(req.PlanValue.ElementType(ctx))
	}
}

type PreserveStateForEquivalentJSON struct{}

func (m PreserveStateForEquivalentJSON) Description(_ context.Context) string {
	return "Preserves the previous state value when the configured JSON is semantically equivalent."
}

func (m PreserveStateForEquivalentJSON) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m PreserveStateForEquivalentJSON) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	if utils.JSONStringsEqual(req.ConfigValue.ValueString(), req.StateValue.ValueString()) {
		resp.PlanValue = req.StateValue
	}
}

type intervalValidator struct{}

func (i intervalValidator) Description(_ context.Context) string {
	return "A duration string, such as 1s or 1m."
}

func (i intervalValidator) MarkdownDescription(_ context.Context) string {
	return "A duration string, such as 1s or 1m."
}

func (i intervalValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() {
		return
	}
	_, err := time.ParseDuration(req.ConfigValue.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("invalid duration", err.Error())
	}
}

type ContentJsonValidator struct{}

func (c ContentJsonValidator) Description(_ context.Context) string {
	return ""
}

func (c ContentJsonValidator) MarkdownDescription(_ context.Context) string {
	return ""
}

func (c ContentJsonValidator) ValidateString(_ context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	err := dashboardjson.Unmarshal([]byte(request.ConfigValue.ValueString()), &dashboardservice.Dashboard{})
	if err != nil {
		response.Diagnostics.Append(diag.NewErrorDiagnostic("content_json validation failed", fmt.Sprintf("json content is not matching layout schema. got an err while unmarshalling - %s", err)))
	}
}

func stringOrVariableSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: stringOrVariableAttr(),
		Optional:   true,
	}
}

func stringOrVariableAttr() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"string_value": schema.StringAttribute{
			Optional: true,
			Validators: []validator.String{
				stringvalidator.ExactlyOneOf(
					path.MatchRelative().AtParent().AtName("variable_name"),
				),
			},
		},
		"variable_name": schema.StringAttribute{
			Optional: true,
		},
	}
}

func logsAndSpansAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"lucene_query": schema.StringAttribute{
			Optional: true,
		},
		"strategy": logsAndSpansStrategy(),
		"message_template": schema.StringAttribute{
			Optional: true,
		},
		"label_fields": schema.ListNestedAttribute{
			NestedObject: schema.NestedAttributeObject{
				Attributes: dashboardwidgets.ObservationFieldSchema(),
			},
			Optional: true,
		},
	}
}

func logsAndSpansStrategy() schema.Attribute {
	return schema.SingleNestedAttribute{
		Attributes: map[string]schema.Attribute{
			"instant": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"timestamp_field": observationFieldSingleNestedAttribute(),
				},
				Optional: true,
			},
			"range": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"start_timestamp_field": observationFieldSingleNestedAttribute(),
					"end_timestamp_field":   observationFieldSingleNestedAttribute(),
				},
				Optional: true,
			},
			"duration": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"start_timestamp_field": observationFieldSingleNestedAttribute(),
					"duration_field":        observationFieldSingleNestedAttribute(),
				},
				Optional: true,
			},
		},
		Required: true,
	}
}

func observationFieldSingleNestedAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: dashboardwidgets.ObservationFieldSchema(),
		Required:   true,
	}
}

func manualAnnotationSourceAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: map[string]schema.Attribute{
			"orientation": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("vertical"),
				Validators: []validator.String{
					stringvalidator.OneOf("vertical", "horizontal"),
				},
			},
			"message_template": schema.StringAttribute{
				Optional: true,
			},
			"strategy": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"instant": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"value": schema.Float64Attribute{
								Optional: true,
							},
							"unit": dashboardwidgets.UnitSchema(),
							"custom_unit": schema.StringAttribute{
								Optional: true,
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("range"),
							),
						},
					},
					"range": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"start_value": schema.Float64Attribute{
								Optional: true,
							},
							"end_value": schema.Float64Attribute{
								Optional: true,
							},
							"unit": dashboardwidgets.UnitSchema(),
							"custom_unit": schema.StringAttribute{
								Optional: true,
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("instant"),
							),
						},
					},
				},
				Required: true,
			},
		},
		Optional: true,
		Validators: []validator.Object{
			objectvalidator.ExactlyOneOf(
				path.MatchRelative().AtParent().AtName("metrics"),
				path.MatchRelative().AtParent().AtName("logs"),
				path.MatchRelative().AtParent().AtName("spans"),
			),
		},
	}
}
