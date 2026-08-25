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
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
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

func dynamicStatCardSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"allow_abbreviation": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Shorten large numbers, for example `1.2K` instead of `1200`.",
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
			"color_label_mapping": dynamicColorLabelMappingSchema(),
			"custom_unit": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "A free-text unit label. Documented as taking effect only when `unit` is `custom`.",
			},
			"decimal_precision": schema.Int64Attribute{
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(0, 15),
				},
				MarkdownDescription: "How many digits to show after the decimal point. Valid values are 0 to 15.",
			},
			"label":  dynamicStatVisualElementSchema(true),
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
			"primary_value": dynamicStatVisualElementSchema(false),
			"title":         dynamicStatVisualElementSchema(true),
			"unit":          UnitSchema(),
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

func dynamicStatVisualElementSchema(allowMappedValues bool) schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		MarkdownDescription: "Text element: read a field with `observation_field`, or write it with `template_text`.",
		Attributes: map[string]schema.Attribute{
			"mapped_values": schema.BoolAttribute{
				Optional:            true,
				Validators:          mappedValuesValidators(allowMappedValues),
				MarkdownDescription: mappedValuesDescription(allowMappedValues),
			},
			"observation_field": schema.SingleNestedAttribute{
				Attributes: ObservationFieldSchema(),
				Optional:   true,
			},
			"template_text": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Display text, which may reference variables declared in `template_variables`.",
			},
			"template_variables": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Variables that `template_text` can reference.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"mapped_values": schema.BoolAttribute{
							Optional:            true,
							Validators:          mappedValuesValidators(true),
							MarkdownDescription: "Set to `true` to resolve the variable from the `color_label_mapping` result instead of a field. Mutually exclusive with `observation_field`.",
						},
						"observation_field": schema.SingleNestedAttribute{
							Attributes: ObservationFieldSchema(),
							Optional:   true,
						},
					},
				},
			},
		},
	}
}

func dynamicColorLabelMappingSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"color_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidColorApplyTarget...),
				},
				MarkdownDescription: fmt.Sprintf("Which part of the widget the color applies to. Valid values are: %s.", strings.Join(dashboardValidColorApplyTarget, ", ")),
			},
			"range": schema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Colour by where the value falls in a numeric range.",
				Attributes: map[string]schema.Attribute{
					"min_max": schema.SingleNestedAttribute{
						Optional:            true,
						MarkdownDescription: "The range ends. Either derived from the data (`auto`) or fixed (`custom`).",
						Attributes: map[string]schema.Attribute{
							"auto": schema.BoolAttribute{
								Optional: true,
								Validators: []validator.Bool{
									mustBeTrueValidator{},
								},
								MarkdownDescription: "Set to `true` to derive the range ends from the data.",
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
				},
			},
			"regex": dynamicMappingSectionsSchema("Colour by matching the value against regular expressions."),
			"value": dynamicMappingSectionsSchema("Colour by matching the value exactly."),
		},
		Validators: []validator.Object{
			ExactlyOneOfChildren("range", "value", "regex"),
		},
	}
}

func dynamicMappingSectionsSchema(description string) schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		MarkdownDescription: description,
		Attributes: map[string]schema.Attribute{
			"sections": schema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "One entry per value to match.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"color": schema.StringAttribute{
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(utils.UNSPECIFIED),
							Validators: []validator.String{
								stringvalidator.OneOf(dashboardValidColorSolidType...),
							},
							MarkdownDescription: fmt.Sprintf("The section color. Valid values are: %s.", strings.Join(dashboardValidColorSolidType, ", ")),
						},
						"map_to": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Text to display instead of the matched value.",
						},
						"value": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func dynamicStatCardModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"category_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
		"color_label_mapping": types.ObjectType{AttrTypes: dynamicColorLabelMappingAttr()},
		"custom_unit":         types.StringType,
		"decimal_precision":   types.Int64Type,
		"label":               types.ObjectType{AttrTypes: dynamicStatVisualElementAttr()},
		"legend":              types.ObjectType{AttrTypes: LegendAttr()},
		"legend_by":           types.StringType,
		"primary_value":       types.ObjectType{AttrTypes: dynamicStatVisualElementAttr()},
		"title":               types.ObjectType{AttrTypes: dynamicStatVisualElementAttr()},
		"unit":                types.StringType,
		"value_fields": types.ListType{
			ElemType: ObservationFieldsObject(),
		},
	}
}

func dynamicStatVisualElementAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"mapped_values":     types.BoolType,
		"observation_field": ObservationFieldsObject(),
		"template_text":     types.StringType,
		"template_variables": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()},
		},
	}
}

func dynamicTemplateVariableAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"mapped_values":     types.BoolType,
		"observation_field": ObservationFieldsObject(),
	}
}

func dynamicColorLabelMappingAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"color_by": types.StringType,
		"range":    types.ObjectType{AttrTypes: dynamicRangeMappingAttr()},
		"regex":    types.ObjectType{AttrTypes: dynamicSectionsMappingAttr()},
		"value":    types.ObjectType{AttrTypes: dynamicSectionsMappingAttr()},
	}
}

func dynamicSectionsMappingAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"sections": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicMappingSectionAttr()},
		},
	}
}

func expandDynamicStatCard(ctx context.Context, statCard *DynamicStatCardModel) (*dashboardservice.StatCard, diag.Diagnostics) {
	if statCard == nil {
		return nil, nil
	}

	categoryFields, diags := ExpandObservationFields(ctx, statCard.CategoryFields)
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := ExpandObservationFields(ctx, statCard.ValueFields)
	if diags.HasError() {
		return nil, diags
	}

	colorLabelMapping, diags := expandDynamicColorLabelMapping(ctx, statCard.ColorLabelMapping)
	if diags.HasError() {
		return nil, diags
	}

	label, diags := expandDynamicStatVisualElement(ctx, statCard.Label)
	if diags.HasError() {
		return nil, diags
	}

	primaryValue, diags := expandDynamicStatVisualElement(ctx, statCard.PrimaryValue)
	if diags.HasError() {
		return nil, diags
	}

	title, diags := expandDynamicStatVisualElement(ctx, statCard.Title)
	if diags.HasError() {
		return nil, diags
	}

	legend, diags := ExpandLegend(ctx, statCard.Legend)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.StatCard{
		AllowAbbreviation: statCard.AllowAbbreviation.ValueBoolPointer(),
		CategoryFields:    categoryFields,
		ColorLabelMapping: colorLabelMapping,
		CustomUnit:        statCard.CustomUnit.ValueStringPointer(),
		DecimalPrecision:  expandInt32Pointer(statCard.DecimalPrecision),
		Label:             label,
		Legend:            legend,
		LegendBy:          OptionalEnumPointer(statCard.LegendBy, DashboardSchemaToProtoLegendBy),
		PrimaryValue:      primaryValue,
		Title:             title,
		Unit:              OptionalEnumPointer(statCard.Unit, DashboardSchemaToProtoUnit),
		ValueFields:       valueFields,
	}, nil
}

func expandDynamicStatVisualElement(ctx context.Context, element *DynamicStatVisualElementModel) (*dashboardservice.StatVisualElement, diag.Diagnostics) {
	if element == nil {
		return nil, nil
	}

	observationField, diags := ExpandObservationFieldObject(ctx, element.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	templateVariables, diags := expandDynamicTemplateVariables(ctx, element.TemplateVariables)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.StatVisualElement{
		MappedValues:      expandDynamicMappedValuesMarker(element.MappedValues),
		ObservationField:  observationField,
		TemplateText:      element.TemplateText.ValueStringPointer(),
		TemplateVariables: templateVariables,
	}, nil
}

func expandDynamicTemplateVariables(ctx context.Context, variables types.List) ([]dashboardservice.DisplayNameTemplateVariable, diag.Diagnostics) {
	var models []DynamicTemplateVariableModel
	diags := variables.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.DisplayNameTemplateVariable, 0, len(models))
	for i := range models {
		observationField, dg := ExpandObservationFieldObject(ctx, models[i].ObservationField)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.DisplayNameTemplateVariable{
			MappedValues:     expandDynamicMappedValuesMarker(models[i].MappedValues),
			ObservationField: observationField,
		})
	}

	return expanded, diags
}

