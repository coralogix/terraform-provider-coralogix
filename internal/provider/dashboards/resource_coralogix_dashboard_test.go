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

package dashboards

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"unicode"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func ptr[T any](v T) *T {
	return &v
}

func TestExtractDashboardContentJSONRestoresAliasesBeforeDiscardingUnknownFields(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "testdata", "dashboards", "content_json_unknown_fields.json"))
	if err != nil {
		t.Fatalf("read content_json unknown-fields fixture: %s", err)
	}

	dashboard, diags := extractDashboard(context.Background(), DashboardResourceModel{
		ContentJson: types.StringValue(string(content)),
		Folder:      types.ObjectNull(dashboardFolderModelAttr()),
	})
	if diags.HasError() {
		t.Fatalf("extract content_json dashboard: %v", diags)
	}
	definition := dashboard.Layout.Sections[0].Rows[0].Widgets[0].Definition
	if definition == nil || definition.DataTable == nil {
		t.Fatal("expected data_table alias to be restored into the typed dataTable field")
	}
	dataTable := definition.DataTable
	if dataTable.ResultsPerPage == nil || *dataTable.ResultsPerPage != 10 {
		t.Fatalf("expected results_per_page alias to be restored, got %v", dataTable.ResultsPerPage)
	}
	if dataTable.RowStyle == nil || *dataTable.RowStyle != dashboardservice.ROWSTYLE_ROW_STYLE_ONE_LINE {
		t.Fatalf("expected row_style alias to be restored, got %v", dataTable.RowStyle)
	}
	if dataTable.Query == nil || dataTable.Query.Metrics == nil || dataTable.Query.Metrics.PromqlQuery == nil {
		t.Fatal("expected nested query and promql_query aliases to be restored into typed fields")
	}

	request := newDashboardOpenAPICreateRequest(*dashboard, nil)
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal normalized content_json request: %s", err)
	}
	for _, unknownField := range []string{
		"unknownRoot", "unknownLayout", "unknownSection", "unknownRow", "unknownWidget",
		"unknownDefinition", "unknownDataTable", "unknownQuery", "unknownMetrics", "unknownPromqlQuery",
	} {
		if strings.Contains(string(encoded), unknownField) {
			t.Fatalf("expected unknown property %q to be discarded from request %s", unknownField, encoded)
		}
	}
}

func TestExtractDashboardContentJSONPreservesDynamicQueriesTable(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "testdata", "dashboards", "content_json_dynamic_queries_table.json"))
	if err != nil {
		t.Fatalf("read dynamic content_json fixture: %s", err)
	}

	dashboard, diags := extractDashboard(context.Background(), DashboardResourceModel{
		ContentJson: types.StringValue(string(content)),
		Folder:      types.ObjectNull(dashboardFolderModelAttr()),
	})
	if diags.HasError() {
		t.Fatalf("extract dynamic content_json dashboard: %v", diags)
	}
	wantQueries := []string{"logs", "metrics", "spans"}
	for i, wantQuery := range wantQueries {
		definition := dashboard.Layout.Sections[0].Rows[i].Widgets[0].Definition
		if definition == nil || definition.Dynamic == nil {
			t.Fatalf("row %d: expected dynamic widget definition to be preserved", i)
		}
		dynamic := definition.Dynamic
		if len(dynamic.QueryDefinitions) != 1 {
			t.Fatalf("row %d: dynamic query definitions = %d, want 1", i, len(dynamic.QueryDefinitions))
		}
		query := dynamic.QueryDefinitions[0].Query
		queryPresent := map[string]bool{
			"dataprime": query.Dataprime != nil,
			"logs":      query.Logs != nil,
			"metrics":   query.Metrics != nil,
			"spans":     query.Spans != nil,
		}
		for branch, present := range queryPresent {
			if present != (branch == wantQuery) {
				t.Fatalf("row %d: dynamic query branch %s populated=%t, want %t; query=%+v", i, branch, present, branch == wantQuery, query)
			}
		}
		if dynamic.Visualization == nil {
			t.Fatalf("row %d: expected dynamic visualization", i)
		}
		visualization := reflect.ValueOf(dynamic.Visualization).Elem()
		for fieldIndex := 0; fieldIndex < visualization.NumField(); fieldIndex++ {
			fieldDefinition := visualization.Type().Field(fieldIndex)
			branch := strings.Split(fieldDefinition.Tag.Get("json"), ",")[0]
			if branch == "" || branch == "-" || branch == "AdditionalProperties" {
				continue
			}
			present := !visualization.Field(fieldIndex).IsZero()
			if present != (branch == "table") {
				t.Fatalf("row %d: dynamic visualization branch %s populated=%t, want %t", i, branch, present, branch == "table")
			}
		}
		if dynamic.Visualization.Table == nil {
			t.Fatalf("row %d: expected dynamic table visualization, got %+v", i, dynamic.Visualization)
		}
	}

	request := newDashboardOpenAPICreateRequest(*dashboard, nil)
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal dynamic content_json request: %s", err)
	}
	if !strings.Contains(string(encoded), `"dynamic"`) ||
		!strings.Contains(string(encoded), `"queryDefinitions"`) ||
		!strings.Contains(string(encoded), `"table"`) ||
		!strings.Contains(string(encoded), `"logs"`) ||
		!strings.Contains(string(encoded), `"metrics"`) ||
		!strings.Contains(string(encoded), `"spans"`) {
		t.Fatalf("dynamic branches were lost from the REST create request: %s", encoded)
	}
}

