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

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func dynamicTableSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"columns": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"field": schema.SingleNestedAttribute{
							Attributes: dynamicTableObservationFieldSchema(),
							Optional:   true,
						},
					},
				},
			},
			"rules": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"description": schema.StringAttribute{
							Optional: true,
						},
						"id": schema.StringAttribute{
							Optional: true,
							Computed: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseNonNullStateForUnknown(),
							},
						},
						"name": schema.StringAttribute{
							Optional: true,
						},
						"properties": schema.ListNestedAttribute{
							Optional: true,
							Validators: []validator.List{
								listvalidator.SizeAtLeast(1),
							},
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"id": schema.StringAttribute{
										Optional: true,
										Computed: true,
										PlanModifiers: []planmodifier.String{
											stringplanmodifier.UseNonNullStateForUnknown(),
										},
									},
									"definition": dynamicTablePropertyDefinitionSchema(),
								},
							},
						},
						"rule_scope": dynamicTableRuleScopeSchema(),
					},
				},
			},
			"settings": dynamicTableSettingsSchema(),
		},
	}
}

func dynamicTablePropertyDefinitionSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"alignment": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidTextAlignment...),
				},
				MarkdownDescription: fmt.Sprintf("The text alignment. Valid values are: %s.", strings.Join(dashboardValidTextAlignment, ", ")),
			},
			"column_display_name": schema.StringAttribute{
				Optional: true,
			},
			"link": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"actions": schema.ListNestedAttribute{
						Optional: true,
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
						},
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Optional: true,
									Computed: true,
									PlanModifiers: []planmodifier.String{
										stringplanmodifier.UseNonNullStateForUnknown(),
									},
								},
								"name": schema.StringAttribute{
									Optional: true,
								},
								"should_open_in_new_window": schema.BoolAttribute{
									Optional: true,
								},
								"url": schema.StringAttribute{
									Optional: true,
								},
							},
						},
					},
				},
			},
			"regex_extract": schema.StringAttribute{
				Optional: true,
			},
			"thresholds": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"max": schema.Float64Attribute{
						Optional: true,
					},
					"min": schema.Float64Attribute{
						Optional: true,
					},
					"type": schema.StringAttribute{
						Optional: true,
						Computed: true,
						Default:  stringdefault.StaticString(utils.UNSPECIFIED),
						Validators: []validator.String{
							stringvalidator.OneOf(DashboardValidThresholdTypes...),
						},
						MarkdownDescription: fmt.Sprintf("The threshold type. Valid values are: %s.", strings.Join(DashboardValidThresholdTypes, ", ")),
					},
					"values": dynamicThresholdsSchema(),
				},
			},
			"units": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"allow_abbreviation": schema.BoolAttribute{
						Optional: true,
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
					"max": schema.Float64Attribute{
						Optional: true,
					},
					"min": schema.Float64Attribute{
						Optional: true,
					},
					"unit": UnitSchema(),
				},
			},
			"values_alias": schema.StringAttribute{
				Optional: true,
			},
			"values_mapping": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"mappings": schema.ListNestedAttribute{
						Optional: true,
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
						},
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"input_value": schema.StringAttribute{
									Optional: true,
								},
								"replace_value": schema.StringAttribute{
									Optional: true,
								},
								"type": schema.StringAttribute{
									Optional: true,
									Computed: true,
									Default:  stringdefault.StaticString(utils.UNSPECIFIED),
									Validators: []validator.String{
										stringvalidator.OneOf(dashboardValidValuesMappingType...),
									},
									MarkdownDescription: fmt.Sprintf("The values mapping type. Valid values are: %s.", strings.Join(dashboardValidValuesMappingType, ", ")),
								},
							},
						},
					},
				},
			},
		},
		Validators: []validator.Object{
			ExactlyOneOfChildren(
				"thresholds", "alignment", "units", "regex_extract",
				"link", "values_alias", "values_mapping", "column_display_name",
			),
		},
	}
}

// Table columns are stored with an absent or empty keypath - the API accepts
// both and this repo applies such a fixture - so the table cannot reuse the
// shared field schema, which requires at least one segment. An explicit empty
// list stays rejected because the read direction cannot produce one.
func dynamicTableObservationFieldSchema() map[string]schema.Attribute {
	attributes := ObservationFieldSchema()

	keypath := attributes["keypath"].(schema.ListAttribute)
	keypath.Required = false
	keypath.Optional = true
	attributes["keypath"] = keypath

	return attributes
}

func dynamicTableRuleScopeSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"field": schema.SingleNestedAttribute{
				Attributes: dynamicTableObservationFieldSchema(),
				Optional:   true,
			},
			"field_type": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidFieldDataType...),
				},
				MarkdownDescription: fmt.Sprintf("The field data type. Valid values are: %s.", strings.Join(dashboardValidFieldDataType, ", ")),
			},
			"regex": schema.StringAttribute{
				Optional: true,
			},
		},
		Validators: []validator.Object{
			ExactlyOneOfChildren("field", "regex", "field_type"),
		},
	}
}

func dynamicTableSettingsSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"column_widths": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"column_name": schema.StringAttribute{
							Optional: true,
						},
						"width": schema.Int64Attribute{
							Optional: true,
							Validators: []validator.Int64{
								int64validator.Between(1, math.MaxInt32),
							},
							MarkdownDescription: "The column width in pixels. Must be at least 1.",
						},
					},
				},
			},
			"row_style": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(utils.UNSPECIFIED),
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidRowStyles...),
				},
				MarkdownDescription: fmt.Sprintf("The row style. Valid values are: %s.", strings.Join(DashboardValidRowStyles, ", ")),
			},
		},
	}
}

func dynamicTableModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"columns": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()},
		},
		"rules": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()},
		},
		"settings": types.ObjectType{AttrTypes: dynamicTableSettingsModelAttr()},
	}
}

func dynamicTableColumnModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field": ObservationFieldsObject(),
	}
}

func dynamicTableRuleModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"description": types.StringType,
		"id":          types.StringType,
		"name":        types.StringType,
		"properties": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()},
		},
		"rule_scope": types.ObjectType{AttrTypes: dynamicTableRuleScopeModelAttr()},
	}
}

func dynamicTablePropertyModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":         types.StringType,
		"definition": types.ObjectType{AttrTypes: dynamicTablePropertyDefinitionModelAttr()},
	}
}

func dynamicTablePropertyDefinitionModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"alignment":           types.StringType,
		"column_display_name": types.StringType,
		"link":                types.ObjectType{AttrTypes: dynamicTablePropertyLinkModelAttr()},
		"regex_extract":       types.StringType,
		"thresholds":          types.ObjectType{AttrTypes: dynamicTablePropertyThresholdsModelAttr()},
		"units":               types.ObjectType{AttrTypes: dynamicTablePropertyUnitsModelAttr()},
		"values_alias":        types.StringType,
		"values_mapping":      types.ObjectType{AttrTypes: dynamicTablePropertyValuesMappingModelAttr()},
	}
}

func dynamicTablePropertyLinkModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"actions": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableLinkActionModelAttr()},
		},
	}
}

func dynamicTableLinkActionModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                        types.StringType,
		"name":                      types.StringType,
		"should_open_in_new_window": types.BoolType,
		"url":                       types.StringType,
	}
}

func dynamicTablePropertyThresholdsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"max":  types.Float64Type,
		"min":  types.Float64Type,
		"type": types.StringType,
		"values": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicThresholdAttr()},
		},
	}
}

func dynamicTablePropertyUnitsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"allow_abbreviation": types.BoolType,
		"custom_unit":        types.StringType,
		"decimal_precision":  types.Int64Type,
		"max":                types.Float64Type,
		"min":                types.Float64Type,
		"unit":               types.StringType,
	}
}

func dynamicTablePropertyValuesMappingModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"mappings": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableValueMappingModelAttr()},
		},
	}
}

func dynamicTableValueMappingModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"input_value":   types.StringType,
		"replace_value": types.StringType,
		"type":          types.StringType,
	}
}

func dynamicTableRuleScopeModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field":      ObservationFieldsObject(),
		"field_type": types.StringType,
		"regex":      types.StringType,
	}
}

func dynamicTableSettingsModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"column_widths": types.ListType{
			ElemType: types.ObjectType{AttrTypes: dynamicTableColumnWidthModelAttr()},
		},
		"row_style": types.StringType,
	}
}

func dynamicTableColumnWidthModelAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"column_name": types.StringType,
		"width":       types.Int64Type,
	}
}