func expandDynamicColorLabelMapping(ctx context.Context, mapping *DynamicColorLabelMappingModel) (*dashboardservice.ColorLabelMapping, diag.Diagnostics) {
	if mapping == nil {
		return nil, nil
	}

	rangeMapping, diags := expandDynamicRangeMapping(ctx, mapping.Range)
	if diags.HasError() {
		return nil, diags
	}

	regex, diags := expandDynamicSectionsRegex(ctx, mapping.Regex)
	if diags.HasError() {
		return nil, diags
	}

	value, diags := expandDynamicSectionsValue(ctx, mapping.Value)
	if diags.HasError() {
		return nil, diags
	}

	set := 0
	for _, selected := range []bool{rangeMapping != nil, regex != nil, value != nil} {
		if selected {
			set++
		}
	}
	if set != 1 {
		return nil, dynamicUnionDiagnostic("color_label_mapping", "`range`, `value` or `regex`")
	}

	return &dashboardservice.ColorLabelMapping{
		ColorBy: OptionalEnumPointer(mapping.ColorBy, dashboardSchemaToProtoColorApplyTarget),
		Range:   rangeMapping,
		Regex:   regex,
		Value:   value,
	}, nil
}

func expandDynamicSectionsRegex(ctx context.Context, mapping *DynamicSectionsMappingModel) (*dashboardservice.RegexMapping, diag.Diagnostics) {
	if mapping == nil {
		return nil, nil
	}
	sections, diags := expandDynamicMappingSections(ctx, mapping.Sections)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.RegexMapping{Sections: sections}, nil
}

func expandDynamicSectionsValue(ctx context.Context, mapping *DynamicSectionsMappingModel) (*dashboardservice.ColorLabelMappingValueMapping, diag.Diagnostics) {
	if mapping == nil {
		return nil, nil
	}
	sections, diags := expandDynamicMappingSections(ctx, mapping.Sections)
	if diags.HasError() {
		return nil, diags
	}
	return &dashboardservice.ColorLabelMappingValueMapping{Sections: sections}, nil
}

func expandDynamicMappingSections(ctx context.Context, sections types.List) ([]dashboardservice.MappingSection, diag.Diagnostics) {
	var models []DynamicMappingSectionModel
	diags := sections.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.MappingSection, 0, len(models))
	for i := range models {
		expanded = append(expanded, dashboardservice.MappingSection{
			Color: OptionalEnumPointer(models[i].Color, dashboardSchemaToProtoColorSolidType),
			MapTo: models[i].MapTo.ValueStringPointer(),
			Value: models[i].Value.ValueStringPointer(),
		})
	}

	return expanded, diags
}

func flattenDynamicStatCard(ctx context.Context, statCard *dashboardservice.StatCard) (*DynamicStatCardModel, diag.Diagnostics) {
	if statCard == nil {
		return nil, nil
	}

	categoryFields, diags := FlattenObservationFields(ctx, statCard.GetCategoryFields())
	if diags.HasError() {
		return nil, diags
	}

	valueFields, diags := FlattenObservationFields(ctx, statCard.GetValueFields())
	if diags.HasError() {
		return nil, diags
	}

	colorLabelMapping, diags := flattenDynamicColorLabelMapping(ctx, statCard.ColorLabelMapping)
	if diags.HasError() {
		return nil, diags
	}

	label, diags := flattenDynamicStatVisualElement(ctx, statCard.Label)
	if diags.HasError() {
		return nil, diags
	}

	primaryValue, diags := flattenDynamicStatVisualElement(ctx, statCard.PrimaryValue)
	if diags.HasError() {
		return nil, diags
	}

	title, diags := flattenDynamicStatVisualElement(ctx, statCard.Title)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicStatCardModel{
		AllowAbbreviation: types.BoolPointerValue(statCard.AllowAbbreviation),
		CategoryFields:    categoryFields,
		ColorLabelMapping: colorLabelMapping,
		CustomUnit:        types.StringPointerValue(statCard.CustomUnit),
		DecimalPrecision:  flattenInt32Pointer(statCard.DecimalPrecision),
		Label:             label,
		Legend:            FlattenLegend(statCard.Legend),
		LegendBy:          flattenOptionalEnum(statCard.LegendBy, DashboardProtoToSchemaLegendBy),
		PrimaryValue:      primaryValue,
		Title:             title,
		Unit:              flattenOptionalEnum(statCard.Unit, DashboardProtoToSchemaUnit),
		ValueFields:       valueFields,
	}, nil
}

