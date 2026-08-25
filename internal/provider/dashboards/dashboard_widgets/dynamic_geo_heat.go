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

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/float64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	dashboardSchemaToProtoColorGradientType = map[string]dashboardservice.ColorGradientType{
		utils.UNSPECIFIED:    dashboardservice.COLORGRADIENTTYPE_COLOR_GRADIENT_TYPE_UNSPECIFIED,
		"blue":               dashboardservice.COLORGRADIENTTYPE_COLOR_GRADIENT_TYPE_BLUE,
		"green":              dashboardservice.COLORGRADIENTTYPE_COLOR_GRADIENT_TYPE_GREEN,
		"red":                dashboardservice.COLORGRADIENTTYPE_COLOR_GRADIENT_TYPE_RED,
		"threshold":          dashboardservice.COLORGRADIENTTYPE_COLOR_GRADIENT_TYPE_THRESHOLD,
		"blue_reversed":      dashboardservice.COLORGRADIENTTYPE_COLOR_GRADIENT_TYPE_BLUE_REVERSED,
		"green_reversed":     dashboardservice.COLORGRADIENTTYPE_COLOR_GRADIENT_TYPE_GREEN_REVERSED,
		"red_reversed":       dashboardservice.COLORGRADIENTTYPE_COLOR_GRADIENT_TYPE_RED_REVERSED,
		"threshold_reversed": dashboardservice.COLORGRADIENTTYPE_COLOR_GRADIENT_TYPE_THRESHOLD_REVERSED,
	}
	dashboardProtoToSchemaColorGradientType = utils.ReverseMap(dashboardSchemaToProtoColorGradientType)
	dashboardValidColorGradientType         = utils.GetKeys(dashboardSchemaToProtoColorGradientType)

	dashboardSchemaToProtoHeatmapColorPreset = map[string]dashboardservice.HeatmapColorPreset{
		utils.UNSPECIFIED:    dashboardservice.HEATMAPCOLORPRESET_HEATMAP_COLOR_PRESET_UNSPECIFIED,
		"blue":               dashboardservice.HEATMAPCOLORPRESET_HEATMAP_COLOR_PRESET_BLUE,
		"green":              dashboardservice.HEATMAPCOLORPRESET_HEATMAP_COLOR_PRESET_GREEN,
		"red":                dashboardservice.HEATMAPCOLORPRESET_HEATMAP_COLOR_PRESET_RED,
		"threshold":          dashboardservice.HEATMAPCOLORPRESET_HEATMAP_COLOR_PRESET_THRESHOLD,
		"blue_reversed":      dashboardservice.HEATMAPCOLORPRESET_HEATMAP_COLOR_PRESET_BLUE_REVERSED,
		"green_reversed":     dashboardservice.HEATMAPCOLORPRESET_HEATMAP_COLOR_PRESET_GREEN_REVERSED,
		"red_reversed":       dashboardservice.HEATMAPCOLORPRESET_HEATMAP_COLOR_PRESET_RED_REVERSED,
		"threshold_reversed": dashboardservice.HEATMAPCOLORPRESET_HEATMAP_COLOR_PRESET_THRESHOLD_REVERSED,
	}
	dashboardProtoToSchemaHeatmapColorPreset = utils.ReverseMap(dashboardSchemaToProtoHeatmapColorPreset)
	dashboardValidHeatmapColorPreset         = utils.GetKeys(dashboardSchemaToProtoHeatmapColorPreset)

	dashboardSchemaToProtoHeatmapHistogramBucketUnit = map[string]dashboardservice.HeatmapHistogramBucketUnit{
		utils.UNSPECIFIED: dashboardservice.HEATMAPHISTOGRAMBUCKETUNIT_HEATMAP_HISTOGRAM_BUCKET_UNIT_UNSPECIFIED,
		"nanoseconds":     dashboardservice.HEATMAPHISTOGRAMBUCKETUNIT_HEATMAP_HISTOGRAM_BUCKET_UNIT_NANOSECONDS,
		"microseconds":    dashboardservice.HEATMAPHISTOGRAMBUCKETUNIT_HEATMAP_HISTOGRAM_BUCKET_UNIT_MICROSECONDS,
		"milliseconds":    dashboardservice.HEATMAPHISTOGRAMBUCKETUNIT_HEATMAP_HISTOGRAM_BUCKET_UNIT_MILLISECONDS,
		"seconds":         dashboardservice.HEATMAPHISTOGRAMBUCKETUNIT_HEATMAP_HISTOGRAM_BUCKET_UNIT_SECONDS,
		"bytes_iec":       dashboardservice.HEATMAPHISTOGRAMBUCKETUNIT_HEATMAP_HISTOGRAM_BUCKET_UNIT_BYTES_IEC,
		"kibytes":         dashboardservice.HEATMAPHISTOGRAMBUCKETUNIT_HEATMAP_HISTOGRAM_BUCKET_UNIT_KIBYTES,
		"mibytes":         dashboardservice.HEATMAPHISTOGRAMBUCKETUNIT_HEATMAP_HISTOGRAM_BUCKET_UNIT_MIBYTES,
		"gibytes":         dashboardservice.HEATMAPHISTOGRAMBUCKETUNIT_HEATMAP_HISTOGRAM_BUCKET_UNIT_GIBYTES,
		"bytes":           dashboardservice.HEATMAPHISTOGRAMBUCKETUNIT_HEATMAP_HISTOGRAM_BUCKET_UNIT_BYTES,
		"kbytes":          dashboardservice.HEATMAPHISTOGRAMBUCKETUNIT_HEATMAP_HISTOGRAM_BUCKET_UNIT_KBYTES,
		"mbytes":          dashboardservice.HEATMAPHISTOGRAMBUCKETUNIT_HEATMAP_HISTOGRAM_BUCKET_UNIT_MBYTES,
		"gbytes":          dashboardservice.HEATMAPHISTOGRAMBUCKETUNIT_HEATMAP_HISTOGRAM_BUCKET_UNIT_GBYTES,
	}
	dashboardProtoToSchemaHeatmapHistogramBucketUnit = utils.ReverseMap(dashboardSchemaToProtoHeatmapHistogramBucketUnit)
	dashboardValidHeatmapHistogramBucketUnit         = utils.GetKeys(dashboardSchemaToProtoHeatmapHistogramBucketUnit)
)

