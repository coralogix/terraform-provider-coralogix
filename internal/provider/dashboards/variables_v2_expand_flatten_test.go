package dashboards

import (
	"context"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestExpandFlattenVariablesV2StaticRoundTrip(t *testing.T) {
	ctx := context.Background()
	elementType := dashboardVariablesV2ElementType()

	sourceType := elementType.AttrTypes["source"].(types.ObjectType)
	valueType := elementType.AttrTypes["value"].(types.ObjectType)
	staticType := sourceType.AttrTypes["static"].(types.ObjectType)
	allOptionType := staticType.AttrTypes["all_option"].(types.ObjectType)
	valuesType := staticType.AttrTypes["values"].(types.ListType)
	valueEntryType := valuesType.ElemType.(types.ObjectType)
	singleStringType := valueType.AttrTypes["single_string"].(types.ObjectType)

	valueEntry, diags := types.ObjectValue(valueEntryType.AttrTypes, map[string]attr.Value{
		"value":      types.StringValue("production"),
		"label":      types.StringValue("production"),
		"is_default": types.BoolValue(true),
	})
	if diags.HasError() {
		t.Fatalf("value entry: %v", diags)
	}

	values, diags := types.ListValue(valueEntryType, []attr.Value{valueEntry})
	if diags.HasError() {
		t.Fatalf("values: %v", diags)
	}

	allOption, diags := types.ObjectValue(allOptionType.AttrTypes, map[string]attr.Value{
		"include_all": types.BoolValue(false),
		"label":       types.StringNull(),
	})
	if diags.HasError() {
		t.Fatalf("all option: %v", diags)
	}

	static, diags := types.ObjectValue(staticType.AttrTypes, map[string]attr.Value{
		"values_order_direction": types.StringValue("none"),
		"all_option":             allOption,
		"values":                 values,
	})
	if diags.HasError() {
		t.Fatalf("static: %v", diags)
	}

	source, diags := types.ObjectValue(sourceType.AttrTypes, map[string]attr.Value{
		"static":  static,
		"textbox": types.ObjectNull(sourceType.AttrTypes["textbox"].(types.ObjectType).AttrTypes),
		"query":   types.ObjectNull(sourceType.AttrTypes["query"].(types.ObjectType).AttrTypes),
	})
	if diags.HasError() {
		t.Fatalf("source: %v", diags)
	}

	singleString, diags := types.ObjectValue(singleStringType.AttrTypes, map[string]attr.Value{
		"value": types.StringValue("production"),
		"label": types.StringValue("production"),
	})
	if diags.HasError() {
		t.Fatalf("single string: %v", diags)
	}

	value, diags := types.ObjectValue(valueType.AttrTypes, map[string]attr.Value{
		"single_string":  singleString,
		"single_numeric": types.ObjectNull(valueType.AttrTypes["single_numeric"].(types.ObjectType).AttrTypes),
		"regex":          types.ObjectNull(valueType.AttrTypes["regex"].(types.ObjectType).AttrTypes),
		"lucene":         types.ObjectNull(valueType.AttrTypes["lucene"].(types.ObjectType).AttrTypes),
		"interval":       types.ObjectNull(valueType.AttrTypes["interval"].(types.ObjectType).AttrTypes),
		"multi_string":   types.ObjectNull(valueType.AttrTypes["multi_string"].(types.ObjectType).AttrTypes),
	})
	if diags.HasError() {
		t.Fatalf("value: %v", diags)
	}

	variable, diags := types.ObjectValue(elementType.AttrTypes, map[string]attr.Value{
		"id":               types.StringNull(),
		"name":             types.StringValue("environment"),
		"display_name":     types.StringValue("Environment"),
		"description":      types.StringNull(),
		"display_type":     types.StringValue("label_value"),
		"display_full_row": types.BoolNull(),
		"source":           source,
		"value":            value,
	})
	if diags.HasError() {
		t.Fatalf("variable: %v", diags)
	}

	list, diags := types.ListValue(elementType, []attr.Value{variable})
	if diags.HasError() {
		t.Fatalf("list: %v", diags)
	}

	expanded, expandDiags := expandDashboardVariablesV2(ctx, list)
	if expandDiags.HasError() {
		t.Fatalf("expand: %v", expandDiags)
	}
	if len(expanded) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(expanded))
	}
	if expanded[0].GetName() != "environment" {
		t.Fatalf("name = %q", expanded[0].GetName())
	}
	if expanded[0].Source == nil || expanded[0].Source.Static == nil {
		t.Fatal("expected static source")
	}
	if len(expanded[0].Source.Static.Values) != 1 || expanded[0].Source.Static.Values[0].GetLabel() != "production" {
		t.Fatalf("static values = %#v", expanded[0].Source.Static.Values)
	}
	if expanded[0].Value == nil || expanded[0].Value.SingleString == nil || expanded[0].Value.SingleString.Value == nil {
		t.Fatal("expected single string value")
	}
	if expanded[0].Value.SingleString.Value.GetValue() != "production" || expanded[0].Value.SingleString.Value.GetLabel() != "production" {
		t.Fatalf("single string = %#v", expanded[0].Value.SingleString.Value)
	}

	flattened, flattenDiags := flattenDashboardVariablesV2(ctx, expanded, list)
	if flattenDiags.HasError() {
		t.Fatalf("flatten: %v", flattenDiags)
	}
	var models []DashboardVariableV2Model
	if diags := flattened.ElementsAs(ctx, &models, false); diags.HasError() {
		t.Fatalf("elements: %v", diags)
	}
	if len(models) != 1 || models[0].Name.ValueString() != "environment" {
		t.Fatalf("flattened models = %#v", models)
	}
	if models[0].ID.IsNull() || models[0].ID.ValueString() == "" {
		t.Fatal("expected generated id after expand/flatten")
	}
}