// TestDashboardContentJSONGeneratedOneOfBranchContract proves the generic
// content_json transport contract for every generated union reachable from a
// Dashboard. Each branch is decoded through its protobuf snake_case alias,
// promoted into the exact typed field, stripped of AdditionalProperties, and
// serialized inside a create request with every sibling left nil.
func TestDashboardContentJSONGeneratedOneOfBranchContract(t *testing.T) {
	rootType := reflect.TypeOf(dashboardservice.Dashboard{})
	reachable, parents := dashboardReachableGeneratedModels(rootType)
	guards := dashboardGeneratedOneOfBranches(t)
	enumValues := dashboardGeneratedEnumValues(t)

	modelNames := make([]string, 0, len(guards))
	for model := range guards {
		if _, ok := reachable[model]; ok {
			modelNames = append(modelNames, model)
		}
	}
	sort.Strings(modelNames)

	for _, modelName := range modelNames {
		modelType := reachable[modelName]
		branches := append([]string(nil), guards[modelName]...)
		sort.Strings(branches)
		for _, branch := range branches {
			branch := branch
			t.Run(modelName+"/"+branch, func(t *testing.T) {
				model := reflect.New(modelType)
				field, ok := dashboardGeneratedJSONField(model.Elem(), branch)
				if !ok {
					t.Fatalf("generated model has no JSON field %q", branch)
				}

				alias := dashboardLowerCamelToSnake(branch)
				payload, ok := dashboardGeneratedJSONData(modelType, enumValues, 0).(map[string]any)
				if !ok {
					t.Fatalf("minimal generated model value has type %T, want object", payload)
				}
				payload[alias] = dashboardGeneratedJSONData(field.Type(), enumValues, 0)
				encodedPayload, err := json.Marshal(payload)
				if err != nil {
					t.Fatalf("marshal protobuf JSON alias %q: %s", alias, err)
				}
				if err := json.Unmarshal(encodedPayload, model.Interface()); err != nil {
					t.Fatalf("decode protobuf JSON alias %q: %s", alias, err)
				}
				if err := restoreOpenAPIProtoFieldNames(model.Interface()); err != nil {
					t.Fatalf("restore protobuf JSON alias %q: %s", alias, err)
				}
				discardOpenAPIAdditionalProperties(model.Interface())

				for _, candidate := range branches {
					candidateField, ok := dashboardGeneratedJSONField(model.Elem(), candidate)
					if !ok {
						t.Fatalf("generated model has no sibling JSON field %q", candidate)
					}
					if got, want := !candidateField.IsZero(), candidate == branch; got != want {
						t.Fatalf("typed branch %s populated=%t, want %t", candidate, got, want)
					}
				}

				dashboard := dashboardEmbedGeneratedModel(t, rootType, modelType, model.Elem(), parents)
				request := newDashboardOpenAPICreateRequest(dashboard.Interface().(dashboardservice.Dashboard), nil)
				encoded, err := json.Marshal(request)
				if err != nil {
					t.Fatalf("serialize generated branch in REST create request: %s", err)
				}
				if !dashboardJSONContainsKey(encoded, branch) {
					t.Fatalf("serialized REST create request lost branch %q: %s", branch, encoded)
				}
			})
		}
	}
}