// Schemas --------------------------------------------------------------------

func dynamicHexagonBinsSchema() schema.Attribute {
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
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 15),
				},
				MarkdownDescription: "How many digits to show after the decimal point. Valid values are 0 to 15.",
			},
			"legend": LegendSchema(),
			"legend_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidLegendBys...),
				},
				MarkdownDescription: fmt.Sprintf("How the legend is grouped. Valid values are: %s.", strings.Join(DashboardValidLegendBys, ", ")),
			},
			"max": schema.Float64Attribute{
				Optional: true,
			},
			"min": schema.Float64Attribute{
				Optional: true,
			},
			"threshold_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidThresholdTypes...),
				},
				MarkdownDescription: fmt.Sprintf("The threshold type. Valid values are: %s.", strings.Join(DashboardValidThresholdTypes, ", ")),
			},
			"thresholds": dynamicThresholdsSchema(),
			"unit":       UnitSchema(),
			"value_field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
		},
	}
}

func dynamicHeatmapSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"allow_abbreviation": schema.BoolAttribute{
				Optional: true,
			},
			"color_axis_max": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The maximum value for the gradient color axis. Stored at float32 precision by the API.",
			},
			"color_axis_min": schema.Float64Attribute{
				Optional:   true,
				CustomType: Float32Type{},
				Validators: []validator.Float64{
					float64validator.Between(-math.MaxFloat32, math.MaxFloat32),
				},
				MarkdownDescription: "The minimum value for the gradient color axis. Stored at float32 precision by the API.",
			},
			"color_range": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidColorGradientType...),
					stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("preset")),
				},
				MarkdownDescription: fmt.Sprintf("The gradient color range. Mutually exclusive with `preset`. Valid values are: %s.", strings.Join(dashboardValidColorGradientType, ", ")),
			},
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 15),
				},
				MarkdownDescription: "How many digits to show after the decimal point. Valid values are 0 to 15.",
			},
			"histogram_bucket_unit": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidHeatmapHistogramBucketUnit...),
				},
				MarkdownDescription: fmt.Sprintf("The histogram bucket unit. Valid values are: %s.", strings.Join(dashboardValidHeatmapHistogramBucketUnit, ", ")),
			},
			"preset": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidHeatmapColorPreset...),
				},
				MarkdownDescription: fmt.Sprintf("The color preset. Mutually exclusive with `color_range`. Valid values are: %s.", strings.Join(dashboardValidHeatmapColorPreset, ", ")),
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
			"show_numbers": schema.BoolAttribute{
				Optional: true,
			},
			"tooltip": dynamicHeatmapTooltipSchema(),
			"unit":    UnitSchema(),
			"value_field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"x_axis_fields": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
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
			"y_axis_fields": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
		},
	}
}

func dynamicHeatmapTooltipSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"labels": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: ObservationFieldSchema(),
				},
			},
			"message_template": schema.StringAttribute{
				Optional: true,
			},
		},
	}
}

func dynamicGeomapSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"aggregation": dynamicGeomapAggregationSchema(),
			"allow_abbreviation": schema.BoolAttribute{
				Optional: true,
			},
			"color": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"color_range": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.OneOf(dashboardValidColorGradientType...),
						},
						MarkdownDescription: fmt.Sprintf("The gradient color range. Valid values are: %s.", strings.Join(dashboardValidColorGradientType, ", ")),
					},
					"size": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.OneOf(dashboardValidColorSolidType...),
						},
						MarkdownDescription: fmt.Sprintf("The solid size color. Valid values are: %s.", strings.Join(dashboardValidColorSolidType, ", ")),
					},
				},
				Validators: []validator.Object{
					ExactlyOneOfChildren("color_range", "size"),
				},
			},
			"config": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"aws_region_config": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"aws_region_field": schema.SingleNestedAttribute{
								Attributes: ObservationFieldSchema(),
								Optional:   true,
							},
						},
					},
					"coordinate_config": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"latitude_field": schema.SingleNestedAttribute{
								Attributes: ObservationFieldSchema(),
								Optional:   true,
							},
							"longitude_field": schema.SingleNestedAttribute{
								Attributes: ObservationFieldSchema(),
								Optional:   true,
							},
						},
					},
				},
				Validators: []validator.Object{
					ExactlyOneOfChildren("aws_region_config", "coordinate_config"),
				},
			},
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 15),
				},
				MarkdownDescription: "How many digits to show after the decimal point. Valid values are 0 to 15.",
			},
			"min_max": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"auto": schema.BoolAttribute{
						Optional: true,
						Validators: []validator.Bool{
							mustBeTrueValidator{},
						},
					},
					"custom": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"max": schema.Float64Attribute{
								Optional: true,
							},
							"min": schema.Float64Attribute{
								Optional: true,
							},
						},
					},
				},
				Validators: []validator.Object{
					ExactlyOneOfChildren("auto", "custom"),
				},
			},
			"tooltip": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"labels": schema.ListNestedAttribute{
						Optional: true,
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
						},
						NestedObject: schema.NestedAttributeObject{
							Attributes: ObservationFieldSchema(),
						},
					},
					"message_template": schema.StringAttribute{
						Optional: true,
					},
				},
			},
			"unit": UnitSchema(),
		},
	}
}

func dynamicGeomapAggregationSchema() schema.Attribute {
	fieldBased := func() schema.Attribute {
		return schema.SingleNestedAttribute{
			Optional: true,
			Attributes: map[string]schema.Attribute{
				"field": schema.SingleNestedAttribute{
					Attributes: ObservationFieldSchema(),
					Optional:   true,
				},
			},
		}
	}
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"avg": fieldBased(),
			"count": schema.BoolAttribute{
				Optional: true,
				Validators: []validator.Bool{
					mustBeTrueValidator{},
				},
			},
			"max": fieldBased(),
			"min": fieldBased(),
			"sum": fieldBased(),
		},
		Validators: []validator.Object{
			ExactlyOneOfChildren("avg", "count", "max", "min", "sum"),
		},
	}
}

// Model attribute maps -------------------------------------------------------

func dynamicHexagonBinsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"custom_unit":       types.StringType,
		"decimal_precision": types.Int64Type,
		"legend":            types.ObjectType{AttrTypes: LegendAttr()},
		"legend_by":         types.StringType,
		"max":               types.Float64Type,
		"min":               types.Float64Type,
		"threshold_type":    types.StringType,
		"thresholds": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicThresholdAttr()},
		},
		"unit":        types.StringType,
		"value_field": ObservationFieldsObject(),
	}
}

func dynamicHeatmapModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation":    types.BoolType,
		"color_axis_max":        Float32Type{},
		"color_axis_min":        Float32Type{},
		"color_range":           types.StringType,
		"custom_unit":           types.StringType,
		"decimal_precision":     types.Int64Type,
		"histogram_bucket_unit": types.StringType,
		"preset":                types.StringType,
		"scale_type":            types.StringType,
		"show_numbers":          types.BoolType,
		"tooltip":               types.ObjectType{AttrTypes: dynamicHeatmapTooltipModelAttr()},
		"unit":                  types.StringType,
		"value_field":           ObservationFieldsObject(),
		"x_axis_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"x_axis_time_format": types.StringType,
		"y_axis_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
	}
}

func dynamicHeatmapTooltipModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"labels": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"message_template": types.StringType,
	}
}

func dynamicGeomapModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"aggregation":        types.ObjectType{AttrTypes: dynamicGeomapAggregationModelAttr()},
		"allow_abbreviation": types.BoolType,
		"color":              types.ObjectType{AttrTypes: dynamicGeomapColorModelAttr()},
		"config":             types.ObjectType{AttrTypes: dynamicGeomapFieldConfigModelAttr()},
		"custom_unit":        types.StringType,
		"decimal_precision":  types.Int64Type,
		"min_max":            types.ObjectType{AttrTypes: dynamicMinMaxAttr()},
		"tooltip":            types.ObjectType{AttrTypes: dynamicGeomapTooltipModelAttr()},
		"unit":               types.StringType,
	}
}

func dynamicGeomapAggregationModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"avg":   types.ObjectType{AttrTypes: dynamicGeomapAggregationFieldBasedModelAttr()},
		"count": types.BoolType,
		"max":   types.ObjectType{AttrTypes: dynamicGeomapAggregationFieldBasedModelAttr()},
		"min":   types.ObjectType{AttrTypes: dynamicGeomapAggregationFieldBasedModelAttr()},
		"sum":   types.ObjectType{AttrTypes: dynamicGeomapAggregationFieldBasedModelAttr()},
	}
}

func dynamicGeomapAggregationFieldBasedModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": ObservationFieldsObject(),
	}
}

func dynamicGeomapColorModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"color_range": types.StringType,
		"size":        types.StringType,
	}
}

func dynamicGeomapFieldConfigModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"aws_region_config": types.ObjectType{AttrTypes: dynamicGeomapAwsRegionConfigModelAttr()},
		"coordinate_config": types.ObjectType{AttrTypes: dynamicGeomapCoordinateConfigModelAttr()},
	}
}

func dynamicGeomapAwsRegionConfigModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"aws_region_field": ObservationFieldsObject(),
	}
}

func dynamicGeomapCoordinateConfigModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"latitude_field":  ObservationFieldsObject(),
		"longitude_field": ObservationFieldsObject(),
	}
}

func dynamicGeomapTooltipModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"labels": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"message_template": types.StringType,
	}
}

// Expand ---------------------------------------------------------------------

func expandDynamicHexagonBins(ctx context.Context, m *DynamicHexagonBinsModel) (*dashboardservice.HexagonBins, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, m.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}
	valueField, diags := ExpandObservationFieldObject(ctx, m.ValueField)
	if diags.HasError() {
		return nil, diags
	}
	thresholds, diags := expandDynamicThresholds(ctx, m.Thresholds)
	if diags.HasError() {
		return nil, diags
	}
	legend, diags := ExpandLegend(ctx, m.Legend)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.HexagonBins{
		AllowAbbreviation: m.AllowAbbreviation.ValueBoolPointer(),
		CategoryFields:    categoryFields,
		CustomUnit:        m.CustomUnit.ValueStringPointer(),
		DecimalPrecision:  expandInt32Pointer(m.DecimalPrecision),
		Legend:            legend,
		LegendBy:          OptionalEnumPointer(m.LegendBy, DashboardSchemaToProtoLegendBy),
		Max:               m.Max.ValueFloat64Pointer(),
		Min:               m.Min.ValueFloat64Pointer(),
		ThresholdType:     OptionalEnumPointer(m.ThresholdType, DashboardSchemaToProtoThresholdType),
		Thresholds:        thresholds,
		Unit:              OptionalEnumPointer(m.Unit, DashboardSchemaToProtoUnit),
		ValueField:        valueField,
	}, nil
}