func TestExpandFlattenVariablesV2StaticOmittedLabel(t *testing.T) {
	ctx := context.Background()
	elementType := dashboardVariablesV2ElementType()

	sourceType := elementType.AttrTypes["source"].(types.ObjectType)
	valueType := elementType.AttrTypes["value"].(types.ObjectType)
	staticType := sourceType.AttrTypes["static"].(types.ObjectType)
	allOptionType := staticType.AttrTypes["all_option"].(types.ObjectType)
	valuesType := staticType.AttrTypes["values"].(types.ListType)
	valueEntryType := valuesType.ElemType.(types.ObjectType)
	singleStringType := valueType.AttrTypes["single_string"].(types.ObjectType)

	valueEntry, diags := types.ObjectValue(valueEntryType.AttrTypes, map[string]attr.Value{
		"value":      types.StringValue("production"),
		"label":      types.StringNull(),
		"is_default": types.BoolValue(true),
	})
	if diags.HasError() {
		t.Fatalf("value entry: %v", diags)
	}
	customEntry, diags := types.ObjectValue(valueEntryType.AttrTypes, map[string]attr.Value{
		"value":      types.StringValue("staging"),
		"label":      types.StringValue("Staging"),
		"is_default": types.BoolNull(),
	})
	if diags.HasError() {
		t.Fatalf("custom entry: %v", diags)
	}
	values, diags := types.ListValue(valueEntryType, []attr.Value{valueEntry, customEntry})
	if diags.HasError() {
		t.Fatalf("values: %v", diags)
	}
	allOption, diags := types.ObjectValue(allOptionType.AttrTypes, map[string]attr.Value{
		"include_all": types.BoolValue(false),
		"label":       types.StringNull(),
	})
	if diags.HasError() {
		t.Fatalf("all option: %v", diags)
	}
	static, diags := types.ObjectValue(staticType.AttrTypes, map[string]attr.Value{
		"values_order_direction": types.StringValue("none"),
		"all_option":             allOption,
		"values":                 values,
	})
	if diags.HasError() {
		t.Fatalf("static: %v", diags)
	}
	source, diags := types.ObjectValue(sourceType.AttrTypes, map[string]attr.Value{
		"static":  static,
		"textbox": types.ObjectNull(sourceType.AttrTypes["textbox"].(types.ObjectType).AttrTypes),
		"query":   types.ObjectNull(sourceType.AttrTypes["query"].(types.ObjectType).AttrTypes),
	})
	if diags.HasError() {
		t.Fatalf("source: %v", diags)
	}
	singleString, diags := types.ObjectValue(singleStringType.AttrTypes, map[string]attr.Value{
		"value": types.StringValue("production"),
		"label": types.StringValue("production"),
	})
	if diags.HasError() {
		t.Fatalf("single string: %v", diags)
	}
	value, diags := types.ObjectValue(valueType.AttrTypes, map[string]attr.Value{
		"single_string":  singleString,
		"single_numeric": types.ObjectNull(valueType.AttrTypes["single_numeric"].(types.ObjectType).AttrTypes),
		"regex":          types.ObjectNull(valueType.AttrTypes["regex"].(types.ObjectType).AttrTypes),
		"lucene":         types.ObjectNull(valueType.AttrTypes["lucene"].(types.ObjectType).AttrTypes),
		"interval":       types.ObjectNull(valueType.AttrTypes["interval"].(types.ObjectType).AttrTypes),
		"multi_string":   types.ObjectNull(valueType.AttrTypes["multi_string"].(types.ObjectType).AttrTypes),
	})
	if diags.HasError() {
		t.Fatalf("value: %v", diags)
	}
	variable, diags := types.ObjectValue(elementType.AttrTypes, map[string]attr.Value{
		"id":               types.StringNull(),
		"name":             types.StringValue("environment"),
		"display_name":     types.StringValue("Environment"),
		"description":      types.StringNull(),
		"display_type":     types.StringValue("label_value"),
		"display_full_row": types.BoolNull(),
		"source":           source,
		"value":            value,
	})
	if diags.HasError() {
		t.Fatalf("variable: %v", diags)
	}
	list, diags := types.ListValue(elementType, []attr.Value{variable})
	if diags.HasError() {
		t.Fatalf("list: %v", diags)
	}

	expanded, expandDiags := expandDashboardVariablesV2(ctx, list)
	if expandDiags.HasError() {
		t.Fatalf("expand: %v", expandDiags)
	}
	gotValues := expanded[0].Source.Static.Values
	if len(gotValues) != 2 {
		t.Fatalf("values len = %d", len(gotValues))
	}
	if gotValues[0].GetLabel() != "production" {
		t.Fatalf("omitted label expand = %q, want production", gotValues[0].GetLabel())
	}
	if gotValues[1].GetLabel() != "Staging" {
		t.Fatalf("custom label expand = %q, want Staging", gotValues[1].GetLabel())
	}

	flattened, flattenDiags := flattenDashboardVariablesV2(ctx, expanded, list)
	if flattenDiags.HasError() {
		t.Fatalf("flatten: %v", flattenDiags)
	}
	var models []DashboardVariableV2Model
	if diags := flattened.ElementsAs(ctx, &models, false); diags.HasError() {
		t.Fatalf("elements: %v", diags)
	}
	var sourceModel variableSourceV2Model
	if diags := models[0].Source.As(ctx, &sourceModel, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("source: %v", diags)
	}
	if sourceModel.Static == nil {
		t.Fatal("expected static source on flattened model")
	}
	var flatValues []staticValueV2Model
	if diags := sourceModel.Static.Values.ElementsAs(ctx, &flatValues, false); diags.HasError() {
		t.Fatalf("flat values: %v", diags)
	}
	if len(flatValues) != 2 {
		t.Fatalf("flat values len = %d", len(flatValues))
	}
	if !flatValues[0].Label.Equal(types.StringValue("production")) {
		t.Fatalf("omitted label should flatten to value, got %#v", flatValues[0].Label)
	}
	if flatValues[1].Label.ValueString() != "Staging" {
		t.Fatalf("custom label flatten = %q, want Staging", flatValues[1].Label.ValueString())
	}
}