func expandDynamicTable(ctx context.Context, table *DynamicTableModel) (*dashboardservice.Table, diag.Diagnostics) {
	if table == nil {
		return nil, nil
	}

	columns, diags := expandDynamicTableColumns(ctx, table.Columns)
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := expandDynamicTableRules(ctx, table.Rules)
	if diags.HasError() {
		return nil, diags
	}

	settings, diags := expandDynamicTableSettings(ctx, table.Settings)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.Table{
		Columns:  columns,
		Rules:    rules,
		Settings: settings,
	}, nil
}

func expandDynamicTableColumns(ctx context.Context, columns types.List) ([]dashboardservice.TableColumn, diag.Diagnostics) {
	var models []DynamicTableColumnModel
	diags := columns.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.TableColumn, 0, len(models))
	for i := range models {
		field, dg := ExpandObservationFieldObject(ctx, models[i].Field)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.TableColumn{Field: field})
	}

	return expanded, diags
}

func expandDynamicTableRules(ctx context.Context, rules types.List) ([]dashboardservice.TableRule, diag.Diagnostics) {
	var models []DynamicTableRuleModel
	diags := rules.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.TableRule, 0, len(models))
	for i := range models {
		properties, dg := expandDynamicTableProperties(ctx, models[i].Properties)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		ruleScope, dg := expandDynamicTableRuleScope(ctx, models[i].RuleScope)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.TableRule{
			Description: models[i].Description.ValueStringPointer(),
			Id:          ExpandDashboardUUID(models[i].ID),
			Name:        models[i].Name.ValueStringPointer(),
			Properties:  properties,
			RuleScope:   ruleScope,
		})
	}

	return expanded, diags
}

func expandDynamicTableProperties(ctx context.Context, properties types.List) ([]dashboardservice.Property, diag.Diagnostics) {
	var models []DynamicTablePropertyModel
	diags := properties.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	expanded := make([]dashboardservice.Property, 0, len(models))
	for i := range models {
		definition, dg := expandDynamicTablePropertyDefinition(ctx, models[i].Definition)
		if dg.HasError() {
			diags.Append(dg...)
			continue
		}
		expanded = append(expanded, dashboardservice.Property{
			Id:         ExpandDashboardUUID(models[i].ID),
			Definition: definition,
		})
	}

	return expanded, diags
}