func expandDynamicHeatmap(ctx context.Context, m *DynamicHeatmapModel) (*dashboardservice.Heatmap, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}

	valueField, diags := ExpandObservationFieldObject(ctx, m.ValueField)
	if diags.HasError() {
		return nil, diags
	}
	xAxisFields, diags := ExpandObservationFields(ctx, m.XAxisFields)
	if diags.HasError() {
		return nil, diags
	}
	yAxisFields, diags := ExpandObservationFields(ctx, m.YAxisFields)
	if diags.HasError() {
		return nil, diags
	}
	tooltip, diags := expandDynamicHeatmapTooltip(ctx, m.Tooltip)
	if diags.HasError() {
		return nil, diags
	}

	// Unlike the other unions here, neither arm set is legitimate: a heatmap
	// with no colour configuration is valid. Only both at once is impossible.
	colorRange := OptionalEnumPointer(m.ColorRange, dashboardSchemaToProtoColorGradientType)
	preset := OptionalEnumPointer(m.Preset, dashboardSchemaToProtoHeatmapColorPreset)
	if colorRange != nil && preset != nil {
		return nil, dynamicUnionDiagnostic("the heatmap colour", "`preset` or `color_range`, or neither")
	}

	return &dashboardservice.Heatmap{
		AllowAbbreviation:   m.AllowAbbreviation.ValueBoolPointer(),
		ColorAxisMax:        expandFloat32Pointer(m.ColorAxisMax),
		ColorAxisMin:        expandFloat32Pointer(m.ColorAxisMin),
		ColorRange:          colorRange,
		CustomUnit:          m.CustomUnit.ValueStringPointer(),
		DecimalPrecision:    expandInt32Pointer(m.DecimalPrecision),
		HistogramBucketUnit: OptionalEnumPointer(m.HistogramBucketUnit, dashboardSchemaToProtoHeatmapHistogramBucketUnit),
		Preset:              preset,
		ScaleType:           OptionalEnumPointer(m.ScaleType, DashboardSchemaToProtoScaleType),
		ShowNumbers:         m.ShowNumbers.ValueBoolPointer(),
		Tooltip:             tooltip,
		Unit:                OptionalEnumPointer(m.Unit, DashboardSchemaToProtoUnit),
		ValueField:          valueField,
		XAxisFields:         xAxisFields,
		XAxisTimeFormat:     OptionalEnumPointer(m.XAxisTimeFormat, dashboardSchemaToProtoXAxisTimeFormat),
		YAxisFields:         yAxisFields,
	}, nil
}

func expandDynamicHeatmapTooltip(ctx context.Context, m *DynamicHeatmapTooltipModel) (*dashboardservice.HeatmapTooltip, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}
	labels, diags := ExpandObservationFields(ctx, m.Labels)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.HeatmapTooltip{
		Labels:          labels,
		MessageTemplate: m.MessageTemplate.ValueStringPointer(),
	}, nil
}

func expandDynamicGeomap(ctx context.Context, m *DynamicGeomapModel) (*dashboardservice.Geomap, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}

	aggregation, diags := expandDynamicGeomapAggregation(ctx, m.Aggregation)
	if diags.HasError() {
		return nil, diags
	}
	config, diags := expandDynamicGeomapFieldConfig(ctx, m.Config)
	if diags.HasError() {
		return nil, diags
	}
	tooltip, diags := expandDynamicGeomapTooltip(ctx, m.Tooltip)
	if diags.HasError() {
		return nil, diags
	}

	minMax, diags := expandDynamicMinMax(m.MinMax)
	if diags.HasError() {
		return nil, diags
	}

	color, diags := expandDynamicGeomapColor(m.Color)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.Geomap{
		Aggregation:       aggregation,
		AllowAbbreviation: m.AllowAbbreviation.ValueBoolPointer(),
		Color:             color,
		Config:            config,
		CustomUnit:        m.CustomUnit.ValueStringPointer(),
		DecimalPrecision:  expandInt32Pointer(m.DecimalPrecision),
		MinMax:            minMax,
		Tooltip:           tooltip,
		Unit:              OptionalEnumPointer(m.Unit, DashboardSchemaToProtoUnit),
	}, nil
}

func expandDynamicGeomapAggregation(ctx context.Context, m *DynamicGeomapAggregationModel) (*dashboardservice.GeomapAggregation, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}

	avg, diags := expandDynamicGeomapAggregationFieldBased(ctx, m.Avg)
	if diags.HasError() {
		return nil, diags
	}
	maxAgg, diags := expandDynamicGeomapAggregationFieldBased(ctx, m.Max)
	if diags.HasError() {
		return nil, diags
	}
	minAgg, diags := expandDynamicGeomapAggregationFieldBased(ctx, m.Min)
	if diags.HasError() {
		return nil, diags
	}
	sum, diags := expandDynamicGeomapAggregationFieldBased(ctx, m.Sum)
	if diags.HasError() {
		return nil, diags
	}

	aggregation := &dashboardservice.GeomapAggregation{
		Avg: avg,
		Max: maxAgg,
		Min: minAgg,
		Sum: sum,
	}
	if m.Count.ValueBool() {
		aggregation.Count = map[string]interface{}{}
	}

	set := 0
	for _, selected := range []bool{aggregation.Count != nil, avg != nil, maxAgg != nil, minAgg != nil, sum != nil} {
		if selected {
			set++
		}
	}
	if set != 1 {
		return nil, dynamicUnionDiagnostic("aggregation", "`count` set to true, `sum`, `min`, `max` or `avg`")
	}

	return aggregation, nil
}

func expandDynamicGeomapAggregationFieldBased(ctx context.Context, m *DynamicGeomapAggregationFieldBasedModel) (*dashboardservice.GeomapAggregationFieldBased, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}
	field, diags := ExpandObservationFieldObject(ctx, m.Field)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.GeomapAggregationFieldBased{Field: field}, nil
}

