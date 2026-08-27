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

// The dynamic widget's pie chart uses its own label-source enum, distinct from
// the legacy pie-chart widget's, so the shared exported map cannot be reused.
var (
	dashboardSchemaToProtoPieChartLabelSource = map[string]dashboardservice.VisualizationPieChartLabelSource{
		utils.UNSPECIFIED: dashboardservice.VISUALIZATIONPIECHARTLABELSOURCE_LABEL_SOURCE_UNSPECIFIED,
		"inner":           dashboardservice.VISUALIZATIONPIECHARTLABELSOURCE_LABEL_SOURCE_INNER,
		"stack":           dashboardservice.VISUALIZATIONPIECHARTLABELSOURCE_LABEL_SOURCE_STACK,
	}
	dashboardProtoToSchemaPieChartLabelSource = utils.ReverseMap(dashboardSchemaToProtoPieChartLabelSource)
	dashboardValidPieChartLabelSource         = utils.GetKeys(dashboardSchemaToProtoPieChartLabelSource)
)

func dynamicGaugeSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"allow_abbreviation": schema.BoolAttribute{
				Optional: true,
			},
			"arc_display": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"threshold_arc": schema.BoolAttribute{
						Optional: true,
					},
					"value_arc": schema.BoolAttribute{
						Optional: true,
					},
				},
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
			"decimal_precision": DynamicDecimalPrecisionSchema(),
			"display_series_name": schema.BoolAttribute{
				Optional: true,
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
			"show_inner_arc": schema.BoolAttribute{
				Optional:            true,
				DeprecationMessage:  "Use `arc_display.value_arc` instead.",
				MarkdownDescription: "Deprecated: use `arc_display.value_arc` instead.",
			},
			"show_min_max": schema.BoolAttribute{
				Optional: true,
			},
			"show_outer_arc": schema.BoolAttribute{
				Optional:            true,
				DeprecationMessage:  "Use `arc_display.threshold_arc` instead.",
				MarkdownDescription: "Deprecated: use `arc_display.threshold_arc` instead.",
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
			"value_fields": schema.ListNestedAttribute{
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

func dynamicPieChartSchema() schema.Attribute {
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
				Optional:            true,
				MarkdownDescription: fmt.Sprintf("How slices are coloured. The API takes a free-form string and stores whatever it is given, so this is deliberately not restricted; the schemes this provider knows about are: %s. A value the product does not recognise is stored but applies no scheme.", strings.Join(DashboardValidColorSchemes, ", ")),
			},
			"custom_unit": schema.StringAttribute{
				Optional: true,
			},
			"decimal_precision": DynamicDecimalPrecisionSchema(),
			"group_name_template": schema.StringAttribute{
				Optional: true,
			},
			"hash_colors": HashColorsSchema(),
			"label_definition": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"is_visible": schema.BoolAttribute{
						Optional: true,
					},
					"label_source": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString(utils.UNSPECIFIED),
						Validators: []validator.String{
							stringvalidator.OneOf(dashboardValidPieChartLabelSource...),
						},
						MarkdownDescription: fmt.Sprintf("The label source. Valid values are: %s.", strings.Join(dashboardValidPieChartLabelSource, ", ")),
					},
					"show_name": schema.BoolAttribute{
						Optional: true,
					},
					"show_percentage": schema.BoolAttribute{
						Optional: true,
					},
					"show_value": schema.BoolAttribute{
						Optional: true,
					},
				},
			},
			"legend": LegendSchema(),
			"max_slices_per_chart": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(1, math.MaxInt32),
				},
				MarkdownDescription: "The most slices to draw on the chart. Must be at least 1.",
			},
			"max_slices_per_stack": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(1, math.MaxInt32),
				},
				MarkdownDescription: "The most slices to fit in one stack. Must be at least 1.",
			},
			"min_slice_percentage": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 100),
				},
				MarkdownDescription: "The smallest share, as a percentage, a slice must reach to be drawn. Valid values are 0 to 100.",
			},
			"show_total": schema.BoolAttribute{
				Optional: true,
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
		},
	}
}

func dynamicGaugeModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"arc_display":        types.ObjectType{AttrTypes: dynamicArcDisplayAttr()},
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"custom_unit":         types.StringType,
		"decimal_precision":   types.Int64Type,
		"display_series_name": types.BoolType,
		"legend":              types.ObjectType{AttrTypes: LegendAttr()},
		"legend_by":           types.StringType,
		"max":                 types.Float64Type,
		"min":                 types.Float64Type,
		"show_inner_arc":      types.BoolType,
		"show_min_max":        types.BoolType,
		"show_outer_arc":      types.BoolType,
		"threshold_type":      types.StringType,
		"thresholds": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicThresholdAttr()},
		},
		"unit":        types.StringType,
		"value_field": ObservationFieldsObject(),
		"value_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
	}
}

func dynamicArcDisplayAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"threshold_arc": types.BoolType,
		"value_arc":     types.BoolType,
	}
}

func dynamicPieChartModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_scheme":         types.StringType,
		"custom_unit":          types.StringType,
		"decimal_precision":    types.Int64Type,
		"group_name_template":  types.StringType,
		"hash_colors":          types.BoolType,
		"label_definition":     types.ObjectType{AttrTypes: dynamicPieChartLabelDefinitionAttr()},
		"legend":               types.ObjectType{AttrTypes: LegendAttr()},
		"max_slices_per_chart": types.Int64Type,
		"max_slices_per_stack": types.Int64Type,
		"min_slice_percentage": types.Int64Type,
		"show_total":           types.BoolType,
		"stack_name_template":  types.StringType,
		"sub_category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"unit":        types.StringType,
		"value_field": ObservationFieldsObject(),
	}
}

func dynamicPieChartLabelDefinitionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"is_visible":      types.BoolType,
		"label_source":    types.StringType,
		"show_name":       types.BoolType,
		"show_percentage": types.BoolType,
		"show_value":      types.BoolType,
	}
}

func expandDynamicGauge(ctx context.Context, gauge *DynamicGaugeModel) (*dashboardservice.VisualizationGauge, diag.Diagnostics) {
	if gauge == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, gauge.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}

	valueField, diags := ExpandObservationFieldObject(ctx, gauge.ValueField)
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := ExpandObservationFields(ctx, gauge.ValueFields)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := expandDynamicThresholds(ctx, gauge.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	legend, diags := ExpandLegend(ctx, gauge.Legend)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.VisualizationGauge{
		AllowAbbreviation: gauge.AllowAbbreviation.ValueBoolPointer(),
		ArcDisplay:        expandDynamicArcDisplay(gauge.ArcDisplay),
		CategoryFields:    categoryFields,
		CustomUnit:        gauge.CustomUnit.ValueStringPointer(),
		DecimalPrecision:  expandInt32Pointer(gauge.DecimalPrecision),
		DisplaySeriesName: gauge.DisplaySeriesName.ValueBoolPointer(),
		Legend:            legend,
		LegendBy:          OptionalEnumPointer(gauge.LegendBy, DashboardSchemaToProtoLegendBy),
		Max:               gauge.Max.ValueFloat64Pointer(),
		Min:               gauge.Min.ValueFloat64Pointer(),
		ShowInnerArc:      gauge.ShowInnerArc.ValueBoolPointer(),
		ShowMinMax:        gauge.ShowMinMax.ValueBoolPointer(),
		ShowOuterArc:      gauge.ShowOuterArc.ValueBoolPointer(),
		ThresholdType:     OptionalEnumPointer(gauge.ThresholdType, DashboardSchemaToProtoThresholdType),
		Thresholds:        thresholds,
		Unit:              OptionalEnumPointer(gauge.Unit, DashboardSchemaToProtoUnit),
		ValueField:        valueField,
		ValueFields:       valueFields,
	}, nil
}