func TestDashboardStructuredConfigurationRejectsUnsupportedAutoRefreshBranches(t *testing.T) {
	root := dashboard_schema.V4()
	autoRefresh, ok := root.Attributes["auto_refresh"].(resourceschema.SingleNestedAttribute)
	if !ok {
		t.Fatal("auto_refresh is not a single nested structured attribute")
	}
	refreshType, ok := autoRefresh.Attributes["type"].(resourceschema.StringAttribute)
	if !ok || len(refreshType.Validators) == 0 {
		t.Fatal("auto_refresh.type has no string validator")
	}

	for _, value := range []string{"one_minute", "fifteen_minutes"} {
		t.Run(value, func(t *testing.T) {
			request := validator.StringRequest{ConfigValue: types.StringValue(value)}
			var response validator.StringResponse
			for _, configuredValidator := range refreshType.Validators {
				configuredValidator.ValidateString(context.Background(), request, &response)
			}
			if !response.Diagnostics.HasError() {
				t.Fatalf("structured auto_refresh.type=%q reached transport validation without an error", value)
			}
		})
	}
}

type dashboardGeneratedParent struct {
	model      reflect.Type
	fieldIndex int
}

func dashboardReachableGeneratedModels(root reflect.Type) (map[string]reflect.Type, map[reflect.Type]dashboardGeneratedParent) {
	packagePath := root.PkgPath()
	reachable := map[string]reflect.Type{root.Name(): root}
	parents := make(map[reflect.Type]dashboardGeneratedParent)
	queue := []reflect.Type{root}
	for len(queue) > 0 {
		model := queue[0]
		queue = queue[1:]
		for fieldIndex := 0; fieldIndex < model.NumField(); fieldIndex++ {
			child := dashboardGeneratedElementType(model.Field(fieldIndex).Type)
			if child.Kind() != reflect.Struct || child.PkgPath() != packagePath || child.Name() == "" {
				continue
			}
			if _, seen := reachable[child.Name()]; seen {
				continue
			}
			reachable[child.Name()] = child
			parents[child] = dashboardGeneratedParent{model: model, fieldIndex: fieldIndex}
			queue = append(queue, child)
		}
	}
	return reachable, parents
}

func dashboardGeneratedElementType(value reflect.Type) reflect.Type {
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		value = value.Elem()
	}
	return value
}

func dashboardEmbedGeneratedModel(t *testing.T, rootType, modelType reflect.Type, model reflect.Value, parents map[reflect.Type]dashboardGeneratedParent) reflect.Value {
	t.Helper()
	path := make([]dashboardGeneratedParent, 0)
	for current := modelType; current != rootType; {
		parent, ok := parents[current]
		if !ok {
			t.Fatalf("generated model %s is not reachable from Dashboard", modelType.Name())
		}
		path = append(path, parent)
		current = parent.model
	}
	for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
		path[left], path[right] = path[right], path[left]
	}

	root := reflect.New(rootType).Elem()
	current := root
	for _, edge := range path {
		current = dashboardInitializeGeneratedField(current.Field(edge.fieldIndex))
	}
	current.Set(model)
	return root
}

func dashboardInitializeGeneratedField(field reflect.Value) reflect.Value {
	for {
		switch field.Kind() {
		case reflect.Pointer:
			field.Set(reflect.New(field.Type().Elem()))
			field = field.Elem()
		case reflect.Slice:
			field.Set(reflect.MakeSlice(field.Type(), 1, 1))
			field = field.Index(0)
		default:
			return field
		}
	}
}