func expandDynamicTablePropertyDefinition(ctx context.Context, definition *DynamicTablePropertyDefinitionModel) (*dashboardservice.PropertyDefinition, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	link, diags := expandDynamicTablePropertyLink(ctx, definition.Link)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := expandDynamicTablePropertyThresholds(ctx, definition.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	valuesMapping, diags := expandDynamicTablePropertyValuesMapping(ctx, definition.ValuesMapping)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.PropertyDefinition{
		Alignment:         OptionalEnumPointer(definition.Alignment, dashboardSchemaToProtoTextAlignment),
		ColumnDisplayName: definition.ColumnDisplayName.ValueStringPointer(),
		Link:              link,
		RegexExtract:      definition.RegexExtract.ValueStringPointer(),
		Thresholds:        thresholds,
		Units:             expandDynamicTablePropertyUnits(definition.Units),
		ValuesAlias:       definition.ValuesAlias.ValueStringPointer(),
		ValuesMapping:     valuesMapping,
	}, nil
}

func expandDynamicTablePropertyLink(ctx context.Context, link *DynamicTablePropertyLinkModel) (*dashboardservice.PropertyLinks, diag.Diagnostics) {
	if link == nil {
		return nil, nil
	}

	var models []DynamicTableLinkActionModel
	diags := link.Actions.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	actions := make([]dashboardservice.LinkAction, 0, len(models))
	for i := range models {
		actions = append(actions, dashboardservice.LinkAction{
			Id:                    ExpandDashboardUUID(models[i].ID),
			Name:                  models[i].Name.ValueStringPointer(),
			ShouldOpenInNewWindow: models[i].ShouldOpenInNewWindow.ValueBoolPointer(),
			Url:                   models[i].Url.ValueStringPointer(),
		})
	}

	return &dashboardservice.PropertyLinks{Actions: actions}, diags
}

func expandDynamicTablePropertyThresholds(ctx context.Context, thresholds *DynamicTablePropertyThresholdsModel) (*dashboardservice.PropertyThresholds, diag.Diagnostics) {
	if thresholds == nil {
		return nil, nil
	}

	values, diags := expandDynamicThresholds(ctx, thresholds.Values)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.PropertyThresholds{
		Max:    thresholds.Max.ValueFloat64Pointer(),
		Min:    thresholds.Min.ValueFloat64Pointer(),
		Type:   OptionalEnumPointer(thresholds.Type, DashboardSchemaToProtoThresholdType),
		Values: values,
	}, nil
}

func expandDynamicTablePropertyUnits(units *DynamicTablePropertyUnitsModel) *dashboardservice.PropertyUnits {
	if units == nil {
		return nil
	}

	return &dashboardservice.PropertyUnits{
		AllowAbbreviation: units.AllowAbbreviation.ValueBoolPointer(),
		CustomUnit:        units.CustomUnit.ValueStringPointer(),
		DecimalPrecision:  expandInt32Pointer(units.DecimalPrecision),
		Max:               units.Max.ValueFloat64Pointer(),
		Min:               units.Min.ValueFloat64Pointer(),
		Unit:              OptionalEnumPointer(units.Unit, DashboardSchemaToProtoUnit),
	}
}

func expandDynamicTablePropertyValuesMapping(ctx context.Context, valuesMapping *DynamicTablePropertyValuesMappingModel) (*dashboardservice.PropertyValuesMapping, diag.Diagnostics) {
	if valuesMapping == nil {
		return nil, nil
	}

	var models []DynamicTableValueMappingModel
	diags := valuesMapping.Mappings.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	mappings := make([]dashboardservice.PropertyValuesMappingValueMapping, 0, len(models))
	for i := range models {
		mappings = append(mappings, dashboardservice.PropertyValuesMappingValueMapping{
			InputValue:   models[i].InputValue.ValueStringPointer(),
			ReplaceValue: models[i].ReplaceValue.ValueStringPointer(),
			Type:         OptionalEnumPointer(models[i].Type, dashboardSchemaToProtoValuesMappingType),
		})
	}

	return &dashboardservice.PropertyValuesMapping{Mappings: mappings}, diags
}

func expandDynamicTableRuleScope(ctx context.Context, ruleScope *DynamicTableRuleScopeModel) (*dashboardservice.RuleScope, diag.Diagnostics) {
	if ruleScope == nil {
		return nil, nil
	}

	field, diags := ExpandObservationFieldObject(ctx, ruleScope.Field)
	if diags.HasError() {
		return nil, diags
	}

	return &dashboardservice.RuleScope{
		Field:     field,
		FieldType: OptionalEnumPointer(ruleScope.FieldType, dashboardSchemaToProtoFieldDataType),
		Regex:     ruleScope.Regex.ValueStringPointer(),
	}, nil
}

func expandDynamicTableSettings(ctx context.Context, settings *DynamicTableSettingsModel) (*dashboardservice.TableSettings, diag.Diagnostics) {
	if settings == nil {
		return nil, nil
	}

	var models []DynamicTableColumnWidthModel
	diags := settings.ColumnWidths.ElementsAs(ctx, &models, true)
	if diags.HasError() {
		return nil, diags
	}

	columnWidths := make([]dashboardservice.ColumnWidthEntry, 0, len(models))
	for i := range models {
		columnWidths = append(columnWidths, dashboardservice.ColumnWidthEntry{
			ColumnName: models[i].ColumnName.ValueStringPointer(),
			Width:      expandInt32Pointer(models[i].Width),
		})
	}

	return &dashboardservice.TableSettings{
		ColumnWidths: columnWidths,
		RowStyle:     OptionalEnumPointer(settings.RowStyle, DashboardRowStyleSchemaToProto),
	}, diags
}

func flattenDynamicTable(ctx context.Context, table *dashboardservice.Table) (*DynamicTableModel, diag.Diagnostics) {
	if table == nil {
		return nil, nil
	}

	columns, diags := flattenDynamicTableColumns(ctx, table.GetColumns())
	if diags.HasError() {
		return nil, diags
	}

	rules, diags := flattenDynamicTableRules(ctx, table.GetRules())
	if diags.HasError() {
		return nil, diags
	}

	settings, diags := flattenDynamicTableSettings(ctx, table.Settings)
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTableModel{
		Columns:  columns,
		Rules:    rules,
		Settings: settings,
	}, nil
}

func flattenDynamicTableColumns(ctx context.Context, columns []dashboardservice.TableColumn) (types.List, diag.Diagnostics) {
	if len(columns) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(columns))
	for i := range columns {
		field, dg := FlattenObservationField(ctx, columns[i].Field)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTableColumnModelAttr(), &DynamicTableColumnModel{Field: field})
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableColumnModelAttr()}, elements)
}