func expandDynamicArcDisplay(arcDisplay *DynamicArcDisplayModel) *dashboardservice.ArcDisplay {
	if arcDisplay == nil {
		return nil
	}

	return &dashboardservice.ArcDisplay{
		ThresholdArc: arcDisplay.ThresholdArc.ValueBoolPointer(),
		ValueArc:     arcDisplay.ValueArc.ValueBoolPointer(),
	}
}

func expandDynamicPieChart(ctx context.Context, pieChart *DynamicPieChartModel) (*dashboardservice.VisualizationPieChart, diag.Diagnostics) {
	if pieChart == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, pieChart.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}

	subCategoryFields, diags := ExpandObservationFields(ctx, pieChart.SubCategoryFields)
	if diags.HasError() {
		return nil, diags
	}

	valueField, diags := ExpandObservationFieldObject(ctx, pieChart.ValueField)
	if diags.HasError() {
		return nil, diags
	}

	legend, diags := ExpandLegend(ctx, pieChart.Legend)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.VisualizationPieChart{
		AllowAbbreviation:  pieChart.AllowAbbreviation.ValueBoolPointer(),
		CategoryFields:     categoryFields,
		ColorScheme:        pieChart.ColorScheme.ValueStringPointer(),
		CustomUnit:         pieChart.CustomUnit.ValueStringPointer(),
		DecimalPrecision:   expandInt32Pointer(pieChart.DecimalPrecision),
		GroupNameTemplate:  pieChart.GroupNameTemplate.ValueStringPointer(),
		HashColors:         pieChart.HashColors.ValueBoolPointer(),
		LabelDefinition:    expandDynamicPieChartLabelDefinition(pieChart.LabelDefinition),
		Legend:             legend,
		MaxSlicesPerChart:  expandInt32Pointer(pieChart.MaxSlicesPerChart),
		MaxSlicesPerStack:  expandInt32Pointer(pieChart.MaxSlicesPerStack),
		MinSlicePercentage: expandInt32Pointer(pieChart.MinSlicePercentage),
		ShowTotal:          pieChart.ShowTotal.ValueBoolPointer(),
		StackNameTemplate:  pieChart.StackNameTemplate.ValueStringPointer(),
		SubCategoryFields:  subCategoryFields,
		Unit:               OptionalEnumPointer(pieChart.Unit, DashboardSchemaToProtoUnit),
		ValueField:         valueField,
	}, nil
}

func expandDynamicPieChartLabelDefinition(labelDefinition *DynamicPieChartLabelDefinitionModel) *dashboardservice.VisualizationPieChartLabelDefinition {
	if labelDefinition == nil {
		return nil
	}

	return &dashboardservice.VisualizationPieChartLabelDefinition{
		IsVisible:      labelDefinition.IsVisible.ValueBoolPointer(),
		LabelSource:    OptionalEnumPointer(labelDefinition.LabelSource, dashboardSchemaToProtoPieChartLabelSource),
		ShowName:       labelDefinition.ShowName.ValueBoolPointer(),
		ShowPercentage: labelDefinition.ShowPercentage.ValueBoolPointer(),
		ShowValue:      labelDefinition.ShowValue.ValueBoolPointer(),
	}
}

func flattenDynamicGauge(ctx context.Context, gauge *dashboardservice.VisualizationGauge) (*DynamicGaugeModel, diag.Diagnostics) {
	if gauge == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, gauge.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}

	valueField, diags := FlattenObservationField(ctx, gauge.ValueField)
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := FlattenObservationFields(ctx, gauge.GetValueFields())
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := flattenDynamicThresholds(ctx, gauge.GetThresholds())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicGaugeModel{
		AllowAbbreviation: types.BoolPointerValue(gauge.AllowAbbreviation),
		ArcDisplay:        flattenDynamicArcDisplay(gauge.ArcDisplay),
		CategoryFields:    categoryFields,
		CustomUnit:        types.StringPointerValue(gauge.CustomUnit),
		DecimalPrecision:  flattenInt32Pointer(gauge.DecimalPrecision),
		DisplaySeriesName: types.BoolPointerValue(gauge.DisplaySeriesName),
		Legend:            FlattenLegend(gauge.Legend),
		LegendBy:          flattenOptionalEnum(gauge.LegendBy, DashboardProtoToSchemaLegendBy),
		Max:               types.Float64PointerValue(gauge.Max),
		Min:               types.Float64PointerValue(gauge.Min),
		ShowInnerArc:      types.BoolPointerValue(gauge.ShowInnerArc),
		ShowMinMax:        types.BoolPointerValue(gauge.ShowMinMax),
		ShowOuterArc:      types.BoolPointerValue(gauge.ShowOuterArc),
		ThresholdType:     flattenOptionalEnum(gauge.ThresholdType, DashboardProtoToSchemaThresholdType),
		Thresholds:        thresholds,
		Unit:              flattenOptionalEnum(gauge.Unit, DashboardProtoToSchemaUnit),
		ValueField:        valueField,
		ValueFields:       valueFields,
	}, nil
}

