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

package dashboard_widgets

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	dashboardSchemaToProtoHorizontalBarsYAxisViewBy = map[string]dashboardservice.HorizontalBarsYAxisViewBy{
		utils.UNSPECIFIED: dashboardservice.HORIZONTALBARSYAXISVIEWBY_Y_AXIS_VIEW_BY_UNSPECIFIED,
		"category":        dashboardservice.HORIZONTALBARSYAXISVIEWBY_Y_AXIS_VIEW_BY_CATEGORY,
		"value":           dashboardservice.HORIZONTALBARSYAXISVIEWBY_Y_AXIS_VIEW_BY_VALUE,
	}
	dashboardProtoToSchemaHorizontalBarsYAxisViewBy = utils.ReverseMap(dashboardSchemaToProtoHorizontalBarsYAxisViewBy)
	dashboardValidHorizontalBarsYAxisViewBy         = utils.GetKeys(dashboardSchemaToProtoHorizontalBarsYAxisViewBy)

	dashboardSchemaToProtoHorizontalBarsMultiYAxisViewBy = map[string]dashboardservice.HorizontalBarsMultiYAxisViewBy{
		utils.UNSPECIFIED: dashboardservice.HORIZONTALBARSMULTIYAXISVIEWBY_Y_AXIS_VIEW_BY_UNSPECIFIED,
		"category":        dashboardservice.HORIZONTALBARSMULTIYAXISVIEWBY_Y_AXIS_VIEW_BY_CATEGORY,
		"value":           dashboardservice.HORIZONTALBARSMULTIYAXISVIEWBY_Y_AXIS_VIEW_BY_VALUE,
	}
	dashboardProtoToSchemaHorizontalBarsMultiYAxisViewBy = utils.ReverseMap(dashboardSchemaToProtoHorizontalBarsMultiYAxisViewBy)
	dashboardValidHorizontalBarsMultiYAxisViewBy         = utils.GetKeys(dashboardSchemaToProtoHorizontalBarsMultiYAxisViewBy)
)

// Shared schema helpers -----------------------------------------------------

func dynamicBarsQueryFieldSettingsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional: true,
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"query_id": schema.StringAttribute{
					Required: true,
				},
				"value_field": schema.SingleNestedAttribute{
					Attributes: ObservationFieldSchema(),
					Optional:   true,
				},
			},
		},
	}
}

func dynamicSortOrderSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"order_direction": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidSortOrderDirections...),
				},
				MarkdownDescription: fmt.Sprintf("The sort order direction. Valid values are: %s.", strings.Join(DashboardValidSortOrderDirections, ", ")),
			},
			"strategy": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"category": schema.BoolAttribute{
						Optional:            true,
						Validators:          []validator.Bool{mustBeTrueValidator{}},
						MarkdownDescription: "Sort by the bar category. Set `true` to select this strategy; the API carries no settings for it.",
					},
					"query_value": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"query_id": schema.StringAttribute{
								Optional:            true,
								MarkdownDescription: "The `query_definitions` entry whose values order the bars. The API accepts this arm without one and stores it, but the result sorts by nothing, so set it.",
							},
						},
					},
					"strategy_type": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Discriminator naming the chosen strategy. The backend stores whatever it is given and does not derive it, so leaving it unset reads back as an empty string. Set it only to match the arm above.",
					},
				},
				Validators: []validator.Object{
					ExactlyOneOfChildren("category", "query_value"),
				},
			},
		},
	}
}

// Per-variant schema --------------------------------------------------------

func dynamicVerticalBarsSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"allow_abbreviation": schema.BoolAttribute{
				Optional: true,
			},
			"bar_value_display": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidBarValueDisplay...),
				},
				MarkdownDescription: fmt.Sprintf("Where bar values are displayed. Valid values are: %s.", strings.Join(dashboardValidBarValueDisplay, ", ")),
			},
			"category_fields": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"color_scheme": schema.StringAttribute{
				Optional: true,
			},
			"colors_by": ColorsBySchema(),
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 15),
				},
			},
			"group_name_template": schema.StringAttribute{
				Optional: true,
			},
			"hash_colors": schema.BoolAttribute{
				Optional: true,
			},
			"legend": LegendSchema(),
			"max_bars_per_chart": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"max_slices_per_bar": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"scale_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidScaleTypes...),
				},
				MarkdownDescription: fmt.Sprintf("The scale type. Valid values are: %s.", strings.Join(DashboardValidScaleTypes, ", ")),
			},
			"sort_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidSortBy...),
				},
				MarkdownDescription: fmt.Sprintf("How bars are sorted. Valid values are: %s.", strings.Join(DashboardValidSortBy, ", ")),
			},
			"stack_name_template": schema.StringAttribute{
				Optional: true,
			},
			"sub_category_fields": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"unit": UnitSchema(),
			"value_field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"y_axis_max": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The y-axis maximum. Stored at float32 precision by the API.",
			},
			"y_axis_min": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The y-axis minimum. Stored at float32 precision by the API.",
			},
		},
	}
}

func dynamicVerticalBarsMultiSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"allow_abbreviation": schema.BoolAttribute{
				Optional: true,
			},
			"bar_value_display": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidBarValueDisplay...),
				},
				MarkdownDescription: fmt.Sprintf("Where bar values are displayed. Valid values are: %s.", strings.Join(dashboardValidBarValueDisplay, ", ")),
			},
			"category_fields": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"color_scheme": schema.StringAttribute{
				Optional: true,
			},
			"colors_by": ColorsBySchema(),
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 15),
				},
			},
			"group_name_template": schema.StringAttribute{
				Optional: true,
			},
			"hash_colors": schema.BoolAttribute{
				Optional: true,
			},
			"legend": LegendSchema(),
			"max_bars_per_chart": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"query_field_settings": dynamicBarsQueryFieldSettingsSchema(),
			"scale_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidScaleTypes...),
				},
				MarkdownDescription: fmt.Sprintf("The scale type. Valid values are: %s.", strings.Join(DashboardValidScaleTypes, ", ")),
			},
			"sort_order": dynamicSortOrderSchema(),
			"unit":       UnitSchema(),
			"y_axis_max": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The y-axis maximum. Stored at float32 precision by the API.",
			},
			"y_axis_min": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The y-axis minimum. Stored at float32 precision by the API.",
			},
		},
	}
}

func dynamicHorizontalBarsSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"allow_abbreviation": schema.BoolAttribute{
				Optional: true,
			},
			"category_fields": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"color_scheme": schema.StringAttribute{
				Optional: true,
			},
			"colors_by": ColorsBySchema(),
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 15),
				},
			},
			"display_on_bar": schema.BoolAttribute{
				Optional: true,
			},
			"group_name_template": schema.StringAttribute{
				Optional: true,
			},
			"hash_colors": schema.BoolAttribute{
				Optional: true,
			},
			"legend": LegendSchema(),
			"max_bars_per_chart": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"max_slices_per_bar": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"scale_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidScaleTypes...),
				},
				MarkdownDescription: fmt.Sprintf("The scale type. Valid values are: %s.", strings.Join(DashboardValidScaleTypes, ", ")),
			},
			"sort_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidSortBy...),
				},
				MarkdownDescription: fmt.Sprintf("How bars are sorted. Valid values are: %s.", strings.Join(DashboardValidSortBy, ", ")),
			},
			"stack_name_template": schema.StringAttribute{
				Optional: true,
			},
			"sub_category_fields": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"unit": UnitSchema(),
			"value_field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"y_axis_max": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The y-axis maximum. Stored at float32 precision by the API.",
			},
			"y_axis_min": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The y-axis minimum. Stored at float32 precision by the API.",
			},
			"y_axis_view_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidHorizontalBarsYAxisViewBy...),
				},
				MarkdownDescription: fmt.Sprintf("How the y-axis is viewed. Valid values are: %s.", strings.Join(dashboardValidHorizontalBarsYAxisViewBy, ", ")),
			},
		},
	}
}

func dynamicHorizontalBarsMultiSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"allow_abbreviation": schema.BoolAttribute{
				Optional: true,
			},
			"category_fields": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"color_scheme": schema.StringAttribute{
				Optional: true,
			},
			"colors_by": ColorsBySchema(),
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 15),
				},
			},
			"display_on_bar": schema.BoolAttribute{
				Optional: true,
			},
			"group_name_template": schema.StringAttribute{
				Optional: true,
			},
			"hash_colors": schema.BoolAttribute{
				Optional: true,
			},
			"legend": LegendSchema(),
			"max_bars_per_chart": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
			"query_field_settings": dynamicBarsQueryFieldSettingsSchema(),
			"scale_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidScaleTypes...),
				},
				MarkdownDescription: fmt.Sprintf("The scale type. Valid values are: %s.", strings.Join(DashboardValidScaleTypes, ", ")),
			},
			"sort_order": dynamicSortOrderSchema(),
			"unit":       UnitSchema(),
			"y_axis_max": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The y-axis maximum. Stored at float32 precision by the API.",
			},
			"y_axis_min": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The y-axis minimum. Stored at float32 precision by the API.",
			},
			"y_axis_view_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidHorizontalBarsMultiYAxisViewBy...),
				},
				MarkdownDescription: fmt.Sprintf("How the y-axis is viewed. Valid values are: %s.", strings.Join(dashboardValidHorizontalBarsMultiYAxisViewBy, ", ")),
			},
		},
	}
}

