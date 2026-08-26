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
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func dynamicTimeSeriesLinesSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		DeprecationMessage:  "Deprecated: use time_series_lines_multi instead.",
		MarkdownDescription: "Deprecated: use `time_series_lines_multi` instead. Retained at full fidelity for importing dashboards that still use the singular time series lines visualization.",
		Attributes: map[string]schema.Attribute{
			"allow_abbreviation": schema.BoolAttribute{
				Optional: true,
			},
			"category_fields": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeBetween(1, 1000),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"color_scheme": schema.StringAttribute{
				Optional: true,
			},
			"connect_nulls": schema.BoolAttribute{
				Optional: true,
			},
			"custom_unit": DynamicCustomUnitSchema(),
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 15),
				},
			},
			"hash_colors": HashColorsSchema(),
			"legend":      LegendSchema(),
			"scale_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidScaleTypes...),
				},
				MarkdownDescription: fmt.Sprintf("The scale type. Valid values are: %s.", strings.Join(DashboardValidScaleTypes, ", ")),
			},
			"series_count_limit": schema.Int64Attribute{
				Optional: true,
			},
			"series_name_template": schema.StringAttribute{
				Optional: true,
			},
			"stacked_line": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidStackedLine...),
				},
				MarkdownDescription: fmt.Sprintf("How lines are stacked. Valid values are: %s.", strings.Join(dashboardValidStackedLine, ", ")),
			},
			"temporal_field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"tooltip": dynamicTimeSeriesTooltipSchema(),
			"unit":    UnitSchema(),
			"use_data_time_range": schema.BoolAttribute{
				Optional: true,
			},
			"value_fields": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeBetween(1, 1000),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"x_axis_time_format": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidXAxisTimeFormat...),
				},
				MarkdownDescription: fmt.Sprintf("The x-axis time format. Valid values are: %s.", strings.Join(dashboardValidXAxisTimeFormat, ", ")),
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

func dynamicTimeSeriesLinesMultiSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		MarkdownDescription: "Plots one line per query. Editing this widget in the Coralogix UI fills in optional settings the configuration may not set, so a later plan may show those being reset.",
		Attributes: map[string]schema.Attribute{
			"connect_nulls": schema.BoolAttribute{
				Optional: true,
			},
			"legend":                 LegendSchema(),
			"query_display_settings": dynamicQueryDisplaySettingsSchema(),
			"stacked_line": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidStackedLine...),
				},
				MarkdownDescription: fmt.Sprintf("How lines are stacked. Valid values are: %s.", strings.Join(dashboardValidStackedLine, ", ")),
			},
			"tooltip": dynamicTimeSeriesTooltipSchema(),
			"use_data_time_range": schema.BoolAttribute{
				Optional: true,
			},
			"x_axis_time_format": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidXAxisTimeFormat...),
				},
				MarkdownDescription: fmt.Sprintf("The x-axis time format. Valid values are: %s.", strings.Join(dashboardValidXAxisTimeFormat, ", ")),
			},
		},
	}
}

func dynamicTimeSeriesBarsSchema() schema.Attribute {
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
					listvalidator.SizeBetween(1, 1000),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"color_scheme": schema.StringAttribute{
				Optional: true,
			},
			"custom_unit": DynamicCustomUnitSchema(),
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 15),
				},
			},
			"hash_colors": HashColorsSchema(),
			"legend":      LegendSchema(),
			"max_slices_per_bar": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(1, math.MaxInt32),
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
			"series_name_template": schema.StringAttribute{
				Optional: true,
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
			"temporal_field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"tooltip": dynamicTimeSeriesTooltipSchema(),
			"unit":    UnitSchema(),
			"value_fields": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeBetween(1, 1000),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"x_axis_time_format": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidXAxisTimeFormat...),
				},
				MarkdownDescription: fmt.Sprintf("The x-axis time format. Valid values are: %s.", strings.Join(dashboardValidXAxisTimeFormat, ", ")),
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

func dynamicTimeSeriesTooltipSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"show_all_series": schema.BoolAttribute{
				Optional: true,
			},
			"show_labels": schema.BoolAttribute{
				Optional: true,
			},
		},
	}
}

func dynamicTimeSeriesLinesModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_scheme":         types.StringType,
		"connect_nulls":        types.BoolType,
		"custom_unit":          types.StringType,
		"decimal_precision":    types.Int64Type,
		"hash_colors":          types.BoolType,
		"legend":               types.ObjectType{AttrTypes: LegendAttr()},
		"scale_type":           types.StringType,
		"series_count_limit":   types.Int64Type,
		"series_name_template": types.StringType,
		"stacked_line":         types.StringType,
		"temporal_field":       ObservationFieldsObject(),
		"tooltip":              types.ObjectType{AttrTypes: dynamicTimeSeriesTooltipModelAttr()},
		"unit":                 types.StringType,
		"use_data_time_range":  types.BoolType,
		"value_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"x_axis_time_format": types.StringType,
		"y_axis_max":         Float32Type{},
		"y_axis_min":         Float32Type{},
	}
}

func dynamicTimeSeriesLinesMultiModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"connect_nulls": types.BoolType,
		"legend":        types.ObjectType{AttrTypes: LegendAttr()},
		"query_display_settings": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicQueryDisplaySettingsModelAttr()},
		},
		"stacked_line":        types.StringType,
		"tooltip":             types.ObjectType{AttrTypes: dynamicTimeSeriesTooltipModelAttr()},
		"use_data_time_range": types.BoolType,
		"x_axis_time_format":  types.StringType,
	}
}

func dynamicTimeSeriesBarsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"bar_value_display":  types.StringType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_scheme":         types.StringType,
		"custom_unit":          types.StringType,
		"decimal_precision":    types.Int64Type,
		"hash_colors":          types.BoolType,
		"legend":               types.ObjectType{AttrTypes: LegendAttr()},
		"max_slices_per_bar":   types.Int64Type,
		"scale_type":           types.StringType,
		"series_name_template": types.StringType,
		"sort_by":              types.StringType,
		"temporal_field":       ObservationFieldsObject(),
		"tooltip":              types.ObjectType{AttrTypes: dynamicTimeSeriesTooltipModelAttr()},
		"unit":                 types.StringType,
		"value_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"x_axis_time_format": types.StringType,
		"y_axis_max":         Float32Type{},
		"y_axis_min":         Float32Type{},
	}
}

func dynamicTimeSeriesTooltipModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"show_all_series": types.BoolType,
		"show_labels":     types.BoolType,
	}
}

func expandDynamicTimeSeriesLines(ctx context.Context, lines *DynamicTimeSeriesLinesModel) (*dashboardservice.TimeSeriesLines, diag.Diagnostics) {
	if lines == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, lines.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := ExpandObservationFields(ctx, lines.ValueFields)
	if diags.HasError() {
		return nil, diags
	}

	temporalField, diags := ExpandObservationFieldObject(ctx, lines.TemporalField)
	if diags.HasError() {
		return nil, diags
	}

	legend, diags := ExpandLegend(ctx, lines.Legend)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.TimeSeriesLines{
		AllowAbbreviation:  lines.AllowAbbreviation.ValueBoolPointer(),
		CategoryFields:     categoryFields,
		ColorScheme:        lines.ColorScheme.ValueStringPointer(),
		ConnectNulls:       lines.ConnectNulls.ValueBoolPointer(),
		CustomUnit:         lines.CustomUnit.ValueStringPointer(),
		DecimalPrecision:   expandInt32Pointer(lines.DecimalPrecision),
		HashColors:         lines.HashColors.ValueBoolPointer(),
		Legend:             legend,
		ScaleType:          OptionalEnumPointer(lines.ScaleType, DashboardSchemaToProtoScaleType),
		SeriesCountLimit:   int64ToStringPointer(lines.SeriesCountLimit),
		SeriesNameTemplate: lines.SeriesNameTemplate.ValueStringPointer(),
		StackedLine:        OptionalEnumPointer(lines.StackedLine, dashboardSchemaToProtoStackedLine),
		TemporalField:      temporalField,
		Tooltip:            expandDynamicTimeSeriesTooltip(lines.Tooltip),
		Unit:               OptionalEnumPointer(lines.Unit, DashboardSchemaToProtoUnit),
		UseDataTimeRange:   lines.UseDataTimeRange.ValueBoolPointer(),
		ValueFields:        valueFields,
		XAxisTimeFormat:    OptionalEnumPointer(lines.XAxisTimeFormat, dashboardSchemaToProtoXAxisTimeFormat),
		YAxisMax:           expandFloat32Pointer(lines.YAxisMax),
		YAxisMin:           expandFloat32Pointer(lines.YAxisMin),
	}, nil
}