func TestFlattenMultiStringListValuesV2KeepsEmptyList(t *testing.T) {
	ctx := context.Background()
	elementType := dashboardVariablesV2ElementType()
	valueType := elementType.AttrTypes["value"].(types.ObjectType)
	multiType := valueType.AttrTypes["multi_string"].(types.ObjectType)
	listType := multiType.AttrTypes["list"].(types.ObjectType)
	valuesType := listType.AttrTypes["values"].(types.ListType)

	flattened, diags := flattenMultiStringListValuesV2(ctx, nil, valuesType)
	if diags.HasError() {
		t.Fatalf("flatten: %v", diags)
	}
	if flattened.IsNull() || flattened.IsUnknown() || len(flattened.Elements()) != 0 {
		t.Fatalf("expected known empty list, got %#v", flattened)
	}
}

func TestExpandVariableV2OptionalEnumsOmitUnset(t *testing.T) {
	ctx := context.Background()
	elementType := dashboardVariablesV2ElementType()
	sourceType := elementType.AttrTypes["source"].(types.ObjectType)
	valueType := elementType.AttrTypes["value"].(types.ObjectType)
	textboxType := sourceType.AttrTypes["textbox"].(types.ObjectType)
	singleStringType := valueType.AttrTypes["single_string"].(types.ObjectType)

	textbox, diags := types.ObjectValue(textboxType.AttrTypes, map[string]attr.Value{
		"default_value": types.ObjectNull(textboxType.AttrTypes["default_value"].(types.ObjectType).AttrTypes),
	})
	if diags.HasError() {
		t.Fatalf("textbox: %v", diags)
	}
	source, diags := types.ObjectValue(sourceType.AttrTypes, map[string]attr.Value{
		"static":  types.ObjectNull(sourceType.AttrTypes["static"].(types.ObjectType).AttrTypes),
		"textbox": textbox,
		"query":   types.ObjectNull(sourceType.AttrTypes["query"].(types.ObjectType).AttrTypes),
	})
	if diags.HasError() {
		t.Fatalf("source: %v", diags)
	}
	singleString, diags := types.ObjectValue(singleStringType.AttrTypes, map[string]attr.Value{
		"value": types.StringValue("hello"),
		"label": types.StringValue("hello"),
	})
	if diags.HasError() {
		t.Fatalf("single string: %v", diags)
	}
	value, diags := types.ObjectValue(valueType.AttrTypes, map[string]attr.Value{
		"single_string":  singleString,
		"single_numeric": types.ObjectNull(valueType.AttrTypes["single_numeric"].(types.ObjectType).AttrTypes),
		"regex":          types.ObjectNull(valueType.AttrTypes["regex"].(types.ObjectType).AttrTypes),
		"lucene":         types.ObjectNull(valueType.AttrTypes["lucene"].(types.ObjectType).AttrTypes),
		"interval":       types.ObjectNull(valueType.AttrTypes["interval"].(types.ObjectType).AttrTypes),
		"multi_string":   types.ObjectNull(valueType.AttrTypes["multi_string"].(types.ObjectType).AttrTypes),
	})
	if diags.HasError() {
		t.Fatalf("value: %v", diags)
	}

	expanded, expandDiags := expandDashboardVariableV2(ctx, DashboardVariableV2Model{
		ID:          types.StringNull(),
		Name:        types.StringValue("search"),
		DisplayName: types.StringValue("Search"),
		DisplayType: types.StringNull(),
		Source:      source,
		Value:       value,
	})
	if expandDiags.HasError() {
		t.Fatalf("expand: %v", expandDiags)
	}
	if expanded.DisplayType != nil {
		t.Fatalf("null display_type should omit API enum, got %q", *expanded.DisplayType)
	}
}

func TestFlattenVariableV2DisplayTypeUnspecifiedIsNull(t *testing.T) {
	ctx := context.Background()
	elementType := dashboardVariablesV2ElementType()
	unspecified := dashboardservice.VARIABLEDISPLAYTYPEV2_VARIABLE_DISPLAY_TYPE_V2_UNSPECIFIED
	name := "search"
	displayName := "Search"
	hello := "hello"

	flattened, diags := flattenDashboardVariableV2(ctx, &dashboardservice.VariableV2{
		Name:        &name,
		DisplayName: &displayName,
		DisplayType: &unspecified,
		Source: &dashboardservice.VariableSourceV2{
			Textbox: &dashboardservice.TextboxSource{},
		},
		Value: &dashboardservice.VariableValueV2{
			SingleString: &dashboardservice.SingleStringValue{
				Value: &dashboardservice.StringValueLabel{Value: &hello},
			},
		},
	}, elementType)
	if diags.HasError() {
		t.Fatalf("flatten: %v", diags)
	}

	attrs := flattened.Attributes()
	displayType, ok := attrs["display_type"].(types.String)
	if !ok {
		t.Fatalf("display_type type = %T, want types.String", attrs["display_type"])
	}
	if !displayType.IsNull() {
		t.Fatalf("UNSPECIFIED display_type should flatten to null, got %q", displayType.ValueString())
	}
}