func flattenDynamicTableRules(ctx context.Context, rules []dashboardservice.TableRule) (types.List, diag.Diagnostics) {
	if len(rules) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(rules))
	for i := range rules {
		properties, dg := flattenDynamicTableProperties(ctx, rules[i].GetProperties())
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		ruleScope, dg := flattenDynamicTableRuleScope(ctx, rules[i].RuleScope)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		model := &DynamicTableRuleModel{
			Description: types.StringPointerValue(rules[i].Description),
			ID:          flattenDashboardUUID(rules[i].Id),
			Name:        types.StringPointerValue(rules[i].Name),
			Properties:  properties,
			RuleScope:   ruleScope,
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTableRuleModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableRuleModelAttr()}, elements)
}

func flattenDynamicTableProperties(ctx context.Context, properties []dashboardservice.Property) (types.List, diag.Diagnostics) {
	if len(properties) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()}), nil
	}

	var diagnostics diag.Diagnostics
	elements := make([]attr.Value, 0, len(properties))
	for i := range properties {
		definition, dg := flattenDynamicTablePropertyDefinition(ctx, properties[i].Definition)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		model := &DynamicTablePropertyModel{
			ID:         flattenDashboardUUID(properties[i].Id),
			Definition: definition,
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTablePropertyModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}

	if diagnostics.HasError() {
		return types.ListNull(types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()}), diagnostics
	}
	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTablePropertyModelAttr()}, elements)
}

func flattenDynamicTablePropertyDefinition(ctx context.Context, definition *dashboardservice.PropertyDefinition) (*DynamicTablePropertyDefinitionModel, diag.Diagnostics) {
	if definition == nil {
		return nil, nil
	}

	link, diags := flattenDynamicTablePropertyLink(ctx, definition.Link)
	if diags.HasError() {
		return nil, diags
	}

	thresholds, diags := flattenDynamicTablePropertyThresholds(ctx, definition.Thresholds)
	if diags.HasError() {
		return nil, diags
	}

	valuesMapping, diags := flattenDynamicTablePropertyValuesMapping(ctx, definition.ValuesMapping)
	if diags.HasError() {
		return nil, diags
	}

	flattened := &DynamicTablePropertyDefinitionModel{
		Alignment:         flattenOptionalEnum(definition.Alignment, dashboardProtoToSchemaTextAlignment),
		ColumnDisplayName: types.StringPointerValue(definition.ColumnDisplayName),
		Link:              link,
		RegexExtract:      types.StringPointerValue(definition.RegexExtract),
		Thresholds:        thresholds,
		Units:             flattenDynamicTablePropertyUnits(definition.Units),
		ValuesAlias:       types.StringPointerValue(definition.ValuesAlias),
		ValuesMapping:     valuesMapping,
	}

	// Same as the rule scope above: a definition with no arm selected must read
	// back as absent rather than as an object the validator refuses.
	if flattened.Alignment.IsNull() && flattened.ColumnDisplayName.IsNull() && flattened.Link == nil &&
		flattened.RegexExtract.IsNull() && flattened.Thresholds == nil && flattened.Units == nil &&
		flattened.ValuesAlias.IsNull() && flattened.ValuesMapping == nil {
		return nil, nil
	}

	return flattened, nil
}

