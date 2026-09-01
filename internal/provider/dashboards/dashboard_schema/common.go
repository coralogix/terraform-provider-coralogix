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
	"strings"
	"time"

	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/dashboardjson"
	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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

// NullWhenContentJSONManaged plans a null value for an attribute that
// content_json always leaves null in state.
//
// The framework marks every Computed attribute that is null in the
// configuration as unknown as soon as the proposed new state differs from
// prior state anywhere, for example when access_policy holds the backend's
// JSON text and the configuration holds an equivalent text. For a
// content_json dashboard the read path writes null into these attributes
// whatever the API returns, so the unknown resolves back to null and the plan
// is never empty. Planning null states the outcome the apply will produce.
type NullWhenContentJSONManaged struct{}

func (m NullWhenContentJSONManaged) Description(_ context.Context) string {
	return "Keeps the attribute null while content_json manages the dashboard."
}

func (m NullWhenContentJSONManaged) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m NullWhenContentJSONManaged) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	// There is no prior plan to stabilise while the resource is created.
	if req.State.Raw.IsNull() {
		return
	}

	// A configured value is the practitioner's, not the provider's, so leave
	// it and the diagnostics it produces unchanged.
	if !req.ConfigValue.IsNull() {
		return
	}

	var contentJSON types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("content_json"), &contentJSON)...)
	if resp.Diagnostics.HasError() || contentJSON.IsNull() || contentJSON.IsUnknown() {
		return
	}

	resp.PlanValue = types.ObjectNull(req.PlanValue.AttributeTypes(ctx))
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
		return
	}

	// Unknown keys are discarded silently, so a misspelled field looks applied
	// but never reaches the API. Warn rather than fail: a newer backend may
	// accept keys this provider's pinned models do not know yet.
	if unknown := unknownContentJSONKeys(request.ConfigValue.ValueString()); len(unknown) > 0 {
		response.Diagnostics.Append(diag.NewAttributeWarningDiagnostic(
			request.Path,
			"Unknown fields in content_json",
			fmt.Sprintf(
				"The provider does not recognise these fields and will drop them, so they will have no effect on the dashboard:\n\n  %s\n\n"+
					"Either a name is misspelled, or the field is newer than the Coralogix SDK this provider is built against. "+
					"A field copied from an existing dashboard's export is usually the second case, and needs a provider release rather than a configuration change.",
				strings.Join(unknown, "\n  "),
			),
		))
	}
}

func stringOrVariableSchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: stringOrVariableAttr(),
		Optional:   true,
		Validators: []validator.Object{
			dashboardwidgets.ExactlyOneOfChildren("string_value", "variable_name"),
		},
	}
}

func stringOrVariableAttr() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"string_value": schema.StringAttribute{
			Optional: true,
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

func dataprimeAnnotationSourceAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: map[string]schema.Attribute{
			"query": schema.StringAttribute{
				Required: true,
			},
			"strategy": schema.SingleNestedAttribute{
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
				Validators: []validator.Object{
					dashboardwidgets.ExactlyOneOfChildren("instant", "range", "duration"),
				},
			},
			"label_fields": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: dashboardwidgets.ObservationFieldSchema(),
				},
				Optional: true,
			},
			"message_template": schema.StringAttribute{
				Optional: true,
			},
			"orientation": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("vertical"),
				Validators: []validator.String{
					stringvalidator.OneOf("vertical", "horizontal"),
				},
			},
			"data_mode_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardwidgets.DashboardValidDataModeTypes...),
				},
			},
		},
		Optional: true,
	}
}

func eventRecurrenceAnnotationSourceAttribute() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Attributes: map[string]schema.Attribute{
			"message_template": schema.StringAttribute{
				Optional: true,
			},
			"recurrence": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"weekly": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"days_of_week": schema.ListAttribute{
								ElementType: types.StringType,
								Required:    true,
								Validators: []validator.List{
									listvalidator.ValueStringsAre(
										stringvalidator.OneOf(dashboardwidgets.DashboardValidWeekdays...),
									),
								},
							},
						},
						Required: true,
					},
				},
				Required: true,
			},
			"strategy": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"instant": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"start_time_hour": schema.Int64Attribute{
								Required: true,
							},
						},
						Optional: true,
					},
					"duration": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"start_time_hour": schema.Int64Attribute{
								Required: true,
							},
							"duration": schema.StringAttribute{
								Required: true,
							},
						},
						Optional: true,
					},
				},
				Required: true,
				Validators: []validator.Object{
					dashboardwidgets.ExactlyOneOfChildren("instant", "duration"),
				},
			},
		},
		Optional: true,
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
					},
				},
				Required: true,
				Validators: []validator.Object{
					dashboardwidgets.ExactlyOneOfChildren("instant", "range"),
				},
			},
		},
		Optional: true,
	}
}