func expandDynamicTimeSeriesLinesMulti(ctx context.Context, linesMulti *DynamicTimeSeriesLinesMultiModel) (*dashboardservice.TimeSeriesLinesMulti, diag.Diagnostics) {
	if linesMulti == nil {
		return nil, nil
	}

	legend, diags := ExpandLegend(ctx, linesMulti.Legend)
	if diags.HasError() {
		return nil, diags
	}

	queryDisplaySettings, diags := expandDynamicQueryDisplaySettings(ctx, linesMulti.QueryDisplaySettings)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.TimeSeriesLinesMulti{
		ConnectNulls:         linesMulti.ConnectNulls.ValueBoolPointer(),
		Legend:               legend,
		QueryDisplaySettings: queryDisplaySettings,
		StackedLine:          OptionalEnumPointer(linesMulti.StackedLine, dashboardSchemaToProtoStackedLine),
		Tooltip:              expandDynamicTimeSeriesTooltip(linesMulti.Tooltip),
		UseDataTimeRange:     linesMulti.UseDataTimeRange.ValueBoolPointer(),
		XAxisTimeFormat:      OptionalEnumPointer(linesMulti.XAxisTimeFormat, dashboardSchemaToProtoXAxisTimeFormat),
	}, nil
}

func expandDynamicTimeSeriesBars(ctx context.Context, bars *DynamicTimeSeriesBarsModel) (*dashboardservice.TimeSeriesBars, diag.Diagnostics) {
	if bars == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, bars.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := ExpandObservationFields(ctx, bars.ValueFields)
	if diags.HasError() {
		return nil, diags
	}

	temporalField, diags := ExpandObservationFieldObject(ctx, bars.TemporalField)
	if diags.HasError() {
		return nil, diags
	}

	legend, diags := ExpandLegend(ctx, bars.Legend)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.TimeSeriesBars{
		AllowAbbreviation:  bars.AllowAbbreviation.ValueBoolPointer(),
		BarValueDisplay:    OptionalEnumPointer(bars.BarValueDisplay, dashboardSchemaToProtoBarValueDisplay),
		CategoryFields:     categoryFields,
		ColorScheme:        bars.ColorScheme.ValueStringPointer(),
		CustomUnit:         bars.CustomUnit.ValueStringPointer(),
		DecimalPrecision:   expandInt32Pointer(bars.DecimalPrecision),
		HashColors:         bars.HashColors.ValueBoolPointer(),
		Legend:             legend,
		MaxSlicesPerBar:    expandInt32Pointer(bars.MaxSlicesPerBar),
		ScaleType:          OptionalEnumPointer(bars.ScaleType, DashboardSchemaToProtoScaleType),
		SeriesNameTemplate: bars.SeriesNameTemplate.ValueStringPointer(),
		SortBy:             OptionalEnumPointer(bars.SortBy, DashboardSchemaToProtoSortBy),
		TemporalField:      temporalField,
		Tooltip:            expandDynamicTimeSeriesTooltip(bars.Tooltip),
		Unit:               OptionalEnumPointer(bars.Unit, DashboardSchemaToProtoUnit),
		ValueFields:        valueFields,
		XAxisTimeFormat:    OptionalEnumPointer(bars.XAxisTimeFormat, dashboardSchemaToProtoXAxisTimeFormat),
		YAxisMax:           expandFloat32Pointer(bars.YAxisMax),
		YAxisMin:           expandFloat32Pointer(bars.YAxisMin),
	}, nil
}