func flattenDynamicTablePropertyLink(ctx context.Context, link *dashboardservice.PropertyLinks) (*DynamicTablePropertyLinkModel, diag.Diagnostics) {
	if link == nil {
		return nil, nil
	}

	actions := link.GetActions()
	if len(actions) == 0 {
		return &DynamicTablePropertyLinkModel{
			Actions: types.ListNull(types.ObjectType{AttrTypes: dynamicTableLinkActionModelAttr()}),
		}, nil
	}

	elements := make([]attr.Value, 0, len(actions))
	var diagnostics diag.Diagnostics
	for i := range actions {
		model := &DynamicTableLinkActionModel{
			ID:                    flattenDashboardUUID(actions[i].Id),
			Name:                  types.StringPointerValue(actions[i].Name),
			ShouldOpenInNewWindow: types.BoolPointerValue(actions[i].ShouldOpenInNewWindow),
			Url:                   types.StringPointerValue(actions[i].Url),
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTableLinkActionModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	list, dg := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableLinkActionModelAttr()}, elements)
	if dg.HasError() {
		return nil, dg
	}
	return &DynamicTablePropertyLinkModel{Actions: list}, nil
}

func flattenDynamicTablePropertyThresholds(ctx context.Context, thresholds *dashboardservice.PropertyThresholds) (*DynamicTablePropertyThresholdsModel, diag.Diagnostics) {
	if thresholds == nil {
		return nil, nil
	}

	values, diags := flattenDynamicThresholds(ctx, thresholds.GetValues())
	if diags.HasError() {
		return nil, diags
	}

	return &DynamicTablePropertyThresholdsModel{
		Max:    types.Float64PointerValue(thresholds.Max),
		Min:    types.Float64PointerValue(thresholds.Min),
		Type:   flattenOptionalEnum(thresholds.Type, DashboardProtoToSchemaThresholdType),
		Values: values,
	}, nil
}

func flattenDynamicTablePropertyUnits(units *dashboardservice.PropertyUnits) *DynamicTablePropertyUnitsModel {
	if units == nil {
		return nil
	}

	return &DynamicTablePropertyUnitsModel{
		AllowAbbreviation: types.BoolPointerValue(units.AllowAbbreviation),
		CustomUnit:        types.StringPointerValue(units.CustomUnit),
		DecimalPrecision:  flattenInt32Pointer(units.DecimalPrecision),
		Max:               types.Float64PointerValue(units.Max),
		Min:               types.Float64PointerValue(units.Min),
		Unit:              flattenOptionalEnum(units.Unit, DashboardProtoToSchemaUnit),
	}
}

func flattenDynamicTablePropertyValuesMapping(ctx context.Context, valuesMapping *dashboardservice.PropertyValuesMapping) (*DynamicTablePropertyValuesMappingModel, diag.Diagnostics) {
	if valuesMapping == nil {
		return nil, nil
	}

	mappings := valuesMapping.GetMappings()
	if len(mappings) == 0 {
		return &DynamicTablePropertyValuesMappingModel{
			Mappings: types.ListNull(types.ObjectType{AttrTypes: dynamicTableValueMappingModelAttr()}),
		}, nil
	}

	elements := make([]attr.Value, 0, len(mappings))
	var diagnostics diag.Diagnostics
	for i := range mappings {
		model := &DynamicTableValueMappingModel{
			InputValue:   types.StringPointerValue(mappings[i].InputValue),
			ReplaceValue: types.StringPointerValue(mappings[i].ReplaceValue),
			Type:         flattenOptionalEnum(mappings[i].Type, dashboardProtoToSchemaValuesMappingType),
		}
		element, dg := types.ObjectValueFrom(ctx, dynamicTableValueMappingModelAttr(), model)
		if dg.HasError() {
			diagnostics.Append(dg...)
			continue
		}
		elements = append(elements, element)
	}
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	list, dg := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableValueMappingModelAttr()}, elements)
	if dg.HasError() {
		return nil, dg
	}
	return &DynamicTablePropertyValuesMappingModel{Mappings: list}, nil
}

func flattenDynamicTableRuleScope(ctx context.Context, ruleScope *dashboardservice.RuleScope) (*DynamicTableRuleScopeModel, diag.Diagnostics) {
	if ruleScope == nil {
		return nil, nil
	}

	field, diags := FlattenObservationField(ctx, ruleScope.Field)
	if diags.HasError() {
		return nil, diags
	}

	scope := &DynamicTableRuleScopeModel{
		Field:     field,
		FieldType: flattenOptionalEnum(ruleScope.FieldType, dashboardProtoToSchemaFieldDataType),
		Regex:     types.StringPointerValue(ruleScope.Regex),
	}

	// The API stores a scope with no arm selected; keeping it as a present
	// object would flatten into config the exactly-one-of validator rejects.
	if scope.Field.IsNull() && scope.FieldType.IsNull() && scope.Regex.IsNull() {
		return nil, nil
	}

	return scope, nil
}