// Model attribute maps ------------------------------------------------------

func dynamicBarsQueryFieldSettingsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"query_id":    types.StringType,
		"value_field": ObservationFieldsObject(),
	}
}

func dynamicSortOrderModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"order_direction": types.StringType,
		"strategy":        types.ObjectType{AttrTypes: dynamicSortStrategyModelAttr()},
	}
}

func dynamicSortStrategyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"category":      types.BoolType,
		"query_value":   types.ObjectType{AttrTypes: dynamicSortByQueryValueModelAttr()},
		"strategy_type": types.StringType,
	}
}

func dynamicSortByQueryValueModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"query_id": types.StringType,
	}
}

func dynamicVerticalBarsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"bar_value_display":  types.StringType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_scheme":        types.StringType,
		"colors_by":           types.StringType,
		"custom_unit":         types.StringType,
		"decimal_precision":   types.Int64Type,
		"group_name_template": types.StringType,
		"hash_colors":         types.BoolType,
		"legend":              types.ObjectType{AttrTypes: LegendAttr()},
		"max_bars_per_chart":  types.Int64Type,
		"max_slices_per_bar":  types.Int64Type,
		"scale_type":          types.StringType,
		"sort_by":             types.StringType,
		"stack_name_template": types.StringType,
		"sub_category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"unit":        types.StringType,
		"value_field": ObservationFieldsObject(),
		"y_axis_max":  Float32Type{},
		"y_axis_min":  Float32Type{},
	}
}

func dynamicVerticalBarsMultiModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"bar_value_display":  types.StringType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_scheme":        types.StringType,
		"colors_by":           types.StringType,
		"custom_unit":         types.StringType,
		"decimal_precision":   types.Int64Type,
		"group_name_template": types.StringType,
		"hash_colors":         types.BoolType,
		"legend":              types.ObjectType{AttrTypes: LegendAttr()},
		"max_bars_per_chart":  types.Int64Type,
		"query_field_settings": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicBarsQueryFieldSettingsModelAttr()},
		},
		"scale_type": types.StringType,
		"sort_order": types.ObjectType{AttrTypes: dynamicSortOrderModelAttr()},
		"unit":       types.StringType,
		"y_axis_max": Float32Type{},
		"y_axis_min": Float32Type{},
	}
}

func dynamicHorizontalBarsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_scheme":        types.StringType,
		"colors_by":           types.StringType,
		"custom_unit":         types.StringType,
		"decimal_precision":   types.Int64Type,
		"display_on_bar":      types.BoolType,
		"group_name_template": types.StringType,
		"hash_colors":         types.BoolType,
		"legend":              types.ObjectType{AttrTypes: LegendAttr()},
		"max_bars_per_chart":  types.Int64Type,
		"max_slices_per_bar":  types.Int64Type,
		"scale_type":          types.StringType,
		"sort_by":             types.StringType,
		"stack_name_template": types.StringType,
		"sub_category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"unit":           types.StringType,
		"value_field":    ObservationFieldsObject(),
		"y_axis_max":     Float32Type{},
		"y_axis_min":     Float32Type{},
		"y_axis_view_by": types.StringType,
	}
}

func dynamicHorizontalBarsMultiModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_scheme":        types.StringType,
		"colors_by":           types.StringType,
		"custom_unit":         types.StringType,
		"decimal_precision":   types.Int64Type,
		"display_on_bar":      types.BoolType,
		"group_name_template": types.StringType,
		"hash_colors":         types.BoolType,
		"legend":              types.ObjectType{AttrTypes: LegendAttr()},
		"max_bars_per_chart":  types.Int64Type,
		"query_field_settings": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicBarsQueryFieldSettingsModelAttr()},
		},
		"scale_type":     types.StringType,
		"sort_order":     types.ObjectType{AttrTypes: dynamicSortOrderModelAttr()},
		"unit":           types.StringType,
		"y_axis_max":     Float32Type{},
		"y_axis_min":     Float32Type{},
		"y_axis_view_by": types.StringType,
	}
}

// Shared expand/flatten helpers ---------------------------------------------