func flattenDynamicStatVisualElement(ctx context.Context, element *dashboardservice.StatVisualElement) (*DynamicStatVisualElementModel, diag.Diagnostics) {
	if element == nil {
		return nil, nil
	}

	observationField, diags := FlattenObservationField(ctx, element.ObservationField)
	if diags.HasError() {
		return nil, diags
	}

	templateVariables, diags := flattenDynamicTemplateVariables(ctx, element.GetTemplateVariables())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicStatVisualElementModel{
		MappedValues:      flattenDynamicMappedValuesMarker(element.MappedValues),
		ObservationField:  observationField,
		TemplateText:      types.StringPointerValue(element.TemplateText),
		TemplateVariables: templateVariables,
	}, nil
}

func flattenDynamicTemplateVariables(ctx context.Context, variables []dashboardservice.DisplayNameTemplateVariable) (types.List, diag.Diagnostics) {
	if len(variables) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(variables))
	for i := range variables {
		observationField, dg := FlattenObservationField(ctx, variables[i].ObservationField)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		model := &DynamicTemplateVariableModel{
			MappedValues:     flattenDynamicMappedValuesMarker(variables[i].MappedValues),
			ObservationField: observationField,
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTemplateVariableAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTemplateVariableAttr()}, elements)
}

func flattenDynamicColorLabelMapping(ctx context.Context, mapping *dashboardservice.ColorLabelMapping) (*DynamicColorLabelMappingModel, diag.Diagnostics) {
	if mapping == nil {
		return nil, nil
	}

	rangeMapping, diags := flattenDynamicRangeMapping(ctx, mapping.Range)
	if diags.HasError() {
		return nil, diags
	}

	var regex *DynamicSectionsMappingModel
	if mapping.Regex != nil {
		sections, dg := flattenDynamicMappingSections(ctx, mapping.Regex.GetSections())
		if dg.HasError() {
			return nil, dg
		}
		regex = &DynamicSectionsMappingModel{Sections: sections}
	}

	var value *DynamicSectionsMappingModel
	if mapping.Value != nil {
		sections, dg := flattenDynamicMappingSections(ctx, mapping.Value.GetSections())
		if dg.HasError() {
			return nil, dg
		}
		value = &DynamicSectionsMappingModel{Sections: sections}
	}

	return &DynamicColorLabelMappingModel{
		ColorBy: flattenOptionalEnum(mapping.ColorBy, dashboardProtoToSchemaColorApplyTarget),
		Range:   rangeMapping,
		Regex:   regex,
		Value:   value,
	}, nil
}

func flattenDynamicMappingSections(ctx context.Context, sections []dashboardservice.MappingSection) (types.List, diag.Diagnostics) {
	if len(sections) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicMappingSectionAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(sections))
	for i := range sections {
		model := &DynamicMappingSectionModel{
			Color: flattenOptionalEnum(sections[i].Color, dashboardProtoToSchemaColorSolidType),
			MapTo: types.StringPointerValue(sections[i].MapTo),
			Value: types.StringPointerValue(sections[i].Value),
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicMappingSectionAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicMappingSectionAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicMappingSectionAttr()}, elements)
}

var dashboardValidColorApplyTarget = utils.GetKeys(dashboardSchemaToProtoColorApplyTarget)

