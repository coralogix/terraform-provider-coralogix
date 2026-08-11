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

package dashboard_schema

import (
	"context"
	"fmt"

	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// StaticValueLabelFromValue sets an omitted static values[].label to the sibling
// value during plan. Do not pair with UseStateForUnknown: removing a custom
// label, or changing value while label is omitted, must recompute from value.
type StaticValueLabelFromValue struct{}

func (m StaticValueLabelFromValue) Description(_ context.Context) string {
	return "When label is omitted, use the sibling value."
}

func (m StaticValueLabelFromValue) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m StaticValueLabelFromValue) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.ConfigValue.IsUnknown() {
		return
	}
	if !req.ConfigValue.IsNull() {
		return
	}

	var sibling types.String
	diags := req.Plan.GetAttribute(ctx, req.Path.ParentPath().AtName("value"), &sibling)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if sibling.IsUnknown() {
		resp.PlanValue = types.StringUnknown()
		return
	}
	if sibling.IsNull() {
		return
	}
	resp.PlanValue = sibling
}

// VariablesV2Schema returns the current dashboard variable schema. V2 supports
// static, textbox, and query-backed variables.
func VariablesV2Schema() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Optional: true,
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional: true, Computed: true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseNonNullStateForUnknown()},
				MarkdownDescription: "Variable UUID. The provider generates a UUID when this is omitted.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Variable name. Use letters, digits, and underscores for portability. The API does not enforce this format.",
			},
			"display_name": schema.StringAttribute{Required: true},
			"description":  schema.StringAttribute{Optional: true},
			"display_type": enumAttribute(dashboardwidgets.DashboardValidDisplayTypesV2, "label_value"),
			"display_full_row": schema.BoolAttribute{
				Optional: true,
				Validators: []validator.Bool{
					displayFullRowTextboxValidator{},
				},
			},
			"source": sourceV2Schema(),
			"value":  valueV2Schema(),
		}},
		Validators:          []validator.List{listvalidator.SizeAtLeast(1)},
		MarkdownDescription: "Dashboard variables v2. This replaces `variables`. Both forms can coexist during migration.",
	}
}

func enumAttribute(values []string, defaultValue string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional: true, Computed: true, Default: stringdefault.StaticString(defaultValue),
		Validators:          []validator.String{stringvalidator.OneOf(values...)},
		MarkdownDescription: fmt.Sprintf("Valid values are %q.", values),
	}
}

func sourceV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"static":  staticSourceV2Schema(),
			"textbox": textboxSourceV2Schema(),
			"query":   querySourceV2Schema(),
		},
		Validators: []validator.Object{dashboardwidgets.ExactlyOneOfChildren("static", "query", "textbox")},
	}
}

func allOptionV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"include_all": schema.BoolAttribute{Required: true},
			"label":       schema.StringAttribute{Optional: true},
		},
	}
}

func staticSourceV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"values_order_direction": enumAttribute(dashboardwidgets.DashboardValidOrderDirectionsV2, "none"),
			"all_option":             allOptionV2Schema(),
			"values": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"value": schema.StringAttribute{Required: true},
					"label": schema.StringAttribute{
						Optional: true, Computed: true,
						PlanModifiers: []planmodifier.String{StaticValueLabelFromValue{}},
						MarkdownDescription: "Display label. When omitted, defaults to `value`. " +
							"Setting `label` to the same string as `value` is allowed and does not drift.",
					},
					"is_default": schema.BoolAttribute{Optional: true},
				}},
				Validators: []validator.List{listvalidator.SizeAtLeast(1)},
			},
		},
	}
}

type displayFullRowTextboxValidator struct{}

func (displayFullRowTextboxValidator) Description(context.Context) string {
	return "`display_full_row` can be true only for a textbox source."
}