func expandDynamicSortOrder(ctx context.Context, sortOrder *DynamicSortOrderModel) (*dashboardservice.VisualizationSortOrder, diag.Diagnostics) {
	if sortOrder == nil {
		return nil, nil
	}

	strategy, diags := expandDynamicSortStrategy(sortOrder.Strategy)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.VisualizationSortOrder{
		OrderDirection: OptionalEnumPointer(sortOrder.OrderDirection, DashboardOrderDirectionSchemaToProto),
		Strategy:       strategy,
	}, nil
}

func expandDynamicSortStrategy(strategy *DynamicSortStrategyModel) (*dashboardservice.SortStrategy, diag.Diagnostics) {
	if strategy == nil {
		return nil, nil
	}

	var queryValue *dashboardservice.SortByQueryValue
	if strategy.QueryValue != nil {
		queryValue = &dashboardservice.SortByQueryValue{
			QueryId: strategy.QueryValue.QueryID.ValueStringPointer(),
		}
	}

	return &dashboardservice.SortStrategy{
		Category:     expandDynamicMappedValuesMarker(strategy.Category),
		QueryValue:   queryValue,
		StrategyType: strategy.StrategyType.ValueStringPointer(),
	}, nil
}

func flattenDynamicSortOrder(sortOrder *dashboardservice.VisualizationSortOrder) *DynamicSortOrderModel {
	if sortOrder == nil {
		return nil
	}
	return &DynamicSortOrderModel{
		OrderDirection: flattenOptionalEnum(sortOrder.OrderDirection, DashboardOrderDirectionProtoToSchema),
		Strategy:       flattenDynamicSortStrategy(sortOrder.Strategy),
	}
}

func flattenDynamicSortStrategy(strategy *dashboardservice.SortStrategy) *DynamicSortStrategyModel {
	if strategy == nil {
		return nil
	}
	var queryValue *DynamicSortByQueryValueModel
	if strategy.QueryValue != nil {
		queryValue = &DynamicSortByQueryValueModel{
			QueryID: types.StringPointerValue(strategy.QueryValue.QueryId),
		}
	}
	return &DynamicSortStrategyModel{
		Category:     flattenDynamicMappedValuesMarker(strategy.Category),
		QueryValue:   queryValue,
		StrategyType: types.StringPointerValue(strategy.StrategyType),
	}
}

func expandDynamicVerticalBarsQueryFieldSettings(ctx context.Context, settings types.List) ([]dashboardservice.VerticalBarsMultiQueryFieldSettings, diag.Diagnostics) {
	var models []DynamicBarsQueryFieldSettingsModel
	diags := settings.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.VerticalBarsMultiQueryFieldSettings, 0, len(models))
	for i := range models {
		valueField, dg := ExpandObservationFieldObject(ctx, models[i].ValueField)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.VerticalBarsMultiQueryFieldSettings{
			QueryId:    models[i].QueryID.ValueString(),
			ValueField: valueField,
		})
	}

	return expanded, diags
}

func expandDynamicHorizontalBarsQueryFieldSettings(ctx context.Context, settings types.List) ([]dashboardservice.HorizontalBarsMultiQueryFieldSettings, diag.Diagnostics) {
	var models []DynamicBarsQueryFieldSettingsModel
	diags := settings.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.HorizontalBarsMultiQueryFieldSettings, 0, len(models))
	for i := range models {
		valueField, dg := ExpandObservationFieldObject(ctx, models[i].ValueField)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.HorizontalBarsMultiQueryFieldSettings{
			QueryId:    models[i].QueryID.ValueString(),
			ValueField: valueField,
		})
	}

	return expanded, diags
}

func flattenDynamicVerticalBarsQueryFieldSettings(ctx context.Context, settings []dashboardservice.VerticalBarsMultiQueryFieldSettings) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: dynamicBarsQueryFieldSettingsModelAttr()}
	if len(settings) == 0 {
		return types.ListNull(elemType), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(settings))
	for i := range settings {
		valueField, dg := FlattenObservationField(ctx, settings[i].ValueField)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		model := &DynamicBarsQueryFieldSettingsModel{
			QueryID:    types.StringValue(settings[i].QueryId),
			ValueField: valueField,
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicBarsQueryFieldSettingsModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(elemType), diagnostics
	}
	return types.ListValueFrom(ctx, elemType, elements)
}