func expandDynamicTimeSeriesTooltip(tooltip *DynamicTimeSeriesTooltipModel) *dashboardservice.TimeSeriesTooltip {
	if tooltip == nil {
		return nil
	}

	return &dashboardservice.TimeSeriesTooltip{
		ShowAllSeries: tooltip.ShowAllSeries.ValueBoolPointer(),
		ShowLabels:    tooltip.ShowLabels.ValueBoolPointer(),
	}
}

func flattenDynamicTimeSeriesLines(ctx context.Context, lines *dashboardservice.TimeSeriesLines) (*DynamicTimeSeriesLinesModel, diag.Diagnostics) {
	if lines == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, lines.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := FlattenObservationFields(ctx, lines.GetValueFields())
	if diags.HasError() {
		return nil, diags
	}

	temporalField, diags := FlattenObservationField(ctx, lines.TemporalField)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTimeSeriesLinesModel{
		AllowAbbreviation:  types.BoolPointerValue(lines.AllowAbbreviation),
		CategoryFields:     categoryFields,
		ColorScheme:        types.StringPointerValue(lines.ColorScheme),
		ConnectNulls:       types.BoolPointerValue(lines.ConnectNulls),
		CustomUnit:         types.StringPointerValue(lines.CustomUnit),
		DecimalPrecision:   flattenInt32Pointer(lines.DecimalPrecision),
		HashColors:         types.BoolPointerValue(lines.HashColors),
		Legend:             FlattenLegend(lines.Legend),
		ScaleType:          flattenOptionalEnum(lines.ScaleType, DashboardProtoToSchemaScaleType),
		SeriesCountLimit:   stringPointerToInt64(lines.SeriesCountLimit),
		SeriesNameTemplate: types.StringPointerValue(lines.SeriesNameTemplate),
		StackedLine:        flattenOptionalEnum(lines.StackedLine, dashboardProtoToSchemaStackedLine),
		TemporalField:      temporalField,
		Tooltip:            flattenDynamicTimeSeriesTooltip(lines.Tooltip),
		Unit:               flattenOptionalEnum(lines.Unit, DashboardProtoToSchemaUnit),
		UseDataTimeRange:   types.BoolPointerValue(lines.UseDataTimeRange),
		ValueFields:        valueFields,
		XAxisTimeFormat:    flattenOptionalEnum(lines.XAxisTimeFormat, dashboardProtoToSchemaXAxisTimeFormat),
		YAxisMax:           flattenFloat32Pointer(lines.YAxisMax),
		YAxisMin:           flattenFloat32Pointer(lines.YAxisMin),
	}, nil
}

func flattenDynamicTimeSeriesLinesMulti(ctx context.Context, linesMulti *dashboardservice.TimeSeriesLinesMulti) (*DynamicTimeSeriesLinesMultiModel, diag.Diagnostics) {
	if linesMulti == nil {
		return nil, nil
	}

	queryDisplaySettings, diags := flattenDynamicQueryDisplaySettings(ctx, linesMulti.GetQueryDisplaySettings())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTimeSeriesLinesMultiModel{
		ConnectNulls:         types.BoolPointerValue(linesMulti.ConnectNulls),
		Legend:               FlattenLegend(linesMulti.Legend),
		QueryDisplaySettings: queryDisplaySettings,
		StackedLine:          flattenOptionalEnum(linesMulti.StackedLine, dashboardProtoToSchemaStackedLine),
		Tooltip:              flattenDynamicTimeSeriesTooltip(linesMulti.Tooltip),
		UseDataTimeRange:     types.BoolPointerValue(linesMulti.UseDataTimeRange),
		XAxisTimeFormat:      flattenOptionalEnum(linesMulti.XAxisTimeFormat, dashboardProtoToSchemaXAxisTimeFormat),
	}, nil
}