func flattenDynamicArcDisplay(arcDisplay *dashboardservice.ArcDisplay) *DynamicArcDisplayModel {
	if arcDisplay == nil {
		return nil
	}

	return &DynamicArcDisplayModel{
		ThresholdArc: types.BoolPointerValue(arcDisplay.ThresholdArc),
		ValueArc:     types.BoolPointerValue(arcDisplay.ValueArc),
	}
}

func flattenDynamicPieChart(ctx context.Context, pieChart *dashboardservice.VisualizationPieChart) (*DynamicPieChartModel, diag.Diagnostics) {
	if pieChart == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, pieChart.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}

	subCategoryFields, diags := FlattenObservationFields(ctx, pieChart.GetSubCategoryFields())
	if diags.HasError() {
		return nil, diags
	}

	valueField, diags := FlattenObservationField(ctx, pieChart.ValueField)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicPieChartModel{
		AllowAbbreviation:  types.BoolPointerValue(pieChart.AllowAbbreviation),
		CategoryFields:     categoryFields,
		ColorScheme:        types.StringPointerValue(pieChart.ColorScheme),
		CustomUnit:         types.StringPointerValue(pieChart.CustomUnit),
		DecimalPrecision:   flattenInt32Pointer(pieChart.DecimalPrecision),
		GroupNameTemplate:  types.StringPointerValue(pieChart.GroupNameTemplate),
		HashColors:         types.BoolPointerValue(pieChart.HashColors),
		LabelDefinition:    flattenDynamicPieChartLabelDefinition(pieChart.LabelDefinition),
		Legend:             FlattenLegend(pieChart.Legend),
		MaxSlicesPerChart:  flattenInt32Pointer(pieChart.MaxSlicesPerChart),
		MaxSlicesPerStack:  flattenInt32Pointer(pieChart.MaxSlicesPerStack),
		MinSlicePercentage: flattenInt32Pointer(pieChart.MinSlicePercentage),
		ShowTotal:          types.BoolPointerValue(pieChart.ShowTotal),
		StackNameTemplate:  types.StringPointerValue(pieChart.StackNameTemplate),
		SubCategoryFields:  subCategoryFields,
		Unit:               flattenOptionalEnum(pieChart.Unit, DashboardProtoToSchemaUnit),
		ValueField:         valueField,
	}, nil
}

func flattenDynamicPieChartLabelDefinition(labelDefinition *dashboardservice.VisualizationPieChartLabelDefinition) *DynamicPieChartLabelDefinitionModel {
	if labelDefinition == nil {
		return nil
	}

	return &DynamicPieChartLabelDefinitionModel{
		IsVisible:      types.BoolPointerValue(labelDefinition.IsVisible),
		LabelSource:    flattenOptionalEnum(labelDefinition.LabelSource, dashboardProtoToSchemaPieChartLabelSource),
		ShowName:       types.BoolPointerValue(labelDefinition.ShowName),
		ShowPercentage: types.BoolPointerValue(labelDefinition.ShowPercentage),
		ShowValue:      types.BoolPointerValue(labelDefinition.ShowValue),
	}
}