func flattenDynamicHorizontalBarsQueryFieldSettings(ctx context.Context, settings []dashboardservice.HorizontalBarsMultiQueryFieldSettings) (types.List, diag.Diagnostics) {
	elemType := types.ObjectType{AttrTypes: dynamicBarsQueryFieldSettingsModelAttr()}
	if len(settings) == 0 {
		return types.ListNull(elemType), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(settings))
	for i := range settings {
		valueField, dg := FlattenObservationField(ctx, settings[i].ValueField)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		model := &DynamicBarsQueryFieldSettingsModel{
			QueryID:    types.StringValue(settings[i].QueryId),
			ValueField: valueField,
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicBarsQueryFieldSettingsModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(elemType), diagnostics
	}
	return types.ListValueFrom(ctx, elemType, elements)
}

// Per-variant expand --------------------------------------------------------

func expandDynamicVerticalBars(ctx context.Context, bars *DynamicVerticalBarsModel) (*dashboardservice.VerticalBars, diag.Diagnostics) {
	if bars == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, bars.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}
	subCategoryFields, diags := ExpandObservationFields(ctx, bars.SubCategoryFields)
	if diags.HasError() {
		return nil, diags
	}
	valueField, diags := ExpandObservationFieldObject(ctx, bars.ValueField)
	if diags.HasError() {
		return nil, diags
	}
	legend, diags := ExpandLegend(ctx, bars.Legend)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.VerticalBars{
		AllowAbbreviation: bars.AllowAbbreviation.ValueBoolPointer(),
		BarValueDisplay:   OptionalEnumPointer(bars.BarValueDisplay, dashboardSchemaToProtoBarValueDisplay),
		CategoryFields:    categoryFields,
		ColorScheme:       bars.ColorScheme.ValueStringPointer(),
		ColorsBy:          ExpandColorsBy(bars.ColorsBy),
		CustomUnit:        bars.CustomUnit.ValueStringPointer(),
		DecimalPrecision:  expandInt32Pointer(bars.DecimalPrecision),
		GroupNameTemplate: bars.GroupNameTemplate.ValueStringPointer(),
		HashColors:        bars.HashColors.ValueBoolPointer(),
		Legend:            legend,
		MaxBarsPerChart:   expandInt32Pointer(bars.MaxBarsPerChart),
		MaxSlicesPerBar:   expandInt32Pointer(bars.MaxSlicesPerBar),
		ScaleType:         OptionalEnumPointer(bars.ScaleType, DashboardSchemaToProtoScaleType),
		SortBy:            OptionalEnumPointer(bars.SortBy, DashboardSchemaToProtoSortBy),
		StackNameTemplate: bars.StackNameTemplate.ValueStringPointer(),
		SubCategoryFields: subCategoryFields,
		Unit:              OptionalEnumPointer(bars.Unit, DashboardSchemaToProtoUnit),
		ValueField:        valueField,
		YAxisMax:          expandFloat32Pointer(bars.YAxisMax),
		YAxisMin:          expandFloat32Pointer(bars.YAxisMin),
	}, nil
}

func expandDynamicVerticalBarsMulti(ctx context.Context, bars *DynamicVerticalBarsMultiModel) (*dashboardservice.VerticalBarsMulti, diag.Diagnostics) {
	if bars == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, bars.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}
	legend, diags := ExpandLegend(ctx, bars.Legend)
	if diags.HasError() {
		return nil, diags
	}
	queryFieldSettings, diags := expandDynamicVerticalBarsQueryFieldSettings(ctx, bars.QueryFieldSettings)
	if diags.HasError() {
		return nil, diags
	}
	sortOrder, diags := expandDynamicSortOrder(ctx, bars.SortOrder)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.VerticalBarsMulti{
		AllowAbbreviation:  bars.AllowAbbreviation.ValueBoolPointer(),
		BarValueDisplay:    OptionalEnumPointer(bars.BarValueDisplay, dashboardSchemaToProtoBarValueDisplay),
		CategoryFields:     categoryFields,
		ColorScheme:        bars.ColorScheme.ValueStringPointer(),
		ColorsBy:           ExpandColorsBy(bars.ColorsBy),
		CustomUnit:         bars.CustomUnit.ValueStringPointer(),
		DecimalPrecision:   expandInt32Pointer(bars.DecimalPrecision),
		GroupNameTemplate:  bars.GroupNameTemplate.ValueStringPointer(),
		HashColors:         bars.HashColors.ValueBoolPointer(),
		Legend:             legend,
		MaxBarsPerChart:    expandInt32Pointer(bars.MaxBarsPerChart),
		QueryFieldSettings: queryFieldSettings,
		ScaleType:          OptionalEnumPointer(bars.ScaleType, DashboardSchemaToProtoScaleType),
		SortOrder:          sortOrder,
		Unit:               OptionalEnumPointer(bars.Unit, DashboardSchemaToProtoUnit),
		YAxisMax:           expandFloat32Pointer(bars.YAxisMax),
		YAxisMin:           expandFloat32Pointer(bars.YAxisMin),
	}, nil
}