func dynamicThresholdsSchema() schema.Attribute {
	return schema.ListNestedAttribute{
		Optional: true,
		Validators: []validator.List{
			listvalidator.SizeAtLeast(1),
		},
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"from": schema.Float64Attribute{
					Optional: true,
				},
				"color": schema.StringAttribute{
					Optional: true,
				},
				"label": schema.StringAttribute{
					Optional: true,
				},
			},
		},
	}
}

var dashboardValidColorSolidType = utils.GetKeys(dashboardSchemaToProtoColorSolidType)

func dynamicRangeMappingAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"min_max":        types.ObjectType{AttrTypes: dynamicMinMaxAttr()},
		"threshold_type": types.StringType,
		"thresholds": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicThresholdAttr()},
		},
	}
}

func dynamicMappingSectionAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"color":  types.StringType,
		"map_to": types.StringType,
		"value":  types.StringType,
	}
}

func expandDynamicRangeMapping(ctx context.Context, rangeMapping *DynamicRangeMappingModel) (*dashboardservice.RangeMapping, diag.Diagnostics) {
	if rangeMapping == nil {
		return nil, nil
	}

	thresholds, diags := expandDynamicThresholds(ctx, rangeMapping.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	// The API has no representation for a min/max with neither arm chosen, so
	// sending an empty one would come back null and fail the apply. Validators
	// cannot catch this when `auto` is only known after apply.
	var minMax *dashboardservice.MinMax
	if rangeMapping.MinMax != nil {
		switch {
		case rangeMapping.MinMax.Custom != nil && rangeMapping.MinMax.Auto.ValueBool():
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
				"Invalid Attribute Combination",
				"min_max requires exactly one of `auto` or `custom`, not both.",
			)}
		case rangeMapping.MinMax.Custom != nil:
			minMax = &dashboardservice.MinMax{Custom: &dashboardservice.MinMaxCustom{
				Max: rangeMapping.MinMax.Custom.Max.ValueFloat64Pointer(),
				Min: rangeMapping.MinMax.Custom.Min.ValueFloat64Pointer(),
			}}
		case rangeMapping.MinMax.Auto.ValueBool():
			minMax = &dashboardservice.MinMax{Auto: map[string]interface{}{}}
		default:
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
				"Invalid Attribute Combination",
				"min_max requires exactly one of `auto` or `custom`, and `auto` must be true. Remove the min_max block to let the widget scale itself.",
			)}
		}
	}

	return &dashboardservice.RangeMapping{
		MinMax:        minMax,
		ThresholdType: OptionalEnumPointer(rangeMapping.ThresholdType, DashboardSchemaToProtoThresholdType),
		Thresholds:    thresholds,
	}, nil
}

var dashboardSchemaToProtoColorApplyTarget = map[string]dashboardservice.ColorApplyTarget{
	utils.UNSPECIFIED: dashboardservice.COLORAPPLYTARGET_COLOR_APPLY_TARGET_UNSPECIFIED,
	"value":           dashboardservice.COLORAPPLYTARGET_COLOR_APPLY_TARGET_VALUE,
	"background":      dashboardservice.COLORAPPLYTARGET_COLOR_APPLY_TARGET_BACKGROUND,
	"row":             dashboardservice.COLORAPPLYTARGET_COLOR_APPLY_TARGET_ROW,
}

var dashboardSchemaToProtoColorSolidType = map[string]dashboardservice.ColorSolidType{
	utils.UNSPECIFIED: dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_UNSPECIFIED,
	"blue":            dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_BLUE,
	"green":           dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_GREEN,
	"red":             dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_RED,
	"purple":          dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_PURPLE,
	"cyan":            dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_CYAN,
	"magenta":         dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_MAGENTA,
	"orange":          dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_ORANGE,
	"yellow":          dashboardservice.COLORSOLIDTYPE_COLOR_SOLID_TYPE_YELLOW,
}