func flattenDynamicTimeSeriesBars(ctx context.Context, bars *dashboardservice.TimeSeriesBars) (*DynamicTimeSeriesBarsModel, diag.Diagnostics) {
	if bars == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, bars.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := FlattenObservationFields(ctx, bars.GetValueFields())
	if diags.HasError() {
		return nil, diags
	}

	temporalField, diags := FlattenObservationField(ctx, bars.TemporalField)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTimeSeriesBarsModel{
		AllowAbbreviation:  types.BoolPointerValue(bars.AllowAbbreviation),
		BarValueDisplay:    flattenOptionalEnum(bars.BarValueDisplay, dashboardProtoToSchemaBarValueDisplay),
		CategoryFields:     categoryFields,
		ColorScheme:        types.StringPointerValue(bars.ColorScheme),
		CustomUnit:         types.StringPointerValue(bars.CustomUnit),
		DecimalPrecision:   flattenInt32Pointer(bars.DecimalPrecision),
		HashColors:         types.BoolPointerValue(bars.HashColors),
		Legend:             FlattenLegend(bars.Legend),
		MaxSlicesPerBar:    flattenInt32Pointer(bars.MaxSlicesPerBar),
		ScaleType:          flattenOptionalEnum(bars.ScaleType, DashboardProtoToSchemaScaleType),
		SeriesNameTemplate: types.StringPointerValue(bars.SeriesNameTemplate),
		SortBy:             flattenOptionalEnum(bars.SortBy, DashboardProtoToSchemaSortBy),
		TemporalField:      temporalField,
		Tooltip:            flattenDynamicTimeSeriesTooltip(bars.Tooltip),
		Unit:               flattenOptionalEnum(bars.Unit, DashboardProtoToSchemaUnit),
		ValueFields:        valueFields,
		XAxisTimeFormat:    flattenOptionalEnum(bars.XAxisTimeFormat, dashboardProtoToSchemaXAxisTimeFormat),
		YAxisMax:           flattenFloat32Pointer(bars.YAxisMax),
		YAxisMin:           flattenFloat32Pointer(bars.YAxisMin),
	}, nil
}

func flattenDynamicTimeSeriesTooltip(tooltip *dashboardservice.TimeSeriesTooltip) *DynamicTimeSeriesTooltipModel {
	if tooltip == nil {
		return nil
	}

	return &DynamicTimeSeriesTooltipModel{
		ShowAllSeries: types.BoolPointerValue(tooltip.ShowAllSeries),
		ShowLabels:    types.BoolPointerValue(tooltip.ShowLabels),
	}
}

var dashboardValidStackedLine = utils.GetKeys(dashboardSchemaToProtoStackedLine)

var dashboardValidXAxisTimeFormat = utils.GetKeys(dashboardSchemaToProtoXAxisTimeFormat)

func dynamicQueryDisplaySettingsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional:            true,
		MarkdownDescription: "Per-query display settings. Each entry styles one query, named by `query_id`.",
		Validators: []validator.List{
			listvalidator.SizeBetween(1, 1000),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"allow_abbreviation": schema.BoolAttribute{
					Optional: true,
				},
				"category_fields": schema.ListNestedAttribute{
					Optional: true,
					Validators: []validator.List{
						listvalidator.SizeBetween(1, 1000),
					},
					NestedObject: schema.NestedAttributeObject{
						Attributes: ObservationFieldSchema(),
					},
				},
				"color_scheme": schema.StringAttribute{
					Optional: true,
				},
				"custom_unit": DynamicCustomUnitSchema(),
				"decimal_precision": schema.Int64Attribute{
					Optional: true,
					Validators: []validator.Int64{
						int64validator.Between(0, 15),
					},
				},
				"hash_colors": HashColorsSchema(),
				"query_id": schema.StringAttribute{
					Required:            true,
					MarkdownDescription: "The `id` of the query in `query_definitions` these settings style. Set that `id` explicitly, since a generated one is not known when the configuration is written.",
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
				"series_count_limit": schema.Int64Attribute{
					Optional: true,
				},
				"series_name_template": schema.StringAttribute{
					Optional: true,
				},
				"temporal_field": schema.SingleNestedAttribute{
					Attributes: ObservationFieldSchema(),
					Optional:   true,
				},
				"unit": UnitSchema(),
				"value_fields": schema.ListNestedAttribute{
					Optional: true,
					Validators: []validator.List{
						listvalidator.SizeBetween(1, 1000),
					},
					NestedObject: schema.NestedAttributeObject{
						Attributes: ObservationFieldSchema(),
					},
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
		},
	}
}