func dashboardGeneratedOneOfBranches(t *testing.T) map[string][]string {
	t.Helper()
	pc := reflect.ValueOf(dashboardservice.Dashboard.ToMap).Pointer()
	function := runtime.FuncForPC(pc)
	if function == nil {
		t.Fatal("locate generated dashboard SDK source")
	}
	file, _ := function.FileLine(pc)
	typePattern := regexp.MustCompile(`(?m)^type ([A-Za-z0-9_]+) struct \{`)
	guardPattern := regexp.MustCompile(`oneOf field ([A-Za-z0-9_]+) must be set through the typed field`)
	result := make(map[string][]string)
	entries, err := os.ReadDir(filepath.Dir(file))
	if err != nil {
		t.Fatalf("read generated dashboard SDK source: %s", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "model_") || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(filepath.Dir(file), entry.Name()))
		if err != nil {
			t.Fatalf("read generated model %s: %s", entry.Name(), err)
		}
		guards := guardPattern.FindAllSubmatch(content, -1)
		if len(guards) == 0 {
			continue
		}
		modelMatch := typePattern.FindSubmatch(content)
		if len(modelMatch) != 2 {
			t.Fatalf("find generated model type in %s", entry.Name())
		}
		for _, guard := range guards {
			result[string(modelMatch[1])] = append(result[string(modelMatch[1])], string(guard[1]))
		}
	}
	return result
}

func dashboardGeneratedJSONField(model reflect.Value, jsonName string) (reflect.Value, bool) {
	for fieldIndex := 0; fieldIndex < model.NumField(); fieldIndex++ {
		field := model.Type().Field(fieldIndex)
		if strings.Split(field.Tag.Get("json"), ",")[0] == jsonName {
			return model.Field(fieldIndex), true
		}
	}
	return reflect.Value{}, false
}

func dashboardGeneratedJSONData(value reflect.Type, enumValues map[string]string, depth int) any {
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	if depth > 32 {
		return nil
	}
	switch value.Kind() {
	case reflect.Struct:
		result := make(map[string]any)
		for fieldIndex := 0; fieldIndex < value.NumField(); fieldIndex++ {
			field := value.Field(fieldIndex)
			jsonTag := field.Tag.Get("json")
			parts := strings.Split(jsonTag, ",")
			if len(parts) == 0 || parts[0] == "" || parts[0] == "-" || slicesContain(parts[1:], "omitempty") {
				continue
			}
			result[parts[0]] = dashboardGeneratedJSONData(field.Type, enumValues, depth+1)
		}
		return result
	case reflect.Map, reflect.Interface:
		return map[string]any{}
	case reflect.Slice, reflect.Array:
		return []any{}
	case reflect.String:
		if enumValue, ok := enumValues[value.Name()]; ok {
			return enumValue
		}
		return ""
	case reflect.Bool:
		return false
	default:
		return 0
	}
}

func dashboardGeneratedEnumValues(t *testing.T) map[string]string {
	t.Helper()
	pc := reflect.ValueOf(dashboardservice.Dashboard.ToMap).Pointer()
	function := runtime.FuncForPC(pc)
	if function == nil {
		t.Fatal("locate generated dashboard SDK source")
	}
	file, _ := function.FileLine(pc)
	constantPattern := regexp.MustCompile(`(?m)^\s*[A-Z][A-Z0-9_]*\s+([A-Za-z0-9_]+)\s+=\s+"([^"]+)"`)
	result := make(map[string]string)
	entries, err := os.ReadDir(filepath.Dir(file))
	if err != nil {
		t.Fatalf("read generated dashboard SDK source: %s", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "model_") || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(filepath.Dir(file), entry.Name()))
		if err != nil {
			t.Fatalf("read generated model %s: %s", entry.Name(), err)
		}
		for _, match := range constantPattern.FindAllSubmatch(content, -1) {
			if _, exists := result[string(match[1])]; !exists {
				result[string(match[1])] = string(match[2])
			}
		}
	}
	return result
}