func flattenDynamicRangeMapping(ctx context.Context, rangeMapping *dashboardservice.RangeMapping) (*DynamicRangeMappingModel, diag.Diagnostics) {
	if rangeMapping == nil {
		return nil, nil
	}

	thresholds, diags := flattenDynamicThresholds(ctx, rangeMapping.GetThresholds())
	if diags.HasError() {
		return nil, diags
	}

	// Leave min_max unset when the backend chose neither arm: a block with both
	// children null is state no configuration can produce, so it would diff
	// forever after an import.
	var minMax *DynamicMinMaxModel
	switch {
	case rangeMapping.MinMax == nil:
	case rangeMapping.MinMax.Custom != nil:
		minMax = &DynamicMinMaxModel{Auto: types.BoolNull(), Custom: &DynamicMinMaxCustomModel{
			Max: types.Float64PointerValue(rangeMapping.MinMax.Custom.Max),
			Min: types.Float64PointerValue(rangeMapping.MinMax.Custom.Min),
		}}
	case rangeMapping.MinMax.Auto != nil:
		minMax = &DynamicMinMaxModel{Auto: types.BoolValue(true)}
	}

	return &DynamicRangeMappingModel{
		MinMax:        minMax,
		ThresholdType: flattenOptionalEnum(rangeMapping.ThresholdType, DashboardProtoToSchemaThresholdType),
		Thresholds:    thresholds,
	}, nil
}

var dashboardProtoToSchemaColorApplyTarget = utils.ReverseMap(dashboardSchemaToProtoColorApplyTarget)

var dashboardProtoToSchemaColorSolidType = utils.ReverseMap(dashboardSchemaToProtoColorSolidType)

func dynamicMinMaxAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"auto":   types.BoolType,
		"custom": types.ObjectType{AttrTypes: dynamicMinMaxCustomAttr()},
	}
}

func dynamicMinMaxCustomAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"max": types.Float64Type,
		"min": types.Float64Type,
	}
}

type mustBeTrueValidator struct{}

func (v mustBeTrueValidator) Description(context.Context) string {
	return "value must be true when set"
}

func (v mustBeTrueValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v mustBeTrueValidator) ValidateBool(ctx context.Context, req validator.BoolRequest, resp *validator.BoolResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if !req.ConfigValue.ValueBool() {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid value", "this marker must be true when set; omit it otherwise")
	}
}

// The API models mapped values as an empty message, so the only information
// carried is whether the branch is selected. Any non-nil object therefore
// flattens to true; if the message ever gains fields, this needs a real object.
func expandDynamicMappedValuesMarker(value types.Bool) map[string]interface{} {
	if value.IsNull() || value.IsUnknown() || !value.ValueBool() {
		return nil
	}
	return map[string]interface{}{}
}

func flattenDynamicMappedValuesMarker(value map[string]interface{}) types.Bool {
	if value == nil {
		return types.BoolNull()
	}
	return types.BoolValue(true)
}

func mappedValuesValidators(allowed bool) []validator.Bool {
	conflict := boolvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("observation_field"))
	if allowed {
		return []validator.Bool{mustBeTrueValidator{}, conflict}
	}
	return []validator.Bool{unsupportedMappedValuesValidator{}, conflict}
}

func mappedValuesDescription(allowed bool) string {
	if allowed {
		return "Set to `true` to display the result of `color_label_mapping` instead of a field. Mutually exclusive with `observation_field`."
	}
	return "Not supported here: the primary value is the source the color label mapping reads from, so it cannot itself display the mapping result."
}

// unsupportedMappedValuesValidator rejects the mapped_values branch on
// primary_value, which the API refuses because the primary value is the source
// the mapping reads from.
type unsupportedMappedValuesValidator struct{}

func (v unsupportedMappedValuesValidator) Description(context.Context) string {
	return "mapped_values is not supported for primary_value"
}

func (v unsupportedMappedValuesValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v unsupportedMappedValuesValidator) ValidateBool(_ context.Context, req validator.BoolRequest, resp *validator.BoolResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Unsupported Attribute Combination",
		"primary_value cannot use mapped_values, because the primary value is the source the color label mapping reads from. Use observation_field or template_text instead.",
	)
}