func expandDynamicHorizontalBars(ctx context.Context, bars *DynamicHorizontalBarsModel) (*dashboardservice.HorizontalBars, diag.Diagnostics) {
	if bars == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, bars.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}
	subCategoryFields, diags := ExpandObservationFields(ctx, bars.SubCategoryFields)
	if diags.HasError() {
		return nil, diags
	}
	valueField, diags := ExpandObservationFieldObject(ctx, bars.ValueField)
	if diags.HasError() {
		return nil, diags
	}
	legend, diags := ExpandLegend(ctx, bars.Legend)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.HorizontalBars{
		AllowAbbreviation: bars.AllowAbbreviation.ValueBoolPointer(),
		CategoryFields:    categoryFields,
		ColorScheme:       bars.ColorScheme.ValueStringPointer(),
		ColorsBy:          ExpandColorsBy(bars.ColorsBy),
		CustomUnit:        bars.CustomUnit.ValueStringPointer(),
		DecimalPrecision:  expandInt32Pointer(bars.DecimalPrecision),
		DisplayOnBar:      bars.DisplayOnBar.ValueBoolPointer(),
		GroupNameTemplate: bars.GroupNameTemplate.ValueStringPointer(),
		HashColors:        bars.HashColors.ValueBoolPointer(),
		Legend:            legend,
		MaxBarsPerChart:   expandInt32Pointer(bars.MaxBarsPerChart),
		MaxSlicesPerBar:   expandInt32Pointer(bars.MaxSlicesPerBar),
		ScaleType:         OptionalEnumPointer(bars.ScaleType, DashboardSchemaToProtoScaleType),
		SortBy:            OptionalEnumPointer(bars.SortBy, DashboardSchemaToProtoSortBy),
		StackNameTemplate: bars.StackNameTemplate.ValueStringPointer(),
		SubCategoryFields: subCategoryFields,
		Unit:              OptionalEnumPointer(bars.Unit, DashboardSchemaToProtoUnit),
		ValueField:        valueField,
		YAxisMax:          expandFloat32Pointer(bars.YAxisMax),
		YAxisMin:          expandFloat32Pointer(bars.YAxisMin),
		YAxisViewBy:       OptionalEnumPointer(bars.YAxisViewBy, dashboardSchemaToProtoHorizontalBarsYAxisViewBy),
	}, nil
}

func expandDynamicHorizontalBarsMulti(ctx context.Context, bars *DynamicHorizontalBarsMultiModel) (*dashboardservice.HorizontalBarsMulti, diag.Diagnostics) {
	if bars == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, bars.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}
	legend, diags := ExpandLegend(ctx, bars.Legend)
	if diags.HasError() {
		return nil, diags
	}
	queryFieldSettings, diags := expandDynamicHorizontalBarsQueryFieldSettings(ctx, bars.QueryFieldSettings)
	if diags.HasError() {
		return nil, diags
	}
	sortOrder, diags := expandDynamicSortOrder(ctx, bars.SortOrder)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.HorizontalBarsMulti{
		AllowAbbreviation:  bars.AllowAbbreviation.ValueBoolPointer(),
		CategoryFields:     categoryFields,
		ColorScheme:        bars.ColorScheme.ValueStringPointer(),
		ColorsBy:           ExpandColorsBy(bars.ColorsBy),
		CustomUnit:         bars.CustomUnit.ValueStringPointer(),
		DecimalPrecision:   expandInt32Pointer(bars.DecimalPrecision),
		DisplayOnBar:       bars.DisplayOnBar.ValueBoolPointer(),
		GroupNameTemplate:  bars.GroupNameTemplate.ValueStringPointer(),
		HashColors:         bars.HashColors.ValueBoolPointer(),
		Legend:             legend,
		MaxBarsPerChart:    expandInt32Pointer(bars.MaxBarsPerChart),
		QueryFieldSettings: queryFieldSettings,
		ScaleType:          OptionalEnumPointer(bars.ScaleType, DashboardSchemaToProtoScaleType),
		SortOrder:          sortOrder,
		Unit:               OptionalEnumPointer(bars.Unit, DashboardSchemaToProtoUnit),
		YAxisMax:           expandFloat32Pointer(bars.YAxisMax),
		YAxisMin:           expandFloat32Pointer(bars.YAxisMin),
		YAxisViewBy:        OptionalEnumPointer(bars.YAxisViewBy, dashboardSchemaToProtoHorizontalBarsMultiYAxisViewBy),
	}, nil
}