var dashboardValidBarValueDisplay = utils.GetKeys(dashboardSchemaToProtoBarValueDisplay)

func dynamicQueryDisplaySettingsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_scheme":         types.StringType,
		"custom_unit":          types.StringType,
		"decimal_precision":    types.Int64Type,
		"hash_colors":          types.BoolType,
		"query_id":             types.StringType,
		"scale_type":           types.StringType,
		"series_count_limit":   types.Int64Type,
		"series_name_template": types.StringType,
		"temporal_field":       ObservationFieldsObject(),
		"unit":                 types.StringType,
		"value_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"y_axis_max": Float32Type{},
		"y_axis_min": Float32Type{},
	}
}

var dashboardSchemaToProtoStackedLine = map[string]dashboardservice.VisualizationStackedLine{
	utils.UNSPECIFIED: dashboardservice.VISUALIZATIONSTACKEDLINE_STACKED_LINE_UNSPECIFIED,
	"absolute":        dashboardservice.VISUALIZATIONSTACKEDLINE_STACKED_LINE_ABSOLUTE,
	"relative":        dashboardservice.VISUALIZATIONSTACKEDLINE_STACKED_LINE_RELATIVE,
}

var dashboardSchemaToProtoXAxisTimeFormat = map[string]dashboardservice.XAxisTimeFormat{
	utils.UNSPECIFIED: dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_UNSPECIFIED,
	"auto":            dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_AUTO,
	"dd_mm":           dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_DD_MM,
	"mm_dd":           dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_MM_DD,
	"hh_mm":           dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_HH_MM,
	"dd_mm_hh_mm":     dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_DD_MM_HH_MM,
	"hh_mm_dd_mm":     dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_HH_MM_DD_MM,
	"mm_dd_hh_mm":     dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_MM_DD_HH_MM,
	"hh_mm_mm_dd":     dashboardservice.XAXISTIMEFORMAT_X_AXIS_TIME_FORMAT_HH_MM_MM_DD,
}

func expandFloat32Pointer(value Float32Value) *float32 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	converted := float32(value.ValueFloat64())
	return &converted
}

func expandDynamicQueryDisplaySettings(ctx context.Context, settings types.List) ([]dashboardservice.QueryDisplaySettings, diag.Diagnostics) {
	var models []DynamicQueryDisplaySettingsModel
	diags := settings.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.QueryDisplaySettings, 0, len(models))
	for i := range models {
		categoryFields, dg := ExpandObservationFields(ctx, models[i].CategoryFields)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		valueFields, dg := ExpandObservationFields(ctx, models[i].ValueFields)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		temporalField, dg := ExpandObservationFieldObject(ctx, models[i].TemporalField)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.QueryDisplaySettings{
			AllowAbbreviation:  models[i].AllowAbbreviation.ValueBoolPointer(),
			CategoryFields:     categoryFields,
			ColorScheme:        models[i].ColorScheme.ValueStringPointer(),
			CustomUnit:         models[i].CustomUnit.ValueStringPointer(),
			DecimalPrecision:   expandInt32Pointer(models[i].DecimalPrecision),
			HashColors:         models[i].HashColors.ValueBoolPointer(),
			QueryId:            models[i].QueryID.ValueString(),
			ScaleType:          OptionalEnumPointer(models[i].ScaleType, DashboardSchemaToProtoScaleType),
			SeriesCountLimit:   int64ToStringPointer(models[i].SeriesCountLimit),
			SeriesNameTemplate: models[i].SeriesNameTemplate.ValueStringPointer(),
			TemporalField:      temporalField,
			Unit:               OptionalEnumPointer(models[i].Unit, DashboardSchemaToProtoUnit),
			ValueFields:        valueFields,
			YAxisMax:           expandFloat32Pointer(models[i].YAxisMax),
			YAxisMin:           expandFloat32Pointer(models[i].YAxisMin),
		})
	}

	return expanded, diags
}