func slicesContain(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func dashboardLowerCamelToSnake(value string) string {
	var result []rune
	for index, current := range []rune(value) {
		if unicode.IsUpper(current) {
			if index > 0 {
				result = append(result, '_')
			}
			current = unicode.ToLower(current)
		}
		result = append(result, current)
	}
	return string(result)
}

func dashboardJSONContainsKey(encoded []byte, key string) bool {
	var value any
	if json.Unmarshal(encoded, &value) != nil {
		return false
	}
	var contains func(any) bool
	contains = func(candidate any) bool {
		switch typed := candidate.(type) {
		case map[string]any:
			if _, ok := typed[key]; ok {
				return true
			}
			for _, nested := range typed {
				if contains(nested) {
					return true
				}
			}
		case []any:
			for _, nested := range typed {
				if contains(nested) {
					return true
				}
			}
		}
		return false
	}
	return contains(value)
}

func TestFlattenDashboardRejectsDynamicWidgetWithoutPartialState(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "testdata", "dashboards", "content_json_dynamic_queries_table.json"))
	if err != nil {
		t.Fatalf("read dynamic content_json fixture: %s", err)
	}

	dashboard := new(dashboardservice.Dashboard)
	if err := json.Unmarshal(content, dashboard); err != nil {
		t.Fatalf("unmarshal dynamic dashboard response: %s", err)
	}

	flattened, diags := flattenDashboard(context.Background(), DashboardResourceModel{
		ID:           types.StringValue("backend-dashboard-id"),
		Folder:       types.ObjectNull(dashboardFolderModelAttr()),
		ContentJson:  types.StringNull(),
		AccessPolicy: types.StringNull(),
	}, &dashboardOpenAPIReadResult{Dashboard: dashboard})
	if flattened != nil {
		t.Fatalf("flatten dynamic dashboard returned partial state: %#v", flattened)
	}
	if !diags.HasError() {
		t.Fatal("flatten dynamic dashboard returned no error diagnostic")
	}
	detail := diags.Errors()[0].Summary() + ": " + diags.Errors()[0].Detail()
	for _, expected := range []string{"Unsupported Dashboard Widget Definition", "dynamic", "content_json", "import", "data-source"} {
		if !strings.Contains(detail, expected) {
			t.Errorf("dynamic dashboard diagnostic %q does not contain %q", detail, expected)
		}
	}
}

func TestDashboardAccessPolicyForConfiguredRequest(t *testing.T) {
	policy := types.StringValue(`{"version":"2025-01-01"}`)

	tests := []struct {
		name   string
		config types.String
		plan   types.String
		want   *string
	}{
		{
			name:   "omitted config does not send preserved state",
			config: types.StringNull(),
			plan:   policy,
		},
		{
			name:   "configured policy sends planned value",
			config: policy,
			plan:   policy,
			want:   policy.ValueStringPointer(),
		},
		{
			name:   "configured empty value does not send",
			config: types.StringValue(""),
			plan:   types.StringValue(""),
		},
		{
			name:   "configured unknown sends planned value",
			config: types.StringUnknown(),
			plan:   policy,
			want:   policy.ValueStringPointer(),
		},
		{
			name:   "unknown plan does not send",
			config: types.StringUnknown(),
			plan:   types.StringUnknown(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dashboardAccessPolicyForConfiguredRequest(tt.config, tt.plan)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("expected nil access policy, got %q", *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("expected access policy %q, got %v", *tt.want, got)
			}
		})
	}
}

func TestExpandAnnotationID(t *testing.T) {
	tests := []struct {
		name   string
		id     types.String
		wantID string
	}{
		{name: "generates ID when omitted", id: types.StringNull()},
		{name: "preserves existing ID", id: types.StringValue("existing-id"), wantID: "existing-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotation, diags := expandAnnotation(context.Background(), DashboardAnnotationModel{
				ID:      tt.id,
				Name:    types.StringValue("test"),
				Enabled: types.BoolValue(true),
				Source:  types.ObjectNull(annotationSourceModelAttr()),
			})
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if annotation.Id == nil {
				t.Fatal("expected annotation ID, got nil")
			}

			if tt.wantID != "" {
				if *annotation.Id != tt.wantID {
					t.Fatalf("expected ID %q, got %q", tt.wantID, *annotation.Id)
				}
				return
			}
			if _, err := uuid.Parse(*annotation.Id); err != nil {
				t.Fatalf("expected generated UUID, got %q: %s", *annotation.Id, err)
			}
		})
	}
}