func flattenDynamicTableSettings(ctx context.Context, settings *dashboardservice.TableSettings) (*DynamicTableSettingsModel, diag.Diagnostics) {
	if settings == nil {
		return nil, nil
	}

	columnWidths := settings.GetColumnWidths()
	widths := types.ListNull(types.ObjectType{AttrTypes: dynamicTableColumnWidthModelAttr()})
	if len(columnWidths) > 0 {
		elements := make([]attr.Value, 0, len(columnWidths))
		var diagnostics diag.Diagnostics
		for i := range columnWidths {
			model := &DynamicTableColumnWidthModel{
				ColumnName: types.StringPointerValue(columnWidths[i].ColumnName),
				Width:      flattenInt32Pointer(columnWidths[i].Width),
			}
			element, dg := types.ObjectValueFrom(ctx, dynamicTableColumnWidthModelAttr(), model)
			if dg.HasError() {
				diagnostics.Append(dg...)
				continue
			}
			elements = append(elements, element)
		}
		if diagnostics.HasError() {
			return nil, diagnostics
		}
		list, dg := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: dynamicTableColumnWidthModelAttr()}, elements)
		if dg.HasError() {
			return nil, dg
		}
		widths = list
	}

	return &DynamicTableSettingsModel{
		ColumnWidths: widths,
		RowStyle:     flattenOptionalEnum(settings.RowStyle, DashboardRowStyleProtoToSchema),
	}, nil
}

var dashboardValidTextAlignment = utils.GetKeys(dashboardSchemaToProtoTextAlignment)

var dashboardValidValuesMappingType = utils.GetKeys(dashboardSchemaToProtoValuesMappingType)

var dashboardValidFieldDataType = utils.GetKeys(dashboardSchemaToProtoFieldDataType)

var dashboardSchemaToProtoTextAlignment = map[string]dashboardservice.TextAlignment{
	utils.UNSPECIFIED: dashboardservice.TEXTALIGNMENT_TEXT_ALIGNMENT_UNSPECIFIED,
	"left":            dashboardservice.TEXTALIGNMENT_TEXT_ALIGNMENT_LEFT,
	"center":          dashboardservice.TEXTALIGNMENT_TEXT_ALIGNMENT_CENTER,
	"right":           dashboardservice.TEXTALIGNMENT_TEXT_ALIGNMENT_RIGHT,
}

var dashboardSchemaToProtoValuesMappingType = map[string]dashboardservice.ValuesMappingType{
	utils.UNSPECIFIED: dashboardservice.VALUESMAPPINGTYPE_VALUES_MAPPING_TYPE_UNSPECIFIED,
	"value":           dashboardservice.VALUESMAPPINGTYPE_VALUES_MAPPING_TYPE_VALUE,
	"regex":           dashboardservice.VALUESMAPPINGTYPE_VALUES_MAPPING_TYPE_REGEX,
}

var dashboardSchemaToProtoFieldDataType = map[string]dashboardservice.FieldDataType{
	utils.UNSPECIFIED: dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_UNSPECIFIED,
	"number":          dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_NUMBER,
	"string":          dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_STRING,
	"boolean":         dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_BOOLEAN,
	"timestamp":       dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_TIMESTAMP,
	"object":          dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_OBJECT,
	"array":           dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_ARRAY,
	"regex":           dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_REGEX,
	"union":           dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_UNION,
	"enum":            dashboardservice.FIELDDATATYPE_FIELD_DATA_TYPE_ENUM,
}

func flattenDashboardUUID(id *dashboardservice.UUID) types.String {
	if id == nil {
		return types.StringNull()
	}
	return types.StringPointerValue(id.Value)
}

var dashboardProtoToSchemaTextAlignment = utils.ReverseMap(dashboardSchemaToProtoTextAlignment)

var dashboardProtoToSchemaValuesMappingType = utils.ReverseMap(dashboardSchemaToProtoValuesMappingType)

var dashboardProtoToSchemaFieldDataType = utils.ReverseMap(dashboardSchemaToProtoFieldDataType)