// Per-variant flatten -------------------------------------------------------

func flattenDynamicVerticalBars(ctx context.Context, bars *dashboardservice.VerticalBars) (*DynamicVerticalBarsModel, diag.Diagnostics) {
	if bars == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, bars.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}
	subCategoryFields, diags := FlattenObservationFields(ctx, bars.GetSubCategoryFields())
	if diags.HasError() {
		return nil, diags
	}
	valueField, diags := FlattenObservationField(ctx, bars.ValueField)
	if diags.HasError() {
		return nil, diags
	}

	colorsBy, dg := FlattenColorsBy(bars.ColorsBy)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &DynamicVerticalBarsModel{
		AllowAbbreviation: types.BoolPointerValue(bars.AllowAbbreviation),
		BarValueDisplay:   flattenOptionalEnum(bars.BarValueDisplay, dashboardProtoToSchemaBarValueDisplay),
		CategoryFields:    categoryFields,
		ColorScheme:       types.StringPointerValue(bars.ColorScheme),
		ColorsBy:          colorsBy,
		CustomUnit:        types.StringPointerValue(bars.CustomUnit),
		DecimalPrecision:  flattenInt32Pointer(bars.DecimalPrecision),
		GroupNameTemplate: types.StringPointerValue(bars.GroupNameTemplate),
		HashColors:        types.BoolPointerValue(bars.HashColors),
		Legend:            FlattenLegend(bars.Legend),
		MaxBarsPerChart:   flattenInt32Pointer(bars.MaxBarsPerChart),
		MaxSlicesPerBar:   flattenInt32Pointer(bars.MaxSlicesPerBar),
		ScaleType:         flattenOptionalEnum(bars.ScaleType, DashboardProtoToSchemaScaleType),
		SortBy:            flattenOptionalEnum(bars.SortBy, DashboardProtoToSchemaSortBy),
		StackNameTemplate: types.StringPointerValue(bars.StackNameTemplate),
		SubCategoryFields: subCategoryFields,
		Unit:              flattenOptionalEnum(bars.Unit, DashboardProtoToSchemaUnit),
		ValueField:        valueField,
		YAxisMax:          flattenFloat32Pointer(bars.YAxisMax),
		YAxisMin:          flattenFloat32Pointer(bars.YAxisMin),
	}, nil
}

func flattenDynamicVerticalBarsMulti(ctx context.Context, bars *dashboardservice.VerticalBarsMulti) (*DynamicVerticalBarsMultiModel, diag.Diagnostics) {
	if bars == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, bars.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}
	queryFieldSettings, diags := flattenDynamicVerticalBarsQueryFieldSettings(ctx, bars.GetQueryFieldSettings())
	if diags.HasError() {
		return nil, diags
	}

	colorsBy, dg := FlattenColorsBy(bars.ColorsBy)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &DynamicVerticalBarsMultiModel{
		AllowAbbreviation:  types.BoolPointerValue(bars.AllowAbbreviation),
		BarValueDisplay:    flattenOptionalEnum(bars.BarValueDisplay, dashboardProtoToSchemaBarValueDisplay),
		CategoryFields:     categoryFields,
		ColorScheme:        types.StringPointerValue(bars.ColorScheme),
		ColorsBy:           colorsBy,
		CustomUnit:         types.StringPointerValue(bars.CustomUnit),
		DecimalPrecision:   flattenInt32Pointer(bars.DecimalPrecision),
		GroupNameTemplate:  types.StringPointerValue(bars.GroupNameTemplate),
		HashColors:         types.BoolPointerValue(bars.HashColors),
		Legend:             FlattenLegend(bars.Legend),
		MaxBarsPerChart:    flattenInt32Pointer(bars.MaxBarsPerChart),
		QueryFieldSettings: queryFieldSettings,
		ScaleType:          flattenOptionalEnum(bars.ScaleType, DashboardProtoToSchemaScaleType),
		SortOrder:          flattenDynamicSortOrder(bars.SortOrder),
		Unit:               flattenOptionalEnum(bars.Unit, DashboardProtoToSchemaUnit),
		YAxisMax:           flattenFloat32Pointer(bars.YAxisMax),
		YAxisMin:           flattenFloat32Pointer(bars.YAxisMin),
	}, nil
}