func expandDynamicGeomapColor(m *DynamicGeomapColorModel) (*dashboardservice.GeomapColor, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}

	colorRange := OptionalEnumPointer(m.ColorRange, dashboardSchemaToProtoColorGradientType)
	size := OptionalEnumPointer(m.Size, dashboardSchemaToProtoColorSolidType)
	if (colorRange == nil) == (size == nil) {
		return nil, dynamicUnionDiagnostic("color", "`size` or `color_range`")
	}

	return &dashboardservice.GeomapColor{ColorRange: colorRange, Size: size}, nil
}

func expandDynamicGeomapFieldConfig(ctx context.Context, m *DynamicGeomapFieldConfigModel) (*dashboardservice.GeomapFieldConfig, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}

	awsRegionConfig, diags := expandDynamicGeomapAwsRegionConfig(ctx, m.AwsRegionConfig)
	if diags.HasError() {
		return nil, diags
	}
	coordinateConfig, diags := expandDynamicGeomapCoordinateConfig(ctx, m.CoordinateConfig)
	if diags.HasError() {
		return nil, diags
	}

	if (awsRegionConfig == nil) == (coordinateConfig == nil) {
		return nil, dynamicUnionDiagnostic("config", "`coordinate_config` or `aws_region_config`")
	}

	return &dashboardservice.GeomapFieldConfig{
		AwsRegionConfig:  awsRegionConfig,
		CoordinateConfig: coordinateConfig,
	}, nil
}

func expandDynamicGeomapAwsRegionConfig(ctx context.Context, m *DynamicGeomapAwsRegionConfigModel) (*dashboardservice.GeomapAwsRegionConfig, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}
	field, diags := ExpandObservationFieldObject(ctx, m.AwsRegionField)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.GeomapAwsRegionConfig{AwsRegionField: field}, nil
}

func expandDynamicGeomapCoordinateConfig(ctx context.Context, m *DynamicGeomapCoordinateConfigModel) (*dashboardservice.GeomapCoordinateConfig, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}
	latitudeField, diags := ExpandObservationFieldObject(ctx, m.LatitudeField)
	if diags.HasError() {
		return nil, diags
	}
	longitudeField, diags := ExpandObservationFieldObject(ctx, m.LongitudeField)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.GeomapCoordinateConfig{
		LatitudeField:  latitudeField,
		LongitudeField: longitudeField,
	}, nil
}

func expandDynamicGeomapTooltip(ctx context.Context, m *DynamicGeomapTooltipModel) (*dashboardservice.GeomapTooltip, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}
	labels, diags := ExpandObservationFields(ctx, m.Labels)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.GeomapTooltip{
		Labels:          labels,
		MessageTemplate: m.MessageTemplate.ValueStringPointer(),
	}, nil
}

// The API has no representation for a min/max with neither arm chosen, so
// sending an empty one comes back as an unset block and fails the apply. The
// object validator cannot catch this when `auto` is only known after apply, so
// the conversion has to refuse it.
func expandDynamicMinMax(m *DynamicMinMaxModel) (*dashboardservice.MinMax, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}

	switch {
	case m.Custom != nil && m.Auto.ValueBool():
		return nil, dynamicUnionDiagnostic("min_max", "`auto` set to true or `custom`, not both")
	case m.Custom != nil:
		return &dashboardservice.MinMax{Custom: &dashboardservice.MinMaxCustom{
			Max: m.Custom.Max.ValueFloat64Pointer(),
			Min: m.Custom.Min.ValueFloat64Pointer(),
		}}, nil
	case m.Auto.ValueBool():
		return &dashboardservice.MinMax{Auto: map[string]interface{}{}}, nil
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
			"Invalid Attribute Combination",
			"min_max requires exactly one of `auto` or `custom`, and `auto` must be true. Remove the min_max block to let the widget scale itself.",
		)}
	}
}

// Flatten --------------------------------------------------------------------

func flattenDynamicHexagonBins(ctx context.Context, m *dashboardservice.HexagonBins) (*DynamicHexagonBinsModel, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, m.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}
	valueField, diags := FlattenObservationField(ctx, m.ValueField)
	if diags.HasError() {
		return nil, diags
	}
	thresholds, diags := flattenDynamicThresholds(ctx, m.GetThresholds())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicHexagonBinsModel{
		AllowAbbreviation: types.BoolPointerValue(m.AllowAbbreviation),
		CategoryFields:    categoryFields,
		CustomUnit:        types.StringPointerValue(m.CustomUnit),
		DecimalPrecision:  flattenInt32Pointer(m.DecimalPrecision),
		Legend:            FlattenLegend(m.Legend),
		LegendBy:          flattenOptionalEnum(m.LegendBy, DashboardProtoToSchemaLegendBy),
		Max:               types.Float64PointerValue(m.Max),
		Min:               types.Float64PointerValue(m.Min),
		ThresholdType:     flattenOptionalEnum(m.ThresholdType, DashboardProtoToSchemaThresholdType),
		Thresholds:        thresholds,
		Unit:              flattenOptionalEnum(m.Unit, DashboardProtoToSchemaUnit),
		ValueField:        valueField,
	}, nil
}