func (v displayFullRowTextboxValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (displayFullRowTextboxValidator) ValidateBool(ctx context.Context, req validator.BoolRequest, resp *validator.BoolResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() || !req.ConfigValue.ValueBool() {
		return
	}
	var source types.Object
	if diags := req.Config.GetAttribute(ctx, req.Path.ParentPath().AtName("source"), &source); diags.HasError() || source.IsUnknown() {
		return
	}
	if source.IsNull() || source.Attributes()["textbox"].IsNull() {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid Attribute Combination", "`display_full_row` can be true only when `source.textbox` is configured.")
	}
}

func textboxSourceV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"default_value": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"default_string_value":   textboxStringValueV2Schema(),
					"default_numeric_value":  textboxNumericValueV2Schema(),
					"default_regex_value":    textboxStringValueV2Schema(),
					"default_lucene_value":   textboxLuceneValueV2Schema(),
					"default_interval_value": textboxStringValueV2Schema(),
				},
				Validators: []validator.Object{dashboardwidgets.ExactlyOneOfChildren("default_string_value", "default_numeric_value", "default_regex_value", "default_lucene_value", "default_interval_value")},
			},
		},
	}
}

func textboxStringValueV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
		"value": schema.StringAttribute{Required: true},
	}}
}

func textboxNumericValueV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
		"value":      schema.Float64Attribute{Required: true},
		"min":        schema.Float64Attribute{Optional: true},
		"max":        schema.Float64Attribute{Optional: true},
		"is_integer": schema.BoolAttribute{Optional: true},
	}}
}

func textboxLuceneValueV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
		"value":          schema.StringAttribute{Required: true},
		"data_mode_type": enumAttribute(dashboardwidgets.DashboardValidDataModeTypesV2, "high"),
	}}
}

func valueV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"single_string":  stringValueV2Schema(),
			"single_numeric": numericValueV2Schema(),
			"regex":          stringValueV2Schema(),
			"lucene":         luceneValueV2Schema(),
			"interval":       stringValueV2Schema(),
			"multi_string":   multiStringValueV2Schema(),
		},
		Validators: []validator.Object{dashboardwidgets.ExactlyOneOfChildren("multi_string", "single_string", "single_numeric", "regex", "lucene", "interval")},
	}
}

func stringValueV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
		"value": schema.StringAttribute{Required: true},
		"label": schema.StringAttribute{Required: true},
	}}
}

func numericValueV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
		"value": schema.Float64Attribute{Required: true},
		"label": schema.StringAttribute{Required: true},
	}}
}

func luceneValueV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
		"value": schema.StringAttribute{Required: true},
		"label": schema.StringAttribute{Required: true},
	}}
}

func multiStringValueV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"selected_all": emptyObjectV2Schema(
				"Select every value returned by the query. This is the usual default for query variables.",
			),
			"all": emptyObjectV2Schema(
				"Select the synthetic All option from `all_option` (not every fetched value).",
			),
			"list": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
				"values": schema.ListNestedAttribute{Required: true, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"value": stringValueV2Schema(),
				}}},
			}},
		},
		Validators: []validator.Object{dashboardwidgets.ExactlyOneOfChildren("selected_all", "all", "list")},
	}
}

func emptyObjectV2Schema(description string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		Attributes:          map[string]schema.Attribute{},
		MarkdownDescription: description,
	}
}

func querySourceV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"values_order_direction": enumAttribute(dashboardwidgets.DashboardValidOrderDirectionsV2, "asc"),
			"all_option":             allOptionV2Schema(),
			"refresh_strategy": schema.StringAttribute{
				Optional:            true,
				Validators:          []validator.String{stringvalidator.OneOf(dashboardwidgets.DashboardValidVariableV2RefreshStrategies...)},
				MarkdownDescription: fmt.Sprintf("Valid values are %q.", dashboardwidgets.DashboardValidVariableV2RefreshStrategies),
			},
			"value_display_options": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"value_regex": schema.StringAttribute{Optional: true},
					"label_regex": schema.StringAttribute{Optional: true},
				},
				Validators: []validator.Object{
					dashboardwidgets.AtLeastOneOfChildren("value_regex", "label_regex"),
				},
			},
			"logs_query":      logsQueryV2Schema(),
			"spans_query":     spansQueryV2Schema(),
			"metrics_query":   metricsQueryV2Schema(),
			"dataprime_query": dataprimeQueryV2Schema(),
		},
		Validators: []validator.Object{dashboardwidgets.ExactlyOneOfChildren("logs_query", "spans_query", "metrics_query", "dataprime_query")},
	}
}