func TestExpandDashboardVariableDefinition_ConstantValueDeprecated(t *testing.T) {
	def := &DashboardVariableDefinitionModel{
		ConstantValue: types.StringValue("production"),
	}

	_, diags := expandDashboardVariableDefinition(context.Background(), def)
	if !diags.HasError() {
		t.Fatalf("expected an error for the deprecated constant_value, got none")
	}

	msg := diags.Errors()[0].Summary() + " " + diags.Errors()[0].Detail()
	if !strings.Contains(msg, "constant_value") || !strings.Contains(msg, "multi_select") {
		t.Fatalf("expected the error to direct users from constant_value to multi_select, got: %s", msg)
	}
}

func TestFlattenDashboardVariableDefinition_LegacyConstantBecomesMultiSelect(t *testing.T) {
	def := &dashboardservice.VariableDefinition{
		Constant: &dashboardservice.Constant{Value: ptr("production")},
	}

	got, diags := flattenDashboardVariableDefinition(context.Background(), def)
	if diags.HasError() {
		t.Fatalf("unexpected error flattening legacy constant: %v", diags)
	}
	if got.MultiSelect == nil {
		t.Fatalf("expected legacy Constant to flatten into multi_select, got %+v", got)
	}
	if !got.ConstantValue.IsNull() {
		t.Fatalf("expected constant_value to be null after remap, got %q", got.ConstantValue.ValueString())
	}
}

func TestMultiSelectSpansFieldNameRoundTrip(t *testing.T) {
	ctx := context.Background()
	query := &dashboardservice.QuerySpansQuery{
		Type: &dashboardservice.QuerySpansQueryType{
			FieldName: &dashboardservice.QuerySpansQueryTypeFieldName{
				SpanRegex: ptr(".*"),
			},
		},
	}

	flattened, diags := flattenDashboardVariableDefinitionMultiSelectQuerySpansModel(ctx, query)
	if diags.HasError() {
		t.Fatalf("flattening spans field-name query: %v", diags)
	}

	expanded, diags := expandMultiSelectSpansQuery(ctx, flattened)
	if diags.HasError() {
		t.Fatalf("expanding spans field-name query: %v", diags)
	}
	if expanded == nil || expanded.Type == nil || expanded.Type.FieldName == nil || expanded.Type.FieldName.GetSpanRegex() != ".*" {
		t.Fatalf("expected spans field-name regex to round-trip, got %#v", expanded)
	}
}

func TestMultiSelectAllSelectionRoundTripUsesEmptyList(t *testing.T) {
	expanded, diags := expandMultiSelectSelection(context.Background(), []attr.Value{})
	if diags.HasError() {
		t.Fatalf("expanding all selection: %v", diags)
	}
	if expanded == nil || expanded.All == nil {
		t.Fatalf("expected all selection, got %#v", expanded)
	}

	flattened, diags := flattenDashboardVariableSelectedValues(expanded)
	if diags.HasError() {
		t.Fatalf("flattening all selection: %v", diags)
	}
	if flattened.IsNull() || flattened.IsUnknown() || len(flattened.Elements()) != 0 {
		t.Fatalf("expected a known empty selected_values list, got %s", flattened)
	}
}

func TestAnnotationRangeStrategyRoundTrip(t *testing.T) {
	ctx := context.Background()
	observationField := func(key string) types.Object {
		return types.ObjectValueMust(dashboardwidgets.ObservationFieldAttr(), map[string]attr.Value{
			"keypath": types.ListValueMust(types.StringType, []attr.Value{types.StringValue(key)}),
			"scope":   types.StringValue("metadata"),
		})
	}
	rangeValue := types.ObjectValueMust(rangeStrategyModelAttr(), map[string]attr.Value{
		"start_timestamp_field": observationField("start"),
		"end_timestamp_field":   observationField("end"),
	})

	logs, diags := expandLogsSourceRangeStrategy(ctx, rangeValue)
	if diags.HasError() {
		t.Fatalf("expanding logs range strategy: %v", diags)
	}
	if logs == nil || logs.Range == nil {
		t.Fatalf("expected logs range strategy, got %#v", logs)
	}
	if _, diags := flattenLogsStrategyRange(ctx, logs.Range); diags.HasError() {
		t.Fatalf("flattening logs range strategy: %v", diags)
	}

	spans, diags := expandSpansSourceRangeStrategy(ctx, rangeValue)
	if diags.HasError() {
		t.Fatalf("expanding spans range strategy: %v", diags)
	}
	if spans == nil || spans.Range == nil {
		t.Fatalf("expected spans range strategy, got %#v", spans)
	}
	if _, diags := flattenSpansStrategyRange(ctx, spans.Range); diags.HasError() {
		t.Fatalf("flattening spans range strategy: %v", diags)
	}
}