func flattenDynamicHeatmap(ctx context.Context, m *dashboardservice.Heatmap) (*DynamicHeatmapModel, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}

	valueField, diags := FlattenObservationField(ctx, m.ValueField)
	if diags.HasError() {
		return nil, diags
	}
	xAxisFields, diags := FlattenObservationFields(ctx, m.GetXAxisFields())
	if diags.HasError() {
		return nil, diags
	}
	yAxisFields, diags := FlattenObservationFields(ctx, m.GetYAxisFields())
	if diags.HasError() {
		return nil, diags
	}
	tooltip, diags := flattenDynamicHeatmapTooltip(ctx, m.Tooltip)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicHeatmapModel{
		AllowAbbreviation:   types.BoolPointerValue(m.AllowAbbreviation),
		ColorAxisMax:        flattenFloat32Pointer(m.ColorAxisMax),
		ColorAxisMin:        flattenFloat32Pointer(m.ColorAxisMin),
		ColorRange:          flattenOptionalEnum(m.ColorRange, dashboardProtoToSchemaColorGradientType),
		CustomUnit:          types.StringPointerValue(m.CustomUnit),
		DecimalPrecision:    flattenInt32Pointer(m.DecimalPrecision),
		HistogramBucketUnit: flattenOptionalEnum(m.HistogramBucketUnit, dashboardProtoToSchemaHeatmapHistogramBucketUnit),
		Preset:              flattenOptionalEnum(m.Preset, dashboardProtoToSchemaHeatmapColorPreset),
		ScaleType:           flattenOptionalEnum(m.ScaleType, DashboardProtoToSchemaScaleType),
		ShowNumbers:         types.BoolPointerValue(m.ShowNumbers),
		Tooltip:             tooltip,
		Unit:                flattenOptionalEnum(m.Unit, DashboardProtoToSchemaUnit),
		ValueField:          valueField,
		XAxisFields:         xAxisFields,
		XAxisTimeFormat:     flattenOptionalEnum(m.XAxisTimeFormat, dashboardProtoToSchemaXAxisTimeFormat),
		YAxisFields:         yAxisFields,
	}, nil
}

func flattenDynamicHeatmapTooltip(ctx context.Context, m *dashboardservice.HeatmapTooltip) (*DynamicHeatmapTooltipModel, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}
	labels, diags := FlattenObservationFields(ctx, m.GetLabels())
	if diags.HasError() {
		return nil, diags
	}
	return &DynamicHeatmapTooltipModel{
		Labels:          labels,
		MessageTemplate: types.StringPointerValue(m.MessageTemplate),
	}, nil
}

func flattenDynamicGeomap(ctx context.Context, m *dashboardservice.Geomap) (*DynamicGeomapModel, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}

	aggregation, diags := flattenDynamicGeomapAggregation(ctx, m.Aggregation)
	if diags.HasError() {
		return nil, diags
	}
	config, diags := flattenDynamicGeomapFieldConfig(ctx, m.Config)
	if diags.HasError() {
		return nil, diags
	}
	tooltip, diags := flattenDynamicGeomapTooltip(ctx, m.Tooltip)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicGeomapModel{
		Aggregation:       aggregation,
		AllowAbbreviation: types.BoolPointerValue(m.AllowAbbreviation),
		Color:             flattenDynamicGeomapColor(m.Color),
		Config:            config,
		CustomUnit:        types.StringPointerValue(m.CustomUnit),
		DecimalPrecision:  flattenInt32Pointer(m.DecimalPrecision),
		MinMax:            flattenDynamicMinMax(m.MinMax),
		Tooltip:           tooltip,
		Unit:              flattenOptionalEnum(m.Unit, DashboardProtoToSchemaUnit),
	}, nil
}