func logsQueryV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
		"type": schema.SingleNestedAttribute{Required: true, Attributes: map[string]schema.Attribute{
			"field_name":  schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{"log_regex": schema.StringAttribute{Required: true}}},
			"field_value": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{"observation_field": observationFieldV2Schema(true)}},
		}, Validators: []validator.Object{dashboardwidgets.ExactlyOneOfChildren("field_name", "field_value")}},
	}}
}

func spansQueryV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
		"type": schema.SingleNestedAttribute{Required: true, Attributes: map[string]schema.Attribute{
			"field_name": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{"span_regex": schema.StringAttribute{Required: true}}},
			"field_value": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
				"value":             dashboardwidgets.SpansFieldSchema(),
				"observation_field": observationFieldV2Schema(false),
			}, Validators: []validator.Object{dashboardwidgets.ExactlyOneOfChildren("value", "observation_field")}},
		}, Validators: []validator.Object{dashboardwidgets.ExactlyOneOfChildren("field_name", "field_value")}},
	}}
}

func observationFieldV2Schema(required bool) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Required: required, Optional: !required, Attributes: dashboardwidgets.ObservationFieldSchema()}
}

func metricsQueryV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
		"type": schema.SingleNestedAttribute{Required: true, Attributes: map[string]schema.Attribute{
			"metric_name": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{"metric_regex": schema.StringAttribute{Required: true}}},
			"label_name":  schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{"metric_regex": schema.StringAttribute{Required: true}}},
			"label_value": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
				"metric_name": stringOrVariableSchema(),
				"label_name": schema.SingleNestedAttribute{
					Required:   true,
					Attributes: stringOrVariableAttr(),
					Validators: []validator.Object{dashboardwidgets.ExactlyOneOfChildren("string_value", "variable_name")},
				},
				"label_filters": schema.ListNestedAttribute{Optional: true, PlanModifiers: []planmodifier.List{NormalizeEmptyListToNull{}}, NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"metric": stringOrVariableSchema(), "label": stringOrVariableSchema(),
					"operator": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.OneOf("equals", "not_equals")}},
						"selected_values": schema.ListNestedAttribute{Optional: true, PlanModifiers: []planmodifier.List{NormalizeEmptyListToNull{}}, NestedObject: schema.NestedAttributeObject{
							Attributes: stringOrVariableAttr(),
							Validators: []validator.Object{dashboardwidgets.ExactlyOneOfChildren("string_value", "variable_name")},
						}},
					}},
				}}},
			}},
			"promql_query": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
				"query":             schema.StringAttribute{Required: true},
				"promql_query_type": enumAttribute([]string{"instant", "range"}, "instant"),
			}},
		}, Validators: []validator.Object{dashboardwidgets.ExactlyOneOfChildren("metric_name", "label_name", "label_value", "promql_query")}},
	}}
}

func dataprimeQueryV2Schema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{
		"type": schema.SingleNestedAttribute{Required: true, Attributes: map[string]schema.Attribute{
			"query_text": schema.SingleNestedAttribute{Required: true, Attributes: map[string]schema.Attribute{
				"query":          schema.StringAttribute{Required: true, MarkdownDescription: "Dataprime query text."},
				"data_mode_type": enumAttribute(dashboardwidgets.DashboardValidDataModeTypesV2, "high"),
			}},
		}},
	}}
}