func TestExpandFlattenVariablesV2QueryWireShapes(t *testing.T) {
	ctx := context.Background()
	elementType := dashboardVariablesV2ElementType()
	sourceType := elementType.AttrTypes["source"].(types.ObjectType)
	valueType := elementType.AttrTypes["value"].(types.ObjectType)
	queryType := sourceType.AttrTypes["query"].(types.ObjectType)
	allOptionType := queryType.AttrTypes["all_option"].(types.ObjectType)
	displayOptionsType := queryType.AttrTypes["value_display_options"].(types.ObjectType)
	metricsType := queryType.AttrTypes["metrics_query"].(types.ObjectType)
	metricsInnerType := metricsType.AttrTypes["type"].(types.ObjectType)
	promqlType := metricsInnerType.AttrTypes["promql_query"].(types.ObjectType)
	dataprimeType := queryType.AttrTypes["dataprime_query"].(types.ObjectType)
	dataprimeInnerType := dataprimeType.AttrTypes["type"].(types.ObjectType)
	queryTextType := dataprimeInnerType.AttrTypes["query_text"].(types.ObjectType)
	multiType := valueType.AttrTypes["multi_string"].(types.ObjectType)
	listType := multiType.AttrTypes["list"].(types.ObjectType)
	listValuesType := listType.AttrTypes["values"].(types.ListType)
	listEntryType := listValuesType.ElemType.(types.ObjectType)
	spansType := queryType.AttrTypes["spans_query"].(types.ObjectType)
	spansInnerType := spansType.AttrTypes["type"].(types.ObjectType)
	spansFieldValueType := spansInnerType.AttrTypes["field_value"].(types.ObjectType)
	spanValueType := spansFieldValueType.AttrTypes["value"].(types.ObjectType)
	stringOrVarType := metricsInnerType.AttrTypes["label_value"].(types.ObjectType).AttrTypes["metric_name"].(types.ObjectType)
	filtersType := metricsInnerType.AttrTypes["label_value"].(types.ObjectType).AttrTypes["label_filters"].(types.ListType)
	filterEntryType := filtersType.ElemType.(types.ObjectType)
	operatorType := filterEntryType.AttrTypes["operator"].(types.ObjectType)

	allOption, diags := types.ObjectValue(allOptionType.AttrTypes, map[string]attr.Value{
		"include_all": types.BoolValue(true),
		"label":       types.StringNull(),
	})
	if diags.HasError() {
		t.Fatalf("all option: %v", diags)
	}

	t.Run("promql_and_empty_multi_list", func(t *testing.T) {
		promql, diags := types.ObjectValue(promqlType.AttrTypes, map[string]attr.Value{
			"query":             types.StringValue("vector(1)"),
			"promql_query_type": types.StringValue("instant"),
		})
		if diags.HasError() {
			t.Fatalf("promql: %v", diags)
		}
		metricsInner, diags := types.ObjectValue(metricsInnerType.AttrTypes, map[string]attr.Value{
			"metric_name":  types.ObjectNull(metricsInnerType.AttrTypes["metric_name"].(types.ObjectType).AttrTypes),
			"label_name":   types.ObjectNull(metricsInnerType.AttrTypes["label_name"].(types.ObjectType).AttrTypes),
			"label_value":  types.ObjectNull(metricsInnerType.AttrTypes["label_value"].(types.ObjectType).AttrTypes),
			"promql_query": promql,
		})
		if diags.HasError() {
			t.Fatalf("metrics type: %v", diags)
		}
		metrics, diags := types.ObjectValue(metricsType.AttrTypes, map[string]attr.Value{"type": metricsInner})
		if diags.HasError() {
			t.Fatalf("metrics: %v", diags)
		}
		query, diags := types.ObjectValue(queryType.AttrTypes, map[string]attr.Value{
			"values_order_direction": types.StringValue("asc"),
			"all_option":             allOption,
			"refresh_strategy":       types.StringValue("on_dashboard_load"),
			"value_display_options":  types.ObjectNull(displayOptionsType.AttrTypes),
			"logs_query":             types.ObjectNull(queryType.AttrTypes["logs_query"].(types.ObjectType).AttrTypes),
			"spans_query":            types.ObjectNull(queryType.AttrTypes["spans_query"].(types.ObjectType).AttrTypes),
			"metrics_query":          metrics,
			"dataprime_query":        types.ObjectNull(queryType.AttrTypes["dataprime_query"].(types.ObjectType).AttrTypes),
		})
		if diags.HasError() {
			t.Fatalf("query: %v", diags)
		}
		emptyListValues, diags := types.ListValue(listEntryType, []attr.Value{})
		if diags.HasError() {
			t.Fatalf("empty list values: %v", diags)
		}
		listObj, diags := types.ObjectValue(listType.AttrTypes, map[string]attr.Value{"values": emptyListValues})
		if diags.HasError() {
			t.Fatalf("list: %v", diags)
		}
		multi, diags := types.ObjectValue(multiType.AttrTypes, map[string]attr.Value{
			"selected_all": types.ObjectNull(multiType.AttrTypes["selected_all"].(types.ObjectType).AttrTypes),
			"all":          types.ObjectNull(multiType.AttrTypes["all"].(types.ObjectType).AttrTypes),
			"list":         listObj,
		})
		if diags.HasError() {
			t.Fatalf("multi: %v", diags)
		}
		value, diags := types.ObjectValue(valueType.AttrTypes, map[string]attr.Value{
			"single_string":  types.ObjectNull(valueType.AttrTypes["single_string"].(types.ObjectType).AttrTypes),
			"single_numeric": types.ObjectNull(valueType.AttrTypes["single_numeric"].(types.ObjectType).AttrTypes),
			"regex":          types.ObjectNull(valueType.AttrTypes["regex"].(types.ObjectType).AttrTypes),
			"lucene":         types.ObjectNull(valueType.AttrTypes["lucene"].(types.ObjectType).AttrTypes),
			"interval":       types.ObjectNull(valueType.AttrTypes["interval"].(types.ObjectType).AttrTypes),
			"multi_string":   multi,
		})
		if diags.HasError() {
			t.Fatalf("value: %v", diags)
		}
		source, diags := types.ObjectValue(sourceType.AttrTypes, map[string]attr.Value{
			"static":  types.ObjectNull(sourceType.AttrTypes["static"].(types.ObjectType).AttrTypes),
			"textbox": types.ObjectNull(sourceType.AttrTypes["textbox"].(types.ObjectType).AttrTypes),
			"query":   query,
		})
		if diags.HasError() {
			t.Fatalf("source: %v", diags)
		}
		variable, diags := types.ObjectValue(elementType.AttrTypes, map[string]attr.Value{
			"id":               types.StringNull(),
			"name":             types.StringValue("prom"),
			"display_name":     types.StringValue("Prom"),
			"description":      types.StringNull(),
			"display_type":     types.StringValue("label_value"),
			"display_full_row": types.BoolNull(),
			"source":           source,
			"value":            value,
		})
		if diags.HasError() {
			t.Fatalf("variable: %v", diags)
		}
		list, diags := types.ListValue(elementType, []attr.Value{variable})
		if diags.HasError() {
			t.Fatalf("list: %v", diags)
		}

		expanded, expandDiags := expandDashboardVariablesV2(ctx, list)
		if expandDiags.HasError() {
			t.Fatalf("expand: %v", expandDiags)
		}
		got := expanded[0]
		if got.Source.Query.ValueDisplayOptions != nil {
			t.Fatalf("omitted display options should expand nil, got %#v", got.Source.Query.ValueDisplayOptions)
		}
		promqlQuery := got.Source.Query.MetricsQuery.Type.PromqlQuery
		if promqlQuery == nil || promqlQuery.Query == nil || promqlQuery.Query.GetValue() != "vector(1)" {
			t.Fatalf("promql wrapper = %#v", promqlQuery)
		}
		if got.Value.MultiString == nil || got.Value.MultiString.List == nil {
			t.Fatal("expected multi_string.list")
		}
		if got.Value.MultiString.List.Values == nil {
			t.Fatal("expected non-nil empty values slice")
		}
		if len(got.Value.MultiString.List.Values) != 0 {
			t.Fatalf("values len = %d", len(got.Value.MultiString.List.Values))
		}

		flattened, flattenDiags := flattenDashboardVariablesV2(ctx, expanded, list)
		if flattenDiags.HasError() {
			t.Fatalf("flatten: %v", flattenDiags)
		}
		var models []DashboardVariableV2Model
		if diags := flattened.ElementsAs(ctx, &models, false); diags.HasError() {
			t.Fatalf("elements: %v", diags)
		}
		var sourceModel variableSourceV2Model
		if diags := models[0].Source.As(ctx, &sourceModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			t.Fatalf("source: %v", diags)
		}
		if sourceModel.Query == nil || sourceModel.Query.ValueDisplayOptions != nil {
			t.Fatalf("flattened display options should stay unset, got %#v", sourceModel.Query)
		}
		var valueModel variableValueV2Model
		if diags := models[0].Value.As(ctx, &valueModel, basetypes.ObjectAsOptions{}); diags.HasError() {
			t.Fatalf("value: %v", diags)
		}
		if valueModel.MultiString == nil || valueModel.MultiString.List == nil {
			t.Fatal("expected flattened multi_string.list")
		}
		if valueModel.MultiString.List.Values.IsNull() || len(valueModel.MultiString.List.Values.Elements()) != 0 {
			t.Fatalf("flattened empty list should stay known empty, got %#v", valueModel.MultiString.List.Values)
		}
	})

	t.Run("dataprime_query", func(t *testing.T) {
		queryText, diags := types.ObjectValue(queryTextType.AttrTypes, map[string]attr.Value{
			"query":          types.StringValue("source logs | limit 10"),
			"data_mode_type": types.StringValue("high"),
		})
		if diags.HasError() {
			t.Fatalf("query text: %v", diags)
		}
		dataprimeInner, diags := types.ObjectValue(dataprimeInnerType.AttrTypes, map[string]attr.Value{"query_text": queryText})
		if diags.HasError() {
			t.Fatalf("dataprime type: %v", diags)
		}
		dataprime, diags := types.ObjectValue(dataprimeType.AttrTypes, map[string]attr.Value{"type": dataprimeInner})
		if diags.HasError() {
			t.Fatalf("dataprime: %v", diags)
		}
		query, diags := types.ObjectValue(queryType.AttrTypes, map[string]attr.Value{
			"values_order_direction": types.StringValue("asc"),
			"all_option":             allOption,
			"refresh_strategy":       types.StringNull(),
			"value_display_options":  types.ObjectNull(displayOptionsType.AttrTypes),
			"logs_query":             types.ObjectNull(queryType.AttrTypes["logs_query"].(types.ObjectType).AttrTypes),
			"spans_query":            types.ObjectNull(queryType.AttrTypes["spans_query"].(types.ObjectType).AttrTypes),
			"metrics_query":          types.ObjectNull(metricsType.AttrTypes),
			"dataprime_query":        dataprime,
		})
		if diags.HasError() {
			t.Fatalf("query: %v", diags)
		}
		selectedAll, diags := types.ObjectValue(multiType.AttrTypes["selected_all"].(types.ObjectType).AttrTypes, map[string]attr.Value{})
		if diags.HasError() {
			t.Fatalf("selected all: %v", diags)
		}
		multi, diags := types.ObjectValue(multiType.AttrTypes, map[string]attr.Value{
			"selected_all": selectedAll,
			"all":          types.ObjectNull(multiType.AttrTypes["all"].(types.ObjectType).AttrTypes),
			"list":         types.ObjectNull(listType.AttrTypes),
		})
		if diags.HasError() {
			t.Fatalf("multi: %v", diags)
		}
		value, diags := types.ObjectValue(valueType.AttrTypes, map[string]attr.Value{
			"single_string":  types.ObjectNull(valueType.AttrTypes["single_string"].(types.ObjectType).AttrTypes),
			"single_numeric": types.ObjectNull(valueType.AttrTypes["single_numeric"].(types.ObjectType).AttrTypes),
			"regex":          types.ObjectNull(valueType.AttrTypes["regex"].(types.ObjectType).AttrTypes),
			"lucene":         types.ObjectNull(valueType.AttrTypes["lucene"].(types.ObjectType).AttrTypes),
			"interval":       types.ObjectNull(valueType.AttrTypes["interval"].(types.ObjectType).AttrTypes),
			"multi_string":   multi,
		})
		if diags.HasError() {
			t.Fatalf("value: %v", diags)
		}
		source, diags := types.ObjectValue(sourceType.AttrTypes, map[string]attr.Value{
			"static":  types.ObjectNull(sourceType.AttrTypes["static"].(types.ObjectType).AttrTypes),
			"textbox": types.ObjectNull(sourceType.AttrTypes["textbox"].(types.ObjectType).AttrTypes),
			"query":   query,
		})
		if diags.HasError() {
			t.Fatalf("source: %v", diags)
		}
		variable, diags := types.ObjectValue(elementType.AttrTypes, map[string]attr.Value{
			"id":               types.StringNull(),
			"name":             types.StringValue("dp"),
			"display_name":     types.StringValue("DP"),
			"description":      types.StringNull(),
			"display_type":     types.StringValue("label_value"),
			"display_full_row": types.BoolNull(),
			"source":           source,
			"value":            value,
		})
		if diags.HasError() {
			t.Fatalf("variable: %v", diags)
		}
		list, diags := types.ListValue(elementType, []attr.Value{variable})
		if diags.HasError() {
			t.Fatalf("list: %v", diags)
		}

		expanded, expandDiags := expandDashboardVariablesV2(ctx, list)
		if expandDiags.HasError() {
			t.Fatalf("expand: %v", expandDiags)
		}
		queryTextExpanded := expanded[0].Source.Query.DataprimeQuery.Type.QueryText
		if queryTextExpanded == nil || queryTextExpanded.Query == nil || queryTextExpanded.Query.GetText() != "source logs | limit 10" {
			t.Fatalf("dataprime wrapper = %#v", queryTextExpanded)
		}
		if expanded[0].Value.MultiString == nil || expanded[0].Value.MultiString.SelectedAll == nil {
			t.Fatalf("selected_all wire = %#v", expanded[0].Value.MultiString)
		}
	})

	t.Run("spans_and_metrics_operator", func(t *testing.T) {
		spanValue, diags := types.ObjectValue(spanValueType.AttrTypes, map[string]attr.Value{
			"type":  types.StringValue("metadata"),
			"value": types.StringValue("service_name"),
		})
		if diags.HasError() {
			t.Fatalf("span value: %v", diags)
		}
		spansFieldValue, diags := types.ObjectValue(spansFieldValueType.AttrTypes, map[string]attr.Value{
			"value":             spanValue,
			"observation_field": types.ObjectNull(spansFieldValueType.AttrTypes["observation_field"].(types.ObjectType).AttrTypes),
		})
		if diags.HasError() {
			t.Fatalf("spans field value: %v", diags)
		}
		spansInner, diags := types.ObjectValue(spansInnerType.AttrTypes, map[string]attr.Value{
			"field_name":  types.ObjectNull(spansInnerType.AttrTypes["field_name"].(types.ObjectType).AttrTypes),
			"field_value": spansFieldValue,
		})
		if diags.HasError() {
			t.Fatalf("spans type: %v", diags)
		}
		spans, diags := types.ObjectValue(spansType.AttrTypes, map[string]attr.Value{"type": spansInner})
		if diags.HasError() {
			t.Fatalf("spans: %v", diags)
		}

		metricName, diags := types.ObjectValue(stringOrVarType.AttrTypes, map[string]attr.Value{
			"string_value":  types.StringValue("http_requests_total"),
			"variable_name": types.StringNull(),
		})
		if diags.HasError() {
			t.Fatalf("metric name: %v", diags)
		}
		labelName, diags := types.ObjectValue(stringOrVarType.AttrTypes, map[string]attr.Value{
			"string_value":  types.StringValue("env"),
			"variable_name": types.StringNull(),
		})
		if diags.HasError() {
			t.Fatalf("label name: %v", diags)
		}
		selectedValue, diags := types.ObjectValue(stringOrVarType.AttrTypes, map[string]attr.Value{
			"string_value":  types.StringValue("prod"),
			"variable_name": types.StringNull(),
		})
		if diags.HasError() {
			t.Fatalf("selected value: %v", diags)
		}
		selectedValues, diags := types.ListValue(stringOrVarType, []attr.Value{selectedValue})
		if diags.HasError() {
			t.Fatalf("selected values: %v", diags)
		}
		operator, diags := types.ObjectValue(operatorType.AttrTypes, map[string]attr.Value{
			"type":            types.StringValue("equals"),
			"selected_values": selectedValues,
		})
		if diags.HasError() {
			t.Fatalf("operator: %v", diags)
		}
		filterMetric, diags := types.ObjectValue(stringOrVarType.AttrTypes, map[string]attr.Value{
			"string_value":  types.StringValue("http_requests_total"),
			"variable_name": types.StringNull(),
		})
		if diags.HasError() {
			t.Fatalf("filter metric: %v", diags)
		}
		filterLabel, diags := types.ObjectValue(stringOrVarType.AttrTypes, map[string]attr.Value{
			"string_value":  types.StringValue("region"),
			"variable_name": types.StringNull(),
		})
		if diags.HasError() {
			t.Fatalf("filter label: %v", diags)
		}
		filter, diags := types.ObjectValue(filterEntryType.AttrTypes, map[string]attr.Value{
			"metric":   filterMetric,
			"label":    filterLabel,
			"operator": operator,
		})
		if diags.HasError() {
			t.Fatalf("filter: %v", diags)
		}
		emptyFilters, diags := types.ListValue(filterEntryType, []attr.Value{})
		if diags.HasError() {
			t.Fatalf("empty filters: %v", diags)
		}

		// Spans variable
		querySpans, diags := types.ObjectValue(queryType.AttrTypes, map[string]attr.Value{
			"values_order_direction": types.StringValue("asc"),
			"all_option":             allOption,
			"refresh_strategy":       types.StringNull(),
			"value_display_options":  types.ObjectNull(displayOptionsType.AttrTypes),
			"logs_query":             types.ObjectNull(queryType.AttrTypes["logs_query"].(types.ObjectType).AttrTypes),
			"spans_query":            spans,
			"metrics_query":          types.ObjectNull(metricsType.AttrTypes),
			"dataprime_query":        types.ObjectNull(dataprimeType.AttrTypes),
		})
		if diags.HasError() {
			t.Fatalf("spans query: %v", diags)
		}
		selectedAll, diags := types.ObjectValue(multiType.AttrTypes["selected_all"].(types.ObjectType).AttrTypes, map[string]attr.Value{})
		if diags.HasError() {
			t.Fatalf("selected all: %v", diags)
		}
		multi, diags := types.ObjectValue(multiType.AttrTypes, map[string]attr.Value{
			"selected_all": selectedAll,
			"all":          types.ObjectNull(multiType.AttrTypes["all"].(types.ObjectType).AttrTypes),
			"list":         types.ObjectNull(listType.AttrTypes),
		})
		if diags.HasError() {
			t.Fatalf("multi: %v", diags)
		}
		value, diags := types.ObjectValue(valueType.AttrTypes, map[string]attr.Value{
			"single_string":  types.ObjectNull(valueType.AttrTypes["single_string"].(types.ObjectType).AttrTypes),
			"single_numeric": types.ObjectNull(valueType.AttrTypes["single_numeric"].(types.ObjectType).AttrTypes),
			"regex":          types.ObjectNull(valueType.AttrTypes["regex"].(types.ObjectType).AttrTypes),
			"lucene":         types.ObjectNull(valueType.AttrTypes["lucene"].(types.ObjectType).AttrTypes),
			"interval":       types.ObjectNull(valueType.AttrTypes["interval"].(types.ObjectType).AttrTypes),
			"multi_string":   multi,
		})
		if diags.HasError() {
			t.Fatalf("value: %v", diags)
		}
		sourceSpans, diags := types.ObjectValue(sourceType.AttrTypes, map[string]attr.Value{
			"static":  types.ObjectNull(sourceType.AttrTypes["static"].(types.ObjectType).AttrTypes),
			"textbox": types.ObjectNull(sourceType.AttrTypes["textbox"].(types.ObjectType).AttrTypes),
			"query":   querySpans,
		})
		if diags.HasError() {
			t.Fatalf("source spans: %v", diags)
		}
		variableSpans, diags := types.ObjectValue(elementType.AttrTypes, map[string]attr.Value{
			"id":               types.StringNull(),
			"name":             types.StringValue("span"),
			"display_name":     types.StringValue("Span"),
			"description":      types.StringNull(),
			"display_type":     types.StringValue("label_value"),
			"display_full_row": types.BoolNull(),
			"source":           sourceSpans,
			"value":            value,
		})
		if diags.HasError() {
			t.Fatalf("variable spans: %v", diags)
		}
		listSpans, diags := types.ListValue(elementType, []attr.Value{variableSpans})
		if diags.HasError() {
			t.Fatalf("list spans: %v", diags)
		}
		expandedSpans, expandDiags := expandDashboardVariablesV2(ctx, listSpans)
		if expandDiags.HasError() {
			t.Fatalf("expand spans: %v", expandDiags)
		}
		spanField := expandedSpans[0].Source.Query.SpansQuery.Type.FieldValue.Value
		if spanField == nil || spanField.MetadataField == nil {
			t.Fatalf("expected spans metadata field, got %#v", spanField)
		}

		// Metrics label_value + empty filters + operator expand helper
		labelValueExpanded, labelDiags := expandMetricsLabelValueV2(ctx, &metricsLabelValueV2Model{
			MetricName:   metricName,
			LabelName:    labelName,
			LabelFilters: emptyFilters,
		})
		if labelDiags.HasError() {
			t.Fatalf("expand label value: %v", labelDiags)
		}
		if labelValueExpanded.LabelFilters != nil {
			t.Fatalf("empty label_filters should expand nil, got %#v", labelValueExpanded.LabelFilters)
		}

		filtersWithOp, diags := types.ListValue(filterEntryType, []attr.Value{filter})
		if diags.HasError() {
			t.Fatalf("filters with op: %v", diags)
		}
		labelValueWithOp, labelDiags := expandMetricsLabelValueV2(ctx, &metricsLabelValueV2Model{
			MetricName:   metricName,
			LabelName:    labelName,
			LabelFilters: filtersWithOp,
		})
		if labelDiags.HasError() {
			t.Fatalf("expand label value with op: %v", labelDiags)
		}
		if len(labelValueWithOp.LabelFilters) != 1 || labelValueWithOp.LabelFilters[0].Operator == nil || labelValueWithOp.LabelFilters[0].Operator.Equals == nil {
			t.Fatalf("metrics operator wire = %#v", labelValueWithOp.LabelFilters)
		}
		selection := labelValueWithOp.LabelFilters[0].Operator.Equals.Selection
		if selection == nil || selection.List == nil || len(selection.List.Values) != 1 || selection.List.Values[0].GetStringValue() != "prod" {
			t.Fatalf("metrics selection list = %#v", selection)
		}
	})
}