func flattenDynamicHorizontalBars(ctx context.Context, bars *dashboardservice.HorizontalBars) (*DynamicHorizontalBarsModel, diag.Diagnostics) {
	if bars == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, bars.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}
	subCategoryFields, diags := FlattenObservationFields(ctx, bars.GetSubCategoryFields())
	if diags.HasError() {
		return nil, diags
	}
	valueField, diags := FlattenObservationField(ctx, bars.ValueField)
	if diags.HasError() {
		return nil, diags
	}

	colorsBy, dg := FlattenColorsBy(bars.ColorsBy)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &DynamicHorizontalBarsModel{
		AllowAbbreviation: types.BoolPointerValue(bars.AllowAbbreviation),
		CategoryFields:    categoryFields,
		ColorScheme:       types.StringPointerValue(bars.ColorScheme),
		ColorsBy:          colorsBy,
		CustomUnit:        types.StringPointerValue(bars.CustomUnit),
		DecimalPrecision:  flattenInt32Pointer(bars.DecimalPrecision),
		DisplayOnBar:      types.BoolPointerValue(bars.DisplayOnBar),
		GroupNameTemplate: types.StringPointerValue(bars.GroupNameTemplate),
		HashColors:        types.BoolPointerValue(bars.HashColors),
		Legend:            FlattenLegend(bars.Legend),
		MaxBarsPerChart:   flattenInt32Pointer(bars.MaxBarsPerChart),
		MaxSlicesPerBar:   flattenInt32Pointer(bars.MaxSlicesPerBar),
		ScaleType:         flattenOptionalEnum(bars.ScaleType, DashboardProtoToSchemaScaleType),
		SortBy:            flattenOptionalEnum(bars.SortBy, DashboardProtoToSchemaSortBy),
		StackNameTemplate: types.StringPointerValue(bars.StackNameTemplate),
		SubCategoryFields: subCategoryFields,
		Unit:              flattenOptionalEnum(bars.Unit, DashboardProtoToSchemaUnit),
		ValueField:        valueField,
		YAxisMax:          flattenFloat32Pointer(bars.YAxisMax),
		YAxisMin:          flattenFloat32Pointer(bars.YAxisMin),
		YAxisViewBy:       flattenOptionalEnum(bars.YAxisViewBy, dashboardProtoToSchemaHorizontalBarsYAxisViewBy),
	}, nil
}

func flattenDynamicHorizontalBarsMulti(ctx context.Context, bars *dashboardservice.HorizontalBarsMulti) (*DynamicHorizontalBarsMultiModel, diag.Diagnostics) {
	if bars == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, bars.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}
	queryFieldSettings, diags := flattenDynamicHorizontalBarsQueryFieldSettings(ctx, bars.GetQueryFieldSettings())
	if diags.HasError() {
		return nil, diags
	}

	colorsBy, dg := FlattenColorsBy(bars.ColorsBy)
	if dg != nil {
		return nil, diag.Diagnostics{dg}
	}

	return &DynamicHorizontalBarsMultiModel{
		AllowAbbreviation:  types.BoolPointerValue(bars.AllowAbbreviation),
		CategoryFields:     categoryFields,
		ColorScheme:        types.StringPointerValue(bars.ColorScheme),
		ColorsBy:           colorsBy,
		CustomUnit:         types.StringPointerValue(bars.CustomUnit),
		DecimalPrecision:   flattenInt32Pointer(bars.DecimalPrecision),
		DisplayOnBar:       types.BoolPointerValue(bars.DisplayOnBar),
		GroupNameTemplate:  types.StringPointerValue(bars.GroupNameTemplate),
		HashColors:         types.BoolPointerValue(bars.HashColors),
		Legend:             FlattenLegend(bars.Legend),
		MaxBarsPerChart:    flattenInt32Pointer(bars.MaxBarsPerChart),
		QueryFieldSettings: queryFieldSettings,
		ScaleType:          flattenOptionalEnum(bars.ScaleType, DashboardProtoToSchemaScaleType),
		SortOrder:          flattenDynamicSortOrder(bars.SortOrder),
		Unit:               flattenOptionalEnum(bars.Unit, DashboardProtoToSchemaUnit),
		YAxisMax:           flattenFloat32Pointer(bars.YAxisMax),
		YAxisMin:           flattenFloat32Pointer(bars.YAxisMin),
		YAxisViewBy:        flattenOptionalEnum(bars.YAxisViewBy, dashboardProtoToSchemaHorizontalBarsMultiYAxisViewBy),
	}, nil
}
