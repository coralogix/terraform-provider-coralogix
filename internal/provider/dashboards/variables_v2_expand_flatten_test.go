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
	valuesType := staticType.AttrTypes["values"].(types.ListType)
	valueEntryType := valuesType.ElemType.(types.ObjectType)

	valueEntry := mustObjectValue(t, valueEntryType.AttrTypes, map[string]attr.Value{
		"value":      types.StringValue("production"),
		"label":      types.StringValue("production"),
		"is_default": types.BoolValue(true),
	})
	static := mustStaticSourceV2(t, staticType, mustListValue(t, valueEntryType, []attr.Value{valueEntry}))
	source := mustSourceWithStaticV2(t, sourceType, static)
	value := mustSingleStringValueV2(t, valueType, "production", "production")
	list := mustListValue(t, elementType, []attr.Value{
		mustVariableV2(t, elementType, "environment", "Environment", source, value),
	})

	expanded := mustExpandVariablesV2(t, ctx, list)
	assertStaticRoundTripExpanded(t, expanded)

	flattened := mustFlattenVariablesV2(t, ctx, expanded, list)
	var models []DashboardVariableV2Model
	mustElementsAs(t, flattened, &models)
	if len(models) != 1 || models[0].Name.ValueString() != "environment" {
		t.Fatalf("flattened models = %#v", models)
	}
	if models[0].ID.IsNull() || models[0].ID.ValueString() == "" {
		t.Fatal("expected generated id after expand/flatten")
	}
}

func assertStaticRoundTripExpanded(t *testing.T, expanded []dashboardservice.VariableV2) {
	t.Helper()
	if len(expanded) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(expanded))
	}
	if expanded[0].GetName() != "environment" {
		t.Fatalf("name = %q", expanded[0].GetName())
	}
	if expanded[0].Source.Static == nil {
		t.Fatal("expected static source")
	}
	if len(expanded[0].Source.Static.Values) != 1 || expanded[0].Source.Static.Values[0].GetLabel() != "production" {
		t.Fatalf("static values = %#v", expanded[0].Source.Static.Values)
	}
	if expanded[0].Value.SingleString == nil || expanded[0].Value.SingleString.Value == nil {
		t.Fatal("expected single string value")
	}
	got := expanded[0].Value.SingleString.Value
	if got.GetValue() != "production" || got.GetLabel() != "production" {
		t.Fatalf("single string = %#v", got)
	}
}

func TestExpandFlattenVariablesV2StaticOmittedLabel(t *testing.T) {
	ctx := context.Background()
	elementType := dashboardVariablesV2ElementType()
	sourceType := elementType.AttrTypes["source"].(types.ObjectType)
	valueType := elementType.AttrTypes["value"].(types.ObjectType)
	staticType := sourceType.AttrTypes["static"].(types.ObjectType)
	valuesType := staticType.AttrTypes["values"].(types.ListType)
	valueEntryType := valuesType.ElemType.(types.ObjectType)

	valueEntry := mustObjectValue(t, valueEntryType.AttrTypes, map[string]attr.Value{
		"value":      types.StringValue("production"),
		"label":      types.StringNull(),
		"is_default": types.BoolValue(true),
	})
	customEntry := mustObjectValue(t, valueEntryType.AttrTypes, map[string]attr.Value{
		"value":      types.StringValue("staging"),
		"label":      types.StringValue("Staging"),
		"is_default": types.BoolNull(),
	})
	static := mustStaticSourceV2(t, staticType, mustListValue(t, valueEntryType, []attr.Value{valueEntry, customEntry}))
	source := mustSourceWithStaticV2(t, sourceType, static)
	value := mustSingleStringValueV2(t, valueType, "production", "production")
	list := mustListValue(t, elementType, []attr.Value{
		mustVariableV2(t, elementType, "environment", "Environment", source, value),
	})

	expanded := mustExpandVariablesV2(t, ctx, list)
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

	flattened := mustFlattenVariablesV2(t, ctx, expanded, list)
	assertStaticOmittedLabelFlattened(t, ctx, flattened)
}