func TestAnnotationLogsAndSpansExpansionOmitsDataModeType(t *testing.T) {
	ctx := context.Background()
	observationField := types.ObjectValueMust(dashboardwidgets.ObservationFieldAttr(), map[string]attr.Value{
		"keypath": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("timestamp")}),
		"scope":   types.StringValue("metadata"),
	})
	strategy := types.ObjectValueMust(logsAndSpansStrategyModelAttr(), map[string]attr.Value{
		"instant": types.ObjectValueMust(instantStrategyModelAttr(), map[string]attr.Value{
			"timestamp_field": observationField,
		}),
		"range":    types.ObjectNull(rangeStrategyModelAttr()),
		"duration": types.ObjectNull(durationStrategyModelAttr()),
	})
	source := types.ObjectValueMust(annotationsLogsAndSpansSourceModelAttr(), map[string]attr.Value{
		"lucene_query":     types.StringValue("*"),
		"strategy":         strategy,
		"message_template": types.StringNull(),
		"label_fields":     types.ListNull(types.ObjectType{AttrTypes: dashboardwidgets.ObservationFieldAttr()}),
	})

	logs, diags := expandLogsSource(ctx, source)
	if diags.HasError() {
		t.Fatalf("expanding logs annotation source: %v", diags)
	}
	if logs == nil || logs.DataModeType != nil {
		t.Fatalf("expected logs data_mode_type to be omitted from the request, got %#v", logs)
	}

	spans, diags := expandSpansSource(ctx, source)
	if diags.HasError() {
		t.Fatalf("expanding spans annotation source: %v", diags)
	}
	if spans == nil || spans.DataModeType != nil {
		t.Fatalf("expected spans data_mode_type to be omitted from the request, got %#v", spans)
	}
}

func TestFlattenDashboardOptionsColor(t *testing.T) {
	tests := []struct {
		name       string
		color      *dashboardservice.SectionColor
		wantNull   bool
		wantString string
	}{
		{
			name:     "color absent (nil)",
			color:    nil,
			wantNull: true,
		},
		{
			name: "predefined unspecified (zero value) is null",
			color: &dashboardservice.SectionColor{
				Predefined: dashboardservice.SECTIONPREDEFINEDCOLOR_SECTION_PREDEFINED_COLOR_UNSPECIFIED.Ptr(),
			},
			wantNull: true,
		},
		{
			name:     "color wrapper present but value oneof unset is null",
			color:    &dashboardservice.SectionColor{},
			wantNull: true,
		},
		{
			name: "predefined cyan flattens to lowercase string",
			color: &dashboardservice.SectionColor{
				Predefined: dashboardservice.SECTIONPREDEFINEDCOLOR_SECTION_PREDEFINED_COLOR_CYAN.Ptr(),
			},
			wantString: "cyan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &dashboardservice.SectionOptions{
				Custom: &dashboardservice.CustomSectionOptions{
					Name:  ptr("section"),
					Color: tt.color,
				},
			}
			model, diags := flattenDashboardOptions(context.Background(), opts)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			if model == nil {
				t.Fatalf("expected non-nil model, got nil")
			}
			if tt.wantNull {
				if !model.Color.IsNull() {
					t.Fatalf("expected null color, got %q", model.Color.ValueString())
				}
				return
			}
			if model.Color.IsNull() {
				t.Fatalf("expected color %q, got null", tt.wantString)
			}
			if got := model.Color.ValueString(); got != tt.wantString {
				t.Fatalf("expected color %q, got %q", tt.wantString, got)
			}
		})
	}
}