func TestFloat32PointerToTypeFloat64PreservesConfiguredFractions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   float64
	}{
		{name: "tenth", in: 0.1},
		{name: "one_point_one", in: 1.1},
		{name: "exact_half", in: 0.5},
		{name: "exact_quarter", in: 0.25},
		{name: "integer", in: 42},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			expanded := typeFloat64ToFloat32Pointer(types.Float64Value(tc.in))
			got := float32PointerToTypeFloat64(expanded)
			if got.IsNull() || got.IsUnknown() {
				t.Fatalf("got null/unknown, want %v", tc.in)
			}
			if got.ValueFloat64() != tc.in {
				t.Fatalf("got %v, want %v (raw widen would be %v)", got.ValueFloat64(), tc.in, float64(*expanded))
			}
		})
	}

	if !float32PointerToTypeFloat64(nil).IsNull() {
		t.Fatal("nil pointer should flatten to null")
	}
}

func TestExpandFlattenSingleNumericFractionalRoundTrip(t *testing.T) {
	t.Parallel()

	elementType := dashboardVariablesV2ElementType()
	valueType := elementType.AttrTypes["value"].(types.ObjectType)
	numericType := valueType.AttrTypes["single_numeric"].(types.ObjectType)

	model := &numericValueLabelFlatModel{
		Value: types.Float64Value(0.1),
		Label: types.StringValue("0.1"),
	}
	expanded := expandNumericValueLabel(model)
	if expanded == nil || expanded.Value == nil {
		t.Fatal("expected expanded numeric value")
	}

	flattened := flattenNumericValueLabelFlat(expanded, numericType)
	attrs := flattened.Attributes()
	gotValue, ok := attrs["value"].(types.Float64)
	if !ok {
		t.Fatalf("value attr type = %T", attrs["value"])
	}
	if gotValue.ValueFloat64() != 0.1 {
		t.Fatalf("single_numeric.value = %v, want 0.1", gotValue.ValueFloat64())
	}

	textboxExpanded, diags := expandTextboxDefaultValueV2(&textboxDefaultValueV2Model{
		DefaultNumericValue: &textboxNumericValueV2Model{
			Value:     types.Float64Value(0.1),
			Min:       types.Float64Value(0.1),
			Max:       types.Float64Value(1.1),
			IsInteger: types.BoolValue(false),
		},
	})
	if diags.HasError() {
		t.Fatalf("expand textbox numeric: %v", diags)
	}
	num := textboxExpanded.DefaultNumericValue
	if float32PointerToTypeFloat64(num.Value).ValueFloat64() != 0.1 {
		t.Fatalf("textbox value = %v, want 0.1", float32PointerToTypeFloat64(num.Value).ValueFloat64())
	}
	if float32PointerToTypeFloat64(num.Min).ValueFloat64() != 0.1 {
		t.Fatalf("textbox min = %v, want 0.1", float32PointerToTypeFloat64(num.Min).ValueFloat64())
	}
	if float32PointerToTypeFloat64(num.Max).ValueFloat64() != 1.1 {
		t.Fatalf("textbox max = %v, want 1.1", float32PointerToTypeFloat64(num.Max).ValueFloat64())
	}
}