func flattenDynamicGeomapAggregation(ctx context.Context, m *dashboardservice.GeomapAggregation) (*DynamicGeomapAggregationModel, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}

	avg, diags := flattenDynamicGeomapAggregationFieldBased(ctx, m.Avg)
	if diags.HasError() {
		return nil, diags
	}
	maxAgg, diags := flattenDynamicGeomapAggregationFieldBased(ctx, m.Max)
	if diags.HasError() {
		return nil, diags
	}
	minAgg, diags := flattenDynamicGeomapAggregationFieldBased(ctx, m.Min)
	if diags.HasError() {
		return nil, diags
	}
	sum, diags := flattenDynamicGeomapAggregationFieldBased(ctx, m.Sum)
	if diags.HasError() {
		return nil, diags
	}

	count := types.BoolNull()
	if m.Count != nil {
		count = types.BoolValue(true)
	}

	return &DynamicGeomapAggregationModel{
		Avg:   avg,
		Count: count,
		Max:   maxAgg,
		Min:   minAgg,
		Sum:   sum,
	}, nil
}

func flattenDynamicGeomapAggregationFieldBased(ctx context.Context, m *dashboardservice.GeomapAggregationFieldBased) (*DynamicGeomapAggregationFieldBasedModel, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}
	field, diags := FlattenObservationField(ctx, m.Field)
	if diags.HasError() {
		return nil, diags
	}
	return &DynamicGeomapAggregationFieldBasedModel{Field: field}, nil
}

func flattenDynamicGeomapColor(m *dashboardservice.GeomapColor) *DynamicGeomapColorModel {
	if m == nil {
		return nil
	}
	return &DynamicGeomapColorModel{
		ColorRange: flattenOptionalEnum(m.ColorRange, dashboardProtoToSchemaColorGradientType),
		Size:       flattenOptionalEnum(m.Size, dashboardProtoToSchemaColorSolidType),
	}
}

func flattenDynamicGeomapFieldConfig(ctx context.Context, m *dashboardservice.GeomapFieldConfig) (*DynamicGeomapFieldConfigModel, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}

	awsRegionConfig, diags := flattenDynamicGeomapAwsRegionConfig(ctx, m.AwsRegionConfig)
	if diags.HasError() {
		return nil, diags
	}
	coordinateConfig, diags := flattenDynamicGeomapCoordinateConfig(ctx, m.CoordinateConfig)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicGeomapFieldConfigModel{
		AwsRegionConfig:  awsRegionConfig,
		CoordinateConfig: coordinateConfig,
	}, nil
}

func flattenDynamicGeomapAwsRegionConfig(ctx context.Context, m *dashboardservice.GeomapAwsRegionConfig) (*DynamicGeomapAwsRegionConfigModel, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}
	field, diags := FlattenObservationField(ctx, m.AwsRegionField)
	if diags.HasError() {
		return nil, diags
	}
	return &DynamicGeomapAwsRegionConfigModel{AwsRegionField: field}, nil
}

func flattenDynamicGeomapCoordinateConfig(ctx context.Context, m *dashboardservice.GeomapCoordinateConfig) (*DynamicGeomapCoordinateConfigModel, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}
	latitudeField, diags := FlattenObservationField(ctx, m.LatitudeField)
	if diags.HasError() {
		return nil, diags
	}
	longitudeField, diags := FlattenObservationField(ctx, m.LongitudeField)
	if diags.HasError() {
		return nil, diags
	}
	return &DynamicGeomapCoordinateConfigModel{
		LatitudeField:  latitudeField,
		LongitudeField: longitudeField,
	}, nil
}

func flattenDynamicGeomapTooltip(ctx context.Context, m *dashboardservice.GeomapTooltip) (*DynamicGeomapTooltipModel, diag.Diagnostics) {
	if m == nil {
		return nil, nil
	}
	labels, diags := FlattenObservationFields(ctx, m.GetLabels())
	if diags.HasError() {
		return nil, diags
	}
	return &DynamicGeomapTooltipModel{
		Labels:          labels,
		MessageTemplate: types.StringPointerValue(m.MessageTemplate),
	}, nil
}

// The API stores a min/max wrapper with neither arm selected - confirmed by
// applying `"minMax": {}` and reading it back. Returning a present block for
// that shape would produce state with both children null, which the block's
// exactly-one-of validator rejects, so it would diff forever after an import.
func flattenDynamicMinMax(m *dashboardservice.MinMax) *DynamicMinMaxModel {
	switch {
	case m == nil:
		return nil
	case m.Custom != nil:
		return &DynamicMinMaxModel{Auto: types.BoolNull(), Custom: &DynamicMinMaxCustomModel{
			Max: types.Float64PointerValue(m.Custom.Max),
			Min: types.Float64PointerValue(m.Custom.Min),
		}}
	case m.Auto != nil:
		return &DynamicMinMaxModel{Auto: types.BoolValue(true)}
	default:
		return nil
	}
}