func assertStaticOmittedLabelFlattened(t *testing.T, ctx context.Context, flattened types.List) {
	t.Helper()
	var models []DashboardVariableV2Model
	mustElementsAs(t, flattened, &models)
	var sourceModel variableSourceV2Model
	mustObjectAs(t, models[0].Source, &sourceModel)
	if sourceModel.Static == nil {
		t.Fatal("expected static source on flattened model")
	}
	var flatValues []staticValueV2Model
	mustElementsAs(t, sourceModel.Static.Values, &flatValues)
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

	textbox := mustObjectValue(t, textboxType.AttrTypes, map[string]attr.Value{
		"default_value": types.ObjectNull(textboxType.AttrTypes["default_value"].(types.ObjectType).AttrTypes),
	})
	source := mustObjectValue(t, sourceType.AttrTypes, map[string]attr.Value{
		"static":  nullObjectAttr(sourceType.AttrTypes["static"]),
		"textbox": textbox,
		"query":   nullObjectAttr(sourceType.AttrTypes["query"]),
	})
	value := mustSingleStringValueV2(t, valueType, "hello", "hello")

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
	if expanded.DisplayType != "" {
		t.Fatalf("null display_type should expand to empty required enum, got %q", expanded.DisplayType)
	}
}

func TestFlattenVariableV2DisplayTypeUnspecifiedIsNull(t *testing.T) {
	ctx := context.Background()
	elementType := dashboardVariablesV2ElementType()
	hello := "hello"

	flattened, diags := flattenDashboardVariableV2(ctx, &dashboardservice.VariableV2{
		Name:        "search",
		DisplayName: "Search",
		DisplayType: dashboardservice.VARIABLEDISPLAYTYPEV2_VARIABLE_DISPLAY_TYPE_V2_UNSPECIFIED,
		Source: dashboardservice.VariableSourceV2{
			Textbox: &dashboardservice.TextboxSource{},
		},
		Value: dashboardservice.VariableValueV2{
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

func TestExpandFlattenVariablesV2PromqlAndEmptyMultiList(t *testing.T) {
	ctx := context.Background()
	typeset := newVariablesV2QueryTypes(t)
	allOption := mustAllOptionV2(t, typeset.queryType, true)

	promql := mustObjectValue(t, typeset.promqlType.AttrTypes, map[string]attr.Value{
		"query":             types.StringValue("vector(1)"),
		"promql_query_type": types.StringValue("instant"),
	})
	metricsInner := mustObjectValue(t, typeset.metricsInnerType.AttrTypes, map[string]attr.Value{
		"metric_name":  nullObjectAttr(typeset.metricsInnerType.AttrTypes["metric_name"]),
		"label_name":   nullObjectAttr(typeset.metricsInnerType.AttrTypes["label_name"]),
		"label_value":  nullObjectAttr(typeset.metricsInnerType.AttrTypes["label_value"]),
		"promql_query": promql,
	})
	metrics := mustObjectValue(t, typeset.metricsType.AttrTypes, map[string]attr.Value{"type": metricsInner})
	query := mustQuerySourceV2(t, typeset, allOption, types.StringValue("on_dashboard_load"), map[string]attr.Value{
		"metrics_query": metrics,
	})
	value := mustMultiStringListValueV2(t, typeset.valueType, typeset.multiType, typeset.listType, typeset.listEntryType, nil)
	list := mustQueryVariableListV2(t, typeset, "prom", "Prom", query, value)

	expanded := mustExpandVariablesV2(t, ctx, list)
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

	assertPromqlFlattened(t, ctx, mustFlattenVariablesV2(t, ctx, expanded, list))
}

func assertPromqlFlattened(t *testing.T, ctx context.Context, flattened types.List) {
	t.Helper()
	var models []DashboardVariableV2Model
	mustElementsAs(t, flattened, &models)
	var sourceModel variableSourceV2Model
	mustObjectAs(t, models[0].Source, &sourceModel)
	if sourceModel.Query == nil || sourceModel.Query.ValueDisplayOptions != nil {
		t.Fatalf("flattened display options should stay unset, got %#v", sourceModel.Query)
	}
	var valueModel variableValueV2Model
	mustObjectAs(t, models[0].Value, &valueModel)
	if valueModel.MultiString == nil || valueModel.MultiString.List == nil {
		t.Fatal("expected flattened multi_string.list")
	}
	if valueModel.MultiString.List.Values.IsNull() || len(valueModel.MultiString.List.Values.Elements()) != 0 {
		t.Fatalf("flattened empty list should stay known empty, got %#v", valueModel.MultiString.List.Values)
	}
}

func TestExpandFlattenVariablesV2DataprimeQuery(t *testing.T) {
	ctx := context.Background()
	typeset := newVariablesV2QueryTypes(t)
	allOption := mustAllOptionV2(t, typeset.queryType, true)

	queryText := mustObjectValue(t, typeset.queryTextType.AttrTypes, map[string]attr.Value{
		"query":          types.StringValue("source logs | limit 10"),
		"data_mode_type": types.StringValue("high"),
	})
	dataprimeInner := mustObjectValue(t, typeset.dataprimeInnerType.AttrTypes, map[string]attr.Value{"query_text": queryText})
	dataprime := mustObjectValue(t, typeset.dataprimeType.AttrTypes, map[string]attr.Value{"type": dataprimeInner})
	query := mustQuerySourceV2(t, typeset, allOption, types.StringNull(), map[string]attr.Value{
		"dataprime_query": dataprime,
	})
	value := mustMultiStringSelectedAllValueV2(t, typeset.valueType, typeset.multiType, typeset.listType)
	list := mustQueryVariableListV2(t, typeset, "dp", "DP", query, value)

	expanded := mustExpandVariablesV2(t, ctx, list)
	queryTextExpanded := expanded[0].Source.Query.DataprimeQuery.Type.QueryText
	if queryTextExpanded == nil || queryTextExpanded.Query == nil || queryTextExpanded.Query.GetText() != "source logs | limit 10" {
		t.Fatalf("dataprime wrapper = %#v", queryTextExpanded)
	}
	if expanded[0].Value.MultiString == nil || expanded[0].Value.MultiString.SelectedAll == nil {
		t.Fatalf("selected_all wire = %#v", expanded[0].Value.MultiString)
	}
}

func TestExpandFlattenVariablesV2SpansAndMetricsOperator(t *testing.T) {
	ctx := context.Background()
	typeset := newVariablesV2QueryTypes(t)
	allOption := mustAllOptionV2(t, typeset.queryType, true)

	spanValue := mustObjectValue(t, typeset.spanValueType.AttrTypes, map[string]attr.Value{
		"type":  types.StringValue("metadata"),
		"value": types.StringValue("service_name"),
	})
	spansFieldValue := mustObjectValue(t, typeset.spansFieldValueType.AttrTypes, map[string]attr.Value{
		"value":             spanValue,
		"observation_field": nullObjectAttr(typeset.spansFieldValueType.AttrTypes["observation_field"]),
	})
	spansInner := mustObjectValue(t, typeset.spansInnerType.AttrTypes, map[string]attr.Value{"field_value": spansFieldValue})
	spans := mustObjectValue(t, typeset.spansType.AttrTypes, map[string]attr.Value{"type": spansInner})
	querySpans := mustQuerySourceV2(t, typeset, allOption, types.StringNull(), map[string]attr.Value{
		"spans_query": spans,
	})
	value := mustMultiStringSelectedAllValueV2(t, typeset.valueType, typeset.multiType, typeset.listType)
	listSpans := mustQueryVariableListV2(t, typeset, "span", "Span", querySpans, value)

	expandedSpans := mustExpandVariablesV2(t, ctx, listSpans)
	spanField := expandedSpans[0].Source.Query.SpansQuery.Type.FieldValue.Value
	if spanField == nil || spanField.MetadataField == nil {
		t.Fatalf("expected spans metadata field, got %#v", spanField)
	}

	assertMetricsLabelFiltersWire(t, ctx, typeset)
}

func assertMetricsLabelFiltersWire(t *testing.T, ctx context.Context, typeset variablesV2QueryTypes) {
	t.Helper()
	metricName := mustStringOrVariableV2(t, typeset.stringOrVarType, "http_requests_total")
	labelName := mustStringOrVariableV2(t, typeset.stringOrVarType, "env")
	emptyFilters := mustListValue(t, typeset.filterEntryType, []attr.Value{})

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

	filter := mustMetricsLabelFilterV2(t, typeset, "http_requests_total", "region", "prod")
	filtersWithOp := mustListValue(t, typeset.filterEntryType, []attr.Value{filter})
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

	assertTextboxNumericFractions(t)
}

func assertTextboxNumericFractions(t *testing.T) {
	t.Helper()
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

type variablesV2QueryTypes struct {
	elementType         types.ObjectType
	sourceType          types.ObjectType
	valueType           types.ObjectType
	queryType           types.ObjectType
	metricsType         types.ObjectType
	metricsInnerType    types.ObjectType
	promqlType          types.ObjectType
	dataprimeType       types.ObjectType
	dataprimeInnerType  types.ObjectType
	queryTextType       types.ObjectType
	multiType           types.ObjectType
	listType            types.ObjectType
	listEntryType       types.ObjectType
	spansType           types.ObjectType
	spansInnerType      types.ObjectType
	spansFieldValueType types.ObjectType
	spanValueType       types.ObjectType
	stringOrVarType     types.ObjectType
	filterEntryType     types.ObjectType
	operatorType        types.ObjectType
}

func newVariablesV2QueryTypes(t *testing.T) variablesV2QueryTypes {
	t.Helper()
	elementType := dashboardVariablesV2ElementType()
	sourceType := elementType.AttrTypes["source"].(types.ObjectType)
	valueType := elementType.AttrTypes["value"].(types.ObjectType)
	queryType := sourceType.AttrTypes["query"].(types.ObjectType)
	metricsType := queryType.AttrTypes["metrics_query"].(types.ObjectType)
	metricsInnerType := metricsType.AttrTypes["type"].(types.ObjectType)
	multiType := valueType.AttrTypes["multi_string"].(types.ObjectType)
	listType := multiType.AttrTypes["list"].(types.ObjectType)
	listValuesType := listType.AttrTypes["values"].(types.ListType)
	spansType := queryType.AttrTypes["spans_query"].(types.ObjectType)
	spansInnerType := spansType.AttrTypes["type"].(types.ObjectType)
	spansFieldValueType := spansInnerType.AttrTypes["field_value"].(types.ObjectType)
	dataprimeType := queryType.AttrTypes["dataprime_query"].(types.ObjectType)
	dataprimeInnerType := dataprimeType.AttrTypes["type"].(types.ObjectType)
	filterEntryType := metricsInnerType.AttrTypes["label_value"].(types.ObjectType).AttrTypes["label_filters"].(types.ListType).ElemType.(types.ObjectType)
	return variablesV2QueryTypes{
		elementType:         elementType,
		sourceType:          sourceType,
		valueType:           valueType,
		queryType:           queryType,
		metricsType:         metricsType,
		metricsInnerType:    metricsInnerType,
		promqlType:          metricsInnerType.AttrTypes["promql_query"].(types.ObjectType),
		dataprimeType:       dataprimeType,
		dataprimeInnerType:  dataprimeInnerType,
		queryTextType:       dataprimeInnerType.AttrTypes["query_text"].(types.ObjectType),
		multiType:           multiType,
		listType:            listType,
		listEntryType:       listValuesType.ElemType.(types.ObjectType),
		spansType:           spansType,
		spansInnerType:      spansInnerType,
		spansFieldValueType: spansFieldValueType,
		spanValueType:       spansFieldValueType.AttrTypes["value"].(types.ObjectType),
		stringOrVarType:     metricsInnerType.AttrTypes["label_value"].(types.ObjectType).AttrTypes["metric_name"].(types.ObjectType),
		filterEntryType:     filterEntryType,
		operatorType:        filterEntryType.AttrTypes["operator"].(types.ObjectType),
	}
}

func mustObjectValue(t *testing.T, attrTypes map[string]attr.Type, attrs map[string]attr.Value) types.Object {
	t.Helper()
	v, diags := types.ObjectValue(attrTypes, attrs)
	if diags.HasError() {
		t.Fatalf("%v", diags)
	}
	return v
}

func mustListValue(t *testing.T, elemType attr.Type, elems []attr.Value) types.List {
	t.Helper()
	v, diags := types.ListValue(elemType, elems)
	if diags.HasError() {
		t.Fatalf("%v", diags)
	}
	return v
}

func nullObjectAttr(objectType attr.Type) types.Object {
	return types.ObjectNull(objectType.(types.ObjectType).AttrTypes)
}

func mustExpandVariablesV2(t *testing.T, ctx context.Context, list types.List) []dashboardservice.VariableV2 {
	t.Helper()
	expanded, diags := expandDashboardVariablesV2(ctx, list)
	if diags.HasError() {
		t.Fatalf("expand: %v", diags)
	}
	return expanded
}

func mustFlattenVariablesV2(t *testing.T, ctx context.Context, expanded []dashboardservice.VariableV2, configured types.List) types.List {
	t.Helper()
	flattened, diags := flattenDashboardVariablesV2(ctx, expanded, configured)
	if diags.HasError() {
		t.Fatalf("flatten: %v", diags)
	}
	return flattened
}

func mustElementsAs(t *testing.T, list types.List, target any) {
	t.Helper()
	if diags := list.ElementsAs(context.Background(), target, false); diags.HasError() {
		t.Fatalf("elements: %v", diags)
	}
}

func mustObjectAs(t *testing.T, object types.Object, target any) {
	t.Helper()
	if diags := object.As(context.Background(), target, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("object as: %v", diags)
	}
}

func mustAllOptionV2(t *testing.T, queryType types.ObjectType, includeAll bool) types.Object {
	t.Helper()
	allOptionType := queryType.AttrTypes["all_option"].(types.ObjectType)
	return mustObjectValue(t, allOptionType.AttrTypes, map[string]attr.Value{
		"include_all": types.BoolValue(includeAll),
		"label":       types.StringNull(),
	})
}

func mustStaticSourceV2(t *testing.T, staticType types.ObjectType, values types.List) types.Object {
	t.Helper()
	allOptionType := staticType.AttrTypes["all_option"].(types.ObjectType)
	allOption := mustObjectValue(t, allOptionType.AttrTypes, map[string]attr.Value{
		"include_all": types.BoolValue(false),
		"label":       types.StringNull(),
	})
	return mustObjectValue(t, staticType.AttrTypes, map[string]attr.Value{
		"values_order_direction": types.StringValue("none"),
		"all_option":             allOption,
		"values":                 values,
	})
}

func mustSourceWithStaticV2(t *testing.T, sourceType types.ObjectType, static types.Object) types.Object {
	t.Helper()
	return mustObjectValue(t, sourceType.AttrTypes, map[string]attr.Value{
		"static":  static,
		"textbox": nullObjectAttr(sourceType.AttrTypes["textbox"]),
		"query":   nullObjectAttr(sourceType.AttrTypes["query"]),
	})
}

func mustSingleStringValueV2(t *testing.T, valueType types.ObjectType, value, label string) types.Object {
	t.Helper()
	singleString := mustObjectValue(t, valueType.AttrTypes["single_string"].(types.ObjectType).AttrTypes, map[string]attr.Value{
		"value": types.StringValue(value),
		"label": types.StringValue(label),
	})
	return mustObjectValue(t, valueType.AttrTypes, map[string]attr.Value{
		"single_string":  singleString,
		"single_numeric": nullObjectAttr(valueType.AttrTypes["single_numeric"]),
		"regex":          nullObjectAttr(valueType.AttrTypes["regex"]),
		"lucene":         nullObjectAttr(valueType.AttrTypes["lucene"]),
		"interval":       nullObjectAttr(valueType.AttrTypes["interval"]),
		"multi_string":   nullObjectAttr(valueType.AttrTypes["multi_string"]),
	})
}

func mustVariableV2(t *testing.T, elementType types.ObjectType, name, displayName string, source, value types.Object) types.Object {
	t.Helper()
	return mustObjectValue(t, elementType.AttrTypes, map[string]attr.Value{
		"id":               types.StringNull(),
		"name":             types.StringValue(name),
		"display_name":     types.StringValue(displayName),
		"description":      types.StringNull(),
		"display_type":     types.StringValue("label_value"),
		"display_full_row": types.BoolNull(),
		"source":           source,
		"value":            value,
	})
}

func mustQuerySourceV2(t *testing.T, typeset variablesV2QueryTypes, allOption types.Object, refresh types.String, arms map[string]attr.Value) types.Object {
	t.Helper()
	attrs := map[string]attr.Value{
		"values_order_direction": types.StringValue("asc"),
		"all_option":             allOption,
		"refresh_strategy":       refresh,
		"value_display_options":  nullObjectAttr(typeset.queryType.AttrTypes["value_display_options"]),
		"logs_query":             nullObjectAttr(typeset.queryType.AttrTypes["logs_query"]),
		"spans_query":            nullObjectAttr(typeset.queryType.AttrTypes["spans_query"]),
		"metrics_query":          nullObjectAttr(typeset.metricsType),
		"dataprime_query":        nullObjectAttr(typeset.dataprimeType),
	}
	for key, value := range arms {
		attrs[key] = value
	}
	return mustObjectValue(t, typeset.queryType.AttrTypes, attrs)
}

func mustQueryVariableListV2(t *testing.T, typeset variablesV2QueryTypes, name, displayName string, query, value types.Object) types.List {
	t.Helper()
	source := mustObjectValue(t, typeset.sourceType.AttrTypes, map[string]attr.Value{
		"static":  nullObjectAttr(typeset.sourceType.AttrTypes["static"]),
		"textbox": nullObjectAttr(typeset.sourceType.AttrTypes["textbox"]),
		"query":   query,
	})
	return mustListValue(t, typeset.elementType, []attr.Value{
		mustVariableV2(t, typeset.elementType, name, displayName, source, value),
	})
}

func mustMultiStringListValueV2(t *testing.T, valueType, multiType, listType, listEntryType types.ObjectType, values []attr.Value) types.Object {
	t.Helper()
	listObj := mustObjectValue(t, listType.AttrTypes, map[string]attr.Value{
		"values": mustListValue(t, listEntryType, values),
	})
	multi := mustObjectValue(t, multiType.AttrTypes, map[string]attr.Value{
		"selected_all": nullObjectAttr(multiType.AttrTypes["selected_all"]),
		"all":          nullObjectAttr(multiType.AttrTypes["all"]),
		"list":         listObj,
	})
	return mustObjectValue(t, valueType.AttrTypes, map[string]attr.Value{
		"single_string":  nullObjectAttr(valueType.AttrTypes["single_string"]),
		"single_numeric": nullObjectAttr(valueType.AttrTypes["single_numeric"]),
		"regex":          nullObjectAttr(valueType.AttrTypes["regex"]),
		"lucene":         nullObjectAttr(valueType.AttrTypes["lucene"]),
		"interval":       nullObjectAttr(valueType.AttrTypes["interval"]),
		"multi_string":   multi,
	})
}

func mustMultiStringSelectedAllValueV2(t *testing.T, valueType, multiType, listType types.ObjectType) types.Object {
	t.Helper()
	selectedAll := mustObjectValue(t, multiType.AttrTypes["selected_all"].(types.ObjectType).AttrTypes, map[string]attr.Value{})
	multi := mustObjectValue(t, multiType.AttrTypes, map[string]attr.Value{
		"selected_all": selectedAll,
		"all":          nullObjectAttr(multiType.AttrTypes["all"]),
		"list":         nullObjectAttr(listType),
	})
	return mustObjectValue(t, valueType.AttrTypes, map[string]attr.Value{
		"single_string":  nullObjectAttr(valueType.AttrTypes["single_string"]),
		"single_numeric": nullObjectAttr(valueType.AttrTypes["single_numeric"]),
		"regex":          nullObjectAttr(valueType.AttrTypes["regex"]),
		"lucene":         nullObjectAttr(valueType.AttrTypes["lucene"]),
		"interval":       nullObjectAttr(valueType.AttrTypes["interval"]),
		"multi_string":   multi,
	})
}

func mustStringOrVariableV2(t *testing.T, stringOrVarType types.ObjectType, stringValue string) types.Object {
	t.Helper()
	return mustObjectValue(t, stringOrVarType.AttrTypes, map[string]attr.Value{
		"string_value":  types.StringValue(stringValue),
		"variable_name": types.StringNull(),
	})
}

func mustMetricsLabelFilterV2(t *testing.T, typeset variablesV2QueryTypes, metric, label, selected string) types.Object {
	t.Helper()
	selectedValues := mustListValue(t, typeset.stringOrVarType, []attr.Value{
		mustStringOrVariableV2(t, typeset.stringOrVarType, selected),
	})
	operator := mustObjectValue(t, typeset.operatorType.AttrTypes, map[string]attr.Value{
		"type":            types.StringValue("equals"),
		"selected_values": selectedValues,
	})
	return mustObjectValue(t, typeset.filterEntryType.AttrTypes, map[string]attr.Value{
		"metric":   mustStringOrVariableV2(t, typeset.stringOrVarType, metric),
		"label":    mustStringOrVariableV2(t, typeset.stringOrVarType, label),
		"operator": operator,
	})
}

// TestFlattenVariableV2EmptyValueDisplayOptionsIsNull covers the empty object the
// API returns when a query variable sets neither regex. The attribute requires
// at least one of them, so an object of nulls in state would produce a
// configuration that cannot be planned.
func TestFlattenVariableV2EmptyValueDisplayOptionsIsNull(t *testing.T) {
	ctx := context.Background()
	elementType := dashboardVariablesV2ElementType()
	regex := ".*"

	flattened, diags := flattenDashboardVariableV2(ctx, &dashboardservice.VariableV2{
		Name:        "env",
		DisplayName: "Env",
		DisplayType: dashboardservice.VARIABLEDISPLAYTYPEV2_VARIABLE_DISPLAY_TYPE_V2_LABEL_VALUE,
		Source: dashboardservice.VariableSourceV2{
			Query: &dashboardservice.VariableSourceV2QuerySource{
				ValueDisplayOptions: &dashboardservice.VariableSourceV2ValueDisplayOptions{},
			},
		},
		Value: dashboardservice.VariableValueV2{
			Regex: &dashboardservice.RegexValue{
				Value: &dashboardservice.StringValueLabel{Value: &regex},
			},
		},
	}, elementType)
	if diags.HasError() {
		t.Fatalf("flatten: %v", diags)
	}

	source, ok := flattened.Attributes()["source"].(types.Object)
	if !ok {
		t.Fatalf("source type = %T, want types.Object", flattened.Attributes()["source"])
	}
	query, ok := source.Attributes()["query"].(types.Object)
	if !ok {
		t.Fatalf("query type = %T, want types.Object", source.Attributes()["query"])
	}
	options, ok := query.Attributes()["value_display_options"].(types.Object)
	if !ok {
		t.Fatalf("value_display_options type = %T, want types.Object", query.Attributes()["value_display_options"])
	}
	if !options.IsNull() {
		t.Fatalf("an empty value_display_options should read as null, got %v", options)
	}

	// A label with no value stays absent, so the regex value reads back as the
	// API stored it.
	value, ok := flattened.Attributes()["value"].(types.Object)
	if !ok {
		t.Fatalf("value type = %T, want types.Object", flattened.Attributes()["value"])
	}
	regexValue, ok := value.Attributes()["regex"].(types.Object)
	if !ok {
		t.Fatalf("regex type = %T, want types.Object", value.Attributes()["regex"])
	}
	if label := regexValue.Attributes()["label"].(types.String); !label.IsNull() {
		t.Fatalf("label = %q, want null", label.ValueString())
	}
}