var dashboardSchemaToProtoBarValueDisplay = map[string]dashboardservice.VisualizationBarValueDisplay{
	utils.UNSPECIFIED: dashboardservice.VISUALIZATIONBARVALUEDISPLAY_BAR_VALUE_DISPLAY_UNSPECIFIED,
	"top":             dashboardservice.VISUALIZATIONBARVALUEDISPLAY_BAR_VALUE_DISPLAY_TOP,
	"inside":          dashboardservice.VISUALIZATIONBARVALUEDISPLAY_BAR_VALUE_DISPLAY_INSIDE,
	"both":            dashboardservice.VISUALIZATIONBARVALUEDISPLAY_BAR_VALUE_DISPLAY_BOTH,
}

var dashboardProtoToSchemaStackedLine = utils.ReverseMap(dashboardSchemaToProtoStackedLine)

var dashboardProtoToSchemaXAxisTimeFormat = utils.ReverseMap(dashboardSchemaToProtoXAxisTimeFormat)

func flattenFloat32Pointer(value *float32) Float32Value {
	if value == nil {
		return Float32Value{Float64Value: basetypes.NewFloat64Null()}
	}
	return Float32Value{Float64Value: basetypes.NewFloat64Value(float64(*value))}
}

func flattenDynamicQueryDisplaySettings(ctx context.Context, settings []dashboardservice.QueryDisplaySettings) (types.List, diag.Diagnostics) {
	if len(settings) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicQueryDisplaySettingsModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(settings))
	for i := range settings {
		categoryFields, dg := FlattenObservationFields(ctx, settings[i].GetCategoryFields())
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		valueFields, dg := FlattenObservationFields(ctx, settings[i].GetValueFields())
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		temporalField, dg := FlattenObservationField(ctx, settings[i].TemporalField)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		model := &DynamicQueryDisplaySettingsModel{
			AllowAbbreviation:  types.BoolPointerValue(settings[i].AllowAbbreviation),
			CategoryFields:     categoryFields,
			ColorScheme:        types.StringPointerValue(settings[i].ColorScheme),
			CustomUnit:         types.StringPointerValue(settings[i].CustomUnit),
			DecimalPrecision:   flattenInt32Pointer(settings[i].DecimalPrecision),
			HashColors:         types.BoolPointerValue(settings[i].HashColors),
			QueryID:            types.StringValue(settings[i].QueryId),
			ScaleType:          flattenOptionalEnum(settings[i].ScaleType, DashboardProtoToSchemaScaleType),
			SeriesCountLimit:   stringPointerToInt64(settings[i].SeriesCountLimit),
			SeriesNameTemplate: types.StringPointerValue(settings[i].SeriesNameTemplate),
			TemporalField:      temporalField,
			Unit:               flattenOptionalEnum(settings[i].Unit, DashboardProtoToSchemaUnit),
			ValueFields:        valueFields,
			YAxisMax:           flattenFloat32Pointer(settings[i].YAxisMax),
			YAxisMin:           flattenFloat32Pointer(settings[i].YAxisMin),
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicQueryDisplaySettingsModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicQueryDisplaySettingsModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicQueryDisplaySettingsModelAttr()}, elements)
}

var dashboardProtoToSchemaBarValueDisplay = utils.ReverseMap(dashboardSchemaToProtoBarValueDisplay)
